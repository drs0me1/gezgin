package unpack

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/mholt/archives"
)

type format int

const (
	formatNone format = iota
	formatZip
	formatRar
	format7z
	formatTar
)

// kind is the format of an archive and, for a tar, its compression.
type kind struct {
	format      format
	compression archives.Compression
}

// tarSuffixes are the names of the tar files Extract opens, and of what compresses them.
var tarSuffixes = []struct {
	suffix      string
	compression archives.Compression
}{
	{".tar", nil},
	{".tar.gz", archives.Gz{}}, {".tgz", archives.Gz{}},
	{".tar.bz2", archives.Bz2{}}, {".tbz2", archives.Bz2{}}, {".tbz", archives.Bz2{}},
	{".tar.xz", archives.Xz{}}, {".txz", archives.Xz{}},
	{".tar.zst", archives.Zstd{}}, {".tzst", archives.Zstd{}},
}

var (
	// newPart is a part of a RAR set named in the new way: name.part1.rar, name.part2.rar, ...
	// or name.part1of3.rar, ...
	newPart = regexp.MustCompile(`(?i)^(.*)(\.part)([0-9]+)((?:of[0-9]+)?\.rar)$`)
	// oldPart is a later part of a RAR set named in the old way: name.rar, name.r00, name.r01,
	// ... name.r99, name.s00, ... A .z01 is a part of a split ZIP, not of a RAR set.
	oldPart = regexp.MustCompile(`(?i)^(.*)\.([r-y])([0-9]{2})$`)
)

func detect(name string) kind {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return kind{format: formatZip}
	case strings.HasSuffix(lower, ".7z"):
		return kind{format: format7z}
	case strings.HasSuffix(lower, ".rar"), oldPart.MatchString(name):
		return kind{format: formatRar}
	}
	for _, t := range tarSuffixes {
		if strings.HasSuffix(lower, t.suffix) {
			return kind{format: formatTar, compression: t.compression}
		}
	}
	return kind{}
}

// IsArchive reports whether a file of this name is one Extract opens: an archive, or a part of a
// RAR set as one is chosen (name.rar, name.partN.rar, name.r00 to name.r99). Later parts of the
// largest sets (name.s00, ...) are found from these; on their own, such names are more often
// other files, as firmware.s19 is.
func IsArchive(name string) bool {
	k := detect(name)
	if k.format != formatRar {
		return k.format != formatNone
	}
	m := oldPart.FindStringSubmatch(name)
	return m == nil || strings.EqualFold(m[2], "r")
}

// Stem is the name of the folder an archive opens into: its name without the extension, and
// without the part number for a RAR set.
func Stem(name string) string {
	stem := ""
	if m := newPart.FindStringSubmatch(name); m != nil {
		stem = m[1]
	} else if m := oldPart.FindStringSubmatch(name); m != nil {
		stem = m[1]
	} else {
		lower := strings.ToLower(name)
		suffixes := []string{".zip", ".7z", ".rar"}
		for _, t := range tarSuffixes {
			suffixes = append(suffixes, t.suffix)
		}
		// The longest suffix first: .tar.gz before .gz-less .tar.
		sort.Slice(suffixes, func(i, j int) bool { return len(suffixes[i]) > len(suffixes[j]) })
		for _, suffix := range suffixes {
			if strings.HasSuffix(lower, suffix) {
				stem = name[:len(name)-len(suffix)]
				break
			}
		}
	}
	stem = strings.TrimSpace(stem)
	if stem == "" || stem == "." || stem == ".." || strings.HasPrefix(stem, reservedPrefix) {
		return "arsiv"
	}
	return stem
}

// partNumber is the place of a part in its RAR set (the first is 1), and the set's key: what
// all of its parts' names have in common.
func partNumber(name string) (key string, number int, ok bool) {
	if m := newPart.FindStringSubmatch(name); m != nil {
		n, err := strconv.Atoi(m[3])
		if err != nil {
			return "", 0, false
		}
		return strings.ToLower(m[1] + m[2] + "#" + m[4]), n, true
	}
	if m := oldPart.FindStringSubmatch(name); m != nil {
		n, _ := strconv.Atoi(m[3])
		letter := strings.ToLower(m[2])[0]
		return strings.ToLower(m[1]) + "#old", int(letter-'r')*100 + n + 2, true
	}
	if strings.HasSuffix(strings.ToLower(name), ".rar") {
		return strings.ToLower(name[:len(name)-len(".rar")]) + "#old", 1, true
	}
	return "", 0, false
}

// continues reports whether dir holds the part that would follow last in its RAR set: a set
// read only up to last although that part is there ended before its end.
func continues(fsys fs.FS, dir, last string) (bool, error) {
	key, number, ok := partNumber(last)
	if !ok {
		return false, nil
	}
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if k, n, ok := partNumber(entry.Name()); ok && k == key && n == number+1 && !entry.IsDir() {
			return true, nil
		}
	}
	return false, nil
}

// firstPart returns the name of the first part of the RAR set that name is a part of, in dir.
func firstPart(fsys fs.FS, dir, name string) (string, error) {
	key, number, ok := partNumber(name)
	if !ok || number == 1 {
		return name, nil
	}
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if k, n, ok := partNumber(entry.Name()); ok && k == key && n == 1 && !entry.IsDir() {
			return entry.Name(), nil
		}
	}
	return "", fail(CodeMissingPart, errors.New("the first part is missing"))
}

// sniff reports whether the file name in fsys starts as an archive of its kind does.
func sniff(fsys fs.FS, name string, k kind) (bool, error) {
	f, err := fsys.Open(name)
	if err != nil {
		return false, err
	}
	defer f.Close()
	head := make([]byte, 262)
	n, err := io.ReadFull(f, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return false, err
	}
	head = head[:n]
	has := func(magic ...string) bool {
		for _, m := range magic {
			if bytes.HasPrefix(head, []byte(m)) {
				return true
			}
		}
		return false
	}

	switch k.format {
	case formatZip:
		return has("PK\x03\x04", "PK\x05\x06", "PK\x07\x08"), nil
	case formatRar:
		return has("Rar!\x1a\x07\x00", "Rar!\x1a\x07\x01\x00"), nil
	case format7z:
		return has("7z\xbc\xaf\x27\x1c"), nil
	}
	switch k.compression.(type) {
	case archives.Gz:
		return has("\x1f\x8b"), nil
	case archives.Bz2:
		return has("BZh"), nil
	case archives.Xz:
		return has("\xfd7zXZ\x00"), nil
	case archives.Zstd:
		return has("\x28\xb5\x2f\xfd"), nil
	}
	return len(head) >= 262 && bytes.Equal(head[257:262], []byte("ustar")), nil
}
