package fbhttp

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	fberrors "github.com/filebrowser/filebrowser/v2/errors"
	"github.com/filebrowser/filebrowser/v2/share"
	"github.com/filebrowser/filebrowser/v2/users"
	"golang.org/x/crypto/bcrypt"
)

// shareResponse is the client-facing representation of a share. It deliberately
// omits the server-side secrets of share.Link — the bcrypt PasswordHash (which
// would be crackable offline) and the bypass Token — exposing only whether the
// share is password-protected via HasPassword.
type shareResponse struct {
	Hash        string `json:"hash"`
	Path        string `json:"path"`
	UserID      uint   `json:"userID"`
	Expire      int64  `json:"expire"`
	HasPassword bool   `json:"hasPassword"`
	Kind        string `json:"kind,omitempty"`
	WebDAVUser  string `json:"webdavUser,omitempty"`
	Writable    bool   `json:"writable,omitempty"`
}

func toShareResponse(l *share.Link) *shareResponse {
	return &shareResponse{
		Hash:        l.Hash,
		Path:        l.Path,
		UserID:      l.UserID,
		Expire:      l.Expire,
		HasPassword: l.PasswordHash != "",
		Kind:        l.Kind,
		WebDAVUser:  l.Username,
		Writable:    l.Writable,
	}
}

func toShareResponses(links []*share.Link) []*shareResponse {
	res := make([]*shareResponse, 0, len(links))
	for _, l := range links {
		res = append(res, toShareResponse(l))
	}
	return res
}

func withPermShare(fn handleFunc) handleFunc {
	return withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		if !d.user.Perm.Share || !d.user.Perm.Download {
			return http.StatusForbidden, nil
		}

		return fn(w, r, d)
	})
}

var shareListHandler = withPermShare(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	var (
		s   []*share.Link
		err error
	)
	if d.user.Perm.Admin {
		s, err = d.store.Share.All()
	} else {
		s, err = d.store.Share.FindByUserID(d.user.ID)
	}
	if errors.Is(err, fberrors.ErrNotExist) {
		return renderJSON(w, r, []*shareResponse{})
	}

	if err != nil {
		return http.StatusInternalServerError, err
	}

	sort.Slice(s, func(i, j int) bool {
		if s[i].UserID != s[j].UserID {
			return s[i].UserID < s[j].UserID
		}
		return s[i].Expire < s[j].Expire
	})

	return renderJSON(w, r, toShareResponses(s))
})

var shareGetsHandler = withPermShare(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	var (
		s   []*share.Link
		err error
	)
	if d.user.Perm.Admin {
		s, err = getSharesForAdminPath(d, r.URL.Path)
	} else {
		s, err = d.store.Share.Gets(r.URL.Path, d.user.ID)
	}
	if errors.Is(err, fberrors.ErrNotExist) {
		return renderJSON(w, r, []*shareResponse{})
	}

	if err != nil {
		return http.StatusInternalServerError, err
	}

	return renderJSON(w, r, toShareResponses(s))
})

func getSharesForAdminPath(d *data, path string) ([]*share.Link, error) {
	links, err := d.store.Share.All()
	if err != nil {
		return nil, err
	}

	adminPath := filepath.Clean(d.user.FullPath(path))
	owners := make(map[uint]*users.User)
	filtered := make([]*share.Link, 0, len(links))
	for _, link := range links {
		owner, ok := owners[link.UserID]
		if !ok {
			owner, err = d.store.Users.Get(d.server.Root, d.server.FollowExternalSymlinks, link.UserID)
			if err != nil && !errors.Is(err, fberrors.ErrNotExist) {
				return nil, err
			}
			owners[link.UserID] = owner // owner is nil on ErrNotExist
		}
		if owner != nil && filepath.Clean(owner.FullPath(link.Path)) == adminPath {
			filtered = append(filtered, link)
		}
	}

	return filtered, nil
}

var shareDeleteHandler = withPermShare(func(_ http.ResponseWriter, r *http.Request, d *data) (int, error) {
	hash := strings.TrimSuffix(r.URL.Path, "/")
	hash = strings.TrimPrefix(hash, "/")

	if hash == "" {
		return http.StatusBadRequest, nil
	}

	link, err := d.store.Share.GetByHash(hash)
	if err != nil {
		return errToStatus(err), err
	}

	if link.UserID != d.user.ID && !d.user.Perm.Admin {
		return http.StatusForbidden, nil
	}

	err = d.store.Share.Delete(hash)
	return errToStatus(err), err
})

var sharePostHandler = withPermShare(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	// Only what the user may see can be shared (Gezgin); a link to anything
	// else would be refused when it is opened anyway.
	if !d.Check(r.URL.Path) {
		return http.StatusForbidden, nil
	}

	// Only allow sharing paths that currently exist. Otherwise a share could be
	// created for a non-existent path and would silently start exposing
	// whatever file later appears there.
	//
	// d.user.Fs is scoped, so Stat also refuses to follow a symlink whose target
	// escapes the user's scope: that returns a permission error here and so
	// blocks creating a share that points out of scope.
	info, err := d.user.Fs.Stat(r.URL.Path)
	if err != nil {
		return errToStatus(err), err
	}

	var s *share.Link
	var body share.CreateBody
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return http.StatusBadRequest, fmt.Errorf("failed to decode body: %w", err)
		}
		defer r.Body.Close()
	}
	if status, err := checkWebDAVShare(d, &body, info); status != 0 {
		return status, err
	}

	// 96 random bits name the link (Gezgin; File Browser used 48).
	bytes := make([]byte, 12)
	_, err = rand.Read(bytes)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	str := base64.URLEncoding.EncodeToString(bytes)

	expire, err := shareExpiry(body, time.Now())
	if err != nil {
		return http.StatusBadRequest, err
	}

	var hash []byte
	if body.Kind == share.KindWebDAV {
		// A WebDAV password is held to the account rules (Gezgin).
		pwd, err := users.ValidateAndHashPwd(body.Password, d.settings.MinimumPasswordLength)
		if err != nil {
			return http.StatusBadRequest, err
		}
		hash = []byte(pwd)
	} else {
		var status int
		if hash, status, err = getSharePasswordHash(body); err != nil {
			return status, err
		}
	}

	// A link to a page takes its token to the downloads; WebDAV asks for the password every time.
	var token string
	if len(hash) > 0 && body.Kind == "" {
		tokenBuffer := make([]byte, 96)
		if _, err := rand.Read(tokenBuffer); err != nil {
			return http.StatusInternalServerError, err
		}
		token = base64.URLEncoding.EncodeToString(tokenBuffer)
	}

	s = &share.Link{
		Path:         r.URL.Path,
		Hash:         str,
		Expire:       expire,
		UserID:       d.user.ID,
		PasswordHash: string(hash),
		Token:        token,
		Kind:         body.Kind,
		Username:     body.Username,
		Writable:     body.Writable,
	}

	if err := d.store.Share.Save(s); err != nil {
		return http.StatusInternalServerError, err
	}

	return renderJSON(w, r, toShareResponse(s))
})

// checkWebDAVShare checks the kind of a share about to be made of the item described by info
// (Gezgin). A WebDAV share needs the WebDAV port on and a folder, a username Basic authentication
// can carry (no colon), and a password; only a user who may create, change, rename and delete may
// let it be written to. It returns 0 when the share may be made.
func checkWebDAVShare(d *data, body *share.CreateBody, info os.FileInfo) (int, error) {
	switch body.Kind {
	case "":
		if body.Username != "" || body.Writable {
			return http.StatusBadRequest, fmt.Errorf("only a WebDAV share has a username or is writable: %w", fberrors.ErrInvalidRequestParams)
		}
		return 0, nil
	case share.KindWebDAV:
	default:
		return http.StatusBadRequest, fmt.Errorf("unknown share kind %q: %w", body.Kind, fberrors.ErrInvalidRequestParams)
	}

	if d.server.WebDAVPort == "" {
		return http.StatusBadRequest, errors.New("WebDAV shares are off")
	}
	if !info.IsDir() {
		return http.StatusBadRequest, errors.New("only a folder is shared over WebDAV")
	}
	body.Username = strings.TrimSpace(body.Username)
	if body.Username == "" || utf8.RuneCountInString(body.Username) > 64 ||
		strings.Contains(body.Username, ":") || strings.ContainsFunc(body.Username, unicode.IsControl) {
		return http.StatusBadRequest, errors.New("a WebDAV share needs a username of at most 64 characters, without a colon")
	}
	if body.Writable && !davCanWrite(d.user.Perm) {
		return http.StatusForbidden, errors.New("a writable WebDAV share needs the create, modify, rename and delete permissions")
	}
	return 0, nil
}

// maxShareDuration bounds how long a link may last (Gezgin), as the trash bounds how long it keeps
// an item.
const maxShareDuration = 3650 * 24 * time.Hour

var shareUnits = map[string]time.Duration{
	"seconds": time.Second,
	"minutes": time.Minute,
	"hours":   time.Hour,
	"":        time.Hour,
	"days":    24 * time.Hour,
}

// shareExpiry returns when a link created at now with the body's duration ends, as a Unix time; 0
// is never. A duration that is not a whole number of a known unit, is negative or is longer than
// maxShareDuration is refused (Gezgin).
func shareExpiry(body share.CreateBody, now time.Time) (int64, error) {
	if body.Expires == "" {
		return 0, nil
	}
	num, err := strconv.ParseInt(body.Expires, 10, 64)
	if err != nil || num < 0 {
		return 0, fmt.Errorf("invalid share duration %q: %w", body.Expires, fberrors.ErrInvalidRequestParams)
	}
	unit, ok := shareUnits[body.Unit]
	if !ok {
		return 0, fmt.Errorf("invalid share duration unit %q: %w", body.Unit, fberrors.ErrInvalidRequestParams)
	}
	if num == 0 {
		return 0, nil
	}
	if num > int64(maxShareDuration/unit) {
		return 0, fmt.Errorf("a share lasts at most 10 years: %w", fberrors.ErrInvalidRequestParams)
	}
	return now.Add(time.Duration(num) * unit).Unix(), nil
}

func getSharePasswordHash(body share.CreateBody) (data []byte, statuscode int, err error) {
	if body.Password == "" {
		return nil, 0, nil
	}
	if len(body.Password) > users.MaxPasswordBytes {
		return nil, http.StatusBadRequest, fberrors.ErrPasswordTooLong
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to hash password: %w", err)
	}

	return hash, 0, nil
}
