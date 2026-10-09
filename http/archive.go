package fbhttp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"
	_ "time/tzdata" // the zones browsers name, which the image does not have
	"unicode"
	"unicode/utf8"

	"github.com/gorilla/mux"
	"github.com/spf13/afero"

	"github.com/filebrowser/filebrowser/v2/pack"
	"github.com/filebrowser/filebrowser/v2/trash"
	"github.com/filebrowser/filebrowser/v2/unpack"
)

// Gezgin opens and makes archives on the server. A job opens the chosen archives, and the
// archives in them, into a folder beside them; or it packs the chosen files and folders into an
// archive beside them, whole or in volumes. It builds its result in ArchiveDir and puts it in
// place once complete.

// ArchiveDir is the folder at the server root where archive jobs build their results,
// unreachable through any path.
const ArchiveDir = ".gezgin-arsiv"

const (
	archiveHistory = 20      // jobs kept to be listed
	archiveItems   = 100     // archives a job may be given
	archiveEntries = 10000   // files and folders a job may make
	archiveLayers  = 5       // how deep archives may lie in archives
	archiveReserve = 1 << 30 // space a job leaves free on the disk
)

// Job kinds.
const (
	archiveExtract = "extract"
	archiveCreate  = "create"
)

// Job states.
const (
	archiveRunning   = "running"
	archiveDone      = "done"
	archiveFailed    = "failed"
	archiveCancelled = "cancelled"
)

var (
	errArchiveBusy      = errors.New("another archive job is running")
	errArchiveRules     = errors.New("the rules refuse a path of the result")
	errArchiveOtherDisk = errors.New("the folder is on another disk")
)

// ArchiveJobs runs archive jobs, one at a time, and keeps the latest to be listed.
type ArchiveJobs struct {
	dir  string
	ctx  context.Context
	stop context.CancelFunc
	wg   sync.WaitGroup

	mu   sync.Mutex
	jobs []*archiveJob // the latest first
	busy bool
}

type archiveJob struct {
	ID     string   `json:"id"`
	Kind   string   `json:"kind"`
	Folder string   `json:"folder"`
	Names  []string `json:"names"` // the items chosen in the folder
	State  string   `json:"state"`
	// Error tells why a failed job stopped (unpack's and pack's codes, "rules", "otherDisk" or
	// "internal").
	Error string `json:"error,omitempty"`
	// Result is what the job made: the folder an extraction opened into, or the archive made,
	// the first of its volumes for a set.
	Result string `json:"result,omitempty"`

	Bytes    int64 `json:"bytes"`
	Entries  int   `json:"entries"`
	Archives int   `json:"archives"`

	// A creation's archive and format, and its progress as pack tells it.
	Name    string `json:"name,omitempty"`
	Format  string `json:"format,omitempty"`
	Planned bool   `json:"planned,omitempty"`
	Files   int    `json:"files,omitempty"`
	Total   int64  `json:"total,omitempty"`
	Skipped int    `json:"skipped,omitempty"`
	Windows int    `json:"windows,omitempty"`
	Volumes int    `json:"volumes,omitempty"`

	Started  time.Time  `json:"started"`
	Finished *time.Time `json:"finished,omitempty"`

	user   uint
	cancel context.CancelFunc
}

// jobWork is what a job does in its stage folder; it returns the job's result.
type jobWork func(ctx context.Context, stage string) (string, error)

// NewArchiveJobs runs the archive jobs in dir, which it empties of what a stopped server left.
func NewArchiveJobs(dir string) *ArchiveJobs {
	if err := os.RemoveAll(dir); err != nil {
		log.Printf("WARNING: could not clear %s: %v", dir, err)
	}
	ctx, stop := context.WithCancel(context.Background())
	return &ArchiveJobs{dir: dir, ctx: ctx, stop: stop}
}

// Close stops the running job and waits until it has cleared up.
func (a *ArchiveJobs) Close() {
	a.stop()
	a.wg.Wait()
}

// list returns copies of the user's jobs, the latest first.
func (a *ArchiveJobs) list(user uint) []archiveJob {
	a.mu.Lock()
	defer a.mu.Unlock()
	jobs := []archiveJob{}
	for _, job := range a.jobs {
		if job.user == user {
			jobs = append(jobs, *job)
		}
	}
	return jobs
}

// start runs a job, unless one runs; work makes it do its kind of work.
func (a *ArchiveJobs) start(d *data, job *archiveJob, work func(job *archiveJob) jobWork) (archiveJob, error) {
	random := make([]byte, 12)
	if _, err := rand.Read(random); err != nil {
		return archiveJob{}, err
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.busy {
		return archiveJob{}, errArchiveBusy
	}
	ctx, cancel := context.WithCancel(a.ctx)
	job.ID, job.State, job.Started = hex.EncodeToString(random), archiveRunning, time.Now().UTC()
	job.user, job.cancel = d.user.ID, cancel
	a.jobs = append([]*archiveJob{job}, a.jobs...)
	if len(a.jobs) > archiveHistory {
		a.jobs = a.jobs[:archiveHistory]
	}
	a.busy = true
	a.wg.Add(1)
	go a.run(ctx, job, work(job))
	return *job, nil
}

// update changes a job's progress, as listings may read it meanwhile.
func (a *ArchiveJobs) update(change func()) {
	a.mu.Lock()
	defer a.mu.Unlock()
	change()
}

func (a *ArchiveJobs) cancel(user uint, id string) (bool, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, job := range a.jobs {
		if job.ID == id && job.user == user {
			if job.State != archiveRunning {
				return true, false
			}
			job.cancel()
			return true, true
		}
	}
	return false, false
}

func (a *ArchiveJobs) run(ctx context.Context, job *archiveJob, work jobWork) {
	defer a.wg.Done()
	stage := filepath.Join(a.dir, job.ID)
	result, err := a.guarded(ctx, job, stage, work)
	if rmErr := os.RemoveAll(stage); rmErr != nil {
		log.Printf("WARNING: could not clear %s: %v", stage, rmErr)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	finished := time.Now().UTC()
	job.Finished = &finished
	switch {
	case err == nil:
		job.State, job.Result = archiveDone, result
	case errors.Is(err, context.Canceled):
		job.State = archiveCancelled
	default:
		job.State, job.Error = archiveFailed, archiveCode(err)
		if job.Error == "internal" {
			log.Printf("ERROR: archive job %s: %q", job.ID, err.Error())
		}
	}
	a.busy = false
	job.cancel()
}

// guarded does a job's work in a new stage folder, on the disk of the job's folder: the result
// is put in place by a rename, which does not cross disks. A panic in a decoder or an encoder
// fails the job, not the server.
func (a *ArchiveJobs) guarded(ctx context.Context, job *archiveJob, stage string, work jobWork) (result string, err error) {
	defer func() {
		if p := recover(); p != nil {
			log.Printf("ERROR: archive job %s: %v\n%s", job.ID, p, debug.Stack())
			err = fmt.Errorf("panic: %v", p)
		}
	}()

	if err = os.MkdirAll(a.dir, 0o700); err != nil {
		return "", err
	}
	if err = os.Mkdir(stage, 0o700); err != nil {
		return "", err
	}
	return work(ctx, stage)
}

// extract opens the job's archives into a folder built in stage, and puts it in place.
func (a *ArchiveJobs) extract(d *data, password string, budget int64) func(job *archiveJob) jobWork {
	return func(job *archiveJob) jobWork {
		return func(ctx context.Context, stage string) (string, error) {
			if !sameDisk(stage, d.user.FullPath(job.Folder)) {
				return "", errArchiveOtherDisk
			}
			src := archiveSource{FS: afero.NewIOFS(afero.NewBasePathFs(d.user.Fs, job.Folder)), d: d, folder: job.Folder}
			built := filepath.Join(stage, "result")
			opt := unpack.Options{
				Password: password,
				FileMode: d.settings.FileMode,
				DirMode:  d.settings.DirMode,
				Limits:   unpack.Limits{Entries: archiveEntries, Layers: archiveLayers, Bytes: budget},
				Progress: func(p unpack.Progress) {
					a.update(func() { job.Bytes, job.Entries, job.Archives = p.Bytes, p.Entries, p.Archives })
				},
			}
			progress, err := unpack.Extract(ctx, src, job.Names, built, opt)
			opt.Progress(progress)
			if err != nil {
				return "", err
			}
			return publishArchive(d, built, job.Folder, unpack.Stem(job.Names[0]))
		}
	}
}

// archiveFormat is a kind of archive a job makes.
type archiveFormat struct {
	ext      string
	format   pack.Format
	compress func(io.Writer) (io.WriteCloser, error)
}

var archiveFormats = map[string]archiveFormat{
	"zip":   {".zip", pack.Zip, nil},
	"tar":   {".tar", pack.Tar, nil},
	"targz": {".tar.gz", pack.Tar, pack.Gzip},
}

// archiveCreation is what a creation job makes: an archive named name plus its format's
// extension, in volumes of volume bytes when that is set.
type archiveCreation struct {
	name   string
	format string
	volume int64
	zone   *time.Location
	folder bool // the one item chosen is a folder, whose contents go at the archive's root
	budget int64
}

// create packs the job's items into an archive made in stage, and puts it in place.
func (a *ArchiveJobs) create(d *data, c archiveCreation) func(job *archiveJob) jobWork {
	return func(job *archiveJob) jobWork {
		return func(ctx context.Context, stage string) (string, error) {
			if !sameDisk(stage, d.user.FullPath(job.Folder)) {
				return "", errArchiveOtherDisk
			}
			items := make([]pack.Item, len(job.Names))
			for i, name := range job.Names {
				items[i] = pack.Item{Path: path.Join(job.Folder, name), Name: name}
			}
			if c.folder {
				items[0].Name = ""
			}
			f := archiveFormats[c.format]
			opt := pack.Options{
				Format:   f.format,
				Compress: f.compress,
				Volume:   c.volume,
				FileMode: d.settings.FileMode,
				Limits:   pack.Limits{Entries: archiveEntries, Bytes: c.budget},
				Location: c.zone,
				Progress: func(p pack.Progress) {
					a.update(func() {
						job.Bytes, job.Entries, job.Planned, job.Files = p.Bytes, p.Entries, p.Planned, p.Files
						job.Total, job.Skipped, job.Windows, job.Volumes = p.Total, p.Skipped, p.Windows, p.Volumes
					})
				},
			}
			made, progress, err := pack.Create(ctx, newPackSource(d), items, stage, c.name+f.ext, opt)
			opt.Progress(progress)
			if err != nil {
				return "", err
			}
			return publishFiles(d, stage, made, job.Folder, c.name, f.ext)
		}
	}
}

func archiveCode(err error) string {
	var e *unpack.Error
	var pe *pack.Error
	switch {
	case errors.As(err, &e):
		return string(e.Code)
	case errors.As(err, &pe):
		return string(pe.Code)
	case errors.Is(err, errArchiveRules):
		return "rules"
	case errors.Is(err, errArchiveOtherDisk):
		return "otherDisk"
	case errors.Is(err, syscall.ENOSPC):
		return string(unpack.CodeNoSpace)
	}
	return "internal"
}

// archiveSource is the folder of the chosen archives as the user sees it: what the rules
// refuse is not there.
type archiveSource struct {
	fs.FS
	d      *data
	folder string
}

func (s archiveSource) allowed(name string) bool {
	return s.d.Check(path.Join(s.folder, name))
}

func (s archiveSource) Open(name string) (fs.File, error) {
	if !s.allowed(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return s.FS.Open(name)
}

func (s archiveSource) Stat(name string) (fs.FileInfo, error) {
	if !s.allowed(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
	}
	return fs.Stat(s.FS, name)
}

func (s archiveSource) ReadDir(name string) ([]fs.DirEntry, error) {
	if !s.allowed(name) {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
	}
	entries, err := fs.ReadDir(s.FS, name)
	shown := entries[:0]
	for _, entry := range entries {
		if s.allowed(path.Join(name, entry.Name())) {
			shown = append(shown, entry)
		}
	}
	return shown, err
}

// packSource is what a user packs: their files as they see them, through their file system,
// which refuses a link out of their scope. A link never leads into Gezgin's own folders,
// whatever its name.
type packSource struct {
	d        *data
	reserved []string // the real paths of Gezgin's folders
}

func newPackSource(d *data) packSource {
	root := d.server.Root
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	s := packSource{d: d}
	for _, dir := range []string{trash.Dir, UploadsDir, ArchiveDir} {
		s.reserved = append(s.reserved, filepath.Join(root, dir))
	}
	return s
}

func (s packSource) Lstat(name string) (fs.FileInfo, error) { return userLstat(s.d)(name) }

func (s packSource) Stat(name string) (fs.FileInfo, error) { return s.d.user.Fs.Stat(name) }

func (s packSource) ReadDir(name string) ([]string, error) {
	f, err := s.d.user.Fs.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.Readdirnames(-1)
}

// Open opens a file without waiting on it, should it have become a FIFO.
func (s packSource) Open(name string) (fs.File, error) {
	return s.d.user.Fs.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}

func (s packSource) Allowed(name string, link bool) bool {
	if !s.d.Check(name) {
		return false
	}
	if !link {
		return true
	}
	target, err := filepath.EvalSymlinks(s.d.user.FullPath(name))
	if err != nil {
		return false
	}
	for _, dir := range s.reserved {
		if s.d.server.CaseInsensitiveFs {
			target, dir = strings.ToLower(target), strings.ToLower(dir)
		}
		if target == dir || strings.HasPrefix(target, dir+string(filepath.Separator)) {
			return false
		}
	}
	return true
}

// userLstat describes a path through the user's file system, a link as a link.
func userLstat(d *data) func(string) (os.FileInfo, error) {
	if l, ok := d.user.Fs.(afero.Lstater); ok {
		return func(p string) (os.FileInfo, error) {
			info, _, err := l.LstatIfPossible(p)
			return info, err
		}
	}
	return d.user.Fs.Stat
}

// publishArchive puts the folder built in place in folder, named name or, when that is taken,
// with a number. The user's rules hold for every path in it.
func publishArchive(d *data, built, folder, name string) (string, error) {
	lstat := userLstat(d)
	for i := 1; i <= 100; i++ {
		target := path.Join(folder, name)
		if i > 1 {
			target = path.Join(folder, fmt.Sprintf("%s (%d)", name, i))
		}
		// Through the user's file system, which refuses a folder that leads out of the scope.
		if _, err := lstat(target); err == nil {
			continue
		} else if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		if err := checkBuilt(d, built, target); err != nil {
			return "", err
		}
		err := renameNoReplace(built, d.user.FullPath(target))
		switch {
		case errors.Is(err, fs.ErrExist):
			continue
		case errors.Is(err, syscall.EXDEV):
			return "", errArchiveOtherDisk
		case err != nil:
			return "", err
		}
		return target, nil
	}
	return "", errors.New("no free name for the result")
}

// publishFiles puts the files made in stage in place in folder: an archive as name plus ext, or
// the volumes of one as name plus ext plus their numbers. When a name is taken, or a volume of
// a set of that name is there, which would read as one of the new set's, the names take a
// number. The user's rules hold for every name.
func publishFiles(d *data, stage string, made []string, folder, name, ext string) (string, error) {
	lstat := userLstat(d)
	for i := 1; i <= 100; i++ {
		whole := name + ext
		if i > 1 {
			whole = fmt.Sprintf("%s (%d)%s", name, i, ext)
		}
		targets := []string{path.Join(folder, whole)}
		if len(made) > 1 {
			targets = targets[:0]
			for k := range made {
				targets = append(targets, path.Join(folder, fmt.Sprintf("%s.%03d", whole, k+1)))
			}
		}
		if taken, err := filesTaken(d, lstat, folder, whole, targets); err != nil {
			return "", err
		} else if taken {
			continue
		}
		for _, target := range targets {
			if !d.CheckRules(target) {
				return "", errArchiveRules
			}
		}
		err := moveAll(d, stage, made, targets)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		return targets[0], nil
	}
	return "", errors.New("no free name for the result")
}

// filesTaken reports whether a target is there, or, for a set, any volume of whole.
func filesTaken(d *data, lstat func(string) (os.FileInfo, error), folder, whole string, targets []string) (bool, error) {
	for _, target := range targets {
		// Through the user's file system, which refuses a folder that leads out of the scope.
		if _, err := lstat(target); err == nil {
			return true, nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			return false, err
		}
	}
	if len(targets) == 1 {
		return false, nil
	}
	f, err := d.user.Fs.Open(folder)
	if err != nil {
		return false, err
	}
	defer f.Close()
	names, err := f.Readdirnames(-1)
	if err != nil {
		return false, err
	}
	for _, n := range names {
		if len(n) == len(whole)+4 && strings.EqualFold(n[:len(whole)+1], whole+".") && unpack.IsArchive(n) {
			return true, nil
		}
	}
	return false, nil
}

// moveAll moves the files made to their targets, replacing nothing. When a target is taken
// meanwhile, those moved go back and it returns fs.ErrExist.
func moveAll(d *data, stage string, made, targets []string) error {
	for k := range made {
		err := renameNoReplace(filepath.Join(stage, made[k]), d.user.FullPath(targets[k]))
		if err == nil {
			continue
		}
		for back := k - 1; back >= 0; back-- {
			if backErr := renameNoReplace(d.user.FullPath(targets[back]), filepath.Join(stage, made[back])); backErr != nil {
				return backErr
			}
		}
		if errors.Is(err, syscall.EXDEV) {
			return errArchiveOtherDisk
		}
		return err
	}
	return nil
}

// checkBuilt checks the rules for every path of the folder built, as it would be at target.
func checkBuilt(d *data, built, target string) error {
	return filepath.WalkDir(built, func(p string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(built, p)
		if err != nil {
			return err
		}
		if !d.CheckRules(path.Join(target, filepath.ToSlash(rel))) {
			return errArchiveRules
		}
		return nil
	})
}

// archiveRequest asks for a job: to extract the archives chosen, all in one folder, with their
// password; or to create an archive of the files and folders chosen, all in one folder, named
// name, in a format, in volumes of volume bytes when that is set, with ZIP's MS-DOS times in the
// time zone the browser names.
type archiveRequest struct {
	Kind     string   `json:"kind"`
	Items    []string `json:"items"`
	Password string   `json:"password"`
	Name     string   `json:"name"`
	Format   string   `json:"format"`
	Volume   int64    `json:"volume"`
	Zone     string   `json:"zone"`
}

// archiveMinVolume is the smallest volume a set may be split in.
const archiveMinVolume = 1 << 20

// archiveStartHandler starts a job.
func archiveStartHandler(jobs *ArchiveJobs) handleFunc {
	return withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		var req archiveRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
			return http.StatusBadRequest, err
		}
		create := req.Kind == archiveCreate
		switch {
		case req.Kind != "" && req.Kind != archiveExtract && !create:
			return http.StatusBadRequest, nil
		case !d.user.Perm.Create:
			return http.StatusForbidden, nil
		case create && !d.user.Perm.Download:
			// An archive holds the files' content, as a download does.
			return http.StatusForbidden, nil
		case len(req.Items) == 0 || len(req.Items) > archiveItems:
			return http.StatusBadRequest, nil
		}

		folder := ""
		names := make([]string, 0, len(req.Items))
		var first os.FileInfo
		for i, item := range req.Items {
			p := slashClean(item)
			dir, name := path.Dir(p), path.Base(p)
			if i == 0 {
				folder = dir
			}
			if dir != folder || p == "/" || !create && !unpack.IsArchive(name) {
				return http.StatusBadRequest, nil
			}
			if !d.Check(p) {
				return http.StatusForbidden, nil
			}
			info, err := d.user.Fs.Stat(p)
			if err != nil {
				return errToStatus(err), err
			}
			if !create && !info.Mode().IsRegular() {
				return http.StatusBadRequest, nil
			}
			if i == 0 {
				first = info
			}
			names = append(names, name)
		}
		if !d.Check(folder) {
			return http.StatusForbidden, nil
		}

		job := &archiveJob{Kind: archiveExtract, Folder: folder, Names: names}
		var c archiveCreation
		if create {
			f, ok := archiveFormats[req.Format]
			if !ok || req.Volume != 0 && req.Volume < archiveMinVolume {
				return http.StatusBadRequest, nil
			}
			if c.name, ok = archiveName(req.Name, f.ext); !ok {
				return http.StatusBadRequest, nil
			}
			// The rules are known before the job: they could refuse its result only at the end.
			if !d.CheckRules(path.Join(folder, c.name+f.ext)) ||
				req.Volume > 0 && !d.CheckRules(path.Join(folder, c.name+f.ext+".001")) {
				return http.StatusForbidden, nil
			}
			c.format, c.volume, c.folder = req.Format, req.Volume, len(names) == 1 && first.IsDir()
			c.zone = time.Local
			if req.Zone != "" && len(req.Zone) <= 64 {
				if zone, err := time.LoadLocation(req.Zone); err == nil {
					c.zone = zone
				}
			}
			job.Kind, job.Name, job.Format = archiveCreate, c.name+f.ext, req.Format
		}

		free, err := freeSpace(d.user.FullPath(folder))
		if err != nil {
			return http.StatusInternalServerError, err
		}
		if free <= archiveReserve {
			return http.StatusInsufficientStorage, nil
		}
		budget := int64(free - archiveReserve)

		work := jobs.extract(d, req.Password, budget)
		if create {
			c.budget = budget
			work = jobs.create(d, c)
		}
		started, err := jobs.start(d, job, work)
		if errors.Is(err, errArchiveBusy) {
			return http.StatusConflict, nil
		}
		if err != nil {
			return http.StatusInternalServerError, err
		}
		return renderJSON(w, r, started)
	})
}

// archiveName checks the name of an archive to make, which comes without its extension: a
// typed one is dropped.
func archiveName(name, ext string) (string, bool) {
	name = strings.TrimSpace(name)
	if len(name) > len(ext) && strings.EqualFold(name[len(name)-len(ext):], ext) {
		name = strings.TrimSpace(name[:len(name)-len(ext)])
	}
	if name == "" || name == "." || name == ".." || len(name) > 200 || !utf8.ValidString(name) ||
		strings.HasPrefix(name, ".gezgin-") || strings.ContainsAny(name, `/\`) ||
		strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return "", false
	}
	return name, true
}

func archiveListHandler(jobs *ArchiveJobs) handleFunc {
	return withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		return renderJSON(w, r, jobs.list(d.user.ID))
	})
}

func archiveCancelHandler(jobs *ArchiveJobs) handleFunc {
	return withUser(func(_ http.ResponseWriter, r *http.Request, d *data) (int, error) {
		found, cancelled := jobs.cancel(d.user.ID, mux.Vars(r)["id"])
		switch {
		case !found:
			return http.StatusNotFound, nil
		case !cancelled:
			return http.StatusConflict, nil
		}
		return http.StatusNoContent, nil
	})
}
