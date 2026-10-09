package fbhttp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/filebrowser/filebrowser/v2/rules"
	"github.com/filebrowser/filebrowser/v2/users"
)

// favorites returns the paths of the user's favourites as the page lists them.
func (e *fileEnv) favorites() []string {
	e.t.Helper()
	rec := e.call(http.MethodGet, "/api/favorites", "")
	if rec.Code != http.StatusOK {
		e.t.Fatalf("favourites = %d %q", rec.Code, rec.Body.String())
	}
	var list []favorite
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		e.t.Fatal(err)
	}
	paths := []string{}
	for _, f := range list {
		paths = append(paths, f.Path)
	}
	return paths
}

func (e *fileEnv) favorite(method, p string) *http.Response {
	return e.call(method, "/api/favorites", fmt.Sprintf(`{"path":%q}`, p)).Result()
}

// K87: a user marks files and folders as favourites, one at a time, at most 20; the list holds
// only what they can reach.
func TestFavorites(t *testing.T) {
	env := newFileEnv(t)
	env.write("filmler/a.mkv", "film", 0o644)
	env.write("notlar.txt", "not", 0o644)

	for _, p := range []string{"/filmler", "notlar.txt", "/filmler/"} {
		if res := env.favorite(http.MethodPost, p); res.StatusCode != http.StatusOK {
			t.Fatalf("adding %s = %d", p, res.StatusCode)
		}
	}
	if got := env.favorites(); !slices.Equal(got, []string{"/filmler", "/notlar.txt"}) {
		t.Errorf("favourites %v; want /filmler and /notlar.txt once each", got)
	}

	var list []favorite
	_ = json.Unmarshal(env.call(http.MethodGet, "/api/favorites", "").Body.Bytes(), &list)
	if len(list) != 2 || !list[0].IsDir || list[0].Name != "filmler" || list[1].IsDir || list[1].Size != 3 {
		t.Errorf("listed favourites %+v", list)
	}

	for p, want := range map[string]int{"/": http.StatusBadRequest, "/yok": http.StatusNotFound,
		"/" + strings.Repeat("a", maxFavoritePath): http.StatusBadRequest} {
		if res := env.favorite(http.MethodPost, p); res.StatusCode != want {
			t.Errorf("adding %.20s = %d; want %d", p, res.StatusCode, want)
		}
	}

	for i := 0; i < maxFavorites-2; i++ {
		name := fmt.Sprintf("dosya%02d.txt", i)
		env.write(name, "x", 0o644)
		if res := env.favorite(http.MethodPost, "/"+name); res.StatusCode != http.StatusOK {
			t.Fatalf("favourite %d = %d", i+3, res.StatusCode)
		}
	}
	env.write("fazla.txt", "x", 0o644)
	if rec := env.call(http.MethodPost, "/api/favorites", `{"path":"/fazla.txt"}`); rec.Code != http.StatusBadRequest ||
		!strings.Contains(rec.Body.String(), "at most 20 favourites") {
		t.Errorf("a 21st favourite = %d %q; want 400", rec.Code, rec.Body.String())
	}

	if res := env.favorite(http.MethodDelete, "/notlar.txt"); res.StatusCode != http.StatusOK {
		t.Fatalf("removing = %d", res.StatusCode)
	}
	if got := env.favorites(); slices.Contains(got, "/notlar.txt") || len(got) != maxFavorites-1 {
		t.Errorf("after removing /notlar.txt: %v", got)
	}

	// An item gone from under Gezgin, or refused by a rule since, is left out and dropped.
	if err := os.Remove(filepath.Join(env.root, "dosya00.txt")); err != nil {
		t.Fatal(err)
	}
	if got := env.favorites(); slices.Contains(got, "/dosya00.txt") {
		t.Errorf("a removed file is still listed: %v", got)
	}
	stored, err := env.st.Users.Get("", false, "root")
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(stored.Favorites, "/dosya00.txt") {
		t.Errorf("a removed file is still stored: %v", stored.Favorites)
	}
}

// K88: a favourite follows its item through a rename or move made in Gezgin, whoever makes it,
// and goes with a delete or when the item leaves its owner's reach.
func TestFavoritesFollowTheirItems(t *testing.T) {
	env := newFileEnv(t)
	ali, aliUser := env.user("ali", "ali-password-1", "/ali", users.Permissions{Create: true, Rename: true, Delete: true})
	env.write("filmler/a.mkv", "film", 0o644)
	env.write("ali/x.txt", "x", 0o644)
	for _, p := range []string{"/filmler", "/filmler/a.mkv"} {
		env.favorite(http.MethodPost, p)
	}
	ali.favorite(http.MethodPost, "/x.txt")

	if code := env.send(http.MethodPatch, "/api/resources/filmler?action=rename&destination=%2Fsinema&override=false", nil); code != http.StatusOK {
		t.Fatalf("rename = %d", code)
	}
	if got := env.favorites(); !slices.Equal(got, []string{"/sinema", "/sinema/a.mkv"}) {
		t.Errorf("after the rename: %v; want /sinema and /sinema/a.mkv", got)
	}

	// The admin renames ali's file: ali's favourite follows, in ali's scope.
	if code := env.send(http.MethodPatch, "/api/resources/ali/x.txt?action=rename&destination=%2Fali%2Fy.txt&override=false", nil); code != http.StatusOK {
		t.Fatalf("rename in ali's folder = %d", code)
	}
	if got := ali.favorites(); !slices.Equal(got, []string{"/y.txt"}) {
		t.Errorf("ali's favourites after the admin's rename: %v; want /y.txt", got)
	}
	// Moved out of ali's scope, it goes.
	if code := env.send(http.MethodPatch, "/api/resources/ali/y.txt?action=rename&destination=%2Fy.txt&override=false", nil); code != http.StatusOK {
		t.Fatalf("move out of ali's folder = %d", code)
	}
	if got := ali.favorites(); len(got) != 0 {
		t.Errorf("ali's favourites after the file left their scope: %v", got)
	}
	if stored, _ := env.st.Users.Get("", false, aliUser.ID); len(stored.Favorites) != 0 {
		t.Errorf("ali's stored favourites: %v", stored.Favorites)
	}

	// A delete, into the trash, takes the favourites of the folder and of what is inside it.
	if code := env.send(http.MethodDelete, "/api/resources/sinema", nil); code != http.StatusNoContent {
		t.Fatalf("delete = %d", code)
	}
	if got := env.favorites(); len(got) != 0 {
		t.Errorf("favourites after the delete: %v", got)
	}
}

// K87: a rule keeps an item out of the favourites, and an admin's edit of a user leaves their
// favourites as they are.
func TestFavoritesRulesAndUserEdits(t *testing.T) {
	env := newFileEnv(t)
	ali, aliUser := env.user("ali", "ali-password-1", "/ali", users.Permissions{Create: true})
	env.write("ali/gizli/x.txt", "x", 0o644)
	env.write("ali/acik.txt", "x", 0o644)
	aliUser.Rules = []rules.Rule{{Path: "/gizli"}}
	if err := env.st.Users.Update(aliUser, "Rules"); err != nil {
		t.Fatal(err)
	}
	if res := ali.favorite(http.MethodPost, "/gizli"); res.StatusCode != http.StatusForbidden {
		t.Errorf("a refused folder as a favourite = %d; want 403", res.StatusCode)
	}
	if res := ali.favorite(http.MethodPost, "/acik.txt"); res.StatusCode != http.StatusOK {
		t.Fatalf("adding = %d", res.StatusCode)
	}

	form := fmt.Sprintf(`{"id":%d,"username":"ali","scope":"/ali","locale":"tr","perm":{"create":true},"favorites":[]}`, aliUser.ID)
	body := fmt.Sprintf(`{"what":"user","which":["all"],"current_password":"root-password-1","data":%s}`, form)
	if rec := env.call(http.MethodPut, fmt.Sprintf("/api/users/%d", aliUser.ID), body); rec.Code != http.StatusOK {
		t.Fatalf("admin edit = %d %q", rec.Code, rec.Body.String())
	}
	if got := ali.favorites(); !slices.Equal(got, []string{"/acik.txt"}) {
		t.Errorf("ali's favourites after the admin's edit: %v", got)
	}
}
