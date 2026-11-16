//go:build darwin

package app

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// finderApp is a bundle every macOS has, with an icon of its own.
const finderApp = "/System/Library/CoreServices/Finder.app"

func TestFileIconShowsAnApplicationItsOwnIcon(t *testing.T) {
	const pixels = 32
	bundle, err := FileIcon(finderApp, pixels)
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Width != pixels || bundle.Height != pixels || bundle.Stride != pixels*4 || len(bundle.Pix) != pixels*pixels*4 {
		t.Fatalf("bitmap = %dx%d stride %d, %d bytes", bundle.Width, bundle.Height, bundle.Stride, len(bundle.Pix))
	}
	opaque := 0
	for offset := 0; offset < len(bundle.Pix); offset += 4 {
		b, g, r, a := bundle.Pix[offset], bundle.Pix[offset+1], bundle.Pix[offset+2], bundle.Pix[offset+3]
		if b > a || g > a || r > a {
			t.Fatalf("pixel %d is not premultiplied: %d,%d,%d over alpha %d", offset/4, b, g, r, a)
		}
		if a == 255 {
			opaque++
		}
	}
	if opaque < pixels*pixels/4 {
		t.Fatalf("only %d of %d pixels are opaque: the icon is blank", opaque, pixels*pixels)
	}
	// A folder that is not a bundle gets the generic folder icon, which is what
	// the file list showed for every application before.
	folder, err := FileIcon(t.TempDir(), pixels)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(bundle.Pix, folder.Pix) {
		t.Fatal("an application and a plain folder have the same icon")
	}
}

func TestFileIconAnswersFromAnyGoroutineAndForAMissingPath(t *testing.T) {
	var wait sync.WaitGroup
	failures := make(chan error, 8)
	for range 8 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := FileIcon(finderApp, 16)
			failures <- err
		}()
	}
	wait.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := FileIcon("/no/such/folder/Missing.app", 16); err != nil {
		t.Fatalf("a missing path has the icon of its extension, got %v", err)
	}
}

func TestFileIconAnswersAMissingPathWithItsExtensionIcon(t *testing.T) {
	// FileIcon documents that a path need not exist: its icon is the one for
	// its extension. The reference is a real empty file of that type, which the
	// system already answers correctly, so the comparison holds whatever the
	// system associates the type with.
	const pixels = 32
	dir := t.TempDir()
	real := filepath.Join(dir, "document.txt")
	if err := os.WriteFile(real, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	existing, err := FileIcon(real, pixels)
	if err != nil {
		t.Fatal(err)
	}
	missing, err := FileIcon(filepath.Join(dir, "never-created", "document.txt"), pixels)
	if err != nil {
		t.Fatalf("a missing path has the icon of its extension, got %v", err)
	}
	if !bytes.Equal(existing.Pix, missing.Pix) {
		t.Fatal("a missing path got a different icon from the same type's real file")
	}
}

func BenchmarkFileIcon(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := FileIcon(finderApp, 32); err != nil {
			b.Fatal(err)
		}
	}
}
