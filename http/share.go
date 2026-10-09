package fbhttp

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	fberrors "github.com/filebrowser/filebrowser/v2/errors"
	"github.com/filebrowser/filebrowser/v2/files"
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

	owners := map[uint]*users.User{d.user.ID: d.user}
	items := make([]shareListItem, 0, len(s))
	for _, l := range s {
		owner, ok := owners[l.UserID]
		if !ok {
			if owner, err = d.store.Users.Get(d.server.Root, d.server.FollowExternalSymlinks, l.UserID); err != nil {
				owner = nil
			}
			owners[l.UserID] = owner
		}
		items = append(items, d.describeShare(l, owner))
	}
	return renderJSON(w, r, items)
})

// shareListItem is a share as the "Paylaşılanlar" page lists it (Gezgin, K95): with its item's
// name, whether it is a folder and a file's type, the folder it lies in, the path the requesting
// user opens it at when it lies in their scope, and, for an admin, the owner of another's share.
type shareListItem struct {
	shareResponse
	Name   string `json:"name"`
	IsDir  bool   `json:"isDir"`
	Type   string `json:"type,omitempty"`
	Folder string `json:"folder"`
	Open   string `json:"open,omitempty"`
	Owner  string `json:"owner,omitempty"`
}

func (d *data) describeShare(l *share.Link, owner *users.User) shareListItem {
	item := shareListItem{shareResponse: *toShareResponse(l)}
	if l.Path != "/" {
		item.Name = path.Base(l.Path)
	}
	where := l.Path
	if owner != nil {
		if owner.ID != d.user.ID {
			item.Owner = owner.Username
		}
		full := owner.FullPath(l.Path)
		if info, err := os.Stat(full); err == nil {
			item.IsDir = info.IsDir()
			if !item.IsDir {
				item.Type = files.NameType(info.Name(), info.Size())
			}
		}
		if names, ok := d.inside(d.user.FullPath("/"), full); ok {
			item.Open = "/" + strings.Join(names, "/")
			where = item.Open
		}
	}
	item.Folder = path.Dir(where)
	return item
}

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
		if token, err = newShareToken(); err != nil {
			return http.StatusInternalServerError, err
		}
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

// newShareToken returns the random token a password-protected link takes to its downloads.
func newShareToken() (string, error) {
	tokenBuffer := make([]byte, 96)
	if _, err := rand.Read(tokenBuffer); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(tokenBuffer), nil
}

// shareUpdate is a change to a share, whose address stays (Gezgin, K96). Expires, when set, gives
// a new duration counted from now in Unit ("0": permanent); PasswordAction keeps ("" or "keep"),
// sets or removes the password; Writable, when set, makes a WebDAV share read-only or read-write.
type shareUpdate struct {
	Expires        *string `json:"expires"`
	Unit           string  `json:"unit"`
	PasswordAction string  `json:"passwordAction"`
	Password       string  `json:"password"`
	Writable       *bool   `json:"writable"`
}

var sharePatchHandler = withPermShare(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	hash := strings.Trim(r.URL.Path, "/")
	if hash == "" {
		return http.StatusBadRequest, nil
	}
	link, err := d.store.Share.GetByHash(hash)
	if err != nil {
		return errToStatus(err), err
	}
	// An admin changes everyone's shares, a user their own (K97).
	if link.UserID != d.user.ID && !d.user.Perm.Admin {
		return http.StatusForbidden, nil
	}
	var body shareUpdate
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return http.StatusBadRequest, fmt.Errorf("%w: %w", fberrors.ErrInvalidRequestParams, err)
	}

	if body.Expires != nil {
		expire, err := shareExpiry(share.CreateBody{Expires: *body.Expires, Unit: body.Unit}, time.Now())
		if err != nil {
			return http.StatusBadRequest, err
		}
		link.Expire = expire
	}

	switch body.PasswordAction {
	case "", "keep":
	case "set":
		if link.Kind == share.KindWebDAV {
			// A WebDAV password is held to the account rules (K41).
			pwd, err := users.ValidateAndHashPwd(body.Password, d.settings.MinimumPasswordLength)
			if err != nil {
				return http.StatusBadRequest, err
			}
			link.PasswordHash = pwd
			break
		}
		if body.Password == "" {
			return http.StatusBadRequest, fberrors.ErrEmptyPassword
		}
		pwd, status, err := getSharePasswordHash(share.CreateBody{Password: body.Password})
		if err != nil {
			return status, err
		}
		link.PasswordHash = string(pwd)
		// A new password ends the downloads the old one let through.
		if link.Token, err = newShareToken(); err != nil {
			return http.StatusInternalServerError, err
		}
	case "remove":
		if link.Kind == share.KindWebDAV {
			return http.StatusBadRequest, fmt.Errorf("a WebDAV share keeps a password: %w", fberrors.ErrInvalidRequestParams)
		}
		link.PasswordHash, link.Token = "", ""
	default:
		return http.StatusBadRequest, fmt.Errorf("unknown password action %q: %w", body.PasswordAction, fberrors.ErrInvalidRequestParams)
	}

	if body.Writable != nil && *body.Writable != link.Writable {
		if link.Kind != share.KindWebDAV {
			return http.StatusBadRequest, fmt.Errorf("only a WebDAV share is writable: %w", fberrors.ErrInvalidRequestParams)
		}
		if *body.Writable {
			// The share writes as its owner, who must be allowed to (K42).
			owner := d.user
			if link.UserID != d.user.ID {
				if owner, err = d.store.Users.Get(d.server.Root, d.server.FollowExternalSymlinks, link.UserID); err != nil {
					return errToStatus(err), err
				}
			}
			if !davCanWrite(owner.Perm) {
				return http.StatusForbidden, errors.New("a writable WebDAV share needs the create, modify, rename and delete permissions")
			}
		}
		link.Writable = *body.Writable
	}

	if err := d.store.Share.Save(link); err != nil {
		return http.StatusInternalServerError, err
	}
	return renderJSON(w, r, toShareResponse(link))
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
