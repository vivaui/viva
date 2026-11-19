// Package skeletonview is the opt-in page control for static loading
// placeholders. A page declares each placeholder with its kind, size, radius and
// opacity, then views it:
//
//	line := skeletonview.New()
//	...
//	line.Reset().Text().Size(236, 14).Radius(4).Opacity(1)
//	skeletonview.View(c, "Body line", line)
//
//	avatar := skeletonview.New()
//	avatar.Reset().Circle().Size(56, 56).Opacity(0.75)
//	skeletonview.View(c, "Avatar placeholder", avatar)
//
// Text is a thin rounded line, Circle is a circle inside the shorter side, and
// Block is the default rounded surface. Size is the placeholder's page box;
// Radius overrides the corner radius; Opacity fades the muted surface without
// changing what it occupies. None of them is a visual constructor: New plus the
// chained options is the only way to declare one.
//
// There is no timer, shimmer worker or implicit animation. The component paints
// one neutral surface, and a page that wants motion schedules it explicitly, as
// it would for any other frame. An idle placeholder does no work.
//
// It is a separate package so an application that does not import it pays
// nothing for it. Keep a Skeleton in the model: its draw function is made once,
// so a warm paint allocates nothing. Do not copy one after it has been painted.
package skeletonview

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/ui"
)

// Kind is the shape of one placeholder.
type Kind uint8

const (
	// Block is a rounded surface the size of its box.
	Block Kind = iota
	// Text is a thin line with the recipe's 4 pixel radius unless Radius says
	// otherwise.
	Text
	// Circle is a circle inside the shorter side of its box.
	Circle
)

// Skeleton is one placeholder's caller-owned values and page control. The zero
// value is a fully opaque Block at no size; set the values through the chained
// methods and give it a size.
type Skeleton struct {
	kind       Kind
	width      float32
	height     float32
	radius     float32
	radiusSet  bool
	opacity    float32
	opacitySet bool

	drawFn   func(*ui.DrawContext)
	prepared bool
}

// New returns an empty Block placeholder.
func New() *Skeleton { return new(Skeleton) }

// Reset returns the placeholder to a fully opaque Block of no size. The draw
// function is kept, so a page that reconfigures the placeholder every paint
// allocates nothing.
func (s *Skeleton) Reset() *Skeleton {
	s.kind = Block
	s.width, s.height = 0, 0
	s.radius, s.radiusSet = 0, false
	s.opacity, s.opacitySet = 0, false
	return s
}

// Block selects the default rounded surface.
func (s *Skeleton) Block() *Skeleton { s.kind = Block; return s }

// Text selects the thin text-line placeholder.
func (s *Skeleton) Text() *Skeleton { s.kind = Text; return s }

// Circle selects the circular placeholder.
func (s *Skeleton) Circle() *Skeleton { s.kind = Circle; return s }

// Size sets the placeholder's page box. The page may still override it on the
// returned ui.Box.
func (s *Skeleton) Size(width, height float32) *Skeleton {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	s.width, s.height = width, height
	return s
}

// Radius overrides the corner radius, clamped at zero. For Circle it overrides
// the circle radius but keeps the square bounds.
func (s *Skeleton) Radius(pixels float32) *Skeleton {
	if pixels < 0 {
		pixels = 0
	}
	s.radius, s.radiusSet = pixels, true
	return s
}

// Opacity sets the surface's alpha, 0 to 1. It is a static visual value, not an
// animation phase; the placeholder never schedules a repaint for it.
func (s *Skeleton) Opacity(fraction float32) *Skeleton {
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}
	s.opacity, s.opacitySet = fraction, true
	return s
}

// View declares the placeholder at c. label names it for assistive technology.
// The box uses the Skeleton's Size; the page may override its dimensions.
//
// Declare the view on its own, not inside a Memo.
func View(c ui.C, label string, skeleton *Skeleton) ui.Box {
	if skeleton == nil {
		panic("skeletonview: View needs its Skeleton")
	}
	skeleton.prepare()
	return ui.Slot(c).Size(skeleton.width, skeleton.height).Label(label).Draw(skeleton.drawFn)
}

func (s *Skeleton) prepare() {
	if s.prepared {
		return
	}
	s.prepared = true
	s.drawFn = s.draw
}

func (s *Skeleton) draw(d *ui.DrawContext) {
	theme := *d.Theme
	if s.opacitySet {
		theme.Muted.Alpha *= s.opacity
	}
	placeholder := components.BeginSkeleton(d.Canvas, theme, d.Rect)
	switch s.kind {
	case Circle:
		placeholder = placeholder.Circle()
	case Text:
		placeholder = placeholder.Radius(4)
	}
	if s.radiusSet {
		placeholder = placeholder.Radius(s.radius)
	}
	placeholder.Paint()
}
