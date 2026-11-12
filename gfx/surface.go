package gfx

import (
	"math"
	"sync/atomic"
)

// Surface is a retained, opaque BGRA picture that changes in place: a remote
// desktop, a video frame, a canvas an application paints pixels into. A Bitmap
// is immutable, so changing one pixel of a large Bitmap means a new Version and
// a new upload of every byte. A Surface keeps its pixels in square tiles, each
// its own Bitmap with its own Version, so Update uploads only the tiles the
// changed rectangle touches. A 1920x1080 picture is about 8 MB; a 64x64 patch of
// it stages 256 KB.
//
// A Surface is not safe for concurrent use. Update runs on the goroutine that
// paints, and the pixel source is copied before Update returns, so the caller
// may keep writing to it.
type Surface struct {
	base          uint64 // bitmap IDs of the tiles are base+index
	width, height int
	edge          int // tile edge in pixels
	columns, rows int
	tiles         []surfaceTile
}

type surfaceTile struct {
	pix     []byte // tight: edge*4 bytes per row, only the tile's own rows
	width   int
	height  int
	version uint32
}

// Tiles are 256 pixels square until the picture needs more than surfaceMaxTiles
// of them, then 512 and 1024: every tile is one native texture and one draw, so
// their count stays bounded for any picture size.
const (
	surfaceMaxTiles = 64
	surfaceBaseEdge = 256
)

// surfaceIDs hands out bitmap identities. They are in a range of their own,
// below the 1<<62 an application might pick for a backdrop and above the small
// counters a library numbers its images with.
var surfaceIDs atomic.Uint64

const surfaceIDSpace = 1 << 61

// Size is the picture's width and height in pixels, zero before Resize.
func (surface *Surface) Size() (width, height int) { return surface.width, surface.height }

// Resize makes the picture width x height pixels and clears it to opaque
// black. A size that is not positive, or too large to address, empties it.
func (surface *Surface) Resize(width, height int) {
	if width <= 0 || height <= 0 || width > 1<<15 || height > 1<<15 {
		width, height = 0, 0
	}
	edge := surfaceBaseEdge
	for edge < 1024 && ((width+edge-1)/edge)*((height+edge-1)/edge) > surfaceMaxTiles {
		edge *= 2
	}
	surface.width, surface.height, surface.edge = width, height, edge
	if width == 0 {
		surface.columns, surface.rows, surface.tiles = 0, 0, surface.tiles[:0]
		return
	}
	surface.columns, surface.rows = (width+edge-1)/edge, (height+edge-1)/edge
	count := surface.columns * surface.rows
	// New identities: a tile of the old picture must never be mistaken for one
	// of the new, whatever its version says.
	surface.base = surfaceIDSpace + surfaceIDs.Add(uint64(count)) - uint64(count)
	if cap(surface.tiles) < count {
		surface.tiles = make([]surfaceTile, count)
	}
	surface.tiles = surface.tiles[:count]
	for index := range surface.tiles {
		row, column := index/surface.columns, index%surface.columns
		tile := &surface.tiles[index]
		tile.width = min(edge, width-column*edge)
		tile.height = min(edge, height-row*edge)
		size := tile.width * tile.height * 4
		if cap(tile.pix) < size {
			tile.pix = make([]byte, size)
		}
		tile.pix = tile.pix[:size]
		for offset := 0; offset < size; offset += 4 {
			tile.pix[offset], tile.pix[offset+1], tile.pix[offset+2], tile.pix[offset+3] = 0, 0, 0, 255
		}
		tile.version = 1
	}
}

// Update copies a width x height rectangle of BGRA pixels, whose top-left
// corner is at (x, y) of the picture, from pix, whose rows are stride bytes
// apart and start at pix[0]. The alpha byte is set to opaque, so a source whose
// fourth byte is padding (RFB, RDP, most capture APIs) can be passed as it is.
// The part of the rectangle outside the picture is ignored. Only the tiles it
// touches get a new Version.
func (surface *Surface) Update(x, y, width, height int, pix []byte, stride int) {
	if width <= 0 || height <= 0 || stride < width*4 || surface.width == 0 {
		return
	}
	// Clip, moving the source origin with the destination.
	if x < 0 {
		width, pix, x = width+x, pix[min(-x*4, len(pix)):], 0
	}
	if y < 0 {
		height += y
		pix, y = pix[min(-y*stride, len(pix)):], 0
	}
	width, height = min(width, surface.width-x), min(height, surface.height-y)
	if width <= 0 || height <= 0 || (height-1)*stride+width*4 > len(pix) {
		return
	}
	edge := surface.edge
	for row := y / edge; row <= (y+height-1)/edge; row++ {
		top, bottom := max(y, row*edge), min(y+height, (row+1)*edge)
		for column := x / edge; column <= (x+width-1)/edge; column++ {
			left, right := max(x, column*edge), min(x+width, (column+1)*edge)
			tile := &surface.tiles[row*surface.columns+column]
			tileStride := tile.width * 4
			for line := top; line < bottom; line++ {
				source := pix[(line-y)*stride+(left-x)*4:][:(right-left)*4]
				target := tile.pix[(line-row*edge)*tileStride+(left-column*edge)*4:][:len(source)]
				copy(target, source)
				for alpha := 3; alpha < len(target); alpha += 4 {
					target[alpha] = 255
				}
			}
			tile.version++
		}
	}
}

// Draw paints the whole picture scaled to bounds. Each tile's edges are rounded
// to whole pixels on the way, so neighbours meet exactly at any scale: no seam
// of the background shows between them and no pixel is drawn twice.
func (surface *Surface) Draw(canvas *Canvas, bounds Rect) {
	if surface.width == 0 || bounds.Width <= 0 || bounds.Height <= 0 {
		return
	}
	scaleX, scaleY := bounds.Width/float32(surface.width), bounds.Height/float32(surface.height)
	edge := surface.edge
	for index := range surface.tiles {
		tile := &surface.tiles[index]
		row, column := index/surface.columns, index%surface.columns
		left, top := column*edge, row*edge
		x0, x1 := snapEdge(bounds.X+float32(left)*scaleX), snapEdge(bounds.X+float32(left+tile.width)*scaleX)
		y0, y1 := snapEdge(bounds.Y+float32(top)*scaleY), snapEdge(bounds.Y+float32(top+tile.height)*scaleY)
		canvas.DrawBitmap(Bitmap{
			Pix: tile.pix, Width: tile.width, Height: tile.height, Stride: tile.width * 4,
			ID: surface.base + uint64(index), Version: tile.version,
		}, Rect{X: x0, Y: y0, Width: x1 - x0, Height: y1 - y0})
	}
}

// DrawPixels paints the picture one picture pixel to one device pixel, with its
// top-left corner on the device pixel nearest origin, and returns the logical
// rectangle it covers. A picture the size of the window's pixels, as a remote
// desktop that follows its window is, then reaches the screen unscaled at any
// display scale: Draw rounds tile edges to logical pixels, which at 125% or
// 150% resamples every tile by up to a pixel and blurs the text in it. The
// canvas must know the display scale (SetPixelScale); without it a device pixel
// is taken to be half a logical one.
func (surface *Surface) DrawPixels(canvas *Canvas, origin Point) Rect {
	scale := canvas.PixelScale()
	left, top := deviceEdge(origin.X, scale), deviceEdge(origin.Y, scale)
	covered := Rect{
		X: float32(left) / scale, Y: float32(top) / scale,
		Width: float32(surface.width) / scale, Height: float32(surface.height) / scale,
	}
	edge := surface.edge
	for index := range surface.tiles {
		tile := &surface.tiles[index]
		row, column := index/surface.columns, index%surface.columns
		before := len(canvas.images)
		canvas.DrawBitmap(Bitmap{
			Pix: tile.pix, Width: tile.width, Height: tile.height, Stride: tile.width * 4,
			ID: surface.base + uint64(index), Version: tile.version,
		}, Rect{
			X: float32(left+column*edge) / scale, Y: float32(top+row*edge) / scale,
			Width: float32(tile.width) / scale, Height: float32(tile.height) / scale,
		})
		if len(canvas.images) > before {
			canvas.images[before].DevicePixels = true
		}
	}
	return covered
}

// deviceEdge is the device pixel nearest a logical coordinate.
func deviceEdge(value, scale float32) int {
	return int(math.Floor(float64(value)*float64(scale) + 0.5))
}

// snapEdge rounds a coordinate to the nearest whole pixel, as the painter does
// for the bounds it receives.
func snapEdge(value float32) float32 { return float32(math.Floor(float64(value) + 0.5)) }
