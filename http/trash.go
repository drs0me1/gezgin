package fbhttp

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path"
	"path/filepath"

	fberrors "github.com/filebrowser/filebrowser/v2/errors"
	"github.com/filebrowser/filebrowser/v2/trash"
)

// The trash (Gezgin): a delete moves the item into the user's bin (see package trash), from where
// it is restored, deleted for good, or swept once it expires.

type trashList struct {
	Items []trash.Item `json:"items"`
	Count int          `json:"count"`
	Size  int64        `json:"size"`
}

type trashIDs struct {
	IDs []string `json:"ids"`
}

func (d *data) bin() *trash.Bin {
	return trash.For(d.server.Root, d.user.ID)
}

// moveToTrash moves the item at the user's path into their bin.
func moveToTrash(d *data, p string) error {
	p = path.Clean("/" + p) // a folder may be named with a trailing slash
	_, err := d.bin().Put(d.user.FullPath(p), path.Dir(p))
	if errors.Is(err, trash.ErrOtherDisk) {
		return fberrors.ErrTrashOtherDisk
	}
	return err
}

func readIDs(w http.ResponseWriter, r *http.Request) ([]string, error) {
	var req trashIDs
	if r.Body == nil {
		return nil, fberrors.ErrEmptyRequest
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		return nil, err
	}
	if len(req.IDs) == 0 {
		return nil, fberrors.ErrEmptyRequest
	}
	for _, id := range req.IDs {
		if !trash.ValidID(id) {
			return nil, fberrors.ErrInvalidRequestParams
		}
	}
	return req.IDs, nil
}

var trashListHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	items, err := d.bin().List()
	if err != nil {
		return http.StatusInternalServerError, err
	}
	list := trashList{Items: items, Count: len(items)}
	for _, item := range items {
		list.Size += item.Size
	}
	return renderJSON(w, r, list)
})

type restored struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

// trashRestoreHandler puts items back where they were: into the root when that folder is gone, with
// a number in the name when it is taken.
var trashRestoreHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	if !d.user.Perm.Create {
		return http.StatusForbidden, nil
	}
	ids, err := readIDs(w, r)
	if err != nil {
		return http.StatusBadRequest, err
	}

	bin := d.bin()
	done := []restored{}
	for _, id := range ids {
		item, err := bin.Get(id)
		if err != nil {
			return http.StatusNotFound, err
		}

		dir := item.Origin
		if info, err := d.user.Fs.Stat(dir); err != nil || !info.IsDir() || !d.Check(dir) {
			dir = "/"
		}
		target := addVersionSuffix(path.Join(dir, item.Name), d.user.Fs)
		if err := checkRestore(d, bin.Path(item), target); err != nil {
			return errToStatus(err), err
		}

		if err := os.Rename(bin.Path(item), d.user.FullPath(target)); err != nil {
			return errToStatus(err), err
		}
		if err := bin.Forget(id); err != nil {
			return http.StatusInternalServerError, err
		}
		done = append(done, restored{ID: id, Path: target})
	}
	return renderJSON(w, r, done)
})

// checkRestore applies the rules to the place an item would take and to everything inside it.
func checkRestore(d *data, held, target string) error {
	return filepath.Walk(held, func(p string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(held, p)
		if err != nil {
			return err
		}
		if !d.Check(path.Join(target, filepath.ToSlash(rel))) {
			return fberrors.ErrPermissionDenied
		}
		return nil
	})
}

var trashPurgeHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	if !d.user.Perm.Delete {
		return http.StatusForbidden, nil
	}
	ids, err := readIDs(w, r)
	if err != nil {
		return http.StatusBadRequest, err
	}

	bin := d.bin()
	for _, id := range ids {
		if _, err := bin.Get(id); err != nil {
			return http.StatusNotFound, err
		}
		if err := bin.Forget(id); err != nil {
			return http.StatusInternalServerError, err
		}
	}
	return http.StatusNoContent, nil
})

var trashEmptyHandler = withUser(func(_ http.ResponseWriter, _ *http.Request, d *data) (int, error) {
	if !d.user.Perm.Delete {
		return http.StatusForbidden, nil
	}
	if err := d.bin().Empty(); err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusNoContent, nil
})

// trashUsageHandler tells an admin how much every bin holds, without naming anything in them.
var trashUsageHandler = withAdmin(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	count, size, err := trash.Usage(d.server.Root)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return renderJSON(w, r, map[string]interface{}{"count": count, "size": size})
})

var trashEmptyAllHandler = withAdmin(func(_ http.ResponseWriter, _ *http.Request, d *data) (int, error) {
	if err := trash.EmptyAll(d.server.Root); err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusNoContent, nil
})
