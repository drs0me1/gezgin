package fbhttp

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/storage"
	"github.com/filebrowser/filebrowser/v2/version"
)

func handleWithStaticData(w http.ResponseWriter, _ *http.Request, d *data, fSys fs.FS, file, contentType string) (int, error) {
	w.Header().Set("Content-Type", contentType)

	// Gezgin's brand is fixed: no instance name, colour, custom styles or images (K70).
	data := map[string]interface{}{
		"DisableUsedPercentage": d.settings.Branding.DisableUsedPercentage,
		"BaseURL":               d.server.BaseURL,
		"Version":               version.Version,
		"StaticURL":             path.Join(d.server.BaseURL, "/static"),
		"AuthMethod":            d.settings.AuthMethod,
		"Theme":                 d.settings.Branding.Theme,
		"EnableThumbs":          d.server.EnableThumbnails,
		"ResizePreview":         d.server.ResizePreview,
		"TusSettings":           d.settings.Tus,
		"WebDAVPort":            d.server.WebDAVPort,
	}

	b, err := json.Marshal(data)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	// The page reads them from a JSON data block, not a script (Gezgin: the CSP allows no inline
	// script). json.Marshal escapes <, > and &, so the block cannot be closed early.
	data["Json"] = template.JS(b)

	fileContents, err := fs.ReadFile(fSys, file)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return http.StatusNotFound, err
		}
		return http.StatusInternalServerError, err
	}
	index := template.Must(template.New("index").Delims("[{[", "]}]").Parse(string(fileContents)))
	err = index.Execute(w, data)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	return 0, nil
}

func getStaticHandlers(store *storage.Storage, server *settings.Server, assetsFs fs.FS) (index, static http.Handler) {
	index = handle(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		if r.Method != http.MethodGet {
			return http.StatusNotFound, nil
		}

		return handleWithStaticData(w, r, d, assetsFs, "public/index.html", "text/html; charset=utf-8")
	}, "", store, server)

	static = handle(func(w http.ResponseWriter, r *http.Request, _ *data) (int, error) {
		if r.Method != http.MethodGet {
			return http.StatusNotFound, nil
		}

		if strings.HasSuffix(r.URL.Path, "/") {
			return http.StatusNotFound, nil
		}

		const maxAge = 86400 // 1 day
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%v", maxAge))
		w.Header().Set("X-Content-Type-Options", "nosniff")

		if !strings.HasSuffix(r.URL.Path, ".js") {
			http.FileServer(http.FS(assetsFs)).ServeHTTP(w, r)
			return 0, nil
		}

		f, err := assetsFs.Open(r.URL.Path + ".gz")
		if err != nil {
			return http.StatusNotFound, err
		}
		defer f.Close()

		acceptEncoding := r.Header.Get("Accept-Encoding")
		if strings.Contains(acceptEncoding, "gzip") {
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")

			if _, err := io.Copy(w, f); err != nil {
				return http.StatusInternalServerError, err
			}
		} else {
			gzReader, err := gzip.NewReader(f)
			if err != nil {
				return http.StatusInternalServerError, err
			}
			defer gzReader.Close()

			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")

			if _, err := io.Copy(w, gzReader); err != nil {
				return http.StatusInternalServerError, err
			}
		}

		return 0, nil
	}, "/static/", store, server)

	return index, static
}
