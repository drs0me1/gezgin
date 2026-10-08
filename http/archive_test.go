package fbhttp

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/filebrowser/filebrowser/v2/rules"
	"github.com/filebrowser/filebrowser/v2/users"
)

// Gezgin opens archives on the server, into a folder beside them.

func zipBytes(t *testing.T, files ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for i := 0; i+1 < len(files); i += 2 {
		out, err := w.Create(files[i])
		if err != nil {
			t.Fatal(err)
		}
		if _, err = out.Write([]byte(files[i+1])); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// startArchive asks for a job and returns its status, and the job when it started.
func (e *fileEnv) startArchive(body string) (archiveJob, int) {
	e.t.Helper()
	rec := e.call(http.MethodPost, "/api/archive", body)
	var job archiveJob
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &job); err != nil {
			e.t.Fatal(err)
		}
	}
	return job, rec.Code
}

func (e *fileEnv) archiveJobs() []archiveJob {
	e.t.Helper()
	rec := e.call(http.MethodGet, "/api/archive", "")
	var jobs []archiveJob
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &jobs) != nil {
		e.t.Fatalf("listing the jobs: %d %s", rec.Code, rec.Body.String())
	}
	return jobs
}

// finish starts a job and waits for its end.
func (e *fileEnv) finish(body string) archiveJob {
	e.t.Helper()
	job, code := e.startArchive(body)
	if code != http.StatusOK || job.State != archiveRunning {
		e.t.Fatalf("starting %s: %d %+v", body, code, job)
	}
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		for _, j := range e.archiveJobs() {
			if j.ID == job.ID && j.State != archiveRunning {
				return j
			}
		}
	}
	e.t.Fatalf("the job %s did not end", job.ID)
	return archiveJob{}
}

// built lists what jobs left in the archive folder: nothing, once they ended.
func (e *fileEnv) built() []string {
	entries, _ := os.ReadDir(filepath.Join(e.root, ArchiveDir))
	names := []string{}
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func TestArchiveOpensIntoAFolderBesideIt(t *testing.T) {
	env := newFileEnv(t)
	inner := zipBytes(t, "c.txt", "C")
	env.write("medya/paket.zip", string(zipBytes(t, "a.txt", "A", "alt/b.txt", "B", "ic.zip", string(inner))), 0o644)

	job := env.finish(`{"items":["/medya/paket.zip"]}`)
	if job.State != archiveDone || job.Result != "/medya/paket" || job.Archives != 2 || job.Folder != "/medya" {
		t.Fatalf("job %+v", job)
	}
	for name, want := range map[string]string{"a.txt": "A", "alt/b.txt": "B", "ic/c.txt": "C", "ic.zip": string(inner)} {
		if got := env.read("medya/paket/" + name); got != want {
			t.Errorf("%s = %q; want %q", name, got, want)
		}
	}
	if got := env.read("medya/paket.zip"); got == "<missing>" {
		t.Error("the archive was removed")
	}

	// Again: the name is taken, so the new folder takes a number.
	if job = env.finish(`{"items":["/medya/paket.zip"]}`); job.Result != "/medya/paket (2)" {
		t.Errorf("second job: %+v", job)
	}
	if got := env.built(); len(got) != 0 {
		t.Errorf("the jobs left %v behind", got)
	}
}

func TestArchiveThatFailsLeavesNothing(t *testing.T) {
	env := newFileEnv(t)
	// A file stored as it is, with some of its content changed after its checksum was taken.
	var stored bytes.Buffer
	w := zip.NewWriter(&stored)
	out, err := w.CreateHeader(&zip.FileHeader{Name: "a.txt", Method: zip.Store})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = out.Write([]byte(strings.Repeat("A", 1000)))
	_ = w.Close()
	broken := bytes.Replace(stored.Bytes(), []byte(strings.Repeat("A", 10)), []byte(strings.Repeat("B", 10)), 1)
	env.write("bozuk.zip", string(broken), 0o644)
	env.write("kacak.zip", string(zipBytes(t, "../kacak.txt", "x")), 0o644)
	env.write("sahte.zip", "not a zip", 0o644)

	for name, code := range map[string]string{"bozuk.zip": "corrupt", "kacak.zip": "unsafe", "sahte.zip": "notArchive"} {
		job := env.finish(`{"items":["/` + name + `"]}`)
		if job.State != archiveFailed || job.Error != code {
			t.Errorf("%s: %+v; want failed with %s", name, job, code)
		}
		if got := env.read(strings.TrimSuffix(name, ".zip")); got != "<missing>" {
			t.Errorf("%s left a result", name)
		}
	}
	if got := env.read("../kacak.txt"); got != "<missing>" {
		t.Error("an entry got out of its folder")
	}
	if got := env.built(); len(got) != 0 {
		t.Errorf("the jobs left %v behind", got)
	}
}

func TestArchiveRequestsAreChecked(t *testing.T) {
	env := newFileEnv(t)
	env.write("a.zip", string(zipBytes(t, "a.txt", "A")), 0o644)
	env.write("b/b.zip", string(zipBytes(t, "b.txt", "B")), 0o644)
	env.write("notlar.txt", "text", 0o644)
	env.write("gizli/g.zip", string(zipBytes(t, "g.txt", "G")), 0o644)
	set, err := env.st.Settings.Get()
	if err != nil {
		t.Fatal(err)
	}
	set.Rules = append(set.Rules, rules.Rule{Path: "/gizli", Allow: false})
	if err = env.st.Settings.Save(set); err != nil {
		t.Fatal(err)
	}

	reader, _ := env.user("okuyucu", "okuyucu-parola-1", "/", users.Permissions{Download: true})
	for body, want := range map[string]int{
		`{"items":[]}`:                       http.StatusBadRequest,
		`{"items":["/notlar.txt"]}`:          http.StatusBadRequest,
		`{"items":["/a.zip","/b/b.zip"]}`:    http.StatusBadRequest,
		`{"items":["/yok.zip"]}`:             http.StatusNotFound,
		`{"items":["/gizli/g.zip"]}`:         http.StatusForbidden,
		`{"items":["/.gezgin-arsiv/x.zip"]}`: http.StatusForbidden,
		`not json`:                           http.StatusBadRequest,
	} {
		if _, code := env.startArchive(body); code != want {
			t.Errorf("%s: %d; want %d", body, code, want)
		}
	}
	if _, code := reader.startArchive(`{"items":["/a.zip"]}`); code != http.StatusForbidden {
		t.Errorf("a user who may not create: %d; want 403", code)
	}

	// One job at a time.
	env.archives.mu.Lock()
	env.archives.busy = true
	env.archives.mu.Unlock()
	if _, code := env.startArchive(`{"items":["/a.zip"]}`); code != http.StatusConflict {
		t.Errorf("while a job runs: %d; want 409", code)
	}
	env.archives.mu.Lock()
	env.archives.busy = false
	env.archives.mu.Unlock()

	// A user's scope holds: "/../a.zip" is their own "/a.zip".
	scoped, _ := env.user("kapsamli", "kapsamli-parola-1", "/b", users.Permissions{Create: true, Download: true})
	if _, code := scoped.startArchive(`{"items":["/../a.zip"]}`); code != http.StatusNotFound {
		t.Errorf("reaching out of the scope: %d; want 404", code)
	}
	if job := scoped.finish(`{"items":["/b.zip"]}`); job.State != archiveDone || env.read("b/b/b.txt") != "B" {
		t.Errorf("in the scope: %+v", job)
	}
}

func TestArchiveRulesHoldForTheResult(t *testing.T) {
	env := newFileEnv(t)
	env.write("kurulum.zip", string(zipBytes(t, "beni-oku.txt", "R", "setup.exe", "MZ")), 0o644)
	set, err := env.st.Settings.Get()
	if err != nil {
		t.Fatal(err)
	}
	set.Rules = append(set.Rules, rules.Rule{Regex: true, Allow: false, Regexp: &rules.Regexp{Raw: `\.exe$`}})
	if err = env.st.Settings.Save(set); err != nil {
		t.Fatal(err)
	}
	if job := env.finish(`{"items":["/kurulum.zip"]}`); job.State != archiveFailed || job.Error != "rules" {
		t.Errorf("job %+v; want failed with rules", job)
	}
	if got := env.read("kurulum/beni-oku.txt"); got != "<missing>" {
		t.Error("a result was put in place")
	}
}

func TestArchiveJobsBelongToTheirUser(t *testing.T) {
	env := newFileEnv(t)
	env.write("a.zip", string(zipBytes(t, "a.txt", "A")), 0o644)
	done := env.finish(`{"items":["/a.zip"]}`)

	other, _ := env.user("diger", "diger-parola-1", "/", users.Permissions{Create: true, Download: true})
	if jobs := other.archiveJobs(); len(jobs) != 0 {
		t.Errorf("another user lists %+v", jobs)
	}
	if rec := other.call(http.MethodDelete, "/api/archive/"+done.ID, ""); rec.Code != http.StatusNotFound {
		t.Errorf("another user cancelling: %d; want 404", rec.Code)
	}
	if rec := env.call(http.MethodDelete, "/api/archive/"+done.ID, ""); rec.Code != http.StatusConflict {
		t.Errorf("cancelling an ended job: %d; want 409", rec.Code)
	}

	// A running job is cancelled by its owner.
	cancelled := false
	running := &archiveJob{ID: "abc123", State: archiveRunning, cancel: func() { cancelled = true }}
	root, err := env.st.Users.Get(env.root, false, "root")
	if err != nil {
		t.Fatal(err)
	}
	running.user = root.ID
	env.archives.mu.Lock()
	env.archives.jobs = append([]*archiveJob{running}, env.archives.jobs...)
	env.archives.mu.Unlock()
	if rec := env.call(http.MethodDelete, "/api/archive/abc123", ""); rec.Code != http.StatusNoContent || !cancelled {
		t.Errorf("cancelling a running job: %d, cancelled %v", rec.Code, cancelled)
	}
}

func TestArchivePassword(t *testing.T) {
	env := newFileEnv(t)
	// A 7z made with py7zr holding belge.txt and klasor/ic.txt, encrypted with "gizli-parola".
	data, err := base64.StdEncoding.DecodeString("N3q8ryccAAQRccvcswAAAAAAAAAWAAAAAAAAACiZHEydT+kUG3KdGrsEyN6F6L8obKoiIHlomozDKQL9KJsrguAAnACLXQAAgTMHrg/QDv4UYPP1fi0hQVztibum0nUv7ZHOJREdc82263cOLGg4nuqxXRnLifXpgmQeYJiBCTyoT7pfv+n5Nzl3lqZMLnpc6/qL3AgXnUuAZEJzxRZX9kQLGys88+2jyc6Wr2dfoSVEmjFFaMMa5eFyXZewXKwC8yzo8BeaLlquwmy6eIQAAAAAABcGIAEJgJMABwsBAAEhIQEYDICdAAA=")
	if err != nil {
		t.Fatal(err)
	}
	env.write("gizli.7z", string(data), 0o644)

	if job := env.finish(`{"items":["/gizli.7z"]}`); job.State != archiveFailed || job.Error != "password" {
		t.Errorf("without a password: %+v", job)
	}
	if job := env.finish(`{"items":["/gizli.7z"],"password":"yanlis"}`); job.State != archiveFailed || job.Error != "wrongPassword" {
		t.Errorf("with a wrong password: %+v", job)
	}
	if job := env.finish(`{"items":["/gizli.7z"],"password":"gizli-parola"}`); job.State != archiveDone {
		t.Errorf("with the password: %+v", job)
	}
	if got := env.read("gizli/belge.txt"); got != "7z icinden merhaba\n" {
		t.Errorf("belge.txt = %q", got)
	}
	if rec := env.call(http.MethodGet, "/api/archive", ""); strings.Contains(rec.Body.String(), "gizli-parola") {
		t.Error("the listing shows the password")
	}
}

func TestArchiveFolderIsReserved(t *testing.T) {
	env := newFileEnv(t)
	env.write(ArchiveDir+"/kalinti/x.txt", "x", 0o644)
	for _, target := range []string{"/api/resources/" + ArchiveDir + "/", "/api/raw/" + ArchiveDir + "/kalinti/x.txt"} {
		if rec := env.call(http.MethodGet, target, ""); rec.Code == http.StatusOK {
			t.Errorf("GET %s: %d", target, rec.Code)
		}
	}
	// A starting server clears what a stopped one left.
	NewArchiveJobs(filepath.Join(env.root, ArchiveDir)).Close()
	if _, err := os.Stat(filepath.Join(env.root, ArchiveDir)); !os.IsNotExist(err) {
		t.Errorf("the archive folder: %v", err)
	}
}

func TestArchiveJobsAreKeptToAFew(t *testing.T) {
	env := newFileEnv(t)
	env.write("a.zip", string(zipBytes(t, "a.txt", "A")), 0o644)
	for i := 0; i < archiveHistory+2; i++ {
		if job := env.finish(`{"items":["/a.zip"]}`); job.State != archiveDone {
			t.Fatalf("job %d: %+v", i, job)
		}
	}
	jobs := env.archiveJobs()
	if len(jobs) != archiveHistory || jobs[0].Result != fmt.Sprintf("/a (%d)", archiveHistory+2) {
		t.Errorf("%d jobs listed, the latest %+v", len(jobs), jobs[0])
	}
}
