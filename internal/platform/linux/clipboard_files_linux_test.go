package linux

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/vivaui/viva/internal/platform"
)

// TestClipboardFilesFromX11Owner reads files another program put on the real
// X11 clipboard the two ways file managers do. It needs a display and xclip, as
// the image test next to it does.
func TestClipboardFilesFromX11Owner(t *testing.T) {
	if testing.Short() || os.Getenv("VIVA_LINUX_XCB_SMOKE") == "" {
		t.Skip("set VIVA_LINUX_XCB_SMOKE=1 with an X11 display and xclip")
	}
	xclip, err := exec.LookPath("xclip")
	if err != nil {
		t.Skip("xclip is not installed")
	}
	if ownsClipboardText() {
		t.Fatal("this process owns the clipboard")
	}
	directory := t.TempDir()
	own := func(target, content string) {
		t.Helper()
		path := filepath.Join(directory, "clipboard.txt")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		// xclip forks to serve the selection and keeps its inherited pipes, so
		// its output must go to files, not to a pipe this test would wait on.
		if err := exec.Command(xclip, "-selection", "clipboard", "-t", target, "-i", path).Run(); err != nil {
			t.Fatalf("xclip %s: %v", target, err)
		}
		time.Sleep(200 * time.Millisecond)
	}

	own("x-special/gnome-copied-files", "copy\nfile:///tmp/a%20b.txt\nfile:///tmp/dir\n")
	got, err := ClipboardFiles()
	if err != nil || !slices.Equal(got, []string{"/tmp/a b.txt", "/tmp/dir"}) {
		t.Fatalf("gnome-copied-files: %q, %v", got, err)
	}
	own("text/uri-list", "# a comment\r\nfile:///home/u/%E4%B8%AD.txt\r\n")
	got, err = ClipboardFiles()
	if err != nil || !slices.Equal(got, []string{"/home/u/中.txt"}) {
		t.Fatalf("text/uri-list: %q, %v", got, err)
	}
	own("UTF8_STRING", "file:///tmp/only-text")
	if _, err := ClipboardFiles(); !errors.Is(err, platform.ErrNoClipboardFiles) {
		t.Fatalf("a text clipboard: %v, want ErrNoClipboardFiles", err)
	}
}
