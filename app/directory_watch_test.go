package app

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

func TestWatchDirectoryWakesAndReportsChanges(t *testing.T) {
	dir := t.TempDir()
	var wakes atomic.Int32
	watch, err := WatchDirectory(dir, func() { wakes.Add(1) })
	if errors.Is(err, ErrUnsupported) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer watch.Close()
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	var names []string
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) && !slices.Contains(names, "a.md") {
		time.Sleep(10 * time.Millisecond)
		names = watch.Changes(names[:0]).Names
	}
	if !slices.Contains(names, "a.md") {
		t.Fatal("the change was not reported")
	}
	if wakes.Load() == 0 {
		t.Fatal("the wake function was not called")
	}
}

func TestWatchDirectoryNeedsAWakeFunction(t *testing.T) {
	if _, err := WatchDirectory(t.TempDir(), nil); err == nil {
		t.Fatal("a nil wake function was accepted")
	}
}

func TestWatchTreeReportsNestedChangesAndSkips(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "a"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".hidden"), 0o700); err != nil {
		t.Fatal(err)
	}
	watch, err := WatchTree(dir, func(name string) bool { return name[0] == '.' }, func() {})
	if errors.Is(err, ErrUnsupported) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer watch.Close()
	os.WriteFile(filepath.Join(dir, ".hidden", "x.md"), []byte("x"), 0o600)
	os.WriteFile(filepath.Join(dir, "a", "n.md"), []byte("x"), 0o600)
	var names []string
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) && !slices.Contains(names, "a/n.md") {
		time.Sleep(10 * time.Millisecond)
		names = append(names, watch.Changes(nil).Names...)
	}
	if !slices.Contains(names, "a/n.md") || slices.Contains(names, ".hidden/x.md") {
		t.Fatalf("names = %v", names)
	}
}
