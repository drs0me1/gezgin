package users

import (
	"path/filepath"

	fberrors "github.com/filebrowser/filebrowser/v2/errors"
	"github.com/filebrowser/filebrowser/v2/files"
	"github.com/filebrowser/filebrowser/v2/rules"
	"github.com/spf13/afero"
)

// ViewMode describes a view mode.
type ViewMode string

const (
	ListViewMode          ViewMode = "list"
	MosaicViewMode        ViewMode = "mosaic"
	MosaicGalleryViewMode ViewMode = "mosaic gallery"
)

// User describes a user.
type User struct {
	ID                    uint          `storm:"id,increment" json:"id"`
	Username              string        `storm:"unique" json:"username"`
	Password              string        `json:"password"`
	Scope                 string        `json:"scope"`
	Locale                string        `json:"locale"`
	LockPassword          bool          `json:"lockPassword"`
	ViewMode              ViewMode      `json:"viewMode"`
	SingleClick           bool          `json:"singleClick"`
	RedirectAfterCopyMove bool          `json:"redirectAfterCopyMove"`
	Perm                  Permissions   `json:"perm"`
	Sorting               files.Sorting `json:"sorting"`
	Fs                    afero.Fs      `json:"-" yaml:"-"`
	Rules                 []rules.Rule  `json:"rules"`
	HideDotfiles          bool          `json:"hideDotfiles"`
	DateFormat            bool          `json:"dateFormat"`
	AceEditorTheme        string        `json:"aceEditorTheme"`
	// SecurityStamp is carried by every token issued to the user, which is valid only while the
	// stamp is unchanged: a password change or "close all sessions" gives a new stamp and ends
	// the sessions issued before it. Empty until the first login after it was introduced.
	SecurityStamp string `json:"securityStamp"`
	// MustChangePassword holds every request but the user's own password change; quick setup sets
	// it for the admin whose generated password was written to the log.
	MustChangePassword bool `json:"mustChangePassword"`
}

// GetRules implements rules.Provider.
func (u *User) GetRules() []rules.Rule {
	return u.Rules
}

var checkableFields = []string{
	"Username",
	"Password",
	"Scope",
	"ViewMode",
	"Sorting",
	"Rules",
}

// Clean cleans up a user and verifies if all its fields
// are alright to be saved.
func (u *User) Clean(baseScope string, followExternalSymlinks bool, fields ...string) error {
	if len(fields) == 0 {
		fields = checkableFields
	}

	for _, field := range fields {
		switch field {
		case "Username":
			if u.Username == "" {
				return fberrors.ErrEmptyUsername
			}
		case "Password":
			if u.Password == "" {
				return fberrors.ErrEmptyPassword
			}
		case "ViewMode":
			if u.ViewMode == "" {
				u.ViewMode = ListViewMode
			}
		case "Sorting":
			if u.Sorting.By == "" {
				u.Sorting.By = "name"
			}
		case "Rules":
			if u.Rules == nil {
				u.Rules = []rules.Rule{}
			}
		}
	}

	if u.Fs == nil {
		scope := u.Scope
		scope = filepath.Join(baseScope, filepath.Join("/", scope))
		u.Fs = files.NewFs(afero.NewOsFs(), scope, followExternalSymlinks)
	}

	return nil
}

// FullPath gets the full path for a user's relative path.
func (u *User) FullPath(path string) string {
	return afero.FullBaseFsPath(files.BasePath(u.Fs), path)
}
