package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/vivaui/viva/tools/winpkg"
)

const windowsTOML = `[app]
name = "Fixture Notes"
id = "dev.example.notes"
version = "2.0.1"
icon = "icon.png"

[windows]
publisher = "Example Ltd"
url = "https://example.com/notes"
`

// packageWindowsFixture runs viva package for windows/amd64 and skips when the
// machine cannot make an installer (no NSIS, or the macOS makensis crash).
func packageWindowsFixture(t *testing.T, dir string, arguments ...string) string {
	t.Helper()
	var output bytes.Buffer
	err := runCLI(append([]string{"package", "-C", dir, "--target", "windows", "--arch", "amd64"}, arguments...), &output, io.Discard)
	if errors.Is(err, winpkg.ErrNoNSIS) || errors.Is(err, winpkg.ErrMakensisCrash) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatalf("%v\n%s", err, output.String())
	}
	return output.String()
}

func readZip(t *testing.T, path string) map[string][]byte {
	t.Helper()
	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	files := map[string][]byte{}
	for _, file := range reader.File {
		entry, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		files[file.Name], _ = io.ReadAll(entry)
		entry.Close()
	}
	return files
}

func TestPackageWindowsMakesAnInstallerAndAZip(t *testing.T) {
	dir := fixture(t, windowsTOML)
	writeIcon(t, filepath.Join(dir, "icon.png"), 512)
	got := packageWindowsFixture(t, dir, "--format", "nsis,zip")

	installer := filepath.Join(dir, "build", "fixture-notes-2.0.1-windows-amd64-setup.exe")
	data, err := os.ReadFile(installer)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 2 || string(data[:2]) != "MZ" {
		t.Errorf("the installer is not a PE file: % x", data[:min(8, len(data))])
	}
	archive := filepath.Join(dir, "build", "fixture-notes-2.0.1-windows-amd64.zip")
	files := readZip(t, archive)
	if len(files) != 1 || len(files["fixture-notes.exe"]) < 2 || string(files["fixture-notes.exe"][:2]) != "MZ" {
		t.Errorf("zip entries: %d", len(files))
	}
	for _, want := range []string{
		"packaged ", "  app      Fixture Notes 2.0.1 (dev.example.notes)", "  target   windows/amd64",
		"  program  ", "  scope    user: installs under the user's profile", "  nsis     ", "  zip      ",
		"  signing  none: Windows SmartScreen warns on an unsigned installer",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the report lacks %q:\n%s", want, got)
		}
	}
	if !strings.Contains(got, formatSize(int64(len(data)))) {
		t.Errorf("the report lacks the installer size %s:\n%s", formatSize(int64(len(data))), got)
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "build", "*.viva-tmp"))
	if len(matches) != 0 {
		t.Errorf("temporary files left behind: %v", matches)
	}
}

func TestPackageWindowsInstallerWorksWithoutAnIcon(t *testing.T) {
	dir := fixture(t, "[app]\nname = \"Notes\"\nid = \"dev.example.notes\"\n\n[windows]\nscope = \"machine\"\n")
	got := packageWindowsFixture(t, dir)
	if _, err := os.Stat(filepath.Join(dir, "build", "notes-0.1.0-windows-amd64-setup.exe")); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"  icon     none (set [app] icon in viva.toml)", "  scope    machine: installs in Program Files"} {
		if !strings.Contains(got, want) {
			t.Errorf("the report lacks %q:\n%s", want, got)
		}
	}
}

func TestPackageWindowsShipsExtraFiles(t *testing.T) {
	dir := fixture(t, windowsTOML)
	writeIcon(t, filepath.Join(dir, "icon.png"), 64)
	extra := filepath.Join(dir, "native", "conpty.dll")
	if err := os.MkdirAll(filepath.Dir(extra), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(extra, []byte("MZ conpty"), 0o644); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := runCLI([]string{
		"package", "-C", dir, "--target", "windows", "--arch", "amd64", "--format", "zip",
		"--file", "native/conpty.dll=conpty/x64/conpty.dll",
	}, &output, io.Discard); err != nil {
		t.Fatalf("%v\n%s", err, output.String())
	}
	files := readZip(t, filepath.Join(dir, "build", "fixture-notes-2.0.1-windows-amd64.zip"))
	if string(files["conpty/x64/conpty.dll"]) != "MZ conpty" {
		t.Errorf("zip files: %v", files)
	}
	if !strings.Contains(output.String(), "  files    1 beside the program") {
		t.Errorf("the report lacks the extra file:\n%s", output.String())
	}
	if err := runCLI([]string{
		"package", "-C", dir, "--target", "windows", "--arch", "amd64", "--format", "zip",
		"--file", "native/missing.dll=conpty/x64/missing.dll",
	}, io.Discard, io.Discard); err == nil {
		t.Error("a missing extra file was accepted")
	}
}

func TestPackageWindowsZipNeedsNoInstallerTool(t *testing.T) {
	dir := fixture(t, windowsTOML)
	writeIcon(t, filepath.Join(dir, "icon.png"), 64) // too small, but a zip has no icon
	t.Setenv("NSIS_HOME", "")
	pack := func() []byte {
		var output bytes.Buffer
		if err := runCLI([]string{"package", "-C", dir, "--target", "windows", "--arch", "amd64", "--zip", "--format", "zip", "-o", filepath.Join(dir, "dist")}, &output, io.Discard); err != nil {
			t.Fatalf("%v\n%s", err, output.String())
		}
		if strings.Contains(output.String(), "  icon ") || strings.Contains(output.String(), "  nsis ") {
			t.Errorf("the zip report mentions the installer:\n%s", output.String())
		}
		data, err := os.ReadFile(filepath.Join(dir, "dist", "fixture-notes-2.0.1-windows-amd64.zip"))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	first := pack()
	if second := pack(); !bytes.Equal(first, second) {
		t.Error("the zip is not reproducible")
	}
	if _, err := os.Stat(filepath.Join(dir, "dist", "fixture-notes-2.0.1-windows-amd64-setup.exe")); err == nil {
		t.Error("an installer was made though only a zip was asked for")
	}
}

func TestPackageWindowsSaysHowToInstallNSIS(t *testing.T) {
	dir := fixture(t, "[app]\nname = \"Notes\"\nid = \"dev.example.notes\"\n")
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no go tool")
	}
	empty := t.TempDir()
	if err := os.Symlink(goTool, filepath.Join(empty, filepath.Base(goTool))); err != nil {
		t.Skip(err)
	}
	t.Setenv("PATH", empty)
	t.Setenv("NSIS_HOME", "")
	saved := winpkg.CommonLocations
	winpkg.CommonLocations = nil
	defer func() { winpkg.CommonLocations = saved }()

	err = runCLI([]string{"package", "-C", dir, "--target", "windows", "--arch", "amd64"}, io.Discard, io.Discard)
	if !errors.Is(err, winpkg.ErrNoNSIS) {
		t.Fatalf("err = %v", err)
	}
	for _, want := range []string{"brew install nsis", "apt install nsis", "nsis.sourceforge.io"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the message lacks %q: %v", want, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "build")); err == nil {
		entries, _ := os.ReadDir(filepath.Join(dir, "build"))
		if len(entries) != 0 {
			t.Errorf("a failed run left %d files", len(entries))
		}
	}
}

func TestPackageWindowsFailureKeepsThePreviousPackages(t *testing.T) {
	dir := fixture(t, windowsTOML)
	writeIcon(t, filepath.Join(dir, "icon.png"), 256)
	build := filepath.Join(dir, "build")
	os.MkdirAll(build, 0o755)
	previous := map[string]string{
		"fixture-notes-2.0.1-windows-amd64.zip":       "previous zip",
		"fixture-notes-2.0.1-windows-amd64-setup.exe": "previous installer",
	}
	for name, content := range previous {
		os.WriteFile(filepath.Join(build, name), []byte(content), 0o644)
	}
	// A program that does not compile stops the run before anything is replaced.
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() { undefined() }\n"), 0o644)
	var errorOutput bytes.Buffer
	err := runCLI([]string{"package", "-C", dir, "--target", "windows", "--arch", "amd64", "--format", "nsis,zip"}, io.Discard, &errorOutput)
	if err == nil || !strings.Contains(err.Error(), "go build failed") {
		t.Fatalf("err = %v", err)
	}
	for name, content := range previous {
		if data, _ := os.ReadFile(filepath.Join(build, name)); string(data) != content {
			t.Errorf("%s was changed: %q", name, data)
		}
	}

	// An icon that cannot be used stops it before the build, with the same result.
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644)
	writeIcon(t, filepath.Join(dir, "icon.png"), 64)
	err = runCLI([]string{"package", "-C", dir, "--target", "windows", "--arch", "amd64"}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "at least 128 px") {
		t.Fatalf("small icon: %v", err)
	}
	for name, content := range previous {
		if data, _ := os.ReadFile(filepath.Join(build, name)); string(data) != content {
			t.Errorf("%s was changed after the icon error: %q", name, data)
		}
	}
}

func TestPackageWindowsRefusesFormatsOfOtherTargets(t *testing.T) {
	dir := fixture(t, windowsTOML)
	err := runCLI([]string{"package", "-C", dir, "--target", "windows", "--format", "dmg"}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "nsis, zip") {
		t.Errorf("err = %v", err)
	}
}

// fakeToolchain puts the go tool and a stand-in makensis alone on PATH. The
// stand-in reads the script it is given, writes a PE-looking file where
// OutFile says, and prints a warning.
func fakeToolchain(t *testing.T, makensis string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in makensis is a shell script")
	}
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no go tool")
	}
	bin := t.TempDir()
	if err := os.Symlink(goTool, filepath.Join(bin, "go")); err != nil {
		t.Skip(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "makensis"), []byte(makensis), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+"/usr/bin:/bin")
}

const fakeMakensis = `#!/bin/sh
for last; do :; done
out=$(sed -n 's/^OutFile "\(.*\)"$/\1/p' "$last")
printf 'MZ stand-in installer for %s' "$out" > "$out"
echo 'warning: something odd (installer.nsi:7)'
`

func TestPackageWindowsPassesTheScriptToMakensis(t *testing.T) {
	fakeToolchain(t, fakeMakensis)
	dir := fixture(t, windowsTOML)
	writeIcon(t, filepath.Join(dir, "icon.png"), 300)
	var output bytes.Buffer
	if err := runCLI([]string{"package", "-C", dir, "--target", "windows", "--arch", "amd64"}, &output, io.Discard); err != nil {
		t.Fatalf("%v\n%s", err, output.String())
	}
	installer := filepath.Join(dir, "build", "fixture-notes-2.0.1-windows-amd64-setup.exe")
	data, err := os.ReadFile(installer)
	if err != nil || !bytes.HasPrefix(data, []byte("MZ")) {
		t.Fatalf("installer: %v %q", err, data)
	}
	// The stand-in recorded the temporary name it was told to write; the file
	// was then renamed, so none is left.
	if !bytes.Contains(data, []byte("-setup.exe.viva-tmp")) {
		t.Errorf("makensis was not given a temporary output: %q", data)
	}
	if matches, _ := filepath.Glob(filepath.Join(dir, "build", "*.viva-tmp")); len(matches) != 0 {
		t.Errorf("left behind: %v", matches)
	}
	for _, want := range []string{"-setup.exe  " + formatSize(int64(len(data))), "  warning  makensis: something odd (installer.nsi:7)",
		"icon.png\n"} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("the report lacks %q:\n%s", want, output.String())
		}
	}
}

func TestPackageWindowsMakensisFailureKeepsThePreviousInstaller(t *testing.T) {
	fakeToolchain(t, "#!/bin/sh\necho 'error: boom' >&2\nprintf partial > \"$(sed -n 's/^OutFile \"\\(.*\\)\"$/\\1/p' \"$3\")\" 2>/dev/null\nexit 1\n")
	dir := fixture(t, windowsTOML)
	writeIcon(t, filepath.Join(dir, "icon.png"), 256)
	build := filepath.Join(dir, "build")
	os.MkdirAll(build, 0o755)
	installer := filepath.Join(build, "fixture-notes-2.0.1-windows-amd64-setup.exe")
	os.WriteFile(installer, []byte("previous installer"), 0o644)
	err := runCLI([]string{"package", "-C", dir, "--target", "windows", "--arch", "amd64"}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "makensis failed") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v", err)
	}
	if data, _ := os.ReadFile(installer); string(data) != "previous installer" {
		t.Errorf("the installer was replaced: %q", data)
	}
	if matches, _ := filepath.Glob(filepath.Join(build, "*.viva-tmp")); len(matches) != 0 {
		t.Errorf("left behind: %v", matches)
	}
}
