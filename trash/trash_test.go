package trash

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func write(t *testing.T, name, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPutListGetForget(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "docs", "a.txt"), "12345")
	write(t, filepath.Join(root, "docs", "dir", "b.txt"), "123")

	bin := For(root, 7)
	file, err := bin.Put(filepath.Join(root, "docs", "a.txt"), "/docs")
	if err != nil {
		t.Fatal(err)
	}
	dir, err := bin.Put(filepath.Join(root, "docs", "dir"), "/docs")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "a.txt")); !os.IsNotExist(err) {
		t.Errorf("the file is still in place: %v", err)
	}
	if file.Size != 5 || file.IsDir || dir.Size != 3 || !dir.IsDir || dir.Origin != "/docs" || dir.Name != "dir" {
		t.Errorf("items: %+v %+v", file, dir)
	}
	if data, _ := os.ReadFile(bin.Path(file)); string(data) != "12345" {
		t.Errorf("held content %q", data)
	}

	items, err := bin.List()
	if err != nil || len(items) != 2 || items[0].ID != dir.ID {
		t.Fatalf("list = %+v, %v; want both, newest first", items, err)
	}
	if other, _ := For(root, 8).List(); len(other) != 0 {
		t.Errorf("another user's bin lists %d items", len(other))
	}

	if err := bin.Forget(file.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := bin.Get(file.ID); err != ErrNotFound {
		t.Errorf("a forgotten item: %v", err)
	}
	for _, id := range []string{"", "../x", "ABCDEF", "0123456789abcdef0123456789abcdeg"} {
		if _, err := bin.Get(id); err != ErrNotFound {
			t.Errorf("Get(%q) = %v; want ErrNotFound", id, err)
		}
	}

	count, size, err := Usage(root)
	if err != nil || count != 1 || size != 3 {
		t.Errorf("usage = %d %d %v; want 1 item of 3 bytes", count, size, err)
	}
	if err := EmptyAll(root); err != nil {
		t.Fatal(err)
	}
	if count, _, _ := Usage(root); count != 0 {
		t.Errorf("%d items after emptying everything", count)
	}
}

func TestSweep(t *testing.T) {
	root := t.TempDir()
	bin := For(root, 1)
	write(t, filepath.Join(root, "old.txt"), "o")
	write(t, filepath.Join(root, "new.txt"), "n")
	old, err := bin.Put(filepath.Join(root, "old.txt"), "/")
	if err != nil {
		t.Fatal(err)
	}
	newer, err := bin.Put(filepath.Join(root, "new.txt"), "/")
	if err != nil {
		t.Fatal(err)
	}

	// A record whose item is gone and an item without a record, as an interrupted move leaves them.
	orphanRecord := "0123456789abcdef0123456789abcdef"
	write(t, filepath.Join(bin.dir, orphanRecord+".json"), `{"id":"`+orphanRecord+`","name":"x"}`)
	orphanItem := "fedcba9876543210fedcba9876543210"
	write(t, filepath.Join(bin.dir, orphanItem, "y.txt"), "y")

	// Right away nothing goes: the items are within their time and the leftovers may be moves in
	// progress.
	if removed, err := Sweep(root, 30*24*time.Hour, time.Now()); err != nil || removed != 0 {
		t.Errorf("an early sweep removed %d (%v)", removed, err)
	}

	later := old.Deleted.Add(10 * 24 * time.Hour)
	if removed, err := Sweep(root, 30*24*time.Hour, later); err != nil || removed != 2 {
		t.Errorf("a sweep within the items' time removed %d (%v); want only the two leftovers", removed, err)
	}
	if items, _ := bin.List(); len(items) != 2 {
		t.Errorf("items within their time: %+v", items)
	}

	removed, err := Sweep(root, 5*24*time.Hour, later)
	if err != nil || removed != 2 {
		t.Errorf("sweep removed %d (%v); want the two expired items", removed, err)
	}
	if items, _ := bin.List(); len(items) != 0 {
		t.Errorf("after the sweep: %+v", items)
	}
	_ = newer
	if entries, _ := os.ReadDir(bin.dir); len(entries) != 0 {
		t.Errorf("left in the bin: %v", entries)
	}
}
