package fbhttp

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/filebrowser/filebrowser/v2/version"
)

// The server page of the settings (Gezgin, K146, K147): what the container runs with, which is
// set in Konsol and only shown here, the sizes Gezgin keeps beside the files, and the thumbnail
// cache, which an admin may clear.

// thumbCache is the part of the thumbnail cache the server page reads and clears; without a cache
// folder there is none.
type thumbCache interface {
	Usage(ctx context.Context) (count int, size int64, err error)
	Clear(ctx context.Context) error
}

type serverInfo struct {
	Version    string `json:"version"`
	WebDAVPort string `json:"webdavPort"`
	Thumbnails bool   `json:"thumbnails"`
	// SessionSeconds is how long a session lasts unused; using it renews it.
	SessionSeconds int64      `json:"sessionSeconds"`
	DatabaseSize   int64      `json:"databaseSize"`
	Cache          *cacheInfo `json:"cache"`
}

type cacheInfo struct {
	Count int   `json:"count"`
	Size  int64 `json:"size"`
}

func serverInfoHandler(fileCache FileCache, tokenExpirationTime time.Duration) handleFunc {
	return withAdmin(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		info := serverInfo{
			Version:        version.Version,
			WebDAVPort:     d.server.WebDAVPort,
			Thumbnails:     d.server.EnableThumbnails,
			SessionSeconds: int64(tokenExpirationTime / time.Second),
		}
		if d.server.Database != "" {
			if stat, err := os.Stat(d.server.Database); err == nil {
				info.DatabaseSize = stat.Size()
			}
		}
		if cache, ok := fileCache.(thumbCache); ok {
			count, size, err := cache.Usage(r.Context())
			if err != nil {
				return http.StatusInternalServerError, err
			}
			info.Cache = &cacheInfo{Count: count, Size: size}
		}
		return renderJSON(w, r, info)
	})
}

func serverCacheDeleteHandler(fileCache FileCache) handleFunc {
	return withAdmin(func(_ http.ResponseWriter, r *http.Request, _ *data) (int, error) {
		cache, ok := fileCache.(thumbCache)
		if !ok {
			return http.StatusNotFound, nil
		}
		if err := cache.Clear(r.Context()); err != nil {
			return http.StatusInternalServerError, err
		}
		return http.StatusNoContent, nil
	})
}
