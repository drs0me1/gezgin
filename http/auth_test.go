package fbhttp

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	fbAuth "github.com/filebrowser/filebrowser/v2/auth"
	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/users"
)

// Regression for GHSA-v3jv-rmh2-635j: proxy sign-in waived an expired token's expiry when a
// logout page was set. Gezgin has neither proxy sign-in (K61) nor a logout page setting (K74), and
// an expired token is refused.
func TestExpiredTokenIsRefused(t *testing.T) {
	key := []byte("test-signing-key")
	perm := users.Permissions{Download: true}
	st := scopedUserStorage(t, t.TempDir(), perm, key)

	if err := st.Settings.Save(&settings.Settings{
		Key:        key,
		AuthMethod: fbAuth.MethodJSONAuth,
	}); err != nil {
		t.Fatalf("failed to save settings: %v", err)
	}

	expired := &authToken{
		User: userInfo{ID: 1, Username: "u", Perm: perm},
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	}
	expiredToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, expired).SignedString(key)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	protected := withUser(func(w http.ResponseWriter, _ *http.Request, _ *data) (int, error) {
		_, writeErr := w.Write([]byte("protected"))
		return 0, writeErr
	})
	get := func(token string) int {
		req, _ := http.NewRequest(http.MethodGet, "/", http.NoBody)
		req.Header.Set("X-Auth", token)
		// The header a proxy used to set counts for nothing.
		req.Header.Set("X-Remote-User", "u")
		rec := httptest.NewRecorder()
		handle(protected, "", st, &settings.Server{}).ServeHTTP(rec, req)
		return rec.Code
	}

	if code := get(expiredToken); code != http.StatusUnauthorized {
		t.Errorf("expired token = %d; want 401", code)
	}
	if code := get(signToken(t, perm, key)); code != http.StatusOK {
		t.Errorf("valid token = %d; want 200", code)
	}
}
