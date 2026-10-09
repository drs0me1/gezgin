package fbhttp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/users"
)

// The global settings are checked when saved (K66-K69): a value out of bounds answers 400 with
// a message the interface translates, and nothing is stored.
func TestGlobalSettingsAreChecked(t *testing.T) {
	env := newSessionEnv(t)
	env.addUser("root", "root-password-1", true, false)
	token := env.login("root", "root-password-1")

	current := func() map[string]any {
		rec := env.do(http.MethodGet, "/api/settings", token, "", "")
		var set map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &set); err != nil {
			t.Fatalf("settings = %d %q", rec.Code, rec.Body.String())
		}
		return set
	}
	put := func(change func(map[string]any)) (int, string) {
		set := current()
		change(set)
		body, _ := json.Marshal(set)
		rec := env.do(http.MethodPut, "/api/settings", token, string(body), "")
		return rec.Code, rec.Body.String()
	}
	at := func(set map[string]any, keys ...string) map[string]any {
		for _, k := range keys {
			set = set[k].(map[string]any)
		}
		return set
	}

	bad := map[string]struct {
		change  func(map[string]any)
		message string
	}{
		"password length 1": {func(s map[string]any) { s["minimumPasswordLength"] = 1 },
			"the minimum password length must be from 8 to 32"},
		"password length 100": {func(s map[string]any) { s["minimumPasswordLength"] = 100 },
			"the minimum password length must be from 8 to 32"},
		"chunk size 0": {func(s map[string]any) { at(s, "tus")["chunkSize"] = 0 },
			"the chunk size must be from 1048576 to 1073741824 bytes"},
		"retries 21": {func(s map[string]any) { at(s, "tus")["retryCount"] = 21 },
			"the retry count must be from 0 to 20"},
		"home base in the archives": {func(s map[string]any) { s["userHomeBasePath"] = "/.gezgin-arsiv" },
			"a scope cannot lie in Gezgin's folders"},
		"default scope in the uploads": {func(s map[string]any) { at(s, "defaults")["scope"] = "/.gezgin-yukleme" },
			"a scope cannot lie in Gezgin's folders"},
		"language xx":  {func(s map[string]any) { at(s, "defaults")["locale"] = "xx" }, `unknown language "xx"`},
		"view mode zz": {func(s map[string]any) { at(s, "defaults")["viewMode"] = "zz" }, `unknown view mode "zz"`},
		"theme bogus":  {func(s map[string]any) { at(s, "branding")["theme"] = "bogus" }, `unknown theme "bogus"`},
	}
	for name, c := range bad {
		if code, body := put(c.change); code != http.StatusBadRequest || !strings.Contains(body, c.message) {
			t.Errorf("%s = %d %q; want 400 with %q", name, code, body, c.message)
		}
	}
	if set := current(); set["minimumPasswordLength"] != float64(8) || at(set, "tus")["chunkSize"] != float64(settings.DefaultTusChunkSize) ||
		set["userHomeBasePath"] != "/users" || at(set, "defaults")["locale"] != "tr" {
		t.Errorf("a refused value was stored: %v", set)
	}

	// K69: the defaults never grant the admin permission, and sharing comes with downloading.
	code, body := put(func(s map[string]any) {
		perm := at(s, "defaults", "perm")
		perm["admin"], perm["share"], perm["download"] = true, true, false
		s["minimumPasswordLength"] = 12
		at(s, "tus")["chunkSize"] = 1 << 20
	})
	if code != http.StatusOK {
		t.Fatalf("valid settings = %d %q", code, body)
	}
	set := current()
	if perm := at(set, "defaults", "perm"); perm["admin"] != false || perm["download"] != true {
		t.Errorf("default permissions stored as %v; want no admin, and download with share", perm)
	}
	if set["minimumPasswordLength"] != float64(12) || at(set, "tus")["chunkSize"] != float64(1<<20) {
		t.Errorf("valid values not stored: %v", set)
	}
	// K70, K71, K74: the branding holds only the theme and the disk bar; the removed fields are gone.
	if branding := at(set, "branding"); len(branding) != 2 {
		t.Errorf("branding %v; want only theme and disableUsedPercentage", branding)
	}
	if _, ok := set["hideLoginButton"]; ok {
		t.Errorf("hideLoginButton is still in the settings")
	}
}

// K68: a user's scope cannot lie in Gezgin's folders, whose contents no path reaches and which
// are emptied at every start.
func TestUserScopeCannotBeAGezginFolder(t *testing.T) {
	env := newSessionEnv(t)
	root := env.addUser("root", "root-password-1", true, false)
	token := env.login("root", "root-password-1")

	for _, scope := range []string{"/.gezgin-arsiv/ayse", "/.gezgin-yukleme", "/.Gezgin-Cop"} {
		body := fmt.Sprintf(`{"what":"user","which":[],"current_password":"root-password-1","data":{"username":"ayse","password":"ayse-password-1","scope":%q,"locale":"tr","perm":{}}}`, scope)
		if rec := env.do(http.MethodPost, "/api/users", token, body, ""); rec.Code != http.StatusBadRequest ||
			!strings.Contains(rec.Body.String(), "a scope cannot lie in Gezgin's folders") {
			t.Errorf("new user scoped at %s = %d %q; want 400", scope, rec.Code, rec.Body.String())
		}
	}
	if _, err := env.st.Users.Get("", false, "ayse"); err == nil {
		t.Errorf("a user scoped in Gezgin's folders was created")
	}

	data := fmt.Sprintf(`{"id":%d,"username":"root","scope":"/.gezgin-arsiv/root"}`, root.ID)
	if rec := env.put(token, root.ID, `["scope"]`, "root-password-1", data); rec.Code != http.StatusBadRequest {
		t.Errorf("moving a scope into the archive folder = %d %q; want 400", rec.Code, rec.Body.String())
	}
	if scope := env.stored(root.ID).Scope; scope != "." {
		t.Errorf("scope after a refused change: %q", scope)
	}
}

// K66: a link password bcrypt cannot take answers 400 instead of 500.
func TestSharePasswordTooLong(t *testing.T) {
	env := newFileEnv(t)
	ali, _ := env.user("ali", "ali-password-1", "/ali", users.Permissions{Share: true, Download: true})
	env.write("ali/rapor.pdf", "pdf", 0o644)
	body := fmt.Sprintf(`{"password":%q}`, strings.Repeat("ğ", 37))
	if rec := ali.call(http.MethodPost, "/api/share/rapor.pdf", body); rec.Code != http.StatusBadRequest ||
		!strings.Contains(rec.Body.String(), "password is too long") {
		t.Errorf("a link password of 74 bytes = %d %q; want 400", rec.Code, rec.Body.String())
	}
}
