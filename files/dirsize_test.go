package files

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
)

// checkerFunc adapts a function to rules.Checker.
type checkerFunc func(string) bool

func (f checkerFunc) Check(p string) bool { return f(p) }

func listedDirs(t *testing.T, opts *FileOptions) map[string]*FileInfo {
	t.Helper()
	dir, err := NewFileInfo(opts)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]*FileInfo{}
	for _, item := range dir.Items {
		if item.IsDir {
			found[item.Name] = item
		}
	}
	return found
}

// K84-K86: a listing asked for its folders' sizes gives each its count, the items a user sees on
// opening it, and its size, everything the rules let them reach, hidden files included.
func TestListingFolderCountsAndSizes(t *testing.T) {
	fs := afero.NewMemMapFs()
	for name, size := range map[string]int{
		"/albüm/a.jpg": 10, "/albüm/2024/b.jpg": 20, "/albüm/.gizli": 5, "/albüm/yasak/c.jpg": 100,
		"/diger/d.txt": 1,
	} {
		if err := afero.WriteFile(fs, name, []byte(strings.Repeat("x", size)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := fs.Mkdir("/bos", 0o755); err != nil {
		t.Fatal(err)
	}
	refused := func(p string) bool { return p == "/albüm/yasak" || strings.HasPrefix(p, "/albüm/yasak/") }
	// The listing hides dotfiles, as for a user who hides them; the size does not.
	list := checkerFunc(func(p string) bool { return !refused(p) && !strings.HasPrefix(filepath.Base(p), ".") })
	sizes := checkerFunc(func(p string) bool { return !refused(p) })

	dirs := listedDirs(t, &FileOptions{Fs: fs, Path: "/", Expand: true, Checker: list, DirSizes: sizes})
	for name, want := range map[string]struct {
		count int
		size  int64
	}{"albüm": {2, 35}, "bos": {0, 0}, "diger": {1, 1}} {
		got := dirs[name]
		if got == nil || got.Count == nil || *got.Count != want.count || got.Size != want.size || got.SizeUnknown {
			t.Errorf("%s: %+v; want count %d, size %d", name, got, want.count, want.size)
		}
	}

	// Not asked, a listing walks nothing.
	for name, item := range listedDirs(t, &FileOptions{Fs: fs, Path: "/", Expand: true, Checker: list}) {
		if item.Count != nil || item.SizeUnknown {
			t.Errorf("%s walked unasked: %+v", name, item)
		}
	}
}

// K85: past the listing's budget a folder keeps its count, and its size is unknown.
func TestListingFolderSizeBudget(t *testing.T) {
	defer func(n int) { dirWalkEntries = n }(dirWalkEntries)
	dirWalkEntries = 3

	fs := afero.NewMemMapFs()
	for _, name := range []string{"/a/1", "/a/2", "/b/1", "/b/2"} {
		if err := afero.WriteFile(fs, name, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	all := checkerFunc(func(string) bool { return true })
	dirs := listedDirs(t, &FileOptions{Fs: fs, Path: "/", Expand: true, Checker: all, DirSizes: all})
	if a := dirs["a"]; a.SizeUnknown || a.Size != 2 || *a.Count != 2 {
		t.Errorf("a, within the budget: %+v", a)
	}
	if b := dirs["b"]; !b.SizeUnknown || b.Count == nil || *b.Count != 2 {
		t.Errorf("b, past the budget: %+v; want its count and an unknown size", b)
	}
}

// K85: no link is followed: a link's target is not added, and a linked folder keeps its count
// without a size.
func TestListingFolderSizeFollowsNoLink(t *testing.T) {
	root := t.TempDir()
	write := func(name string, size int) {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(strings.Repeat("x", size)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("buyuk/film.mkv", 1000)
	write("klasor/not.txt", 3)
	if err := os.Symlink(filepath.Join(root, "buyuk", "film.mkv"), filepath.Join(root, "klasor", "film.mkv")); err != nil {
		t.Skip("no symbolic links here:", err)
	}
	if err := os.Symlink(filepath.Join(root, "buyuk"), filepath.Join(root, "kisayol")); err != nil {
		t.Fatal(err)
	}

	all := checkerFunc(func(string) bool { return true })
	fs := NewFs(afero.NewOsFs(), root, false)
	dirs := listedDirs(t, &FileOptions{Fs: fs, Path: "/", Expand: true, Checker: all, DirSizes: all})
	if k := dirs["klasor"]; k == nil || k.Size != 3 || *k.Count != 2 {
		t.Errorf("klasor: %+v; want size 3 (the link not followed) and 2 items", k)
	}
	if k := dirs["kisayol"]; k == nil || !k.SizeUnknown || k.Count == nil || *k.Count != 1 {
		t.Errorf("kisayol, a link to a folder: %+v; want its count without a size", k)
	}
}
