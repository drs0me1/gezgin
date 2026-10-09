// Package pack makes archives of files and folders: ZIP, or tar, plain or compressed. Create
// writes one on disk, whole or in volumes (name.zip.001, name.zip.002, ...); Write streams one,
// for a download. Only regular files and folders go in: a link goes in as what it leads to, when
// the source lets it be followed, and special files are left out. Every name in an archive is
// one Gezgin's unpack takes back.
package pack

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// Format is the kind of archive written.
type Format int

const (
	Zip Format = iota + 1
	Tar
)

// Item is a file or folder to pack: Path in the source, Name in the archive, "/"-separated. A
// folder with no Name puts what it holds at the archive's root.
type Item struct {
	Path string
	Name string
}

// Source is the tree items are packed from, as the caller lets it be seen. Its names are
// "/"-separated paths, as the items give them.
type Source interface {
	// Lstat describes name itself: a link as a link.
	Lstat(name string) (fs.FileInfo, error)
	// Stat describes what name leads to, and fails for a link the caller does not follow.
	Stat(name string) (fs.FileInfo, error)
	// ReadDir lists the names in a folder.
	ReadDir(name string) ([]string, error)
	// Open opens a file to read, without waiting on a special file.
	Open(name string) (fs.File, error)
	// Allowed reports whether name may go in the archive; link tells that it is a link, to be
	// judged by where it leads too.
	Allowed(name string, link bool) bool
}

// Code tells why a job stopped.
type Code string

const (
	CodeEmpty   Code = "empty"   // nothing to pack
	CodeEntries Code = "entries" // more files and folders than allowed
	CodeNoSpace Code = "noSpace" // the archive does not fit
	CodeVolumes Code = "volumes" // more volumes than allowed
	CodeChanged Code = "changed" // a file changed while it was packed
)

// Error is why a job stopped.
type Error struct {
	Code Code
	Err  error
}

func (e *Error) Error() string {
	if e.Err == nil {
		return "pack: " + string(e.Code)
	}
	return "pack: " + string(e.Code) + ": " + e.Err.Error()
}

func (e *Error) Unwrap() error { return e.Err }

func fail(code Code, err error) error {
	return &Error{Code: code, Err: err}
}

// Limits bound a job: the files and folders it packs, the archive's size, and how many volumes
// it may take. Zero is no limit, and 999 volumes.
type Limits struct {
	Entries int
	Bytes   int64
	Volumes int
}

// maxVolumes is the most volumes a set may have: their numbers have three digits.
const maxVolumes = 999

// Options of a job.
type Options struct {
	Format Format
	// Compress wraps a tar's stream in a compressor; nil leaves it plain.
	Compress func(io.Writer) (io.WriteCloser, error)
	// Volume splits what Create writes in volumes of this many bytes; 0 keeps one file.
	Volume int64
	// FileMode is the mode of the files Create makes.
	FileMode fs.FileMode
	Limits   Limits
	// Lenient takes a file that grows while it is packed as it was when the job began, as a
	// download does; otherwise any change stops the job.
	Lenient bool
	// Location is the time zone of a ZIP's MS-DOS times, which Windows shows as they are;
	// nil is the server's.
	Location *time.Location
	// Progress, when set, is told now and then what the job has done.
	Progress func(Progress)
}

// Progress is what a job has done so far. Files is the number of files and folders to pack and
// Total their bytes, both known once Planned; Entries and Bytes are how many of them are in.
type Progress struct {
	Planned bool  `json:"planned"`
	Files   int   `json:"files"`
	Total   int64 `json:"total"`
	Entries int   `json:"entries"`
	Bytes   int64 `json:"bytes"`
	// Skipped counts what was left out: what the rules refuse, links that lead nowhere or
	// where they may not, special files, and names no archive can hold.
	Skipped int `json:"skipped"`
	// Windows counts the names Windows cannot take as they are.
	Windows int `json:"windows"`
	// Volumes counts the files the archive was written in.
	Volumes int `json:"volumes"`
}

// reportEvery is how often a job tells its progress at most.
const reportEvery = 500 * time.Millisecond

// reservedPrefix starts the names Gezgin keeps for itself, which are never packed.
const reservedPrefix = ".gezgin-"

// maxDepth bounds how many folders deep an entry may lie, as unpack does.
const maxDepth = 64

// entry is a file or folder planned to be packed.
type entry struct {
	src  string      // its name in the source
	name string      // its name in the archive; a folder's ends with "/"
	info fs.FileInfo // what was found: a regular file or a folder
}

type job struct {
	ctx      context.Context
	src      Source
	opt      Options
	entries  []entry
	taken    map[string]bool // the names in the archive, without a folder's "/"
	folded   map[string]bool // the same in lower case, as Windows compares them
	least    int64           // the fewest bytes the archive takes: what goes in as it is
	progress Progress
	reported time.Time
	buf      []byte
}

func newJob(ctx context.Context, src Source, opt Options) *job {
	return &job{ctx: ctx, src: src, opt: opt, taken: map[string]bool{}, folded: map[string]bool{}}
}

func (j *job) report(now bool) {
	if j.opt.Progress == nil || (!now && time.Since(j.reported) < reportEvery) {
		return
	}
	j.reported = time.Now()
	j.opt.Progress(j.progress)
}

// plan finds what to pack, in a fixed order, before anything is written.
func (j *job) plan(items []Item) error {
	for _, item := range items {
		name, ok := itemName(item.Name)
		if !ok {
			j.skip()
			continue
		}
		if err := j.walk(item.Path, name, nil); err != nil {
			return err
		}
	}
	j.progress.Planned = true
	j.report(true)
	return nil
}

// walk plans p, to be named name in the archive; ancestors are the folders it lies in.
func (j *job) walk(p, name string, ancestors []fs.FileInfo) error {
	if err := j.ctx.Err(); err != nil {
		return err
	}
	info, err := j.src.Lstat(p)
	if err != nil {
		return j.unreadable(err)
	}
	link := info.Mode()&fs.ModeSymlink != 0
	if !j.src.Allowed(p, link) {
		j.skip()
		return nil
	}
	if link {
		if info, err = j.src.Stat(p); err != nil {
			// Leads nowhere, or out of what the user may see.
			j.skip()
			return nil
		}
	}

	switch {
	case info.IsDir():
		for _, a := range ancestors {
			if os.SameFile(a, info) {
				// A link to a folder it lies in.
				j.skip()
				return nil
			}
		}
		if name != "" {
			if strings.Count(name, "/")+1 > maxDepth {
				j.skip()
				return nil
			}
			if ok, err := j.add(p, name+"/", info); err != nil || !ok {
				return err
			}
		}
		names, err := j.src.ReadDir(p)
		if err != nil {
			return j.unreadable(err)
		}
		sort.Strings(names)
		ancestors = append(ancestors, info)
		for _, n := range names {
			if strings.HasPrefix(n, reservedPrefix) {
				continue
			}
			part, ok := archiveName(n)
			if !ok {
				j.skip()
				continue
			}
			child := part
			if name != "" {
				child = name + "/" + part
			}
			if err := j.walk(path.Join(p, n), child, ancestors); err != nil {
				return err
			}
		}
		return nil
	case info.Mode().IsRegular():
		if name == "" {
			// A file has to have a name.
			name, _ = archiveName(path.Base(p))
		}
		_, err := j.add(p, name, info)
		return err
	}
	j.skip()
	return nil
}

// unreadable skips what went away or may not be read since it was listed.
func (j *job) unreadable(err error) error {
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrPermission) {
		j.skip()
		return nil
	}
	return err
}

func (j *job) skip() {
	j.progress.Skipped++
}

// add plans an entry, unless its name is taken: two names can become one once made safe.
func (j *job) add(p, name string, info fs.FileInfo) (bool, error) {
	key := strings.TrimSuffix(name, "/")
	if j.taken[key] {
		j.skip()
		return false, nil
	}
	if j.opt.Limits.Entries > 0 && len(j.entries) >= j.opt.Limits.Entries {
		return false, fail(CodeEntries, nil)
	}
	j.taken[key] = true
	folded := strings.ToLower(key)
	if j.folded[folded] || !windowsName(key) {
		j.progress.Windows++
	}
	j.folded[folded] = true

	j.entries = append(j.entries, entry{src: p, name: name, info: info})
	j.progress.Files++
	j.least += overhead(j.opt.Format, name)
	if info.Mode().IsRegular() {
		j.progress.Total += info.Size()
		if j.opt.Format == Tar && j.opt.Compress == nil || j.opt.Format == Zip && stored(name) {
			j.least += info.Size()
		}
	}
	if j.opt.Limits.Bytes > 0 && j.least > j.opt.Limits.Bytes {
		return false, fail(CodeNoSpace, nil)
	}
	if j.opt.Volume > 0 && j.volumes(j.least) > j.maxVolumes() {
		return false, fail(CodeVolumes, nil)
	}
	j.report(false)
	return true, nil
}

func (j *job) maxVolumes() int {
	if j.opt.Limits.Volumes > 0 {
		return j.opt.Limits.Volumes
	}
	return maxVolumes
}

func (j *job) volumes(size int64) int {
	return int((size + j.opt.Volume - 1) / j.opt.Volume)
}

// overhead is about the most an entry's headers take beside its data.
func overhead(format Format, name string) int64 {
	if format == Zip {
		// A local and a central header, with the name and the extra fields in each.
		return 160 + 2*int64(len(name))
	}
	// A header, its padding, and a PAX header for a long or non-ASCII name.
	return 2048 + 2*int64(len(name))
}

// archiveName makes a name from the source one an archive can hold, and Gezgin's unpack and
// Windows take apart the same way: a backslash, which Windows reads as a separator, becomes
// "_". A name that is not UTF-8, or holds a control character, cannot be held.
func archiveName(name string) (string, bool) {
	if name == "" || name == "." || name == ".." || len(name) > 255 || !utf8.ValidString(name) {
		return "", false
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f || r == '/' {
			return "", false
		}
	}
	return strings.ReplaceAll(name, `\`, "_"), true
}

// itemName makes an item's name one an archive can hold, part by part.
func itemName(name string) (string, bool) {
	if name == "" {
		return "", true
	}
	parts := strings.Split(name, "/")
	for i, part := range parts {
		var ok bool
		if parts[i], ok = archiveName(part); !ok {
			return "", false
		}
	}
	return strings.Join(parts, "/"), true
}

// windowsName reports whether Windows takes the path as it is: no reserved device name, no
// character it refuses, no part ending in a dot or a space, and short enough.
func windowsName(p string) bool {
	if len(p) > 240 {
		return false
	}
	for _, part := range strings.Split(p, "/") {
		if strings.ContainsAny(part, `<>:"|?*`) || strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") {
			return false
		}
		device := strings.ToUpper(part)
		if i := strings.IndexByte(device, '.'); i >= 0 {
			device = device[:i]
		}
		device = strings.TrimRight(device, " ")
		switch {
		case device == "CON", device == "PRN", device == "AUX", device == "NUL":
			return false
		case len(device) == 4 && (strings.HasPrefix(device, "COM") || strings.HasPrefix(device, "LPT")) &&
			device[3] >= '1' && device[3] <= '9':
			return false
		}
	}
	return true
}

// writer writes the entries of an archive.
type writer interface {
	dir(e entry) error
	// file writes a file, whose data writes its content into the writer it is given.
	file(e entry, data func(io.Writer) error) error
	close() error
}

// write writes the planned entries.
func (j *job) write(w writer) error {
	j.buf = make([]byte, 1<<20)
	for _, e := range j.entries {
		if err := j.ctx.Err(); err != nil {
			return err
		}
		var err error
		if e.info.IsDir() {
			err = w.dir(e)
		} else {
			err = w.file(e, func(dst io.Writer) error { return j.copy(e, dst) })
		}
		if err != nil {
			return err
		}
		j.progress.Entries++
		j.report(false)
	}
	return w.close()
}

// copy writes a file's data as it was planned: the same file, of the same size, unchanged
// until it is read whole. A lenient job takes a file that grew as far as it was planned.
func (j *job) copy(e entry, dst io.Writer) error {
	f, err := j.src.Open(e.src)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fail(CodeChanged, err)
		}
		return err
	}
	defer f.Close()
	if info, err := f.Stat(); err != nil {
		return err
	} else if !j.unchanged(e.info, info, false) {
		return fail(CodeChanged, fmt.Errorf("%s changed", e.src))
	}

	for left := e.info.Size(); left > 0; {
		if err := j.ctx.Err(); err != nil {
			return err
		}
		chunk := j.buf
		if int64(len(chunk)) > left {
			chunk = chunk[:left]
		}
		n, err := io.ReadFull(f, chunk)
		if n > 0 {
			if _, werr := dst.Write(chunk[:n]); werr != nil {
				return werr
			}
			left -= int64(n)
			j.progress.Bytes += int64(n)
			j.report(false)
		}
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return fail(CodeChanged, fmt.Errorf("%s got shorter", e.src))
		}
		if err != nil {
			return err
		}
	}
	if j.opt.Lenient {
		return nil
	}
	if n, _ := f.Read(j.buf[:1]); n > 0 {
		return fail(CodeChanged, fmt.Errorf("%s got longer", e.src))
	}
	if info, err := f.Stat(); err != nil {
		return err
	} else if !j.unchanged(e.info, info, true) {
		return fail(CodeChanged, fmt.Errorf("%s changed", e.src))
	}
	return nil
}

// unchanged reports whether now is the file planned: a lenient job asks only that it be the
// same file and not shorter.
func (j *job) unchanged(planned, now fs.FileInfo, after bool) bool {
	if !now.Mode().IsRegular() || !os.SameFile(planned, now) {
		return false
	}
	if j.opt.Lenient {
		return after || now.Size() >= planned.Size()
	}
	return now.Size() == planned.Size() && now.ModTime().Equal(planned.ModTime()) &&
		changeTime(now) == changeTime(planned)
}

// newWriter makes the writer of the job's format, writing into out.
func (j *job) newWriter(out *output) (writer, error) {
	if j.opt.Format == Zip {
		return newZipWriter(out, j.opt.Location), nil
	}
	return newTarWriter(out, j.opt.Compress)
}

// Create packs items from src into a new archive in dir, named name: one file, or, with a
// volume size, the volumes name.001, name.002, ... of it, unless it fits in one. It returns
// the names of the files made, which a failed job leaves for the caller to remove with dir.
func Create(ctx context.Context, src Source, items []Item, dir, name string, opt Options) ([]string, Progress, error) {
	j := newJob(ctx, src, opt)
	if err := j.plan(items); err != nil {
		return nil, j.progress, err
	}
	if len(j.entries) == 0 {
		return nil, j.progress, fail(CodeEmpty, nil)
	}
	limits := opt.Limits
	if opt.Volume > 0 && limits.Volumes == 0 {
		limits.Volumes = maxVolumes
	}
	out := newOutput(dir, name, opt.Volume, limits)
	w, err := j.newWriter(out)
	if err == nil {
		err = j.write(w)
	}
	made, closeErr := out.finish(opt.FileMode, err == nil)
	if err == nil {
		err = closeErr
	}
	j.progress.Volumes = len(made)
	j.report(true)
	return made, j.progress, err
}

// Write packs items from src into w as it goes, for a download; it does not seek. With nothing
// to pack, it writes an empty archive.
func Write(ctx context.Context, src Source, items []Item, w io.Writer, opt Options) (Progress, error) {
	j := newJob(ctx, src, opt)
	if err := j.plan(items); err != nil {
		return j.progress, err
	}
	out := newStream(w)
	aw, err := j.newWriter(out)
	if err == nil {
		err = j.write(aw)
	}
	if err == nil {
		err = out.flush()
	}
	return j.progress, err
}

// fileFor is the path of a file named name in dir.
func fileFor(dir, name string) string {
	return filepath.Join(dir, name)
}
