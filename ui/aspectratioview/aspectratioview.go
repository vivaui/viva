// Package aspectratioview is the opt-in page box that keeps its height at a
// caller-given ratio of its width. The content stays ordinary page children:
//
//	media := aspectratioview.New()
//	...
//	media.Ratio(16, 9)
//	aspectratioview.View(c, "16:9 release preview", media, func() {
//		ui.Column(c).Radius(c.Theme().Metrics.RadiusSurface).
//			Background(c.Theme().Muted).Children(func() {
//			ui.Text(c, "16:9 release preview").Muted()
//		})
//	}).FillWidth().MaxWidth(520)
//
// Ratio takes the numerator and denominator as written, so a call site reads as
// the ratio it shows; Square is Ratio(1, 1). There is no constructor per ratio.
// The box has no preferred width of its own: give it FillWidth, a width, or the
// layout of the page around it, and the height follows.
//
// The content is the caller's. Every in-flow child is given the full aspect box,
// which is what a page normally wants for one card, cover or media surface; a
// page that needs several regions declares a Column or Row as that child. An
// invalid ratio (zero, negative or infinite) produces a zero-height box, as the
// component's AspectRatio does, so bad layout input stays deterministic.
//
// It is a separate package so an application that does not import it pays
// nothing for it. Keep a Box in the model: its layout is reused, so a warm paint
// allocates nothing. Do not copy one after it has been declared.
package aspectratioview

import (
	"math"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Box is one ratio box and its page layout. New returns a square box; set the
// ratio before View.
type Box struct {
	width  float32
	height float32
	layout ratioLayout
}

// New returns a 1:1 ratio box.
func New() *Box {
	box := &Box{width: 1, height: 1}
	box.link()
	return box
}

// Ratio sets the width/height ratio, written as its two terms. A non-positive
// term or an infinite result makes the box zero height; it never divides by
// zero or paints at a non-finite size.
func (b *Box) Ratio(width, height float32) *Box {
	b.width, b.height = width, height
	b.link()
	return b
}

// Square sets the 1:1 ratio.
func (b *Box) Square() *Box { return b.Ratio(1, 1) }

// View declares the ratio box at c. label names the region for assistive
// technology. content declares the caller's page children; each in-flow child
// is laid out in the full aspect box. The caller sizes the box in the page.
//
// Declare the view on its own, not inside a Memo.
func View(c ui.C, label string, box *Box, content func()) ui.Box {
	if box == nil {
		panic("aspectratioview: View needs its Box")
	}
	if content == nil {
		panic("aspectratioview: View needs its content")
	}
	box.link()
	return ui.CustomLayout(c, &box.layout).
		HitWith(ui.ID(c), label, gfx.HitOptions{Role: gfx.RoleGroup, SkipFocus: true}).
		Children(content)
}

// link keeps the layout pointing at its box, so the zero Box and a copied Box
// still read the ratio the page set rather than a stale copy.
func (b *Box) link() { b.layout.box = b }

// ratio is the width/height ratio, or zero when it is not finite and positive.
func (b *Box) ratio() float32 {
	if !(b.width > 0) || !(b.height > 0) {
		return 0
	}
	ratio := b.width / b.height
	if math.IsInf(float64(ratio), 0) {
		return 0
	}
	return ratio
}

// ratioLayout lays the caller's children in the largest box that keeps the
// ratio inside the container's width. A CustomLayout is kept in the model, so
// it holds only the box it reads.
type ratioLayout struct {
	box *Box
}

func (layout *ratioLayout) MeasureWidth(*ui.LayoutContext) float32 {
	// The page owns the width: FillWidth or an explicit Width. The height is
	// what this layout derives.
	return 0
}

func (layout *ratioLayout) PlaceWidth(ctx *ui.LayoutContext, bounds ui.Rect) {
	for index := 0; index < ctx.Count(); index++ {
		ctx.SetX(index, bounds.X)
		ctx.SetWidth(index, bounds.Width)
	}
}

func (layout *ratioLayout) MeasureHeight(ctx *ui.LayoutContext) float32 {
	return aspectHeight(ctx.ContentWidth(), layout.ratio())
}

func (layout *ratioLayout) PlaceHeight(ctx *ui.LayoutContext, bounds ui.Rect) {
	height := aspectHeight(bounds.Width, layout.ratio())
	for index := 0; index < ctx.Count(); index++ {
		ctx.SetY(index, bounds.Y)
		ctx.SetHeight(index, height)
	}
}

func (layout *ratioLayout) ratio() float32 {
	if layout.box == nil {
		return 0
	}
	return layout.box.ratio()
}

func aspectHeight(width, ratio float32) float32 {
	if !(ratio > 0) || !(width > 0) {
		return 0
	}
	return width / ratio
}
