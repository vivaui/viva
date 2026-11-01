// Package raster draws Viva's retained scene on the CPU, without a GPU, a window system or cgo.
//
// It takes the same data the GPU backends take (render.Rectangle instances in paint order, and
// label images the platform's text engine has rasterized) and produces the pixels those backends
// would: a B8G8R8A8 unorm target blended in raw, non-linear space, premultiplied alpha, pixel
// centers sampled the way a GPU does. It is the renderer for machines with no hardware GPU, for
// headless builds, and a reference for the GPU paths.
//
// Nothing in this package knows a window, a display server or an operating system.
package raster

import "github.com/vivaui/viva/internal/render"

// Canvas is a target of device pixels: premultiplied alpha, four bytes per pixel in the order
// blue, green, red, alpha (little-endian ARGB32, the byte order of cairo's ARGB32, Wayland's
// ARGB8888 and a Vulkan B8G8R8A8 image).
type Canvas struct {
	Pix    []byte
	Stride int // bytes from one row to the next
	Width  int
	Height int
}

// NewCanvas returns a zeroed (transparent) canvas of width x height device pixels.
func NewCanvas(width, height int) *Canvas {
	c := &Canvas{}
	c.Resize(width, height)
	return c
}

// Resize makes the canvas width x height, zeroed. It keeps the pixel storage when it is big
// enough and releases it when the canvas shrinks to under a quarter of it, so a window that was
// once large does not hold its memory for good.
func (c *Canvas) Resize(width, height int) {
	width, height = max(width, 0), max(height, 0)
	need := width * height * 4
	switch {
	case cap(c.Pix) < need || cap(c.Pix) > 4*need && cap(c.Pix) > 1<<16:
		c.Pix = make([]byte, need)
	default:
		c.Pix = c.Pix[:need]
		clear(c.Pix)
	}
	c.Stride, c.Width, c.Height = width*4, width, height
}

// Bounds is the whole canvas as a scissor.
func (c *Canvas) Bounds() render.Scissor {
	return render.Scissor{Right: c.Width, Bottom: c.Height}
}

// intersect is the part of a that is inside b.
func intersect(a, b render.Scissor) render.Scissor {
	return render.Scissor{Left: max(a.Left, b.Left), Top: max(a.Top, b.Top), Right: min(a.Right, b.Right), Bottom: min(a.Bottom, b.Bottom)}
}

func empty(s render.Scissor) bool { return s.Right <= s.Left || s.Bottom <= s.Top }

// unorm8 converts a value in [0, 1] to the nearest of 256 levels, as a GPU stores a unorm target.
func unorm8(v float32) byte {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 255
	}
	return byte(v*255 + 0.5)
}

// over blends premultiplied (r, g, b, a), each in [0, 1], over the pixel at pix[i:i+4]: the
// "one, one minus source alpha" blend of the GPU backends, rounded to the nearest level.
func over(pix []byte, i int, r, g, b, a float32) {
	if a >= 1 {
		pix[i], pix[i+1], pix[i+2], pix[i+3] = unorm8(b), unorm8(g), unorm8(r), 255
		return
	}
	k := 1 - a
	const inv = 1.0 / 255
	pix[i] = unorm8(b + float32(pix[i])*inv*k)
	pix[i+1] = unorm8(g + float32(pix[i+1])*inv*k)
	pix[i+2] = unorm8(r + float32(pix[i+2])*inv*k)
	pix[i+3] = unorm8(a + float32(pix[i+3])*inv*k)
}

// fill writes the premultiplied color over the pixels of s (alpha 1 replaces, otherwise blends).
func (c *Canvas) fill(s render.Scissor, r, g, b, a float32) {
	if empty(s) {
		return
	}
	if a >= 1 {
		px := [4]byte{unorm8(b), unorm8(g), unorm8(r), 255}
		first := c.Pix[s.Top*c.Stride+s.Left*4:]
		row := first[:(s.Right-s.Left)*4]
		for i := 0; i < len(row); i += 4 {
			copy(row[i:i+4], px[:])
		}
		for y := s.Top + 1; y < s.Bottom; y++ {
			copy(c.Pix[y*c.Stride+s.Left*4:], row)
		}
		return
	}
	for y := s.Top; y < s.Bottom; y++ {
		i := y*c.Stride + s.Left*4
		for x := s.Left; x < s.Right; x++ {
			over(c.Pix, i, r, g, b, a)
			i += 4
		}
	}
}
