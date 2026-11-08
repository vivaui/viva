package filewatch

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// open watches dir and returns a function that waits for the next wake.
func open(t *testing.T, dir string) (*Watch, func() Pending) {
	t.Helper()
	woken := make(chan struct{}, 1)
	w, err := Open(dir, func() {
		select {
		case woken <- struct{}{}:
		default:
		}
	})
	if err == ErrUnsupported {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(w.Close)
	return w, func() Pending {
		t.Helper()
		select {
		case <-woken:
		case <-time.After(10 * time.Second):
			t.Fatal("no wake")
		}
		return w.Drain(nil)
	}
}

// waitFor drains until name has been reported, as platforms may deliver one
// change as several wakes.
func waitFor(t *testing.T, next func() Pending, name string) {
	t.Helper()
	for range 20 {
		p := next()
		if p.Resync || slices.Contains(p.Names, name) {
			return
		}
	}
	t.Fatalf("%q never reported", name)
}

func TestReportsCreateModifyReplaceRemove(t *testing.T) {
	dir := t.TempDir()
	_, next := open(t, dir)
	path := filepath.Join(dir, "note.md")
	steps := []struct {
		what string
		do   func() error
	}{
		{"create", func() error { return os.WriteFile(path, []byte("one"), 0o600) }},
		{"modify in place", func() error { return os.WriteFile(path, []byte("two"), 0o600) }},
		{"replace by rename", func() error {
			tmp := filepath.Join(dir, ".note.md.tmp")
			if err := os.WriteFile(tmp, []byte("three"), 0o600); err != nil {
				return err
			}
			return os.Rename(tmp, path)
		}},
		{"remove", func() error { return os.Remove(path) }},
	}
	for _, step := range steps {
		if err := step.do(); err != nil {
			t.Fatalf("%s: %v", step.what, err)
		}
		t.Run(step.what, func(t *testing.T) { waitFor(t, next, "note.md") })
	}
}

func TestIgnoresSubdirectoryContent(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	w, next := open(t, dir)
	if err := os.WriteFile(filepath.Join(sub, "inner.md"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "outer.md"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	waitFor(t, next, "outer.md")
	time.Sleep(100 * time.Millisecond)
	if p := w.Drain(nil); slices.Contains(p.Names, "inner.md") {
		t.Fatalf("subdirectory entry leaked: %v", p.Names)
	}
}

func TestBurstIsCoalescedByName(t *testing.T) {
	dir := t.TempDir()
	var wakes atomic.Int32
	w, err := Open(dir, func() { wakes.Add(1) })
	if err == ErrUnsupported {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	path := filepath.Join(dir, "a.md")
	for i := range 50 {
		if err := os.WriteFile(path, []byte{byte(i)}, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	deadline := time.Now().Add(10 * time.Second)
	for wakes.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(300 * time.Millisecond)
	p := w.Drain(nil)
	n := 0
	for _, name := range p.Names {
		if name == "a.md" {
			n++
		}
	}
	if n > 1 {
		t.Fatalf("a.md reported %d times in one batch", n)
	}
	if got := wakes.Load(); got > 5 {
		t.Fatalf("%d wakes for one drained burst", got)
	}
}

func TestRemovingTheDirectoryAsksForResync(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows refuses to remove a directory that is being watched")
	}
	dir := filepath.Join(t.TempDir(), "watched")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	_, next := open(t, dir)
	if err := os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	for range 20 {
		if next().Resync {
			return
		}
	}
	t.Fatal("no resync")
}

func TestOverflowOfDistinctNamesAsksForResync(t *testing.T) {
	w := &Watch{wake: func() {}}
	for i := 0; i <= MaxPending; i++ {
		w.post(string(rune(0x4e00 + i)))
	}
	p := w.Drain(nil)
	if !p.Resync || len(p.Names) != 0 {
		t.Fatalf("Resync=%v with %d names", p.Resync, len(p.Names))
	}
}

func TestOpenMissingDirectoryFails(t *testing.T) {
	_, err := Open(filepath.Join(t.TempDir(), "missing"), func() {})
	if err == ErrUnsupported {
		t.Skip(err)
	}
	if err == nil {
		t.Fatal("opened a directory that does not exist")
	}
}

func TestCloseStopsWakesAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	var wakes atomic.Int32
	w, err := Open(dir, func() { wakes.Add(1) })
	if err == ErrUnsupported {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	w.Close()
	w.Close()
	if err := os.WriteFile(filepath.Join(dir, "late.md"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	if n := wakes.Load(); n != 0 {
		t.Fatalf("%d wakes after Close", n)
	}
}

func BenchmarkPostDedup(b *testing.B) {
	w := &Watch{wake: func() {}}
	var buf []string
	b.ReportAllocs()
	for b.Loop() {
		w.post("a.md")
		w.post("a.md")
		buf = w.Drain(buf).Names
	}
}

func openTree(t *testing.T, dir string, skip func(string) bool) (*Watch, func() Pending) {
	t.Helper()
	woken := make(chan struct{}, 1)
	w, err := OpenTree(dir, skip, func() {
		select {
		case woken <- struct{}{}:
		default:
		}
	})
	if err == ErrUnsupported {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(w.Close)
	return w, func() Pending {
		t.Helper()
		select {
		case <-woken:
		case <-time.After(10 * time.Second):
			t.Fatal("no wake")
		}
		return w.Drain(nil)
	}
}

func TestTreeReportsNestedEntriesAsSlashPaths(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "a", "b"), 0o700); err != nil {
		t.Fatal(err)
	}
	_, next := openTree(t, dir, nil)
	if err := os.WriteFile(filepath.Join(dir, "a", "b", "deep.md"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	waitFor(t, next, "a/b/deep.md")
}

func TestTreeFollowsADirectoryCreatedLater(t *testing.T) {
	dir := t.TempDir()
	_, next := openTree(t, dir, nil)
	if err := os.MkdirAll(filepath.Join(dir, "new", "inner"), 0o700); err != nil {
		t.Fatal(err)
	}
	waitFor(t, next, "new")
	// Whether the registration came before this write is the race the watch has to
	// close; the write below must be seen either way.
	time.Sleep(200 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(dir, "new", "inner", "late.md"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	waitFor(t, next, "new/inner/late.md")
}

func TestTreeSeesFilesMovedInWithADirectory(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "moved.md"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, next := openTree(t, dir, nil)
	if err := os.Rename(outside, filepath.Join(dir, "arrived")); err != nil {
		t.Skipf("cannot move across volumes: %v", err)
	}
	for range 20 {
		p := next()
		if p.Resync || slices.Contains(p.Names, "arrived/moved.md") {
			return
		}
		if slices.Contains(p.Names, "arrived") {
			return // the directory itself is the news: a consumer lists it
		}
	}
	t.Fatal("the moved-in directory was not reported")
}

func TestTreeLeavesSkippedDirectoriesOut(t *testing.T) {
	dir := t.TempDir()
	for _, sub := range []string{".git", "docs"} {
		if err := os.Mkdir(filepath.Join(dir, sub), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	w, next := openTree(t, dir, func(name string) bool { return strings.HasPrefix(name, ".") })
	if err := os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docs", "a.md"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	waitFor(t, next, "docs/a.md")
	time.Sleep(200 * time.Millisecond)
	for _, name := range w.Drain(nil).Names {
		if strings.HasPrefix(name, ".git") {
			t.Fatalf("skipped directory leaked: %s", name)
		}
	}
}

func TestTreeStillReportsRenamedAndRemovedFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "d"), 0o700); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(dir, "d", "old.md")
	if err := os.WriteFile(old, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, next := openTree(t, dir, nil)
	if err := os.Rename(old, filepath.Join(dir, "d", "new.md")); err != nil {
		t.Fatal(err)
	}
	waitFor(t, next, "d/new.md")
	if err := os.Remove(filepath.Join(dir, "d", "new.md")); err != nil {
		t.Fatal(err)
	}
	waitFor(t, next, "d/new.md")
}

func TestPlainWatchStillIgnoresSubdirectories(t *testing.T) {
	// Guards the non-tree contract on backends that watch recursively underneath.
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	w, next := open(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "sub", "in.md"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "top.md"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	waitFor(t, next, "top.md")
	time.Sleep(150 * time.Millisecond)
	for _, name := range w.Drain(nil).Names {
		if strings.Contains(name, "/") {
			t.Fatalf("nested name %q from a plain watch", name)
		}
	}
}
