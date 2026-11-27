//go:build darwin || windows || linux

package app

import (
	"errors"
	"io"
	"runtime"
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func newPromiseDriver(t *testing.T) *Driver {
	t.Helper()
	driver, err := NewDriver(Config{Title: "promises", Width: 100, Height: 100, Paint: func(*gfx.Canvas, gfx.Size) {}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { driver.Close() })
	return driver
}

func promiseTree(t *testing.T) []FilePromise {
	t.Helper()
	file := func(name, value string) FilePromise {
		return FilePromise{Name: name, Size: int64(len(value)), Open: func() (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(value)), nil
		}}
	}
	return []FilePromise{
		file("report.pdf", "%PDF"),
		{Name: "folder", Directory: true, List: func() ([]FilePromise, error) {
			return []FilePromise{file("notes.txt", "hello")}, nil
		}},
	}
}

// TestClipboardFilePromisesAreReadWhenPasted checks the driver's view of the
// promise: the clipboard names no paths, the content is still the caller's to
// produce, and replacing the clipboard releases the promise once.
func TestClipboardFilePromisesAreReadWhenPasted(t *testing.T) {
	driver := newPromiseDriver(t)
	released := 0
	files := promiseTree(t)
	if err := SetClipboardFilePromises(files, func() { released++ }); err != nil {
		t.Fatal(err)
	}
	if got := driver.ClipboardFilePromises(); len(got) != 2 || got[0].Name != "report.pdf" || !got[1].Directory {
		t.Fatalf("the clipboard promises %+v", got)
	}
	if _, err := ClipboardFiles(); !errors.Is(err, ErrNoFiles) {
		t.Fatalf("a promised clipboard names paths: %v", err)
	}
	reader, err := driver.ClipboardFilePromises()[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(reader)
	reader.Close()
	if err != nil || string(body) != "%PDF" {
		t.Fatalf("the promised file reads %q, %v", body, err)
	}

	// Something else copied replaces the promise, as it does on a real
	// clipboard; the old content is then no longer needed.
	driver.SetClipboard("just text")
	if released != 1 {
		t.Fatalf("released %d times, want 1", released)
	}
	if got := driver.ClipboardFilePromises(); len(got) != 0 {
		t.Fatalf("the clipboard still promises %d files", len(got))
	}
	driver.SetClipboard("again")
	if released != 1 {
		t.Fatalf("released %d times after the promise was gone, want 1", released)
	}
}

// TestClipboardFilePromisesRefuseUnsafeTrees keeps a name that cannot be one
// path component, a duplicate sibling, or an empty list from reaching a
// platform's clipboard.
func TestClipboardFilePromisesRefuseUnsafeTrees(t *testing.T) {
	driver := newPromiseDriver(t)
	file := func(name string) FilePromise {
		return FilePromise{Name: name, Open: func() (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("")), nil
		}}
	}
	for name, files := range map[string][]FilePromise{
		"empty":           nil,
		"separator":       {file("a/b")},
		"parent":          {file("..")},
		"duplicate":       {file("a"), file("a")},
		"missing content": {{Name: "a"}},
	} {
		if err := SetClipboardFilePromises(files, nil); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if got := driver.ClipboardFilePromises(); len(got) != 0 {
		t.Fatalf("a refused tree reached the clipboard: %+v", got)
	}
}

// TestClipboardFilePromisesPromiseTheSameTreeTwice checks that replacing a
// promise with another releases only the first.
func TestClipboardFilePromisesPromiseTheSameTreeTwice(t *testing.T) {
	driver := newPromiseDriver(t)
	released := 0
	if err := SetClipboardFilePromises(promiseTree(t), func() { released++ }); err != nil {
		t.Fatal(err)
	}
	if err := SetClipboardFilePromises(promiseTree(t), nil); err != nil {
		t.Fatal(err)
	}
	if released != 1 {
		t.Fatalf("released %d times, want 1", released)
	}
	if len(driver.ClipboardFilePromises()) != 2 {
		t.Fatal("the second promise is not on the clipboard")
	}
}

// TestClipboardFilePromisesNeedAPlatform checks that a target without the
// capability says so instead of silently leaving an eager copy on the
// clipboard, on platforms whose file managers read paths.
func TestClipboardFilePromisesNeedAPlatform(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows takes promised files")
	}
	if Supports(CapabilityClipboardFilePromise) {
		t.Fatal("this platform reports clipboard file promises it cannot take")
	}
	if err := SetClipboardFilePromises(promiseTree(t), nil); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("SetClipboardFilePromises = %v, want ErrUnsupported", err)
	}
}
