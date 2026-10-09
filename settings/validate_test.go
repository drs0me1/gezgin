package settings

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	fberrors "github.com/filebrowser/filebrowser/v2/errors"
	"github.com/filebrowser/filebrowser/v2/users"
)

// validSettings are settings every check passes.
func validSettings() *Settings {
	s := &Settings{
		Key:              []byte("key"),
		UserHomeBasePath: DefaultUsersHomeBasePath,
		Defaults:         UserDefaults{Scope: ".", Locale: "tr", ViewMode: users.MosaicViewMode},
	}
	s.normalize()
	return s
}

func TestValidateBounds(t *testing.T) {
	if err := validSettings().Validate(); err != nil {
		t.Fatalf("valid settings: %v", err)
	}

	days := func(n uint) *uint { return &n }
	cases := []struct {
		name   string
		change func(*Settings)
		ok     bool
	}{
		// K66: the minimum password length is 8-32.
		{"password length 7", func(s *Settings) { s.MinimumPasswordLength = 7 }, false},
		{"password length 8", func(s *Settings) { s.MinimumPasswordLength = 8 }, true},
		{"password length 32", func(s *Settings) { s.MinimumPasswordLength = 32 }, true},
		{"password length 33", func(s *Settings) { s.MinimumPasswordLength = 33 }, false},
		// K67: a chunk size of 0 had the browser send empty chunks without end.
		{"chunk size 0", func(s *Settings) { s.Tus.ChunkSize = 0 }, false},
		{"chunk size under 1 MiB", func(s *Settings) { s.Tus.ChunkSize = MinTusChunkSize - 1 }, false},
		{"chunk size 1 MiB", func(s *Settings) { s.Tus.ChunkSize = MinTusChunkSize }, true},
		{"chunk size 1 GiB", func(s *Settings) { s.Tus.ChunkSize = MaxTusChunkSize }, true},
		{"chunk size over 1 GiB", func(s *Settings) { s.Tus.ChunkSize = MaxTusChunkSize + 1 }, false},
		{"retries 0", func(s *Settings) { s.Tus.RetryCount = 0 }, true},
		{"retries 20", func(s *Settings) { s.Tus.RetryCount = 20 }, true},
		{"retries 21", func(s *Settings) { s.Tus.RetryCount = 21 }, false},
		{"trash days 3650", func(s *Settings) { s.TrashDays = days(3650) }, true},
		{"trash days 3651", func(s *Settings) { s.TrashDays = days(3651) }, false},
		// K68: Gezgin's folders are nobody's scope.
		{"home base in the uploads", func(s *Settings) { s.UserHomeBasePath = "/.gezgin-yukleme" }, false},
		{"home base in the archives", func(s *Settings) { s.UserHomeBasePath = ".Gezgin-Arsiv/kullanicilar" }, false},
		{"default scope in the trash", func(s *Settings) { s.Defaults.Scope = "/.gezgin-cop" }, false},
		{"default scope elsewhere", func(s *Settings) { s.Defaults.Scope = "/ortak" }, true},
		// K69: the language and view mode are known values.
		{"language en", func(s *Settings) { s.Defaults.Locale = "en" }, true},
		{"language fa", func(s *Settings) { s.Defaults.Locale = "fa" }, false},
		{"language xx", func(s *Settings) { s.Defaults.Locale = "xx" }, false},
		{"view mode gallery", func(s *Settings) { s.Defaults.ViewMode = users.MosaicGalleryViewMode }, true},
		{"view mode zz", func(s *Settings) { s.Defaults.ViewMode = "zz" }, false},
		{"theme dark", func(s *Settings) { s.Branding.Theme = "dark" }, true},
		{"theme bogus", func(s *Settings) { s.Branding.Theme = "bogus" }, false},
	}
	for _, c := range cases {
		s := validSettings()
		c.change(s)
		err := s.Validate()
		if c.ok && err != nil {
			t.Errorf("%s: %v; want no error", c.name, err)
		}
		if !c.ok && !errors.Is(err, fberrors.ErrInvalidRequestParams) {
			t.Errorf("%s: %v; want ErrInvalidRequestParams", c.name, err)
		}
	}
}

// Unset values take the defaults when saved, as they do when read.
func TestNormalizeFillsUnset(t *testing.T) {
	s := &Settings{}
	s.normalize()
	if s.MinimumPasswordLength != DefaultMinimumPasswordLength || s.Tus.ChunkSize != DefaultTusChunkSize ||
		s.Tus.RetryCount != DefaultTusRetryCount || s.Defaults.Locale != "tr" || s.Defaults.ViewMode != users.MosaicViewMode {
		t.Errorf("normalized empty settings: %+v", s)
	}
	if err := s.Validate(); err != nil {
		t.Errorf("normalized empty settings: %v", err)
	}
}

// K69: the defaults never grant the admin permission, and sharing comes with downloading.
func TestNormalizeDefaultPermissions(t *testing.T) {
	s := validSettings()
	s.Defaults.Perm = users.Permissions{Admin: true, Share: true}
	s.normalize()
	if s.Defaults.Perm.Admin || !s.Defaults.Perm.Download || !s.Defaults.Perm.Share {
		t.Errorf("normalized defaults %+v; want no admin, share with download", s.Defaults.Perm)
	}
}

// Old databases kept a chunk size of 0 with retries; reading them gives a working chunk size.
func TestStorageGetRepairsZeroChunkSize(t *testing.T) {
	set := validSettings()
	set.Tus = Tus{ChunkSize: 0, RetryCount: 5}
	got, err := NewStorage(memoryBackend{set: set}).Get()
	if err != nil {
		t.Fatal(err)
	}
	if got.Tus.ChunkSize != DefaultTusChunkSize || got.Tus.RetryCount != 5 {
		t.Errorf("tus read back %+v; want the default chunk size and 5 retries", got.Tus)
	}
}

// Locales must name the interface's translation files, so that a language the interface offers
// is never refused and an unknown one never accepted.
func TestLocalesMatchTheInterface(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "frontend", "src", "i18n", "*.json"))
	if err != nil || len(files) == 0 {
		t.Skipf("no translation files: %v", err)
	}
	var names []string
	for _, f := range files {
		names = append(names, strings.TrimSuffix(filepath.Base(f), ".json"))
	}
	slices.Sort(names)
	want := slices.Clone(Locales)
	slices.Sort(want)
	if !slices.Equal(names, want) {
		t.Errorf("translation files %v; Locales %v", names, want)
	}
}

type memoryBackend struct{ set *Settings }

func (m memoryBackend) Get() (*Settings, error) {
	c := *m.set
	return &c, nil
}

func (m memoryBackend) Save(*Settings) error        { return nil }
func (m memoryBackend) GetServer() (*Server, error) { return &Server{}, nil }
func (m memoryBackend) SaveServer(*Server) error    { return nil }
