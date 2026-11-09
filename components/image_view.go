package components

import (
	"github.com/vivaui/viva/gfx"
	"math"
)

// ImageViewOptions holds caller-owned zoom/pan state. Zoom zero fits the image;
// one is actual size (one bitmap pixel per device pixel), independent of DPI.
type ImageViewOptions struct {
	Bitmap gfx.Bitmap
	Zoom   float32
	Pan    gfx.Point
}
type ImageViewGeometry struct {
	Image gfx.Rect
	Scale float32
}

// PaintImageView paints an immutable bitmap clipped to bounds. File decoding,
// byte/pixel limits and document lifetime belong to the application.
func PaintImageView(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, options ImageViewOptions) ImageViewGeometry {
	if canvas == nil || bounds.Width <= 0 || bounds.Height <= 0 || options.Bitmap.Width <= 0 || options.Bitmap.Height <= 0 {
		return ImageViewGeometry{}
	}
	canvas.Fill(bounds, theme.Muted)
	w, h := float32(options.Bitmap.Width), float32(options.Bitmap.Height)
	scale := min(bounds.Width/w, bounds.Height/h)
	if options.Zoom > 0 {
		scale = options.Zoom / canvas.PixelScale()
	}
	width, height := w*scale, h*scale
	pan := options.Pan
	pan.X = max(min(pan.X, max((width-bounds.Width)/2, 0)), -max((width-bounds.Width)/2, 0))
	pan.Y = max(min(pan.Y, max((height-bounds.Height)/2, 0)), -max((height-bounds.Height)/2, 0))
	image := gfx.Rect{X: bounds.X + (bounds.Width-width)/2 + pan.X, Y: bounds.Y + (bounds.Height-height)/2 + pan.Y, Width: width, Height: height}
	if options.Zoom == 1 {
		// Actual-size pixels must also begin on a device pixel, otherwise a
		// half-pixel origin blurs a 1:1 image at fractional display scales.
		s := float64(canvas.PixelScale())
		image.X = float32(math.Round(float64(image.X)*s) / s)
		image.Y = float32(math.Round(float64(image.Y)*s) / s)
		if width >= bounds.Width {
			image.X = min(image.X, float32(math.Floor(float64(bounds.X)*s)/s))
			image.X = max(image.X, float32(math.Ceil(float64(bounds.X+bounds.Width-width)*s)/s))
		}
		if height >= bounds.Height {
			image.Y = min(image.Y, float32(math.Floor(float64(bounds.Y)*s)/s))
			image.Y = max(image.Y, float32(math.Ceil(float64(bounds.Y+bounds.Height-height)*s)/s))
		}
	}
	canvas.Clip(bounds)
	canvas.DrawBitmap(options.Bitmap, image)
	canvas.Unclip()
	return ImageViewGeometry{Image: image, Scale: scale}
}
