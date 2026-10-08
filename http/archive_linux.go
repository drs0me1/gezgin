package fbhttp

import (
	"errors"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

// renameNoReplace moves oldpath to newpath unless something is there, even something that
// appeared a moment ago: a plain rename would replace an empty folder.
func renameNoReplace(oldpath, newpath string) error {
	err := unix.Renameat2(unix.AT_FDCWD, oldpath, unix.AT_FDCWD, newpath, unix.RENAME_NOREPLACE)
	if errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EINVAL) {
		// A file system without the flag.
		if _, statErr := os.Lstat(newpath); statErr == nil {
			return os.ErrExist
		}
		return os.Rename(oldpath, newpath)
	}
	if err != nil {
		return &os.LinkError{Op: "rename", Old: oldpath, New: newpath, Err: err}
	}
	return nil
}

// sameDisk reports whether a and b are on the same file system.
func sameDisk(a, b string) bool {
	var sa, sb syscall.Stat_t
	if syscall.Stat(a, &sa) != nil || syscall.Stat(b, &sb) != nil {
		return true // the rename will tell
	}
	return sa.Dev == sb.Dev
}
