package fbhttp

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	gopath "path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v4/disk"
	"github.com/spf13/afero"

	"github.com/filebrowser/filebrowser/v2/files"
	"github.com/filebrowser/filebrowser/v2/fileutils"
)

// Gezgin stages a tus upload's data (see UploadCache) and puts the file in place only once it is
// complete: an upload that is cancelled, abandoned or cut by a restart leaves its destination as it
// was, and no one sees a file half way.

// maxPatchDrainBytes bounds how much of a rejected PATCH body is discarded to
// keep the connection reusable. It comfortably covers a default-sized chunk;
// beyond that the body is not worth reading just to throw away.
const maxPatchDrainBytes = 32 << 20 // 32MB

// chunkStallTimeout ends a chunk whose bytes stop coming (Gezgin, K169). The server has no time
// limit on a request's body, so a client that stalls in the middle of one would hold the upload
// for ever, and its own retry would wait behind it.
var chunkStallTimeout = 30 * time.Second

// stallReader moves the request's read deadline on before every read: a chunk may take any time
// as long as its bytes keep coming.
type stallReader struct {
	body io.Reader
	rc   *http.ResponseController
}

func (s stallReader) Read(p []byte) (int, error) {
	// A writer without deadlines (a test's recorder) reads as before.
	_ = s.rc.SetReadDeadline(time.Now().Add(chunkStallTimeout))
	return s.body.Read(p)
}

// drainRequestBody discards what the client already put on the wire for a
// request the handler answered without reading. net/http only drains 256KiB on
// its own before giving up and closing the connection, and a connection closed
// while the client is still streaming a chunk reaches the browser as a transport
// error instead of the status we replied with. The client then cannot tell a
// conflict from a network fault, retries blindly, and the upload stalls.
func drainRequestBody(r *http.Request) {
	if r.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(r.Body, maxPatchDrainBytes))
}

// keepUploadActive periodically touches the cache entry to prevent eviction during transfer
func keepUploadActive(cache *UploadCache, key string) func() {
	stop := make(chan bool)

	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				cache.touch(key)
			}
		}
	}()

	return func() {
		close(stop)
	}
}

// uploadKey names the user's upload to path; another user's upload to it is another upload.
func uploadKey(d *data, p string) string {
	return strconv.FormatUint(uint64(d.user.ID), 10) + ":" + gopath.Clean("/"+p)
}

// freeSpace returns how many bytes Gezgin may still write on the disk that holds path, or would
// hold it: a path that does not exist yet is on its nearest existing parent's disk.
func freeSpace(p string) (uint64, error) {
	for {
		usage, err := disk.Usage(p)
		if err == nil {
			return usage.Free, nil
		}
		parent := filepath.Dir(p)
		if parent == p || !errors.Is(err, fs.ErrNotExist) {
			return 0, err
		}
		p = parent
	}
}

// roomFor reports whether an upload of length bytes to the user's path fits: on the staging disk
// besides what the uploads in progress still have to write, and on the destination's disk, which
// may be another one.
func roomFor(d *data, cache *UploadCache, p string, length int64) (bool, error) {
	if length == 0 {
		return true, nil
	}
	free, err := freeSpace(cache.dir)
	if err != nil {
		return false, err
	}
	if uint64(length)+uint64(cache.pending()) > free {
		return false, nil
	}
	free, err = freeSpace(d.user.FullPath(p))
	if err != nil {
		return false, err
	}
	return uint64(length) <= free, nil
}

func tusPostHandler(cache *UploadCache, fileCache FileCache) handleFunc {
	return withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		if !d.user.Perm.Create || !d.Check(r.URL.Path) {
			return http.StatusForbidden, nil
		}

		uploadLength, err := getUploadLength(r)
		if err != nil || uploadLength < 0 {
			return http.StatusBadRequest, fmt.Errorf("invalid upload length: %w", err)
		}
		override := r.URL.Query().Get("override") == "true"

		file, err := files.NewFileInfo(&files.FileOptions{
			Fs:      d.user.Fs,
			Path:    r.URL.Path,
			Modify:  d.user.Perm.Modify,
			Expand:  false,
			Checker: d,
		})
		switch {
		case errors.Is(err, afero.ErrFileNotFound):
			dirPath := filepath.Dir(r.URL.Path)
			if _, statErr := d.user.Fs.Stat(dirPath); os.IsNotExist(statErr) {
				if mkdirErr := d.user.Fs.MkdirAll(dirPath, d.settings.DirMode); mkdirErr != nil {
					return errToStatus(mkdirErr), mkdirErr
				}
			}
		case err != nil:
			return errToStatus(err), err
		case file.IsDir:
			return http.StatusBadRequest, fmt.Errorf("cannot upload to a directory %s", file.RealPath())
		case !override:
			// Existing files will remain untouched unless explicitly instructed to override
			return http.StatusConflict, nil
		case !d.user.Perm.Modify:
			// Permission for overwriting the file
			return http.StatusForbidden, nil
		}

		room, err := roomFor(d, cache, r.URL.Path, uploadLength)
		if err != nil {
			return http.StatusInternalServerError, err
		}
		if !room {
			return http.StatusInsufficientStorage, nil
		}

		key := uploadKey(d, r.URL.Path)
		up, err := cache.begin(key, uploadLength, override)
		if err != nil {
			return http.StatusInternalServerError, err
		}
		// The client sends no chunk for an empty file: it is complete already.
		if uploadLength == 0 {
			if status, err := finishUpload(r, d, cache, fileCache, key, up); status != 0 {
				return status, err
			}
		}

		basePath := "/" + strings.Trim(strings.TrimSpace(d.server.BaseURL), "/")
		if basePath == "/" {
			basePath = ""
		}

		w.Header().Set("Location", basePath+"/api/tus"+r.URL.EscapedPath())
		return http.StatusCreated, nil
	})
}

func tusHeadHandler(cache *UploadCache) handleFunc {
	return withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		w.Header().Set("Cache-Control", "no-store")
		if !d.user.Perm.Create || !d.Check(r.URL.Path) {
			return http.StatusForbidden, nil
		}

		up, ok := cache.get(uploadKey(d, r.URL.Path))
		if !ok {
			return http.StatusNotFound, nil
		}
		offset, err := up.offset()
		if err != nil {
			// The upload ended meanwhile.
			return http.StatusNotFound, err
		}

		w.Header().Set("Upload-Offset", strconv.FormatInt(offset, 10))
		w.Header().Set("Upload-Length", strconv.FormatInt(up.length, 10))

		return http.StatusOK, nil
	})
}

func tusPatchHandler(cache *UploadCache, fileCache FileCache) handleFunc {
	return withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		key := userUploads(d.user.ID)
		if !running.begin(key) {
			drainRequestBody(r)
			return tooMany(w, r, tooManyUploads.For(d.user.Locale))
		}
		defer running.end(key)

		status, err := tusPatchUpload(w, r, d, cache, fileCache)
		// A rejected chunk is still a chunk the client is streaming: read what is
		// left of it so the answer reaches the client on a connection that stays
		// usable, instead of being lost to a reset.
		if status >= 400 {
			drainRequestBody(r)
		}
		return status, err
	})
}

func tusPatchUpload(w http.ResponseWriter, r *http.Request, d *data, cache *UploadCache, fileCache FileCache) (int, error) {
	if !d.user.Perm.Create || !d.Check(r.URL.Path) {
		return http.StatusForbidden, nil
	}
	if r.Header.Get("Content-Type") != "application/offset+octet-stream" {
		return http.StatusUnsupportedMediaType, nil
	}

	uploadOffset, err := getUploadOffset(r)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid upload offset")
	}

	key := uploadKey(d, r.URL.Path)
	up, ok := cache.get(key)
	if !ok {
		return http.StatusNotFound, nil
	}

	if uploadOffset > up.length {
		return http.StatusBadRequest, fmt.Errorf("upload offset %d exceeds declared length %d", uploadOffset, up.length)
	}

	// One chunk at a time, and the upload does not expire while it is written.
	up.mu.Lock()
	defer up.mu.Unlock()
	stop := keepUploadActive(cache, key)
	defer stop()

	staged, err := os.OpenFile(up.staged, os.O_WRONLY, 0)
	if errors.Is(err, fs.ErrNotExist) {
		// The upload ended meanwhile.
		return http.StatusNotFound, nil
	}
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("could not open the staged upload: %w", err)
	}
	defer staged.Close()

	info, err := staged.Stat()
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("could not stat the staged upload: %w", err)
	}
	if info.Size() != uploadOffset {
		return http.StatusConflict, fmt.Errorf(
			"%s: the upload is at %d bytes, not at the provided offset %d",
			r.URL.Path,
			info.Size(),
			uploadOffset,
		)
	}

	_, err = staged.Seek(uploadOffset, io.SeekStart)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("could not seek file: %w", err)
	}

	// The server closes the request body once the handler returns; closing it
	// here would run before the caller gets to drain a rejected chunk, so every
	// status raised below this point would still cost the connection.
	//
	// Enforce the declared Upload-Length: never write more than the bytes
	// still expected for this upload. Reading one byte past the remainder
	// lets an over-length body be detected and rejected. Without this bound a
	// PATCH could stream arbitrary data to disk regardless of the length the
	// client declared when the upload was created.
	remaining := up.length - uploadOffset
	body := stallReader{body: r.Body, rc: http.NewResponseController(w)}
	bytesWritten, err := io.Copy(staged, io.LimitReader(body, remaining+1))
	if errors.Is(err, os.ErrDeadlineExceeded) {
		// What came is kept: the client resumes from the offset a HEAD reports. The deadline
		// stays, so the rest of the stalled body is not waited for either.
		return http.StatusRequestTimeout, fmt.Errorf("%s: no data for %s, the chunk ends at offset %d",
			r.URL.Path, chunkStallTimeout, uploadOffset+bytesWritten)
	}
	if err == nil {
		// Sync the file to ensure all data is written to storage
		// to prevent file corruption.
		err = staged.Sync()
	}
	if errors.Is(err, syscall.ENOSPC) {
		// The disk filled up all the same (Gezgin): the upload cannot finish, so its data goes now.
		cache.finish(key, up)
		return http.StatusInsufficientStorage, err
	}
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("could not write to file: %w", err)
	}
	if bytesWritten > remaining {
		// The client sent more than it declared; roll this chunk back so the
		// file stays consistent with the tracked offset, and reject it.
		if truncErr := staged.Truncate(uploadOffset); truncErr != nil {
			return http.StatusInternalServerError, fmt.Errorf("could not truncate file: %w", truncErr)
		}
		return http.StatusRequestEntityTooLarge, fmt.Errorf("upload exceeds declared length of %d bytes", up.length)
	}

	newOffset := uploadOffset + bytesWritten
	w.Header().Set("Upload-Offset", strconv.FormatInt(newOffset, 10))

	if newOffset < up.length {
		return http.StatusNoContent, nil
	}
	if err = staged.Close(); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("could not close the staged upload: %w", err)
	}
	if status, err := finishUpload(r, d, cache, fileCache, key, up); status != 0 {
		return status, err
	}
	return http.StatusNoContent, nil
}

// finishUpload puts the complete upload in its place, once the place is checked again: it may have
// changed while the data came in. It returns 0 when the file is in place. Either way the upload
// ends.
func finishUpload(r *http.Request, d *data, cache *UploadCache, fileCache FileCache, key string, up *upload) (int, error) {
	defer cache.finish(key, up)

	old, err := files.NewFileInfo(&files.FileOptions{
		Fs:      d.user.Fs,
		Path:    r.URL.Path,
		Modify:  d.user.Perm.Modify,
		Expand:  false,
		Checker: d,
	})
	switch {
	case errors.Is(err, afero.ErrFileNotFound):
		old = nil
	case err != nil:
		return errToStatus(err), err
	case old.IsDir:
		return http.StatusBadRequest, fmt.Errorf("cannot upload to a directory %s", old.RealPath())
	case !up.override:
		return http.StatusConflict, nil
	case !d.user.Perm.Modify:
		return http.StatusForbidden, nil
	}

	// A replaced file keeps its permissions.
	perm := d.settings.FileMode
	if old != nil && old.Mode.IsRegular() {
		perm = old.Mode.Perm()
	}
	if _, err = fileutils.Place(d.user.Fs, up.staged, r.URL.Path, d.user.FullPath(r.URL.Path), perm); err != nil {
		return errToStatus(err), err
	}

	if old != nil {
		if err := delThumbs(r.Context(), fileCache, old); err != nil {
			log.Printf("WARNING: could not delete the thumbnails of %s: %v", old.Path, err)
		}
	}
	return 0, nil
}

// tusDeleteHandler cancels an upload. That drops its staged data and nothing else, so it takes
// the permission that started the upload (Gezgin).
func tusDeleteHandler(cache *UploadCache) handleFunc {
	return withUser(func(_ http.ResponseWriter, r *http.Request, d *data) (int, error) {
		if !d.user.Perm.Create || !d.Check(r.URL.Path) {
			return http.StatusForbidden, nil
		}

		if !cache.drop(uploadKey(d, r.URL.Path)) {
			return http.StatusNotFound, nil
		}

		return http.StatusNoContent, nil
	})
}

func getUploadLength(r *http.Request) (int64, error) {
	uploadOffset, err := strconv.ParseInt(r.Header.Get("Upload-Length"), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid upload length: %w", err)
	}
	return uploadOffset, nil
}

func getUploadOffset(r *http.Request) (int64, error) {
	uploadOffset, err := strconv.ParseInt(r.Header.Get("Upload-Offset"), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid upload offset: %w", err)
	}
	return uploadOffset, nil
}
