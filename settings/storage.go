package settings

import (
	"slices"

	fberrors "github.com/filebrowser/filebrowser/v2/errors"
)

// StorageBackend is a settings storage backend.
type StorageBackend interface {
	Get() (*Settings, error)
	Save(*Settings) error
	GetServer() (*Server, error)
	SaveServer(*Server) error
}

// Storage is a settings storage.
type Storage struct {
	back StorageBackend
}

// NewStorage creates a settings storage from a backend.
func NewStorage(back StorageBackend) *Storage {
	return &Storage{back: back}
}

// Get returns the settings for the current instance.
func (s *Storage) Get() (*Settings, error) {
	set, err := s.back.Get()
	if err != nil {
		return nil, err
	}

	if set.UserHomeBasePath == "" {
		set.UserHomeBasePath = DefaultUsersHomeBasePath
	}

	if set.MinimumPasswordLength == 0 {
		set.MinimumPasswordLength = DefaultMinimumPasswordLength
	}

	// A language Gezgin no longer has (K142) reads as English, as the interface shows it.
	if set.Defaults.Locale != "" && !slices.Contains(Locales, set.Defaults.Locale) {
		set.Defaults.Locale = "en"
	}

	if set.Tus == (Tus{}) {
		set.Tus = Tus{
			ChunkSize:  DefaultTusChunkSize,
			RetryCount: DefaultTusRetryCount,
		}
	}
	// A chunk size of 0 had the browser send empty chunks without end; no save takes it now.
	if set.Tus.ChunkSize == 0 {
		set.Tus.ChunkSize = DefaultTusChunkSize
	}

	if set.FileMode == 0 {
		set.FileMode = DefaultFileMode
	}

	if set.DirMode == 0 {
		set.DirMode = DefaultDirMode
	}

	return set, nil
}

// Save saves the settings for the current instance.
func (s *Storage) Save(set *Settings) error {
	if len(set.Key) == 0 {
		return fberrors.ErrEmptyKey
	}

	set.normalize()
	if err := set.Validate(); err != nil {
		return err
	}

	err := s.back.Save(set)
	if err != nil {
		return err
	}

	return nil
}

// GetServer wraps StorageBackend.GetServer.
func (s *Storage) GetServer() (*Server, error) {
	return s.back.GetServer()
}

// SaveServer wraps StorageBackend.SaveServer and adds some verification.
func (s *Storage) SaveServer(ser *Server) error {
	ser.Clean()
	return s.back.SaveServer(ser)
}
