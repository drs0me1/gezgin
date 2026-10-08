// Package unpack opens archives into a folder, in Go and without any other program (Gezgin):
// ZIP, RAR with its sets of parts, 7z and tar, plain or compressed, and then the archives that
// come out of them, layer by layer.
package unpack

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
	"unicode"
)

// Code tells why a job stopped, for the user to read in their own language.
type Code string

const (
	CodeNotArchive    Code = "notArchive"    // not an archive of a kind Extract opens
	CodePassword      Code = "password"      // encrypted, and no password was given
	CodeWrongPassword Code = "wrongPassword" // the password does not open it
	CodeCorrupt       Code = "corrupt"       // damaged: a checksum or a structure is wrong
	CodeMissingPart   Code = "missingPart"   // a part of a RAR set is missing
	CodeUnsafe        Code = "unsafe"        // a name leads out or is used twice, or a link or device
	CodeUnsupported   Code = "unsupported"   // an encrypted ZIP, an unknown method, a dictionary too large
	CodeEntries       Code = "entries"       // more files and folders than allowed
	CodeLayers        Code = "layers"        // archives nested deeper than allowed
	CodeNoSpace       Code = "noSpace"       // the disk is full, or would be
	CodeChanged       Code = "changed"       // a source file changed while it was read
)

// Error is why a job stopped.
type Error struct {
	Code Code
	Err  error
}

func (e *Error) Error() string {
	if e.Err == nil {
		return "unpack: " + string(e.Code)
	}
	return "unpack: " + string(e.Code) + ": " + e.Err.Error()
}

func (e *Error) Unwrap() error { return e.Err }

func fail(code Code, err error) error {
	return &Error{Code: code, Err: err}
}

// reservedPrefix starts the names Gezgin keeps for itself, which no archive may bring in.
const reservedPrefix = ".gezgin-"

// maxDepth bounds how many folders deep an entry may lie in its archive.
const maxDepth = 64

// maxDictionary bounds the memory a RAR file's decoding may take.
const maxDictionary = 1 << 30

// Limits bound a job. Entries counts the files and folders it makes, all layers together;
// Layers how deep archives may lie in archives, the chosen ones being the first layer; Bytes
// how much it may write.
type Limits struct {
	Entries int
	Layers  int
	Bytes   int64
}

// Progress is what a job has done so far.
type Progress struct {
	Bytes    int64 `json:"bytes"`
	Entries  int   `json:"entries"`
	Archives int   `json:"archives"`
}

// Options of a job.
type Options struct {
	// Password opens encrypted RAR and 7z archives, the inner ones too.
	Password string
	FileMode fs.FileMode
	DirMode  fs.FileMode
	Limits   Limits
	// Progress, when set, is told now and then what the job has done.
	Progress func(Progress)
}

// reportEvery is how often a job tells its progress at most.
const reportEvery = 500 * time.Millisecond

// Extract opens the archives named in src into dst, a folder it makes, and then each archive
// that comes out of them into a folder beside it, named after it, layer by layer. A RAR set is
// named by any of its parts. Nothing but regular files and folders is written, all under dst.
// A job stops at the first problem; whatever it wrote is then to be thrown away with dst.
func Extract(ctx context.Context, src fs.FS, names []string, dst string, opt Options) (Progress, error) {
	j := &job{ctx: ctx, opt: opt, dst: dst, buf: make([]byte, 1<<20)}
	if err := os.Mkdir(dst, 0o700); err != nil {
		return j.progress, err
	}
	j.dirs = append(j.dirs, dst)

	units, err := j.chosen(src, names)
	for layer := 1; err == nil && len(units) > 0; layer++ {
		if layer > opt.Limits.Layers {
			err = fail(CodeLayers, nil)
			break
		}
		j.found = j.found[:0]
		for _, u := range units {
			if err = j.open(u); err != nil {
				break
			}
		}
		if err == nil {
			units, err = j.inner()
		}
	}
	if err != nil {
		return j.progress, err
	}

	// The folders were private while they filled.
	for _, dir := range j.dirs {
		if err = os.Chmod(dir, opt.DirMode); err != nil {
			return j.progress, err
		}
	}
	j.report(true)
	return j.progress, nil
}

type job struct {
	ctx      context.Context
	opt      Options
	dst      string
	buf      []byte
	progress Progress
	reported time.Time
	dirs     []string // the folders made, which get their mode at the end
	found    []string // the files of this layer that look like archives, under dst
}

// unit is an archive to open: a file, or a RAR set by its first part.
type unit struct {
	fsys fs.FS  // where it is
	dir  string // its folder in fsys
	name string
	kind kind
	out  string // the folder it opens into, under dst
}

func (u unit) file() string { return path.Join(u.dir, u.name) }

// chosen plans the archives the user chose, which open into dst itself.
func (j *job) chosen(src fs.FS, names []string) ([]unit, error) {
	seen := map[string]bool{}
	var units []unit
	for _, name := range names {
		k := detect(name)
		if k.format == formatNone {
			return nil, fail(CodeNotArchive, nil)
		}
		first := name
		if k.format == formatRar {
			var err error
			if first, err = firstPart(src, ".", name); err != nil {
				return nil, err
			}
		}
		if seen[first] {
			continue
		}
		seen[first] = true
		if ok, err := sniff(src, first, k); err != nil {
			return nil, err
		} else if !ok {
			return nil, fail(CodeNotArchive, nil)
		}
		units = append(units, unit{fsys: src, dir: ".", name: first, kind: k, out: "."})
	}
	return units, nil
}

// inner plans the archives that came out of the last layer. Each opens into a new folder beside
// it; a file named like an archive that does not start as one stays as it is.
func (j *job) inner() ([]unit, error) {
	dstFS := os.DirFS(j.dst)
	found := append([]string(nil), j.found...)
	sort.Strings(found)
	seen := map[string]bool{}
	var units []unit
	for _, file := range found {
		dir, name := path.Dir(file), path.Base(file)
		k := detect(name)
		first := name
		if k.format == formatRar {
			var err error
			if first, err = firstPart(dstFS, dir, name); err != nil {
				return nil, err
			}
		}
		if seen[path.Join(dir, first)] {
			continue
		}
		seen[path.Join(dir, first)] = true
		if ok, err := sniff(dstFS, path.Join(dir, first), k); err != nil {
			return nil, err
		} else if !ok {
			continue
		}
		out, err := j.folder(dir, Stem(first))
		if err != nil {
			return nil, err
		}
		units = append(units, unit{fsys: dstFS, dir: dir, name: first, kind: k, out: out})
	}
	return units, nil
}

// folder makes a new folder named name in dir, under dst, with a number when the name is taken.
func (j *job) folder(dir, name string) (string, error) {
	for i := 1; i <= 100; i++ {
		candidate := name
		if i > 1 {
			candidate = fmt.Sprintf("%s (%d)", name, i)
		}
		rel := path.Join(dir, candidate)
		err := os.Mkdir(j.onDisk(rel), 0o700)
		if err == nil {
			j.dirs = append(j.dirs, j.onDisk(rel))
			return rel, j.entry()
		}
		if !errors.Is(err, fs.ErrExist) {
			return "", err
		}
	}
	return "", fail(CodeUnsafe, errors.New("no free folder name"))
}

func (j *job) onDisk(rel string) string {
	return filepath.Join(j.dst, filepath.FromSlash(rel))
}

// open opens an archive into its folder, and makes sure its files did not change meanwhile.
func (j *job) open(u unit) error {
	j.progress.Archives++
	j.report(false)
	w := &watchFS{FS: u.fsys}
	var err error
	switch u.kind.format {
	case formatRar:
		err = j.rar(w, u)
	case formatZip:
		err = j.zip(w, u)
	case format7z:
		err = j.sevenZip(w, u)
	default:
		err = j.tar(w, u)
	}
	if err != nil {
		return err
	}
	return w.unchanged()
}

// entry is a file or folder in an archive.
type entry struct {
	name string
	dir  bool
	mode fs.FileMode // its type
	time time.Time
	open func() (io.ReadCloser, error)
}

// put writes an entry into out.
func (j *job) put(out string, e entry) error {
	if err := j.ctx.Err(); err != nil {
		return err
	}
	parts, err := clean(utf8Name(e.name))
	if err != nil {
		return err
	}
	if len(parts) == 0 {
		// The archive's own folder, as "./" in a tar.
		return nil
	}
	if e.dir || e.mode.IsDir() {
		return j.mkdirs(out, parts)
	}
	if !e.mode.IsRegular() {
		return fail(CodeUnsafe, errors.New("a link or a special file"))
	}
	if err := j.mkdirs(out, parts[:len(parts)-1]); err != nil {
		return err
	}
	return j.write(path.Join(out, path.Join(parts...)), e)
}

// clean checks an entry's name and splits it into its folders and its own name. "." parts
// are dropped; a name that leads out of the archive's folder or that Gezgin keeps is refused.
func clean(name string) ([]string, error) {
	if name == "" || strings.HasPrefix(name, "/") {
		return nil, fail(CodeUnsafe, errors.New("an absolute or empty name"))
	}
	var parts []string
	for _, part := range strings.Split(name, "/") {
		switch {
		case part == "" || part == ".":
			continue
		case part == ".." || len(part) > 255 || strings.HasPrefix(part, reservedPrefix) ||
			strings.IndexFunc(part, unicode.IsControl) >= 0:
			return nil, fail(CodeUnsafe, errors.New("a name that leads out or is not allowed"))
		}
		parts = append(parts, part)
	}
	if len(parts) > maxDepth {
		return nil, fail(CodeUnsafe, errors.New("a name too deep"))
	}
	return parts, nil
}

// mkdirs makes the folders parts in out, as far as they are not there.
func (j *job) mkdirs(out string, parts []string) error {
	for i := range parts {
		full := j.onDisk(path.Join(out, path.Join(parts[:i+1]...)))
		err := os.Mkdir(full, 0o700)
		switch {
		case err == nil:
			j.dirs = append(j.dirs, full)
			if err = j.entry(); err != nil {
				return err
			}
		case errors.Is(err, fs.ErrExist):
			if info, statErr := os.Lstat(full); statErr != nil || !info.IsDir() {
				return fail(CodeUnsafe, errors.New("a name used for a file and a folder"))
			}
		default:
			return err
		}
	}
	return nil
}

// write writes a file entry at rel, under dst.
func (j *job) write(rel string, e entry) error {
	full := j.onDisk(rel)
	f, err := os.OpenFile(full, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return fail(CodeUnsafe, errors.New("the same name twice"))
	}
	if err != nil {
		return err
	}
	if err = j.entry(); err == nil {
		err = j.copy(f, e)
	}
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Chmod(full, j.opt.FileMode)
	}
	if err == nil && !e.time.IsZero() {
		err = os.Chtimes(full, e.time, e.time)
	}
	if err != nil {
		return err
	}
	if IsArchive(path.Base(rel)) {
		j.found = append(j.found, rel)
	}
	return nil
}

func (j *job) copy(f *os.File, e entry) error {
	r, err := e.open()
	if err != nil {
		return err
	}
	defer r.Close()
	_, err = io.CopyBuffer(&limitedWriter{j: j, f: f}, r, j.buf)
	return err
}

// limitedWriter writes a file's content as long as the job may.
type limitedWriter struct {
	j *job
	f *os.File
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	if err := w.j.ctx.Err(); err != nil {
		return 0, err
	}
	if w.j.progress.Bytes+int64(len(p)) > w.j.opt.Limits.Bytes {
		return 0, fail(CodeNoSpace, errors.New("over the space the job may take"))
	}
	n, err := w.f.Write(p)
	w.j.progress.Bytes += int64(n)
	w.j.report(false)
	return n, err
}

func (j *job) entry() error {
	j.progress.Entries++
	if j.progress.Entries > j.opt.Limits.Entries {
		return fail(CodeEntries, nil)
	}
	return nil
}

func (j *job) report(now bool) {
	if j.opt.Progress == nil || (!now && time.Since(j.reported) < reportEvery) {
		return
	}
	j.reported = time.Now()
	j.opt.Progress(j.progress)
}

// watchFS notes the files an archive is read from, so that a change while they are read is
// found.
type watchFS struct {
	fs.FS
	seen []watched
}

type watched struct {
	name    string
	size    int64
	modTime time.Time
}

func (w *watchFS) Open(name string) (fs.File, error) {
	f, err := w.FS.Open(name)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() {
		f.Close()
		return nil, fail(CodeNotArchive, errors.New("not a regular file"))
	}
	w.seen = append(w.seen, watched{name, info.Size(), info.ModTime()})
	return f, nil
}

func (w *watchFS) unchanged() error {
	for _, s := range w.seen {
		info, err := fs.Stat(w.FS, s.name)
		if err != nil || info.Size() != s.size || !info.ModTime().Equal(s.modTime) {
			return fail(CodeChanged, nil)
		}
	}
	return nil
}
