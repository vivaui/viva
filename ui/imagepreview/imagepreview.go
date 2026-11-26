// Package imagepreview is the opt-in page control for looking at a picture
// larger than the place it is shown in: a dialog with the image, zoom buttons
// and a pan by dragging. The thumbnail is the page's own (an image.View, a
// card, a button): the page opens the preview from whatever the user pressed,
// and declares the preview where the page is declared:
//
//	m.preview = imagepreview.New()
//	// in the page:
//	if thumbnail.Clicked() {
//		m.preview.Open()
//	}
//	m.preview.View(c, imagepreview.Spec{Bitmap: m.bitmap, Title: "Screenshot"})
//
// The dialog opens with the whole image in view. The wheel, the buttons and the
// keys + and - zoom, 0 fits the image again, and a drag moves an image larger
// than the view. The Actual size button shows one image pixel for each device pixel.
// Escape, the close button and a press outside close the dialog, as for any
// Modal.
//
// The caller owns the decoded pixels and keeps them unchanged while the dialog
// is open (see image.State.Bitmap).
//
// It is a separate package so an application that does not import it pays
// nothing for it. Keep a Preview in the model; a warm paint allocates nothing.
// Do not copy a Preview after it has been declared.
package imagepreview

import (
	"strconv"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/image"
)

var (
	plusIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(5, 12), gfx.LineTo(19, 12), gfx.MoveTo(12, 5), gfx.LineTo(12, 19),
	}}
	minusIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(5, 12), gfx.LineTo(19, 12),
	}}
	fitIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(8, 3), gfx.LineTo(5, 3), gfx.QuadTo(3, 3, 3, 5), gfx.LineTo(3, 8),
		gfx.MoveTo(21, 8), gfx.LineTo(21, 5), gfx.QuadTo(21, 3, 19, 3), gfx.LineTo(16, 3),
		gfx.MoveTo(16, 21), gfx.LineTo(19, 21), gfx.QuadTo(21, 21, 21, 19), gfx.LineTo(21, 16),
		gfx.MoveTo(3, 16), gfx.LineTo(3, 19), gfx.QuadTo(3, 21, 5, 21), gfx.LineTo(8, 21),
	}}
)

const (
	zoomStep = 1.25
	// maxZoom is the largest zoom: thirty-two device pixels for an image pixel.
	maxZoom = 32
	// defaultWidth and defaultHeight are the view's size in the dialog.
	defaultWidth, defaultHeight = 640, 420
	// chromeHeight is what the dialog's title, the buttons below the view and the
	// room around them add to the view's height.
	chromeHeight = 132
)

// Spec declares the dialog.
type Spec struct {
	// Bitmap is the picture shown.
	Bitmap gfx.Bitmap
	// Title names the dialog for assistive technology and heads it; "Image
	// preview" when empty.
	Title string
	// Width and Height are the size of the view in the dialog; 640 by 420 when
	// zero or less.
	Width, Height float32
	// Labels are the names of the zoom out, zoom in, fit and actual-size
	// buttons: "Zoom out", "Zoom in", "Fit" and "Actual size" when empty. The
	// first three are icons and name them for assistive technology; between
	// the first two the zoom is shown as a percentage.
	Labels [4]string
}

// Preview is one dialog and the zoom and pan of its picture.
type Preview struct {
	open     bool
	view     image.State
	zoom     float32 // device pixels of the screen for an image pixel; zero fits the view
	pan      gfx.Point
	last     gfx.Point
	dragging bool
	size     gfx.Size // the view's size in the last paint
	bitmap   gfx.Bitmap
	scale    float32 // the canvas's pixel scale in the last paint
	percent  [2]string
	pct      int
	body     func()
	c        ui.C
	spec     Spec
}

// New returns a closed preview.
func New() *Preview {
	p := &Preview{pct: -1}
	p.view.Pointer = p.pointer
	p.view.Scroll = p.scroll
	p.view.Key = p.key
	return p
}

// Open shows the dialog with the whole picture in view.
func (p *Preview) Open() {
	p.open = true
	p.zoom, p.pan = 0, gfx.Point{}
}

// Close hides the dialog.
func (p *Preview) Close() { p.open = false }

// IsOpen reports whether the dialog is shown.
func (p *Preview) IsOpen() bool { return p.open }

// fitZoom is the zoom at which the whole picture is in view.
func (p *Preview) fitZoom() float32 {
	if p.bitmap.Width <= 0 || p.bitmap.Height <= 0 || p.size.Width <= 0 {
		return 1
	}
	return min(p.size.Width/float32(p.bitmap.Width), p.size.Height/float32(p.bitmap.Height)) * p.scale
}

// current is the zoom in effect.
func (p *Preview) current() float32 {
	if p.zoom > 0 {
		return p.zoom
	}
	return p.fitZoom()
}

func (p *Preview) zoomBy(factor float32) {
	p.zoom = min(max(p.current()*factor, p.fitZoom()/4), maxZoom)
	p.clampPan()
}

// clampPan keeps the picture from leaving empty space in the view, as the
// component does when it paints, so a drag past the edge does not build up.
func (p *Preview) clampPan() {
	if p.zoom <= 0 {
		p.pan = gfx.Point{}
		return
	}
	scale := p.zoom / p.scale
	reachX := max((float32(p.bitmap.Width)*scale-p.size.Width)/2, 0)
	reachY := max((float32(p.bitmap.Height)*scale-p.size.Height)/2, 0)
	p.pan.X = max(min(p.pan.X, reachX), -reachX)
	p.pan.Y = max(min(p.pan.Y, reachY), -reachY)
}

func (p *Preview) pointer(event gfx.PointerEvent) gfx.EventResult {
	switch event.Kind {
	case gfx.PointerDown:
		p.dragging = true
		p.last = gfx.Point{X: event.X, Y: event.Y}
		return gfx.EventResult{Handled: true}
	case gfx.PointerMove:
		if !p.dragging && !event.Captured {
			return gfx.EventResult{}
		}
		p.pan.X += event.X - p.last.X
		p.pan.Y += event.Y - p.last.Y
		p.last = gfx.Point{X: event.X, Y: event.Y}
		p.clampPan()
		return gfx.EventResult{Handled: true, Repaint: true}
	case gfx.PointerUp:
		p.dragging = false
		return gfx.EventResult{Handled: true}
	}
	return gfx.EventResult{}
}

func (p *Preview) scroll(event gfx.ScrollEvent) gfx.EventResult {
	if event.DeltaY == 0 {
		return gfx.EventResult{}
	}
	factor := float32(zoomStep)
	if event.DeltaY > 0 {
		factor = 1 / zoomStep
	}
	p.zoomBy(factor)
	return gfx.EventResult{Handled: true, Repaint: true}
}

func (p *Preview) key(event gfx.KeyEvent) gfx.EventResult {
	if event.Release {
		return gfx.EventResult{}
	}
	switch event.Text {
	case "+", "=":
		p.zoomBy(zoomStep)
	case "-":
		p.zoomBy(1 / zoomStep)
	case "0":
		p.zoom, p.pan = 0, gfx.Point{}
	default:
		return gfx.EventResult{}
	}
	return gfx.EventResult{Handled: true, Repaint: true}
}

// View declares the dialog at c while it is open, and nothing while it is
// closed. Declare it on its own, not inside a Memo.
func (p *Preview) View(c ui.C, spec Spec) {
	if p == nil {
		panic("imagepreview: View needs its Preview")
	}
	if !p.open {
		return
	}
	if spec.Title == "" {
		spec.Title = "Image preview"
	}
	if spec.Width <= 0 || spec.Height <= 0 {
		spec.Width, spec.Height = defaultWidth, defaultHeight
	}
	p.bitmap, p.size, p.scale = spec.Bitmap, gfx.Size{Width: spec.Width, Height: spec.Height}, c.Canvas().PixelScale()
	p.c, p.spec = c, spec
	if p.body == nil {
		p.body = p.draw
	}
	pad := c.Theme().Metrics.PadSurface
	ui.Modal(c, &p.open, spec.Title, ui.ModalSpec{Width: spec.Width + 2*pad, Height: spec.Height + chromeHeight}, p.body, nil)
}

// label is the accessible name of a button, the caller's or the default.
func (s *Spec) label(i int) string {
	if s.Labels[i] != "" {
		return s.Labels[i]
	}
	return [4]string{"Zoom out", "Zoom in", "Fit", "Actual size"}[i]
}

// percentText is the zoom as a percentage of the picture's own size.
func (p *Preview) percentText() string {
	pct := int(p.current()*100 + 0.5)
	if pct != p.pct || p.percent[0] == "" {
		p.pct = pct
		p.percent[0] = strconv.Itoa(pct) + "%"
	}
	return p.percent[0]
}

func (p *Preview) draw() {
	c, spec := p.c, &p.spec
	p.clampPan()
	ui.Column(c).Gap(c.Theme().Metrics.GapItem).Children(func() {
		p.view.Reset().Bitmap(spec.Bitmap).Zoom(p.zoom).Pan(p.pan)
		image.View(c, spec.Title, &p.view).Size(spec.Width, spec.Height)
		ui.Row(c).Items(ui.Center).Justify(ui.Center).Gap(c.Theme().Metrics.GapItem).Children(func() {
			if ui.Button(c, spec.label(0)).Icon(minusIcon).IconOnly().Outline().Clicked() {
				p.zoomBy(1 / zoomStep)
			}
			ui.Text(c, p.percentText()).Muted().Width(48).Shrink(0)
			if ui.Button(c, spec.label(1)).Icon(plusIcon).IconOnly().Outline().Clicked() {
				p.zoomBy(zoomStep)
			}
			if ui.Button(c, spec.label(2)).Icon(fitIcon).IconOnly().Outline().Clicked() {
				p.zoom, p.pan = 0, gfx.Point{}
			}
			if ui.Button(c, spec.label(3)).Outline().Clicked() {
				p.zoom = 1
				p.clampPan()
			}
		})
	})
}
