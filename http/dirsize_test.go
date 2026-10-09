package fbhttp

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/filebrowser/filebrowser/v2/files"
	"github.com/filebrowser/filebrowser/v2/rules"
	"github.com/filebrowser/filebrowser/v2/users"
)

func folderFacts(t *testing.T, body []byte, name string) *files.FileInfo {
	t.Helper()
	var dir files.FileInfo
	if err := json.Unmarshal(body, &dir); err != nil {
		t.Fatalf("listing %q: %v", body, err)
	}
	for _, item := range dir.Items {
		if item.Name == name {
			return item
		}
	}
	t.Fatalf("no %s in %q", name, body)
	return nil
}

// K84-K86: asked with ?sizes=true, a listing gives its folders the count a user sees and the
// size of what the rules let them reach, hidden files included; a share page too.
func TestListingSizesThroughTheAPI(t *testing.T) {
	env := newFileEnv(t)
	ali, aliUser := env.user("ali", "ali-password-1", "/ali", users.Permissions{Share: true, Download: true})
	env.write("ali/albüm/a.jpg", "0123456789", 0o644)
	env.write("ali/albüm/.gizli", "12345", 0o644)
	env.write("ali/albüm/yasak/b.jpg", "x", 0o644)
	env.write("ali/albüm/2024/c.jpg", "01234567890123456789", 0o644)
	aliUser.HideDotfiles = true
	aliUser.Rules = []rules.Rule{{Path: "/albüm/yasak"}}
	if err := env.st.Users.Update(aliUser, "HideDotfiles", "Rules"); err != nil {
		t.Fatal(err)
	}

	rec := ali.call(http.MethodGet, "/api/resources/?sizes=true", "")
	if album := folderFacts(t, rec.Body.Bytes(), "albüm"); album.Count == nil || *album.Count != 2 ||
		album.Size != 35 || album.SizeUnknown {
		t.Errorf("albüm: %+v; want 2 items (a.jpg, 2024) and 35 bytes (with .gizli, without yasak)", album)
	}
	if album := folderFacts(t, ali.call(http.MethodGet, "/api/resources/", "").Body.Bytes(), "albüm"); album.Count != nil {
		t.Errorf("albüm without ?sizes: %+v; want no count", album)
	}

	hash := ali.mustShare("/albüm", "{}")
	rec = env.call(http.MethodGet, "/api/public/share/"+hash+"?sizes=true", "")
	if year := folderFacts(t, rec.Body.Bytes(), "2024"); year.Count == nil || *year.Count != 1 || year.Size != 20 {
		t.Errorf("2024 on the share page: %+v; want 1 item and 20 bytes", year)
	}
}
