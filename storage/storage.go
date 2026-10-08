package storage

import (
	"github.com/filebrowser/filebrowser/v2/auth"
	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/share"
	"github.com/filebrowser/filebrowser/v2/users"
)

// Storage is a storage powered by a Backend which makes the necessary
// verifications when fetching and saving data to ensure consistency.
type Storage struct {
	Users    users.Store
	Share    *share.Storage
	Auth     *auth.Storage
	Settings *settings.Storage
}

// DeleteUser deletes a user, named by id or username, and the share links they
// made (Gezgin): a link would outlive its owner otherwise. It returns the
// user's id, which names what else of theirs is to go, such as their trash.
func (s *Storage) DeleteUser(id interface{}) (uint, error) {
	user, err := s.Users.Get("", false, id)
	if err != nil {
		return 0, err
	}
	if err := s.Users.Delete(user.ID); err != nil {
		return 0, err
	}
	return user.ID, s.Share.DeleteByUserID(user.ID)
}
