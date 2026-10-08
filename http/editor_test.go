package fbhttp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/filebrowser/filebrowser/v2/files"
)

func sha(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func (e *fileEnv) raw(name string) []byte {
	e.t.Helper()
	data, err := os.ReadFile(filepath.Join(e.root, name))
	if err != nil {
		e.t.Fatal(err)
	}
	return data
}

func (e *fileEnv) openText(name string) files.FileInfo {
	e.t.Helper()
	rec := e.call(http.MethodGet, "/api/resources/"+name, "")
	var info files.FileInfo
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &info) != nil {
		e.t.Fatalf("open %s = %d %q", name, rec.Code, rec.Body.String())
	}
	return info
}

func save(e *fileEnv, name, text, encoding, version string) (int, string) {
	q := url.Values{}
	if encoding != "" {
		q.Set("encoding", encoding)
	}
	if version != "" {
		q.Set("version", version)
	}
	rec := e.call(http.MethodPut, "/api/resources/"+name+"?"+q.Encode(), text)
	return rec.Code, rec.Header().Get("X-Version")
}

func TestEditorKeepsTheTurkishCodePage(t *testing.T) {
	env := newFileEnv(t)
	original := []byte{0xFE, 'e', 'k', 'e', 'r', ' ', 0xFD, 0xF0, 'd', 0xFD, 'r', '\r', '\n'}
	env.write("tr.txt", string(original), 0o644)

	info := env.openText("tr.txt")
	if info.Type != "text" || info.Encoding != files.EncodingWindows1254 || info.Content != "şeker ığdır\r\n" || info.Version != sha(original) {
		t.Fatalf("opened as %q, %q, %q, %q", info.Type, info.Encoding, info.Content, info.Version)
	}

	code, version := save(env, "tr.txt", "şeker ığdır\r\nüç çiçek\r\n", info.Encoding, info.Version)
	want := append(append([]byte{}, original...), 0xFC, 0xE7, ' ', 0xE7, 'i', 0xE7, 'e', 'k', '\r', '\n')
	if code != http.StatusOK || string(env.raw("tr.txt")) != string(want) || version != sha(want) {
		t.Fatalf("save = %d, file % x, version %s; want the text in windows-1254", code, env.raw("tr.txt"), version)
	}

	if code, _ := save(env, "tr.txt", "gülümse \U0001F600\r\n", info.Encoding, version); code != http.StatusBadRequest {
		t.Errorf("an emoji in windows-1254 = %d; want 400", code)
	}
	if string(env.raw("tr.txt")) != string(want) {
		t.Errorf("a refused save changed the file")
	}
}

func TestEditorRefusesToOverwriteAnotherChange(t *testing.T) {
	env := newFileEnv(t)
	env.write("notes.txt", "first\n", 0o644)
	opened := env.openText("notes.txt")

	env.write("notes.txt", "someone else's\n", 0o644)
	if code, _ := save(env, "notes.txt", "mine\n", opened.Encoding, opened.Version); code != http.StatusConflict {
		t.Fatalf("a save over another change = %d; want 409", code)
	}
	if string(env.raw("notes.txt")) != "someone else's\n" {
		t.Errorf("the other change was overwritten")
	}

	code, version := save(env, "notes.txt", "mine\n", opened.Encoding, "")
	if code != http.StatusOK || string(env.raw("notes.txt")) != "mine\n" || version != sha([]byte("mine\n")) {
		t.Errorf("an overwrite asked for = %d, %q", code, env.raw("notes.txt"))
	}
	if code, _ := save(env, "notes.txt", "mine, again\n", opened.Encoding, version); code != http.StatusOK {
		t.Errorf("a second save with the returned version = %d; want 200", code)
	}
}

func TestUnknownEncodingsOpenReadOnly(t *testing.T) {
	env := newFileEnv(t)
	env.write("odd.txt", "a\x81b\n", 0o644)
	if info := env.openText("odd.txt"); info.Type != "textImmutable" || info.Encoding != "" {
		t.Errorf("a text no encoding reads back = %q, %q; want read-only, no encoding", info.Type, info.Encoding)
	}
}

func TestTurkishSubtitlesReachThePlayerAsUTF8(t *testing.T) {
	env := newFileEnv(t)
	srt := "1\r\n00:00:01,000 --> 00:00:02,000\r\n\xFEeker \xFD\xF0d\xFDr\r\n\r\n"
	env.write("film.srt", srt, 0o644)
	env.write("film.vtt", "WEBVTT\n\n00:01.000 --> 00:02.000\n\xE7i\xE7ek\n", 0o644)

	for name, want := range map[string]string{"film.srt": "şeker ığdır", "film.vtt": "çiçek"} {
		rec := env.call(http.MethodGet, "/api/subtitle/"+name, "")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), want) {
			t.Errorf("%s = %d %q; want %q in UTF-8", name, rec.Code, rec.Body.String(), want)
		}
	}
}
