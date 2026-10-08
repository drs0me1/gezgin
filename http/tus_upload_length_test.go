package fbhttp

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/filebrowser/filebrowser/v2/diskcache"
	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/users"
)

// A TUS PATCH must not write more than the declared Upload-Length. A client that
// declares a small length and then streams a large body must be rejected without
// the excess being written to disk, while a correctly-sized upload still works.
func TestTusPatchEnforcesUploadLength(t *testing.T) {
	root := t.TempDir()
	userScope := filepath.Join(root, "user")
	if err := os.MkdirAll(userScope, 0o755); err != nil {
		t.Fatal(err)
	}

	key := []byte("test-signing-key")
	perm := users.Permissions{Create: true, Modify: true}
	st := scopedUserStorage(t, userScope, perm, key)
	signed := signToken(t, perm, key)

	cache := newUploadCache(filepath.Join(root, UploadsDir), uploadCacheTTL)
	t.Cleanup(cache.Close)
	post := handle(tusPostHandler(cache, diskcache.NewNoOp()), "", st, &settings.Server{})
	patch := handle(tusPatchHandler(cache, diskcache.NewNoOp()), "", st, &settings.Server{})

	patchReq := func(body string) *httptest.ResponseRecorder {
		req, _ := http.NewRequest(http.MethodPatch, "/file.txt", strings.NewReader(body))
		req.Header.Set("X-Auth", signed)
		req.Header.Set("Content-Type", "application/offset+octet-stream")
		req.Header.Set("Upload-Offset", "0")
		rec := httptest.NewRecorder()
		patch.ServeHTTP(rec, req)
		return rec
	}

	// Create an upload that declares only 5 bytes.
	reqPost, _ := http.NewRequest(http.MethodPost, "/file.txt", http.NoBody)
	reqPost.Header.Set("X-Auth", signed)
	reqPost.Header.Set("Upload-Length", "5")
	recPost := httptest.NewRecorder()
	post.ServeHTTP(recPost, reqPost)
	if recPost.Code != http.StatusCreated {
		t.Fatalf("POST expected 201, got %d body=%q", recPost.Code, recPost.Body.String())
	}

	// A body far larger than the declared length must be rejected, and nothing
	// beyond the declared length may reach disk.
	if rec := patchReq(strings.Repeat("A", 5000)); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("over-length PATCH expected 413, got %d body=%q", rec.Code, rec.Body.String())
	}
	up, ok := cache.get("1:/file.txt")
	if !ok {
		t.Fatal("the rejected chunk ended the upload")
	}
	if offset, err := up.offset(); err != nil || offset > 5 {
		t.Fatalf("staged %d bytes (%v) despite Upload-Length 5", offset, err)
	}
	if _, err := os.Stat(filepath.Join(userScope, "file.txt")); !os.IsNotExist(err) {
		t.Fatalf("an unfinished upload reached its destination: %v", err)
	}

	// A correctly-sized upload still completes.
	if rec := patchReq("hello"); rec.Code != http.StatusNoContent {
		t.Fatalf("valid PATCH expected 204, got %d body=%q", rec.Code, rec.Body.String())
	}
	if data, _ := os.ReadFile(filepath.Join(userScope, "file.txt")); string(data) != "hello" {
		t.Fatalf("expected file content \"hello\", got %q", string(data))
	}
}
