package fbhttp

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"strings"

	"github.com/spf13/afero"
	"golang.org/x/net/webdav"

	fberrors "github.com/filebrowser/filebrowser/v2/errors"
	"github.com/filebrowser/filebrowser/v2/files"
	"github.com/filebrowser/filebrowser/v2/fileutils"
)

// davFS is the folder of a WebDAV share, through its owner's rules (Gezgin). Names are relative to
// the folder, which a symbolic link cannot lead out of; the rules see them in the owner's scope.
type davFS struct {
	d         *data    // the owner
	fs        afero.Fs // the folder
	base      string   // the folder, in the owner's scope
	writable  bool
	fileCache FileCache
}

func davClean(name string) string {
	return path.Clean("/" + name)
}

// owned is the name's path in the owner's scope.
func (f *davFS) owned(name string) string {
	return path.Join(f.base, name)
}

// Check implements rules.Checker: what the owner's rules refuse is not in the share, nor are
// Gezgin's own files, such as the temporary ones of writes in progress.
func (f *davFS) Check(name string) bool {
	if strings.HasPrefix(path.Base(name), ".gezgin-") {
		return false
	}
	return f.d.CheckRules(f.owned(name))
}

func (f *davFS) Stat(_ context.Context, name string) (os.FileInfo, error) {
	name = davClean(name)
	if !f.Check(name) {
		return nil, os.ErrNotExist
	}
	return f.fs.Stat(name)
}

func (f *davFS) OpenFile(ctx context.Context, name string, flag int, _ os.FileMode) (webdav.File, error) {
	name = davClean(name)
	if !f.Check(name) {
		return nil, os.ErrNotExist
	}
	if flag&(os.O_WRONLY|os.O_RDWR|os.O_APPEND|os.O_CREATE|os.O_TRUNC) != 0 {
		if !f.writable {
			return nil, os.ErrPermission
		}
		return f.create(ctx, name)
	}
	file, err := f.fs.Open(name)
	if err != nil {
		return nil, err
	}
	return &davFile{File: file, dav: f, name: name}, nil
}

// create opens name for writing; see davStaged. A replaced file keeps its permissions.
func (f *davFS) create(ctx context.Context, name string) (webdav.File, error) {
	perm := f.d.settings.FileMode
	switch info, err := f.fs.Stat(name); {
	case err == nil && info.IsDir():
		return nil, &os.PathError{Op: "write", Path: name, Err: errors.New("is a directory")}
	case err == nil:
		perm = info.Mode().Perm()
	case !errors.Is(err, fs.ErrNotExist):
		return nil, err
	}

	tmp, err := fileutils.TempName(name)
	if err != nil {
		return nil, err
	}
	file, err := f.fs.OpenFile(tmp, os.O_RDWR|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return nil, err
	}
	body, _ := ctx.Value(davBodyKey{}).(*davBody)
	return &davStaged{File: file, dav: f, name: name, tmp: tmp, perm: perm, body: body}, nil
}

func (f *davFS) Mkdir(_ context.Context, name string, _ os.FileMode) error {
	name = davClean(name)
	if !f.writable || !f.Check(name) {
		return os.ErrPermission
	}
	return f.fs.Mkdir(name, f.d.settings.DirMode)
}

// RemoveAll moves the item into the owner's trash, as a delete in Gezgin does, and ends its share
// links. An item on another disk than the trash is not deleted.
func (f *davFS) RemoveAll(ctx context.Context, name string) error {
	name = davClean(name)
	if !f.writable || name == "/" || !f.Check(name) {
		return os.ErrPermission
	}
	if err := f.checkTree(name); err != nil {
		return err
	}
	f.dropThumbs(ctx, name)
	if err := moveToTrash(f.d, f.owned(name)); err != nil {
		if errors.Is(err, fberrors.ErrTrashOtherDisk) {
			return os.ErrPermission
		}
		return err
	}
	if err := dropShares(f.d, f.owned(name)); err != nil {
		log.Printf("WARNING: could not end the shares of %s: %v", f.owned(name), err)
	}
	return nil
}

// Rename moves an item, and its share links along.
func (f *davFS) Rename(ctx context.Context, oldName, newName string) error {
	oldName, newName = davClean(oldName), davClean(newName)
	if !f.writable || oldName == "/" || !f.Check(oldName) || !f.Check(newName) {
		return os.ErrPermission
	}
	if err := f.checkTree(oldName); err != nil {
		return err
	}
	f.dropThumbs(ctx, oldName)
	if err := fileutils.MoveFile(f.fs, oldName, newName, f.d.settings.FileMode, f.d.settings.DirMode); err != nil {
		return err
	}
	if err := moveShares(f.d, f.owned(oldName), f.owned(newName)); err != nil {
		log.Printf("WARNING: could not move the shares of %s: %v", f.owned(oldName), err)
	}
	return nil
}

// checkTree refuses to move or delete an item with anything inside it the rules refuse.
func (f *davFS) checkTree(name string) error {
	return afero.Walk(f.fs, name, func(p string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !f.d.CheckRules(f.owned(p)) {
			return os.ErrPermission
		}
		return nil
	})
}

func (f *davFS) dropThumbs(ctx context.Context, name string) {
	if f.fileCache == nil {
		return
	}
	file, err := files.NewFileInfo(&files.FileOptions{Fs: f.fs, Path: name, Checker: f})
	if err != nil {
		return
	}
	if err := delThumbs(ctx, f.fileCache, file); err != nil {
		log.Printf("WARNING: could not delete the thumbnails of %s: %v", f.owned(name), err)
	}
}

// davFile is a file or folder opened to read. A folder lists only what the share shows, and a
// symbolic link as what it leads to, unless it leads out of the share.
type davFile struct {
	afero.File
	dav  *davFS
	name string
}

// davLinked describes what a symbolic link leads to, under the link's name.
type davLinked struct {
	os.FileInfo
	name string
}

func (l davLinked) Name() string { return l.name }

func (f *davFile) Readdir(count int) ([]os.FileInfo, error) {
	infos, err := f.File.Readdir(count)
	shown := infos[:0]
	for _, info := range infos {
		p := path.Join(f.name, info.Name())
		if !f.dav.Check(p) {
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, statErr := f.dav.fs.Stat(p)
			if statErr != nil {
				continue
			}
			info = davLinked{FileInfo: target, name: info.Name()}
		}
		shown = append(shown, info)
	}
	return shown, err
}

func (f *davFile) Write([]byte) (int, error) {
	return 0, os.ErrPermission
}

// davStaged is a file being written. Its content goes to a temporary file beside it and takes its
// place on Close, unless a write failed or, for a PUT, the request's body ended short: a client
// that went away half way leaves the file as it was.
type davStaged struct {
	afero.File
	dav       *davFS
	name, tmp string
	perm      os.FileMode
	body      *davBody // the PUT's body; nil for a copy or a lock
	failed    bool
}

func (s *davStaged) Write(p []byte) (int, error) {
	n, err := s.File.Write(p)
	if err != nil {
		s.failed = true
	}
	return n, err
}

func (s *davStaged) Close() error {
	err := s.Sync()
	if closeErr := s.File.Close(); err == nil {
		err = closeErr
	}
	if err == nil && (s.failed || (s.body != nil && !s.body.done)) {
		err = errors.New("the content ended before it was complete")
	}
	if err == nil {
		err = s.dav.fs.Chmod(s.tmp, s.perm)
	}
	if err == nil {
		err = s.dav.fs.Rename(s.tmp, s.name)
	}
	if err != nil {
		_ = s.dav.fs.Remove(s.tmp)
	}
	return err
}

type davBodyKey struct{}

// davBody tells whether a PUT's body was read to its end.
type davBody struct {
	io.ReadCloser
	done bool
}

func (b *davBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if errors.Is(err, io.EOF) {
		b.done = true
	}
	return n, err
}
