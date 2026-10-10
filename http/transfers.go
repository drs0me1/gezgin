package fbhttp

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/tomasen/realip"
)

// transferLimit is how many downloads, and how many uploads, may run at the same time for one
// user, one share link or one WebDAV share (Gezgin K64). Previews, thumbnails and subtitles are
// not counted.
const transferLimit = 10

// downloadEntries bounds the files and folders a download packs, as archive jobs do (K65): a
// download plans every entry in memory before it sends anything.
var downloadEntries = archiveEntries

// limitMessage is a limit's message in Turkish and in English: a download's tab shows it as it
// is, so the server writes it in the reader's language (Gezgin, K167).
type limitMessage struct{ tr, en string }

// For gives the message in a language: a user's locale, or a browser's Accept-Language. Turkish
// for Turkish, English for anything else.
func (m limitMessage) For(language string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(language)), "tr") {
		return m.tr
	}
	return m.en
}

var (
	tooManyDownloads = limitMessage{
		fmt.Sprintf("Aynı anda en fazla %d indirme yapılabilir; süren bir indirme bitince yeniden deneyin.", transferLimit),
		fmt.Sprintf("At most %d downloads can run at the same time; try again once one has ended.", transferLimit),
	}
	tooManyUploads = limitMessage{
		fmt.Sprintf("Aynı anda en fazla %d yükleme yapılabilir; süren bir yükleme bitince yeniden deneyin.", transferLimit),
		fmt.Sprintf("At most %d uploads can run at the same time; try again once one has ended.", transferLimit),
	}
	tooManyTransfers = limitMessage{
		fmt.Sprintf("Bu paylaşımda aynı anda en fazla %d aktarım yapılabilir; biri bitince yeniden deneyin.", transferLimit),
		fmt.Sprintf("This share runs at most %d transfers at the same time; try again once one has ended.", transferLimit),
	}
	tooManyEntries = limitMessage{
		fmt.Sprintf("Bu indirmede %d.%03d'den fazla dosya ve klasör var; daha küçük parçalar hâlinde indirin.",
			downloadEntries/1000, downloadEntries%1000),
		fmt.Sprintf("This download holds more than %d,%03d files and folders; download it in smaller parts.",
			downloadEntries/1000, downloadEntries%1000),
	}
)

// transfers counts the transfers under way, by who makes them.
type transfers struct {
	mu    sync.Mutex
	count map[string]int
}

var running = &transfers{count: map[string]int{}}

// begin takes one of key's places and reports whether there was one free. A transfer that got a
// place gives it back with end.
func (t *transfers) begin(key string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.count[key] >= transferLimit {
		return false
	}
	t.count[key]++
	return true
}

func (t *transfers) end(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.count[key]--; t.count[key] <= 0 {
		delete(t.count, key)
	}
}

// The keys transfers are counted by.
func userDownloads(id uint) string     { return fmt.Sprintf("user %d downloads", id) }
func userUploads(id uint) string       { return fmt.Sprintf("user %d uploads", id) }
func linkDownloads(hash string) string { return "link " + hash }
func davTransfers(hash string) string  { return "webdav " + hash }

// tooMany refuses a transfer over the limit with 429 and message, which the browser tab a download
// opens shows as it is.
func tooMany(w http.ResponseWriter, r *http.Request, message string) (int, error) {
	log.Printf("%s: %d %s too many transfers", r.URL.Path, http.StatusTooManyRequests, realip.FromRequest(r))
	w.Header().Set("Retry-After", "5")
	http.Error(w, message, http.StatusTooManyRequests)
	return 0, nil
}
