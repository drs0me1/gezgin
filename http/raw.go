package fbhttp

import (
	"errors"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/filebrowser/filebrowser/v2/files"
	"github.com/filebrowser/filebrowser/v2/fileutils"
	"github.com/filebrowser/filebrowser/v2/pack"
	"github.com/filebrowser/filebrowser/v2/users"
	"github.com/mholt/archives"
)

func parseQueryFiles(r *http.Request, f *files.FileInfo, _ *users.User) ([]string, error) {
	var fileSlice []string
	names := strings.Split(r.URL.Query().Get("files"), ",")

	if len(names) == 0 {
		fileSlice = append(fileSlice, f.Path)
	} else {
		for _, name := range names {
			name, err := url.QueryUnescape(strings.ReplaceAll(name, "+", "%2B"))
			if err != nil {
				return nil, err
			}

			name = slashClean(name)
			fileSlice = append(fileSlice, filepath.Join(f.Path, name))
		}
	}

	return fileSlice, nil
}

// parseQueryAlgorithm is the archive a download packs a folder or several items in. The
// dialog offers zip, tar and tar.gz.
func parseQueryAlgorithm(r *http.Request) (string, pack.Options, error) {
	tar := func(c archives.Compressor) (pack.Options, error) {
		return pack.Options{Format: pack.Tar, Compress: c.OpenWriter}, nil
	}
	var opt pack.Options
	var err error
	ext := ""
	switch r.URL.Query().Get("algo") {
	case "zip", "true", "":
		ext, opt = ".zip", pack.Options{Format: pack.Zip}
	case "tar":
		ext, opt = ".tar", pack.Options{Format: pack.Tar}
	case "targz":
		ext, opt = ".tar.gz", pack.Options{Format: pack.Tar, Compress: pack.Gzip}
	case "tarbz2":
		ext = ".tar.bz2"
		opt, err = tar(archives.Bz2{})
	case "tarxz":
		ext = ".tar.xz"
		opt, err = tar(archives.Xz{})
	case "tarlz4":
		ext = ".tar.lz4"
		opt, err = tar(archives.Lz4{})
	case "tarsz":
		ext = ".tar.sz"
		opt, err = tar(archives.Sz{})
	case "tarbr":
		ext = ".tar.br"
		opt, err = tar(archives.Brotli{})
	case "tarzst":
		ext = ".tar.zst"
		opt, err = tar(archives.Zstd{})
	default:
		return "", opt, errors.New("format not implemented")
	}
	return ext, opt, err
}

func setContentDisposition(w http.ResponseWriter, r *http.Request, file *files.FileInfo) {
	if r.URL.Query().Get("inline") == "true" {
		// As per RFC6266 section 4.3
		w.Header().Set("Content-Disposition", "inline; filename*=utf-8''"+url.PathEscape(file.Name))
	} else {
		// As per RFC6266 section 4.3
		w.Header().Set("Content-Disposition", "attachment; filename*=utf-8''"+url.PathEscape(file.Name))
		w.Header().Set("Content-Type", "application/octet-stream")
	}
}

var rawHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	// Without the download permission no content is served, here as for previews, subtitles and
	// checksums; File Browser answered 202 with nothing, which reads as a broken file (Gezgin).
	if !d.user.Perm.Download {
		return http.StatusForbidden, nil
	}
	key := userDownloads(d.user.ID)
	if !running.begin(key) {
		return tooMany(w, r, tooManyDownloads)
	}
	defer running.end(key)

	file, err := files.NewFileInfo(&files.FileOptions{
		Fs:      d.user.Fs,
		Path:    r.URL.Path,
		Modify:  d.user.Perm.Modify,
		Expand:  false,
		Checker: d,
	})
	if err != nil {
		return errToStatus(err), err
	}

	if files.IsNamedPipe(file.Mode) {
		setContentDisposition(w, r, file)
		return 0, nil
	}

	if !file.IsDir {
		return rawFileHandler(w, r, file)
	}

	return rawDirHandler(w, r, d, file)
})

// rawDirHandler packs a folder, or the items chosen in it, as the user sees them: what the
// rules refuse, links that lead out of the scope, special files and Gezgin's own files are left
// out. A ZIP keeps media and other compressed files as they are and compresses the rest.
func rawDirHandler(w http.ResponseWriter, r *http.Request, d *data, file *files.FileInfo) (int, error) {
	filenames, err := parseQueryFiles(r, file, d.user)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	extension, opt, err := parseQueryAlgorithm(r)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	commonDir := fileutils.CommonPrefix(filepath.Separator, filenames...)

	// One folder puts its contents at the archive's root; several items keep their names under
	// the folder they share.
	items := make([]pack.Item, 0, len(filenames))
	for _, fname := range filenames {
		name := filepath.ToSlash(strings.TrimPrefix(strings.TrimPrefix(fname, commonDir), string(filepath.Separator)))
		items = append(items, pack.Item{Path: filepath.ToSlash(fname), Name: name})
	}

	name := filepath.Base(commonDir)
	if name == "." || name == "" || name == string(filepath.Separator) {
		if file.Name != "" {
			name = file.Name
		} else {
			actual, statErr := file.Fs.Stat(".")
			if statErr != nil {
				return http.StatusInternalServerError, statErr
			}
			name = actual.Name()
		}
	}
	if len(filenames) > 1 {
		name = "_" + name
	}
	name += extension
	w.Header().Set("Content-Disposition", "attachment; filename*=utf-8''"+url.PathEscape(name))

	// A file that grows meanwhile goes in as it was, as a download always took it.
	opt.Lenient = true
	opt.Limits.Entries = downloadEntries
	opt.Location = time.Local
	if zone := r.URL.Query().Get("zone"); zone != "" && len(zone) <= 64 {
		if loc, err := time.LoadLocation(zone); err == nil {
			opt.Location = loc
		}
	}
	if _, err := pack.Write(r.Context(), newPackSource(d), items, w, opt); err != nil {
		var packErr *pack.Error
		if errors.As(err, &packErr) && packErr.Code == pack.CodeEntries {
			// Found while planning, before anything was sent: not an attachment but a message.
			w.Header().Del("Content-Disposition")
			http.Error(w, tooManyEntries, http.StatusUnprocessableEntity)
			return 0, nil
		}
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

func rawFileHandler(w http.ResponseWriter, r *http.Request, file *files.FileInfo) (int, error) {
	fd, err := file.Fs.Open(file.Path)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer fd.Close()

	setContentDisposition(w, r, file)
	w.Header().Add("Content-Security-Policy", `script-src 'none';`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private")
	http.ServeContent(w, r, file.Name, file.ModTime, fd)
	return 0, nil
}
