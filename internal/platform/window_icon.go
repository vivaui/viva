package platform

import (
	"image"

	"github.com/vivaui/viva/internal/iconshape"
)

// WindowIcon is the application's square picture, as the application supplied
// it: a full-bleed image, or one with a shape of its own. Each platform shapes
// it to its own convention with Tile when it sets the icon.
type WindowIcon struct {
	// Size is the width and height in pixels.
	Size int
	// RGBA is Size*Size straight (not premultiplied) 8-bit pixels, top row first.
	RGBA []byte
}

// MacInset is the share of the canvas a macOS icon's tile fills.
const MacInset = iconshape.MacInset

// Valid reports whether the icon's pixel data matches its size. The zero icon is
// not valid; it means a window has none.
func (icon WindowIcon) Valid() bool {
	return StatusIcon{Size: icon.Size, RGBA: icon.RGBA}.Valid()
}

// Tile returns the icon as a size x size desktop tile: rounded, and inset by the
// given share of the canvas (1 fills it). An invalid icon gives the zero icon.
func (icon WindowIcon) Tile(size int, inset float64) WindowIcon {
	if !icon.Valid() || size < 1 {
		return WindowIcon{}
	}
	source := &image.NRGBA{Pix: icon.RGBA, Stride: icon.Size * 4, Rect: image.Rect(0, 0, icon.Size, icon.Size)}
	tile := iconshape.Tile(source, size, inset)
	return WindowIcon{Size: size, RGBA: tile.Pix}
}

// TileMark returns the icon as a size x size desktop tile whose mark fills it:
// Mark crops away the canvas margin around the mark before the tile is shaped.
// It is the form a desktop uses where it draws the icon small. An invalid icon
// gives the zero icon.
func (icon WindowIcon) TileMark(size int) WindowIcon {
	if !icon.Valid() || size < 1 {
		return WindowIcon{}
	}
	source := &image.NRGBA{Pix: icon.RGBA, Stride: icon.Size * 4, Rect: image.Rect(0, 0, icon.Size, icon.Size)}
	tile := iconshape.TileMark(source, size)
	return WindowIcon{Size: size, RGBA: tile.Pix}
}

// Silhouette returns the icon's mark as a black shape with alpha, cropped to its
// extent, size x size. An invalid icon gives the zero icon.
func (icon WindowIcon) Silhouette(size int) WindowIcon {
	if !icon.Valid() || size < 1 {
		return WindowIcon{}
	}
	source := &image.NRGBA{Pix: icon.RGBA, Stride: icon.Size * 4, Rect: image.Rect(0, 0, icon.Size, icon.Size)}
	shape := iconshape.Silhouette(source, size)
	return WindowIcon{Size: size, RGBA: shape.Pix}
}
