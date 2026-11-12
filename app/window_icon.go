package app

import (
	"fmt"
	"runtime"

	"github.com/vivaui/viva/internal/platform"
)

// WindowIcon is the application's square picture, the source of every icon the
// desktop shows for a running window; see Config.Icon. Supply it as the artist
// drew it: a full-bleed square, which each platform shapes to its own convention
// (the macOS tile with its margin, the plain rounded tile elsewhere), or a
// picture whose corners are transparent, which is only scaled.
type WindowIcon struct {
	// Size is the width and height in pixels. 512 or more keeps it sharp where the
	// desktop shows it large.
	Size int
	// RGBA is Size*Size straight (not premultiplied) 8-bit pixels, top row first.
	RGBA []byte
}

// StatusIcon is the picture as a status-area icon of size pixels, in the form
// the platform's status area expects. macOS tints its icons to the menu bar, so
// there it is the picture's mark alone, a template; Windows and Linux show an
// icon as it is, so there it is the rounded tile a window shows, cropped to the
// mark: a status area draws the icon tiny, and the margin the canvas carries for
// a large icon would leave the mark a size smaller than the icons beside it.
func (icon WindowIcon) StatusIcon(size int) StatusIcon {
	if runtime.GOOS == "darwin" {
		mark := icon.platform().Silhouette(size)
		return StatusIcon{Size: mark.Size, RGBA: mark.RGBA, Template: true}
	}
	tile := icon.platform().TileMark(size)
	return StatusIcon{Size: tile.Size, RGBA: tile.RGBA}
}

func (icon WindowIcon) validate() error {
	if icon.Size == 0 && len(icon.RGBA) == 0 {
		return nil
	}
	if !icon.platform().Valid() {
		return fmt.Errorf("app: window icon must be Size*Size*4 bytes with Size between 1 and 1024, got size %d and %d bytes", icon.Size, len(icon.RGBA))
	}
	return nil
}

func (icon WindowIcon) platform() platform.WindowIcon {
	return platform.WindowIcon{Size: icon.Size, RGBA: icon.RGBA}
}
