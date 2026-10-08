package fbhttp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/filebrowser/filebrowser/v2/diskcache"
	"github.com/filebrowser/filebrowser/v2/rules"
	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/share"
	"github.com/filebrowser/filebrowser/v2/trash"
	"github.com/filebrowser/filebrowser/v2/users"
)

// Gezgin's WebDAV shares, on a port of their own.

// newDavEnv is a share environment with the WebDAV port on, and the WebDAV server.
func newDavEnv(t *testing.T) (*fileEnv, *httptest.Server) {
	t.Helper()
	env := newShareEnv(t)
	server := &settings.Server{Root: env.root, WebDAVPort: "8092"}
	handler, err := NewHandler(nil, diskcache.NewNoOp(), env.uploads, env.archives, env.st, server, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	env.handler = handler
	dav := httptest.NewServer(NewWebDAVHandler(diskcache.NewNoOp(), env.st, server))
	t.Cleanup(dav.Close)
	return env, dav
}

// davShare makes a WebDAV share of the user's folder p for webdav-user and its password.
func (e *fileEnv) davShare(p string, writable bool) string {
	e.t.Helper()
	body := fmt.Sprintf(`{"kind":"webdav","webdavUser":"webdav-user","password":"webdav-password","writable":%t}`, writable)
	link, code := e.share(p, body)
	if link == nil {
		e.t.Fatalf("WebDAV share of %s: %d", p, code)
	}
	return link.Hash
}

type davClient struct {
	t    *testing.T
	base string // the share's URL
	user string
	pass string
}

func (c *davClient) do(method, p string, headers map[string]string, body string) *http.Response {
	c.t.Helper()
	req, err := http.NewRequest(method, c.base+p, strings.NewReader(body))
	if err != nil {
		c.t.Fatal(err)
	}
	if c.user != "" {
		req.SetBasicAuth(c.user, c.pass)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	c.t.Cleanup(func() { res.Body.Close() })
	return res
}

func (c *davClient) code(method, p string, headers map[string]string, body string) int {
	c.t.Helper()
	res := c.do(method, p, headers, body)
	_, _ = io.Copy(io.Discard, res.Body)
	return res.StatusCode
}

var hrefPattern = regexp.MustCompile(`<D:href>([^<]*)</D:href>`)

// list returns the names a PROPFIND of the folder p shows, the folder itself aside.
func (c *davClient) list(p string) []string {
	c.t.Helper()
	res := c.do("PROPFIND", p, map[string]string{"Depth": "1"}, "")
	data, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusMultiStatus {
		c.t.Fatalf("PROPFIND %s: %d %s", p, res.StatusCode, data)
	}
	names := []string{}
	for _, m := range hrefPattern.FindAllStringSubmatch(string(data), -1) {
		rest := strings.TrimPrefix(m[1], strings.TrimPrefix(c.base, "http://"+strings.Split(strings.TrimPrefix(c.base, "http://"), "/")[0]))
		rest = strings.TrimPrefix(rest, p)
		if rest = strings.Trim(rest, "/"); rest != "" {
			names = append(names, rest)
		}
	}
	sort.Strings(names)
	return names
}

func (c *davClient) get(p string) (int, string) {
	c.t.Helper()
	res := c.do(http.MethodGet, p, nil, "")
	data, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(data)
}

func TestWebDAVShareCreation(t *testing.T) {
	env, _ := newDavEnv(t)
	env.write("docs/a.txt", "a", 0o644)
	create := func(p, body string) int {
		_, code := env.share(p, body)
		return code
	}

	for body, want := range map[string]int{
		`{"kind":"webdav","webdavUser":"u","password":"webdav-password"}`: http.StatusBadRequest, // a file
		`{"kind":"other","webdavUser":"u","password":"webdav-password"}`:  http.StatusBadRequest,
		`{"webdavUser":"u","password":"webdav-password"}`:                 http.StatusBadRequest, // a link with a username
	} {
		if code := create("/docs/a.txt", body); code != want {
			t.Errorf("%s on a file: %d, want %d", body, code, want)
		}
	}
	for body, want := range map[string]int{
		`{"kind":"webdav","webdavUser":"","password":"webdav-password"}`:    http.StatusBadRequest,
		`{"kind":"webdav","webdavUser":"a:b","password":"webdav-password"}`: http.StatusBadRequest,
		`{"kind":"webdav","webdavUser":"u","password":"short"}`:             http.StatusBadRequest,
		`{"kind":"webdav","webdavUser":"u","password":"password"}`:          http.StatusBadRequest, // a common one
		`{"kind":"webdav","webdavUser":"u","password":"webdav-password"}`:   http.StatusOK,
	} {
		if code := create("/docs", body); code != want {
			t.Errorf("%s: %d, want %d", body, code, want)
		}
	}

	link, _ := env.share("/docs", `{"kind":"webdav","webdavUser":" Ali ","password":"webdav-password","writable":true,"expires":"7","unit":"days"}`)
	if link == nil || link.Kind != share.KindWebDAV || link.WebDAVUser != "Ali" || !link.Writable || !link.HasPassword || link.Expire == 0 {
		t.Fatalf("the WebDAV share: %+v", link)
	}
	// It is not a page.
	for _, target := range []string{"/api/public/share/" + link.Hash, "/api/public/dl/" + link.Hash} {
		if rec := env.call(http.MethodGet, target, ""); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s: %d, want 404", target, rec.Code)
		}
	}

	// Writing needs every permission writing takes.
	reader, _ := env.user("reader", "reader-password", "/reader", users.Permissions{Share: true, Download: true, Create: true})
	if _, code := reader.share("/", `{"kind":"webdav","webdavUser":"u","password":"webdav-password","writable":true}`); code != http.StatusForbidden {
		t.Errorf("a writable share without the permissions: %d, want 403", code)
	}

	// Off without a port.
	off := newShareEnv(t)
	off.write("docs/a.txt", "a", 0o644)
	if _, code := off.share("/docs", `{"kind":"webdav","webdavUser":"u","password":"webdav-password"}`); code != http.StatusBadRequest {
		t.Errorf("a WebDAV share with WebDAV off: %d, want 400", code)
	}
}

func TestWebDAVAuthentication(t *testing.T) {
	env, dav := newDavEnv(t)
	env.write("docs/a.txt", "a", 0o644)
	hash := env.davShare("/docs", false)
	linkHash := env.mustShare("/docs/a.txt", "{}")

	anonymous := &davClient{t: t, base: dav.URL + "/" + hash}
	res := anonymous.do("PROPFIND", "/", map[string]string{"Depth": "1"}, "")
	if res.StatusCode != http.StatusUnauthorized || !strings.HasPrefix(res.Header.Get("WWW-Authenticate"), "Basic ") {
		t.Fatalf("without credentials: %d %q", res.StatusCode, res.Header.Get("WWW-Authenticate"))
	}
	for _, other := range []string{"", "nothing-like-it", linkHash} {
		c := &davClient{t: t, base: dav.URL + "/" + other, user: "webdav-user", pass: "webdav-password"}
		if code := c.code("PROPFIND", "/", map[string]string{"Depth": "0"}, ""); code != http.StatusNotFound {
			t.Errorf("share %q: %d, want 404", other, code)
		}
	}

	right := &davClient{t: t, base: dav.URL + "/" + hash, user: "webdav-user", pass: "webdav-password"}
	if names := right.list("/"); len(names) != 1 || names[0] != "a.txt" {
		t.Fatalf("listing: %v", names)
	}

	wrongName := &davClient{t: t, base: right.base, user: "someone", pass: "webdav-password"}
	if code := wrongName.code("PROPFIND", "/", nil, ""); code != http.StatusUnauthorized {
		t.Fatalf("wrong username: %d", code)
	}
	wrong := &davClient{t: t, base: right.base, user: "webdav-user", pass: "wrong-password"}
	for i := 1; i < loginAccountLimit; i++ {
		if code := wrong.code("PROPFIND", "/", nil, ""); code != http.StatusUnauthorized {
			t.Fatalf("wrong password %d: %d", i+1, code)
		}
	}
	res = wrong.do("PROPFIND", "/", nil, "")
	if res.StatusCode != http.StatusTooManyRequests || res.Header.Get("Retry-After") == "" {
		t.Fatalf("after %d wrong attempts: %d, Retry-After %q", loginAccountLimit, res.StatusCode, res.Header.Get("Retry-After"))
	}
	// A password that passed before is still taken.
	if code, body := right.get("/a.txt"); code != http.StatusOK || body != "a" {
		t.Fatalf("the right password while the share is locked: %d %q", code, body)
	}
}

func TestWebDAVReadOnlyShare(t *testing.T) {
	env, dav := newDavEnv(t)
	env.write("docs/a.txt", "a", 0o644)
	env.write("docs/sub/b.txt", "b", 0o644)
	env.write("docs/gizli/c.txt", "c", 0o644)
	env.write("docs/.gezgin-0123456789abcdef.tmp", "half", 0o644)
	env.write("outside/secret.txt", "secret", 0o644)
	if err := os.Symlink(filepath.Join(env.root, "outside"), filepath.Join(env.root, "docs", "link")); err != nil {
		t.Fatal(err)
	}
	set, err := env.st.Settings.Get()
	if err != nil {
		t.Fatal(err)
	}
	set.Rules = append(set.Rules, rules.Rule{Path: "/docs/gizli", Allow: false})
	if err := env.st.Settings.Save(set); err != nil {
		t.Fatal(err)
	}

	hash := env.davShare("/docs", false)
	c := &davClient{t: t, base: dav.URL + "/" + hash, user: "webdav-user", pass: "webdav-password"}

	if names := c.list("/"); strings.Join(names, ",") != "a.txt,sub" {
		t.Fatalf("listing: %v", names)
	}
	if names := c.list("/sub/"); strings.Join(names, ",") != "b.txt" {
		t.Fatalf("listing of sub: %v", names)
	}
	if code, body := c.get("/a.txt"); code != http.StatusOK || body != "a" {
		t.Fatalf("GET: %d %q", code, body)
	}
	for _, p := range []string{"/gizli/c.txt", "/link/secret.txt", "/.gezgin-0123456789abcdef.tmp"} {
		if code, body := c.get(p); code == http.StatusOK {
			t.Fatalf("GET %s served %q", p, body)
		}
	}

	for _, req := range []struct{ method, path, body string }{
		{http.MethodPut, "/new.txt", "x"},
		{http.MethodPut, "/a.txt", "x"},
		{http.MethodDelete, "/a.txt", ""},
		{"MKCOL", "/new/", ""},
		{"MOVE", "/a.txt", ""},
		{"LOCK", "/a.txt", ""},
	} {
		if code := c.code(req.method, req.path, map[string]string{"Destination": c.base + "/moved.txt"}, req.body); code != http.StatusForbidden {
			t.Errorf("%s %s on a read-only share: %d, want 403", req.method, req.path, code)
		}
	}
	if got := env.read("docs/a.txt"); got != "a" {
		t.Fatalf("a read-only share changed a.txt: %q", got)
	}

	// An expired share is gone.
	link, err := env.st.Share.GetByHash(hash)
	if err != nil {
		t.Fatal(err)
	}
	link.Expire = time.Now().Add(-time.Minute).Unix()
	if err := env.st.Share.Save(link); err != nil {
		t.Fatal(err)
	}
	if code := c.code("PROPFIND", "/", nil, ""); code != http.StatusNotFound {
		t.Fatalf("an expired share: %d, want 404", code)
	}
}

func TestWebDAVReadWriteShare(t *testing.T) {
	env, dav := newDavEnv(t)
	env.write("docs/a.txt", "a", 0o640)
	hash := env.davShare("/docs", true)
	c := &davClient{t: t, base: dav.URL + "/" + hash, user: "webdav-user", pass: "webdav-password"}
	linkHash := env.mustShare("/docs/a.txt", "{}")

	if code := c.code(http.MethodPut, "/new.txt", nil, "new"); code != http.StatusCreated {
		t.Fatalf("PUT new.txt: %d", code)
	}
	if got := env.read("docs/new.txt"); got != "new" {
		t.Fatalf("new.txt: %q", got)
	}
	if code := c.code(http.MethodPut, "/a.txt", nil, "a2"); code != http.StatusCreated {
		t.Fatalf("PUT a.txt: %d", code)
	}
	if info, err := os.Stat(filepath.Join(env.root, "docs", "a.txt")); err != nil || info.Mode().Perm() != 0o640 || env.read("docs/a.txt") != "a2" {
		t.Fatalf("the replaced a.txt: %v %v %q", info.Mode(), err, env.read("docs/a.txt"))
	}

	if code := c.code("MKCOL", "/sub/", nil, ""); code != http.StatusCreated {
		t.Fatalf("MKCOL: %d", code)
	}
	if code := c.code("MOVE", "/a.txt", map[string]string{"Destination": c.base + "/sub/b.txt"}, ""); code != http.StatusCreated {
		t.Fatalf("MOVE: %d", code)
	}
	if code, body := env.download(linkHash); code != http.StatusOK || body != "a2" {
		t.Fatalf("the link to the moved file: %d %q", code, body)
	}
	if code := c.code("COPY", "/sub/b.txt", map[string]string{"Destination": c.base + "/c.txt"}, ""); code != http.StatusCreated {
		t.Fatalf("COPY: %d", code)
	}
	if got := env.read("docs/c.txt"); got != "a2" {
		t.Fatalf("the copy: %q", got)
	}

	if code := c.code(http.MethodDelete, "/c.txt", nil, ""); code != http.StatusNoContent {
		t.Fatalf("DELETE: %d", code)
	}
	items, err := trash.For(env.root, 1).List()
	if err != nil || len(items) != 1 || items[0].Name != "c.txt" || items[0].Origin != "/docs" {
		t.Fatalf("the owner's trash: %+v %v", items, err)
	}
	// The share itself is not deleted (x/net/webdav answers a refused delete with 405).
	if code := c.code(http.MethodDelete, "/", nil, ""); code != http.StatusMethodNotAllowed {
		t.Fatalf("DELETE of the share itself: %d, want 405", code)
	}
	if _, err := os.Stat(filepath.Join(env.root, "docs")); err != nil {
		t.Fatalf("the shared folder: %v", err)
	}

	// A lock keeps others from writing; its holder writes with its token.
	res := c.do("LOCK", "/new.txt", map[string]string{"Timeout": "Second-60"},
		`<?xml version="1.0" encoding="utf-8"?><D:lockinfo xmlns:D="DAV:"><D:lockscope><D:exclusive/></D:lockscope><D:locktype><D:write/></D:locktype></D:lockinfo>`)
	token := res.Header.Get("Lock-Token")
	if res.StatusCode != http.StatusOK || token == "" {
		t.Fatalf("LOCK: %d %q", res.StatusCode, token)
	}
	if code := c.code(http.MethodPut, "/new.txt", nil, "other"); code != http.StatusLocked {
		t.Fatalf("PUT without the lock: %d, want 423", code)
	}
	if code := c.code(http.MethodPut, "/new.txt", map[string]string{"If": "(" + token + ")"}, "mine"); code != http.StatusCreated {
		t.Fatalf("PUT with the lock: %d", code)
	}
	if code := c.code("UNLOCK", "/new.txt", map[string]string{"Lock-Token": token}, ""); code != http.StatusNoContent {
		t.Fatalf("UNLOCK: %d", code)
	}
	if got := env.read("docs/new.txt"); got != "mine" {
		t.Fatalf("new.txt: %q", got)
	}

	// Without a permission writing takes, the share is read-only.
	root, err := env.st.Users.Get(env.root, false, uint(1))
	if err != nil {
		t.Fatal(err)
	}
	root.Perm.Delete = false
	if err := env.st.Users.Update(root, "Perm"); err != nil {
		t.Fatal(err)
	}
	if code := c.code(http.MethodPut, "/new.txt", nil, "x"); code != http.StatusForbidden {
		t.Fatalf("PUT without the delete permission: %d, want 403", code)
	}
}

// A PUT whose client goes away half way leaves the file as it was, and no temporary file.
func TestWebDAVInterruptedPutKeepsTheFile(t *testing.T) {
	env, dav := newDavEnv(t)
	env.write("docs/a.txt", "original", 0o644)
	hash := env.davShare("/docs", true)

	conn, err := net.Dial("tcp", strings.TrimPrefix(dav.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	req := fmt.Sprintf("PUT /%s/a.txt HTTP/1.1\r\nHost: dav\r\nAuthorization: Basic d2ViZGF2LXVzZXI6d2ViZGF2LXBhc3N3b3Jk\r\nContent-Length: 100\r\n\r\nonly ten b", hash)
	if _, err := conn.Write([]byte(req)); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	conn.Close()

	deadline := time.Now().Add(3 * time.Second)
	for len(env.leftovers("docs")) > 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if left := env.leftovers("docs"); len(left) > 0 {
		t.Fatalf("the interrupted PUT left %v", left)
	}
	if got := env.read("docs/a.txt"); got != "original" {
		t.Fatalf("the interrupted PUT changed a.txt: %q", got)
	}

	// A complete one, as a check that the request was right.
	conn, err = net.Dial("tcp", strings.TrimPrefix(dav.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	req = fmt.Sprintf("PUT /%s/a.txt HTTP/1.1\r\nHost: dav\r\nAuthorization: Basic d2ViZGF2LXVzZXI6d2ViZGF2LXBhc3N3b3Jk\r\nContent-Length: 10\r\n\r\nonly ten b", hash)
	if _, err := conn.Write([]byte(req)); err != nil {
		t.Fatal(err)
	}
	res, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil || res.StatusCode != http.StatusCreated {
		t.Fatalf("the complete PUT: %v %v", res, err)
	}
	if got := env.read("docs/a.txt"); got != "only ten b" {
		t.Fatalf("a.txt after the complete PUT: %q", got)
	}
}

// The share list names a WebDAV share's kind and username.
func TestWebDAVShareIsListed(t *testing.T) {
	env, _ := newDavEnv(t)
	env.write("docs/a.txt", "a", 0o644)
	hash := env.davShare("/docs", true)
	rec := env.call(http.MethodGet, "/api/shares", "")
	var links []shareResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &links); err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 || links[0].Hash != hash || links[0].Kind != share.KindWebDAV ||
		links[0].WebDAVUser != "webdav-user" || !links[0].Writable {
		t.Fatalf("listed %+v", links)
	}
	if strings.Contains(rec.Body.String(), "password_hash") || strings.Contains(rec.Body.String(), "$2a$") {
		t.Fatalf("the list shows the password hash: %s", rec.Body.String())
	}
}
