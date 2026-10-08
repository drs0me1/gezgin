package fbhttp

import (
	"net/http"
	"testing"

	"github.com/filebrowser/filebrowser/v2/users"
)

// A user without the download permission is refused every read of a file's content (Gezgin: 403
// instead of an empty 202).
func TestContentNeedsDownloadPermission(t *testing.T) {
	env := newFileEnv(t)
	env.write("a.txt", "secret", 0o644)
	env.write("a.srt", "1\n00:00:01,000 --> 00:00:02,000\nsecret\n", 0o644)
	env.write("a.jpg", "not really an image", 0o644)
	hash, err := users.HashPwd("viewer-pass")
	if err != nil {
		t.Fatal(err)
	}
	if err := env.st.Users.Save(&users.User{Username: "viewer", Password: hash, Scope: ".", Perm: users.Permissions{}}); err != nil {
		t.Fatal(err)
	}
	env.token = env.login("viewer", "viewer-pass")

	for _, target := range []string{
		"/api/raw/a.txt",
		"/api/raw/a.txt?inline=true",
		"/api/raw/?files=a.txt&algo=zip",
		"/api/preview/thumb/a.jpg",
		"/api/subtitle/a.srt",
		"/api/resources/a.txt?checksum=sha256",
	} {
		if rec := env.call(http.MethodGet, target, ""); rec.Code != http.StatusForbidden {
			t.Errorf("GET %s: %d, want 403", target, rec.Code)
		}
	}

	if rec := env.callWith(http.MethodGet, "/api/resources/a.txt", map[string]string{"X-Encoding": "true"}, ""); rec.Code != http.StatusForbidden {
		t.Errorf("GET the raw text: %d, want 403", rec.Code)
	}
}
