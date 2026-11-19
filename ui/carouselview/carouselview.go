// Package carouselview is the opt-in page control for the clipped carousel of
// package components. It wraps components.BeginCarousel in a ui.Control, so a
// page declares a carousel without calling the component itself:
//
//	tour := &carouselview.Carousel{
//		ItemCount: 3, Index: m.slide,
//		PreviousID: m.previous, PreviousLabel: "Previous slide",
//		NextID: m.next, NextLabel: "Next slide",
//	}
//	tour.Draw = func(d *ui.DrawContext, geometry carouselview.Geometry) {
//		for index := 0; index < tour.ItemCount; index++ {
//			d.Canvas.Fill(tour.ItemBounds(geometry, index), d.Theme.Muted)
//		}
//	}
//	...
//	carouselview.View(c, "Product tour", tour).FillWidth().Height(200)
//	if tour.Clicked(m.previous) && m.slide > 0 {
//		m.slide--
//	} else if tour.Clicked(m.next) && m.slide < 2 {
//		m.slide++
//	}
//
// The caller owns the index, the offset, the content extent and the slide
// content; the wrapper paints through the component, clips the slide content to
// the viewport and routes pointer, keys and the wheel aimed at the control to
// the caller's callbacks. The navigation controls keep the caller's identities,
// so their clicks are read with Clicked after View. There is no autoplay,
// timer, animation, dragging or inertia: an idle carousel does no work.
//
// It is a separate package so an application that does not import it pays
// nothing for it. Keep a Carousel in the model, beside its slides: its handlers
// and draw function are made once, so a warm paint allocates nothing. Do not
// copy a Carousel after it has been declared.
package carouselview

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Orientation selects the content axis of a Carousel.
type Orientation = components.CarouselOrientation

const (
	Horizontal = components.CarouselHorizontal
	Vertical   = components.CarouselVertical
)

// Geometry is the complete carousel layout handed to Draw.
type Geometry = components.CarouselGeometry

// Carousel is one carousel's state and page control. Fill the fields from the
// caller's model every paint; Draw and the callbacks stay as they are and are
// read when they are needed.
type Carousel struct {
	// Orientation selects the content axis. The zero value is horizontal.
	Orientation components.CarouselOrientation
	// ItemCount is the number of caller-owned slides.
	ItemCount int
	// Index is the selected slide. It is clamped by the component.
	Index int
	// Offset is the caller-owned content offset. When Extent is zero and Offset
	// is zero, the wrapper derives one viewport of each slide from Index.
	Offset float32
	// Extent is the total content extent along the axis. When zero, it is
	// ItemCount viewports, one slide per view.
	Extent float32
	// PreviousID, NextID and their labels declare the navigation controls.
	PreviousID    gfx.ID
	PreviousLabel string
	NextID        gfx.ID
	NextLabel     string
	// Disabled paints both controls and keeps them inert.
	Disabled bool

	// Draw paints the caller's slides. It runs inside the viewport clip, after
	// the component has laid the content out; geometry is the complete carousel
	// layout and ItemBounds maps a slide of an evenly divided content extent.
	Draw func(*ui.DrawContext, components.CarouselGeometry)

	// Pointer receives every pointer event aimed at the control itself.
	Pointer func(gfx.PointerEvent) gfx.EventResult
	// Key receives the keys pressed while the control has the focus.
	Key func(gfx.KeyEvent) gfx.EventResult
	// Scroll receives the wheel over the control. A nil callback leaves it to
	// the page.
	Scroll func(gfx.ScrollEvent) gfx.EventResult

	handlers ui.Handlers
	drawFn   func(*ui.DrawContext)
	prepared bool
	label    string
	geometry components.CarouselGeometry
	clicked  gfx.ID
	hadClick bool
}

// View declares the carousel at c. label names the region for assistive
// technology. Give it a size; it is a ui.Control painted through the component.
//
// Declare the view on its own, not inside a Memo.
func View(c ui.C, label string, carousel *Carousel) ui.Box {
	if carousel == nil {
		panic("carouselview: View needs its Carousel")
	}
	carousel.prepare()
	carousel.label = label
	return ui.Control(c, label, &carousel.handlers).Draw(carousel.drawFn)
}

// Clicked reports whether the paint just made was an event pass delivering a
// click on id, PreviousID or NextID. It is false in ordinary paints. Ask it
// after View, where a component's clicks have always been read.
func (c *Carousel) Clicked(id gfx.ID) bool {
	return c.hadClick && !id.IsZero() && c.clicked == id
}

func (c *Carousel) prepare() {
	if c.prepared {
		return
	}
	c.prepared = true
	c.handlers.Pointer = c.pointer
	c.handlers.Key = c.key
	c.handlers.Scroll = c.scroll
	c.drawFn = c.draw
}

func (c *Carousel) pointer(event gfx.PointerEvent) gfx.EventResult {
	if c.Pointer == nil {
		return gfx.EventResult{}
	}
	return c.Pointer(event)
}

func (c *Carousel) key(event gfx.KeyEvent) gfx.EventResult {
	if c.Key == nil {
		return gfx.EventResult{}
	}
	return c.Key(event)
}

func (c *Carousel) scroll(event gfx.ScrollEvent) gfx.EventResult {
	if c.Scroll == nil {
		return gfx.EventResult{}
	}
	return c.Scroll(event)
}

func (c *Carousel) draw(d *ui.DrawContext) {
	c.clicked, c.hadClick = d.Canvas.ClickTarget()

	viewportExtent := d.Rect.Width
	if c.Orientation == components.CarouselVertical {
		viewportExtent = d.Rect.Height
	}
	extent, offset := c.Extent, c.Offset
	if extent <= 0 {
		extent = viewportExtent * float32(max(c.ItemCount, 0))
		if offset == 0 {
			offset = viewportExtent * float32(c.Index)
		}
	}

	carousel := components.BeginCarousel(d.Canvas, *d.Theme, d.Rect, c.ItemCount, c.Index).
		Content(extent, offset).
		Controls(c.PreviousID, c.PreviousLabel, c.NextID, c.NextLabel).
		Label(c.label)
	if c.Orientation == components.CarouselVertical {
		carousel = carousel.Vertical()
	}
	if c.Disabled {
		carousel = carousel.Disabled()
	}
	geometry := carousel.BeginContent()
	c.geometry = geometry
	if c.Draw != nil {
		c.Draw(d, geometry)
	}
	carousel.EndContent()
}

// Geometry is the carousel layout painted last: viewport, translated content,
// both control bounds and the clamped index. It is zero until View has painted
// at least once.
func (c *Carousel) Geometry() components.CarouselGeometry { return c.geometry }

// ItemBounds maps one slide of an evenly divided content extent to page
// coordinates: Index along the axis, the viewport extent across it. It returns
// an empty rectangle for an index outside the item count. Use it in Draw for
// the common one-slide-per-view layout; a caller with items of different
// extents maps them from Geometry.Content instead.
func (c *Carousel) ItemBounds(geometry components.CarouselGeometry, index int) gfx.Rect {
	if index < 0 || index >= c.ItemCount || c.ItemCount <= 0 {
		return gfx.Rect{}
	}
	content := geometry.Content
	itemExtent := content.Width
	if c.Orientation == components.CarouselVertical {
		itemExtent = content.Height
	}
	itemExtent /= float32(c.ItemCount)
	if c.Orientation == components.CarouselVertical {
		return gfx.Rect{
			X: content.X, Y: content.Y + float32(index)*itemExtent,
			Width: geometry.Viewport.Width, Height: itemExtent,
		}
	}
	return gfx.Rect{
		X: content.X + float32(index)*itemExtent, Y: content.Y,
		Width: itemExtent, Height: geometry.Viewport.Height,
	}
}
