package files

import (
	"os"
	"testing"

	"github.com/spf13/afero"
)

// countingFs counts the files opened, folders aside.
type countingFs struct {
	afero.Fs
	opened []string
}

func (c *countingFs) Open(name string) (afero.File, error) {
	if info, err := c.Stat(name); err == nil && !info.IsDir() {
		c.opened = append(c.opened, name)
	}
	return c.Fs.Open(name)
}

func (c *countingFs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	if info, err := c.Stat(name); err == nil && !info.IsDir() {
		c.opened = append(c.opened, name)
	}
	return c.Fs.OpenFile(name, flag, perm)
}

type allowAll struct{}

func (allowAll) Check(string) bool { return true }

func TestListingDoesNotOpenFiles(t *testing.T) {
	afs := &countingFs{Fs: afero.NewMemMapFs()}
	for _, name := range []string{"/movie.mkv", "/notes.txt", "/README", "/photo.jpg"} {
		_ = afero.WriteFile(afs.Fs, name, []byte("content"), 0o644)
	}

	dir, err := NewFileInfo(&FileOptions{Fs: afs, Path: "/", Expand: true, Checker: allowAll{}})
	if err != nil {
		t.Fatal(err)
	}
	if len(dir.Items) != 4 {
		t.Fatalf("listed %d entries; want 4", len(dir.Items))
	}
	if len(afs.opened) != 0 {
		t.Errorf("listing opened %v", afs.opened)
	}
	types := map[string]string{}
	for _, item := range dir.Items {
		types[item.Name] = item.Type
	}
	if types["movie.mkv"] != "video" || types["photo.jpg"] != "image" || types["notes.txt"] != "text" {
		t.Errorf("types by extension: %v", types)
	}

	// A single file is still looked at.
	if _, err := NewFileInfo(&FileOptions{Fs: afs, Path: "/README", Expand: true, Checker: allowAll{}}); err != nil {
		t.Fatal(err)
	}
	if len(afs.opened) == 0 {
		t.Errorf("opening a single file did not read it")
	}
}
