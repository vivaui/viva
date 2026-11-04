//go:build darwin || windows || linux

package app

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestRevealPathRecordsAnExistingEntryWithoutOpeningIt(t *testing.T) {
	driver, err := NewDriver(Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if _, ok := driver.LastRevealedPath(); ok {
		t.Fatal("a new driver has already revealed a path")
	}
	directory := t.TempDir()
	file := filepath.Join(directory, "a runnable script.sh")
	if err := os.WriteFile(file, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Reveal selects the executable; OpenPath would refuse to run it.
	for _, path := range []string{file, directory} {
		if err := RevealPath(path); err != nil {
			t.Fatal(err)
		}
		if got, ok := driver.LastRevealedPath(); !ok || got != path {
			t.Fatalf("revealed %q, %t; want %q", got, ok, path)
		}
	}
	if opened, ok := driver.LastOpenedPath(); ok {
		t.Fatalf("revealing opened %q instead of selecting it", opened)
	}
	link := filepath.Join(directory, "shortcut")
	if err := os.Symlink(file, link); err == nil {
		if err := RevealPath(link); err != nil {
			t.Fatal(err)
		}
		if got, _ := driver.LastRevealedPath(); got != link {
			t.Fatalf("selected %q instead of the link %q", got, link)
		}
	}
}

func TestRevealPathRejectsInvalidOrMissingEntriesBeforeDriver(t *testing.T) {
	driver, err := NewDriver(Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	for _, path := range []string{"", "relative.txt", "/tmp/bad\x00name"} {
		if err := RevealPath(path); err == nil {
			t.Fatalf("accepted invalid path %q", path)
		}
	}
	missing := filepath.Join(t.TempDir(), "missing")
	if err := RevealPath(missing); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing path error = %v", err)
	}
	if got, ok := driver.LastRevealedPath(); ok {
		t.Fatalf("invalid path reached the file manager: %q", got)
	}
}

func TestRevealPathAndDriverQueryAreSafeFromBackgroundWorkers(t *testing.T) {
	driver, err := NewDriver(Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	file := filepath.Join(t.TempDir(), "download.txt")
	if err := os.WriteFile(file, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	for range 16 {
		workers.Go(func() {
			for range 10 {
				if err := RevealPath(file); err != nil {
					t.Errorf("reveal: %v", err)
				}
				if got, ok := driver.LastRevealedPath(); !ok || got != file {
					t.Errorf("revealed %q, %t", got, ok)
				}
			}
		})
	}
	workers.Wait()
}

func BenchmarkRevealPathDriver(b *testing.B) {
	driver, err := NewDriver(Config{}, nil)
	if err != nil {
		b.Fatal(err)
	}
	defer driver.Close()
	path := filepath.Join(b.TempDir(), "download.txt")
	if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := RevealPath(path); err != nil {
			b.Fatal(err)
		}
	}
}

// A worker that outlives the window can still call RevealPath after the driver
// closed. It must report that it has nowhere to go, never start a real file
// manager in a process that has been running headless.
func TestRevealPathAfterTheDriverClosedNeverReachesTheFileManager(t *testing.T) {
	driver, err := NewDriver(Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "download.txt")
	if err := os.WriteFile(file, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	driver.Close()
	if err := RevealPath(file); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("reveal after the driver closed = %v, want ErrUnsupported", err)
	}
}

func BenchmarkRevealPathRejected(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if err := RevealPath("relative.txt"); err == nil {
			b.Fatal("accepted relative path")
		}
	}
}
