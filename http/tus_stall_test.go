package fbhttp

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// A chunk whose bytes stop coming ends after chunkStallTimeout (Gezgin, K169): what came is
// kept, a HEAD reports it, and the PATCH that resumes from there is not held up behind the
// stalled one.
func TestTusStalledChunkEnds(t *testing.T) {
	old := chunkStallTimeout
	chunkStallTimeout = 300 * time.Millisecond
	t.Cleanup(func() { chunkStallTimeout = old })

	const fileSize = 64 * 1024
	const sent = 1000

	f := newTusTestFixture(t)
	// Every request of this test is answered in time, or the stalled chunk still holds the upload.
	f.client.Timeout = 5 * time.Second
	payload := testPayload(fileSize)
	uploadURL := f.create(t, "stall.bin", fileSize)

	// The chunk announces the whole file, sends its first bytes and then nothing.
	body, feed := io.Pipe()
	t.Cleanup(func() { feed.Close() })
	go func() { _, _ = feed.Write(payload[:sent]) }()

	req, err := http.NewRequest(http.MethodPatch, uploadURL, body)
	if err != nil {
		t.Fatal(err)
	}
	req.ContentLength = fileSize
	req.Header.Set("X-Auth", f.token)
	req.Header.Set("Content-Type", "application/offset+octet-stream")
	req.Header.Set("Upload-Offset", "0")

	start := time.Now()
	res, err := f.client.Do(req)
	if err == nil {
		res.Body.Close()
		if res.StatusCode != http.StatusRequestTimeout {
			t.Fatalf("stalled PATCH: status %d, want %d", res.StatusCode, http.StatusRequestTimeout)
		}
	}
	// The answer may also be lost with the connection; either way the server ended the chunk.
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("stalled PATCH ended after %s", elapsed)
	}

	head, err := http.NewRequest(http.MethodHead, uploadURL, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	head.Header.Set("X-Auth", f.token)
	res, err = f.client.Do(head)
	if err != nil {
		t.Fatalf("HEAD: %v", err)
	}
	res.Body.Close()
	if got := res.Header.Get("Upload-Offset"); got != strconv.Itoa(sent) {
		t.Fatalf("HEAD after the stall: Upload-Offset = %q, want %d", got, sent)
	}

	res, _ = f.patch(t, uploadURL, sent, payload[sent:])
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("resumed PATCH: status %d", res.StatusCode)
	}

	got, err := os.ReadFile(filepath.Join(f.scope, "stall.bin"))
	if err != nil {
		t.Fatalf("read uploaded file: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("uploaded file differs from source (got %d bytes, want %d)", len(got), len(payload))
	}
}

// After a chunk that came in full, its read deadline does not stay on the connection (net/http
// sets the next request's own): a client that waits longer than chunkStallTimeout before the next
// chunk still finds it open.
func TestTusIdleConnectionOutlivesTheStallTimeout(t *testing.T) {
	old := chunkStallTimeout
	chunkStallTimeout = 200 * time.Millisecond
	t.Cleanup(func() { chunkStallTimeout = old })

	const chunkSize = 16 * 1024

	f := newTusTestFixture(t)
	payload := testPayload(chunkSize * 2)
	uploadURL := f.create(t, "idle.bin", len(payload))

	res, _ := f.patch(t, uploadURL, 0, payload[:chunkSize])
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("first PATCH: status %d", res.StatusCode)
	}

	time.Sleep(3 * chunkStallTimeout)

	res, reused := f.patch(t, uploadURL, chunkSize, payload[chunkSize:])
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("second PATCH: status %d", res.StatusCode)
	}
	if !reused {
		t.Fatal("the idle connection was closed before the second chunk")
	}
}
