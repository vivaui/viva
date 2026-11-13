package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/vivaui/viva/gfx"
)

// MaxFileIconPixels is the largest side FileIcon draws.
const MaxFileIconPixels = 512

// FileIcon returns the icon the system's file manager shows for the file or
// folder at path, drawn pixels wide and tall as premultiplied BGRA with the top
// row first, ready for gfx.Canvas.DrawBitmap. It asks the system, so an
// application bundle has its own icon and a document the icon of its type; the
// path need not exist, and then the icon is the one for its extension.
//
// The bitmap's ID is zero. A caller that keeps the bitmap picks an ID not used
// by any other bitmap it draws, because the renderer identifies unchanged
// content by it, and a zero ID is uploaded again every frame. Nothing is
// retained by Viva.
//
// It reads the system's icon database and may touch the disk, so call it from a
// goroutine, never from a paint. It is safe from any goroutine. The path must be
// absolute. macOS reads the icon Finder shows and Windows the one the shell
// gives, an existing file or folder's own icon included; elsewhere it returns
// ErrUnsupported and the caller keeps whatever glyph it draws for a file of that
// kind.
func FileIcon(path string, pixels int) (gfx.Bitmap, error) {
	if strings.IndexByte(path, 0) >= 0 {
		return gfx.Bitmap{}, fmt.Errorf("app: file icon path contains NUL")
	}
	if !filepath.IsAbs(path) {
		return gfx.Bitmap{}, fmt.Errorf("app: file icon path %q is not absolute", path)
	}
	if pixels < 1 || pixels > MaxFileIconPixels {
		return gfx.Bitmap{}, fmt.Errorf("app: file icon size %d is outside 1..%d", pixels, MaxFileIconPixels)
	}
	pix, err := nativeFileIcon(path, pixels)
	if err != nil {
		return gfx.Bitmap{}, err
	}
	return gfx.Bitmap{Pix: pix, Width: pixels, Height: pixels, Stride: pixels * 4}, nil
}
