package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type archiveEntry struct {
	name string
	mode os.FileMode
	body string
}

func makeZip(t *testing.T, entries []archiveEntry) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "archive.zip")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Deflate}
		header.SetMode(entry.mode)
		part, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write([]byte(entry.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func makeTarGz(t *testing.T, entries []archiveEntry) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "archive.tar.gz")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	compressed := gzip.NewWriter(file)
	writer := tar.NewWriter(compressed)
	for _, entry := range entries {
		header := &tar.Header{
			Name:     entry.name,
			Mode:     int64(entry.mode.Perm()),
			Size:     int64(len(entry.body)),
			Typeflag: tar.TypeReg,
		}
		if entry.mode&os.ModeDir != 0 {
			header.Typeflag = tar.TypeDir
			header.Size = 0
		}
		if entry.mode&os.ModeSymlink != 0 {
			header.Typeflag = tar.TypeSymlink
			header.Linkname = entry.body
			header.Size = 0
		}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if header.Typeflag == tar.TypeReg && len(entry.body) > 0 {
			if _, err := writer.Write([]byte(entry.body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestExtractZipKeepsFilesAndModes(t *testing.T) {
	path := makeZip(t, []archiveEntry{
		{name: "App.app/Contents/MacOS/App", mode: 0o755, body: "binary"},
		{name: "App.app/Contents/Info.plist", mode: 0o644, body: "plist"},
	})
	destination := t.TempDir()
	if err := extractZip(path, destination); err != nil {
		t.Fatalf("extractZip: %v", err)
	}
	program := filepath.Join(destination, "App.app/Contents/MacOS/App")
	info, err := os.Stat(program)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o100 == 0 {
		t.Errorf("mode %v lost the executable bit", info.Mode())
	}
	body, err := os.ReadFile(program)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "binary" {
		t.Errorf("body = %q", body)
	}
}

func TestExtractRefusesPathsThatLeaveTheArchive(t *testing.T) {
	tests := []struct {
		name  string
		build func(t *testing.T) string
	}{
		{"zip parent", func(t *testing.T) string {
			return makeZip(t, []archiveEntry{{name: "../escape.txt", mode: 0o644, body: "no"}})
		}},
		{"zip absolute", func(t *testing.T) string {
			return makeZip(t, []archiveEntry{{name: "/tmp/escape.txt", mode: 0o644, body: "no"}})
		}},
		{"zip symlink", func(t *testing.T) string {
			return makeZip(t, []archiveEntry{{name: "link", mode: os.ModeSymlink | 0o777, body: "/etc/passwd"}})
		}},
		{"tar parent", func(t *testing.T) string {
			return makeTarGz(t, []archiveEntry{{name: "../../escape.txt", mode: 0o644, body: "no"}})
		}},
		{"tar symlink", func(t *testing.T) string {
			return makeTarGz(t, []archiveEntry{{name: "link", mode: os.ModeSymlink | 0o777, body: "/etc/passwd"}})
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := test.build(t)
			destination := t.TempDir()
			var err error
			if filepath.Ext(path) == ".zip" {
				err = extractZip(path, destination)
			} else {
				err = extractTarGz(path, destination)
			}
			if !errors.Is(err, ErrUnsafeArchive) {
				t.Fatalf("error %v does not wrap ErrUnsafeArchive", err)
			}
			if _, statErr := os.Stat(filepath.Join(filepath.Dir(destination), "escape.txt")); statErr == nil {
				t.Error("an entry was written outside the destination")
			}
		})
	}
}

func TestExtractTarGzKeepsFiles(t *testing.T) {
	path := makeTarGz(t, []archiveEntry{
		{name: "app", mode: 0o755, body: "binary"},
		{name: "LICENSE", mode: 0o644, body: "license"},
	})
	destination := t.TempDir()
	if err := extractTarGz(path, destination); err != nil {
		t.Fatalf("extractTarGz: %v", err)
	}
	info, err := os.Stat(filepath.Join(destination, "app"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o100 == 0 {
		t.Errorf("mode %v lost the executable bit", info.Mode())
	}
}

func TestExtractRefusesADuplicateEntry(t *testing.T) {
	path := makeZip(t, []archiveEntry{
		{name: "same", mode: 0o644, body: "one"},
		{name: "same", mode: 0o644, body: "two"},
	})
	if err := extractZip(path, t.TempDir()); !errors.Is(err, ErrUnsafeArchive) {
		t.Errorf("error %v does not wrap ErrUnsafeArchive", err)
	}
}
