//go:build darwin || windows || linux

package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenPathInBrowserValidatesBeforeAskingTheSystem(t *testing.T) {
	dir := t.TempDir()
	page := filepath.Join(dir, "page.html")
	if err := os.WriteFile(page, []byte("<p>x</p>"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"relative.html", filepath.Join(dir, "missing.html"), dir, page + "\x00"} {
		if err := OpenPathInBrowser(path); err == nil {
			t.Errorf("%q was accepted", path)
		}
	}
	link := filepath.Join(dir, "link.html")
	if err := os.Symlink(page, link); err == nil {
		if err := OpenPathInBrowser(link); err == nil {
			t.Error("a symbolic link was accepted")
		}
	}
}

// TestDriverRecordsOpenPathInBrowserAndStartsNothing pins that the browser
// request is its own record: a page that goes to the browser is neither an
// OpenPath nor an OpenURL, so a test can tell which way the file left.
func TestDriverRecordsOpenPathInBrowserAndStartsNothing(t *testing.T) {
	driver, err := NewDriver(Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	dir := t.TempDir()
	page := filepath.Join(dir, "my page.html")
	if err := os.WriteFile(page, []byte("<p>x</p>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := OpenPathInBrowser(dir); err == nil {
		t.Fatal("a directory was accepted")
	}
	if _, ok := driver.LastOpenedInBrowser(); ok {
		t.Fatal("a refused path reached the driver")
	}
	if err := OpenPathInBrowser(page); err != nil {
		t.Fatal(err)
	}
	if opened, ok := driver.LastOpenedInBrowser(); !ok || opened != page {
		t.Fatalf("LastOpenedInBrowser = %q, %t", opened, ok)
	}
	if _, ok := driver.LastOpenedPath(); ok {
		t.Fatal("OpenPathInBrowser was recorded as OpenPath")
	}
	if _, ok := driver.LastOpenedURL(); ok {
		t.Fatal("OpenPathInBrowser was recorded as OpenURL")
	}
}
