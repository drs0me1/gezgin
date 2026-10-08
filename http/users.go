package fbhttp

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/filebrowser/filebrowser/v2/auth"
	fberrors "github.com/filebrowser/filebrowser/v2/errors"
	"github.com/filebrowser/filebrowser/v2/users"
)

var (
	NonModifiableFieldsForNonAdmin = []string{"Username", "Scope", "LockPassword", "Perm", "Commands", "Rules"}

	// NonModifiableFields follow the password and the sessions; no request sets them.
	NonModifiableFields = []string{"SecurityStamp", "MustChangePassword"}
)

type modifyUserRequest struct {
	modifyRequest
	Data *users.User `json:"data"`
}

func getUserID(r *http.Request) (uint, error) {
	vars := mux.Vars(r)
	i, err := strconv.ParseUint(vars["id"], 10, 0)
	if err != nil {
		return 0, err
	}
	return uint(i), err
}

func getUser(_ http.ResponseWriter, r *http.Request) (*modifyUserRequest, error) {
	if r.Body == nil {
		return nil, fberrors.ErrEmptyRequest
	}

	req := &modifyUserRequest{}
	err := json.NewDecoder(r.Body).Decode(req)
	if err != nil {
		return nil, err
	}

	if req.What != "user" {
		return nil, fberrors.ErrInvalidDataType
	}

	return req, nil
}

func withSelfOrAdmin(fn handleFunc) handleFunc {
	return withUser(selfOrAdmin(fn))
}

func selfOrAdmin(fn handleFunc) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		id, err := getUserID(r)
		if err != nil {
			return http.StatusInternalServerError, err
		}

		if d.user.ID != id && !d.user.Perm.Admin {
			return http.StatusForbidden, nil
		}

		d.raw = id
		return fn(w, r, d)
	}
}

var usersGetHandler = withAdmin(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	users, err := d.store.Users.Gets(d.server.Root, d.server.FollowExternalSymlinks)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	for _, u := range users {
		u.Password = ""
		u.SecurityStamp = ""
	}

	sort.Slice(users, func(i, j int) bool {
		return users[i].ID < users[j].ID
	})

	return renderJSON(w, r, users)
})

var userGetHandler = withSelfOrAdmin(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	u, err := d.store.Users.Get(d.server.Root, d.server.FollowExternalSymlinks, d.raw.(uint))
	if errors.Is(err, fberrors.ErrNotExist) {
		return http.StatusNotFound, err
	}

	if err != nil {
		return http.StatusInternalServerError, err
	}

	u.Password = ""
	u.SecurityStamp = ""
	if !d.user.Perm.Admin {
		u.Scope = ""
	}
	return renderJSON(w, r, u)
})

var userDeleteHandler = withSelfOrAdmin(func(_ http.ResponseWriter, r *http.Request, d *data) (int, error) {
	if r.Body == nil {
		return http.StatusBadRequest, fberrors.ErrEmptyRequest
	}

	var body struct {
		CurrentPassword string `json:"current_password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return http.StatusBadRequest, err
	}

	if d.settings.AuthMethod == auth.MethodJSONAuth {
		if !users.CheckPwd(body.CurrentPassword, d.user.Password) {
			return http.StatusBadRequest, fberrors.ErrCurrentPasswordIncorrect
		}
	}

	err := d.store.Users.Delete(d.raw.(uint))
	if err != nil {
		return errToStatus(err), err
	}

	return http.StatusOK, nil
})

var userPostHandler = withAdmin(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	req, err := getUser(w, r)
	if err != nil {
		return http.StatusBadRequest, err
	}

	if d.settings.AuthMethod == auth.MethodJSONAuth {
		if !users.CheckPwd(req.CurrentPassword, d.user.Password) {
			return http.StatusBadRequest, fberrors.ErrCurrentPasswordIncorrect
		}
	}

	if len(req.Which) != 0 {
		return http.StatusBadRequest, nil
	}

	if req.Data.Password == "" {
		return http.StatusBadRequest, fberrors.ErrEmptyPassword
	}

	req.Data.Password, err = users.ValidateAndHashPwd(req.Data.Password, d.settings.MinimumPasswordLength)
	if err != nil {
		return http.StatusBadRequest, err
	}

	if req.Data.Perm.Share && !req.Data.Perm.Download {
		return http.StatusBadRequest, fberrors.ErrShareRequiresDownload
	}

	req.Data.SecurityStamp = ""
	req.Data.MustChangePassword = false

	userHome, err := d.settings.MakeUserDir(req.Data.Username, req.Data.Scope, d.server.Root)
	if err != nil {
		log.Printf("create user: failed to mkdir user home dir: [%s]", userHome)
		return http.StatusInternalServerError, err
	}
	req.Data.Scope = userHome
	log.Printf("user: %s, home dir: [%s].", req.Data.Username, userHome)

	err = d.store.Users.Save(req.Data)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	w.Header().Set("Location", "/settings/users/"+strconv.FormatUint(uint64(req.Data.ID), 10))
	return http.StatusCreated, nil
})

var userPutHandler = withPasswordChange(selfOrAdmin(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	req, err := getUser(w, r)
	if err != nil {
		return http.StatusBadRequest, err
	}

	self := d.raw.(uint) == d.user.ID
	var stampChanged bool

	// Until the forced change is done, the user's own new password is the only change accepted.
	if d.user.MustChangePassword && (!self || len(req.Which) != 1 || !strings.EqualFold(req.Which[0], "password")) {
		return http.StatusForbidden, nil
	}

	if d.settings.AuthMethod == auth.MethodJSONAuth {
		var sensibleFields = map[string]struct{}{
			"all":          {},
			"username":     {},
			"password":     {},
			"scope":        {},
			"lockPassword": {},
			"commands":     {},
			"perm":         {},
		}

		for _, field := range req.Which {
			if _, ok := sensibleFields[strings.ToLower(field)]; ok {
				if !users.CheckPwd(req.CurrentPassword, d.user.Password) {
					return http.StatusBadRequest, fberrors.ErrCurrentPasswordIncorrect
				}
				break
			}
		}
	}

	if req.Data.ID != d.raw.(uint) {
		return http.StatusBadRequest, nil
	}

	// The forced change replaces the password the log showed: it may not be kept.
	if d.user.MustChangePassword && users.CheckPwd(req.Data.Password, d.user.Password) {
		return http.StatusBadRequest, fberrors.ErrPasswordUnchanged
	}

	for _, field := range req.Which {
		if strings.ToLower(field) == "perm" || strings.ToLower(field) == "all" {
			if req.Data.Perm.Share && !req.Data.Perm.Download {
				return http.StatusBadRequest, fberrors.ErrShareRequiresDownload
			}
		}
	}

	if len(req.Which) == 0 || (len(req.Which) == 1 && req.Which[0] == "all") {
		if !d.user.Perm.Admin {
			return http.StatusForbidden, nil
		}

		suser, err := d.store.Users.Get(d.server.Root, d.server.FollowExternalSymlinks, d.raw.(uint))
		if err != nil {
			return errToStatus(err), err
		}
		req.Data.SecurityStamp = suser.SecurityStamp
		req.Data.MustChangePassword = suser.MustChangePassword

		if req.Data.Password != "" {
			req.Data.Password, err = users.ValidateAndHashPwd(req.Data.Password, d.settings.MinimumPasswordLength)
			if err != nil {
				return http.StatusBadRequest, err
			}
			if err = passwordChanged(req.Data, self); err != nil {
				return http.StatusInternalServerError, err
			}
			stampChanged = true
		} else {
			req.Data.Password = suser.Password
		}

		req.Which = []string{}
	}

	var followers []string
	for k, v := range req.Which {
		v = cases.Title(language.English, cases.NoLower).String(v)
		req.Which[k] = v

		if slices.Contains(NonModifiableFields, v) {
			return http.StatusForbidden, nil
		}

		if v == "Password" {
			if !d.user.Perm.Admin && d.user.LockPassword {
				return http.StatusForbidden, nil
			}

			req.Data.Password, err = users.ValidateAndHashPwd(req.Data.Password, d.settings.MinimumPasswordLength)
			if err != nil {
				return http.StatusBadRequest, err
			}
			if err = passwordChanged(req.Data, self); err != nil {
				return http.StatusInternalServerError, err
			}
			stampChanged = true
			followers = []string{"SecurityStamp"}
			if self {
				followers = append(followers, "MustChangePassword")
			}
		}

		for _, f := range NonModifiableFieldsForNonAdmin {
			if !d.user.Perm.Admin && v == f {
				return http.StatusForbidden, nil
			}
		}
	}

	err = d.store.Users.Update(req.Data, append(req.Which, followers...)...)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	if self && stampChanged {
		noRenewal(w)
	}
	return http.StatusOK, nil
}))

// noRenewal withdraws the renewal offer of a request that ended its own session: the token cannot
// be renewed any more.
func noRenewal(w http.ResponseWriter) {
	w.Header().Set("X-Renew-Token", "false")
}

// passwordChanged gives a user whose password changed a new security stamp, which ends the
// sessions issued with the old password. A user who chose the password themselves has also done
// any forced change.
func passwordChanged(user *users.User, self bool) error {
	stamp, err := users.NewSecurityStamp()
	if err != nil {
		return err
	}
	user.SecurityStamp = stamp
	if self {
		user.MustChangePassword = false
	}
	return nil
}

// userSessionsDeleteHandler ends every session of a user by giving them a new security stamp.
var userSessionsDeleteHandler = withSelfOrAdmin(func(w http.ResponseWriter, _ *http.Request, d *data) (int, error) {
	user, err := d.store.Users.Get(d.server.Root, d.server.FollowExternalSymlinks, d.raw.(uint))
	if err != nil {
		return errToStatus(err), err
	}

	if user.SecurityStamp, err = users.NewSecurityStamp(); err != nil {
		return http.StatusInternalServerError, err
	}
	if err = d.store.Users.Update(user, "SecurityStamp"); err != nil {
		return http.StatusInternalServerError, err
	}

	log.Printf("user %s: all sessions closed", user.Username)
	if user.ID == d.user.ID {
		noRenewal(w)
	}
	return http.StatusOK, nil
})
