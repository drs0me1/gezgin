package users

import (
	"errors"
	"slices"
	"strings"
	"sync"
	"time"

	fberrors "github.com/filebrowser/filebrowser/v2/errors"
	"github.com/filebrowser/filebrowser/v2/rules"
)

// StorageBackend is the interface to implement for a users storage.
type StorageBackend interface {
	GetBy(interface{}) (*User, error)
	Gets() ([]*User, error)
	Save(u *User) error
	Update(u *User, fields ...string) error
	DeleteByID(uint) error
	DeleteByUsername(string) error
	CountAdmins() (int, error)
}

type Store interface {
	Get(baseScope string, followExternalSymlinks bool, id interface{}) (user *User, err error)
	Gets(baseScope string, followExternalSymlinks bool) ([]*User, error)
	Update(user *User, fields ...string) error
	Save(user *User) error
	Delete(id interface{}) error
	LastUpdate(id uint) int64
}

// Storage is a users storage.
type Storage struct {
	back    StorageBackend
	updated map[uint]int64
	mux     sync.RWMutex

	// guard serializes the checks that look at other users (a free username,
	// the last admin) with the write they allow.
	guard sync.Mutex
}

// NewStorage creates a users storage from a backend.
func NewStorage(back StorageBackend) *Storage {
	return &Storage{
		back:    back,
		updated: map[uint]int64{},
	}
}

// Get allows you to get a user by its name or username. The provided
// id must be a string for username lookup or a uint for id lookup. If id
// is neither, a ErrInvalidDataType will be returned.
func (s *Storage) Get(baseScope string, followExternalSymlinks bool, id interface{}) (user *User, err error) {
	user, err = s.back.GetBy(id)
	if err != nil {
		return
	}
	if err := user.Clean(baseScope, followExternalSymlinks); err != nil {
		return nil, err
	}
	return
}

// Gets gets a list of all users.
func (s *Storage) Gets(baseScope string, followExternalSymlinks bool) ([]*User, error) {
	users, err := s.back.Gets()
	if err != nil {
		return nil, err
	}

	for _, user := range users {
		if err := user.Clean(baseScope, followExternalSymlinks); err != nil {
			return nil, err
		}
	}

	return users, err
}

// Update updates a user in the database. Without fields every field is
// written.
func (s *Storage) Update(user *User, fields ...string) error {
	err := user.Clean("", false, fields...)
	if err != nil {
		return err
	}

	writes := func(field string) bool {
		return len(fields) == 0 || slices.Contains(fields, field)
	}

	if writes("Rules") {
		if err = rules.Validate(user.Rules); err != nil {
			return err
		}
	}

	s.guard.Lock()
	defer s.guard.Unlock()

	if writes("Username") {
		if err = s.nameTaken(user); err != nil {
			return err
		}
	}

	if writes("Perm") && !user.Perm.Admin {
		stored, err := s.back.GetBy(user.ID)
		if err != nil {
			return err
		}
		if stored.Perm.Admin && s.IsUniqueAdmin(stored) {
			return fberrors.ErrLastAdmin
		}
	}

	err = s.back.Update(user, fields...)
	if err != nil {
		return err
	}

	s.mux.Lock()
	s.updated[user.ID] = time.Now().Unix()
	s.mux.Unlock()
	return nil
}

// Save saves the user in a storage.
func (s *Storage) Save(user *User) error {
	if err := user.Clean("", false); err != nil {
		return err
	}

	if err := rules.Validate(user.Rules); err != nil {
		return err
	}

	s.guard.Lock()
	defer s.guard.Unlock()

	if err := s.nameTaken(user); err != nil {
		return err
	}

	return s.back.Save(user)
}

// nameTaken reports, as ErrExist, a username another user already has in any
// case: logins match names exactly, so names that differ only in case would
// be easy to confuse.
func (s *Storage) nameTaken(user *User) error {
	all, err := s.back.Gets()
	if errors.Is(err, fberrors.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	for _, other := range all {
		if other.ID != user.ID && strings.EqualFold(other.Username, user.Username) {
			return fberrors.ErrExist
		}
	}
	return nil
}

// Delete allows you to delete a user by its name or username. The provided
// id must be a string for username lookup or a uint for id lookup. If id
// is neither, a ErrInvalidDataType will be returned.
func (s *Storage) Delete(id interface{}) error {
	s.guard.Lock()
	defer s.guard.Unlock()

	switch id := id.(type) {
	case string:
		user, err := s.back.GetBy(id)
		if err != nil {
			return err
		}
		if s.IsUniqueAdmin(user) {
			return fberrors.ErrRootUserDeletion
		}

		return s.back.DeleteByUsername(id)
	case uint:
		user, err := s.back.GetBy(id)
		if err != nil {
			return err
		}
		if s.IsUniqueAdmin(user) {
			return fberrors.ErrRootUserDeletion
		}

		return s.back.DeleteByID(id)
	default:
		return fberrors.ErrInvalidDataType
	}
}

// LastUpdate gets the timestamp for the last update of an user.
func (s *Storage) LastUpdate(id uint) int64 {
	s.mux.RLock()
	defer s.mux.RUnlock()
	if val, ok := s.updated[id]; ok {
		return val
	}
	return 0
}

func (s *Storage) IsUniqueAdmin(user *User) bool {
	if !user.Perm.Admin {
		return false
	}

	count, err := s.back.CountAdmins()
	if err != nil {
		return true
	}
	return count <= 1
}
