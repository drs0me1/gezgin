package fbhttp

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/filebrowser/filebrowser/v2/users"
)

// brokenBody sends a little of the new content, then fails like a client that went away.
type brokenBody struct{ sent bool }

func (b *brokenBody) Read(p []byte) (int, error) {
	if !b.sent {
		b.sent = true
		return copy(p, "new"), nil
	}
	return 0, errors.New("client went away")
}

type fileEnv struct {
	*sessionEnv
	token string
}

func newFileEnv(t *testing.T) *fileEnv {
	t.Helper()
	env := newSessionEnv(t)
	hash, err := users.HashPwd("root-password-1")
	if err != nil {
		t.Fatal(err)
	}
	perm := users.Permissions{Admin: true, Create: true, Rename: true, Modify: true, Delete: true, Download: true}
	if err := env.st.Users.Save(&users.User{Username: "root", Password: hash, Scope: ".", Perm: perm}); err != nil {
		t.Fatal(err)
	}
	return &fileEnv{sessionEnv: env, token: env.login("root", "root-password-1")}
}

func (e *fileEnv) write(name, content string, mode os.FileMode) {
	e.t.Helper()
	full := filepath.Join(e.root, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		e.t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), mode); err != nil {
		e.t.Fatal(err)
	}
	if err := os.Chmod(full, mode); err != nil {
		e.t.Fatal(err)
	}
}

func (e *fileEnv) read(name string) string {
	data, err := os.ReadFile(filepath.Join(e.root, name))
	if err != nil {
		return "<missing>"
	}
	return string(data)
}

func (e *fileEnv) send(method, target string, body io.Reader) int {
	req := httptest.NewRequest(method, target, body)
	req.Header.Set("X-Auth", e.token)
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	return rec.Code
}

// leftovers lists the temporary files a write left in dir.
func (e *fileEnv) leftovers(dir string) []string {
	var found []string
	entries, _ := os.ReadDir(filepath.Join(e.root, dir))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".gezgin-") {
			found = append(found, entry.Name())
		}
	}
	return found
}

func TestFailedWritesLeaveTheFileAsItWas(t *testing.T) {
	env := newFileEnv(t)
	env.write("a.txt", "original", 0o644)
	env.write("b.txt", "original", 0o644)

	if code := env.send(http.MethodPost, "/api/resources/a.txt?override=true", &brokenBody{}); code == http.StatusOK {
		t.Fatalf("an interrupted upload answered 200")
	}
	if got := env.read("a.txt"); got != "original" {
		t.Errorf("after an interrupted overwrite a.txt = %q; want the original", got)
	}

	if code := env.send(http.MethodPut, "/api/resources/b.txt", &brokenBody{}); code == http.StatusOK {
		t.Fatalf("an interrupted save answered 200")
	}
	if got := env.read("b.txt"); got != "original" {
		t.Errorf("after an interrupted save b.txt = %q; want the original", got)
	}

	if code := env.send(http.MethodPost, "/api/resources/new.txt", &brokenBody{}); code == http.StatusOK {
		t.Fatalf("an interrupted new upload answered 200")
	}
	if got := env.read("new.txt"); got != "<missing>" {
		t.Errorf("an interrupted new upload left %q", got)
	}
	if left := env.leftovers("."); len(left) != 0 {
		t.Errorf("temporary files left behind: %v", left)
	}
}

func TestWritesKeepPermissionsAndLinks(t *testing.T) {
	env := newFileEnv(t)
	env.write("run.sh", "#!/bin/sh\n", 0o755)
	if code := env.send(http.MethodPut, "/api/resources/run.sh", strings.NewReader("#!/bin/sh\necho hi\n")); code != http.StatusOK {
		t.Fatalf("save = %d", code)
	}
	info, err := os.Stat(filepath.Join(env.root, "run.sh"))
	if err != nil || info.Mode().Perm() != 0o755 || env.read("run.sh") != "#!/bin/sh\necho hi\n" {
		t.Errorf("saved run.sh: mode %v, content %q; want 0755 and the new content", info.Mode().Perm(), env.read("run.sh"))
	}

	if code := env.send(http.MethodPost, "/api/resources/fresh.txt", strings.NewReader("x")); code != http.StatusOK {
		t.Fatalf("upload = %d", code)
	}
	if info, _ := os.Stat(filepath.Join(env.root, "fresh.txt")); info.Mode().Perm() != 0o640 {
		t.Errorf("a new upload has mode %v; want the configured 0640", info.Mode().Perm())
	}

	env.write("target.conf", "old", 0o600)
	if err := os.Symlink("target.conf", filepath.Join(env.root, "link.conf")); err != nil {
		t.Fatal(err)
	}
	if code := env.send(http.MethodPut, "/api/resources/link.conf", strings.NewReader("new")); code != http.StatusOK {
		t.Fatalf("save through a link = %d", code)
	}
	if st, err := os.Lstat(filepath.Join(env.root, "link.conf")); err != nil || st.Mode()&os.ModeSymlink == 0 {
		t.Errorf("saving through a link replaced the link")
	}
	if got := env.read("target.conf"); got != "new" {
		t.Errorf("the link's target = %q; want the new content", got)
	}
}

func TestFilesAndFoldersNeverReplaceEachOther(t *testing.T) {
	env := newFileEnv(t)
	env.write("dir/inner.txt", "inner", 0o644)
	env.write("file.txt", "precious", 0o644)

	cases := []string{
		"/api/resources/dir?action=rename&destination=%2Ffile.txt&override=true",
		"/api/resources/file.txt?action=rename&destination=%2Fdir&override=true",
		"/api/resources/dir?action=copy&destination=%2Ffile.txt&override=true",
		"/api/resources/file.txt?action=copy&destination=%2Fdir&override=true",
	}
	for _, target := range cases {
		if code := env.send(http.MethodPatch, target, nil); code != http.StatusConflict {
			t.Errorf("%s = %d; want 409", target, code)
		}
	}
	if env.read("file.txt") != "precious" || env.read("dir/inner.txt") != "inner" {
		t.Errorf("a refused replacement changed the files: %q %q", env.read("file.txt"), env.read("dir/inner.txt"))
	}
}

func TestMovingAFolderOntoAFolderMergesThem(t *testing.T) {
	env := newFileEnv(t)
	env.write("src/new.txt", "new", 0o644)
	env.write("src/same.txt", "from src", 0o644)
	env.write("src/sub/deep.txt", "deep", 0o644)
	env.write("dst/src/old.txt", "old", 0o644)
	env.write("dst/src/same.txt", "from dst", 0o644)

	if code := env.send(http.MethodPatch, "/api/resources/src?action=rename&destination=%2Fdst%2Fsrc&override=true", nil); code != http.StatusOK {
		t.Fatalf("merge move = %d", code)
	}
	var names []string
	_ = filepath.Walk(filepath.Join(env.root, "dst", "src"), func(p string, info os.FileInfo, _ error) error {
		if !info.IsDir() {
			rel, _ := filepath.Rel(filepath.Join(env.root, "dst", "src"), p)
			names = append(names, rel)
		}
		return nil
	})
	sort.Strings(names)
	if strings.Join(names, ",") != "new.txt,old.txt,same.txt,sub/deep.txt" || env.read("dst/src/same.txt") != "from src" {
		t.Errorf("merged folder = %v, same.txt %q", names, env.read("dst/src/same.txt"))
	}
	if _, err := os.Stat(filepath.Join(env.root, "src")); !os.IsNotExist(err) {
		t.Errorf("the moved folder is still there: %v", err)
	}

	env.write("one.txt", "one", 0o644)
	env.write("two.txt", "two", 0o644)
	if code := env.send(http.MethodPatch, "/api/resources/one.txt?action=rename&destination=%2Ftwo.txt&override=true", nil); code != http.StatusOK {
		t.Fatalf("file over file = %d", code)
	}
	if env.read("two.txt") != "one" || env.read("one.txt") != "<missing>" {
		t.Errorf("file over file: two.txt %q one.txt %q", env.read("two.txt"), env.read("one.txt"))
	}
}
