package fbhttp

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func exists(t *testing.T, name string) bool {
	t.Helper()
	_, err := os.Stat(name)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return err == nil
}

// An abandoned upload expires with its staged data, and with nothing else.
func TestUploadCacheExpiryDeletesStagedData(t *testing.T) {
	dir := filepath.Join(t.TempDir(), UploadsDir)
	c := newUploadCache(dir, 50*time.Millisecond)
	t.Cleanup(c.Close)

	u, err := c.begin("1:/a.bin", 10, false)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(u.staged) != dir || !exists(t, u.staged) {
		t.Fatalf("the data is not staged in %s: %s", dir, u.staged)
	}

	deadline := time.Now().Add(5 * time.Second)
	for exists(t, u.staged) {
		if time.Now().After(deadline) {
			t.Fatal("the expired upload kept its data")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, ok := c.get("1:/a.bin"); ok {
		t.Fatal("the expired upload is still known")
	}
}

// A client that starts over replaces its upload; a finished upload does not end a newer one.
func TestUploadCacheBeginAndFinish(t *testing.T) {
	c := newUploadCache(filepath.Join(t.TempDir(), UploadsDir), time.Minute)
	t.Cleanup(c.Close)

	first, err := c.begin("1:/a.bin", 10, false)
	if err != nil {
		t.Fatal(err)
	}
	second, err := c.begin("1:/a.bin", 20, true)
	if err != nil {
		t.Fatal(err)
	}
	if exists(t, first.staged) {
		t.Fatal("the replaced upload kept its data")
	}

	c.finish("1:/a.bin", first)
	if u, ok := c.get("1:/a.bin"); !ok || u != second || !exists(t, second.staged) {
		t.Fatal("finishing the replaced upload ended the newer one")
	}

	if err := os.WriteFile(second.staged, []byte("12345"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := c.pending(); got != 15 {
		t.Fatalf("pending() = %d, want 15", got)
	}

	if !c.drop("1:/a.bin") || exists(t, second.staged) {
		t.Fatal("drop kept the upload or its data")
	}
	if c.drop("1:/a.bin") {
		t.Fatal("drop found an upload that had ended")
	}
}

// What a stopped run left behind cannot be resumed: a new cache starts empty.
func TestNewUploadCacheDeletesLeftovers(t *testing.T) {
	dir := filepath.Join(t.TempDir(), UploadsDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	left := filepath.Join(dir, "0123456789abcdef0123456789abcdef.part")
	if err := os.WriteFile(left, []byte("half"), 0o600); err != nil {
		t.Fatal(err)
	}

	c := NewUploadCache(dir)
	t.Cleanup(c.Close)
	if exists(t, left) {
		t.Fatal("the new cache kept an earlier run's upload")
	}
}
