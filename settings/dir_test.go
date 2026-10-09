package settings

import (
	"errors"
	"testing"

	fberrors "github.com/filebrowser/filebrowser/v2/errors"
)

// Gezgin's folders are nobody's scope (K68): a user scoped in one was created, then refused
// everything, and the uploads' and archive jobs' folders are emptied at every start.
func TestMakeUserDirRefusesGezginFolders(t *testing.T) {
	s := &Settings{UserHomeBasePath: "/users"}
	for _, scope := range []string{"/.gezgin-cop", "/.gezgin-cop/1", ".gezgin-yukleme", "/.gezgin-arsiv/ayse",
		"/.GEZGIN-ARSIV", "/users/../.gezgin-yukleme/x"} {
		if got, err := s.MakeUserDir("ayse", scope, t.TempDir()); !errors.Is(err, fberrors.ErrInvalidRequestParams) {
			t.Errorf("scope %q = %q, %v; want ErrInvalidRequestParams", scope, got, err)
		}
	}
	for scope, want := range map[string]string{"/ortak": "/ortak", ".": "/", ".gezgin-cop-degil": "/.gezgin-cop-degil"} {
		if got, err := s.MakeUserDir("ayse", scope, t.TempDir()); err != nil || got != want {
			t.Errorf("scope %q = %q, %v; want %q", scope, got, err, want)
		}
	}

	// A home derived from the username is held to the same rule.
	s = &Settings{CreateUserDir: true, UserHomeBasePath: "/.gezgin-arsiv"}
	if got, err := s.MakeUserDir("ayse", "", t.TempDir()); !errors.Is(err, fberrors.ErrInvalidRequestParams) {
		t.Errorf("home under the archive folder = %q, %v; want ErrInvalidRequestParams", got, err)
	}
	s.UserHomeBasePath = "/users"
	if got, err := s.MakeUserDir("ayse", "", t.TempDir()); err != nil || got != "/users/ayse" {
		t.Errorf("derived home = %q, %v; want /users/ayse", got, err)
	}
}
