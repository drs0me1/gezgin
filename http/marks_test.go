package fbhttp

import (
	"bufio"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/filebrowser/filebrowser/v2/files"
	"github.com/filebrowser/filebrowser/v2/users"
)

// marks returns the marks of a folder's items by name, as the user lists it.
func (e *fileEnv) marks(folder string) map[string]files.FileInfo {
	e.t.Helper()
	rec := e.call(http.MethodGet, "/api/resources"+folder, "")
	if rec.Code != http.StatusOK {
		e.t.Fatalf("listing %s = %d", folder, rec.Code)
	}
	var listing files.FileInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &listing); err != nil {
		e.t.Fatal(err)
	}
	items := map[string]files.FileInfo{}
	for _, item := range listing.Items {
		items[item.Name] = *item
	}
	return items
}

// K114-K116: a listing marks the user's favourites and the items with a share in force: a
// user's own shares, every user's for an admin.
func TestListingMarks(t *testing.T) {
	env, _ := newDavEnv(t)
	ali, _ := env.user("ali", "ali-password-1", "/ali", users.Permissions{Share: true, Download: true})
	veli, _ := env.user("veli", "veli-password-1", "/", users.Permissions{Share: true, Download: true})
	env.write("notlar.txt", "not", 0o644)
	env.write("eski.txt", "eski", 0o644)
	env.write("ortak/a.txt", "a", 0o644)
	env.write("ali/tatil/a.jpg", "jpeg", 0o644)

	if res := env.favorite(http.MethodPost, "/notlar.txt"); res.StatusCode != http.StatusOK {
		t.Fatalf("favourite = %d", res.StatusCode)
	}
	env.mustShare("/notlar.txt", "{}")
	env.mustShare("/notlar.txt", `{"expires":"2","unit":"days"}`)
	env.davShare("/ortak", false)
	ali.mustShare("/tatil", "{}")
	// A share past its end marks nothing.
	old := env.mustShare("/eski.txt", "{}")
	link, err := env.st.Share.GetByHash(old)
	if err != nil {
		t.Fatal(err)
	}
	link.Expire = time.Now().Add(-time.Minute).Unix()
	if err := env.st.Share.Save(link); err != nil {
		t.Fatal(err)
	}

	top := env.marks("/")
	if n := top["notlar.txt"]; !n.Favorite || n.SharedLinks != 2 || n.SharedDAV != 0 {
		t.Errorf("notlar.txt: favourite %t, %d links, %d WebDAV; want a favourite with 2 links", n.Favorite, n.SharedLinks, n.SharedDAV)
	}
	if o := top["ortak"]; o.Favorite || o.SharedLinks != 0 || o.SharedDAV != 1 {
		t.Errorf("ortak: %+v; want one WebDAV share", o)
	}
	if e := top["eski.txt"]; e.SharedLinks != 0 {
		t.Errorf("an expired share still marks eski.txt")
	}
	if a := env.marks("/ali/")["tatil"]; a.SharedLinks != 1 {
		t.Errorf("the admin does not see ali's share of tatil: %+v", a)
	}

	if a := ali.marks("/")["tatil"]; a.SharedLinks != 1 || a.Favorite {
		t.Errorf("ali's own share of tatil: %+v", a)
	}
	// Another user sees neither the admin's favourites nor shares that are not theirs.
	if n := veli.marks("/")["notlar.txt"]; n.Favorite || n.SharedLinks != 0 || n.SharedDAV != 0 {
		t.Errorf("veli sees others' marks on notlar.txt: %+v", n)
	}
	if a := veli.marks("/ali/")["tatil"]; a.SharedLinks != 0 {
		t.Errorf("veli sees ali's share of tatil")
	}
}

// K111: a search result carries what the folder view shows: name, size, time, type and marks.
func TestSearchResultsDescribeTheirItems(t *testing.T) {
	env := newShareEnv(t)
	env.write("belgeler/rapor-2024.pdf", "%PDF", 0o644)
	env.write("belgeler/rapor-eski/not.txt", "x", 0o644)
	if res := env.favorite(http.MethodPost, "/belgeler/rapor-2024.pdf"); res.StatusCode != http.StatusOK {
		t.Fatalf("favourite = %d", res.StatusCode)
	}
	env.mustShare("/belgeler/rapor-eski", "{}")

	rec := env.call(http.MethodGet, "/api/search/belgeler/?query=rapor", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("search = %d %q", rec.Code, rec.Body.String())
	}
	found := map[string]map[string]any{}
	scanner := bufio.NewScanner(strings.NewReader(rec.Body.String()))
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" {
			var item map[string]any
			if err := json.Unmarshal([]byte(line), &item); err != nil {
				t.Fatalf("result %q: %v", line, err)
			}
			found[item["name"].(string)] = item
		}
	}
	pdf, dir := found["rapor-2024.pdf"], found["rapor-eski"]
	if pdf == nil || dir == nil || len(found) != 2 {
		t.Fatalf("results %v; want the PDF and the folder", found)
	}
	if pdf["path"] != "rapor-2024.pdf" || pdf["isDir"] != false || pdf["size"] != float64(4) ||
		pdf["type"] != "pdf" || pdf["favorite"] != true || pdf["modified"] == nil {
		t.Errorf("the PDF: %v", pdf)
	}
	if dir["isDir"] != true || dir["dir"] != true || dir["sharedLinks"] != float64(1) || dir["size"] != nil {
		t.Errorf("the folder: %v", dir)
	}
}
