package fbhttp

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	fberrors "github.com/filebrowser/filebrowser/v2/errors"
	"github.com/filebrowser/filebrowser/v2/rules"
	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/users"
)

func TestTheLastAdminKeepsTheAdminPermission(t *testing.T) {
	env := newSessionEnv(t)
	root := env.addUser("root", "root-password-1", true, false)
	token := env.login("root", "root-password-1")
	demote := func(id uint, which string) *http.Response {
		form := fmt.Sprintf(`{"id":%d,"username":"%s","scope":".","perm":{"admin":false,"download":true}}`, id, env.stored(id).Username)
		return env.put(token, id, which, "root-password-1", form).Result()
	}

	for _, which := range []string{`["all"]`, `["perm"]`} {
		res := demote(root.ID, which)
		if res.StatusCode != http.StatusBadRequest || !env.stored(root.ID).Perm.Admin {
			t.Errorf("the sole admin dropping %s their admin permission = %d, admin %t; want 400 and still admin",
				which, res.StatusCode, env.stored(root.ID).Perm.Admin)
		}
	}

	// The command line goes through the same check.
	stored := env.stored(root.ID)
	stored.Perm.Admin = false
	if err := env.st.Users.Update(stored); !errors.Is(err, fberrors.ErrLastAdmin) {
		t.Errorf("demoting the sole admin from the command line: %v; want ErrLastAdmin", err)
	}

	second := env.addUser("second", "second-password-1", true, false)
	if res := demote(second.ID, `["perm"]`); res.StatusCode != http.StatusOK || env.stored(second.ID).Perm.Admin {
		t.Errorf("demoting one of two admins = %d; want 200", res.StatusCode)
	}
}

func TestOnlyAdminsDeleteUsers(t *testing.T) {
	env := newSessionEnv(t)
	env.addUser("root", "root-password-1", true, false)
	alice := env.addUser("alice", "alice-password-1", false, false)
	remove := func(token string, id uint, password string) int {
		return env.do(http.MethodDelete, fmt.Sprintf("/api/users/%d", id), token, fmt.Sprintf(`{"current_password":%q}`, password), "").Code
	}

	if code := remove(env.login("alice", "alice-password-1"), alice.ID, "alice-password-1"); code != http.StatusForbidden {
		t.Errorf("a user deleting their own account = %d; want 403", code)
	}
	if code := remove(env.login("root", "root-password-1"), alice.ID, "root-password-1"); code != http.StatusOK {
		t.Errorf("the admin deleting the user = %d; want 200", code)
	}
}

func TestRulesAreCheckedWhenSaved(t *testing.T) {
	env := newSessionEnv(t)
	env.addUser("root", "root-password-1", true, false)
	alice := env.addUser("alice", "alice-password-1", false, false)
	token := env.login("root", "root-password-1")

	bad := map[string]string{
		"broken expression": `[{"regex":true,"allow":false,"regexp":{"raw":"("}}]`,
		"empty expression":  `[{"regex":true,"allow":false,"regexp":{"raw":""}}]`,
		"missing regexp":    `[{"regex":true,"allow":false}]`,
		"empty path":        `[{"regex":false,"allow":false,"path":""}]`,
	}
	for name, list := range bad {
		data := fmt.Sprintf(`{"id":%d,"rules":%s}`, alice.ID, list)
		if rec := env.put(token, alice.ID, `["rules"]`, "root-password-1", data); rec.Code != http.StatusBadRequest ||
			!strings.Contains(rec.Body.String(), "invalid rule: rule 1") {
			t.Errorf("user rule with %s = %d %q; want 400 naming the rule", name, rec.Code, rec.Body.String())
		}
		settings := fmt.Sprintf(`{"minimumPasswordLength":8,"rules":%s}`, list)
		if rec := env.do(http.MethodPut, "/api/settings", token, settings, ""); rec.Code != http.StatusBadRequest {
			t.Errorf("global rule with %s = %d %q; want 400", name, rec.Code, rec.Body.String())
		}
	}
	if len(env.stored(alice.ID).Rules) != 0 {
		t.Errorf("a refused rule was stored")
	}

	good := fmt.Sprintf(`{"id":%d,"rules":[{"regex":true,"allow":false,"regexp":{"raw":"\\.secret$"}},{"regex":false,"allow":false,"path":"/private"}]}`, alice.ID)
	if rec := env.put(token, alice.ID, `["rules"]`, "root-password-1", good); rec.Code != http.StatusOK {
		t.Fatalf("valid rules = %d %q; want 200", rec.Code, rec.Body.String())
	}
	if rec := env.do(http.MethodGet, "/api/resources/", env.login("alice", "alice-password-1"), "", ""); rec.Code != http.StatusOK {
		t.Errorf("listing with valid rules = %d; want 200", rec.Code)
	}

	created := `{"what":"user","which":[],"current_password":"root-password-1","data":{"username":"bob","password":"bob-password-1","scope":"bob","perm":{},"rules":[{"regex":true,"regexp":{"raw":"("}}]}}`
	if rec := env.do(http.MethodPost, "/api/users", token, created, ""); rec.Code != http.StatusBadRequest {
		t.Errorf("creating a user with a broken rule = %d; want 400", rec.Code)
	}
	if err := rules.Validate([]rules.Rule{{Path: "/"}}); err != nil {
		t.Errorf("a rule for the whole tree: %v", err)
	}
}

func TestOwnAccountFields(t *testing.T) {
	env := newSessionEnv(t)
	root := env.addUser("root", "root-password-1", true, false)
	alice := env.addUser("alice", "alice-password-1", false, false)
	aliceToken := env.login("alice", "alice-password-1")
	rootToken := env.login("root", "root-password-1")
	data := fmt.Sprintf(`{"id":%d,"locale":"tr","viewMode":"list","scope":"/x","username":"mallory","perm":{"admin":true},"lockPassword":true}`, alice.ID)

	for _, field := range []string{"locale", "viewMode", "sorting", "hideDotfiles", "singleClick", "dateFormat", "redirectAfterCopyMove"} {
		if rec := env.put(aliceToken, alice.ID, `["`+field+`"]`, "", data); rec.Code != http.StatusOK {
			t.Errorf("a user changing their own %s = %d; want 200", field, rec.Code)
		}
	}
	for _, field := range []string{"scope", "username", "perm", "lockPassword", "rules", "commands", "id", "fs", "securityStamp"} {
		if rec := env.put(aliceToken, alice.ID, `["`+field+`"]`, "alice-password-1", data); rec.Code != http.StatusForbidden {
			t.Errorf("a user changing their own %s = %d; want 403", field, rec.Code)
		}
	}
	if u := env.stored(alice.ID); u.Locale != "tr" || u.Username != "alice" || u.Perm.Admin || u.LockPassword {
		t.Errorf("stored user after the requests: %+v", u)
	}

	lock := fmt.Sprintf(`{"id":%d,"lockPassword":true}`, alice.ID)
	if rec := env.put(rootToken, alice.ID, `["lockPassword"]`, "", lock); rec.Code != http.StatusBadRequest {
		t.Errorf("an admin locking a password without their current one = %d; want 400", rec.Code)
	}
	if rec := env.put(rootToken, alice.ID, `["lockPassword"]`, "root-password-1", lock); rec.Code != http.StatusOK || !env.stored(alice.ID).LockPassword {
		t.Errorf("an admin locking a password = %d; want 200", rec.Code)
	}
	_ = root
}

func TestUsernamesAreUniqueInAnyCase(t *testing.T) {
	env := newSessionEnv(t)
	env.addUser("root", "root-password-1", true, false)
	alice := env.addUser("alice", "alice-password-1", false, false)
	token := env.login("root", "root-password-1")
	create := func(name string) int {
		body := fmt.Sprintf(`{"what":"user","which":[],"current_password":"root-password-1","data":{"username":%q,"password":"some-password-1","scope":"x","perm":{}}}`, name)
		return env.do(http.MethodPost, "/api/users", token, body, "").Code
	}

	for _, name := range []string{"alice", "Alice", "ALICE"} {
		if code := create(name); code != http.StatusConflict {
			t.Errorf("creating %q next to alice = %d; want 409", name, code)
		}
	}
	if code := create("bob"); code != http.StatusCreated {
		t.Fatalf("creating bob = %d; want 201", code)
	}

	rename := fmt.Sprintf(`{"id":%d,"username":"BOB"}`, alice.ID)
	if rec := env.put(token, alice.ID, `["username"]`, "root-password-1", rename); rec.Code != http.StatusConflict {
		t.Errorf("renaming alice to BOB = %d; want 409", rec.Code)
	}
	rename = fmt.Sprintf(`{"id":%d,"username":"Alice"}`, alice.ID)
	if rec := env.put(token, alice.ID, `["username"]`, "root-password-1", rename); rec.Code != http.StatusOK {
		t.Errorf("changing the case of one's own name = %d; want 200", rec.Code)
	}
	if err := env.st.Users.Save(&users.User{Username: "ROOT", Password: "x"}); !errors.Is(err, fberrors.ErrExist) {
		t.Errorf("the command line adding ROOT: %v; want ErrExist", err)
	}
}

func TestScopeChangeCreatesTheFolder(t *testing.T) {
	env := newSessionEnv(t)
	env.addUser("root", "root-password-1", true, false)
	alice := env.addUser("alice", "alice-password-1", false, false)
	token := env.login("root", "root-password-1")

	form := fmt.Sprintf(`{"id":%d,"username":"alice","scope":"teams/one","perm":{}}`, alice.ID)
	if rec := env.put(token, alice.ID, `["all"]`, "root-password-1", form); rec.Code != http.StatusOK {
		t.Fatalf("form save with a new scope = %d %q", rec.Code, rec.Body.String())
	}
	if st, err := os.Stat(filepath.Join(env.root, "teams", "one")); err != nil || !st.IsDir() {
		t.Errorf("the scope folder was not created: %v", err)
	}
	if got := env.stored(alice.ID).Scope; got != "/teams/one" {
		t.Errorf("stored scope %q; want /teams/one", got)
	}

	field := fmt.Sprintf(`{"id":%d,"scope":"../../escape/two"}`, alice.ID)
	if rec := env.put(token, alice.ID, `["scope"]`, "root-password-1", field); rec.Code != http.StatusOK {
		t.Fatalf("scope field save = %d %q", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(env.root, "escape", "two")); err != nil {
		t.Errorf("the scope folder was not created inside the root: %v", err)
	}
	if rec := env.do(http.MethodGet, "/api/resources/", env.login("alice", "alice-password-1"), "", ""); rec.Code != http.StatusOK {
		t.Errorf("listing the new scope = %d; want 200", rec.Code)
	}
}

// "Kendi klasörü" (K148): the user's own folder, in the folder of the users' folders, made if
// missing, also when new users get no folder of their own by default.
func TestOwnFolderScope(t *testing.T) {
	env := newSessionEnv(t)
	env.addUser("root", "root-password-1", true, false)
	alice := env.addUser("alice", "alice-password-1", false, false)
	token := env.login("root", "root-password-1")
	if err := env.st.Settings.Save(&settings.Settings{Key: sessionKey, AuthMethod: "json", MinimumPasswordLength: 8, UserHomeBasePath: "/users"}); err != nil {
		t.Fatal(err)
	}

	body := `{"what":"user","which":[],"current_password":"root-password-1","ownFolder":true,"data":{"username":"Bob Ross","password":"given-password-1","scope":".","perm":{}}}`
	if rec := env.do(http.MethodPost, "/api/users", token, body, ""); rec.Code != http.StatusCreated {
		t.Fatalf("creating a user with their own folder = %d %q; want 201", rec.Code, rec.Body.String())
	}
	bob, err := env.st.Users.Get(env.root, false, "Bob Ross")
	if err != nil {
		t.Fatal(err)
	}
	if bob.Scope != "/users/Bob-Ross" {
		t.Errorf("the new user's scope %q; want /users/Bob-Ross", bob.Scope)
	}

	form := fmt.Sprintf(`{"what":"user","which":["all"],"current_password":"root-password-1","ownFolder":true,"data":{"id":%d,"username":"alice","scope":".","perm":{}}}`, alice.ID)
	if rec := env.do(http.MethodPut, fmt.Sprintf("/api/users/%d", alice.ID), token, form, ""); rec.Code != http.StatusOK {
		t.Fatalf("giving alice her own folder = %d %q; want 200", rec.Code, rec.Body.String())
	}
	if got := env.stored(alice.ID).Scope; got != "/users/alice" {
		t.Errorf("alice's scope %q; want /users/alice", got)
	}
	for _, name := range []string{"Bob-Ross", "alice"} {
		if st, err := os.Stat(filepath.Join(env.root, "users", name)); err != nil || !st.IsDir() {
			t.Errorf("the folder of %s was not made: %v", name, err)
		}
	}
}

func TestDefaultLocaleIsTurkish(t *testing.T) {
	env := newSessionEnv(t)
	env.addUser("root", "root-password-1", true, false)
	token := env.login("root", "root-password-1")

	if rec := env.do(http.MethodPut, "/api/settings", token, `{"minimumPasswordLength":8,"defaults":{"locale":""}}`, ""); rec.Code != http.StatusOK {
		t.Fatalf("settings save = %d %q", rec.Code, rec.Body.String())
	}
	set, err := env.st.Settings.Get()
	if err != nil {
		t.Fatal(err)
	}
	if set.Defaults.Locale != "tr" {
		t.Errorf("default locale %q; want tr", set.Defaults.Locale)
	}
}
