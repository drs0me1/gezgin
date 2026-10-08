// Package trash keeps what Gezgin users delete until they restore it, delete it for good or it
// expires. Every user has a bin at <server root>/.gezgin-cop/<user id>/, on the same disk as the
// files, so that moving an item in or out is a rename. An item is kept as <id>/<name> beside an
// <id>.json record of where it came from. No user path reaches the bins: Gezgin's rule check
// refuses the folder for every user whose scope contains it.
package trash

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Dir is the name of the folder at the server root that holds the bins.
const Dir = ".gezgin-cop"

var (
	// ErrOtherDisk is returned for an item that is not on the bins' disk: it cannot be moved
	// there without a copy, so it can only be deleted for good.
	ErrOtherDisk = errors.New("the item is on another disk than the trash")
	// ErrNotFound is returned for an id the bin does not hold.
	ErrNotFound = errors.New("the item is not in the trash")

	idPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)
)

// Item describes something in a bin.
type Item struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Origin  string    `json:"origin"` // the folder it was in, relative to the owner's scope
	Deleted time.Time `json:"deleted"`
	Size    int64     `json:"size"`
	IsDir   bool      `json:"isDir"`
}

// Bin is one user's trash.
type Bin struct {
	dir string
}

// For returns the bin of the user with the given id under the server root.
func For(root string, user uint) *Bin {
	return &Bin{dir: filepath.Join(root, Dir, strconv.FormatUint(uint64(user), 10))}
}

// ValidID reports whether id has the form of an item id, so that it can name a path.
func ValidID(id string) bool {
	return idPattern.MatchString(id)
}

func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (b *Bin) record(id string) string { return filepath.Join(b.dir, id+".json") }
func (b *Bin) holder(id string) string { return filepath.Join(b.dir, id) }

// Path is where the item's file or folder is kept.
func (b *Bin) Path(item Item) string { return filepath.Join(b.holder(item.ID), item.Name) }

// Put moves the file or folder at the real path src, which the user sees in the folder origin,
// into the bin.
func (b *Bin) Put(src, origin string) (Item, error) {
	info, err := os.Lstat(src)
	if err != nil {
		return Item{}, err
	}
	id, err := newID()
	if err != nil {
		return Item{}, err
	}
	item := Item{ID: id, Name: filepath.Base(src), Origin: origin, Deleted: time.Now().UTC(),
		Size: size(src, info), IsDir: info.IsDir()}

	if err = os.MkdirAll(b.holder(id), 0o700); err != nil {
		return Item{}, err
	}
	if err = writeRecord(b.record(id), item); err != nil {
		_ = os.RemoveAll(b.holder(id))
		return Item{}, err
	}
	if err = os.Rename(src, b.Path(item)); err != nil {
		_ = os.Remove(b.record(id))
		_ = os.RemoveAll(b.holder(id))
		if errors.Is(err, syscall.EXDEV) {
			return Item{}, ErrOtherDisk
		}
		return Item{}, err
	}
	return item, nil
}

// size adds up the regular files under path, without following links.
func size(path string, info fs.FileInfo) int64 {
	if !info.IsDir() {
		return info.Size()
	}
	var total int64
	_ = filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Type().IsRegular() {
			if fi, err := d.Info(); err == nil {
				total += fi.Size()
			}
		}
		return nil
	})
	return total
}

func writeRecord(name string, item Item) error {
	data, err := json.Marshal(item)
	if err != nil {
		return err
	}
	tmp := name + ".tmp"
	if err = os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err = os.Rename(tmp, name); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// Get returns the item with the given id.
func (b *Bin) Get(id string) (Item, error) {
	if !ValidID(id) {
		return Item{}, ErrNotFound
	}
	data, err := os.ReadFile(b.record(id))
	if errors.Is(err, fs.ErrNotExist) {
		return Item{}, ErrNotFound
	}
	if err != nil {
		return Item{}, err
	}
	var item Item
	if err = json.Unmarshal(data, &item); err != nil || item.ID != id || item.Name == "" ||
		strings.ContainsAny(item.Name, `/\`) || item.Name == "." || item.Name == ".." {
		return Item{}, ErrNotFound
	}
	if _, err = os.Lstat(b.Path(item)); err != nil {
		return Item{}, ErrNotFound
	}
	return item, nil
}

// List returns the bin's items, the most recently deleted first.
func (b *Bin) List() ([]Item, error) {
	entries, err := os.ReadDir(b.dir)
	if errors.Is(err, fs.ErrNotExist) {
		return []Item{}, nil
	}
	if err != nil {
		return nil, err
	}
	items := []Item{}
	for _, e := range entries {
		id, ok := strings.CutSuffix(e.Name(), ".json")
		if !ok || !ValidID(id) {
			continue
		}
		if item, err := b.Get(id); err == nil {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Deleted.After(items[j].Deleted) })
	return items, nil
}

// Forget drops the item's record and whatever is left of it; after a restore that is nothing.
func (b *Bin) Forget(id string) error {
	if !ValidID(id) {
		return ErrNotFound
	}
	if err := os.RemoveAll(b.holder(id)); err != nil {
		return err
	}
	for _, name := range []string{b.record(id), b.record(id) + ".tmp"} {
		if err := os.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}

// Empty deletes everything in the bin for good.
func (b *Bin) Empty() error {
	return os.RemoveAll(b.dir)
}

// Usage counts the items in every bin under the server root and their size.
func Usage(root string) (count int, total int64, err error) {
	err = eachBin(root, func(b *Bin) error {
		items, err := b.List()
		for _, item := range items {
			count++
			total += item.Size
		}
		return err
	})
	return count, total, err
}

// EmptyAll deletes everything in every bin for good.
func EmptyAll(root string) error {
	return os.RemoveAll(filepath.Join(root, Dir))
}

// Sweep deletes for good the items deleted more than maxAge before now, and leftovers of
// interrupted moves (a held item without a record or a record without its item), in every bin.
func Sweep(root string, maxAge time.Duration, now time.Time) (removed int, err error) {
	err = eachBin(root, func(b *Bin) error {
		entries, err := os.ReadDir(b.dir)
		if err != nil {
			return err
		}

		// An item has a holder folder, a record and, while it is written, a temporary record.
		touched := map[string]time.Time{}
		for _, e := range entries {
			id := strings.TrimSuffix(strings.TrimSuffix(e.Name(), ".tmp"), ".json")
			info, err := e.Info()
			if !ValidID(id) || err != nil {
				continue
			}
			if t, ok := touched[id]; !ok || info.ModTime().After(t) {
				touched[id] = info.ModTime()
			}
		}

		for id, last := range touched {
			item, err := b.Get(id)
			switch {
			case err == nil:
				if now.Sub(item.Deleted) <= maxAge {
					continue
				}
			case errors.Is(err, ErrNotFound):
				// A leftover is only swept once it is old enough not to be a move in progress.
				if now.Sub(last) < time.Hour {
					continue
				}
			default:
				return err
			}
			if err := b.Forget(id); err != nil {
				return err
			}
			removed++
		}
		return nil
	})
	return removed, err
}

func eachBin(root string, fn func(*Bin) error) error {
	entries, err := os.ReadDir(filepath.Join(root, Dir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var errs []error
	for _, e := range entries {
		user, err := strconv.ParseUint(e.Name(), 10, 0)
		if err != nil || !e.IsDir() {
			continue
		}
		errs = append(errs, fn(For(root, uint(user))))
	}
	return errors.Join(errs...)
}
