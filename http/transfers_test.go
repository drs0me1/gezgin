package fbhttp

import (
	"archive/zip"
	"bytes"
	"net/http"
	"strings"
	"testing"
)

// fill takes every place of key, as transfers under way would, and returns what gives them back.
func fill(t *testing.T, key string) func() {
	t.Helper()
	for i := 0; i < transferLimit; i++ {
		if !running.begin(key) {
			t.Fatalf("%s: place %d taken", key, i+1)
		}
	}
	released := false
	release := func() {
		if released {
			return
		}
		released = true
		for i := 0; i < transferLimit; i++ {
			running.end(key)
		}
	}
	t.Cleanup(release)
	return release
}

func TestTransfersAreCountedByKey(t *testing.T) {
	release := fill(t, "a")
	if running.begin("a") {
		t.Fatal("a place over the limit")
	}
	if !running.begin("b") {
		t.Fatal("another key has no place")
	}
	running.end("b")
	release()
	if !running.begin("a") {
		t.Fatal("no place once the transfers ended")
	}
	running.end("a")
	if len(running.count) != 0 {
		t.Errorf("counts left: %v", running.count)
	}
}

func TestDownloadsAreLimited(t *testing.T) {
	env := newShareEnv(t)
	env.write("a.txt", "A", 0o644)
	root, err := env.st.Users.Get(env.root, false, "root")
	if err != nil {
		t.Fatal(err)
	}

	release := fill(t, userDownloads(root.ID))
	rec := env.call(http.MethodGet, "/api/raw/a.txt", "")
	if rec.Code != http.StatusTooManyRequests || !strings.Contains(rec.Body.String(), "en fazla 10 indirme") {
		t.Errorf("an eleventh download: %d %q", rec.Code, rec.Body.String())
	}
	// Previews are not counted.
	if rec := env.call(http.MethodGet, "/api/resources/a.txt", ""); rec.Code != http.StatusOK {
		t.Errorf("the file's page with every download place taken: %d", rec.Code)
	}
	release()
	if rec := env.call(http.MethodGet, "/api/raw/a.txt", ""); rec.Code != http.StatusOK || rec.Body.String() != "A" {
		t.Errorf("once a download ended: %d %q", rec.Code, rec.Body.String())
	}

	// A share link's downloads are counted by link, whoever makes them.
	hash := env.mustShare("/a.txt", "{}")
	release = fill(t, linkDownloads(hash))
	if code, body := env.download(hash); code != http.StatusTooManyRequests || !strings.Contains(body, "en fazla 10 indirme") {
		t.Errorf("an eleventh download through the link: %d %q", code, body)
	}
	if rec := env.call(http.MethodGet, "/api/raw/a.txt", ""); rec.Code != http.StatusOK {
		t.Errorf("the owner's own download while the link's places are taken: %d", rec.Code)
	}
	release()
	if code, body := env.download(hash); code != http.StatusOK || body != "A" {
		t.Errorf("once a link download ended: %d %q", code, body)
	}
}

func TestUploadsAreLimited(t *testing.T) {
	env := newFileEnv(t)
	root, err := env.st.Users.Get(env.root, false, "root")
	if err != nil {
		t.Fatal(err)
	}

	release := fill(t, userUploads(root.ID))
	rec := env.call(http.MethodPatch, "/api/tus/a.bin", "chunk")
	if rec.Code != http.StatusTooManyRequests || !strings.Contains(rec.Body.String(), "en fazla 10 yükleme") {
		t.Errorf("an eleventh upload: %d %q", rec.Code, rec.Body.String())
	}
	release()
	// No upload was begun, so the chunk is refused for that, not for the limit.
	if rec := env.call(http.MethodPatch, "/api/tus/a.bin", "chunk"); rec.Code == http.StatusTooManyRequests {
		t.Errorf("once an upload ended: %d", rec.Code)
	}
}

func TestWebDAVTransfersAreLimited(t *testing.T) {
	env, dav := newDavEnv(t)
	env.write("docs/a.txt", "a", 0o644)
	hash := env.davShare("/docs", true)
	c := &davClient{t: t, base: dav.URL + "/" + hash, user: "webdav-user", pass: "webdav-password"}

	release := fill(t, davTransfers(hash))
	if code := c.code(http.MethodGet, "/a.txt", nil, ""); code != http.StatusTooManyRequests {
		t.Errorf("an eleventh GET: %d", code)
	}
	if code := c.code(http.MethodPut, "/b.txt", nil, "b"); code != http.StatusTooManyRequests {
		t.Errorf("an eleventh PUT: %d", code)
	}
	// Listing is no transfer.
	if names := c.list("/"); len(names) != 1 || names[0] != "a.txt" {
		t.Errorf("listing with every place taken: %v", names)
	}
	release()
	if code := c.code(http.MethodGet, "/a.txt", nil, ""); code != http.StatusOK {
		t.Errorf("once a transfer ended: %d", code)
	}
}

func TestDownloadEntriesAreLimited(t *testing.T) {
	defer func(n int) { downloadEntries = n }(downloadEntries)
	downloadEntries = 3

	env := newFileEnv(t)
	for _, name := range []string{"k/1.txt", "k/2.txt", "k/3.txt", "k/4.txt", "j/1.txt", "j/2.txt"} {
		env.write(name, name, 0o644)
	}
	rec := env.call(http.MethodGet, "/api/raw/k?algo=zip", "")
	if rec.Code != http.StatusUnprocessableEntity || rec.Header().Get("Content-Disposition") != "" ||
		!strings.Contains(rec.Body.String(), "fazla dosya ve klasör") {
		t.Errorf("a download of 4 entries over a limit of 3: %d %q %q", rec.Code,
			rec.Header().Get("Content-Disposition"), rec.Body.String())
	}

	rec = env.call(http.MethodGet, "/api/raw/j?algo=zip", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("a download within the limit: %d %q", rec.Code, rec.Body.String())
	}
	zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil || len(zr.File) != 2 {
		t.Errorf("the ZIP within the limit: %v, %d entries", err, len(zr.File))
	}
}
