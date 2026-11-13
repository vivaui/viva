//go:build windows

package app

import (
	"path/filepath"
	"testing"
)

// A path that does not exist still names a file type: the shell icon is the one
// for the extension, which is what a remote listing needs.
func TestFileIconAnswersMissingPathsByExtension(t *testing.T) {
	for _, name := range []string{"missing.txt", "missing.nc", "missing.unknown"} {
		t.Run(name, func(t *testing.T) {
			bitmap, err := FileIcon(filepath.Join(t.TempDir(), name), 32)
			if err != nil {
				t.Fatalf("FileIcon(%q): %v", name, err)
			}
			if bitmap.Width != 32 || bitmap.Height != 32 || len(bitmap.Pix) != 32*32*4 {
				t.Fatalf("icon = %dx%d with %d bytes", bitmap.Width, bitmap.Height, len(bitmap.Pix))
			}
		})
	}
}
