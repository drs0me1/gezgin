package fbhttp

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/filebrowser/filebrowser/v2/share"
	"github.com/filebrowser/filebrowser/v2/trash"
	"github.com/filebrowser/filebrowser/v2/users"
)

func (e *fileEnv) as(token string) *fileEnv {
	return &fileEnv{sessionEnv: e.sessionEnv, token: token}
}

func (e *fileEnv) call(method, target, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("X-Auth", e.token)
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	return rec
}

func (e *fileEnv) trash() []trash.Item {
	e.t.Helper()
	rec := e.call(http.MethodGet, "/api/trash", "")
	var list struct {
		Items []trash.Item `json:"items"`
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &list) != nil {
		e.t.Fatalf("trash list = %d %q", rec.Code, rec.Body.String())
	}
	return list.Items
}

func (e *fileEnv) addUser(name string, perm users.Permissions) string {
	e.t.Helper()
	hash, _ := users.HashPwd(name + "-password-1")
	if err := e.st.Users.Save(&users.User{Username: name, Password: hash, Scope: ".", Perm: perm}); err != nil {
		e.t.Fatal(err)
	}
	return e.login(name, name+"-password-1")
}

func ids(items ...trash.Item) string {
	var list []string
	for _, item := range items {
		list = append(list, `"`+item.ID+`"`)
	}
	return `{"ids":[` + strings.Join(list, ",") + `]}`
}

func TestDeleteMovesToTheTrashAndRestoreBringsBack(t *testing.T) {
	env := newFileEnv(t)
	env.write("docs/a.txt", "a", 0o644)
	env.write("docs/sub/b.txt", "b", 0o644)
	link := &share.Link{Hash: "h1", Path: "/docs/a.txt", UserID: 1}
	if err := env.st.Share.Save(link); err != nil {
		t.Fatal(err)
	}

	if code := env.send(http.MethodDelete, "/api/resources/docs/a.txt", nil); code != http.StatusNoContent {
		t.Fatalf("delete = %d", code)
	}
	if code := env.send(http.MethodDelete, "/api/resources/docs/sub/", nil); code != http.StatusNoContent {
		t.Fatalf("delete folder = %d", code)
	}
	if env.read("docs/a.txt") != "<missing>" {
		t.Errorf("the deleted file is still in place")
	}
	if _, err := env.st.Share.GetByHash("h1"); err == nil {
		t.Errorf("the deleted file's share is still there")
	}

	items := env.trash()
	if len(items) != 2 || items[0].Name != "sub" || items[0].Origin != "/docs" || !items[0].IsDir || items[1].Name != "a.txt" || items[1].Origin != "/docs" {
		t.Fatalf("trash = %+v", items)
	}

	rec := env.call(http.MethodPost, "/api/trash/restore", ids(items...))
	if rec.Code != http.StatusOK {
		t.Fatalf("restore = %d %q", rec.Code, rec.Body.String())
	}
	if env.read("docs/a.txt") != "a" || env.read("docs/sub/b.txt") != "b" || len(env.trash()) != 0 {
		t.Errorf("after the restore: %q %q, %d in the trash", env.read("docs/a.txt"), env.read("docs/sub/b.txt"), len(env.trash()))
	}
	if _, err := env.st.Share.GetByHash("h1"); err == nil {
		t.Errorf("the share came back with the restore")
	}
}

func TestRestoreFindsAPlace(t *testing.T) {
	env := newFileEnv(t)
	env.write("gone/x.txt", "x", 0o644)
	env.write("here/y.txt", "old", 0o644)

	env.send(http.MethodDelete, "/api/resources/gone/x.txt", nil)
	env.send(http.MethodDelete, "/api/resources/here/y.txt", nil)
	if err := os.RemoveAll(filepath.Join(env.root, "gone")); err != nil {
		t.Fatal(err)
	}
	env.write("here/y.txt", "new", 0o644)

	if rec := env.call(http.MethodPost, "/api/trash/restore", ids(env.trash()...)); rec.Code != http.StatusOK {
		t.Fatalf("restore = %d %q", rec.Code, rec.Body.String())
	}
	if env.read("x.txt") != "x" {
		t.Errorf("an item whose folder is gone was not restored into the root")
	}
	if env.read("here/y.txt") != "new" || env.read("here/y(1).txt") != "old" {
		t.Errorf("a taken name: y.txt %q, y(1).txt %q", env.read("here/y.txt"), env.read("here/y(1).txt"))
	}
}

func TestTheTrashFolderIsUnreachable(t *testing.T) {
	env := newFileEnv(t)
	env.write("a.txt", "a", 0o644)
	env.send(http.MethodDelete, "/api/resources/a.txt", nil)
	item := env.trash()[0]
	held := "/" + trash.Dir + "/1/" + item.ID + "/a.txt"

	rec := env.call(http.MethodGet, "/api/resources/", "")
	if strings.Contains(rec.Body.String(), trash.Dir) {
		t.Errorf("the root listing shows the trash folder: %s", rec.Body.String())
	}
	for _, c := range []struct{ method, target string }{
		{http.MethodGet, "/api/resources/" + trash.Dir + "/"},
		{http.MethodGet, "/api/resources" + held},
		{http.MethodGet, "/api/raw" + held},
		{http.MethodPost, "/api/resources/" + trash.Dir + "/new.txt"},
		{http.MethodDelete, "/api/resources/" + trash.Dir + "/"},
		{http.MethodPatch, "/api/resources" + held + "?action=copy&destination=%2Fstolen.txt"},
		{http.MethodPatch, "/api/resources/" + "x?action=rename&destination=%2F" + trash.Dir + "%2Fx"},
	} {
		if rec := env.call(c.method, c.target, ""); rec.Code < 400 {
			t.Errorf("%s %s = %d; want it refused", c.method, c.target, rec.Code)
		}
	}
	// An archive skips refused paths, as for any rule: it comes back without the trash.
	for _, target := range []string{"/api/raw/?files=" + trash.Dir + "&algo=zip", "/api/raw/?algo=zip"} {
		rec := env.call(http.MethodGet, target, "")
		zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
		if err != nil {
			t.Fatalf("%s: %v", target, err)
		}
		for _, f := range zr.File {
			if strings.Contains(f.Name, trash.Dir) || strings.Contains(f.Name, "a.txt") {
				t.Errorf("%s holds %s", target, f.Name)
			}
		}
	}
	if env.read("stolen.txt") != "<missing>" {
		t.Errorf("a trashed file was copied out")
	}
	if rec := env.call(http.MethodGet, "/api/resources/recursive/", ""); strings.Contains(rec.Body.String(), trash.Dir) {
		t.Errorf("the recursive listing shows the trash")
	}

	form := `{"what":"user","which":[],"current_password":"root-password-1","data":{"username":"spy","password":"spy-password-12","scope":"/` + trash.Dir + `/1","perm":{}}}`
	if rec := env.call(http.MethodPost, "/api/users", form); rec.Code != http.StatusBadRequest {
		t.Errorf("a user scoped to the trash = %d; want 400", rec.Code)
	}
}

func TestPermanentDeletePurgeAndEmpty(t *testing.T) {
	env := newFileEnv(t)
	for _, name := range []string{"p.txt", "q.txt", "r.txt", "s.txt"} {
		env.write(name, name, 0o644)
	}

	env.send(http.MethodDelete, "/api/resources/p.txt?permanent=true", nil)
	if env.read("p.txt") != "<missing>" || len(env.trash()) != 0 {
		t.Errorf("a permanent delete went to the trash")
	}

	env.send(http.MethodDelete, "/api/resources/q.txt", nil)
	env.send(http.MethodDelete, "/api/resources/r.txt", nil)
	items := env.trash()
	if rec := env.call(http.MethodPost, "/api/trash/purge", ids(items[0])); rec.Code != http.StatusNoContent || len(env.trash()) != 1 {
		t.Errorf("purge = %d, %d left", rec.Code, len(env.trash()))
	}
	for body, want := range map[string]int{`{"ids":["../../x"]}`: 400, `{"ids":[]}`: 400, `{"ids":["0123456789abcdef0123456789abcdef"]}`: 404} {
		if rec := env.call(http.MethodPost, "/api/trash/purge", body); rec.Code != want {
			t.Errorf("purge %s = %d; want %d", body, rec.Code, want)
		}
	}

	env.send(http.MethodDelete, "/api/resources/s.txt", nil)
	if rec := env.call(http.MethodDelete, "/api/trash", ""); rec.Code != http.StatusNoContent || len(env.trash()) != 0 {
		t.Errorf("empty = %d, %d left", rec.Code, len(env.trash()))
	}
}

func TestTrashBelongsToItsUser(t *testing.T) {
	env := newFileEnv(t)
	alice := env.as(env.addUser("alice", users.Permissions{Create: true, Delete: true, Download: true}))
	viewer := env.as(env.addUser("viewer", users.Permissions{Download: true}))
	env.write("mine.txt", "root's", 0o644)
	env.write("hers.txt", "alice's", 0o644)

	env.send(http.MethodDelete, "/api/resources/mine.txt", nil)
	alice.send(http.MethodDelete, "/api/resources/hers.txt", nil)
	rootItems, aliceItems := env.trash(), alice.trash()
	if len(rootItems) != 1 || rootItems[0].Name != "mine.txt" || len(aliceItems) != 1 || aliceItems[0].Name != "hers.txt" {
		t.Fatalf("bins: root %+v, alice %+v", rootItems, aliceItems)
	}
	if rec := alice.call(http.MethodPost, "/api/trash/restore", ids(rootItems...)); rec.Code != http.StatusNotFound {
		t.Errorf("alice restoring root's item = %d; want 404", rec.Code)
	}

	if code := viewer.send(http.MethodDelete, "/api/resources/hers.txt", nil); code != http.StatusForbidden {
		t.Errorf("a user without delete permission deleting = %d; want 403", code)
	}
	if rec := viewer.call(http.MethodDelete, "/api/trash", ""); rec.Code != http.StatusForbidden {
		t.Errorf("a user without delete permission emptying = %d; want 403", rec.Code)
	}
	if rec := viewer.call(http.MethodPost, "/api/trash/restore", `{"ids":["0123456789abcdef0123456789abcdef"]}`); rec.Code != http.StatusForbidden {
		t.Errorf("a user without create permission restoring = %d; want 403", rec.Code)
	}

	if rec := alice.call(http.MethodGet, "/api/trash/all", ""); rec.Code != http.StatusForbidden {
		t.Errorf("a user asking for every bin = %d; want 403", rec.Code)
	}
	rec := env.call(http.MethodGet, "/api/trash/all", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"count":2`) || strings.Contains(rec.Body.String(), "hers") {
		t.Errorf("admin usage = %d %q; want the count, no names", rec.Code, rec.Body.String())
	}
	if rec := env.call(http.MethodDelete, "/api/trash/all", ""); rec.Code != http.StatusNoContent || len(alice.trash()) != 0 {
		t.Errorf("admin emptying every bin = %d, alice has %d", rec.Code, len(alice.trash()))
	}
}

func TestTrashDaysSetting(t *testing.T) {
	env := newFileEnv(t)
	get := func() uint {
		var set struct {
			TrashDays uint `json:"trashDays"`
		}
		rec := env.call(http.MethodGet, "/api/settings", "")
		if err := json.Unmarshal(rec.Body.Bytes(), &set); err != nil {
			t.Fatal(err)
		}
		return set.TrashDays
	}
	if days := get(); days != 30 {
		t.Errorf("default trash days %d; want 30", days)
	}
	for value, want := range map[uint]int{7: 200, 0: 200, 3651: 400} {
		body := fmt.Sprintf(`{"minimumPasswordLength":8,"trashDays":%d}`, value)
		if rec := env.call(http.MethodPut, "/api/settings", body); rec.Code != want {
			t.Errorf("trash days %d = %d; want %d", value, rec.Code, want)
		}
		if want == 200 && get() != value {
			t.Errorf("trash days after saving %d: %d", value, get())
		}
	}
}

// The trash page shows its items in a folder's view (Gezgin): a file with its type by its name, a
// folder with how many items it holds.
func TestTrashListTypesAndCounts(t *testing.T) {
	env := newFileEnv(t)
	env.write("tatil/a.jpg", "jpeg", 0o644)
	env.write("tatil/b.jpg", "jpeg", 0o644)
	env.write("film.mkv", "film", 0o644)
	for _, p := range []string{"/api/resources/tatil", "/api/resources/film.mkv"} {
		if code := env.send(http.MethodDelete, p, nil); code != http.StatusNoContent {
			t.Fatalf("delete %s = %d", p, code)
		}
	}
	var list struct {
		Items []struct {
			Name  string `json:"name"`
			Type  string `json:"type"`
			Count *int   `json:"count"`
			Size  int64  `json:"size"`
		} `json:"items"`
	}
	if err := json.Unmarshal(env.call(http.MethodGet, "/api/trash", "").Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	for _, item := range list.Items {
		switch item.Name {
		case "tatil":
			if item.Count == nil || *item.Count != 2 || item.Size != 8 {
				t.Errorf("tatil: %+v; want 2 items, 8 bytes", item)
			}
		case "film.mkv":
			if item.Type != "video" || item.Count != nil {
				t.Errorf("film.mkv: %+v; want a video", item)
			}
		default:
			t.Errorf("unexpected %s", item.Name)
		}
	}
}
