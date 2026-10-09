package pack

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/filebrowser/filebrowser/v2/unpack"
)

// dirSource is a folder on disk as a Source: refuse stands for the rules, and links leading
// out of the folder are followed only with follow.
type dirSource struct {
	root   string
	refuse func(name string) bool
	follow bool
	// opened, when set, is called as a file is opened, before it is read.
	opened func(name string)
}

func (s dirSource) full(name string) string {
	return filepath.Join(s.root, filepath.FromSlash(name))
}

func (s dirSource) Lstat(name string) (fs.FileInfo, error) { return os.Lstat(s.full(name)) }

func (s dirSource) Stat(name string) (fs.FileInfo, error) {
	if !s.follow {
		target, err := filepath.EvalSymlinks(s.full(name))
		if err != nil {
			return nil, err
		}
		root, _ := filepath.EvalSymlinks(s.root)
		if target != root && !strings.HasPrefix(target, root+string(filepath.Separator)) {
			return nil, fs.ErrPermission
		}
	}
	return os.Stat(s.full(name))
}

func (s dirSource) ReadDir(name string) ([]string, error) {
	f, err := os.Open(s.full(name))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.Readdirnames(-1)
}

func (s dirSource) Open(name string) (fs.File, error) {
	if s.opened != nil {
		s.opened(name)
	}
	return os.OpenFile(s.full(name), os.O_RDONLY|syscall.O_NONBLOCK, 0)
}

func (s dirSource) Allowed(name string, _ bool) bool {
	return s.refuse == nil || !s.refuse(name)
}

// when is the time the test files are given.
var when = time.Date(2026, 7, 14, 9, 30, 42, 0, time.UTC)

// tree makes files (name=content, or name/ for a folder) under a new folder.
func tree(t *testing.T, entries ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, e := range entries {
		name, content, file := strings.Cut(e, "=")
		p := filepath.Join(root, filepath.FromSlash(strings.TrimSuffix(name, "/")))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if file {
			if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		} else if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Folders last, so that writing into them does not change their times.
	var all []string
	_ = filepath.WalkDir(root, func(p string, _ fs.DirEntry, err error) error {
		all = append(all, p)
		return err
	})
	for i := len(all) - 1; i >= 0; i-- {
		if err := os.Chtimes(all[i], when, when); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func create(t *testing.T, src Source, items []Item, name string, opt Options) (string, []string, Progress, error) {
	t.Helper()
	out := t.TempDir()
	if opt.FileMode == 0 {
		opt.FileMode = 0o640
	}
	made, p, err := Create(context.Background(), src, items, out, name, opt)
	return out, made, p, err
}

func codeOf(err error) Code {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

// contents lists a ZIP's entries as name=content, folders as name/.
func zipContents(t *testing.T, file string) []string {
	t.Helper()
	r, err := zip.OpenReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
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
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("%s: %v", f.Name, err)
		}
		got = append(got, f.Name+"="+string(data))
	}
	return got
}

func same(t *testing.T, got []string, want ...string) {
	t.Helper()
	got, want = append([]string(nil), got...), append([]string(nil), want...)
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

// tools checks the archive with the programs on this machine that read it.
func tools(t *testing.T, file string) {
	t.Helper()
	t.Setenv("LC_ALL", "C.UTF-8")
	isZip := strings.HasSuffix(file, ".zip")
	if p, err := exec.LookPath("7z"); err == nil {
		out, err := exec.Command(p, "t", file).CombinedOutput()
		if err != nil || !strings.Contains(string(out), "Everything is Ok") || strings.Contains(string(out), "WARNING") {
			t.Errorf("7z t %s: %v\n%s", filepath.Base(file), err, out)
		}
	}
	if p, err := exec.LookPath("unzip"); err == nil && isZip {
		if out, err := exec.Command(p, "-t", file).CombinedOutput(); err != nil {
			t.Errorf("unzip -t: %v\n%s", err, out)
		}
	}
	if p, err := exec.LookPath("python3"); err == nil && isZip {
		script := "import sys,zipfile\nz=zipfile.ZipFile(sys.argv[1])\nbad=z.testzip()\nassert bad is None, bad\n"
		if out, err := exec.Command(p, "-I", "-c", script, file).CombinedOutput(); err != nil {
			t.Errorf("python zipfile: %v\n%s", err, out)
		}
	}
	if p, err := exec.LookPath("bsdtar"); err == nil {
		if out, err := exec.Command(p, "-tf", file).CombinedOutput(); err != nil {
			t.Errorf("bsdtar -tf: %v\n%s", err, out)
		}
	}
}

// extract opens the archive with Gezgin's unpack and lists what comes out.
func extract(t *testing.T, file string) map[string]string {
	t.Helper()
	dst := filepath.Join(t.TempDir(), "out")
	_, err := unpack.Extract(context.Background(), os.DirFS(filepath.Dir(file)), []string{filepath.Base(file)}, dst,
		unpack.Options{FileMode: 0o644, DirMode: 0o755, Limits: unpack.Limits{Entries: 10000, Layers: 1, Bytes: 1 << 30}})
	if err != nil {
		t.Fatalf("unpack: %v", err)
	}
	got := map[string]string{}
	_ = filepath.WalkDir(dst, func(p string, d fs.DirEntry, err error) error {
		if err != nil || p == dst {
			return err
		}
		rel, _ := filepath.Rel(dst, p)
		info, _ := d.Info()
		if !d.IsDir() && !info.ModTime().Equal(when) {
			t.Errorf("unpack: %s has time %v", rel, info.ModTime())
		}
		if d.IsDir() {
			got[filepath.ToSlash(rel)+"/"] = ""
		} else {
			data, _ := os.ReadFile(p)
			got[filepath.ToSlash(rel)] = string(data)
		}
		return nil
	})
	return got
}

var sample = []string{
	"Tatil/notlar.txt=" + strings.Repeat("Işık ve gölge. ", 200),
	"Tatil/fotoğraflar/deniz.jpg=jpeg data",
	"Tatil/film.mkv=matroska",
	"Tatil/boş/",
	"Tatil/çalıştır.sh=#!/bin/sh",
	"Tatil/sıfır.txt=",
}

func sampleTree(t *testing.T) string {
	root := tree(t, sample...)
	if err := os.Chmod(filepath.Join(root, "Tatil/çalıştır.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filepath.Join(root, "Tatil/çalıştır.sh"), when, when); err != nil {
		t.Fatal(err)
	}
	return root
}

var sampleWant = []string{
	"notlar.txt=" + strings.Repeat("Işık ve gölge. ", 200),
	"fotoğraflar/", "fotoğraflar/deniz.jpg=jpeg data",
	"film.mkv=matroska", "boş/", "çalıştır.sh=#!/bin/sh", "sıfır.txt=",
}

func TestZip(t *testing.T) {
	for _, force := range []bool{false, true} {
		t.Run(fmt.Sprintf("zip64=%v", force), func(t *testing.T) {
			forceZip64 = force
			defer func() { forceZip64 = false }()
			src := dirSource{root: sampleTree(t)}
			istanbul, err := time.LoadLocation("Europe/Istanbul")
			if err != nil {
				t.Skip(err)
			}
			dir, made, p, err := create(t, src, []Item{{Path: "Tatil"}}, "Tatil.zip", Options{Format: Zip, Location: istanbul})
			if err != nil {
				t.Fatal(err)
			}
			if len(made) != 1 || made[0] != "Tatil.zip" || p.Files != 7 || p.Entries != 7 || p.Skipped != 0 ||
				p.Total != int64(len(sampleWant[0])-len("notlar.txt=")+9+8+9) || p.Bytes != p.Total || p.Volumes != 1 {
				t.Errorf("made %v, progress %+v", made, p)
			}
			file := filepath.Join(dir, "Tatil.zip")
			if info, _ := os.Stat(file); info.Mode().Perm() != 0o640 {
				t.Errorf("mode %v", info.Mode())
			}
			same(t, zipContents(t, file), sampleWant...)

			r, err := zip.OpenReader(file)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			for _, f := range r.File {
				wantMethod := zip.Deflate
				if strings.HasSuffix(f.Name, "/") || strings.Contains(f.Name, ".jpg") || strings.Contains(f.Name, ".mkv") ||
					strings.Contains(f.Name, "sıfır") {
					wantMethod = zip.Store
				}
				if f.Method != wantMethod {
					t.Errorf("%s: method %d", f.Name, f.Method)
				}
				if f.Flags&0x8 != 0 {
					t.Errorf("%s: has a data descriptor", f.Name)
				}
				if utf8 := f.Flags&0x800 != 0; utf8 == ascii(f.Name) {
					t.Errorf("%s: UTF-8 flag %v", f.Name, utf8)
				}
				// Windows shows the MS-DOS time as it is: the time in Istanbul.
				dos := time.Date(1980+int(f.ModifiedDate>>9), time.Month(f.ModifiedDate>>5&0xf), int(f.ModifiedDate&0x1f),
					int(f.ModifiedTime>>11), int(f.ModifiedTime>>5&0x3f), int(f.ModifiedTime&0x1f)*2, 0, istanbul)
				if !dos.Equal(when) || !f.Modified.Equal(when) {
					t.Errorf("%s: MS-DOS time %v, time %v", f.Name, dos, f.Modified)
				}
				wantMode := fs.FileMode(0o644)
				switch {
				case strings.HasSuffix(f.Name, "/"):
					wantMode = fs.ModeDir | 0o755
				case strings.HasSuffix(f.Name, ".sh"):
					wantMode = 0o755
				}
				if f.Mode() != wantMode {
					t.Errorf("%s: mode %v", f.Name, f.Mode())
				}
			}
			tools(t, file)

			got := extract(t, file)
			var list []string
			for k, v := range got {
				if !strings.HasSuffix(k, "/") {
					k += "=" + v
				}
				list = append(list, k)
			}
			same(t, list, sampleWant...)
		})
	}
}

func TestZipStream(t *testing.T) {
	for _, force := range []bool{false, true} {
		t.Run(fmt.Sprintf("zip64=%v", force), func(t *testing.T) {
			forceZip64 = force
			defer func() { forceZip64 = false }()
			src := dirSource{root: sampleTree(t)}
			file := filepath.Join(t.TempDir(), "indir.zip")
			var buf bytes.Buffer
			if _, err := Write(context.Background(), src, []Item{{Path: "Tatil", Name: "Tatil"}}, &buf, Options{Format: Zip}); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(file, buf.Bytes(), 0o644); err != nil {
				t.Fatal(err)
			}
			var want []string
			for _, w := range append([]string{"/"}, sampleWant...) {
				want = append(want, "Tatil/"+strings.TrimPrefix(w, "/"))
			}
			same(t, zipContents(t, file), want...)
			r, _ := zip.OpenReader(file)
			for _, f := range r.File {
				if descriptor := f.Flags&0x8 != 0; descriptor == strings.HasSuffix(f.Name, "/") {
					t.Errorf("%s: data descriptor %v", f.Name, descriptor)
				}
			}
			r.Close()
			tools(t, file)
		})
	}
}

func TestTar(t *testing.T) {
	for _, gz := range []bool{false, true} {
		t.Run(fmt.Sprintf("gzip=%v", gz), func(t *testing.T) {
			src := dirSource{root: sampleTree(t)}
			opt, name := Options{Format: Tar}, "Tatil.tar"
			if gz {
				opt.Compress, name = Gzip, "Tatil.tar.gz"
			}
			dir, _, _, err := create(t, src, []Item{{Path: "Tatil"}}, name, opt)
			if err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(dir, name)
			f, err := os.Open(file)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			var r io.Reader = f
			if gz {
				if r, err = gzip.NewReader(f); err != nil {
					t.Fatal(err)
				}
			}
			tr := tar.NewReader(r)
			var got []string
			for {
				h, err := tr.Next()
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				if h.Uid != 0 || h.Uname != "" || !h.ModTime.Equal(when) {
					t.Errorf("%s: %+v", h.Name, h)
				}
				data, _ := io.ReadAll(tr)
				if h.Typeflag == tar.TypeDir {
					got = append(got, h.Name)
				} else {
					got = append(got, h.Name+"="+string(data))
				}
			}
			same(t, got, sampleWant...)
			tools(t, file)
			if len(extract(t, file)) != len(sampleWant) {
				t.Error("unpack took out something else")
			}
		})
	}
}

func TestVolumes(t *testing.T) {
	src := dirSource{root: sampleTree(t)}
	items := []Item{{Path: "Tatil"}}
	dir, _, _, err := create(t, src, items, "Tatil.zip", Options{Format: Zip})
	if err != nil {
		t.Fatal(err)
	}
	whole, _ := os.ReadFile(filepath.Join(dir, "Tatil.zip"))

	vdir, made, p, err := create(t, src, items, "Tatil.zip", Options{Format: Zip, Volume: 256})
	if err != nil {
		t.Fatal(err)
	}
	if want := (len(whole) + 255) / 256; len(made) != want || p.Volumes != want || want < 3 {
		t.Fatalf("%d volumes, progress %+v; want %d", len(made), p, want)
	}
	var joined []byte
	for i, name := range made {
		if name != fmt.Sprintf("Tatil.zip.%03d", i+1) {
			t.Errorf("volume %d named %s", i, name)
		}
		data, _ := os.ReadFile(filepath.Join(vdir, name))
		if i < len(made)-1 && len(data) != 256 {
			t.Errorf("%s has %d bytes", name, len(data))
		}
		joined = append(joined, data...)
	}
	if !bytes.Equal(joined, whole) {
		t.Error("the volumes joined are not the archive")
	}
	// Gezgin opens the set by any of its volumes.
	if got := extract(t, filepath.Join(vdir, "Tatil.zip.002")); len(got) != len(sampleWant) {
		t.Errorf("unpack took out %v", got)
	}
	if p, err := exec.LookPath("7z"); err == nil {
		out, err := exec.Command(p, "t", filepath.Join(vdir, "Tatil.zip.001")).CombinedOutput()
		if err != nil || !strings.Contains(string(out), "Everything is Ok") || strings.Contains(string(out), "WARNING") {
			t.Errorf("7z t Tatil.zip.001: %v\n%s", err, out)
		}
	}

	// An archive that fits in one volume is the archive.
	_, made, _, err = create(t, src, items, "Tatil.zip", Options{Format: Zip, Volume: 1 << 20})
	if err != nil || len(made) != 1 || made[0] != "Tatil.zip" {
		t.Errorf("one volume: %v, %v", made, err)
	}

	// Too many volumes are refused before anything is written, when what is stored already
	// tells, or as they are written.
	big := tree(t, "a.mkv="+strings.Repeat("v", 5000))
	if _, made, _, err := create(t, dirSource{root: big}, []Item{{Path: "a.mkv", Name: "a.mkv"}}, "a.zip",
		Options{Format: Zip, Volume: 1000, Limits: Limits{Volumes: 3}}); codeOf(err) != CodeVolumes || len(made) != 0 {
		t.Errorf("planned: %v, made %v", err, made)
	}
	if _, _, _, err := create(t, src, items, "Tatil.zip", Options{Format: Zip, Volume: 256, Limits: Limits{Volumes: 3}}); codeOf(err) != CodeVolumes {
		t.Errorf("written: %v", err)
	}
}

func TestWhatIsLeftOut(t *testing.T) {
	root := tree(t,
		"k/izinli.txt=ok",
		"k/gizli.txt=no",
		"k/.gezgin-1a2b.tmp=half",
		"k/a\\b.txt=backslash",
		"k/a_b.txt=underscore",
		"k/CON.txt=device",
		"k/soru?.txt=question",
		"k/Buyuk.txt=upper",
		"k/alt/x.txt=x",
		"dis/out.txt=outside",
	)
	k := filepath.Join(root, "k")
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(syscall.Mkfifo(filepath.Join(k, "boru"), 0o644))
	must(os.Symlink("izinli.txt", filepath.Join(k, "bag.txt")))
	must(os.Symlink("yok.txt", filepath.Join(k, "kopuk.txt")))
	must(os.Symlink("../dis/out.txt", filepath.Join(k, "disari.txt")))
	must(os.Symlink("..", filepath.Join(k, "alt", "dongu")))
	want := []string{"izinli.txt=ok", "bag.txt=ok", "a_b.txt=backslash", "CON.txt=device", "soru?.txt=question",
		"Buyuk.txt=upper", "alt/", "alt/x.txt=x"}
	// gizli.txt, a_b.txt's twin, the FIFO, the dangling link, the link out and the loop; CON.txt
	// and soru?.txt.
	skipped, windows := 6, 2
	// Two names in other letter case, which a file system that tells them apart holds (macOS's
	// does not): Windows does not.
	if _, err := os.Stat(filepath.Join(k, "BUYUK.TXT")); errors.Is(err, fs.ErrNotExist) {
		must(os.WriteFile(filepath.Join(k, "buyuk.txt"), []byte("lower"), 0o644))
		want = append(want, "buyuk.txt=lower")
		windows++
	}
	// A name that is not UTF-8, where the file system takes one (macOS's does not).
	if os.WriteFile(filepath.Join(k, "bozuk-\xff.txt"), []byte("not UTF-8"), 0o644) == nil {
		skipped++
	}

	src := dirSource{root: k, refuse: func(name string) bool { return strings.HasSuffix(name, "gizli.txt") }}
	done := make(chan struct{})
	var dir string
	var p Progress
	var err error
	go func() {
		defer close(done)
		dir, _, p, err = create(t, src, []Item{{Path: "."}}, "k.zip", Options{Format: Zip})
	}()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("the job hangs, as on a FIFO")
	}
	if err != nil {
		t.Fatal(err)
	}
	same(t, zipContents(t, filepath.Join(dir, "k.zip")), want...)
	if p.Skipped != skipped || p.Windows != windows {
		t.Errorf("progress %+v; want %d skipped, %d Windows names", p, skipped, windows)
	}

	// A source that follows links out takes the file it leads to.
	src.follow = true
	dir, _, _, err = create(t, src, []Item{{Path: "disari.txt", Name: "disari.txt"}}, "d.zip", Options{Format: Zip})
	if err != nil {
		t.Fatal(err)
	}
	same(t, zipContents(t, filepath.Join(dir, "d.zip")), "disari.txt=outside")
}

// TestUnpackTakesItBack leaves out what Gezgin's unpack would refuse the whole archive for: a C1
// control character, a file deeper than unpack goes, and an item chosen with a name Gezgin keeps
// for itself.
func TestUnpackTakesItBack(t *testing.T) {
	deep := strings.Repeat("d/", maxDepth-1)
	for _, c := range []struct {
		name  string
		tree  []string
		items []Item
		want  []string
	}{
		{"C1 control", []string{"k/a.txt=a", "k/c1-\u0085.txt=c1"}, []Item{{Path: "k"}}, []string{"a.txt=a"}},
		{"too deep", []string{"k/" + deep + "son.txt=last", "k/" + deep + "d/derin.txt=deep"}, []Item{{Path: "k"}},
			[]string{deep + "son.txt=last"}},
		{"chosen reserved", []string{"k/a.txt=a", "k/.gezgin-1a2b.tmp=half"},
			[]Item{{Path: "k/.gezgin-1a2b.tmp", Name: ".gezgin-1a2b.tmp"}, {Path: "k/a.txt", Name: "a.txt"}},
			[]string{"a.txt=a"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir, _, p, err := create(t, dirSource{root: tree(t, c.tree...)}, c.items, "k.zip", Options{Format: Zip})
			if err != nil {
				t.Fatal(err)
			}
			if p.Skipped != 1 {
				t.Errorf("progress %+v; want 1 skipped", p)
			}
			var files []string
			for name, content := range extract(t, filepath.Join(dir, "k.zip")) {
				if !strings.HasSuffix(name, "/") {
					files = append(files, name+"="+content)
				}
			}
			same(t, files, c.want...)
		})
	}

	// Chosen alone, Gezgin's file leaves nothing to pack.
	src := dirSource{root: tree(t, "k/.gezgin-1a2b.tmp=half")}
	if _, _, _, err := create(t, src, []Item{{Path: "k/.gezgin-1a2b.tmp"}}, "y.zip", Options{Format: Zip}); codeOf(err) != CodeEmpty {
		t.Errorf("only Gezgin's file: err = %v", err)
	}
}

func TestLimits(t *testing.T) {
	root := tree(t, "k/a.txt=aaaa", "k/b.mkv="+strings.Repeat("b", 4000), "k/c/", "bos/")
	src := dirSource{root: root}
	items := []Item{{Path: "k"}}
	if _, _, _, err := create(t, src, items, "x.zip", Options{Format: Zip, Limits: Limits{Entries: 2}}); codeOf(err) != CodeEntries {
		t.Errorf("entries: %v", err)
	}
	// What is stored as it is counts before anything is written.
	if _, made, _, err := create(t, src, items, "x.zip", Options{Format: Zip, Limits: Limits{Bytes: 3000}}); codeOf(err) != CodeNoSpace || len(made) != 0 {
		t.Errorf("planned bytes: %v, %v", err, made)
	}
	// What is compressed counts as it is written.
	text := tree(t, "t.txt="+strings.Repeat("0123456789abcdef", 10000))
	if _, _, _, err := create(t, dirSource{root: text}, []Item{{Path: "t.txt", Name: "t.txt"}}, "t.tar.gz",
		Options{Format: Tar, Compress: func(w io.Writer) (io.WriteCloser, error) { return nopCloser{w}, nil }, Limits: Limits{Bytes: 50000}}); codeOf(err) != CodeNoSpace {
		t.Errorf("written bytes: %v", err)
	}
	if _, _, _, err := create(t, src, []Item{{Path: "bos"}}, "x.zip", Options{Format: Zip}); codeOf(err) != CodeEmpty {
		t.Errorf("empty: %v", err)
	}
}

type nopCloser struct{ io.Writer }

func (nopCloser) Close() error { return nil }

func TestChanges(t *testing.T) {
	root := tree(t, "k/a.txt=first", "k/b.txt=second")
	grow := func(name string) {
		if strings.HasSuffix(name, "b.txt") {
			f, _ := os.OpenFile(filepath.Join(root, name), os.O_APPEND|os.O_WRONLY, 0)
			_, _ = f.WriteString(" and more")
			f.Close()
		}
	}
	src := dirSource{root: root, opened: grow}
	if _, _, _, err := create(t, src, []Item{{Path: "k"}}, "x.zip", Options{Format: Zip}); codeOf(err) != CodeChanged {
		t.Errorf("err = %v; want changed", err)
	}

	// A download takes the file as it was planned.
	root = tree(t, "k/a.txt=first", "k/b.txt=second")
	src = dirSource{root: root, opened: grow}
	var buf bytes.Buffer
	if _, err := Write(context.Background(), src, []Item{{Path: "k"}}, &buf, Options{Format: Zip, Lenient: true}); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "x.zip")
	_ = os.WriteFile(file, buf.Bytes(), 0o644)
	same(t, zipContents(t, file), "a.txt=first", "b.txt=second")

	// A file replaced by another is not taken, even by a download.
	root = tree(t, "k/a.txt=first")
	src = dirSource{root: root, opened: func(name string) {
		// Written beside it first, so that it cannot take the old file's inode number.
		p := filepath.Join(root, name)
		_ = os.WriteFile(p+".new", []byte("other"), 0o644)
		_ = os.Rename(p+".new", p)
	}}
	if _, err := Write(context.Background(), src, []Item{{Path: "k"}}, io.Discard, Options{Format: Zip, Lenient: true}); codeOf(err) != CodeChanged {
		t.Errorf("replaced: %v", err)
	}
}

func TestCancel(t *testing.T) {
	root := tree(t, "k/a.txt=a")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := Create(ctx, dirSource{root: root}, []Item{{Path: "k"}}, t.TempDir(), "x.zip", Options{Format: Zip}); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v", err)
	}
}

func TestItemsUnderTheirNames(t *testing.T) {
	root := tree(t, "k/a.txt=a", "k/alt/b.txt=b", "c.txt=c")
	dir, _, _, err := create(t, dirSource{root: root}, []Item{{Path: "k", Name: "k"}, {Path: "c.txt", Name: "c.txt"}}, "x.zip", Options{Format: Zip})
	if err != nil {
		t.Fatal(err)
	}
	same(t, zipContents(t, filepath.Join(dir, "x.zip")), "k/", "k/a.txt=a", "k/alt/", "k/alt/b.txt=b", "c.txt=c")
}

// TestLargeFile packs a file of more than 4 GiB, when PACK_LARGE is set: it takes a minute.
func TestLargeFile(t *testing.T) {
	if os.Getenv("PACK_LARGE") == "" {
		t.Skip("PACK_LARGE is not set")
	}
	root := t.TempDir()
	f, err := os.Create(filepath.Join(root, "buyuk.bin"))
	if err != nil {
		t.Fatal(err)
	}
	const size = 4_700_000_000
	_, _ = f.WriteAt([]byte("son"), size-3)
	f.Close()
	for _, stream := range []bool{false, true} {
		file := filepath.Join(t.TempDir(), "buyuk.zip")
		items := []Item{{Path: "buyuk.bin", Name: "buyuk.bin"}}
		if stream {
			out, _ := os.Create(file)
			if _, err := Write(context.Background(), dirSource{root: root}, items, out, Options{Format: Zip}); err != nil {
				t.Fatal(err)
			}
			out.Close()
		} else {
			made, _, err := Create(context.Background(), dirSource{root: root}, items, filepath.Dir(file), "buyuk.zip", Options{Format: Zip, FileMode: 0o644})
			if err != nil || len(made) != 1 {
				t.Fatal(made, err)
			}
		}
		r, err := zip.OpenReader(file)
		if err != nil {
			t.Fatal(err)
		}
		if f := r.File[0]; f.UncompressedSize64 != size {
			t.Errorf("size %d", f.UncompressedSize64)
		}
		r.Close()
		tools(t, file)
	}
}
