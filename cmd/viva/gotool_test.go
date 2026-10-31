package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseGoVersion(t *testing.T) {
	for _, test := range []struct {
		in           string
		minor        int
		ok, atLeast  bool
		atLeastMinor int
	}{
		{"go1.26.1", 26, true, true, 26},
		{"go1.26", 26, true, true, 26},
		{"go1.27rc1", 27, true, true, 26},
		{"devel go1.27-abcdef", 27, true, true, 26},
		{"go1.24.5\n", 24, true, false, 26},
		{"go2.0", 0, true, true, 26},
		{"", 0, false, false, 26},
		{"nonsense", 0, false, false, 26},
	} {
		_, minor, ok := parseGoVersion(test.in)
		if ok != test.ok || ok && test.in != "go2.0" && minor != test.minor {
			t.Errorf("parseGoVersion(%q) = %d, %v", test.in, minor, ok)
		}
		if got := goVersionAtLeast(test.in, test.atLeastMinor); got != test.atLeast {
			t.Errorf("goVersionAtLeast(%q, %d) = %v, want %v", test.in, test.atLeastMinor, got, test.atLeast)
		}
	}
}

// The minimum Go viva fetches must be the one the module declares, or viva would
// hand out a toolchain that cannot build the library.
func TestMinGoMinorMatchesGoMod(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("\ngo 1.%d\n", minGoMinor)
	if !strings.Contains(string(data), want) {
		t.Errorf("go.mod does not declare %q; update minGoMinor", strings.TrimSpace(want))
	}
}

type fakeFile struct {
	name string
	data string
	mode int64
	link string
}

func tarGzOf(t *testing.T, files ...fakeFile) []byte {
	t.Helper()
	var buffer bytes.Buffer
	compressed := gzip.NewWriter(&buffer)
	archive := tar.NewWriter(compressed)
	for _, file := range files {
		header := &tar.Header{Name: file.name, Mode: file.mode, Size: int64(len(file.data)), Typeflag: tar.TypeReg}
		if file.link != "" {
			header = &tar.Header{Name: file.name, Typeflag: tar.TypeSymlink, Linkname: file.link}
		}
		if err := archive.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if file.link == "" {
			if _, err := archive.Write([]byte(file.data)); err != nil {
				t.Fatal(err)
			}
		}
	}
	archive.Close()
	compressed.Close()
	return buffer.Bytes()
}

func zipOf(t *testing.T, files ...fakeFile) []byte {
	t.Helper()
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	for _, file := range files {
		writer, err := archive.Create(file.name)
		if err != nil {
			t.Fatal(err)
		}
		writer.Write([]byte(file.data))
	}
	archive.Close()
	return buffer.Bytes()
}

// goMirror serves a go.dev-style index and the archives it lists.
func goMirror(t *testing.T, goos, goarch string, archives map[string][]byte, releases ...goRelease) goSource {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/dl/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/dl/")
		if name == "" {
			json.NewEncoder(w).Encode(releases)
			return
		}
		data, ok := archives[name]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(data)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return goSource{base: server.URL + "/dl/", goos: goos, goarch: goarch}
}

func sumOf(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func TestInstallDownloadsVerifiesAndUnpacksTheNewestStableToolchain(t *testing.T) {
	good := tarGzOf(t,
		fakeFile{name: "go/VERSION", data: "go1.99.2", mode: 0o644},
		fakeFile{name: "go/bin/" + goExecutable(), data: "#!/bin/sh\n", mode: 0o755},
		fakeFile{name: "go/lib/link", link: "../VERSION"})
	old := tarGzOf(t, fakeFile{name: "go/bin/" + goExecutable(), data: "old", mode: 0o755})
	filename := fmt.Sprintf("go1.99.2.%s-%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	source := goMirror(t, runtime.GOOS, runtime.GOARCH,
		map[string][]byte{filename: good, "go1.24.9.old.tar.gz": old},
		goRelease{Version: "go1.99rc1", Stable: false, Files: []goFile{{Filename: "never", OS: runtime.GOOS, Arch: runtime.GOARCH, Kind: "archive", SHA256: "00"}}},
		goRelease{Version: "go1.24.9", Stable: true, Files: []goFile{{Filename: "go1.24.9.old.tar.gz", OS: runtime.GOOS, Arch: runtime.GOARCH, Kind: "archive", SHA256: sumOf(old)}}},
		goRelease{Version: "go1.99.2", Stable: true, Files: []goFile{
			{Filename: "go1.99.2.src.tar.gz", Kind: "source", SHA256: "11"},
			{Filename: filename, OS: runtime.GOOS, Arch: runtime.GOARCH, Kind: "archive", SHA256: sumOf(good), Size: int64(len(good))},
		}},
	)
	cache := t.TempDir()
	var notice bytes.Buffer
	root, err := source.install(cache, &notice)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(cache, "go1.99.2"); root != want {
		t.Errorf("installed into %s, want %s", root, want)
	}
	if _, err := os.Stat(filepath.Join(root, "bin", goExecutable())); err != nil {
		t.Errorf("the toolchain has no go binary: %v", err)
	}
	info, err := os.Stat(filepath.Join(root, "bin", goExecutable()))
	if err == nil && runtime.GOOS != "windows" && info.Mode()&0o111 == 0 {
		t.Error("go lost its executable bit")
	}
	if !strings.Contains(notice.String(), "go1.99.2") {
		t.Errorf("the user was not told what is downloaded:\n%s", notice.String())
	}
	if entries, _ := os.ReadDir(cache); len(entries) != 1 {
		t.Errorf("download scratch files were left in the cache: %v", entries)
	}
	if found := cachedToolchain(cache); found != root {
		t.Errorf("cachedToolchain = %q, want %q", found, root)
	}

	// A second install finds the first and downloads nothing.
	notice.Reset()
	if _, err := source.install(cache, &notice); err != nil || notice.Len() != 0 {
		t.Errorf("reinstall: %v, %q", err, notice.String())
	}
}

func TestInstallRefusesAnArchiveThatDoesNotMatchItsPublishedDigest(t *testing.T) {
	archive := tarGzOf(t, fakeFile{name: "go/bin/go", data: "evil", mode: 0o755})
	filename := fmt.Sprintf("go1.99.2.%s-%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	source := goMirror(t, runtime.GOOS, runtime.GOARCH, map[string][]byte{filename: archive},
		goRelease{Version: "go1.99.2", Stable: true, Files: []goFile{
			{Filename: filename, OS: runtime.GOOS, Arch: runtime.GOARCH, Kind: "archive", SHA256: sumOf([]byte("something else"))}}})
	cache := t.TempDir()
	_, err := source.install(cache, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("install accepted a tampered archive: %v", err)
	}
	if entries, _ := os.ReadDir(cache); len(entries) != 0 {
		t.Errorf("a rejected download left %v behind", entries)
	}
}

func TestInstallRefusesEntriesOutsideTheDirectory(t *testing.T) {
	archive := tarGzOf(t, fakeFile{name: "go/bin/go", data: "x", mode: 0o755}, fakeFile{name: "../escape", data: "x", mode: 0o644})
	filename := fmt.Sprintf("go1.99.2.%s-%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	source := goMirror(t, runtime.GOOS, runtime.GOARCH, map[string][]byte{filename: archive},
		goRelease{Version: "go1.99.2", Stable: true, Files: []goFile{
			{Filename: filename, OS: runtime.GOOS, Arch: runtime.GOARCH, Kind: "archive", SHA256: sumOf(archive)}}})
	cache := filepath.Join(t.TempDir(), "cache")
	if _, err := source.install(cache, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "leaves") {
		t.Fatalf("install followed a path out of the cache: %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(cache), "escape")); err == nil {
		t.Error("a file was written outside the cache")
	}
}

func TestInstallUnpacksTheWindowsZip(t *testing.T) {
	archive := zipOf(t, fakeFile{name: "go/bin/go.exe", data: "MZ"}, fakeFile{name: "go/VERSION", data: "go1.99.2"})
	source := goMirror(t, "windows", "amd64", map[string][]byte{"go1.99.2.windows-amd64.zip": archive},
		goRelease{Version: "go1.99.2", Stable: true, Files: []goFile{
			{Filename: "go1.99.2.windows-amd64.zip", OS: "windows", Arch: "amd64", Kind: "archive", SHA256: sumOf(archive)}}})
	root, err := source.install(t.TempDir(), &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "bin", "go.exe")); err != nil {
		t.Error(err)
	}
}

func TestInstallReportsAPlatformTheIndexDoesNotCover(t *testing.T) {
	source := goMirror(t, "plan9", "mips", nil,
		goRelease{Version: "go1.99.2", Stable: true, Files: []goFile{{Filename: "x", OS: "linux", Arch: "amd64", Kind: "archive", SHA256: "aa"}}})
	if _, err := source.install(t.TempDir(), &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "plan9/mips") {
		t.Fatalf("got %v", err)
	}
}

func TestCachedToolchainPrefersTheNewestCompleteInstall(t *testing.T) {
	cache := t.TempDir()
	for _, name := range []string{"go1.26.1", "go1.26.10", "go1.26.2", "go1.20.0"} {
		if err := os.MkdirAll(filepath.Join(cache, name, "bin"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(cache, name, "bin", goExecutable()), nil, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// An interrupted install: a directory with no go binary in it.
	os.MkdirAll(filepath.Join(cache, "go1.99.0"), 0o755)
	if got := filepath.Base(cachedToolchain(cache)); got != "go1.26.10" {
		t.Errorf("picked %s", got)
	}
	if cachedToolchain(filepath.Join(cache, "missing")) != "" {
		t.Error("found a toolchain in a directory that is not there")
	}
}
