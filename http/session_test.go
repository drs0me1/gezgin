package fbhttp

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/asdine/storm/v3"
	"github.com/golang-jwt/jwt/v5"

	fbAuth "github.com/filebrowser/filebrowser/v2/auth"
	"github.com/filebrowser/filebrowser/v2/diskcache"
	fberrors "github.com/filebrowser/filebrowser/v2/errors"
	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/storage"
	"github.com/filebrowser/filebrowser/v2/storage/bolt"
	"github.com/filebrowser/filebrowser/v2/users"
)

var sessionKey = []byte("test-signing-key")

// sessionEnv serves the whole API over a temporary database with JSON auth.
type sessionEnv struct {
	t        *testing.T
	st       *storage.Storage
	handler  http.Handler
	root     string
	uploads  *UploadCache
	archives *ArchiveJobs
}

func newSessionEnv(t *testing.T) *sessionEnv {
	t.Helper()
	db, err := storm.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	st, err := bolt.NewStorage(db)
	if err != nil {
		t.Fatalf("failed to get storage: %v", err)
	}
	if err := st.Settings.Save(&settings.Settings{
		Key:                   sessionKey,
		AuthMethod:            fbAuth.MethodJSONAuth,
		MinimumPasswordLength: 8,
	}); err != nil {
		t.Fatalf("failed to save settings: %v", err)
	}
	if err := st.Auth.Save(&fbAuth.JSONAuth{}); err != nil {
		t.Fatalf("failed to save auther: %v", err)
	}

	root := t.TempDir()
	uploads := newUploadCache(filepath.Join(root, UploadsDir), uploadCacheTTL)
	t.Cleanup(uploads.Close)
	archives := NewArchiveJobs(filepath.Join(root, ArchiveDir))
	t.Cleanup(archives.Close)
	handler, err := NewHandler(nil, diskcache.NewNoOp(), uploads, archives, st, &settings.Server{Root: root}, fstest.MapFS{})
	if err != nil {
		t.Fatalf("failed to build the handler: %v", err)
	}
	return &sessionEnv{t: t, st: st, handler: handler, root: root, uploads: uploads, archives: archives}
}

func (e *sessionEnv) addUser(name, password string, admin, mustChange bool) *users.User {
	e.t.Helper()
	hash, err := users.HashPwd(password)
	if err != nil {
		e.t.Fatal(err)
	}
	u := &users.User{
		Username:           name,
		Password:           hash,
		Scope:              ".",
		Perm:               users.Permissions{Admin: admin},
		MustChangePassword: mustChange,
	}
	if err := e.st.Users.Save(u); err != nil {
		e.t.Fatalf("failed to save %s: %v", name, err)
	}
	return u
}

func (e *sessionEnv) stored(id uint) *users.User {
	e.t.Helper()
	u, err := e.st.Users.Get("", false, id)
	if err != nil {
		e.t.Fatalf("failed to read user %d: %v", id, err)
	}
	return u
}

func (e *sessionEnv) do(method, path, token, body, remote string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if remote != "" {
		req.RemoteAddr = remote
	}
	if token != "" {
		req.Header.Set("X-Auth", token)
	}
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	return rec
}

func (e *sessionEnv) tryLogin(name, password, remote string) *httptest.ResponseRecorder {
	return e.do(http.MethodPost, "/api/login", "", fmt.Sprintf(`{"username":%q,"password":%q}`, name, password), remote)
}

func (e *sessionEnv) login(name, password string) string {
	e.t.Helper()
	rec := e.tryLogin(name, password, "")
	if rec.Code != http.StatusOK {
		e.t.Fatalf("login %s = %d %q; want 200", name, rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

// status is the answer to a request that needs a session: reading one's own user.
func (e *sessionEnv) status(token string, id uint) int {
	return e.do(http.MethodGet, fmt.Sprintf("/api/users/%d", id), token, "", "").Code
}

func (e *sessionEnv) put(token string, id uint, which, current, data string) *httptest.ResponseRecorder {
	body := fmt.Sprintf(`{"what":"user","which":%s,"current_password":%q,"data":%s}`, which, current, data)
	return e.do(http.MethodPut, fmt.Sprintf("/api/users/%d", id), token, body, "")
}

func (e *sessionEnv) changePassword(token string, id uint, current, next string) *httptest.ResponseRecorder {
	return e.put(token, id, `["password"]`, current, fmt.Sprintf(`{"id":%d,"password":%q}`, id, next))
}

func tokenClaims(t *testing.T, token string) *authToken {
	t.Helper()
	var tk authToken
	if _, err := jwt.ParseWithClaims(token, &tk, func(*jwt.Token) (interface{}, error) { return sessionKey, nil }); err != nil {
		t.Fatalf("failed to parse token: %v", err)
	}
	return &tk
}

// stamplessToken is a valid token as issued before security stamps existed.
func stamplessToken(t *testing.T, id uint) string {
	t.Helper()
	claims := &authToken{
		User: userInfo{ID: id},
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-time.Minute)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(sessionKey)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func TestRemovedAuthMethodsAreRefused(t *testing.T) {
	env := newSessionEnv(t)
	for _, method := range []settings.AuthMethod{"noauth", "hook"} {
		if _, err := env.st.Auth.Get(method); !errors.Is(err, fberrors.ErrInvalidAuthMethod) {
			t.Errorf("auth method %s: err = %v; want ErrInvalidAuthMethod", method, err)
		}
	}
}

func TestFirstLoginStoresAStampThatEndsOlderTokens(t *testing.T) {
	env := newSessionEnv(t)
	alice := env.addUser("alice", "alice-password-1", false, false)
	legacy := stamplessToken(t, alice.ID)

	if code := env.status(legacy, alice.ID); code != http.StatusOK {
		t.Fatalf("a token from before stamps, for a user without one = %d; want 200", code)
	}

	token := env.login("alice", "alice-password-1")
	stamp := env.stored(alice.ID).SecurityStamp
	if stamp == "" || tokenClaims(t, token).Stamp != stamp {
		t.Fatalf("the login stored stamp %q and issued %q; want the same, non-empty", stamp, tokenClaims(t, token).Stamp)
	}
	if code := env.status(token, alice.ID); code != http.StatusOK {
		t.Errorf("the new token = %d; want 200", code)
	}
	if code := env.status(legacy, alice.ID); code != http.StatusUnauthorized {
		t.Errorf("the stampless token after the first login = %d; want 401", code)
	}
	if again := env.login("alice", "alice-password-1"); tokenClaims(t, again).Stamp != stamp {
		t.Errorf("a second login changed the stamp")
	}
}

func TestPasswordChangeEndsEverySession(t *testing.T) {
	env := newSessionEnv(t)
	alice := env.addUser("alice", "alice-password-1", false, false)
	laptop := env.login("alice", "alice-password-1")
	phone := env.login("alice", "alice-password-1")

	if rec := env.changePassword(laptop, alice.ID, "alice-password-1", "alice-password-2"); rec.Code != http.StatusOK {
		t.Fatalf("password change = %d %q; want 200", rec.Code, rec.Body.String())
	}
	for name, token := range map[string]string{"laptop": laptop, "phone": phone} {
		if code := env.status(token, alice.ID); code != http.StatusUnauthorized {
			t.Errorf("%s token after the password change = %d; want 401", name, code)
		}
	}
	if code := env.status(env.login("alice", "alice-password-2"), alice.ID); code != http.StatusOK {
		t.Errorf("a login with the new password = %d; want 200", code)
	}
}

func TestClosingSessions(t *testing.T) {
	env := newSessionEnv(t)
	root := env.addUser("root", "root-password-1", true, false)
	alice := env.addUser("alice", "alice-password-1", false, false)
	closeSessions := func(token string, id uint) int {
		return env.do(http.MethodDelete, fmt.Sprintf("/api/users/%d/sessions", id), token, "", "").Code
	}

	own := env.login("alice", "alice-password-1")
	if code := closeSessions(own, alice.ID); code != http.StatusOK {
		t.Fatalf("closing one's own sessions = %d; want 200", code)
	}
	if code := env.status(own, alice.ID); code != http.StatusUnauthorized {
		t.Errorf("the token after closing the sessions = %d; want 401", code)
	}

	aliceToken := env.login("alice", "alice-password-1")
	rootToken := env.login("root", "root-password-1")
	if code := closeSessions(aliceToken, root.ID); code != http.StatusForbidden {
		t.Errorf("a user closing another user's sessions = %d; want 403", code)
	}
	if code := env.status(rootToken, root.ID); code != http.StatusOK {
		t.Errorf("the admin's token after the refused request = %d; want 200", code)
	}
	if code := closeSessions(rootToken, alice.ID); code != http.StatusOK {
		t.Errorf("an admin closing a user's sessions = %d; want 200", code)
	}
	if code := env.status(aliceToken, alice.ID); code != http.StatusUnauthorized {
		t.Errorf("the user's token after the admin closed the sessions = %d; want 401", code)
	}
	if code := closeSessions(rootToken, 99); code != http.StatusNotFound {
		t.Errorf("closing the sessions of a missing user = %d; want 404", code)
	}
}

func TestStampAndForcedChangeAreNotSetByRequests(t *testing.T) {
	env := newSessionEnv(t)
	root := env.addUser("root", "root-password-1", true, false)
	alice := env.addUser("alice", "alice-password-1", false, false)
	rootToken := env.login("root", "root-password-1")
	aliceToken := env.login("alice", "alice-password-1")
	stamp := env.stored(alice.ID).SecurityStamp

	for _, field := range []string{"securityStamp", "mustChangePassword"} {
		data := fmt.Sprintf(`{"id":%d,"securityStamp":"chosen","mustChangePassword":true}`, alice.ID)
		if rec := env.put(rootToken, alice.ID, `["`+field+`"]`, "root-password-1", data); rec.Code != http.StatusForbidden {
			t.Errorf("setting %s = %d; want 403", field, rec.Code)
		}
	}

	read := env.do(http.MethodGet, fmt.Sprintf("/api/users/%d", alice.ID), rootToken, "", "")
	if strings.Contains(read.Body.String(), stamp) {
		t.Errorf("the user API shows the security stamp: %s", read.Body.String())
	}
	if list := env.do(http.MethodGet, "/api/users", rootToken, "", ""); strings.Contains(list.Body.String(), stamp) {
		t.Errorf("the user list shows the security stamp")
	}

	// The admin form saves every field: without a password it keeps the sessions...
	form := fmt.Sprintf(`{"id":%d,"username":"alice","scope":".","securityStamp":"","mustChangePassword":true,"perm":{}}`, alice.ID)
	if rec := env.put(rootToken, alice.ID, `["all"]`, "root-password-1", form); rec.Code != http.StatusOK {
		t.Fatalf("admin form save = %d %q; want 200", rec.Code, rec.Body.String())
	}
	if u := env.stored(alice.ID); u.SecurityStamp != stamp || u.MustChangePassword {
		t.Errorf("the form save changed the stamp or the forced change: %q %t", u.SecurityStamp, u.MustChangePassword)
	}
	if code := env.status(aliceToken, alice.ID); code != http.StatusOK {
		t.Errorf("the user's token after a form save without a password = %d; want 200", code)
	}

	// ...and with one it ends them.
	form = fmt.Sprintf(`{"id":%d,"username":"alice","scope":".","password":"alice-password-9","perm":{}}`, alice.ID)
	if rec := env.put(rootToken, alice.ID, `["all"]`, "root-password-1", form); rec.Code != http.StatusOK {
		t.Fatalf("admin form save with a password = %d %q; want 200", rec.Code, rec.Body.String())
	}
	if code := env.status(aliceToken, alice.ID); code != http.StatusUnauthorized {
		t.Errorf("the user's token after the admin set a password = %d; want 401", code)
	}
	if code := env.status(rootToken, root.ID); code != http.StatusOK {
		t.Errorf("the admin's own token after changing another user's password = %d; want 200", code)
	}
}

func TestDeletedUsersTokensAreUnauthorized(t *testing.T) {
	env := newSessionEnv(t)
	env.addUser("root", "root-password-1", true, false)
	alice := env.addUser("alice", "alice-password-1", false, false)
	aliceToken := env.login("alice", "alice-password-1")
	rootToken := env.login("root", "root-password-1")

	del := env.do(http.MethodDelete, fmt.Sprintf("/api/users/%d", alice.ID), rootToken, `{"current_password":"root-password-1"}`, "")
	if del.Code != http.StatusOK {
		t.Fatalf("delete = %d %q; want 200", del.Code, del.Body.String())
	}
	if code := env.status(aliceToken, alice.ID); code != http.StatusUnauthorized {
		t.Errorf("a deleted user's token = %d; want 401", code)
	}
}

func TestForcedPasswordChange(t *testing.T) {
	env := newSessionEnv(t)
	root := env.addUser("admin", "generated-password-1", true, true)
	token := env.login("admin", "generated-password-1")

	if !tokenClaims(t, token).User.MustChangePassword {
		t.Fatalf("the token does not tell the interface to ask for a new password")
	}
	for _, path := range []string{"/api/users", "/api/resources/", "/api/settings"} {
		if rec := env.do(http.MethodGet, path, token, "", ""); rec.Code != http.StatusForbidden {
			t.Errorf("GET %s before the change = %d; want 403", path, rec.Code)
		}
	}
	if rec := env.do(http.MethodPost, "/api/renew", token, "", ""); rec.Code != http.StatusOK {
		t.Errorf("renewing before the change = %d; want 200", rec.Code)
	}

	data := fmt.Sprintf(`{"id":%d,"locale":"tr"}`, root.ID)
	if rec := env.put(token, root.ID, `["locale"]`, "generated-password-1", data); rec.Code != http.StatusForbidden {
		t.Errorf("another change before the password = %d; want 403", rec.Code)
	}
	if rec := env.changePassword(token, root.ID, "wrong-password-1", "chosen-password-1"); rec.Code != http.StatusBadRequest ||
		!strings.Contains(rec.Body.String(), fberrors.ErrCurrentPasswordIncorrect.Error()) {
		t.Errorf("a change with a wrong current password = %d %q; want 400 current password", rec.Code, rec.Body.String())
	}
	if rec := env.changePassword(token, root.ID, "generated-password-1", "generated-password-1"); rec.Code != http.StatusBadRequest ||
		!strings.Contains(rec.Body.String(), fberrors.ErrPasswordUnchanged.Error()) {
		t.Errorf("keeping the logged password = %d %q; want 400 unchanged", rec.Code, rec.Body.String())
	}

	if rec := env.changePassword(token, root.ID, "generated-password-1", "chosen-password-1"); rec.Code != http.StatusOK {
		t.Fatalf("the change = %d %q; want 200", rec.Code, rec.Body.String())
	}
	if env.stored(root.ID).MustChangePassword {
		t.Errorf("the change left the user held")
	}
	if code := env.status(token, root.ID); code != http.StatusUnauthorized {
		t.Errorf("the token from the logged password after the change = %d; want 401", code)
	}

	fresh := env.login("admin", "chosen-password-1")
	if tokenClaims(t, fresh).User.MustChangePassword {
		t.Errorf("the new token still asks for a new password")
	}
	if rec := env.do(http.MethodGet, "/api/users", fresh, "", ""); rec.Code != http.StatusOK {
		t.Errorf("GET /api/users after the change = %d; want 200", rec.Code)
	}
}

func TestAdminSettingAnotherUsersPasswordKeepsTheForcedChange(t *testing.T) {
	env := newSessionEnv(t)
	env.addUser("root", "root-password-1", true, false)
	held := env.addUser("held", "held-password-1", false, true)
	rootToken := env.login("root", "root-password-1")

	if rec := env.changePassword(rootToken, held.ID, "root-password-1", "held-password-2"); rec.Code != http.StatusOK {
		t.Fatalf("admin setting a password = %d %q; want 200", rec.Code, rec.Body.String())
	}
	if !env.stored(held.ID).MustChangePassword {
		t.Errorf("a password the admin chose cleared the user's forced change")
	}
}

func TestLoginAttemptLimit(t *testing.T) {
	env := newSessionEnv(t)
	env.addUser("alice", "alice-password-1", false, false)
	env.addUser("bob", "bob-password-1", false, false)
	const attacker = "192.0.2.1:4000"

	for i := 0; i < loginAccountLimit; i++ {
		if rec := env.tryLogin("alice", "guess", attacker); rec.Code != http.StatusForbidden {
			t.Fatalf("wrong guess %d = %d; want 403", i+1, rec.Code)
		}
	}
	rec := env.tryLogin("alice", "alice-password-1", attacker)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("the right password after %d wrong ones = %d; want 429", loginAccountLimit, rec.Code)
	}
	if wait, err := strconv.Atoi(rec.Header().Get("Retry-After")); err != nil || wait < 1 || wait > int(loginWindow/time.Second) {
		t.Errorf("Retry-After = %q; want 1..%d seconds", rec.Header().Get("Retry-After"), int(loginWindow/time.Second))
	}

	if rec := env.tryLogin("bob", "bob-password-1", attacker); rec.Code != http.StatusOK {
		t.Errorf("another account from the same address = %d; want 200", rec.Code)
	}
	if rec := env.tryLogin("alice", "alice-password-1", "198.51.100.7:4000"); rec.Code != http.StatusOK {
		t.Errorf("the locked account from another address = %d; want 200", rec.Code)
	}

	// A success clears the account's budget.
	const user = "203.0.113.5:4000"
	for i := 0; i < loginAccountLimit-1; i++ {
		env.tryLogin("alice", "guess", user)
	}
	if rec := env.tryLogin("alice", "alice-password-1", user); rec.Code != http.StatusOK {
		t.Fatalf("the right password on the last attempt = %d; want 200", rec.Code)
	}
	for i := 0; i < loginAccountLimit-1; i++ {
		if rec := env.tryLogin("alice", "guess", user); rec.Code != http.StatusForbidden {
			t.Fatalf("wrong guess %d after a success = %d; want 403", i+1, rec.Code)
		}
	}

	// An address trying many usernames runs out of its own budget.
	const sprayer = "192.0.2.9:4000"
	for i := 0; i < loginAddressLimit; i++ {
		if rec := env.tryLogin(fmt.Sprintf("user%d", i), "guess", sprayer); rec.Code != http.StatusForbidden {
			t.Fatalf("spray %d = %d; want 403", i+1, rec.Code)
		}
	}
	if rec := env.tryLogin("bob", "bob-password-1", sprayer); rec.Code != http.StatusTooManyRequests {
		t.Errorf("a login after %d wrong usernames = %d; want 429", loginAddressLimit, rec.Code)
	}
}

func TestLoginLimiterWindow(t *testing.T) {
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	l := newLoginLimiter()
	l.now = func() time.Time { return now }

	// Attempts count before the password is checked, so parallel ones cannot exceed the budget.
	for i := 0; i < loginAccountLimit; i++ {
		if _, wait := l.begin("192.0.2.1", "alice"); wait != 0 {
			t.Fatalf("attempt %d waits %s; want none", i+1, wait)
		}
		now = now.Add(time.Minute)
	}
	_, wait := l.begin("192.0.2.1", "alice")
	if want := loginWindow - loginAccountLimit*time.Minute; wait != want {
		t.Fatalf("wait after the budget = %s; want %s", wait, want)
	}

	now = now.Add(wait)
	at, wait := l.begin("192.0.2.1", "alice")
	if wait != 0 {
		t.Fatalf("an attempt once the oldest left the window waits %s", wait)
	}
	l.succeeded("192.0.2.1", "alice", at)
	if n := len(l.attempts["a\x00192.0.2.1"]); n != loginAccountLimit-1 {
		t.Errorf("the address keeps %d attempts after the success; want %d", n, loginAccountLimit-1)
	}

	// Expired budgets are swept once the map is large.
	for i := 0; i < loginSweepSize; i++ {
		l.begin(fmt.Sprintf("198.51.100.%d", i), "x")
	}
	now = now.Add(loginWindow)
	l.begin("203.0.113.1", "y")
	if n := len(l.attempts); n != 2 {
		t.Errorf("%d budgets after the sweep; want the 2 of the last attempt", n)
	}
}

func TestEndingOwnSessionWithdrawsTheRenewal(t *testing.T) {
	env := newSessionEnv(t)
	alice := env.addUser("alice", "alice-password-1", false, false)
	stamp := tokenClaims(t, env.login("alice", "alice-password-1")).Stamp

	// A token close to its expiry is offered a renewal, which the request below makes useless.
	claims := &authToken{
		User:  userInfo{ID: alice.ID, Username: "alice"},
		Stamp: stamp,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-90 * time.Minute)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * time.Minute)),
		},
	}
	old, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(sessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if rec := env.do(http.MethodGet, fmt.Sprintf("/api/users/%d", alice.ID), old, "", ""); rec.Header().Get("X-Renew-Token") != "true" {
		t.Fatalf("an expiring token is not offered a renewal")
	}

	rec := env.changePassword(old, alice.ID, "alice-password-1", "alice-password-2")
	if rec.Code != http.StatusOK || rec.Header().Get("X-Renew-Token") != "false" {
		t.Errorf("own password change = %d, X-Renew-Token %q; want 200 and false", rec.Code, rec.Header().Get("X-Renew-Token"))
	}
}
