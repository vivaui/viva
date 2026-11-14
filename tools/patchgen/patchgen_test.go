package patchgen

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/update"
)

func writeTarGz(t *testing.T, path, root string, files map[string][]byte) {
	t.Helper()
	var encoded bytes.Buffer
	gz := gzip.NewWriter(&encoded)
	writer := tar.NewWriter(gz)
	for name, data := range files {
		header := &tar.Header{Name: root + "/" + name, Mode: 0o755, Size: int64(len(data)), Typeflag: tar.TypeReg}
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
