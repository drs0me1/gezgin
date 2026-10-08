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
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/spf13/afero"

	"github.com/filebrowser/filebrowser/v2/unpack"
)

// Gezgin opens archives on the server: a job opens the chosen ones, and the archives in them,
// into a folder beside them. It builds the folder in ArchiveDir and puts it in place whole.

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
	Folder string   `json:"folder"`
	Names  []string `json:"names"`
	State  string   `json:"state"`
	// Error tells why a failed job stopped (unpack's codes, "rules", "otherDisk" or "internal").
	Error  string `json:"error,omitempty"`
	Result string `json:"result,omitempty"` // the folder made
	unpack.Progress
	Started  time.Time  `json:"started"`
	Finished *time.Time `json:"finished,omitempty"`

	user   uint
	cancel context.CancelFunc
}

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

func (a *ArchiveJobs) start(d *data, folder string, names []string, password string, budget int64) (archiveJob, error) {
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
	job := &archiveJob{ID: hex.EncodeToString(random), Folder: folder, Names: names, State: archiveRunning,
		Started: time.Now().UTC(), user: d.user.ID, cancel: cancel}
	a.jobs = append([]*archiveJob{job}, a.jobs...)
	if len(a.jobs) > archiveHistory {
		a.jobs = a.jobs[:archiveHistory]
	}
	a.busy = true
	a.wg.Add(1)
	go a.run(ctx, d, job, password, budget)
	return *job, nil
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

func (a *ArchiveJobs) run(ctx context.Context, d *data, job *archiveJob, password string, budget int64) {
	defer a.wg.Done()
	stage := filepath.Join(a.dir, job.ID)
	result, err := a.extract(ctx, d, job, stage, password, budget)
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

// extract builds the job's folder in stage and puts it in place. A panic in a decoder fails the
// job, not the server.
func (a *ArchiveJobs) extract(ctx context.Context, d *data, job *archiveJob, stage, password string, budget int64) (result string, err error) {
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
	// The folder is put in place by a rename, which does not cross disks.
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
			a.mu.Lock()
			job.Progress = p
			a.mu.Unlock()
		},
	}
	progress, err := unpack.Extract(ctx, src, job.Names, built, opt)
	opt.Progress(progress)
	if err != nil {
		return "", err
	}
	return publishArchive(d, built, job.Folder, unpack.Stem(job.Names[0]))
}

func archiveCode(err error) string {
	var e *unpack.Error
	switch {
	case errors.As(err, &e):
		return string(e.Code)
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

// publishArchive puts the folder built in place in folder, named name or, when that is taken,
// with a number. The user's rules hold for every path in it.
func publishArchive(d *data, built, folder, name string) (string, error) {
	lstat := d.user.Fs.Stat
	if l, ok := d.user.Fs.(afero.Lstater); ok {
		lstat = func(p string) (os.FileInfo, error) {
			info, _, err := l.LstatIfPossible(p)
			return info, err
		}
	}
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

// archiveStartHandler starts a job: {"items": [paths of archives in one folder], "password"}.
func archiveStartHandler(jobs *ArchiveJobs) handleFunc {
	return withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		if !d.user.Perm.Create {
			return http.StatusForbidden, nil
		}
		var req struct {
			Items    []string `json:"items"`
			Password string   `json:"password"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
			return http.StatusBadRequest, err
		}
		if len(req.Items) == 0 || len(req.Items) > archiveItems {
			return http.StatusBadRequest, nil
		}

		folder := ""
		names := make([]string, 0, len(req.Items))
		for i, item := range req.Items {
			p := slashClean(item)
			dir, name := path.Dir(p), path.Base(p)
			if i == 0 {
				folder = dir
			}
			if dir != folder || p == "/" || !unpack.IsArchive(name) {
				return http.StatusBadRequest, nil
			}
			if !d.Check(p) {
				return http.StatusForbidden, nil
			}
			info, err := d.user.Fs.Stat(p)
			if err != nil {
				return errToStatus(err), err
			}
			if !info.Mode().IsRegular() {
				return http.StatusBadRequest, nil
			}
			names = append(names, name)
		}
		if !d.Check(folder) {
			return http.StatusForbidden, nil
		}

		free, err := freeSpace(d.user.FullPath(folder))
		if err != nil {
			return http.StatusInternalServerError, err
		}
		if free <= archiveReserve {
			return http.StatusInsufficientStorage, nil
		}

		job, err := jobs.start(d, folder, names, req.Password, int64(free-archiveReserve))
		if errors.Is(err, errArchiveBusy) {
			return http.StatusConflict, nil
		}
		if err != nil {
			return http.StatusInternalServerError, err
		}
		return renderJSON(w, r, job)
	})
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
