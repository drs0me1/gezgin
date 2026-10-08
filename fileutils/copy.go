package fileutils

import (
	"io/fs"
	"os"
	"path"

	"github.com/spf13/afero"

	fberrors "github.com/filebrowser/filebrowser/v2/errors"
)

// Copy copies a file or folder from one place to another.
func Copy(afs afero.Fs, src, dst string, fileMode, dirMode fs.FileMode) error {
	if src = path.Clean("/" + src); src == "" {
		return os.ErrNotExist
	}

	if dst = path.Clean("/" + dst); dst == "" {
		return os.ErrNotExist
	}

	if src == "/" || dst == "/" {
		// Prohibit copying from or to the virtual root directory.
		return os.ErrInvalid
	}

	if dst == src {
		return os.ErrInvalid
	}

	info, err := afs.Stat(src)
	if err != nil {
		return err
	}

	// A file and a folder never replace each other (Gezgin).
	if existing, err := afs.Stat(dst); err == nil && existing.IsDir() != info.IsDir() {
		return fberrors.ErrTypeMismatch
	}

	if info.IsDir() {
		return CopyDir(afs, src, dst, fileMode, dirMode)
	}

	return CopyFile(afs, src, dst, fileMode, dirMode)
}
