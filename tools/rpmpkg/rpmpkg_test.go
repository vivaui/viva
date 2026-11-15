package rpmpkg

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/tools/debpkg"
	"github.com/vivaui/viva/tools/tarball"
)

// sample is a small application description shared by the tests.
func sample() Package {
	return Package{
		App: debpkg.Package{
			Executable: "lithe_desktop", ID: "app.lithe.desktop", Name: "Lithe Desktop",
			Version: "0.9.1", Arch: "amd64", Maintainer: "Lithe Maintainers",
			Homepage: "https://example.com/lithe", Comment: "A test application",
			Description: "First paragraph.\n\nSecond paragraph.",
			Category:    "Development", Keywords: []string{"ide"},
			Program: []byte("#!/bin/sh\necho hi\n"),
		},
		Requires: []string{"vulkan-loader", "libxcb", "cairo"},
	}
}

func TestFileNameFollowsRPMConvention(t *testing.T) {
	pkg := sample()
	name, err := pkg.FileName()
	if err != nil {
		t.Fatal(err)
	}
	if want := "lithe-desktop-0.9.1-1.x86_64.rpm"; name != want {
		t.Fatalf("FileName() = %q, want %q", name, want)
	}
	pkg.App.Arch = "arm64"
	if name, _ = pkg.FileName(); !strings.HasSuffix(name, ".aarch64.rpm") {
		t.Fatalf("arm64 FileName() = %q, want aarch64 suffix", name)
	}
}

func TestRejectsVersionsRPMCannotName(t *testing.T) {
	pkg := sample()
	pkg.App.Version = "0.9.1-preview"
	if _, err := pkg.FileName(); err == nil {
		t.Fatal("a version with a hyphen must be refused: rpm versions cannot contain '-'")
	}
}

func TestLeadMagicAndNEVR(t *testing.T) {
	lead := leadBytes("lithe-desktop-0.9.1-1")
	if len(lead) != 96 {
		t.Fatalf("lead is %d bytes, want 96", len(lead))
	}
	if !bytes.Equal(lead[:4], []byte{0xed, 0xab, 0xee, 0xdb}) {
		t.Fatalf("lead magic = % x", lead[:4])
	}
	if got := strings.TrimRight(string(lead[10:76]), "\x00"); got != "lithe-desktop-0.9.1-1" {
		t.Fatalf("lead name = %q", got)
	}
}

func TestCPIONamesAndTrailer(t *testing.T) {
	files := installedFiles(sample().mustFiles(t))
	archive := cpioArchive(files, 0)
	if !bytes.HasPrefix(archive, []byte("070701")) {
		t.Fatalf("payload does not start with a newc header: %q", archive[:6])
	}
	if !bytes.Contains(archive, []byte("./usr/bin/lithe_desktop\x00")) {
		t.Fatal("payload does not name the program ./usr/bin/lithe_desktop")
	}
	if !bytes.Contains(archive, []byte("TRAILER!!!\x00")) {
		t.Fatal("payload has no TRAILER!!! entry")
	}
}

func (pkg Package) mustFiles(t *testing.T) []tarball.File {
	t.Helper()
	files, err := pkg.App.Files()
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// TestWriteVerifiesWithRPMTools builds a package and asks the real rpm tools to
// read it. It is skipped where rpm is not installed (on macOS: brew install rpm).
func TestWriteVerifiesWithRPMTools(t *testing.T) {
	rpm, err := exec.LookPath("rpm")
	if err != nil {
		t.Skip("rpm is not installed")
	}
	dir := t.TempDir()
	pkg := sample()
	name, err := pkg.FileName()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := pkg.Write(file); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	run := func(args ...string) string {
		t.Helper()
		out, err := exec.Command(rpm, args...).CombinedOutput()
		if err != nil {
			t.Fatalf("rpm %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return string(out)
	}
	fields := run("-qp", "--queryformat", "%{NAME} %{VERSION} %{RELEASE} %{ARCH} %{LICENSE} %{URL}\n", path)
	if want := "lithe-desktop 0.9.1 1 x86_64 Unspecified https://example.com/lithe\n"; fields != want {
		t.Fatalf("rpm query fields = %q, want %q", fields, want)
	}
	if summary := run("-qp", "--queryformat", "%{SUMMARY}\n", path); summary != "A test application\n" {
		t.Fatalf("rpm summary = %q, want %q", summary, "A test application\n")
	}
	list := run("-qpl", path)
	for _, want := range []string{"/usr/bin/lithe_desktop", "/usr/share/applications/app.lithe.desktop.desktop"} {
		if !strings.Contains(list, want) {
			t.Fatalf("rpm -qpl lacks %q:\n%s", want, list)
		}
	}
	// The package is unsigned, so the signature check is skipped; the digests
	// must all be intact, and the header and payload SHA-256 must be present.
	check := run("-K", "--nosignature", "-v", path)
	for _, want := range []string{"Header SHA256 digest: OK", "Payload SHA256 digest: OK"} {
		if !strings.Contains(check, want) {
			t.Fatalf("rpm -K lacks %q:\n%s", want, check)
		}
	}
	if strings.Contains(check, "BAD") || strings.Contains(check, "NOT OK") {
		t.Fatalf("rpm -K reports a digest mismatch:\n%s", check)
	}
}
