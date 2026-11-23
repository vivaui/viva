package vulkan

import "github.com/vivaui/viva/internal/platform"

// ColorSpan is the columns of an LCD grid label's texture, left to right in
// device pixels, that are text of one color. The texture's coverage is the same
// for every color, so the renderer draws it once per span, with the span's
// color, scissored to its columns.
type ColorSpan struct {
	Left, Right int
	Color       [4]float32
}

// TextRasterizer draws the glyphs of a text label into the coverage bitmap of its
// texture. The renderer owns the texture, its upload and its placement; what draws
// the glyphs is the platform's text engine (Pango on a Linux desktop, the system's
// Paint on Android), which the platform gives the renderer as a factory.
type TextRasterizer interface {
	// Rasterize draws the device columns from skip to skip+width of the label, height
	// rows tall, at scale. With allowLCD the result may be the coverage of each
	// color channel (lcd reports it), to be blended with the label's color by the
	// renderer's LCD pipeline.
	Rasterize(label platform.TextLabel, scale float32, skip, width, height int, allowLCD bool) (pixels []byte, lcd bool, err error)
	// Spans are the colors of the last LCD grid label rasterized, valid until the
	// next call of Rasterize; empty for a label that is all in label.Color.
	Spans() []ColorSpan
	// Close releases what the rasterizer holds.
	Close()
}

// NewTextRasterizer makes the rasterizer of one renderer.
type NewTextRasterizer func() (TextRasterizer, error)
