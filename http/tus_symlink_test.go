package fbhttp

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/asdine/storm/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/afero"

	"github.com/filebrowser/filebrowser/v2/diskcache"
	"github.com/filebrowser/filebrowser/v2/files"
	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/storage/bolt"
	"github.com/filebrowser/filebrowser/v2/users"
)

// Reproduces the TUS write vector of GHSA-v9g6-9pp4-3w22: a scoped user must
// not be able to create or write files through a symlinked directory that
// escapes their scope.
func TestTusHandlersRejectSymlinkScopeEscape(t *testing.T) {
	root := t.TempDir()
	userScope := filepath.Join(root, "user")
	outside := filepath.Join(root, "otheruser")
	for _, d := range []string{userScope, outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// A directory symlink inside the user's scope pointing outside it.
	if err := os.Symlink(outside, filepath.Join(userScope, "escape_link")); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	key := []byte("test-signing-key")

	db, err := storm.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	st, err := bolt.NewStorage(db)
	if err != nil {
		t.Fatalf("failed to get storage: %v", err)
	}
	if err := st.Users.Save(&users.User{
		Username: "u",
		Password: "pw",
		Perm:     users.Permissions{Create: true, Modify: true},
	}); err != nil {
		t.Fatalf("failed to save user: %v", err)
	}
	if err := st.Settings.Save(&settings.Settings{Key: key}); err != nil {
		t.Fatalf("failed to save settings: %v", err)
	}
	st.Users = &customFSUser{
		Store: st.Users,
		fs:    files.NewScopedFs(afero.NewOsFs(), userScope),
	}

	// Forge a valid auth token for user ID 1.
	claims := &authToken{
		User: userInfo{ID: 1, Username: "u", Perm: users.Permissions{Create: true, Modify: true}},
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-time.Minute)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(key)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	cache := newUploadCache(filepath.Join(root, UploadsDir), uploadCacheTTL)
	t.Cleanup(cache.Close)
	send := func(method, target string, headers map[string]string, body string) int {
		req, err := http.NewRequest(method, target, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Auth", signed)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		recorder := httptest.NewRecorder()
		var handler handleFunc
		switch method {
		case http.MethodPost:
			handler = tusPostHandler(cache, diskcache.NewNoOp())
		case http.MethodPatch:
			handler = tusPatchHandler(cache, diskcache.NewNoOp())
		}
		handle(handler, "", st, &settings.Server{}).ServeHTTP(recorder, req)
		return recorder.Code
	}
	chunk := map[string]string{"Content-Type": "application/offset+octet-stream", "Upload-Offset": "0"}
	escaped := func(name string) {
		t.Helper()
		if _, statErr := os.Stat(filepath.Join(outside, name)); statErr == nil {
			t.Errorf("VULNERABLE: file was created outside the user's scope")
		}
	}

	t.Run("POST create through symlinked dir", func(t *testing.T) {
		if code := send(http.MethodPost, "escape_link/injected.txt", map[string]string{"Upload-Length": "20"}, ""); code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", code)
		}
		escaped("injected.txt")
	})

	// Gezgin stages the data, so a chunk can only reach a destination through an
	// upload that was started; the destination is checked again when it is put
	// in place.
	t.Run("PATCH write through symlinked dir", func(t *testing.T) {
		if code := send(http.MethodPatch, "escape_link/injected.txt", chunk, "01234567890123456789"); code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", code)
		}
		escaped("injected.txt")
	})

	t.Run("folder swapped for an escaping link during the upload", func(t *testing.T) {
		moved := filepath.Join(userScope, "moved")
		if err := os.MkdirAll(moved, 0o755); err != nil {
			t.Fatal(err)
		}
		if code := send(http.MethodPost, "moved/swapped.txt", map[string]string{"Upload-Length": "20"}, ""); code != http.StatusCreated {
			t.Fatalf("POST: expected 201, got %d", code)
		}
		if err := os.RemoveAll(moved); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, moved); err != nil {
			t.Fatal(err)
		}
		if code := send(http.MethodPatch, "moved/swapped.txt", chunk, "01234567890123456789"); code != http.StatusForbidden {
			t.Errorf("PATCH: expected 403, got %d", code)
		}
		escaped("swapped.txt")
		if entries, _ := os.ReadDir(cache.dir); len(entries) != 0 {
			t.Errorf("the refused upload left its data: %v", entries)
		}
	})
}
