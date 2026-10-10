package fbhttp

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/net/webdav"

	"github.com/filebrowser/filebrowser/v2/files"
	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/share"
	"github.com/filebrowser/filebrowser/v2/storage"
	"github.com/filebrowser/filebrowser/v2/users"
)

// Gezgin serves the WebDAV shares on a port of their own: /<hash>/ is the folder a share names, for
// the share's username and password. Nothing else of Gezgin is on that port.

// davLoginTTL is how long a password that passed is taken without checking its hash again: clients
// send it with every request, and a bcrypt check costs tens of milliseconds.
const davLoginTTL = 10 * time.Minute

// davWrites are the methods that change a share.
var davWrites = map[string]bool{
	"PUT": true, "DELETE": true, "MKCOL": true, "COPY": true, "MOVE": true,
	"PROPPATCH": true, "LOCK": true, "UNLOCK": true,
}

type davServer struct {
	store     *storage.Storage
	server    *settings.Server
	fileCache FileCache
	limiter   *loginLimiter

	mu     sync.Mutex
	logins map[[sha256.Size]byte]time.Time // until when a login holds
	locks  map[string]webdav.LockSystem    // by share
}

// NewWebDAVHandler serves the WebDAV shares. It goes on a port of its own.
func NewWebDAVHandler(fileCache FileCache, store *storage.Storage, server *settings.Server) http.Handler {
	return &davServer{
		store:     store,
		server:    server,
		fileCache: fileCache,
		limiter:   newLoginLimiter(),
		logins:    map[[sha256.Size]byte]time.Time{},
		locks:     map[string]webdav.LockSystem{},
	}
}

func (s *davServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hash, _, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")
	if hash == "" {
		http.NotFound(w, r)
		return
	}
	link, err := s.store.Share.GetByHash(hash)
	if err != nil || link.Kind != share.KindWebDAV {
		http.NotFound(w, r)
		return
	}

	if status, wait := s.authenticate(r, link); status != 0 {
		if status == http.StatusTooManyRequests {
			w.Header().Set("Retry-After", strconv.Itoa(int((wait+time.Second-1)/time.Second)))
		} else {
			w.Header().Set("WWW-Authenticate", `Basic realm="Gezgin", charset="UTF-8"`)
		}
		http.Error(w, http.StatusText(status), status)
		return
	}

	set, err := s.store.Settings.Get()
	if err != nil {
		log.Printf("webdav: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	owner, err := s.store.Users.Get(s.server.Root, s.server.FollowExternalSymlinks, link.UserID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	d := &data{
		settings: set,
		server:   s.server,
		store:    s.store,
		user:     owner,
	}

	// The owner's permissions and rules hold as they are now, as for a link to a page.
	base := slashClean(link.Path)
	if !owner.Perm.Share || !owner.Perm.Download || !d.CheckRules(base) {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	if info, err := owner.Fs.Stat(base); err != nil || !info.IsDir() {
		http.NotFound(w, r)
		return
	}
	writable := link.Writable && davCanWrite(owner.Perm)
	if davWrites[r.Method] && !writable {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	if r.Method == http.MethodPut {
		body := &davBody{ReadCloser: r.Body}
		r.Body = body
		r = r.WithContext(context.WithValue(r.Context(), davBodyKey{}, body))
	}

	handler := &webdav.Handler{
		Prefix: "/" + hash,
		FileSystem: &davFS{
			d:         d,
			fs:        files.NewFs(owner.Fs, base, s.server.FollowExternalSymlinks),
			base:      base,
			writable:  writable,
			fileCache: s.fileCache,
		},
		LockSystem: s.lockSystem(hash),
		// Clients look for what is not there and make folders that are: those are no errors.
		Logger: func(r *http.Request, err error) {
			if err != nil && !errors.Is(err, fs.ErrNotExist) && !errors.Is(err, fs.ErrExist) {
				log.Printf("webdav %s %s: %v", r.Method, r.URL.Path, err)
			}
		},
	}
	if r.Method == http.MethodGet || r.Method == http.MethodPut {
		key := davTransfers(hash)
		if !running.begin(key) {
			_, _ = tooMany(w, r, tooManyTransfers.For(r.Header.Get("Accept-Language")))
			return
		}
		defer running.end(key)
	}
	handler.ServeHTTP(w, r)
}

// authenticate checks the request's Basic credentials against the share. A password that passed
// lately is taken from memory; wrong ones are limited like logins, per address and per share. It
// returns 0, or the status to refuse with and, for 429, how long to wait.
func (s *davServer) authenticate(r *http.Request, link *share.Link) (int, time.Duration) {
	username, password, ok := r.BasicAuth()
	if !ok {
		return http.StatusUnauthorized, 0
	}

	key := sha256.Sum256([]byte(link.Hash + "\x00" + link.PasswordHash + "\x00" + username + "\x00" + password))
	now := time.Now()
	s.mu.Lock()
	until, known := s.logins[key]
	s.mu.Unlock()
	if known && now.Before(until) {
		return 0, 0
	}

	address, account := loginAddress(r), "dav\x00"+link.Hash
	attempt, wait := s.limiter.begin(address, account)
	if wait > 0 {
		return http.StatusTooManyRequests, wait
	}
	nameOK := subtle.ConstantTimeCompare([]byte(username), []byte(link.Username)) == 1
	passwordOK := bcrypt.CompareHashAndPassword([]byte(link.PasswordHash), []byte(password)) == nil
	if !nameOK || !passwordOK {
		return http.StatusUnauthorized, 0
	}
	s.limiter.succeeded(address, account, attempt)

	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.logins) >= loginSweepSize {
		for k, until := range s.logins {
			if !now.Before(until) {
				delete(s.logins, k)
			}
		}
	}
	s.logins[key] = now.Add(davLoginTTL)
	return 0, 0
}

// davCanWrite reports whether a user's permissions cover what writing to a WebDAV share takes.
func davCanWrite(perm users.Permissions) bool {
	return perm.Create && perm.Modify && perm.Rename && perm.Delete
}

// lockSystem returns the share's locks; clients such as Finder lock a file before writing it.
func (s *davServer) lockSystem(hash string) webdav.LockSystem {
	s.mu.Lock()
	defer s.mu.Unlock()
	ls, ok := s.locks[hash]
	if !ok {
		ls = webdav.NewMemLS()
		s.locks[hash] = ls
	}
	return ls
}
