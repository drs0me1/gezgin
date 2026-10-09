package fbhttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path"
	"slices"
	"sync"
	"time"

	"github.com/filebrowser/filebrowser/v2/files"
	"github.com/filebrowser/filebrowser/v2/search"
)

const searchPingInterval = 5

var searchHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	response := make(chan map[string]interface{})
	ctx, cancel := context.WithCancelCause(r.Context())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Avoid connection timeout
		timeout := time.NewTimer(searchPingInterval * time.Second)
		defer timeout.Stop()
		for {
			var err error
			var infoBytes []byte
			select {
			case info := <-response:
				if info == nil {
					return
				}
				infoBytes, err = json.Marshal(info)
			case <-timeout.C:
				// Send a heartbeat packet
				infoBytes = nil
			case <-ctx.Done():
				return
			}
			if err != nil {
				cancel(err)
				return
			}
			_, err = w.Write(infoBytes)
			if err == nil {
				_, err = w.Write([]byte("\n"))
			}
			if err != nil {
				cancel(err)
				return
			}
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}()
	query := r.URL.Query().Get("query")

	marks := d.shareMarks()
	err := search.Search(ctx, d.user.Fs, r.URL.Path, query, d, func(p string, f os.FileInfo) error {
		select {
		case <-ctx.Done():
		case response <- searchResult(d, marks, r.URL.Path, p, f):
		}
		return context.Cause(ctx)
	})
	close(response)
	wg.Wait()
	if err == nil {
		err = context.Cause(ctx)
	}
	// ignore cancellation errors from user aborts
	if err != nil && !errors.Is(err, context.Canceled) {
		return http.StatusInternalServerError, err
	}

	return 0, nil
})

// searchResult describes a found item as the folder view shows it (Gezgin, K111): where it lies
// under the searched folder, its name, size, time and type, and its marks (K114-K116).
func searchResult(d *data, marks map[string]shareMark, base, p string, f os.FileInfo) map[string]interface{} {
	full := path.Join("/", base, p)
	result := map[string]interface{}{
		"dir":      f.IsDir(),
		"isDir":    f.IsDir(),
		"path":     p,
		"name":     f.Name(),
		"modified": f.ModTime(),
	}
	if !f.IsDir() {
		result["size"] = f.Size()
		result["type"] = files.NameType(f.Name(), f.Size())
	}
	if slices.Contains(d.user.Favorites, full) {
		result["favorite"] = true
	}
	if m := marks[full]; m.links > 0 || m.dav > 0 {
		result["sharedLinks"], result["sharedDav"] = m.links, m.dav
	}
	return result
}
