package settings

import (
	"path"
	"strings"

	"github.com/filebrowser/filebrowser/v2/trash"
)

// UploadsDir is the folder at the server root that holds the data of the uploads in progress
// (Gezgin).
const UploadsDir = ".gezgin-yukleme"

// ArchiveDir is the folder at the server root where archive jobs build their results (Gezgin).
const ArchiveDir = ".gezgin-arsiv"

// ReservedDirs are Gezgin's own folders at the server root: the trash, the uploads in progress
// and the archive jobs. No user path reaches them, and none of them can be a scope.
var ReservedDirs = []string{trash.Dir, UploadsDir, ArchiveDir}

// reservedScope is the error text for a scope in ReservedDirs.
const reservedScope = "a scope cannot lie in Gezgin's folders"

// IsReserved reports whether the "/"-separated path p, taken from the server root, lies in one
// of ReservedDirs.
func IsReserved(p string, caseInsensitive bool) bool {
	p = path.Join("/", p)
	if caseInsensitive {
		p = strings.ToLower(p)
	}
	for _, dir := range ReservedDirs {
		top := "/" + dir
		if caseInsensitive {
			top = strings.ToLower(top)
		}
		if p == top || strings.HasPrefix(p, top+"/") {
			return true
		}
	}
	return false
}
