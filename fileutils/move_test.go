package fileutils

import (
	"errors"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/spf13/afero"
)

// otherDisk makes every rename fail as between two file systems, and can refuse to create the
// temporary file a copy goes through.
type otherDisk struct {
	afero.Fs
	failTemp bool
}

func (o *otherDisk) Rename(oldname, newname string) error {
	if strings.Contains(oldname, ".gezgin-") {
		return o.Fs.Rename(oldname, newname)
	}
	return &os.LinkError{Op: "rename", Old: oldname, New: newname, Err: syscall.EXDEV}
}

func (o *otherDisk) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	if o.failTemp && strings.Contains(name, ".gezgin-") {
		return nil, errors.New("disk full")
	}
	return o.Fs.OpenFile(name, flag, perm)
}

func content(t *testing.T, afs afero.Fs, name string) string {
	t.Helper()
	data, err := afero.ReadFile(afs, name)
	if err != nil {
		return "<missing>"
	}
	return string(data)
}

func TestMoveAcrossFileSystems(t *testing.T) {
	mem := afero.NewMemMapFs()
	afs := &otherDisk{Fs: mem}
	_ = afero.WriteFile(mem, "/a/file.txt", []byte("moved"), 0o644)
	_ = afero.WriteFile(mem, "/b/file.txt", []byte("replaced"), 0o644)
	_ = afero.WriteFile(mem, "/dir/inner.txt", []byte("inner"), 0o644)

	if err := MoveFile(afs, "/a/file.txt", "/b/file.txt", 0o640, 0o750); err != nil {
		t.Fatalf("file across file systems: %v", err)
	}
	if content(t, mem, "/b/file.txt") != "moved" || content(t, mem, "/a/file.txt") != "<missing>" {
		t.Errorf("after the move: dst %q src %q", content(t, mem, "/b/file.txt"), content(t, mem, "/a/file.txt"))
	}

	if err := MoveFile(afs, "/dir", "/elsewhere", 0o640, 0o750); err != nil {
		t.Fatalf("folder across file systems: %v", err)
	}
	if content(t, mem, "/elsewhere/inner.txt") != "inner" || content(t, mem, "/dir/inner.txt") != "<missing>" {
		t.Errorf("after the folder move: dst %q src %q", content(t, mem, "/elsewhere/inner.txt"), content(t, mem, "/dir/inner.txt"))
	}
}

func TestFailedMoveAcrossFileSystemsKeepsBothSides(t *testing.T) {
	mem := afero.NewMemMapFs()
	afs := &otherDisk{Fs: mem, failTemp: true}
	_ = afero.WriteFile(mem, "/a.txt", []byte("source"), 0o644)
	_ = afero.WriteFile(mem, "/b.txt", []byte("destination"), 0o644)

	if err := MoveFile(afs, "/a.txt", "/b.txt", 0o640, 0o750); err == nil {
		t.Fatalf("a failed copy reported success")
	}
	if content(t, mem, "/a.txt") != "source" || content(t, mem, "/b.txt") != "destination" {
		t.Errorf("a failed move changed the files: src %q dst %q", content(t, mem, "/a.txt"), content(t, mem, "/b.txt"))
	}
}
