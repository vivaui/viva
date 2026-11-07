//go:build darwin || windows || linux

package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"testing"

	"github.com/vivaui/viva/ui"
)

func newFilesDriver(t *testing.T) *Driver {
	t.Helper()
	driver, err := NewDriver(Config{Title: "files", Width: 100, Height: 100, Paint: func(*ui.Canvas, ui.Size) {}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { driver.Close() })
	return driver
}

func TestClipboardFilesOfADriverRun(t *testing.T) {
	driver := newFilesDriver(t)
	if _, err := ClipboardFiles(); !errors.Is(err, ErrNoFiles) {
		t.Fatalf("an empty clipboard: %v, want ErrNoFiles", err)
	}
	want := []string{filepath.Join(t.TempDir(), "report.pdf"), filepath.Join(t.TempDir(), "folder")}
	driver.SetClipboardFiles(want)
	got, err := ClipboardFiles()
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("ClipboardFiles = %q, %v; want %q", got, err, want)
	}
	// A file manager leaves the names as text too, which is why callers ask for
	// the files first.
	if text, _ := ClipboardText(); text != "report.pdf\nfolder" {
		t.Fatalf("text beside the files = %q", text)
	}
	got[0] = "changed"
	if again, _ := ClipboardFiles(); again[0] != want[0] {
		t.Fatal("a caller changed the driver's clipboard through the slice it was given")
	}

	// Anything else copied replaces the files, as on a real clipboard.
	driver.SetClipboard("just text")
	if _, err := ClipboardFiles(); !errors.Is(err, ErrNoFiles) {
		t.Fatalf("after text replaced the files: %v, want ErrNoFiles", err)
	}
	driver.SetClipboardFiles(want)
	if err := SetClipboardText("typed"); err != nil {
		t.Fatal(err)
	}
	if _, err := ClipboardFiles(); !errors.Is(err, ErrNoFiles) {
		t.Fatalf("after SetClipboardText replaced the files: %v, want ErrNoFiles", err)
	}
	driver.SetClipboardFiles(want)
	if err := driver.SetClipboardImage(onePixelPNG(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := ClipboardFiles(); !errors.Is(err, ErrNoFiles) {
		t.Fatalf("after an image replaced the files: %v, want ErrNoFiles", err)
	}
}

func TestClipboardFilesAreCutAtTheCap(t *testing.T) {
	driver := newFilesDriver(t)
	paths := make([]string, MaxClipboardFiles+3)
	for index := range paths {
		paths[index] = fmt.Sprintf("/tmp/f%d", index)
	}
	driver.SetClipboardFiles(paths)
	got, err := ClipboardFiles()
	if err != nil || len(got) != MaxClipboardFiles {
		t.Fatalf("got %d paths, %v; want the cap %d", len(got), err, MaxClipboardFiles)
	}
	driver.SetClipboardFiles(nil)
	if _, err := ClipboardFiles(); !errors.Is(err, ErrNoFiles) {
		t.Fatalf("after no files were set: %v, want ErrNoFiles", err)
	}
}

func TestSetClipboardFilesOfADriverRun(t *testing.T) {
	driver := newFilesDriver(t)
	want := []string{filepath.Join(t.TempDir(), "report.pdf"), filepath.Join(t.TempDir(), "文件夹")}
	if err := SetClipboardFiles(want); err != nil {
		t.Fatal(err)
	}
	got, err := ClipboardFiles()
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("ClipboardFiles = %q, %v; want %q", got, err, want)
	}
	if text, _ := ClipboardText(); text != "report.pdf\n文件夹" {
		t.Fatalf("text beside the files = %q", text)
	}
	driver.SetClipboard("text")
	if _, err := ClipboardFiles(); !errors.Is(err, ErrNoFiles) {
		t.Fatalf("after text replaced the files: %v, want ErrNoFiles", err)
	}
	if err := SetClipboardFiles(nil); err == nil {
		t.Fatal("an empty file list was accepted")
	}
	if err := SetClipboardFiles([]string{"relative.txt"}); err == nil {
		t.Fatal("a relative path was accepted")
	}
}
