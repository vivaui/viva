//go:build windows

package app

import (
	"bytes"
	"path/filepath"
	"testing"
)

// A path that does not exist still names a file type: the shell icon is the one
// for the extension, which is what a remote listing needs.
func TestFileIconAnswersMissingPathsByExtension(t *testing.T) {
	folder := t.TempDir()
	icons := map[string][]byte{}
	for _, name := range []string{"missing.txt", "missing.nc", "missing.unknown"} {
		t.Run(name, func(t *testing.T) {
			bitmap, err := FileIcon(filepath.Join(folder, name), 32)
			if err != nil {
				t.Fatalf("FileIcon(%q): %v", name, err)
			}
			if bitmap.Width != 32 || bitmap.Height != 32 || len(bitmap.Pix) != 32*32*4 {
				t.Fatalf("icon = %dx%d with %d bytes", bitmap.Width, bitmap.Height, len(bitmap.Pix))
			}
			for offset := 0; offset < len(bitmap.Pix); offset += 4 {
				b, g, r, a := bitmap.Pix[offset], bitmap.Pix[offset+1], bitmap.Pix[offset+2], bitmap.Pix[offset+3]
				if a != 0 && (b > a || g > a || r > a) {
					t.Fatalf("pixel %d is not premultiplied: %d,%d,%d over alpha %d", offset/4, b, g, r, a)
				}
			}
			icons[name] = bitmap.Pix
		})
	}
	// The icon must come from the extension, not from a constant blank bitmap: a
	// text document and a file of an unknown type cannot look the same.
	text, unknown := icons["missing.txt"], icons["missing.unknown"]
	if text == nil || unknown == nil {
		return // the subtest above already reported why
	}
	if bytes.Equal(text, unknown) {
		t.Fatal("a .txt file and a file of an unknown type have the same icon")
	}
}
