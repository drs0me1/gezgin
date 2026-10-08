package fileutils

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"syscall"

	"github.com/spf13/afero"
)

// lstat stats name without following a final symbolic link when the filesystem
// allows it.
func lstat(afs afero.Fs, name string) (os.FileInfo, error) {
	if l, ok := afs.(afero.Lstater); ok {
		info, _, err := l.LstatIfPossible(name)
		return info, err
	}
	return afs.Stat(name)
}

// TempName returns a hidden, unused name beside name for a file that will
// take its place.
func TempName(name string) (string, error) {
	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return path.Join(path.Dir(name), ".gezgin-"+hex.EncodeToString(random)+".tmp"), nil
}

// WriteAtomic writes in to name with the given permissions. The content goes
// to a temporary file beside name, which takes name's place only once it is
// complete, so a write that fails half way leaves an existing file as it was
// (Gezgin). A symbolic link at name is written through instead, since
// replacing it would replace the link. A file with other hard links gets its
// own copy; the other names keep the old content.
func WriteAtomic(afs afero.Fs, name string, in io.Reader, perm fs.FileMode) (os.FileInfo, error) {
	if info, err := lstat(afs, name); err == nil {
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			return writeThrough(afs, name, in, perm)
		case info.IsDir():
			return nil, &os.PathError{Op: "write", Path: name, Err: errors.New("is a directory")}
		}
	}

	tmp, err := TempName(name)
	if err != nil {
		return nil, err
	}
	file, err := afs.OpenFile(tmp, os.O_RDWR|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(file, in)
	if err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	// The mode is set explicitly: the umask must not narrow an existing file's
	// permissions when its new content takes its place.
	if err == nil {
		err = afs.Chmod(tmp, perm)
	}
	if err == nil {
		err = afs.Rename(tmp, name)
	}
	if err != nil {
		_ = afs.Remove(tmp)
		return nil, err
	}

	return afs.Stat(name)
}

// Place puts the complete file at the real path staged, outside afs, at name,
// whose real path is realName (Gezgin). Like WriteAtomic, it replaces an
// existing file only with the complete content and writes through a symbolic
// link at name. The file is renamed into place, or copied when it is on
// another disk.
func Place(afs afero.Fs, staged, name, realName string, perm fs.FileMode) (os.FileInfo, error) {
	info, err := lstat(afs, name)
	switch {
	case err == nil && info.Mode()&os.ModeSymlink != 0:
		return placeCopy(afs, staged, name, perm)
	case err == nil && info.IsDir():
		return nil, &os.PathError{Op: "write", Path: name, Err: errors.New("is a directory")}
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		return nil, err
	}

	if err = os.Chmod(staged, perm); err != nil {
		return nil, err
	}
	err = os.Rename(staged, realName)
	if errors.Is(err, syscall.EXDEV) {
		return placeCopy(afs, staged, name, perm)
	}
	if err != nil {
		return nil, err
	}
	return afs.Stat(name)
}

// placeCopy copies the file at the real path staged to name.
func placeCopy(afs afero.Fs, staged, name string, perm fs.FileMode) (os.FileInfo, error) {
	in, err := os.Open(staged)
	if err != nil {
		return nil, err
	}
	defer in.Close()
	return WriteAtomic(afs, name, in, perm)
}

// writeThrough writes in to the file a symbolic link points at, in place.
func writeThrough(afs afero.Fs, name string, in io.Reader, perm fs.FileMode) (os.FileInfo, error) {
	file, err := afs.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	if _, err = io.Copy(file, in); err != nil {
		return nil, err
	}
	if err = file.Sync(); err != nil {
		return nil, err
	}
	return file.Stat()
}
