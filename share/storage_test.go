package share

import (
	"reflect"
	"sort"
	"testing"
	"time"

	fberrors "github.com/filebrowser/filebrowser/v2/errors"
)

// memory is a StorageBackend that lists links in the order of their hashes.
type memory map[string]*Link

func (m memory) list(keep func(*Link) bool) ([]*Link, error) {
	hashes := make([]string, 0, len(m))
	for hash := range m {
		hashes = append(hashes, hash)
	}
	sort.Strings(hashes)
	links := []*Link{}
	for _, hash := range hashes {
		if keep(m[hash]) {
			links = append(links, m[hash])
		}
	}
	if len(links) == 0 {
		return links, fberrors.ErrNotExist
	}
	return links, nil
}

func (m memory) All() ([]*Link, error) { return m.list(func(*Link) bool { return true }) }
func (m memory) FindByUserID(id uint) ([]*Link, error) {
	return m.list(func(l *Link) bool { return l.UserID == id })
}
func (m memory) GetByHash(hash string) (*Link, error) {
	if l, ok := m[hash]; ok {
		return l, nil
	}
	return nil, fberrors.ErrNotExist
}
func (m memory) GetPermanent(string, uint) (*Link, error) { return nil, fberrors.ErrNotExist }
func (m memory) Gets(p string, id uint) ([]*Link, error) {
	return m.list(func(l *Link) bool { return l.Path == p && l.UserID == id })
}
func (m memory) Save(l *Link) error                      { m[l.Hash] = l; return nil }
func (m memory) Delete(hash string) error                { delete(m, hash); return nil }
func (m memory) DeleteWithPathPrefix(string, uint) error { return nil }

func hashes(links []*Link) []string {
	out := []string{}
	for _, l := range links {
		out = append(out, l.Hash)
	}
	return out
}

// Expired links are dropped from every listing and deleted, wherever they are in the list (File
// Browser skipped the link after an expired one and could panic).
func TestExpiredLinksAreDropped(t *testing.T) {
	past := time.Now().Add(-time.Hour).Unix()
	future := time.Now().Add(time.Hour).Unix()
	for _, expired := range []string{"ab", "ac", "abc", "bc", "b", ""} {
		back := memory{}
		var live []string
		for _, hash := range []string{"a", "b", "c", "d"} {
			l := &Link{Hash: hash, Path: "/p", UserID: 1, Expire: future}
			if hash == "d" {
				l.Expire = 0
			}
			for _, e := range expired {
				if string(e) == hash {
					l.Expire = past
				}
			}
			if l.Expire != past {
				live = append(live, hash)
			}
			back[hash] = l
		}
		s := NewStorage(back)

		for name, list := range map[string]func() ([]*Link, error){
			"All":          s.All,
			"FindByUserID": func() ([]*Link, error) { return s.FindByUserID(1) },
			"Gets":         func() ([]*Link, error) { return s.Gets("/p", 1) },
		} {
			links, err := list()
			if err != nil {
				t.Fatalf("%s with %q expired: %v", name, expired, err)
			}
			if got := hashes(links); !reflect.DeepEqual(got, live) {
				t.Fatalf("%s with %q expired = %v, want %v", name, expired, got, live)
			}
		}
		for _, e := range expired {
			if _, ok := back[string(e)]; ok {
				t.Fatalf("expired link %c was kept", e)
			}
		}
	}
}

func TestDeleteByUserID(t *testing.T) {
	back := memory{
		"a": {Hash: "a", UserID: 1},
		"b": {Hash: "b", UserID: 2},
		"c": {Hash: "c", UserID: 1},
	}
	s := NewStorage(back)
	if err := s.DeleteByUserID(1); err != nil {
		t.Fatal(err)
	}
	if got := hashes(mustAll(t, s)); !reflect.DeepEqual(got, []string{"b"}) {
		t.Fatalf("left %v, want [b]", got)
	}
	if err := s.DeleteByUserID(3); err != nil {
		t.Fatalf("a user without links: %v", err)
	}
}

func mustAll(t *testing.T, s *Storage) []*Link {
	t.Helper()
	links, err := s.All()
	if err != nil {
		t.Fatal(err)
	}
	return links
}
