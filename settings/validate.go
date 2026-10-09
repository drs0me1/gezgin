package settings

import (
	"fmt"
	"slices"

	fberrors "github.com/filebrowser/filebrowser/v2/errors"
	"github.com/filebrowser/filebrowser/v2/rules"
	"github.com/filebrowser/filebrowser/v2/users"
)

// The bounds of the global settings (Gezgin).
const (
	// LowestMinimumPasswordLength and HighestMinimumPasswordLength bound the minimum password
	// length. 32 letters, Turkish ones too, fit in the 72 bytes bcrypt takes.
	LowestMinimumPasswordLength  = 8
	HighestMinimumPasswordLength = 32

	MinTusChunkSize  = 1 << 20 // 1 MiB
	MaxTusChunkSize  = 1 << 30 // 1 GiB
	MaxTusRetryCount = 20
)

// Locales are the languages the interface has, named as its translation files are: Turkish and
// English only (K142).
var Locales = []string{"en", "tr"}

// Themes are the interface's themes; "" follows the system.
var Themes = []string{"", "light", "dark"}

var viewModes = []users.ViewMode{users.ListViewMode, users.MosaicViewMode, users.MosaicGalleryViewMode}

// normalize fills what is left unset and makes the defaults consistent: they never grant the
// admin permission, and sharing comes with downloading.
func (s *Settings) normalize() {
	if s.MinimumPasswordLength == 0 {
		s.MinimumPasswordLength = DefaultMinimumPasswordLength
	}
	if s.Tus == (Tus{}) {
		s.Tus = Tus{ChunkSize: DefaultTusChunkSize, RetryCount: DefaultTusRetryCount}
	}
	if s.Defaults.Locale == "" {
		s.Defaults.Locale = "tr"
	}
	if s.Defaults.ViewMode == "" {
		s.Defaults.ViewMode = users.MosaicViewMode
	}
	s.Defaults.Perm.Admin = false
	if s.Defaults.Perm.Share {
		s.Defaults.Perm.Download = true
	}
	if s.Rules == nil {
		s.Rules = []rules.Rule{}
	}
}

// Validate checks the settings an admin can change. Every error wraps
// fberrors.ErrInvalidRequestParams.
func (s *Settings) Validate() error {
	invalid := func(format string, a ...any) error {
		return fmt.Errorf("%w: "+format, append([]any{fberrors.ErrInvalidRequestParams}, a...)...)
	}

	if s.MinimumPasswordLength < LowestMinimumPasswordLength || s.MinimumPasswordLength > HighestMinimumPasswordLength {
		return invalid("the minimum password length must be from %d to %d",
			LowestMinimumPasswordLength, HighestMinimumPasswordLength)
	}
	if s.Tus.ChunkSize < MinTusChunkSize || s.Tus.ChunkSize > MaxTusChunkSize {
		return invalid("the chunk size must be from %d to %d bytes", MinTusChunkSize, MaxTusChunkSize)
	}
	if s.Tus.RetryCount > MaxTusRetryCount {
		return invalid("the retry count must be from 0 to %d", MaxTusRetryCount)
	}
	if s.TrashDays != nil && *s.TrashDays > MaxTrashDays {
		return invalid("the trash days must be from 0 to %d", MaxTrashDays)
	}
	if IsReserved(s.UserHomeBasePath, true) || IsReserved(s.Defaults.Scope, true) {
		return invalid(reservedScope)
	}
	if !slices.Contains(Locales, s.Defaults.Locale) {
		return invalid("unknown language %q", s.Defaults.Locale)
	}
	if !slices.Contains(viewModes, s.Defaults.ViewMode) {
		return invalid("unknown view mode %q", s.Defaults.ViewMode)
	}
	if !slices.Contains(Themes, s.Branding.Theme) {
		return invalid("unknown theme %q", s.Branding.Theme)
	}
	return rules.Validate(s.Rules)
}
