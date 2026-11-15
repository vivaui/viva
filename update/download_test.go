package update

import (
	"context"
	"crypto/ed25519"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func downloadFixture(t *testing.T, content []byte) (*httptest.Server, *Client) {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write(content)
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

func testAsset(server *httptest.Server, content []byte) Asset {
	return Asset{
		OS:     "darwin",
		Arch:   "arm64",
		Format: FormatZIP,
		URL:    server.URL + "/downloads/app.zip",
		Size:   int64(len(content)),
		SHA256: testDigest(string(content)),
	}
}

func TestDownloadWritesExactlyTheSignedBytes(t *testing.T) {
	content := []byte("hello, update")
	server, client := downloadFixture(t, content)
	asset := testAsset(server, content)
	dir := t.TempDir()

	var progress [][2]int64
	path, err := client.Download(context.Background(), asset, dir, func(done, total int64) {
		progress = append(progress, [2]int64{done, total})
	})
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if filepath.Base(path) != "app.zip" {
		t.Errorf("file name = %q, want app.zip", path)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Errorf("file = %q, want %q", got, content)
	}
	if len(progress) < 2 {
		t.Fatalf("progress called %d times", len(progress))
	}
	if progress[0] != [2]int64{0, int64(len(content))} {
		t.Errorf("first progress = %v", progress[0])
	}
	last := progress[len(progress)-1]
	if last != [2]int64{int64(len(content)), int64(len(content))} {
		t.Errorf("last progress = %v", last)
	}
}

func TestDownloadRefusesAnotherDigest(t *testing.T) {
	content := []byte("hello, update")
	server, client := downloadFixture(t, content)
	asset := testAsset(server, content)
	asset.SHA256 = testDigest("something else")
	dir := t.TempDir()

	if _, err := client.Download(context.Background(), asset, dir, nil); !errors.Is(err, ErrChecksum) {
		t.Errorf("error %v does not wrap ErrChecksum", err)
	}
	assertEmptyDirectory(t, dir)
}

func TestDownloadRefusesAnotherLength(t *testing.T) {
	content := []byte("hello, update")
	server, client := downloadFixture(t, content)
	asset := testAsset(server, content)
	asset.Size++
	dir := t.TempDir()

	if _, err := client.Download(context.Background(), asset, dir, nil); !errors.Is(err, ErrSize) {
		t.Errorf("error %v does not wrap ErrSize", err)
	}
	assertEmptyDirectory(t, dir)
}

func TestDownloadRefusesAFileOverTheLimit(t *testing.T) {
	content := []byte("hello, update")
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write(content)
	}))
	t.Cleanup(server.Close)
	client, err := New(Config{
		FeedURL:       server.URL + "/latest.json",
		PublicKeys:    []ed25519.PublicKey{make(ed25519.PublicKey, ed25519.PublicKeySize)},
		Current:       "1.0.0",
		HTTP:          server.Client(),
		MaxAssetBytes: 4,
	})
	if err != nil {
		t.Fatal(err)
	}
	asset := testAsset(server, content)

	if _, err := client.Download(context.Background(), asset, t.TempDir(), nil); !errors.Is(err, ErrTooLarge) {
		t.Errorf("error %v does not wrap ErrTooLarge", err)
	}
}

func TestDownloadRefusesAnInsecureOrInvalidAsset(t *testing.T) {
	content := []byte("hello, update")
	server, client := downloadFixture(t, content)
	dir := t.TempDir()

	asset := testAsset(server, content)
	asset.URL = "http://example.com/app.zip"
	if _, err := client.Download(context.Background(), asset, dir, nil); !errors.Is(err, ErrInsecureURL) {
		t.Errorf("http URL: error %v does not wrap ErrInsecureURL", err)
	}

	asset = testAsset(server, content)
	asset.SHA256 = "not a digest"
	if _, err := client.Download(context.Background(), asset, dir, nil); !errors.Is(err, ErrManifest) {
		t.Errorf("bad digest: error %v does not wrap ErrManifest", err)
	}
	assertEmptyDirectory(t, dir)
}

func TestDownloadReportsAStatus(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.NotFound(writer, request)
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
	asset := Asset{
		OS:     "darwin",
		Arch:   "arm64",
		Format: FormatZIP,
		URL:    server.URL + "/missing.zip",
		Size:   1,
		SHA256: testDigest("x"),
	}
	if _, err := client.Download(context.Background(), asset, t.TempDir(), nil); err == nil {
		t.Fatal("Download accepted a 404")
	}
}

func TestAssetFileName(t *testing.T) {
	tests := map[string]string{
		"https://example.com/a/app-1.2.3.zip": "app-1.2.3.zip",
		"https://example.com/":                "update-archive",
		"https://example.com":                 "update-archive",
	}
	for raw, want := range tests {
		if got := assetFileName(Asset{URL: raw}); got != want {
			t.Errorf("assetFileName(%q) = %q, want %q", raw, got, want)
		}
	}
}

func assertEmptyDirectory(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("directory holds %d entries after a refused download", len(entries))
	}
}

func TestAssetFileNameRefusesAPathFromTheURL(t *testing.T) {
	for raw, want := range map[string]string{
		"https://example.com/a/app.zip":            "app.zip",
		"https://example.com/a%5Cb.zip":            "b.zip",
		"https://example.com/a/..%5C..%5Cevil.zip": "evil.zip",
		"https://example.com/a%3Ab.zip":            "update-archive",
	} {
		if got := assetFileName(Asset{URL: raw}); got != want {
			t.Errorf("assetFileName(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestDownloadPatchWritesTheVerifiedFile(t *testing.T) {
	content := []byte("a delta between two versions")
	server, client := downloadFixture(t, content)
	asset := testAsset(server, []byte("the full archive"))
	patch := Patch{
		FromVersion: "0.9.0", URL: server.URL + "/downloads/app.patch",
		Size: int64(len(content)), SHA256: testDigest(string(content)),
	}
	asset.Patches = []Patch{patch}
	dir := t.TempDir()
	path, err := client.DownloadPatch(context.Background(), asset, patch, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "app.zip.0.9.0.patch" {
		t.Errorf("file name = %q", filepath.Base(path))
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Errorf("patch = %q", got)
	}
}

func TestDownloadPatchRefusesTheWrongBytes(t *testing.T) {
	content := []byte("the patch's bytes")
	server, client := downloadFixture(t, content)
	asset := testAsset(server, []byte("the full archive"))
	patch := Patch{
		FromVersion: "0.9.0", URL: server.URL + "/app.patch",
		Size: int64(len(content)), SHA256: strings.Repeat("0", 64),
	}
	asset.Patches = []Patch{patch}
	dir := t.TempDir()
	if _, err := client.DownloadPatch(context.Background(), asset, patch, dir, nil); !errors.Is(err, ErrChecksum) {
		t.Fatalf("err = %v, want ErrChecksum", err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("a refused patch left %d files", len(entries))
	}
	patch.SHA256 = testDigest(string(content))
	patch.Size++
	if _, err := client.DownloadPatch(context.Background(), asset, patch, dir, nil); !errors.Is(err, ErrSize) {
		t.Fatalf("err = %v, want ErrSize", err)
	}
}

func TestDownloadPatchRefusesPlaintext(t *testing.T) {
	content := []byte("patch")
	server, client := downloadFixture(t, content)
	asset := testAsset(server, []byte("the full archive"))
	patch := Patch{
		FromVersion: "0.9.0", URL: "http://example.com/app.patch",
		Size: int64(len(content)), SHA256: testDigest(string(content)),
	}
	if _, err := client.DownloadPatch(context.Background(), asset, patch, t.TempDir(), nil); !errors.Is(err, ErrInsecureURL) {
		t.Fatalf("err = %v, want ErrInsecureURL", err)
	}
}
