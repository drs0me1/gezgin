package fbhttp

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/filebrowser/filebrowser/v2/diskcache"
	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/users"
)

// Gezgin stages a tus upload's data and puts the file in place only once it is complete.

func (e *fileEnv) callWith(method, target string, headers map[string]string, body string) *httptest.ResponseRecorder {
	e.t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("X-Auth", e.token)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	return rec
}

func (e *fileEnv) startUpload(name string, length string, override bool) {
	e.t.Helper()
	target := "/api/tus/" + name + "?override=false"
	if override {
		target = "/api/tus/" + name + "?override=true"
	}
	if rec := e.callWith(http.MethodPost, target, map[string]string{"Upload-Length": length}, ""); rec.Code != http.StatusCreated {
		e.t.Fatalf("POST %s: %d %s", name, rec.Code, rec.Body.String())
	}
}

func (e *fileEnv) sendChunk(name, offset, chunk string) int {
	e.t.Helper()
	return e.callWith(http.MethodPatch, "/api/tus/"+name, map[string]string{
		"Content-Type":  "application/offset+octet-stream",
		"Upload-Offset": offset,
	}, chunk).Code
}

// staged lists the data of the uploads in progress.
func (e *fileEnv) staged() []string {
	entries, _ := os.ReadDir(filepath.Join(e.root, UploadsDir))
	names := []string{}
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

// restart serves the same storage and files with a new upload cache, as a restarted server does.
func (e *fileEnv) restart(ttl time.Duration) {
	e.t.Helper()
	dir := filepath.Join(e.root, UploadsDir)
	NewUploadCache(dir).Close() // a starting server empties the staging folder
	e.uploads = newUploadCache(dir, ttl)
	e.t.Cleanup(e.uploads.Close)
	handler, err := NewHandler(nil, diskcache.NewNoOp(), e.uploads, e.st, &settings.Server{Root: e.root}, fstest.MapFS{})
	if err != nil {
		e.t.Fatal(err)
	}
	e.handler = handler
}

func TestOverrideUploadKeepsTheOriginalUntilComplete(t *testing.T) {
	env := newFileEnv(t)
	env.write("a.txt", "original", 0o640)

	env.startUpload("a.txt", "7", true)
	if got := env.read("a.txt"); got != "original" {
		t.Fatalf("starting the upload changed the file: %q", got)
	}
	if code := env.sendChunk("a.txt", "0", "new"); code != http.StatusNoContent {
		t.Fatalf("first chunk: %d", code)
	}
	if got := env.read("a.txt"); got != "original" {
		t.Fatalf("an unfinished upload changed the file: %q", got)
	}
	if len(env.staged()) != 1 {
		t.Fatalf("staged = %v, want the upload's data", env.staged())
	}

	if code := env.sendChunk("a.txt", "3", "data"); code != http.StatusNoContent {
		t.Fatalf("last chunk: %d", code)
	}
	if got := env.read("a.txt"); got != "newdata" {
		t.Fatalf("the complete upload is %q", got)
	}
	if info, err := os.Stat(filepath.Join(env.root, "a.txt")); err != nil || info.Mode().Perm() != 0o640 {
		t.Fatalf("the replaced file lost its permissions: %v %v", info.Mode(), err)
	}
	if len(env.staged()) != 0 {
		t.Fatalf("the upload left data behind: %v", env.staged())
	}
	for _, name := range env.leftovers("") {
		if name != UploadsDir {
			t.Fatalf("the upload left %s behind", name)
		}
	}
	if rec := env.callWith(http.MethodHead, "/api/tus/a.txt", nil, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("HEAD after the upload: %d", rec.Code)
	}
}

func TestCancelledOrAbandonedUploadLeavesTheDestination(t *testing.T) {
	env := newFileEnv(t)
	env.write("a.txt", "original", 0o644)

	env.startUpload("a.txt", "7", true)
	if code := env.sendChunk("a.txt", "0", "new"); code != http.StatusNoContent {
		t.Fatalf("chunk: %d", code)
	}
	if rec := env.callWith(http.MethodDelete, "/api/tus/a.txt", nil, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("cancel: %d", rec.Code)
	}
	if got := env.read("a.txt"); got != "original" {
		t.Fatalf("cancelling the upload changed the file: %q", got)
	}
	if len(env.staged()) != 0 {
		t.Fatalf("the cancelled upload kept its data: %v", env.staged())
	}
	if rec := env.callWith(http.MethodDelete, "/api/tus/a.txt", nil, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("cancelling an ended upload: %d", rec.Code)
	}

	env.restart(50 * time.Millisecond)
	env.startUpload("a.txt", "7", true)
	if code := env.sendChunk("a.txt", "0", "new"); code != http.StatusNoContent {
		t.Fatalf("chunk: %d", code)
	}
	deadline := time.Now().Add(5 * time.Second)
	for len(env.staged()) != 0 {
		if time.Now().After(deadline) {
			t.Fatal("the abandoned upload kept its data")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := env.read("a.txt"); got != "original" {
		t.Fatalf("the abandoned upload changed the file: %q", got)
	}
}

// Cancelling drops only the staged data, so it takes the permission that started the upload.
func TestUploadCanBeCancelledWithoutDeletePermission(t *testing.T) {
	env := newFileEnv(t)
	hash, err := users.HashPwd("uploader-pass")
	if err != nil {
		t.Fatal(err)
	}
	if err := env.st.Users.Save(&users.User{Username: "uploader", Password: hash, Scope: ".", Perm: users.Permissions{Create: true}}); err != nil {
		t.Fatal(err)
	}
	env.token = env.login("uploader", "uploader-pass")

	env.startUpload("b.bin", "10", false)
	if rec := env.callWith(http.MethodDelete, "/api/tus/b.bin", nil, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("cancel without the delete permission: %d", rec.Code)
	}
	if env.read("b.bin") != "<missing>" || len(env.staged()) != 0 {
		t.Fatal("the cancelled upload left something behind")
	}
}

func TestUnfinishedUploadIsNotSeen(t *testing.T) {
	env := newFileEnv(t)
	env.startUpload("b.bin", "10", false)
	if code := env.sendChunk("b.bin", "0", "half"); code != http.StatusNoContent {
		t.Fatalf("chunk: %d", code)
	}

	if env.read("b.bin") != "<missing>" {
		t.Fatal("the unfinished upload is at its destination")
	}
	listing := env.callWith(http.MethodGet, "/api/resources/", nil, "")
	if listing.Code != http.StatusOK || strings.Contains(listing.Body.String(), "b.bin") ||
		strings.Contains(listing.Body.String(), UploadsDir) {
		t.Fatalf("the listing shows the upload: %d %s", listing.Code, listing.Body.String())
	}
	if rec := env.callWith(http.MethodGet, "/api/raw/b.bin", nil, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("download of the unfinished upload: %d", rec.Code)
	}
	if rec := env.callWith(http.MethodGet, "/api/search/?query=b.bin", nil, ""); strings.Contains(rec.Body.String(), "b.bin") {
		t.Fatalf("search finds the unfinished upload: %s", rec.Body.String())
	}
	for _, target := range []string{
		"/api/resources/" + UploadsDir + "/",
		"/api/raw/" + UploadsDir + "/" + env.staged()[0],
	} {
		if rec := env.callWith(http.MethodGet, target, nil, ""); rec.Code != http.StatusForbidden {
			t.Fatalf("GET %s: %d, want 403", target, rec.Code)
		}
	}
	if rec := env.callWith(http.MethodPost, "/api/resources/"+UploadsDir+"/x.txt", nil, "x"); rec.Code != http.StatusForbidden {
		t.Fatalf("writing into the staging folder: %d, want 403", rec.Code)
	}
}

// A restart forgets the uploads in progress: their data goes, and the client starts over.
func TestUploadAfterRestartStartsOver(t *testing.T) {
	env := newFileEnv(t)
	env.startUpload("b.bin", "10", false)
	if code := env.sendChunk("b.bin", "0", "half"); code != http.StatusNoContent {
		t.Fatalf("chunk: %d", code)
	}

	env.restart(uploadCacheTTL)
	if len(env.staged()) != 0 {
		t.Fatalf("the restart kept the data of an upload no one can resume: %v", env.staged())
	}
	if rec := env.callWith(http.MethodHead, "/api/tus/b.bin", nil, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("HEAD after the restart: %d", rec.Code)
	}
	env.startUpload("b.bin", "10", false)
	if code := env.sendChunk("b.bin", "0", "0123456789"); code != http.StatusNoContent {
		t.Fatalf("chunk: %d", code)
	}
	if got := env.read("b.bin"); got != "0123456789" {
		t.Fatalf("the upload is %q", got)
	}
}

// The tus client sends no chunk for an empty file.
func TestEmptyUploadIsCompleteAtOnce(t *testing.T) {
	env := newFileEnv(t)
	env.startUpload("empty.txt", "0", false)
	if info, err := os.Stat(filepath.Join(env.root, "empty.txt")); err != nil || info.Size() != 0 {
		t.Fatalf("the empty file is not in place: %v", err)
	}
	if len(env.staged()) != 0 {
		t.Fatalf("staged = %v", env.staged())
	}

	env.write("full.txt", "content", 0o600)
	env.startUpload("full.txt", "0", true)
	if info, err := os.Stat(filepath.Join(env.root, "full.txt")); err != nil || info.Size() != 0 || info.Mode().Perm() != 0o600 {
		t.Fatalf("the emptied file: %v %v", info, err)
	}
}

// The destination is checked again when the data is in: it may have been taken meanwhile.
func TestUploadRefusedWhenTheNameWasTakenMeanwhile(t *testing.T) {
	env := newFileEnv(t)
	env.startUpload("c.txt", "3", false)
	env.write("c.txt", "other", 0o644)
	if code := env.sendChunk("c.txt", "0", "abc"); code != http.StatusConflict {
		t.Fatalf("last chunk: %d, want 409", code)
	}
	if got := env.read("c.txt"); got != "other" {
		t.Fatalf("the refused upload replaced the file: %q", got)
	}
	if len(env.staged()) != 0 {
		t.Fatalf("the refused upload kept its data: %v", env.staged())
	}
}

func TestUploadNeedsRoomOnTheDisk(t *testing.T) {
	env := newFileEnv(t)
	huge := "4611686018427387904" // 4 EiB
	rec := env.callWith(http.MethodPost, "/api/tus/huge.bin?override=false", map[string]string{"Upload-Length": huge}, "")
	if rec.Code != http.StatusInsufficientStorage {
		t.Fatalf("tus POST: %d, want 507", rec.Code)
	}
	if len(env.staged()) != 0 {
		t.Fatalf("staged = %v", env.staged())
	}

	req := httptest.NewRequest(http.MethodPost, "/api/resources/huge.bin", strings.NewReader("x"))
	req.ContentLength = 1 << 62
	req.Header.Set("X-Auth", env.token)
	res := httptest.NewRecorder()
	env.handler.ServeHTTP(res, req)
	if res.Code != http.StatusInsufficientStorage {
		t.Fatalf("POST: %d, want 507", res.Code)
	}
	if env.read("huge.bin") != "<missing>" {
		t.Fatal("the refused upload was written")
	}
}
