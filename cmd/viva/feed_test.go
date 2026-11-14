package main

import (
	"archive/zip"
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/update"
)

const feedTestBasicTOML = `[app]
name = "Fixture Notes"
id = "dev.example.notes"
version = "2.0.1"
`

// feedTestTOML points the default archive directory somewhere other than
// build, so the feed proves that it follows [build] out.
const feedTestTOML = `[app]
name = "Fixture Notes"
id = "dev.example.notes"
version = "2.0.1"

[build]
out = "dist"
`

func writeFeedFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// feedTestKey writes a signing key the way viva update-keygen does and returns
// its path together with the public key a feed reader would trust.
func feedTestKey(t *testing.T, dir string) (string, ed25519.PublicKey) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "update.key")
	writeFeedFile(t, path, base64.StdEncoding.EncodeToString(private)+"\n")
	return path, public
}

type feedTestAssetKey struct {
	goos, arch, format string
}

func TestFeedWritesASignedFeed(t *testing.T) {
	t.Setenv("VIVA_UPDATE_KEY", "")
	dir := t.TempDir()
	writeFeedFile(t, filepath.Join(dir, "viva.toml"), feedTestTOML)
	dist := filepath.Join(dir, "dist")

	type expectedAsset struct {
		name, content, url string
	}
	expected := map[feedTestAssetKey]expectedAsset{
		{"darwin", "amd64", update.FormatZIP}: {
			"fixture-notes-2.0.1-darwin-amd64.zip", "darwin amd64",
			"https://cdn.example.com/apps/fixture-notes-2.0.1-darwin-amd64.zip",
		},
		{"darwin", "arm64", update.FormatZIP}: {
			"fixture notes-2.0.1-darwin-arm64.zip", "darwin arm64",
			"https://cdn.example.com/apps/fixture%20notes-2.0.1-darwin-arm64.zip",
		},
		{"windows", "amd64", update.FormatNSIS}: {
			"fixture-notes-2.0.1-windows-amd64-setup.exe", "windows amd64 installer",
			"https://cdn.example.com/apps/fixture-notes-2.0.1-windows-amd64-setup.exe",
		},
		{"windows", "arm64", update.FormatNSIS}: {
			"fixture-notes-2.0.1-windows-arm64-setup.exe", "windows arm64 installer",
			"https://cdn.example.com/apps/fixture-notes-2.0.1-windows-arm64-setup.exe",
		},
		{"windows", "arm64", update.FormatZIP}: {
			"fixture-notes-2.0.1-windows-arm64.zip", "windows arm64 zip",
			"https://cdn.example.com/apps/fixture-notes-2.0.1-windows-arm64.zip",
		},
		{"linux", "arm64", update.FormatTarGz}: {
			"fixture-notes-2.0.1-linux-arm64.tar.gz", "linux arm64",
			"https://cdn.example.com/apps/fixture-notes-2.0.1-linux-arm64.tar.gz",
		},
	}
	for _, asset := range expected {
		writeFeedFile(t, filepath.Join(dist, asset.name), asset.content)
	}
	// None of these may become an asset: another architecture, a name of no
	// known shape and a directory that only looks like an archive.
	writeFeedFile(t, filepath.Join(dist, "fixture-notes-2.0.1-darwin-riscv64.zip"), "unknown architecture")
	writeFeedFile(t, filepath.Join(dist, "notes.txt"), "release notes")
	if err := os.MkdirAll(filepath.Join(dist, "fixture-notes-2.0.1-linux-amd64.tar.gz"), 0o755); err != nil {
		t.Fatal(err)
	}
	keyPath, public := feedTestKey(t, dir)

	var output bytes.Buffer
	err := runFeed([]string{
		"-C", dir,
		"-base-url", "https://cdn.example.com/apps/",
		"-channel", "beta",
		"-valid-for", "48h",
		"-notes", "https://example.com/notes/2.0.1",
		"-key", keyPath,
	}, &output, io.Discard)
	if err != nil {
		t.Fatalf("%v\n%s", err, output.String())
	}

	out := filepath.Join(dist, "latest.json")
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var feed update.Feed
	if err := json.Unmarshal(raw, &feed); err != nil {
		t.Fatalf("latest.json: %v\n%s", err, raw)
	}
	if feed.Schema != update.SchemaVersion {
		t.Errorf("schema = %d, want %d", feed.Schema, update.SchemaVersion)
	}
	if feed.App != "dev.example.notes" || feed.Latest.Version != "2.0.1" {
		t.Errorf("feed is for %q %q", feed.App, feed.Latest.Version)
	}
	if feed.Channel != "beta" {
		t.Errorf("channel = %q, want beta", feed.Channel)
	}
	if feed.Latest.NotesURL != "https://example.com/notes/2.0.1" {
		t.Errorf("notes_url = %q", feed.Latest.NotesURL)
	}
	if feed.Latest.Date.IsZero() || feed.Latest.Date.Location() != time.UTC || time.Since(feed.Latest.Date) > time.Minute {
		t.Errorf("date = %v, want a recent UTC time", feed.Latest.Date)
	}
	if want := feed.Latest.Date.Add(48 * time.Hour); !feed.Expires.Equal(want) {
		t.Errorf("expires = %v, want the release date plus -valid-for: %v", feed.Expires, want)
	}
	if len(feed.Latest.Assets) != len(expected) {
		t.Fatalf("assets = %d, want %d: %+v", len(feed.Latest.Assets), len(expected), feed.Latest.Assets)
	}

	seen := make(map[feedTestAssetKey]bool, len(expected))
	for _, asset := range feed.Latest.Assets {
		key := feedTestAssetKey{asset.OS, asset.Arch, asset.Format}
		want, ok := expected[key]
		if !ok {
			t.Errorf("unexpected asset %s/%s/%s", asset.OS, asset.Arch, asset.Format)
			continue
		}
		seen[key] = true
		if asset.URL != want.url {
			t.Errorf("%s: URL = %q, want %q", key, asset.URL, want.url)
		}
		if asset.Size != int64(len(want.content)) {
			t.Errorf("%s: size = %d, want %d", key, asset.Size, len(want.content))
		}
		digest := sha256.Sum256([]byte(want.content))
		if asset.SHA256 != hex.EncodeToString(digest[:]) {
			t.Errorf("%s: sha256 = %q, want %q", key, asset.SHA256, hex.EncodeToString(digest[:]))
		}
	}
	for key := range expected {
		if !seen[key] {
			t.Errorf("the feed lacks %s/%s/%s", key.goos, key.arch, key.format)
		}
	}

	// The signature covers the file bytes, not a re-encoding of the feed.
	signatureText, err := os.ReadFile(out + ".sig")
	if err != nil {
		t.Fatal(err)
	}
	signature, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(signatureText)))
	if err != nil {
		t.Fatalf(".sig: %v", err)
	}
	if !ed25519.Verify(public, raw, signature) {
		t.Error("the signature does not verify the bytes of latest.json")
	}
	if runtime.GOOS != "windows" {
		for _, path := range []string{out, out + ".sig"} {
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0o644 {
				t.Errorf("%s mode = %v, want 0644", path, info.Mode().Perm())
			}
		}
	}

	for _, want := range []string{
		out, out + ".sig", "  app      dev.example.notes", "  version  2.0.1", "  channel  beta", "  assets   6",
	} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("the report lacks %q:\n%s", want, output.String())
		}
	}
}

func TestFeedReadsTheKeyFromTheEnvironment(t *testing.T) {
	dir := t.TempDir()
	writeFeedFile(t, filepath.Join(dir, "viva.toml"), feedTestBasicTOML)
	archive := "notes-2.0.1-darwin-arm64.zip"
	content := "the archive"
	writeFeedFile(t, filepath.Join(dir, "build", archive), content)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("VIVA_UPDATE_KEY", base64.StdEncoding.EncodeToString(private))

	var output bytes.Buffer
	if err := runFeed([]string{"-C", dir, "-base-url", "https://example.com/downloads"}, &output, io.Discard); err != nil {
		t.Fatalf("%v\n%s", err, output.String())
	}
	out := filepath.Join(dir, "build", "latest.json")
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var feed update.Feed
	if err := json.Unmarshal(raw, &feed); err != nil {
		t.Fatalf("latest.json: %v\n%s", err, raw)
	}
	if feed.Channel != "stable" {
		t.Errorf("channel = %q, want the stable default", feed.Channel)
	}
	if len(feed.Latest.Assets) != 1 {
		t.Fatalf("assets = %d, want 1", len(feed.Latest.Assets))
	}
	asset := feed.Latest.Assets[0]
	if asset.URL != "https://example.com/downloads/"+archive || asset.Size != int64(len(content)) {
		t.Errorf("asset = %+v", asset)
	}
	signatureText, err := os.ReadFile(out + ".sig")
	if err != nil {
		t.Fatal(err)
	}
	signature, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(signatureText)))
	if err != nil {
		t.Fatal(err)
	}
	if !ed25519.Verify(public, raw, signature) {
		t.Error("the signature does not verify the bytes of latest.json")
	}
}

func TestFeedReportsWhatItCannotUse(t *testing.T) {
	tests := []struct {
		name      string
		toml      string
		artifacts map[string]string
		arguments []string
		baseURL   string
		noBaseURL bool
		key       string // good, bad or missing
		want      string
	}{
		{
			name: "no archives", toml: feedTestBasicTOML, key: "good",
			want: "no update archives found",
		},
		{
			name: "base URL is not HTTPS", toml: feedTestBasicTOML,
			artifacts: map[string]string{"notes-2.0.1-darwin-arm64.zip": "x"},
			baseURL:   "http://example.com/apps", key: "good", want: "HTTPS",
		},
		{
			name: "no base URL", toml: feedTestBasicTOML, noBaseURL: true,
			artifacts: map[string]string{"notes-2.0.1-darwin-arm64.zip": "x"},
			key:       "good", want: "-base-url",
		},
		{
			name: "missing key", toml: feedTestBasicTOML, key: "missing",
			artifacts: map[string]string{"notes-2.0.1-darwin-arm64.zip": "x"},
			want:      "VIVA_UPDATE_KEY",
		},
		{
			name: "invalid key", toml: feedTestBasicTOML, key: "bad",
			artifacts: map[string]string{"notes-2.0.1-darwin-arm64.zip": "x"},
			want:      "invalid signing key",
		},
		{
			name: "invalid version", toml: feedTestBasicTOML, key: "good",
			artifacts: map[string]string{"notes-2.0.1-darwin-arm64.zip": "x"},
			arguments: []string{"-version", "latest"}, want: "major.minor.patch",
		},
		{
			name: "duplicate archive", toml: feedTestBasicTOML, key: "good",
			artifacts: map[string]string{
				"one-2.0.1-windows-amd64.zip": "x",
				"two-2.0.1-windows-amd64.zip": "y",
			},
			want: "one-2.0.1-windows-amd64.zip and two-2.0.1-windows-amd64.zip",
		},
		{
			name: "archive version", toml: feedTestBasicTOML, key: "good",
			artifacts: map[string]string{"notes-1.9.0-darwin-arm64.zip": "x"},
			want:      "is version 1.9.0, but the feed is 2.0.1",
		},
		{
			name: "no app", toml: "[app]\nname = \"Notes\"\nversion = \"1.2.3\"\n", key: "good",
			artifacts: map[string]string{"notes-1.2.3-darwin-arm64.zip": "x"},
			want:      "[app] id",
		},
		{
			name: "negative lifetime", toml: feedTestBasicTOML, key: "good",
			artifacts: map[string]string{"notes-2.0.1-darwin-arm64.zip": "x"},
			arguments: []string{"-valid-for", "-1h"}, want: "-valid-for",
		},
		{
			name: "notes is not HTTPS", toml: feedTestBasicTOML, key: "good",
			artifacts: map[string]string{"notes-2.0.1-darwin-arm64.zip": "x"},
			arguments: []string{"-notes", "http://example.com/notes"}, want: "-notes",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("VIVA_UPDATE_KEY", "")
			dir := t.TempDir()
			writeFeedFile(t, filepath.Join(dir, "viva.toml"), test.toml)
			build := filepath.Join(dir, "build")
			if err := os.MkdirAll(build, 0o755); err != nil {
				t.Fatal(err)
			}
			for name, content := range test.artifacts {
				writeFeedFile(t, filepath.Join(build, name), content)
			}

			arguments := []string{"-C", dir}
			if !test.noBaseURL {
				baseURL := test.baseURL
				if baseURL == "" {
					baseURL = "https://example.com/apps"
				}
				arguments = append(arguments, "-base-url", baseURL)
			}
			keyPath := ""
			switch test.key {
			case "good":
				keyPath, _ = feedTestKey(t, dir)
			case "bad":
				keyPath = filepath.Join(dir, "bad.key")
				writeFeedFile(t, keyPath, "not a key")
			}
			arguments = append(arguments, test.arguments...)
			if keyPath != "" {
				arguments = append(arguments, "-key", keyPath)
			}

			err := runFeed(arguments, io.Discard, io.Discard)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Errorf("error = %v, want it to mention %q", err, test.want)
			}
		})
	}
}

func TestFeedNeedsAnArchiveDirectory(t *testing.T) {
	t.Setenv("VIVA_UPDATE_KEY", "")
	dir := t.TempDir()
	writeFeedFile(t, filepath.Join(dir, "viva.toml"), feedTestBasicTOML)
	keyPath, _ := feedTestKey(t, dir)
	err := runFeed([]string{"-C", dir, "-base-url", "https://example.com/apps", "-key", keyPath}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("a missing archive directory: %v", err)
	}
}

func TestKeygenWritesAndPrintsTheKeyPair(t *testing.T) {
	dir := t.TempDir()
	privatePath := filepath.Join(dir, "update.key")
	publicPath := filepath.Join(dir, "update.pub")
	var output bytes.Buffer
	if err := runUpdateKeygen([]string{"-private", privatePath, "-public", publicPath}, &output, io.Discard); err != nil {
		t.Fatalf("%v\n%s", err, output.String())
	}

	privateText, err := os.ReadFile(privatePath)
	if err != nil {
		t.Fatal(err)
	}
	private, err := update.ParsePrivateKey(string(privateText))
	if err != nil {
		t.Fatalf("the private key file: %v", err)
	}
	if len(private) != ed25519.PrivateKeySize {
		t.Errorf("private key is %d bytes, want %d", len(private), ed25519.PrivateKeySize)
	}
	publicText, err := os.ReadFile(publicPath)
	if err != nil {
		t.Fatal(err)
	}
	public, err := update.ParsePublicKey(string(publicText))
	if err != nil {
		t.Fatalf("the public key file: %v", err)
	}
	if len(public) != ed25519.PublicKeySize {
		t.Errorf("public key is %d bytes, want %d", len(public), ed25519.PublicKeySize)
	}
	if !public.Equal(private.Public()) {
		t.Error("the public key does not belong to the private key")
	}

	encodedPrivate := base64.StdEncoding.EncodeToString(private)
	encodedPublic := base64.StdEncoding.EncodeToString(public)
	if strings.TrimSpace(string(privateText)) != encodedPrivate || strings.TrimSpace(string(publicText)) != encodedPublic {
		t.Error("the key files are not standard base64 of the keys")
	}
	if !strings.Contains(output.String(), encodedPublic) {
		t.Errorf("the printed public key is missing:\n%s", output.String())
	}
	if strings.Contains(output.String(), encodedPrivate) {
		t.Error("the private key was printed")
	}
	if !strings.Contains(output.String(), privatePath) || !strings.Contains(output.String(), publicPath) {
		t.Errorf("the report lacks a key path:\n%s", output.String())
	}
	if runtime.GOOS != "windows" {
		for _, test := range []struct {
			path string
			mode os.FileMode
		}{{privatePath, 0o600}, {publicPath, 0o644}} {
			info, err := os.Stat(test.path)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != test.mode {
				t.Errorf("%s mode = %v, want %v", test.path, info.Mode().Perm(), test.mode)
			}
		}
	}
}

func TestKeygenDefaultsToUpdateKey(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	var output bytes.Buffer
	if err := runUpdateKeygen(nil, &output, io.Discard); err != nil {
		t.Fatalf("%v\n%s", err, output.String())
	}
	data, err := os.ReadFile(filepath.Join(dir, "update.key"))
	if err != nil {
		t.Fatalf("update.key was not written by default: %v", err)
	}
	if _, err := update.ParsePrivateKey(string(data)); err != nil {
		t.Errorf("update.key: %v", err)
	}
}

func TestKeygenKeepsKeysUnlessForced(t *testing.T) {
	dir := t.TempDir()
	privatePath := filepath.Join(dir, "update.key")
	if err := runUpdateKeygen([]string{"-private", privatePath}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(privatePath)
	if err != nil {
		t.Fatal(err)
	}
	err = runUpdateKeygen([]string{"-private", privatePath}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "-force") {
		t.Fatalf("an existing key: %v", err)
	}
	before, err := os.ReadFile(privatePath)
	if err != nil || !bytes.Equal(first, before) {
		t.Error("a refused run replaced the key")
	}
	if err := runUpdateKeygen([]string{"-private", privatePath, "-force"}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(privatePath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first, after) {
		t.Error("-force left the old key in place")
	}
	if _, err := update.ParsePrivateKey(string(after)); err != nil {
		t.Errorf("the replaced key: %v", err)
	}
}

func TestFeedAndKeygenHelpListsFlags(t *testing.T) {
	for name, flags := range map[string][]string{
		"feed":          {"-C", "-dir", "-out", "-version", "-app", "-channel", "-base-url", "-notes", "-key"},
		"update-keygen": {"-private", "-public", "-force"},
	} {
		var output bytes.Buffer
		if err := runCLI([]string{"help", name}, &output, io.Discard); err != nil {
			t.Fatal(err)
		}
		for _, flag := range flags {
			if !strings.Contains(output.String(), "\n  "+flag+" ") && !strings.Contains(output.String(), "\n  "+flag+"\n") {
				t.Errorf("help for %s does not list %s:\n%s", name, flag, output.String())
			}
		}
	}
}

func copyFeedFile(t *testing.T, source, destination string) {
	t.Helper()
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFeedAddsADeltaFromAnEarlierRelease(t *testing.T) {
	t.Setenv("VIVA_UPDATE_KEY", "")
	const oldTOML = `[app]
name = "Fixture Notes"
id = "dev.example.notes"
version = "2.0.0"
`
	dir := t.TempDir()
	oldDir, newDir := fixture(t, oldTOML), fixture(t, feedTestBasicTOML)
	// A different program, so the patch has something to diff.
	if err := os.WriteFile(filepath.Join(oldDir, "main.go"), []byte("package main\n\nfunc main() { extra() }\n\nfunc extra() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := runLinuxPackage(t, oldDir); err != nil {
		t.Fatalf("packaging the old release: %v\n%s", err, output)
	}
	if output, err := runLinuxPackage(t, newDir); err != nil {
		t.Fatalf("packaging the new release: %v\n%s", err, output)
	}
	dist := filepath.Join(dir, "dist")
	oldArchive := filepath.Join(oldDir, "build", "fixture-notes-2.0.0-linux-amd64.tar.gz")
	newArchive := filepath.Join(newDir, "build", "fixture-notes-2.0.1-linux-amd64.tar.gz")
	copyFeedFile(t, newArchive, filepath.Join(dist, filepath.Base(newArchive)))

	key, _ := feedTestKey(t, dir)
	feedPath := filepath.Join(dist, "latest.json")
	var output bytes.Buffer
	err := runCLI([]string{
		"feed", "-C", newDir, "-dir", dist, "-base-url", "https://example.com/app",
		"-key", key, "-out", feedPath,
		"--patch-from", "linux/amd64=2.0.0=" + oldArchive,
	}, &output, io.Discard)
	if err != nil {
		t.Fatalf("%v\n%s", err, output.String())
	}
	data, err := os.ReadFile(feedPath)
	if err != nil {
		t.Fatal(err)
	}
	var feed update.Feed
	if err := json.Unmarshal(data, &feed); err != nil {
		t.Fatal(err)
	}
	if len(feed.Latest.Assets) != 1 || len(feed.Latest.Assets[0].Patches) != 1 {
		t.Fatalf("the feed does not carry one patch:\n%s", data)
	}
	patch := feed.Latest.Assets[0].Patches[0]
	if patch.FromVersion != "2.0.0" || patch.Size <= 0 || len(patch.SHA256) != 64 {
		t.Fatalf("patch entry = %+v", patch)
	}
	patchPath := filepath.Join(dist, "fixture-notes-2.0.1-linux-amd64.tar.gz.2.0.0.patch")
	reader, err := update.ReadPatch(patchPath)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	manifest := reader.Manifest()
	if manifest.App != "dev.example.notes" || manifest.FromVersion != "2.0.0" || manifest.ToVersion != "2.0.1" ||
		manifest.OS != "linux" || manifest.Arch != "amd64" || manifest.Payload != "file" || len(manifest.Files) == 0 {
		t.Fatalf("manifest = %+v", manifest)
	}
	// The patch must be much smaller than the archive it replaces.
	if patch.Size >= feed.Latest.Assets[0].Size/2 {
		t.Errorf("patch is %d bytes for a %d-byte archive", patch.Size, feed.Latest.Assets[0].Size)
	}
}

func TestFeedRejectsBadPatchFrom(t *testing.T) {
	t.Setenv("VIVA_UPDATE_KEY", "")
	dir := t.TempDir()
	feedDir := fixture(t, feedTestBasicTOML)
	if output, err := runLinuxPackage(t, feedDir); err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
	dist := filepath.Join(dir, "dist")
	archive := filepath.Join(feedDir, "build", "fixture-notes-2.0.1-linux-amd64.tar.gz")
	copyFeedFile(t, archive, filepath.Join(dist, filepath.Base(archive)))
	key, _ := feedTestKey(t, dir)
	run := func(extra ...string) error {
		arguments := append([]string{"feed", "-C", feedDir, "-dir", dist, "-base-url", "https://example.com/app", "-key", key}, extra...)
		var output bytes.Buffer
		return runCLI(arguments, &output, io.Discard)
	}
	for name, argument := range map[string]string{
		"windows":       "windows/amd64=2.0.0=" + archive,
		"bad platform":  "linux/sparc=2.0.0=" + archive,
		"not older":     "linux/amd64=2.0.1=" + archive,
		"unknown asset": "darwin/arm64=2.0.0=" + archive,
		"bad syntax":    "linux/amd64=2.0.0",
		"missing file":  "linux/amd64=2.0.0=/does/not/exist.tar.gz",
	} {
		if err := run("--patch-from", argument); err == nil {
			t.Errorf("%s: --patch-from %q was accepted", name, argument)
		}
	}
}

func writeFeedZip(t *testing.T, path string, files map[string][]byte) {
	t.Helper()
	var encoded bytes.Buffer
	writer := zip.NewWriter(&encoded)
	for name, data := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFeedAddsAWindowsPortablePatch(t *testing.T) {
	t.Setenv("VIVA_UPDATE_KEY", "")
	dir := t.TempDir()
	writeFeedFile(t, filepath.Join(dir, "viva.toml"), feedTestBasicTOML)
	dist := filepath.Join(dir, "dist")
	writeFeedZip(t, filepath.Join(dist, "fixture-notes-2.0.1-windows-amd64.zip"), map[string][]byte{
		"fixture-notes.exe":     []byte("the new program"),
		"conpty/x64/conpty.dll": []byte("dll"),
		"motty-data/README.txt": []byte("user data"),
	})
	oldArchive := filepath.Join(dir, "fixture-notes-2.0.0-windows-amd64.zip")
	writeFeedZip(t, oldArchive, map[string][]byte{
		"fixture-notes.exe":     []byte("the old program"),
		"conpty/x64/conpty.dll": []byte("dll"),
		"motty-data/README.txt": []byte("user data"),
	})
	key, _ := feedTestKey(t, dir)
	feedPath := filepath.Join(dist, "latest.json")
	var output bytes.Buffer
	err := runCLI([]string{
		"feed", "-C", dir, "-dir", dist, "-base-url", "https://example.com/app",
		"-key", key, "-out", feedPath,
		"--patch-from", "windows/amd64=2.0.0=" + oldArchive,
		"--patch-exclude", "motty-data",
	}, &output, io.Discard)
	if err != nil {
		t.Fatalf("%v\n%s", err, output.String())
	}
	data, err := os.ReadFile(feedPath)
	if err != nil {
		t.Fatal(err)
	}
	var feed update.Feed
	if err := json.Unmarshal(data, &feed); err != nil {
		t.Fatal(err)
	}
	if len(feed.Latest.Assets) != 1 || len(feed.Latest.Assets[0].Patches) != 1 {
		t.Fatalf("feed = %s", data)
	}
	patchPath := filepath.Join(dist, "fixture-notes-2.0.1-windows-amd64.zip.2.0.0.patch")
	reader, err := update.ReadPatch(patchPath)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	manifest := reader.Manifest()
	if manifest.OS != "windows" || manifest.Payload != "dir" {
		t.Fatalf("manifest = %+v", manifest)
	}
	for _, file := range manifest.Files {
		if strings.Contains(file.Path, "motty-data") {
			t.Fatalf("the excluded data folder is in the patch: %+v", manifest.Files)
		}
	}
}
