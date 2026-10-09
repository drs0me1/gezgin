package fbhttp

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/filebrowser/filebrowser/v2/rules"
	"github.com/filebrowser/filebrowser/v2/users"
)

// Gezgin makes archives on the server, beside what they hold.

// zipEntries lists a ZIP's entries as name=content, folders as name/.
func zipEntries(t *testing.T, data []byte) []string {
	t.Helper()
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range r.File {
		if strings.HasSuffix(f.Name, "/") {
			got = append(got, f.Name)
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, f.Name+"="+string(content))
	}
	sort.Strings(got)
	return got
}

func sameList(t *testing.T, what string, got []string, want ...string) {
	t.Helper()
	sort.Strings(want)
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("%s:\n got %v\nwant %v", what, got, want)
	}
}

func TestArchiveCreateBesideItsItems(t *testing.T) {
	env := newFileEnv(t)
	env.write("medya/Tatil/notlar.txt", "notlar", 0o644)
	env.write("medya/Tatil/alt/film.mkv", "film", 0o644)

	// One folder: what it holds goes at the archive's root.
	job := env.finish(`{"kind":"create","items":["/medya/Tatil"],"name":"Tatil","format":"zip"}`)
	if job.State != archiveDone || job.Kind != archiveCreate || job.Result != "/medya/Tatil.zip" || job.Format != "zip" ||
		job.Files != 3 || job.Entries != 3 || job.Total != 10 || job.Bytes != 10 || job.Volumes != 1 || job.Skipped != 0 {
		t.Fatalf("job %+v", job)
	}
	sameList(t, "Tatil.zip", zipEntries(t, []byte(env.read("medya/Tatil.zip"))), "notlar.txt=notlar", "alt/", "alt/film.mkv=film")
	if info, _ := os.Stat(filepath.Join(env.root, "medya/Tatil.zip")); info.Mode().Perm() != 0o640 {
		t.Errorf("mode %v", info.Mode())
	}

	// Again, with a typed extension: the name takes a number.
	if job = env.finish(`{"kind":"create","items":["/medya/Tatil"],"name":"Tatil.ZIP","format":"zip"}`); job.Result != "/medya/Tatil (2).zip" {
		t.Errorf("second job %+v", job)
	}

	// Several items go under their names; a tar.gz opens with "Arşivi aç".
	env.write("medya/a.txt", "A", 0o644)
	job = env.finish(`{"kind":"create","items":["/medya/Tatil","/medya/a.txt"],"name":"hepsi","format":"targz","zone":"Europe/Istanbul"}`)
	if job.State != archiveDone || job.Result != "/medya/hepsi.tar.gz" {
		t.Fatalf("tar.gz job %+v", job)
	}
	if job = env.finish(`{"items":["/medya/hepsi.tar.gz"]}`); job.State != archiveDone || env.read("medya/hepsi/Tatil/alt/film.mkv") != "film" ||
		env.read("medya/hepsi/a.txt") != "A" {
		t.Errorf("opening it: %+v", job)
	}
	if got := env.built(); len(got) != 0 {
		t.Errorf("the jobs left %v behind", got)
	}
}

func TestArchiveCreateInVolumes(t *testing.T) {
	env := newFileEnv(t)
	film := make([]byte, 2*archiveMinVolume+1000)
	_, _ = rand.Read(film)
	env.write("medya/Tatil/film.mkv", string(film), 0o644)
	env.write("medya/Eski.zip.004", "a volume of another set", 0o644)

	body := `{"kind":"create","items":["/medya/Tatil"],"name":"Tatil","format":"zip","volume":1048576}`
	job := env.finish(body)
	if job.State != archiveDone || job.Result != "/medya/Tatil.zip.001" || job.Volumes != 3 {
		t.Fatalf("job %+v", job)
	}
	var whole []byte
	for _, n := range []string{"001", "002", "003"} {
		data := env.read("medya/Tatil.zip." + n)
		if n != "003" && len(data) != archiveMinVolume {
			t.Errorf(".%s has %d bytes", n, len(data))
		}
		whole = append(whole, data...)
	}
	sameList(t, "the volumes", zipEntries(t, whole), "film.mkv="+string(film))

	// The next set takes a number; so does one whose name has a volume of another set.
	if job = env.finish(body); job.Result != "/medya/Tatil (2).zip.001" {
		t.Errorf("second set %+v", job)
	}
	if job = env.finish(strings.Replace(body, `"name":"Tatil"`, `"name":"Eski"`, 1)); job.Result != "/medya/Eski (2).zip.001" {
		t.Errorf("beside a stale volume %+v", job)
	}

	// Any volume opens the set.
	if job = env.finish(`{"items":["/medya/Tatil.zip.002"]}`); job.State != archiveDone || env.read(strings.TrimPrefix(job.Result, "/")+"/film.mkv") != string(film) {
		t.Errorf("opening the set %+v", job)
	}
}

func TestArchiveCreateLeavesOut(t *testing.T) {
	env := newFileEnv(t)
	env.write("medya/k/izinli.txt", "ok", 0o644)
	env.write("medya/k/gizli.txt", "secret", 0o644)
	env.write("medya/k/.gezgin-1a2b.tmp", "half written", 0o644)
	env.write(".gezgin-cop/1/cop.txt", "someone's trash", 0o644)
	if err := syscall.Mkfifo(filepath.Join(env.root, "medya/k/boru"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A link with an ordinary name into Gezgin's own folders.
	if err := os.Symlink(filepath.Join(env.root, ".gezgin-cop"), filepath.Join(env.root, "medya/k/cop")); err != nil {
		t.Fatal(err)
	}
	set, err := env.st.Settings.Get()
	if err != nil {
		t.Fatal(err)
	}
	set.Rules = append(set.Rules, rules.Rule{Regex: true, Allow: false, Regexp: &rules.Regexp{Raw: `gizli|yasak\.zip$`}})
	if err = env.st.Settings.Save(set); err != nil {
		t.Fatal(err)
	}

	job := env.finish(`{"kind":"create","items":["/medya/k"],"name":"k","format":"zip"}`)
	if job.State != archiveDone || job.Skipped != 3 {
		t.Fatalf("job %+v", job)
	}
	sameList(t, "k.zip", zipEntries(t, []byte(env.read("medya/k.zip"))), "izinli.txt=ok")

	// A name the rules refuse is refused at the start.
	if _, code := env.startArchive(`{"kind":"create","items":["/medya/k"],"name":"yasak","format":"zip"}`); code != http.StatusForbidden {
		t.Errorf("a refused name: %d", code)
	}
	// Downloads leave out the same.
	rec := env.call(http.MethodGet, "/api/raw/medya/k?algo=zip", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("download: %d", rec.Code)
	}
	sameList(t, "the download", zipEntries(t, rec.Body.Bytes()), "izinli.txt=ok")
}

func TestArchiveCreateRequestsAreChecked(t *testing.T) {
	env := newFileEnv(t)
	env.write("medya/a.txt", "A", 0o644)
	env.write("b/b.txt", "B", 0o644)
	const item = `"items":["/medya/a.txt"]`
	for body, want := range map[string]int{
		`{"kind":"create",` + item + `,"name":"a","format":"rar"}`:                        http.StatusBadRequest,
		`{"kind":"create",` + item + `,"name":" ","format":"zip"}`:                        http.StatusBadRequest,
		`{"kind":"create",` + item + `,"name":"a/b","format":"zip"}`:                      http.StatusBadRequest,
		`{"kind":"create",` + item + `,"name":"a\\b","format":"zip"}`:                     http.StatusBadRequest,
		`{"kind":"create",` + item + `,"name":".gezgin-x","format":"zip"}`:                http.StatusBadRequest,
		`{"kind":"create",` + item + `,"name":"a","format":"zip","volume":1000}`:          http.StatusBadRequest,
		`{"kind":"create","items":["/medya/a.txt","/b/b.txt"],"name":"a","format":"zip"}`: http.StatusBadRequest,
		`{"kind":"create","items":["/"],"name":"a","format":"zip"}`:                       http.StatusBadRequest,
		`{"kind":"pack",` + item + `,"name":"a","format":"zip"}`:                          http.StatusBadRequest,
		`{"kind":"create","items":["/medya/yok"],"name":"a","format":"zip"}`:              http.StatusNotFound,
	} {
		if _, code := env.startArchive(body); code != want {
			t.Errorf("%s: %d; want %d", body, code, want)
		}
	}
	// An archive holds the content of the files: it takes the download permission too.
	creator, _ := env.user("yazar", "yazar-parola-1", "/", users.Permissions{Create: true})
	if _, code := creator.startArchive(`{"kind":"create",` + item + `,"name":"a","format":"zip"}`); code != http.StatusForbidden {
		t.Errorf("without the download permission: %d", code)
	}
	if job := env.finish(`{"kind":"create",` + item + `,"name":"a","format":"tar"}`); job.State != archiveDone || job.Result != "/medya/a.tar" {
		t.Errorf("job %+v", job)
	}
}

func TestDownloadsAreCompressedAndDoNotHang(t *testing.T) {
	env := newFileEnv(t)
	env.write("medya/notlar.txt", strings.Repeat("notlar ", 1000), 0o644)
	env.write("medya/film.mkv", "film", 0o644)
	if err := syscall.Mkfifo(filepath.Join(env.root, "medya/boru"), 0o644); err != nil {
		t.Fatal(err)
	}
	done := make(chan []byte)
	go func() {
		rec := env.call(http.MethodGet, "/api/raw/medya?algo=zip&zone=Europe/Istanbul", "")
		done <- rec.Body.Bytes()
	}()
	var data []byte
	select {
	case data = <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("the download hangs on a FIFO")
	}
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	methods := map[string]uint16{}
	for _, f := range r.File {
		methods[f.Name] = f.Method
	}
	if len(methods) != 2 || methods["notlar.txt"] != zip.Deflate || methods["film.mkv"] != zip.Store {
		t.Errorf("entries %v", methods)
	}
}

func TestSharedFolderDownloads(t *testing.T) {
	env := newShareEnv(t)
	env.write("docs/k/a.txt", "A", 0o644)
	env.write("docs/k/alt/b.txt", "B", 0o644)
	env.write("docs/gizli.txt", "outside the share", 0o644)
	// A link in the shared folder to a file of the owner's outside it.
	if err := os.Symlink("../gizli.txt", filepath.Join(env.root, "docs/k/bag.txt")); err != nil {
		t.Fatal(err)
	}
	hash := env.mustShare("/docs/k", `{}`)
	for target, want := range map[string][]string{
		"/api/public/dl/" + hash + "?algo=zip":     {"a.txt=A", "alt/", "alt/b.txt=B"},
		"/api/public/dl/" + hash + "/alt?algo=zip": {"b.txt=B"},
	} {
		rec := env.call(http.MethodGet, target, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", target, rec.Code, rec.Body.String())
		}
		sameList(t, target, zipEntries(t, rec.Body.Bytes()), want...)
	}
}
