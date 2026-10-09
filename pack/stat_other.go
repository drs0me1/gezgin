//go:build !linux

package pack

import "io/fs"

// changeTime is not looked at on other systems.
func changeTime(fs.FileInfo) int64 { return 0 }
