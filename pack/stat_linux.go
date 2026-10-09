package pack

import (
	"io/fs"
	"syscall"
)

// changeTime is when the file's inode last changed, which a write sets whatever its
// modification time is made to say.
func changeTime(info fs.FileInfo) int64 {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return st.Ctim.Nano()
	}
	return 0
}
