package fbhttp

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/filebrowser/filebrowser/v2/users"
)

func (e *fileEnv) patchShare(hash, body string) (int, shareResponse) {
	e.t.Helper()
	rec := e.call(http.MethodPatch, "/api/share/"+hash, body)
	var link shareResponse
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &link); err != nil {
			e.t.Fatal(err)
		}
	}
	return rec.Code, link
}

// tokenOf opens a protected link with its password and returns the token its downloads take.
func (e *fileEnv) tokenOf(hash, password string) string {
	e.t.Helper()
	rec := e.callWith(http.MethodGet, "/api/public/share/"+hash, map[string]string{"X-SHARE-PASSWORD": password}, "")
	var info struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &info); err != nil || info.Token == "" {
		e.t.Fatalf("no token for %s: %d %q", hash, rec.Code, rec.Body.String())
	}
	return info.Token
}

// K96: a link is changed where it is: its duration counted from now, or permanent, its password
// set, replaced or removed; the address stays.
func TestShareEditLink(t *testing.T) {
	env := newShareEnv(t)
	env.write("docs/rapor.pdf", "pdf", 0o644)
	hash := env.mustShare("/docs/rapor.pdf", `{"expires":"1","unit":"hours"}`)

	code, link := env.patchShare(hash, `{"expires":"3","unit":"days"}`)
	want := time.Now().Add(72 * time.Hour).Unix()
	if code != http.StatusOK || link.Hash != hash || link.Expire < want-5 || link.Expire > want+5 {
		t.Fatalf("3 days = %d %+v; want the same address ending in 3 days", code, link)
	}
	if code, link = env.patchShare(hash, `{"expires":"0"}`); code != http.StatusOK || link.Expire != 0 {
		t.Fatalf("permanent = %d %+v", code, link)
	}
	if code, link = env.patchShare(hash, `{"passwordAction":"set","password":"ilk-sifre"}`); code != http.StatusOK ||
		!link.HasPassword || link.Expire != 0 {
		t.Fatalf("a password = %d %+v; want it protected, still permanent", code, link)
	}
	old := env.tokenOf(hash, "ilk-sifre")
	if code, _ = env.patchShare(hash, `{"passwordAction":"set","password":"ikinci-sifre"}`); code != http.StatusOK {
		t.Fatalf("a new password = %d", code)
	}
	if rec := env.call(http.MethodGet, "/api/public/dl/"+hash+"?token="+url.QueryEscape(old), ""); rec.Code == http.StatusOK {
		t.Errorf("the old password's token still downloads")
	}
	if fresh := env.tokenOf(hash, "ikinci-sifre"); fresh == old {
		t.Errorf("the token did not change with the password")
	}
	if code, link = env.patchShare(hash, `{"passwordAction":"remove"}`); code != http.StatusOK || link.HasPassword {
		t.Fatalf("no password = %d %+v", code, link)
	}
	if code, body := env.download(hash); code != http.StatusOK || body != "pdf" {
		t.Errorf("the open link = %d %q", code, body)
	}

	for body, want := range map[string]int{
		`{"expires":"3651","unit":"days"}`:       http.StatusBadRequest,
		`{"expires":"1.5","unit":"days"}`:        http.StatusBadRequest,
		`{"passwordAction":"set","password":""}`: http.StatusBadRequest,
		`{"passwordAction":"rename"}`:            http.StatusBadRequest,
		`{"writable":true}`:                      http.StatusBadRequest,
	} {
		if code, _ := env.patchShare(hash, body); code != want {
			t.Errorf("%s = %d; want %d", body, code, want)
		}
	}
	if code, _ := env.patchShare("yok", `{"expires":"0"}`); code != http.StatusNotFound {
		t.Errorf("an unknown share = %d; want 404", code)
	}
}

// K96: a WebDAV share keeps a password, held to the account rules, and becomes read-write only for
// an owner who may create, change, rename and delete.
func TestShareEditWebDAV(t *testing.T) {
	env, _ := newDavEnv(t)
	env.write("ortak/a.txt", "a", 0o644)
	hash := env.davShare("/ortak", false)

	if code, _ := env.patchShare(hash, `{"passwordAction":"remove"}`); code != http.StatusBadRequest {
		t.Errorf("removing a WebDAV password = %d; want 400", code)
	}
	if code, _ := env.patchShare(hash, `{"passwordAction":"set","password":"kisa"}`); code != http.StatusBadRequest {
		t.Errorf("a short WebDAV password = %d; want 400", code)
	}
	if code, link := env.patchShare(hash, `{"passwordAction":"set","password":"yeni-webdav-sifresi","writable":true}`); code != http.StatusOK ||
		!link.Writable || link.Kind != "webdav" {
		t.Fatalf("read-write with a new password = %d %+v", code, link)
	}

	ali, _ := env.user("ali", "ali-password-1", "/ali", users.Permissions{Share: true, Download: true})
	env.write("ali/x/b.txt", "b", 0o644)
	aliHash := ali.davShare("/x", false)
	if code, _ := ali.patchShare(aliHash, `{"writable":true}`); code != http.StatusForbidden {
		t.Errorf("read-write by a user who may not write = %d; want 403", code)
	}
	if code, _ := env.patchShare(aliHash, `{"writable":true}`); code != http.StatusForbidden {
		t.Errorf("an admin making ali's share read-write, ali not allowed to write = %d; want 403", code)
	}
}

// K97: an admin changes everyone's shares, a user their own; the list tells the page what each is.
func TestShareEditOwnersAndList(t *testing.T) {
	env := newShareEnv(t)
	ali, _ := env.user("ali", "ali-password-1", "/ali", users.Permissions{Share: true, Download: true})
	veli, _ := env.user("veli", "veli-password-1", "/veli", users.Permissions{Share: true, Download: true})
	env.write("ali/tatil/a.jpg", "jpeg", 0o644)
	env.write("ali/film.mkv", "film", 0o644)
	folder := ali.mustShare("/tatil", "{}")
	ali.mustShare("/film.mkv", "{}")

	if code, _ := veli.patchShare(folder, `{"expires":"0"}`); code != http.StatusForbidden {
		t.Errorf("another user's share = %d; want 403", code)
	}
	if code, _ := env.patchShare(folder, `{"expires":"2","unit":"days"}`); code != http.StatusOK {
		t.Errorf("an admin = %d; want 200", code)
	}

	var list []shareListItem
	if err := json.Unmarshal(env.call(http.MethodGet, "/api/shares", "").Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	found := map[string]shareListItem{}
	for _, item := range list {
		found[item.Name] = item
	}
	if f := found["tatil"]; !f.IsDir || f.Owner != "ali" || f.Open != "/ali/tatil" || f.Folder != "/ali" {
		t.Errorf("tatil for the admin: %+v", f)
	}
	if f := found["film.mkv"]; f.IsDir || f.Type != "video" || f.Open != "/ali/film.mkv" {
		t.Errorf("film.mkv for the admin: %+v", f)
	}

	list = nil
	if err := json.Unmarshal(ali.call(http.MethodGet, "/api/shares", "").Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	for _, item := range list {
		if item.Owner != "" || item.Open == "" || item.Folder != "/" {
			t.Errorf("ali's own share: %+v; want no owner, opened at /%s", item, item.Name)
		}
	}
}
