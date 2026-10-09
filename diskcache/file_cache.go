package diskcache

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/spf13/afero"
)

type FileCache struct {
	fs afero.Fs

	// granular locks
	scopedLocks struct {
		sync.Mutex
		sync.Once
		locks map[string]sync.Locker
	}
}

func New(fs afero.Fs, root string) *FileCache {
	return &FileCache{
		fs: afero.NewBasePathFs(fs, root),
	}
}

func (f *FileCache) Store(_ context.Context, key string, value []byte) error {
	mu := f.getScopedLocks(key)
	mu.Lock()
	defer mu.Unlock()

	fileName := f.getFileName(key)
	if err := f.fs.MkdirAll(filepath.Dir(fileName), 0700); err != nil {
		return err
	}

	if err := afero.WriteFile(f.fs, fileName, value, 0700); err != nil {
		return err
	}

	return nil
}

func (f *FileCache) Load(_ context.Context, key string) (value []byte, exist bool, err error) {
	r, ok, err := f.open(key)
	if err != nil || !ok {
		return nil, ok, err
	}
	defer r.Close()

	value, err = io.ReadAll(r)
	if err != nil {
		return nil, false, err
	}
	return value, true, nil
}

func (f *FileCache) Delete(_ context.Context, key string) error {
	mu := f.getScopedLocks(key)
	mu.Lock()
	defer mu.Unlock()

	fileName := f.getFileName(key)
	if err := f.fs.Remove(fileName); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Usage counts the cached files and their bytes (Gezgin, K146).
func (f *FileCache) Usage(_ context.Context) (count int, size int64, err error) {
	err = afero.Walk(f.fs, "/", func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			// A file removed meanwhile is not counted.
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		if info.Mode().IsRegular() {
			count++
			size += info.Size()
		}
		return nil
	})
	return count, size, err
}

// Clear removes every cached file (Gezgin, K146); each is made again when it is next asked for.
func (f *FileCache) Clear(_ context.Context) error {
	entries, err := afero.ReadDir(f.fs, "/")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if err := f.fs.RemoveAll(filepath.Join("/", entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func (f *FileCache) open(key string) (afero.File, bool, error) {
	fileName := f.getFileName(key)
	file, err := f.fs.Open(fileName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}

	return file, true, nil
}

// getScopedLocks pull lock from the map if found or create a new one
func (f *FileCache) getScopedLocks(key string) (lock sync.Locker) {
	f.scopedLocks.Do(func() { f.scopedLocks.locks = map[string]sync.Locker{} })

	f.scopedLocks.Lock()
	lock, ok := f.scopedLocks.locks[key]
	if !ok {
		lock = &sync.Mutex{}
		f.scopedLocks.locks[key] = lock
	}
	f.scopedLocks.Unlock()

	return lock
}

func (f *FileCache) getFileName(key string) string {
	hasher := sha1.New()
	_, _ = hasher.Write([]byte(key))
	hash := hex.EncodeToString(hasher.Sum(nil))
	return fmt.Sprintf("%s/%s/%s", hash[:1], hash[1:3], hash)
}
