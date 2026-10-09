package fbhttp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/jellydator/ttlcache/v3"

	"github.com/filebrowser/filebrowser/v2/settings"
)

// UploadsDir is the folder at the server root that holds the data of the uploads in progress
// (Gezgin). No user path reaches it: the rule check refuses it, as it refuses the trash.
const UploadsDir = settings.UploadsDir

const uploadCacheTTL = 3 * time.Minute

// upload is a tus upload in progress. Its data is staged in a file of its own until it is
// complete, so that the destination is not touched before then.
type upload struct {
	mu       sync.Mutex // held while a chunk is written
	staged   string     // the real path of the staged data
	length   int64
	override bool
}

// offset is how much of the upload has arrived.
func (u *upload) offset() (int64, error) {
	info, err := os.Stat(u.staged)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// UploadCache keeps track of the uploads in progress and of their staged data, in a folder of its
// own (Gezgin). An upload no chunk reaches for uploadCacheTTL is dropped with its data, and its
// destination never sees it.
type UploadCache struct {
	dir   string
	mu    sync.Mutex // orders the changes of the uploads
	cache *ttlcache.Cache[string, *upload]
}

// NewUploadCache stages the uploads' data in dir, deleting what an earlier run left there: no one
// can resume those uploads.
func NewUploadCache(dir string) *UploadCache {
	if err := os.RemoveAll(dir); err != nil {
		log.Printf("WARNING: could not delete the uploads an earlier run left in %s: %v", dir, err)
	}
	return newUploadCache(dir, uploadCacheTTL)
}

func newUploadCache(dir string, ttl time.Duration) *UploadCache {
	cache := ttlcache.New(ttlcache.WithTTL[string, *upload](ttl))
	cache.OnEviction(func(_ context.Context, reason ttlcache.EvictionReason, item *ttlcache.Item[string, *upload]) {
		// The other reasons are drop and finish, which delete the data themselves.
		if reason == ttlcache.EvictionReasonExpired {
			log.Printf("deleting the data of an abandoned upload: %s", item.Key())
			discard(item.Value())
		}
	})
	go cache.Start()
	return &UploadCache{dir: dir, cache: cache}
}

// discard deletes an upload's staged data, which a completed upload no longer has.
func discard(u *upload) {
	if err := os.Remove(u.staged); err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Printf("could not delete the staged upload %s: %v", u.staged, err)
	}
}

// begin registers an upload of length bytes under key, with an empty file for its data. An
// unfinished upload under the same key is dropped: its client started over.
func (c *UploadCache) begin(key string, length int64, override bool) (*upload, error) {
	if err := os.MkdirAll(c.dir, 0o700); err != nil {
		return nil, err
	}
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return nil, err
	}
	staged := filepath.Join(c.dir, hex.EncodeToString(random)+".part")
	file, err := os.OpenFile(staged, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, err
	}
	if err = file.Close(); err != nil {
		_ = os.Remove(staged)
		return nil, err
	}

	u := &upload{staged: staged, length: length, override: override}
	c.mu.Lock()
	defer c.mu.Unlock()
	if old, ok := c.cache.GetAndDelete(key); ok {
		discard(old.Value())
	}
	c.cache.Set(key, u, ttlcache.DefaultTTL)
	return u, nil
}

// get returns the upload under key.
func (c *UploadCache) get(key string) (*upload, bool) {
	item := c.cache.Get(key)
	if item == nil {
		return nil, false
	}
	return item.Value(), true
}

// touch keeps the upload under key from expiring.
func (c *UploadCache) touch(key string) {
	c.cache.Touch(key)
}

// drop ends the upload under key and deletes its data. It reports whether there was one.
func (c *UploadCache) drop(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	item, ok := c.cache.GetAndDelete(key)
	if ok {
		discard(item.Value())
	}
	return ok
}

// finish ends the upload u, which was under key, once it is in place or was refused. A newer
// upload under key is left alone.
func (c *UploadCache) finish(key string, u *upload) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if item := c.cache.Get(key, ttlcache.WithDisableTouchOnHit[string, *upload]()); item != nil && item.Value() == u {
		c.cache.Delete(key)
	}
	discard(u)
}

// pending is how many bytes the uploads in progress still have to write.
func (c *UploadCache) pending() int64 {
	var total int64
	for _, item := range c.cache.Items() {
		u := item.Value()
		if offset, err := u.offset(); err == nil && offset < u.length {
			total += u.length - offset
		}
	}
	return total
}

// Close stops the cache.
func (c *UploadCache) Close() {
	c.cache.Stop()
}
