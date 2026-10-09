package fbhttp

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	fberrors "github.com/filebrowser/filebrowser/v2/errors"
	"github.com/filebrowser/filebrowser/v2/files"
)

// Favourites (Gezgin, K87, K88): each user marks files and folders, which the "Sık kullanılanlar"
// page lists. They are kept in the user's record, so every device shows the same list, and change
// one at a time, so two open tabs cannot undo each other. A favourite follows its item through a
// rename or move made in Gezgin, and goes with a delete; one the user can no longer reach is left
// out, and dropped, when the list is read.

const (
	maxFavorites    = 20
	maxFavoritePath = 1024
)

// favoritesMu orders the changes of every user's favourites.
var favoritesMu sync.Mutex

type favorite struct {
	Path     string    `json:"path"`
	Name     string    `json:"name"`
	IsDir    bool      `json:"isDir"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
}

type favoriteRequest struct {
	Path string `json:"path"`
}

// favoritePath is the clean path a request names, or an error.
func favoritePath(r *http.Request) (string, error) {
	var req favoriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return "", fmt.Errorf("%w: %w", fberrors.ErrInvalidRequestParams, err)
	}
	p := path.Clean("/" + req.Path)
	if p == "/" || len(p) > maxFavoritePath {
		return "", fberrors.ErrInvalidRequestParams
	}
	return p, nil
}

// reach is what the user's favourite at p shows, or an error when they cannot reach it.
func (d *data) reach(p string) (*favorite, error) {
	file, err := files.NewFileInfo(&files.FileOptions{Fs: d.user.Fs, Path: p, Checker: d})
	if err != nil {
		return nil, err
	}
	return &favorite{Path: p, Name: file.Name, IsDir: file.IsDir, Size: file.Size, Modified: file.ModTime}, nil
}

// listFavorites returns the user's favourites they can reach, dropping the others from the record.
// favoritesMu must be held.
func listFavorites(d *data) ([]favorite, error) {
	list := []favorite{}
	var kept []string
	for _, p := range d.user.Favorites {
		f, err := d.reach(p)
		if err != nil {
			continue
		}
		list = append(list, *f)
		kept = append(kept, p)
	}
	if len(kept) != len(d.user.Favorites) {
		d.user.Favorites = kept
		if err := d.store.Users.Update(d.user, "Favorites"); err != nil {
			return nil, err
		}
	}
	return list, nil
}

var favoritesGetHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	favoritesMu.Lock()
	defer favoritesMu.Unlock()
	list, err := listFavorites(d)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return renderJSON(w, r, list)
})

var favoritesPostHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	p, err := favoritePath(r)
	if err != nil {
		return http.StatusBadRequest, err
	}
	favoritesMu.Lock()
	defer favoritesMu.Unlock()
	if _, err := d.reach(p); err != nil {
		return errToStatus(err), err
	}
	if !slices.Contains(d.user.Favorites, p) {
		if len(d.user.Favorites) >= maxFavorites {
			return http.StatusBadRequest, fmt.Errorf("%w: at most %d favourites", fberrors.ErrInvalidRequestParams, maxFavorites)
		}
		d.user.Favorites = append(d.user.Favorites, p)
		if err := d.store.Users.Update(d.user, "Favorites"); err != nil {
			return http.StatusInternalServerError, err
		}
	}
	list, err := listFavorites(d)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return renderJSON(w, r, list)
})

var favoritesDeleteHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	p, err := favoritePath(r)
	if err != nil {
		return http.StatusBadRequest, err
	}
	favoritesMu.Lock()
	defer favoritesMu.Unlock()
	if i := slices.Index(d.user.Favorites, p); i >= 0 {
		d.user.Favorites = slices.Delete(slices.Clone(d.user.Favorites), i, i+1)
		if err := d.store.Users.Update(d.user, "Favorites"); err != nil {
			return http.StatusInternalServerError, err
		}
	}
	list, err := listFavorites(d)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return renderJSON(w, r, list)
})

// followFavorites rewrites every user's favourites of the item at the real path from, and of
// anything inside it: to the real path to, or, when to is empty or out of a user's scope, away.
func followFavorites(d *data, from, to string) error {
	favoritesMu.Lock()
	defer favoritesMu.Unlock()

	all, err := d.store.Users.Gets(d.server.Root, d.server.FollowExternalSymlinks)
	if err != nil {
		if errors.Is(err, fberrors.ErrNotExist) {
			return nil
		}
		return err
	}
	var errs []error
	for _, u := range all {
		changed := false
		kept := make([]string, 0, len(u.Favorites))
		for _, p := range u.Favorites {
			below, ok := d.inside(from, u.FullPath(p))
			if !ok {
				kept = append(kept, p)
				continue
			}
			changed = true
			if to == "" {
				continue
			}
			names, ok := d.inside(u.FullPath("/"), filepath.Join(append([]string{to}, below...)...))
			if !ok {
				continue
			}
			moved := "/" + strings.Join(names, "/")
			if !slices.Contains(kept, moved) {
				kept = append(kept, moved)
			}
		}
		if changed {
			u.Favorites = kept
			errs = append(errs, d.store.Users.Update(u, "Favorites"))
		}
	}
	return errors.Join(errs...)
}

// moveFavorites points the favourites of the item moved from the user's path src to dst, and of
// anything inside it, everyone's, at the new place.
func moveFavorites(d *data, src, dst string) error {
	return followFavorites(d, d.user.FullPath(src), d.user.FullPath(dst))
}

// dropFavorites removes the favourites of the item at the user's path p and of anything inside it,
// everyone's.
func dropFavorites(d *data, p string) error {
	return followFavorites(d, d.user.FullPath(p), "")
}
