package fbhttp

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/filebrowser/filebrowser/v2/rules"
	"github.com/filebrowser/filebrowser/v2/share"
	"github.com/filebrowser/filebrowser/v2/trash"
	"github.com/filebrowser/filebrowser/v2/users"
)

// Gezgin's share links: they follow the item, end with it and with their owner, limit password
// attempts and are checked when made.

// newShareEnv is a fileEnv whose admin may share.
func newShareEnv(t *testing.T) *fileEnv {
	t.Helper()
	env := newFileEnv(t)
	root, err := env.st.Users.Get(env.root, false, "root")
	if err != nil {
		t.Fatal(err)
	}
	root.Perm.Share = true
	if err := env.st.Users.Update(root, "Perm"); err != nil {
		t.Fatal(err)
	}
	return env
}

// user adds a user scoped to scope and returns an environment acting as them.
func (e *fileEnv) user(name, password, scope string, perm users.Permissions) (*fileEnv, *users.User) {
	e.t.Helper()
	if err := os.MkdirAll(filepath.Join(e.root, scope), 0o755); err != nil {
		e.t.Fatal(err)
	}
	hash, err := users.HashPwd(password)
	if err != nil {
		e.t.Fatal(err)
	}
	u := &users.User{Username: name, Password: hash, Scope: scope, Perm: perm}
	if err := e.st.Users.Save(u); err != nil {
		e.t.Fatal(err)
	}
	return e.as(e.login(name, password)), u
}

// share makes a link to the user's path and returns it, or fails with the status.
func (e *fileEnv) share(p, body string) (*shareResponse, int) {
	e.t.Helper()
	rec := e.call(http.MethodPost, "/api/share"+p, body)
	if rec.Code != http.StatusOK {
		return nil, rec.Code
	}
	var link shareResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &link); err != nil {
		e.t.Fatal(err)
	}
	return &link, rec.Code
}

func (e *fileEnv) mustShare(p, body string) string {
	e.t.Helper()
	link, code := e.share(p, body)
	if link == nil {
		e.t.Fatalf("sharing %s: %d", p, code)
	}
	return link.Hash
}

// download fetches the link's file and returns the status and the content.
func (e *fileEnv) download(hash string) (int, string) {
	rec := e.call(http.MethodGet, "/api/public/dl/"+hash, "")
	return rec.Code, rec.Body.String()
}

func (e *fileEnv) move(src, dst string) {
	e.t.Helper()
	target := "/api/resources" + src + "?action=rename&destination=" + url.QueryEscape(dst) + "&override=false&rename=false"
	if rec := e.call(http.MethodPatch, target, ""); rec.Code != http.StatusOK {
		e.t.Fatalf("moving %s to %s: %d %s", src, dst, rec.Code, rec.Body.String())
	}
}

func TestShareFollowsTheItem(t *testing.T) {
	env := newShareEnv(t)
	env.write("docs/rapor.pdf", "v1", 0o644)
	hash := env.mustShare("/docs/rapor.pdf", "{}")

	env.move("/docs/rapor.pdf", "/docs/rapor-eski.pdf")
	if code, body := env.download(hash); code != http.StatusOK || body != "v1" {
		t.Fatalf("after a rename the link gives %d %q", code, body)
	}
	env.write("docs/rapor.pdf", "another file", 0o644)
	if _, body := env.download(hash); body != "v1" {
		t.Fatalf("the link serves what took the old place: %q", body)
	}

	if err := os.MkdirAll(filepath.Join(env.root, "arsiv"), 0o755); err != nil {
		t.Fatal(err)
	}
	env.move("/docs", "/arsiv/docs")
	if code, body := env.download(hash); code != http.StatusOK || body != "v1" {
		t.Fatalf("after moving the folder the link gives %d %q", code, body)
	}
	if rec := env.call(http.MethodGet, "/api/share/arsiv/docs/rapor-eski.pdf", ""); !strings.Contains(rec.Body.String(), hash) {
		t.Fatalf("the link is not listed at the new place: %s", rec.Body.String())
	}
}

func TestSharesOfEveryUserFollowAndEndWithTheItem(t *testing.T) {
	env := newShareEnv(t)
	all := users.Permissions{Share: true, Download: true, Create: true, Rename: true, Modify: true, Delete: true}
	ali, _ := env.user("ali", "ali-password-1", "/ali", all)
	env.write("ali/x.txt", "x", 0o644)
	hash := ali.mustShare("/x.txt", "{}")

	// The admin moves ali's file within ali's folder: ali's link follows it.
	if err := os.MkdirAll(filepath.Join(env.root, "ali", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	env.move("/ali/x.txt", "/ali/sub/y.txt")
	if code, body := env.download(hash); code != http.StatusOK || body != "x" {
		t.Fatalf("ali's link after the move: %d %q", code, body)
	}
	if rec := ali.call(http.MethodGet, "/api/share/sub/y.txt", ""); !strings.Contains(rec.Body.String(), hash) {
		t.Fatalf("ali's link is not at the new place: %s", rec.Body.String())
	}

	// Out of ali's reach, the link ends.
	if err := os.MkdirAll(filepath.Join(env.root, "out"), 0o755); err != nil {
		t.Fatal(err)
	}
	env.move("/ali/sub/y.txt", "/out/y.txt")
	if code, _ := env.download(hash); code != http.StatusNotFound {
		t.Fatalf("ali's link to a file out of ali's scope: %d", code)
	}

	// The admin's link to a file in ali's folder ends when ali deletes the file.
	env.write("ali/z.txt", "z", 0o644)
	adminHash := env.mustShare("/ali/z.txt", "{}")
	if rec := ali.call(http.MethodDelete, "/api/resources/z.txt", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", rec.Code)
	}
	env.write("ali/z.txt", "a new z", 0o644)
	if code, body := env.download(adminHash); code != http.StatusNotFound {
		t.Fatalf("the admin's link after ali deleted the file: %d %q", code, body)
	}
}

func TestDeletingAUserEndsTheirSharesAndTrash(t *testing.T) {
	env := newShareEnv(t)
	ali, u := env.user("ali", "ali-password-1", "/ali", users.Permissions{Share: true, Download: true, Delete: true})
	env.write("ali/x.txt", "x", 0o644)
	hash := ali.mustShare("/x.txt", "{}")
	env.write("ali/old.txt", "old", 0o644)
	if rec := ali.call(http.MethodDelete, "/api/resources/old.txt", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", rec.Code)
	}
	bin := filepath.Join(env.root, trash.Dir, strconv.FormatUint(uint64(u.ID), 10))
	if _, err := os.Stat(bin); err != nil {
		t.Fatalf("ali's trash: %v", err)
	}

	target := "/api/users/" + strconv.FormatUint(uint64(u.ID), 10)
	if rec := env.call(http.MethodDelete, target, `{"current_password":"root-password-1"}`); rec.Code != http.StatusOK {
		t.Fatalf("delete user: %d %s", rec.Code, rec.Body.String())
	}
	if rec := env.call(http.MethodGet, "/api/shares", ""); strings.Contains(rec.Body.String(), hash) {
		t.Fatalf("the deleted user's link is still listed: %s", rec.Body.String())
	}
	if code, _ := env.download(hash); code != http.StatusNotFound {
		t.Fatalf("the deleted user's link: %d", code)
	}
	if _, err := os.Stat(bin); !os.IsNotExist(err) {
		t.Fatalf("the deleted user's trash is still there: %v", err)
	}
}

func TestSharePasswordAttemptsAreLimited(t *testing.T) {
	env := newShareEnv(t)
	env.write("docs/a.txt", "secret", 0o644)
	hash := env.mustShare("/docs/a.txt", `{"password":"right-password"}`)
	open := func(password string) (int, string) {
		rec := env.callWith(http.MethodGet, "/api/public/share/"+hash, map[string]string{"X-SHARE-PASSWORD": password}, "")
		if rec.Code == http.StatusTooManyRequests {
			return rec.Code, rec.Header().Get("Retry-After")
		}
		return rec.Code, rec.Body.String()
	}

	code, body := open("right-password")
	if code != http.StatusOK {
		t.Fatalf("right password: %d", code)
	}
	var info struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal([]byte(body), &info); err != nil || info.Token == "" {
		t.Fatalf("no token: %v %s", err, body)
	}

	for i := 0; i < loginAccountLimit; i++ {
		if code, _ := open("wrong-password"); code != http.StatusUnauthorized {
			t.Fatalf("wrong password %d: %d", i+1, code)
		}
	}
	code, retry := open("wrong-password")
	if seconds, err := strconv.Atoi(retry); code != http.StatusTooManyRequests || err != nil || seconds <= 0 {
		t.Fatalf("after %d wrong passwords: %d, Retry-After %q", loginAccountLimit, code, retry)
	}
	if code, _ := open("right-password"); code != http.StatusTooManyRequests {
		t.Fatalf("the right password while locked: %d", code)
	}
	if rec := env.call(http.MethodGet, "/api/public/dl/"+hash+"?token="+url.QueryEscape(info.Token), ""); rec.Code != http.StatusOK {
		t.Fatalf("the token while locked: %d", rec.Code)
	}

	// Another link keeps its own budget.
	env.write("docs/b.txt", "b", 0o644)
	other := env.mustShare("/docs/b.txt", `{"password":"right-password"}`)
	if rec := env.callWith(http.MethodGet, "/api/public/share/"+other, map[string]string{"X-SHARE-PASSWORD": "right-password"}, ""); rec.Code != http.StatusOK {
		t.Fatalf("another link: %d", rec.Code)
	}
}

func TestShareCreationIsChecked(t *testing.T) {
	env := newShareEnv(t)
	env.write("docs/a.txt", "a", 0o644)
	env.write("gizli/b.txt", "b", 0o644)

	if hash := env.mustShare("/docs/a.txt", "{}"); len(hash) != 16 {
		t.Fatalf("link id %q has %d characters, want 16", hash, len(hash))
	}

	now := time.Now()
	for body, want := range map[string]time.Duration{
		`{"expires":"0","unit":"days"}`:     0,
		`{"expires":"7","unit":"days"}`:     7 * 24 * time.Hour,
		`{"expires":"90","unit":"minutes"}`: 90 * time.Minute,
		`{"expires":"2"}`:                   2 * time.Hour,
		`{"expires":"3650","unit":"days"}`:  3650 * 24 * time.Hour,
	} {
		link, code := env.share("/docs/a.txt", body)
		if link == nil {
			t.Fatalf("%s: %d", body, code)
		}
		switch {
		case want == 0 && link.Expire != 0:
			t.Fatalf("%s expires at %d, want never", body, link.Expire)
		case want != 0 && (link.Expire < now.Add(want).Unix()-5 || link.Expire > now.Add(want).Unix()+5):
			t.Fatalf("%s expires at %s, want %s", body, time.Unix(link.Expire, 0), now.Add(want))
		}
	}
	for _, body := range []string{
		`{"expires":"abc"}`,
		`{"expires":"-5","unit":"days"}`,
		`{"expires":"1","unit":"weeks"}`,
		`{"expires":"3651","unit":"days"}`,
		`{"expires":"99999999999999","unit":"seconds"}`,
	} {
		if _, code := env.share("/docs/a.txt", body); code != http.StatusBadRequest {
			t.Fatalf("%s: %d, want 400", body, code)
		}
	}

	set, err := env.st.Settings.Get()
	if err != nil {
		t.Fatal(err)
	}
	set.Rules = append(set.Rules, rules.Rule{Path: "/gizli", Allow: false})
	if err := env.st.Settings.Save(set); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"/gizli/b.txt", "/" + UploadsDir} {
		if _, code := env.share(p, "{}"); code != http.StatusForbidden {
			t.Fatalf("sharing %s: %d, want 403", p, code)
		}
	}
}

// Listing links with expired ones among them neither fails nor shows them (File Browser panicked).
func TestShareListSkipsExpiredLinks(t *testing.T) {
	env := newShareEnv(t)
	env.write("docs/a.txt", "a", 0o644)
	past := time.Now().Add(-time.Hour).Unix()
	for _, link := range []*share.Link{
		{Hash: "aaaaaaaaaaaaaaaa", Path: "/docs/a.txt", UserID: 1, Expire: past},
		{Hash: "bbbbbbbbbbbbbbbb", Path: "/docs/a.txt", UserID: 1},
		{Hash: "cccccccccccccccc", Path: "/docs/a.txt", UserID: 1, Expire: past},
	} {
		if err := env.st.Share.Save(link); err != nil {
			t.Fatal(err)
		}
	}
	for _, target := range []string{"/api/shares", "/api/share/docs/a.txt"} {
		rec := env.call(http.MethodGet, target, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d", target, rec.Code)
		}
		var links []shareResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &links); err != nil {
			t.Fatal(err)
		}
		if len(links) != 1 || links[0].Hash != "bbbbbbbbbbbbbbbb" {
			t.Fatalf("%s lists %v", target, links)
		}
	}
}
