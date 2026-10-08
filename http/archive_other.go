//go:build !linux

package fbhttp

import "os"

// renameNoReplace moves oldpath to newpath unless something is there.
func renameNoReplace(oldpath, newpath string) error {
	if _, err := os.Lstat(newpath); err == nil {
		return os.ErrExist
	}
	return os.Rename(oldpath, newpath)
}

// sameDisk is left to the rename on other systems.
func sameDisk(_, _ string) bool { return true }
