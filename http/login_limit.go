package fbhttp

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"
	"sync"
	"time"
)

// Gezgin limits password logins: every attempt is spent from a budget of the client's address and
// from one of the address and username together. Attempts are counted before the password is
// checked, so that parallel requests cannot outrun the limit, and a successful login takes its
// attempt back and clears the account's budget.
const (
	loginAccountLimit = 5  // attempts per address and username within loginWindow
	loginAddressLimit = 20 // attempts per address within loginWindow
	loginWindow       = 15 * time.Minute
	loginSweepSize    = 1024 // budgets kept before the expired ones are swept
)

type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	now      func() time.Time
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{attempts: map[string][]time.Time{}, now: time.Now}
}

type loginBudget struct {
	key   string
	limit int
}

// loginBudgets names the address's budget first, then the account's. The username is hashed so
// that a long one cannot grow the map.
func loginBudgets(address, username string) [2]loginBudget {
	sum := sha256.Sum256([]byte(username))
	return [2]loginBudget{
		{"a\x00" + address, loginAddressLimit},
		{"u\x00" + address + "\x00" + hex.EncodeToString(sum[:]), loginAccountLimit},
	}
}

// loginAddress is the client's address as the connection shows it. Forwarded headers are not
// trusted: anyone can send them.
func loginAddress(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// recent drops the budget's attempts that left the window and returns the rest, oldest first.
func (l *loginLimiter) recent(key string, now time.Time) []time.Time {
	kept := l.attempts[key][:0]
	for _, at := range l.attempts[key] {
		if now.Sub(at) < loginWindow {
			kept = append(kept, at)
		}
	}
	if len(kept) == 0 {
		delete(l.attempts, key)
		return nil
	}
	l.attempts[key] = kept
	return kept
}

// begin counts an attempt and returns its time. When either budget is spent it counts nothing and
// returns how long until the next attempt is allowed.
func (l *loginLimiter) begin(address, username string) (time.Time, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	budgets := loginBudgets(address, username)
	var wait time.Duration
	for _, b := range budgets {
		if recent := l.recent(b.key, now); len(recent) >= b.limit {
			wait = max(wait, recent[len(recent)-b.limit].Add(loginWindow).Sub(now))
		}
	}
	if wait > 0 {
		return now, wait
	}

	if len(l.attempts) >= loginSweepSize {
		for key := range l.attempts {
			l.recent(key, now)
		}
	}
	for _, b := range budgets {
		l.attempts[b.key] = append(l.attempts[b.key], now)
	}
	return now, 0
}

// succeeded clears the account's budget and takes the successful attempt back from the address's.
func (l *loginLimiter) succeeded(address, username string, at time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()

	budgets := loginBudgets(address, username)
	delete(l.attempts, budgets[1].key)

	key := budgets[0].key
	list := l.attempts[key]
	for i := len(list) - 1; i >= 0; i-- {
		if list[i].Equal(at) {
			list = append(list[:i], list[i+1:]...)
			break
		}
	}
	if len(list) == 0 {
		delete(l.attempts, key)
	} else {
		l.attempts[key] = list
	}
}
