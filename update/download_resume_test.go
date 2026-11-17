package update

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// resumeServer answers each request with the handler for its number, counting
// from zero, and records the Range header every request carried.
type resumeServer struct {
	*httptest.Server
	mu     sync.Mutex
	ranges []string
}

func (server *resumeServer) requests() []string {
	server.mu.Lock()
	defer server.mu.Unlock()
	return append([]string(nil), server.ranges...)
}

func newResumeServer(t *testing.T, handlers ...func(http.ResponseWriter, *http.Request)) (*resumeServer, *Client) {
	t.Helper()
	server := &resumeServer{}
	server.Server = httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		server.mu.Lock()
		index := len(server.ranges)
		server.ranges = append(server.ranges, request.Header.Get("Range"))
		server.mu.Unlock()
		if index >= len(handlers) {
			t.Errorf("unexpected request %d", index+1)
			http.Error(writer, "unexpected", http.StatusInternalServerError)
			return
		}
		handlers[index](writer, request)
	}))
	t.Cleanup(server.Close)
	client, err := New(Config{
		FeedURL:    server.URL + "/latest.json",
		PublicKeys: []ed25519.PublicKey{make(ed25519.PublicKey, ed25519.PublicKeySize)},
		Current:    "1.0.0",
		HTTP:       server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return server, client
}

// cutAfter declares the whole file, sends its first n bytes and drops the
// connection, the way a network that fails mid-transfer does.
func cutAfter(content []byte, n int) func(http.ResponseWriter, *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Length", strconv.Itoa(len(content)))
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(content[:n])
		writer.(http.Flusher).Flush()
		panic(http.ErrAbortHandler)
	}
}

// serveRanges answers with the file, honouring a Range header as a release
// host does.
func serveRanges(content []byte) func(http.ResponseWriter, *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		http.ServeContent(writer, request, "app.zip", time.Time{}, bytes.NewReader(content))
	}
}

// serveWhole answers with the whole file and ignores any Range header.
func serveWhole(content []byte) func(http.ResponseWriter, *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Length", strconv.Itoa(len(content)))
		_, _ = writer.Write(content)
	}
}

func resumeContent() []byte {
	return bytes.Repeat([]byte("0123456789abcdef"), 4096) // 64 KiB
}

func assertFileHolds(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s holds %d bytes that are not the file's %d", filepath.Base(path), len(got), len(want))
	}
}

func assertOnlyEntry(t *testing.T, dir, name string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != name {
		var names []string
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Fatalf("directory holds %v, want only %s", names, name)
	}
}

func TestDownloadResumesAnInterruptedTransfer(t *testing.T) {
	content := resumeContent()
	half := len(content) / 2
	server, client := newResumeServer(t, cutAfter(content, half), serveRanges(content))
	asset := testAsset(server.Server, content)
	dir := t.TempDir()

	_, err := client.Download(context.Background(), asset, dir, nil)
	if !errors.Is(err, ErrInterrupted) {
		t.Fatalf("first Download: %v, want ErrInterrupted", err)
	}
	if errors.Is(err, context.Canceled) {
		t.Fatalf("a dropped connection reads as a cancel: %v", err)
	}
	partial := filepath.Join(dir, partialName(asset.SHA256))
	info, err := os.Stat(partial)
	if err != nil {
		t.Fatalf("the bytes that arrived were not kept: %v", err)
	}
	if info.Size() != int64(half) {
		t.Fatalf("kept %d bytes, want %d", info.Size(), half)
	}

	var first [2]int64
	calls := 0
	path, err := client.Download(context.Background(), asset, dir, func(done, total int64) {
		if calls == 0 {
			first = [2]int64{done, total}
		}
		calls++
	})
	if err != nil {
		t.Fatalf("second Download: %v", err)
	}
	if got := server.requests(); len(got) != 2 || got[0] != "" || got[1] != fmt.Sprintf("bytes=%d-", half) {
		t.Fatalf("Range headers = %q, want none then bytes=%d-", got, half)
	}
	if first != [2]int64{int64(half), int64(len(content))} {
		t.Errorf("a resumed download first reports %v, want what was kept", first)
	}
	assertFileHolds(t, path, content)
	assertOnlyEntry(t, dir, "app.zip")
}

func TestDownloadStartsOverWhenTheServerIgnoresTheRange(t *testing.T) {
	content := resumeContent()
	server, client := newResumeServer(t, cutAfter(content, 1000), serveWhole(content))
	asset := testAsset(server.Server, content)
	dir := t.TempDir()

	if _, err := client.Download(context.Background(), asset, dir, nil); !errors.Is(err, ErrInterrupted) {
		t.Fatalf("first Download: %v, want ErrInterrupted", err)
	}
	path, err := client.Download(context.Background(), asset, dir, nil)
	if err != nil {
		t.Fatalf("second Download: %v", err)
	}
	if got := server.requests(); len(got) != 2 || got[1] != "bytes=1000-" {
		t.Fatalf("Range headers = %q", got)
	}
	assertFileHolds(t, path, content)
	assertOnlyEntry(t, dir, "app.zip")
}

func TestDownloadStartsOverWhenTheRangeAnswerIsAnotherPart(t *testing.T) {
	content := resumeContent()
	wrongRange := func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Range", fmt.Sprintf("bytes 0-%d/%d", len(content)-1, len(content)))
		writer.Header().Set("Content-Length", strconv.Itoa(len(content)))
		writer.WriteHeader(http.StatusPartialContent)
		_, _ = writer.Write(content)
	}
	server, client := newResumeServer(t, cutAfter(content, 1000), wrongRange, serveWhole(content))
	asset := testAsset(server.Server, content)
	dir := t.TempDir()

	if _, err := client.Download(context.Background(), asset, dir, nil); !errors.Is(err, ErrInterrupted) {
		t.Fatalf("first Download: %v, want ErrInterrupted", err)
	}
	path, err := client.Download(context.Background(), asset, dir, nil)
	if err != nil {
		t.Fatalf("second Download: %v", err)
	}
	if got := server.requests(); len(got) != 3 || got[1] != "bytes=1000-" || got[2] != "" {
		t.Fatalf("Range headers = %q, want a range and then the whole file", got)
	}
	assertFileHolds(t, path, content)
}

func TestDownloadStartsOverWhenTheRangeIsRefused(t *testing.T) {
	content := resumeContent()
	refuse := func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", len(content)))
		writer.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
	}
	server, client := newResumeServer(t, cutAfter(content, 1000), refuse, serveWhole(content))
	asset := testAsset(server.Server, content)
	dir := t.TempDir()

	if _, err := client.Download(context.Background(), asset, dir, nil); !errors.Is(err, ErrInterrupted) {
		t.Fatalf("first Download: %v, want ErrInterrupted", err)
	}
	path, err := client.Download(context.Background(), asset, dir, nil)
	if err != nil {
		t.Fatalf("second Download: %v", err)
	}
	if got := server.requests(); len(got) != 3 || got[1] != "bytes=1000-" || got[2] != "" {
		t.Fatalf("Range headers = %q, want a range and then the whole file", got)
	}
	assertFileHolds(t, path, content)
	assertOnlyEntry(t, dir, "app.zip")
}

func TestDownloadStartsOverWhenKeptBytesAreNotTheFile(t *testing.T) {
	content := resumeContent()
	server, client := newResumeServer(t, serveRanges(content), serveRanges(content))
	asset := testAsset(server.Server, content)
	dir := t.TempDir()
	// What a crash can leave: a file of the right length so far, but zeros.
	stale := make([]byte, 5000)
	if err := os.WriteFile(filepath.Join(dir, partialName(asset.SHA256)), stale, 0o600); err != nil {
		t.Fatal(err)
	}

	path, err := client.Download(context.Background(), asset, dir, nil)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if got := server.requests(); len(got) != 2 || got[0] != "bytes=5000-" || got[1] != "" {
		t.Fatalf("Range headers = %q, want a resume and then the whole file", got)
	}
	assertFileHolds(t, path, content)
	assertOnlyEntry(t, dir, "app.zip")
}

func TestDownloadRefusesTheWholeFileOnceAfterAResumeFails(t *testing.T) {
	content := resumeContent()
	server, client := newResumeServer(t, serveRanges(content), serveRanges(content))
	asset := testAsset(server.Server, content)
	asset.SHA256 = testDigest("another file")
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, partialName(asset.SHA256)), content[:5000], 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := client.Download(context.Background(), asset, dir, nil); !errors.Is(err, ErrChecksum) {
		t.Fatalf("Download: %v, want ErrChecksum", err)
	}
	if got := server.requests(); len(got) != 2 {
		t.Fatalf("made %d requests, want the resume and one fresh attempt", len(got))
	}
	assertEmptyDirectory(t, dir)
}

func TestDownloadReusesAVerifiedFile(t *testing.T) {
	content := resumeContent()
	server, client := newResumeServer(t, serveWhole(content))
	asset := testAsset(server.Server, content)
	dir := t.TempDir()

	path, err := client.Download(context.Background(), asset, dir, nil)
	if err != nil {
		t.Fatalf("first Download: %v", err)
	}
	var last [2]int64
	again, err := client.Download(context.Background(), asset, dir, func(done, total int64) { last = [2]int64{done, total} })
	if err != nil {
		t.Fatalf("second Download: %v", err)
	}
	if again != path {
		t.Errorf("second Download returned %q, want %q", again, path)
	}
	if got := server.requests(); len(got) != 1 {
		t.Errorf("made %d requests, want the file on disk reused", len(got))
	}
	if last != [2]int64{int64(len(content)), int64(len(content))} {
		t.Errorf("progress = %v, want complete", last)
	}
}

func TestDownloadReplacesAFileThatIsNotTheRelease(t *testing.T) {
	content := resumeContent()
	server, client := newResumeServer(t, serveWhole(content))
	asset := testAsset(server.Server, content)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "app.zip"), []byte("an older release"), 0o644); err != nil {
		t.Fatal(err)
	}

	path, err := client.Download(context.Background(), asset, dir, nil)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if got := server.requests(); len(got) != 1 {
		t.Fatalf("made %d requests, want one", len(got))
	}
	assertFileHolds(t, path, content)
}

func TestDownloadEndsAStalledTransfer(t *testing.T) {
	content := resumeContent()
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	stall := func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Length", strconv.Itoa(len(content)))
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(content[:2000])
		writer.(http.Flusher).Flush()
		select {
		case <-request.Context().Done():
		case <-release:
		}
	}
	server, client := newResumeServer(t, stall, serveRanges(content))
	// Long enough that a loaded runner still delivers the first 2000 bytes before
	// the stall fires, short enough to end well inside the test's own deadline.
	client.stall = 2 * time.Second
	asset := testAsset(server.Server, content)
	dir := t.TempDir()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := client.Download(ctx, asset, dir, nil)
	if !errors.Is(err, ErrInterrupted) {
		t.Fatalf("stalled Download: %v, want ErrInterrupted", err)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("a stall reads as the caller's cancel or deadline: %v", err)
	}
	if ctx.Err() != nil {
		t.Fatal("the stall was not detected before the test's own deadline")
	}
	path, err := client.Download(ctx, asset, dir, nil)
	if err != nil {
		t.Fatalf("resumed Download: %v", err)
	}
	if got := server.requests(); len(got) != 2 || got[1] != "bytes=2000-" {
		t.Fatalf("Range headers = %q", got)
	}
	assertFileHolds(t, path, content)
}

func TestDownloadKeepsTheBytesWhenCancelled(t *testing.T) {
	content := resumeContent()
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	hold := func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Length", strconv.Itoa(len(content)))
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(content[:3000])
		writer.(http.Flusher).Flush()
		select {
		case <-request.Context().Done():
		case <-release:
		}
	}
	server, client := newResumeServer(t, hold, serveRanges(content))
	asset := testAsset(server.Server, content)
	dir := t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	_, err := client.Download(ctx, asset, dir, func(done, total int64) {
		if done >= 3000 {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled Download: %v, want context.Canceled", err)
	}
	path, err := client.Download(context.Background(), asset, dir, nil)
	if err != nil {
		t.Fatalf("resumed Download: %v", err)
	}
	if got := server.requests(); len(got) != 2 || got[1] != "bytes=3000-" {
		t.Fatalf("Range headers = %q", got)
	}
	assertFileHolds(t, path, content)
}

func TestDownloadPatchResumesAnInterruptedTransfer(t *testing.T) {
	content := resumeContent()
	server, client := newResumeServer(t, cutAfter(content, 4096), serveRanges(content))
	asset := testAsset(server.Server, []byte("the full archive"))
	patch := Patch{
		FromVersion: "0.9.0", URL: server.URL + "/downloads/app.patch",
		Size: int64(len(content)), SHA256: testDigest(string(content)),
	}
	asset.Patches = []Patch{patch}
	dir := t.TempDir()

	if _, err := client.DownloadPatch(context.Background(), asset, patch, dir, nil); !errors.Is(err, ErrInterrupted) {
		t.Fatalf("first DownloadPatch: %v, want ErrInterrupted", err)
	}
	path, err := client.DownloadPatch(context.Background(), asset, patch, dir, nil)
	if err != nil {
		t.Fatalf("second DownloadPatch: %v", err)
	}
	if got := server.requests(); len(got) != 2 || got[1] != "bytes=4096-" {
		t.Fatalf("Range headers = %q", got)
	}
	assertFileHolds(t, path, content)
}

func TestDownloadCallsAServerErrorAnInterruption(t *testing.T) {
	content := resumeContent()
	unavailable := func(writer http.ResponseWriter, request *http.Request) {
		http.Error(writer, "busy", http.StatusServiceUnavailable)
	}
	missing := func(writer http.ResponseWriter, request *http.Request) {
		http.NotFound(writer, request)
	}
	server, client := newResumeServer(t, unavailable, missing)
	asset := testAsset(server.Server, content)

	if _, err := client.Download(context.Background(), asset, t.TempDir(), nil); !errors.Is(err, ErrInterrupted) {
		t.Errorf("503: %v, want ErrInterrupted", err)
	}
	_, err := client.Download(context.Background(), asset, t.TempDir(), nil)
	if err == nil || errors.Is(err, ErrInterrupted) || !strings.Contains(err.Error(), "404") {
		t.Errorf("404: %v, want a status error that is not an interruption", err)
	}
}
