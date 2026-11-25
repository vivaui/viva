package tarball

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/tools/mimeinfo"
)

func readTar(t *testing.T, compressed []byte) (headers []*tar.Header, contents map[string][]byte) {
	t.Helper()
	zipped, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	if !zipped.ModTime.IsZero() || zipped.Name != "" {
		t.Errorf("the gzip header carries a time or name: %v %q", zipped.ModTime, zipped.Name)
	}
	reader := tar.NewReader(zipped)
	contents = map[string][]byte{}
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return headers, contents
		}
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(reader)
		headers = append(headers, header)
		contents[header.Name] = data
	}
}

func TestWriteTarIsSortedOwnedByRootAndHasDirectories(t *testing.T) {
	modified := time.Unix(1700000000, 0)
	files := []File{
		{Path: "usr/share/b.txt", Data: []byte("b"), Mode: 0o644},
		{Path: "usr/bin/a", Data: []byte("aa"), Mode: 0o755},
		{Path: "usr/share/doc/c", Data: nil, Mode: 0o644},
	}
	var buffer bytes.Buffer
	if err := WriteTar(&buffer, files, ".", modified); err != nil {
		t.Fatal(err)
	}
	headers, _ := readTar(t, mustGzip(t, buffer.Bytes()))
	var names []string
	for _, header := range headers {
		names = append(names, header.Name)
		if header.Uid != 0 || header.Gid != 0 || header.Uname != "root" || header.Gname != "root" {
			t.Errorf("%s: owner %d:%d %s:%s", header.Name, header.Uid, header.Gid, header.Uname, header.Gname)
		}
		if !header.ModTime.Equal(modified) {
			t.Errorf("%s: mtime %v", header.Name, header.ModTime)
		}
		wantMode := int64(0o755)
		if header.Typeflag == tar.TypeReg && header.Name != "./usr/bin/a" {
			wantMode = 0o644
		}
		if header.Mode != wantMode {
			t.Errorf("%s: mode %o, want %o", header.Name, header.Mode, wantMode)
		}
	}
	want := "./ ./usr/ ./usr/bin/ ./usr/bin/a ./usr/share/ ./usr/share/b.txt ./usr/share/doc/ ./usr/share/doc/c"
	if got := strings.Join(names, " "); got != want {
		t.Errorf("entries\n got %s\nwant %s", got, want)
	}
}

func mustGzip(t *testing.T, data []byte) []byte {
	t.Helper()
	out, err := Gzip(data)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestWriteTarRejectsBadPaths(t *testing.T) {
	for _, name := range []string{"", "/abs", "a/../b", "dir/", "../x", "a//b", "./a"} {
		if err := WriteTar(io.Discard, []File{{Path: name, Mode: 0o644}}, "", time.Unix(0, 0)); err == nil {
			t.Errorf("%q was accepted", name)
		}
	}
	if err := WriteTar(io.Discard, []File{{Path: "a", Mode: 0o644}, {Path: "a", Mode: 0o644}}, "", time.Unix(0, 0)); err == nil {
		t.Error("a duplicate was accepted")
	}
	if err := WriteTar(io.Discard, []File{{Path: "a", Mode: 0o644}, {Path: "a/b", Mode: 0o644}}, "", time.Unix(0, 0)); err == nil {
		t.Error("a file used as a directory was accepted")
	}
}

func TestSourceDate(t *testing.T) {
	t.Setenv("SOURCE_DATE_EPOCH", "")
	if got := SourceDate(); !got.Equal(time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("default = %v", got)
	}
	t.Setenv("SOURCE_DATE_EPOCH", "1700000000")
	if got := SourceDate(); got.Unix() != 1700000000 {
		t.Errorf("epoch = %v", got)
	}
	t.Setenv("SOURCE_DATE_EPOCH", "garbage")
	if got := SourceDate(); got.Year() != 1980 {
		t.Errorf("garbage = %v", got)
	}
}

func sample() Linux {
	return Linux{
		Executable: "fixture-notes", Name: "Fixture Notes", Version: "2.0.1", Arch: "amd64", Comment: "Takes notes",
		Category: "Utility", Keywords: []string{"notes"}, Program: []byte("\x7fELFprogram"),
		Icon: []byte("\x89PNGicon"), IconPixels: 256,
	}
}

func build(t *testing.T, spec Linux) []byte {
	t.Helper()
	var buffer bytes.Buffer
	if err := spec.Write(&buffer); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestLinuxArchiveLayout(t *testing.T) {
	headers, contents := readTar(t, build(t, sample()))
	root := "fixture-notes-2.0.1-linux-amd64/"
	modes := map[string]int64{
		root: 0o755, root + "fixture-notes": 0o755, root + "fixture-notes.desktop": 0o644, root + "fixture-notes.png": 0o644,
		root + "install.sh": 0o755, root + "README.txt": 0o644,
	}
	if len(headers) != len(modes) {
		t.Errorf("%d entries, want %d", len(headers), len(modes))
	}
	for _, header := range headers {
		want, ok := modes[header.Name]
		if !ok {
			t.Errorf("unexpected entry %s", header.Name)
			continue
		}
		if header.Mode != want || header.Uid != 0 || header.Gid != 0 || !header.ModTime.Equal(SourceDate()) {
			t.Errorf("%s: mode %o owner %d:%d mtime %v", header.Name, header.Mode, header.Uid, header.Gid, header.ModTime)
		}
	}
	desktop := string(contents[root+"fixture-notes.desktop"])
	for _, want := range []string{"Exec=fixture-notes\n", "Icon=fixture-notes\n", "Terminal=false\n", "Type=Application\n", "Categories=Utility;\n", "StartupWMClass=fixture-notes\n", "Keywords=notes;\n", "Name=Fixture Notes\n"} {
		if !strings.Contains(desktop, want) {
			t.Errorf("the desktop file lacks %q:\n%s", want, desktop)
		}
	}
	if string(contents[root+"fixture-notes"]) != "\x7fELFprogram" || string(contents[root+"fixture-notes.png"]) != "\x89PNGicon" {
		t.Error("program or icon bytes changed")
	}
	script := string(contents[root+"install.sh"])
	if !strings.HasPrefix(script, "#!/bin/sh\n") || !strings.Contains(script, "set -eu") || !strings.Contains(script, "app=fixture-notes\n") || !strings.Contains(script, "icon_size=256\n") || strings.Contains(script, "@") {
		t.Errorf("install.sh template: %s", script[:200])
	}
	if !strings.Contains(string(contents[root+"README.txt"]), "./install.sh uninstall") || strings.Contains(string(contents[root+"README.txt"]), "@") {
		t.Error("README.txt is wrong")
	}
}

func TestLinuxArchiveWithoutAnIcon(t *testing.T) {
	spec := sample()
	spec.Icon, spec.IconPixels = nil, 0
	headers, contents := readTar(t, build(t, spec))
	for _, header := range headers {
		if strings.HasSuffix(header.Name, ".png") {
			t.Error("a png without an icon")
		}
	}
	desktop := string(contents["fixture-notes-2.0.1-linux-amd64/fixture-notes.desktop"])
	if strings.Contains(desktop, "Icon=") {
		t.Errorf("the desktop file names an icon:\n%s", desktop)
	}
}

func TestLinuxArchiveIsReproducible(t *testing.T) {
	if !bytes.Equal(build(t, sample()), build(t, sample())) {
		t.Error("two builds differ")
	}
	t.Setenv("SOURCE_DATE_EPOCH", "1700000000")
	first := build(t, sample())
	headers, _ := readTar(t, first)
	if headers[0].ModTime.Unix() != 1700000000 {
		t.Errorf("SOURCE_DATE_EPOCH ignored: %v", headers[0].ModTime)
	}
	if !bytes.Equal(first, build(t, sample())) {
		t.Error("builds with SOURCE_DATE_EPOCH differ")
	}
}

func TestLinuxArchiveRejectsBadSpecs(t *testing.T) {
	for _, change := range []func(*Linux){
		func(l *Linux) { l.Executable = "bad name" },
		func(l *Linux) { l.Executable = "../x" },
		func(l *Linux) { l.Name = "" },
		func(l *Linux) { l.Version = "1/2" },
		func(l *Linux) { l.Arch = "386" },
		func(l *Linux) { l.Program = nil },
		func(l *Linux) { l.IconPixels = 0 },
		func(l *Linux) { l.Name = "bad\x01" },
	} {
		spec := sample()
		change(&spec)
		if err := spec.Write(io.Discard); err == nil {
			t.Errorf("%+v was accepted", spec)
		}
	}
}

// shellPath is path as the sh these tests run sees it. install.sh is a POSIX
// script that insists on an absolute prefix; on Windows sh is MSYS or Git
// Bash, which reads a drive letter as a relative name and wants /c/Users/...
func shellPath(path string) string {
	if runtime.GOOS != "windows" {
		return path
	}
	path = filepath.ToSlash(path)
	if len(path) >= 2 && path[1] == ':' {
		return "/" + strings.ToLower(path[:1]) + path[2:]
	}
	return path
}

// TestInstallScript unpacks the archive and runs install.sh with sh: install,
// replace, and uninstall under a prefix, and the checks on the prefix.
func TestInstallScript(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	extract := t.TempDir()
	headers, contents := readTar(t, build(t, sample()))
	for _, header := range headers {
		path := filepath.Join(extract, header.Name)
		if header.Typeflag == tar.TypeDir {
			os.MkdirAll(path, 0o755)
			continue
		}
		if err := os.WriteFile(path, contents[header.Name], os.FileMode(header.Mode)); err != nil {
			t.Fatal(err)
		}
	}
	script := filepath.Join(extract, "fixture-notes-2.0.1-linux-amd64", "install.sh")
	if out, err := exec.Command("sh", "-n", script).CombinedOutput(); err != nil {
		t.Fatalf("sh -n: %v\n%s", err, out)
	}
	run := func(arguments ...string) (string, error) {
		command := exec.Command("sh", append([]string{script}, arguments...)...)
		command.Env = append(os.Environ(), "HOME="+extract)
		out, err := command.CombinedOutput()
		return string(out), err
	}

	prefix := filepath.Join(t.TempDir(), "prefix")
	shellPrefix := shellPath(prefix)
	for round := 0; round < 2; round++ { // the second round replaces
		if out, err := run("--prefix", shellPrefix); err != nil {
			t.Fatalf("install: %v\n%s", err, out)
		}
	}
	for _, path := range []string{"bin/fixture-notes", "share/applications/fixture-notes.desktop", "share/icons/hicolor/256x256/apps/fixture-notes.png"} {
		if _, err := os.Stat(filepath.Join(prefix, path)); err != nil {
			t.Error(err)
		}
	}
	info, _ := os.Stat(filepath.Join(prefix, "bin", "fixture-notes"))
	// NTFS has no execute bit; Go reports every file there as 0666 or 0444.
	if runtime.GOOS != "windows" && info.Mode()&0o111 == 0 {
		t.Error("the installed program is not executable")
	}
	desktop, _ := os.ReadFile(filepath.Join(prefix, "share/applications/fixture-notes.desktop"))
	for _, want := range []string{"Exec=" + shellPrefix + "/bin/fixture-notes\n", "Icon=" + shellPrefix + "/share/icons/hicolor/256x256/apps/fixture-notes.png\n", "Name=Fixture Notes\n"} {
		if !strings.Contains(string(desktop), want) {
			t.Errorf("the installed desktop file lacks %q:\n%s", want, desktop)
		}
	}
	if entries, _ := os.ReadDir(filepath.Join(prefix, "bin")); len(entries) != 1 {
		t.Errorf("bin holds %d files; a temporary file was left", len(entries))
	}

	if out, err := run("uninstall", "--prefix="+shellPrefix); err != nil || !strings.Contains(out, "removed") {
		t.Fatalf("uninstall: %v\n%s", err, out)
	}
	for _, path := range []string{"bin/fixture-notes", "share/applications/fixture-notes.desktop", "share/icons/hicolor/256x256/apps/fixture-notes.png"} {
		if _, err := os.Stat(filepath.Join(prefix, path)); !os.IsNotExist(err) {
			t.Errorf("%s survived the uninstall", path)
		}
	}

	if _, err := os.Stat(filepath.Join(prefix, "share/icons/hicolor/256x256")); !os.IsNotExist(err) {
		t.Error("the empty icon directories survived the uninstall")
	}

	// The default prefix is under HOME.
	if out, err := run(); err != nil {
		t.Fatalf("default install: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(extract, ".local", "bin", "fixture-notes")); err != nil {
		t.Error(err)
	}

	for _, arguments := range [][]string{{"--prefix", "relative"}, {"--prefix", "/tmp/has space"}, {"--prefix"}, {"--nonsense"}} {
		if out, err := run(arguments...); err == nil {
			t.Errorf("%v was accepted: %s", arguments, out)
		}
	}
	if out, err := run("--help"); err != nil || !strings.Contains(out, "usage:") {
		t.Errorf("--help: %v %s", err, out)
	}
}

func TestFileTypesAreInstalledAndRemoved(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	spec := sample()
	spec.FileTypes = []mimeinfo.Type{{MIME: "text/markdown", Name: "Markdown", Extensions: []string{"md"}}}
	extract := t.TempDir()
	headers, contents := readTar(t, build(t, spec))
	for _, header := range headers {
		path := filepath.Join(extract, header.Name)
		if header.Typeflag == tar.TypeDir {
			os.MkdirAll(path, 0o755)
			continue
		}
		if err := os.WriteFile(path, contents[header.Name], os.FileMode(header.Mode)); err != nil {
			t.Fatal(err)
		}
	}
	root := filepath.Join(extract, "fixture-notes-2.0.1-linux-amd64")
	desktop := string(contents["fixture-notes-2.0.1-linux-amd64/fixture-notes.desktop"])
	for _, want := range []string{"Exec=fixture-notes %F\n", "MimeType=text/markdown;\n"} {
		if !strings.Contains(desktop, want) {
			t.Errorf("the desktop file lacks %q:\n%s", want, desktop)
		}
	}
	prefix := filepath.Join(t.TempDir(), "prefix")
	run := func(arguments ...string) {
		command := exec.Command("sh", append([]string{filepath.Join(root, "install.sh")}, arguments...)...)
		command.Env = append(os.Environ(), "HOME="+extract)
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", arguments, err, out)
		}
	}
	run("--prefix", shellPath(prefix))
	installed := filepath.Join(prefix, "share", "applications", "fixture-notes.desktop")
	if data, err := os.ReadFile(installed); err != nil || !strings.Contains(string(data), "/bin/fixture-notes %F\n") {
		t.Errorf("installed entry: %v\n%s", err, data)
	}
	types := filepath.Join(prefix, "share", "mime", "packages", "fixture-notes.xml")
	if data, err := os.ReadFile(types); err != nil || !strings.Contains(string(data), `<glob pattern="*.md"/>`) {
		t.Errorf("installed mime package: %v\n%s", err, data)
	}
	run("uninstall", "--prefix", shellPath(prefix))
	if _, err := os.Stat(types); err == nil {
		t.Error("uninstall left the mime package")
	}
}
