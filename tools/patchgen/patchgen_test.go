package patchgen

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/update"
)

func writeTarGz(t *testing.T, path, root string, files map[string][]byte) {
	t.Helper()
	modes := make(map[string]int64, len(files))
	for name := range files {
		modes[name] = 0o755
	}
	writeTarGzModes(t, path, root, files, modes)
}

// writeTarGzModes writes a tar.gz whose headers carry the given modes, so a
// test decides what the archive says is executable, whatever the host keeps on
// its own file system.
func writeTarGzModes(t *testing.T, path, root string, files map[string][]byte, modes map[string]int64) {
	t.Helper()
	var encoded bytes.Buffer
	gz := gzip.NewWriter(&encoded)
	writer := tar.NewWriter(gz)
	for name, data := range files {
		mode, ok := modes[name]
		if !ok {
			t.Fatalf("%s has no mode", name)
		}
		header := &tar.Header{Name: root + "/" + name, Mode: mode, Size: int64(len(data)), Typeflag: tar.TypeReg}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeZip(t *testing.T, path string, files map[string][]byte) {
	t.Helper()
	var encoded bytes.Buffer
	writer := zip.NewWriter(&encoded)
	for name, data := range files {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWriteLinuxPatch(t *testing.T) {
	dir := t.TempDir()
	oldArchive := filepath.Join(dir, "app-1.0.0-linux-amd64.tar.gz")
	newArchive := filepath.Join(dir, "app-1.1.0-linux-amd64.tar.gz")
	writeTarGz(t, oldArchive, "app-1.0.0-linux-amd64", map[string][]byte{
		"app":        []byte("the old program, with a fair amount of content"),
		"install.sh": []byte("#!/bin/sh\n"),
	})
	writeTarGz(t, newArchive, "app-1.1.0-linux-amd64", map[string][]byte{
		"app":        []byte("the new program, with a fair amount of content and more"),
		"install.sh": []byte("#!/bin/sh\n"),
	})
	patchPath := filepath.Join(dir, "app.patch")
	file, err := os.Create(patchPath)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := Write(Request{
		App: "dev.example.notes", FromVersion: "1.0.0", ToVersion: "1.1.0",
		OS: "linux", Arch: "amd64", OldArchive: oldArchive, NewArchive: newArchive,
		Executable: "app", Output: file,
	})
	file.Close()
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Payload != "file" || len(manifest.Files) != 1 || manifest.Files[0].Path != "app" {
		t.Fatalf("manifest = %+v", manifest)
	}
	patch, err := update.ReadPatch(patchPath)
	if err != nil {
		t.Fatal(err)
	}
	defer patch.Close()
	oldRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(oldRoot, "app"), []byte("the old program, with a fair amount of content"), 0o755); err != nil {
		t.Fatal(err)
	}
	staging := filepath.Join(dir, "staged")
	if err := patch.Apply(oldRoot, staging); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(staging, "app"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "the new program, with a fair amount of content and more" {
		t.Fatalf("patched program = %q", got)
	}
}

// TestWriteLinuxPatchPicksTheOnlyExecutableByArchiveMode pins what a Windows
// host cannot re-derive by statting the unpacked tree: the execution bits of
// the program come from the tar header, and the one regular file that has them
// (install.sh aside) is the patch's payload with no name in the manifest.
func TestWriteLinuxPatchPicksTheOnlyExecutableByArchiveMode(t *testing.T) {
	dir := t.TempDir()
	oldArchive := filepath.Join(dir, "app-1.0.0-linux-amd64.tar.gz")
	newArchive := filepath.Join(dir, "app-1.1.0-linux-amd64.tar.gz")
	oldFiles := map[string][]byte{
		"app":        []byte("the old program, with a fair amount of content"),
		"install.sh": []byte("#!/bin/sh\n"),
		"README.txt": []byte("read me"),
	}
	oldModes := map[string]int64{"app": 0o755, "install.sh": 0o755, "README.txt": 0o644}
	newFiles := map[string][]byte{
		"app":        []byte("the new program, with a fair amount of content and more"),
		"install.sh": []byte("#!/bin/sh\n"),
		"README.txt": []byte("read me"),
	}
	newModes := map[string]int64{"app": 0o755, "install.sh": 0o755, "README.txt": 0o644}
	writeTarGzModes(t, oldArchive, "app-1.0.0-linux-amd64", oldFiles, oldModes)
	writeTarGzModes(t, newArchive, "app-1.1.0-linux-amd64", newFiles, newModes)
	patchPath := filepath.Join(dir, "app.patch")
	file, err := os.Create(patchPath)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := Write(Request{
		App: "dev.example.notes", FromVersion: "1.0.0", ToVersion: "1.1.0",
		OS: "linux", Arch: "amd64", OldArchive: oldArchive, NewArchive: newArchive, Output: file,
	})
	file.Close()
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Payload != "file" || len(manifest.Files) != 1 || manifest.Files[0].Path != "app" {
		t.Fatalf("manifest = %+v", manifest)
	}
}

// TestWriteLinuxPatchRefusesAProgramWithoutExecutionBits keeps a file that the
// archive does not mark executable from being picked by name, whatever the
// host's file system would say about the unpacked copy.
func TestWriteLinuxPatchRefusesAProgramWithoutExecutionBits(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "app-1.0.0-linux-amd64.tar.gz")
	writeTarGzModes(t, archive, "app-1.0.0-linux-amd64", map[string][]byte{
		"app": []byte("the old program"),
	}, map[string]int64{"app": 0o644})
	file, err := os.Create(filepath.Join(dir, "app.patch"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	_, err = Write(Request{
		App: "dev.example.notes", FromVersion: "1.0.0", ToVersion: "1.1.0",
		OS: "linux", Arch: "amd64", OldArchive: archive, NewArchive: archive,
		Executable: "app", Output: file,
	})
	if err == nil || !strings.Contains(err.Error(), "holds no executable") {
		t.Fatalf("Write() = %v, want an archive with no executable", err)
	}
}

// TestFindExecutableTrustsTheArchiveNotTheDisk pins the fix's reason to exist:
// an unpacked copy whose file has no execution bit on disk (every file reads
// back 0666 on Windows) is still the program the archive marked executable.
// It holds on every host, so a regression to a disk-mode check fails here too.
func TestFindExecutableTrustsTheArchiveNotTheDisk(t *testing.T) {
	dir := t.TempDir()
	program := filepath.Join(dir, "app")
	if err := os.WriteFile(program, []byte("the program"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := findExecutable(dir, "", []string{"app"})
	if err != nil {
		t.Fatalf("findExecutable() error = %v", err)
	}
	if got != program {
		t.Fatalf("findExecutable() = %q, want %q", got, program)
	}
}

func TestWriteDarwinPatch(t *testing.T) {
	dir := t.TempDir()
	oldArchive := filepath.Join(dir, "app-1.0.0-darwin-arm64.zip")
	newArchive := filepath.Join(dir, "app-1.1.0-darwin-arm64.zip")
	writeZip(t, oldArchive, map[string][]byte{
		"App.app/Contents/MacOS/app":  []byte("old executable"),
		"App.app/Contents/Info.plist": []byte("<plist>1.0.0</plist>"),
	})
	writeZip(t, newArchive, map[string][]byte{
		"App.app/Contents/MacOS/app":           []byte("new executable with more"),
		"App.app/Contents/Info.plist":          []byte("<plist>1.1.0</plist>"),
		"App.app/Contents/Resources/added.txt": []byte("added"),
	})
	patchPath := filepath.Join(dir, "app.patch")
	file, err := os.Create(patchPath)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := Write(Request{
		App: "dev.example.notes", FromVersion: "1.0.0", ToVersion: "1.1.0",
		OS: "darwin", Arch: "arm64", OldArchive: oldArchive, NewArchive: newArchive, Output: file,
	})
	file.Close()
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Payload != "app" || len(manifest.Files) != 3 {
		t.Fatalf("manifest = %+v", manifest)
	}
	patch, err := update.ReadPatch(patchPath)
	if err != nil {
		t.Fatal(err)
	}
	defer patch.Close()
	oldRoot := t.TempDir()
	for name, data := range map[string][]byte{
		"Contents/MacOS/app":  []byte("old executable"),
		"Contents/Info.plist": []byte("<plist>1.0.0</plist>"),
	} {
		path := filepath.Join(oldRoot, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	staging := filepath.Join(dir, "staged")
	if err := patch.Apply(oldRoot, staging); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(staging, "Contents", "Resources", "added.txt")); err != nil || string(data) != "added" {
		t.Fatalf("added file = %q, %v", data, err)
	}
}

func TestWriteRefusesUnsupportedTargets(t *testing.T) {
	for _, request := range []Request{
		{App: "app", FromVersion: "1.0.0", ToVersion: "1.1.0", OS: "windows", Arch: "amd64"},
		{App: "app", FromVersion: "1.0.0", ToVersion: "1.1.0", OS: "linux", Arch: "386"},
		{App: "", FromVersion: "1.0.0", ToVersion: "1.1.0", OS: "linux", Arch: "amd64"},
	} {
		if _, err := Write(request); err == nil {
			t.Errorf("Write(%+v) accepted", request)
		}
	}
}

func TestWriteWindowsPortablePatch(t *testing.T) {
	dir := t.TempDir()
	oldArchive := filepath.Join(dir, "app-1.0.0-windows-amd64.zip")
	newArchive := filepath.Join(dir, "app-1.1.0-windows-amd64.zip")
	writeZip(t, oldArchive, map[string][]byte{
		"app.exe":               []byte("old program"),
		"conpty/x64/conpty.dll": []byte("dll"),
		"motty-data/README.txt": []byte("data"),
	})
	writeZip(t, newArchive, map[string][]byte{
		"app.exe":               []byte("new program with more"),
		"conpty/x64/conpty.dll": []byte("dll"),
		"motty-data/README.txt": []byte("changed data"),
	})
	patchPath := filepath.Join(dir, "app.patch")
	file, err := os.Create(patchPath)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := Write(Request{
		App: "dev.example.notes", FromVersion: "1.0.0", ToVersion: "1.1.0",
		OS: "windows", Arch: "amd64", OldArchive: oldArchive, NewArchive: newArchive,
		Exclude: []string{"motty-data"}, Output: file,
	})
	file.Close()
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Payload != "dir" || len(manifest.Files) != 2 {
		t.Fatalf("manifest = %+v", manifest)
	}
	for _, file := range manifest.Files {
		if file.Path == "motty-data/README.txt" {
			t.Fatal("the data folder reached the patch")
		}
	}
	patch, err := update.ReadPatch(patchPath)
	if err != nil {
		t.Fatal(err)
	}
	defer patch.Close()
	oldRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(oldRoot, "conpty", "x64"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldRoot, "app.exe"), []byte("old program"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldRoot, "conpty", "x64", "conpty.dll"), []byte("dll"), 0o644); err != nil {
		t.Fatal(err)
	}
	staging := filepath.Join(dir, "staged")
	if err := patch.Apply(oldRoot, staging); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(staging, "app.exe"))
	if err != nil || string(got) != "new program with more" {
		t.Fatalf("app.exe = %q, %v", got, err)
	}
}
