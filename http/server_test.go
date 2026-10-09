package fbhttp

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/spf13/afero"

	"github.com/filebrowser/filebrowser/v2/diskcache"
)

func TestServerInfoIsForAdmins(t *testing.T) {
	env := newSessionEnv(t)
	env.addUser("root", "root-password-1", true, false)
	env.addUser("alice", "alice-password-1", false, false)

	for path, method := range map[string]string{"/api/server": http.MethodGet, "/api/server/cache": http.MethodDelete} {
		if code := env.do(method, path, env.login("alice", "alice-password-1"), "", "").Code; code != http.StatusForbidden {
			t.Errorf("a user's %s %s = %d; want 403", method, path, code)
		}
	}

	rec := env.do(http.MethodGet, "/api/server", env.login("root", "root-password-1"), "", "")
	var info serverInfo
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &info) != nil {
		t.Fatalf("the server page = %d %q; want 200 and its JSON", rec.Code, rec.Body.String())
	}
	if info.SessionSeconds != int64(DefaultTokenExpirationTime.Seconds()) || info.Cache != nil {
		t.Errorf("session %d s, cache %v; want %v and no cache", info.SessionSeconds, info.Cache, DefaultTokenExpirationTime)
	}
}

func TestServerCacheClears(t *testing.T) {
	ctx := context.Background()
	cache := diskcache.New(afero.NewMemMapFs(), "/cache")
	if err := cache.Store(ctx, "thumb", []byte("12345")); err != nil {
		t.Fatal(err)
	}
	env := newSessionEnvWithCache(t, cache)
	env.addUser("root", "root-password-1", true, false)
	token := env.login("root", "root-password-1")

	read := func() *cacheInfo {
		var info serverInfo
		rec := env.do(http.MethodGet, "/api/server", token, "", "")
		if err := json.Unmarshal(rec.Body.Bytes(), &info); err != nil {
			t.Fatalf("the server page = %d %q", rec.Code, rec.Body.String())
		}
		return info.Cache
	}
	if c := read(); c == nil || c.Count != 1 || c.Size != 5 {
		t.Fatalf("the cache before clearing = %+v; want 1 file of 5 bytes", c)
	}
	if code := env.do(http.MethodDelete, "/api/server/cache", token, "", "").Code; code != http.StatusNoContent {
		t.Fatalf("clearing the cache = %d; want 204", code)
	}
	if c := read(); c == nil || c.Count != 0 || c.Size != 0 {
		t.Errorf("the cache after clearing = %+v; want empty", c)
	}
}
