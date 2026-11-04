//go:build darwin

package darwin

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/vivaui/viva/internal/platform"
)

func TestPasteboardFiles(t *testing.T) {
	name := privateName(t)
	if _, err := privatePasteboardFiles(name); !errors.Is(err, platform.ErrNoClipboardFiles) {
		t.Fatalf("empty pasteboard error = %v, want ErrNoClipboardFiles", err)
	}
	dir := t.TempDir()
	want := []string{filepath.Join(dir, "中文 name.txt"), filepath.Join(dir, "folder")}
	if err := os.WriteFile(want[0], []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(want[1], 0o700); err != nil {
		t.Fatal(err)
	}
	if err := putPrivatePasteboardFiles(name, want); err != nil {
		t.Fatal(err)
	}
	got, err := privatePasteboardFiles(name)
	if err != nil {
		t.Fatal(err)
	}
	// The temp directory may be reached through a symlink (/var -> /private/var):
	// the pasteboard hands over the paths as it was given them.
	if !slices.Equal(got, want) {
		t.Fatalf("files = %q, want %q", got, want)
	}
}

func TestPasteboardFilesAreCutAtTheCap(t *testing.T) {
	name := privateName(t)
	paths := make([]string, platform.MaxClipboardFiles+5)
	for index := range paths {
		paths[index] = fmt.Sprintf("/tmp/viva-cap-%d", index)
	}
	if err := putPrivatePasteboardFiles(name, paths); err != nil {
		t.Fatal(err)
	}
	got, err := privatePasteboardFiles(name)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != platform.MaxClipboardFiles {
		t.Fatalf("got %d paths, want the cap %d", len(got), platform.MaxClipboardFiles)
	}
}

func TestPasteboardWithOnlyTextHoldsNoFiles(t *testing.T) {
	name := privateName(t)
	if err := putPrivatePasteboardFiles(name, []string{"/tmp/a"}); err != nil {
		t.Fatal(err)
	}
	// A pasteboard another application filled with text only.
	if err := putPrivatePasteboardImage(name, testPNG(t), false); err != nil {
		t.Fatal(err)
	}
	if _, err := privatePasteboardFiles(name); !errors.Is(err, platform.ErrNoClipboardFiles) {
		t.Fatalf("after the files were replaced: %v, want ErrNoClipboardFiles", err)
	}
}
