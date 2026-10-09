package fbhttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/filebrowser/filebrowser/v2/diskcache"
	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/version"
)

// pageEnv serves the API and a page template like the built one.
func pageEnv(t *testing.T) (*sessionEnv, http.Handler) {
	t.Helper()
	env := newSessionEnv(t)
	assets := fstest.MapFS{
		"public/index.html": {Data: []byte(`<!doctype html><title>Gezgin</title>` +
			`<script id="gezgin-settings" type="application/json">[{[ .Json ]}]</script>`)},
	}
	handler, err := NewHandler(nil, diskcache.NewNoOp(), env.uploads, env.archives, env.st,
		&settings.Server{Root: env.root, BaseURL: "/gezgin"}, assets)
	if err != nil {
		t.Fatal(err)
	}
	return env, handler
}

func get(h http.Handler, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("X-Auth", token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// K76: the page itself carries the CSP and may not be framed. It is the router's not-found
// handler, which a middleware never reached, so it had no CSP at all.
func TestPageHasSecurityHeaders(t *testing.T) {
	_, h := pageEnv(t)
	for _, p := range []string{"/gezgin/", "/gezgin/login", "/gezgin/files/a/b", "/gezgin/share/abc"} {
		rec := get(h, p, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s = %d", p, rec.Code)
		}
		csp := rec.Header().Values("Content-Security-Policy")
		if len(csp) != 1 || csp[0] != pageCSP {
			t.Errorf("%s: CSP %q; want the page policy", p, csp)
		}
		if !strings.Contains(pageCSP, "frame-ancestors 'none'") || !strings.Contains(pageCSP, "script-src 'self';") {
			t.Errorf("the page policy lets scripts in or other sites frame the page: %s", pageCSP)
		}
		if got := rec.Header().Get("Referrer-Policy"); got != "same-origin" {
			t.Errorf("%s: Referrer-Policy %q", p, got)
		}
		if got := rec.Header().Get("X-Xss-Protection"); got != "" {
			t.Errorf("%s: X-Xss-Protection %q is still sent", p, got)
		}
	}
}

// K77: the settings reach the page as a JSON data block, which a page under a CSP without inline
// scripts can read.
func TestPageSettingsAreAJSONBlock(t *testing.T) {
	_, h := pageEnv(t)
	body := get(h, "/gezgin/", "").Body.String()
	m := regexp.MustCompile(`(?s)<script id="gezgin-settings" type="application/json">(.*)</script>`).FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("no settings block in %q", body)
	}
	var page map[string]any
	if err := json.Unmarshal([]byte(m[1]), &page); err != nil {
		t.Fatalf("settings block %q: %v", m[1], err)
	}
	if page["BaseURL"] != "/gezgin" || page["StaticURL"] != "/gezgin/static" {
		t.Errorf("settings %v", page)
	}

	rec := get(h, "/gezgin/manifest.webmanifest", "")
	var manifest struct {
		Name     string `json:"name"`
		StartURL string `json:"start_url"`
		Icons    []struct {
			Src string `json:"src"`
		} `json:"icons"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &manifest); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("manifest = %d %q: %v", rec.Code, rec.Body.String(), err)
	}
	if manifest.Name != "Gezgin" || manifest.StartURL != "/gezgin/" || len(manifest.Icons) != 2 ||
		!strings.HasPrefix(manifest.Icons[0].Src, "/gezgin/static/img/icons/") {
		t.Errorf("manifest %+v", manifest)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/manifest+json" {
		t.Errorf("manifest type %q", ct)
	}
	// The icons keep their names from one release to the next and are cached for a day; the
	// version in their addresses brings a changed one to a browser that kept the old.
	for _, icon := range manifest.Icons {
		if !strings.HasSuffix(icon.Src, "?v="+url.QueryEscape(version.Version)) {
			t.Errorf("icon %q carries no version", icon.Src)
		}
	}
}

// K78: an unknown API path answers 404, not the page.
func TestUnknownAPIPathIsNotFound(t *testing.T) {
	env, h := pageEnv(t)
	env.addUser("root", "root-password-1", true, false)
	token := env.login("root", "root-password-1")
	for _, p := range []string{"/gezgin/api/yok", "/gezgin/api/public/yok", "/gezgin/api/"} {
		if rec := get(h, p, token); rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), "<title>") {
			t.Errorf("%s = %d %q; want 404", p, rec.Code, rec.Body.String())
		}
	}
	if rec := get(h, "/gezgin/api/resources/", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("a known API path without a session = %d; want 401", rec.Code)
	}
	if rec := get(h, "/gezgin/api/resources/", token); rec.Code != http.StatusOK {
		t.Errorf("a known API path = %d; want 200", rec.Code)
	}
}

// K76: a user's file served as it is runs no script and may be framed by Gezgin alone (the PDF
// preview), with one policy instead of two that had to be combined.
func TestRawFileHeaders(t *testing.T) {
	env := newFileEnv(t)
	env.write("sayfa.html", "<script>alert(1)</script>", 0o644)
	for _, target := range []string{"/api/raw/sayfa.html?inline=true", "/api/raw/sayfa.html"} {
		rec := env.call(http.MethodGet, target, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s = %d", target, rec.Code)
		}
		csp := rec.Header().Values("Content-Security-Policy")
		if len(csp) != 1 || csp[0] != rawCSP {
			t.Errorf("%s: CSP %q; want the raw policy", target, csp)
		}
		if !strings.Contains(rawCSP, "script-src 'none'") || !strings.Contains(rawCSP, "frame-ancestors 'self'") {
			t.Errorf("raw policy %q", rawCSP)
		}
	}
}
