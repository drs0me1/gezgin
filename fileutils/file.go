package fileutils

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"syscall"

	"github.com/spf13/afero"

	fberrors "github.com/filebrowser/filebrowser/v2/errors"
)

// MoveFile moves src to dst (Gezgin). When dst exists (an overwrite was
// asked) a file replaces a file, a folder is merged into a folder, and a file
// and a folder never replace each other. The rename system call does the
// work; copying is only the fallback across file systems, and a failed
// fallback leaves what was at dst as it was.
func MoveFile(afs afero.Fs, src, dst string, fileMode, dirMode fs.FileMode) error {
	srcInfo, err := lstat(afs, src)
	if err != nil {
		return err
	}
	if dstInfo, err := lstat(afs, dst); err == nil && !os.SameFile(srcInfo, dstInfo) {
		switch {
		case srcInfo.IsDir() != dstInfo.IsDir():
			return fberrors.ErrTypeMismatch
		case srcInfo.IsDir():
			return mergeDir(afs, src, dst, fileMode, dirMode)
		}
	}

	err = afs.Rename(src, dst)
	if err == nil || !errors.Is(err, syscall.EXDEV) {
		return err
	}
	return moveAcross(afs, src, dst, srcInfo, fileMode, dirMode)
}

// mergeDir moves the entries of the folder src into the folder dst, then
// removes src.
func mergeDir(afs afero.Fs, src, dst string, fileMode, dirMode fs.FileMode) error {
	dir, err := afs.Open(src)
	if err != nil {
		return err
	}
	names, err := dir.Readdirnames(-1)
	dir.Close()
	if err != nil {
		return err
	}

	for _, name := range names {
		if err := MoveFile(afs, path.Join(src, name), path.Join(dst, name), fileMode, dirMode); err != nil {
			return err
		}
	}
	return afs.Remove(src)
}

// moveAcross copies src to dst on another file system, then removes src.
func moveAcross(afs afero.Fs, src, dst string, info os.FileInfo, fileMode, dirMode fs.FileMode) error {
	if info.IsDir() {
		// dst does not exist here: an existing folder is merged entry by entry.
		if err := CopyDir(afs, src, dst, fileMode, dirMode); err != nil {
			_ = afs.RemoveAll(dst)
			return err
		}
		return afs.RemoveAll(src)
	}

	// CopyFile replaces dst only once the copy is complete.
	if err := CopyFile(afs, src, dst, fileMode, dirMode); err != nil {
		return err
	}
	return afs.Remove(src)
}

// CopyFile copies a file from source to dest with the source's permissions.
// An existing dest is replaced only once the copy is complete.
func CopyFile(afs afero.Fs, source, dest string, _, dirMode fs.FileMode) error {
	src, err := afs.Open(source)
	if err != nil {
		return err
	}
	defer src.Close()

	info, err := src.Stat()
	if err != nil {
		return err
	}

	// Makes the directory needed to create the dst
	// file.
	err = afs.MkdirAll(filepath.Dir(dest), dirMode)
	if err != nil {
		return err
	}

	_, err = WriteAtomic(afs, dest, src, info.Mode().Perm())
	return err
}

// CommonPrefix returns common directory path of provided files
func CommonPrefix(sep byte, paths ...string) string {
	// Handle special cases.
	switch len(paths) {
	case 0:
		return ""
	case 1:
		return path.Clean(paths[0])
	}

	// Note, we treat string as []byte, not []rune as is often
	// done in Go. (And sep as byte, not rune). This is because
	// most/all supported OS' treat paths as string of non-zero
	// bytes. A filename may be displayed as a sequence of Unicode
	// runes (typically encoded as UTF-8) but paths are
	// not required to be valid UTF-8 or in any normalized form
	// (e.g. "é" (U+00C9) and "é" (U+0065,U+0301) are different
	// file names.
	c := []byte(path.Clean(paths[0]))

	// We add a trailing sep to handle the case where the
	// common prefix directory is included in the path list
	// (e.g. /home/user1, /home/user1/foo, /home/user1/bar).
	// path.Clean will have cleaned off trailing / separators with
	// the exception of the root directory, "/" (in which case we
	// make it "//", but this will get fixed up to "/" below).
	c = append(c, sep)

	// Ignore the first path since it's already in c
	for _, v := range paths[1:] {
		// Clean up each path before testing it
		v = path.Clean(v) + string(sep)

		// Find the first non-common byte and truncate c
		if len(v) < len(c) {
			c = c[:len(v)]
		}
		for i := 0; i < len(c); i++ {
			if v[i] != c[i] {
				c = c[:i]
				break
			}
		}
	}

	// Remove trailing non-separator characters and the final separator
	for i := len(c) - 1; i >= 0; i-- {
		if c[i] == sep {
			c = c[:i]
			break
		}
	}

	return string(c)
}
