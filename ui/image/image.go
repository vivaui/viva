// Package image is the opt-in page control for an image of package
// components. A page declares an image with the caller's decoded bitmap and a
// viewport, then chooses how the image is scaled:
//
//	picture := image.New()
//	...
//	picture.Reset().Bitmap(m.bitmap).Fit()
//	image.View(c, "Fit preview", picture).FillWidth().Height(220)
//
//	picture.Reset().Bitmap(m.bitmap).Actual()
//	image.View(c, "Actual pixels", picture).Size(160, 100)
//
//	picture.Reset().Bitmap(m.bitmap).Zoom(m.zoom).Pan(m.pan)
//	image.View(c, "Zoom and pan", picture).FillWidth().Height(220)
//
// The caller owns the decoded pixels, their lifetime and the viewport size; the
// wrapper owns the control's handlers and draw function. File decoding, byte
// and pixel limits and document lifetime remain the application's. There is no
// hidden loading or animation: the bitmap is painted where the caller put it.
//
// Fit is the default and shows the whole image inside the viewport. Actual
// shows one bitmap pixel per device pixel, independent of the display scale.
// Zoom sets an explicit scale; Pan moves the scaled image inside the viewport.
// The component clamps Pan so it never exposes empty space.
//
// Pointer, Key and Scroll are the caller's interaction callbacks. The wrapper
// routes the events aimed at the view to them, so a page can drag to pan or
// handle the wheel to zoom without reproducing the control's input plumbing.
// Clicked reports a click after View, like the component views do.
//
// It is a separate package so an application that does not import it pays
// nothing for it. Keep an Image in the model beside its bitmap: its handlers
// and draw function are made once, so a warm paint allocates nothing. Do not
// copy one after it has been painted.
package image

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Image is one image's caller-owned values and page control. Configure it with
// the chainable methods, then view it. A new Image fits its bitmap; the zero
// value paints nothing.
type Image struct {
	bitmap gfx.Bitmap
	zoom   float32
	pan    gfx.Point

	// Pointer receives the presses, moves and releases over the view, such as a
	// drag that pans the image. A nil callback leaves them to the page.
	Pointer func(gfx.PointerEvent) gfx.EventResult
	// Key receives the keys pressed while the view has the focus.
	Key func(gfx.KeyEvent) gfx.EventResult
	// Scroll receives the wheel over the view, such as a zoom gesture. A nil
	// callback leaves the wheel to the page.
	Scroll func(gfx.ScrollEvent) gfx.EventResult

	handlers ui.Handlers
	drawFn   func(*ui.DrawContext)
	prepared bool
	id       gfx.ID
	clicked  gfx.ID
	hadClick bool
}

// New returns an empty image view that fits its bitmap in the viewport.
func New() *Image { return new(Image) }

// Reset clears the bitmap, zoom and pan, returning the image to fit. The
// interaction callbacks are kept, so a page that reconfigures the view every
// paint does not lose its input wiring.
func (i *Image) Reset() *Image {
	i.bitmap = gfx.Bitmap{}
	i.zoom = 0
	i.pan = gfx.Point{}
	i.id = gfx.ID{}
	i.clicked = gfx.ID{}
	i.hadClick = false
	return i
}

// Bitmap is the caller-decoded, immutable picture painted in the viewport.
// The caller owns Pix and must keep it unchanged while a canvas can retain it.
func (i *Image) Bitmap(bitmap gfx.Bitmap) *Image { i.bitmap = bitmap; return i }

// Fit shows the whole image inside the viewport, centered and with its original
// proportions. It is the default and clears any explicit zoom and pan.
func (i *Image) Fit() *Image {
	i.zoom = 0
	i.pan = gfx.Point{}
	return i
}

// Actual shows one bitmap pixel per device pixel, independent of the display
// scale, like the component's Zoom 1. It keeps the current pan.
func (i *Image) Actual() *Image { i.zoom = 1; return i }

// Zoom sets an explicit scale. A zoom of one is Actual; zero or less fits the
// viewport. It keeps the current pan.
func (i *Image) Zoom(zoom float32) *Image {
	if zoom < 0 {
		zoom = 0
	}
	i.zoom = zoom
	return i
}

// Pan offsets the scaled image inside the viewport. The component clamps it so
// the viewport is never left with empty space beside an oversized image.
func (i *Image) Pan(pan gfx.Point) *Image { i.pan = pan; return i }

// View declares the image at c. label names it for assistive technology. The
// caller must give the box its viewport dimensions: Size, FillWidth with
// Height, or the layout of the page around it.
//
// Declare the view on its own, not inside a Memo.
func View(c ui.C, label string, image *Image) ui.Box {
	if image == nil {
		panic("image: View needs its Image")
	}
	image.prepare()
	return ui.Control(c, label, &image.handlers).Draw(image.drawFn)
}

// Clicked reports whether the paint just made delivered a click on the view. It
// is false in ordinary paints. Ask it after View.
func (i *Image) Clicked() bool {
	return i.hadClick && !i.id.IsZero() && i.clicked == i.id
}

func (i *Image) prepare() {
	if i.prepared {
		return
	}
	i.prepared = true
	i.handlers.Role = gfx.RoleGroup
	i.handlers.Pointer = i.pointer
	i.handlers.Key = i.key
	i.handlers.Scroll = i.scroll
	i.drawFn = i.draw
}

func (i *Image) pointer(event gfx.PointerEvent) gfx.EventResult {
	if i.Pointer == nil {
		return gfx.EventResult{}
	}
	return i.Pointer(event)
}

func (i *Image) key(event gfx.KeyEvent) gfx.EventResult {
	if i.Key == nil {
		return gfx.EventResult{}
	}
	return i.Key(event)
}

func (i *Image) scroll(event gfx.ScrollEvent) gfx.EventResult {
	if i.Scroll == nil {
		return gfx.EventResult{}
	}
	return i.Scroll(event)
}

func (i *Image) draw(d *ui.DrawContext) {
	i.id = d.Node.ID
	i.clicked, i.hadClick = d.Canvas.ClickTarget()
	components.PaintImageView(d.Canvas, *d.Theme, d.Rect, components.ImageViewOptions{
		Bitmap: i.bitmap,
		Zoom:   i.zoom,
		Pan:    i.pan,
	})
}
