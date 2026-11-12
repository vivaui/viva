package ui_test

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func TestScrollAtRoutesWheelToItsOwnerBeforeAnOuterArea(t *testing.T) {
	var area, outer, first ui.Box
	offset := float32(80)
	view := func(c ui.C) {
		outer = ui.Scroll(c).Fill().Children(func() {
			area = ui.ScrollAt(c, offset).Height(150).Shrink(0).Children(func() {
				first = ui.Slot(c).Height(1600)
			})
			ui.Slot(c).Height(2000).Shrink(0)
		})
	}
	page := ui.NewPage(ui.PageConfig{Root: "scroll-at-wheel", View: view})
	calls := 0
	config := page.Config(app.Config{OnScroll: func(event gfx.ScrollEvent) gfx.EventResult {
		calls++
		offset += event.DeltaY
		return gfx.EventResult{Handled: true, Repaint: true}
	}})
	canvas := benchCanvas()
	drawPage(page, canvas, view)
	r := area.Rect()
	result := config.OnScroll(gfx.ScrollEvent{X: r.X + 10, Y: r.Y + 10, DeltaY: 20, Precise: true})
	if calls != 1 || !result.Handled || !result.Repaint {
		t.Fatalf("controlled area did not give its owner the wheel: calls=%d result=%+v", calls, result)
	}
	drawPage(page, canvas, view)
	if offset != 100 || area.ScrollState().Offset != 100 || first.Rect().Y != r.Y-100 {
		t.Fatalf("repaint lost the owner's offset: model=%v area=%v first=%+v", offset, area.ScrollState().Offset, first.Rect())
	}
	if outer.ScrollState().Offset != 0 {
		t.Fatal("the controlled area's wheel moved its outer area")
	}
}

// ScrollAt sets the area's offset when the page declares it, and the layout
// clamps it to the content; the settled value is readable from the same paint's
// Box, so an application never keeps the ScrollState between paints.
func TestScrollAtStartsAtItsOffset(t *testing.T) {
	var area, first ui.Box
	var page *ui.Page
	view := func(c ui.C) {
		ui.Column(c).Fill().Children(func() {
			area = ui.ScrollAt(c, 80).Grow(1).Children(func() {
				first = ui.Slot(c).Height(1600)
			})
		})
	}
	page = ui.NewPage(ui.PageConfig{Root: "scroll-at", View: view})
	canvas := benchCanvas()
	drawPage(page, canvas, view)
	state := area.ScrollState()
	if state.Content <= state.Viewport {
		t.Fatalf("content %v, viewport %v: the area did not lay out", state.Content, state.Viewport)
	}
	if state.Offset != 80 {
		t.Fatalf("offset = %v, want the declared 80", state.Offset)
	}
	if first.Rect().Y != -80 {
		t.Fatalf("first row at y=%v, want the content laid out 80 up", first.Rect().Y)
	}
	drawPage(page, canvas, view)
	if got := area.ScrollState().Offset; got != 80 {
		t.Fatalf("second paint offset = %v, want 80", got)
	}
}

func TestScrollAtClampsToTheContent(t *testing.T) {
	var area ui.Box
	view := func(c ui.C) {
		ui.Column(c).Fill().Children(func() {
			area = ui.ScrollAt(c, 5000).Grow(1).Children(func() {
				ui.Slot(c).Height(1600)
			})
		})
	}
	page := ui.NewPage(ui.PageConfig{Root: "scroll-at-clamp", View: view})
	drawPage(page, benchCanvas(), view)
	state := area.ScrollState()
	if state.Offset != state.Max() || state.Max() <= 0 {
		t.Fatalf("offset = %v, max = %v: want the clamped end", state.Offset, state.Max())
	}
}

// Bounds and Canvas are the measurements a page view needs without reaching for
// the Frame that lays it out.
func TestCtxBoundsAndCanvas(t *testing.T) {
	var bounds gfx.Rect
	var canvas *gfx.Canvas
	view := func(c ui.C) { bounds, canvas = c.Bounds(), c.Canvas() }
	page := ui.NewPage(ui.PageConfig{Root: "ctx-bounds", View: view})
	want := benchCanvas()
	drawPage(page, want, view)
	if bounds != (gfx.Rect{Width: 600, Height: 400}) {
		t.Fatalf("bounds = %+v, want the page's 600x400", bounds)
	}
	if canvas != want {
		t.Fatal("Canvas is not the canvas the page was drawn on")
	}
}

// Paragraph with no line limit and Wrap are the same text: neither stops at one
// line as Paragraph used to when its limit was zero.
func TestParagraphZeroAndWrapTakeAllLines(t *testing.T) {
	text := strings.Repeat("word ", 40)
	var one, zero, wrap ui.Box
	view := func(c ui.C) {
		ui.Column(c).Width(80).Children(func() {
			one = ui.Paragraph(c, text, 1)
			zero = ui.Paragraph(c, text, 0)
			wrap = ui.Wrap(c, text)
		})
	}
	page := ui.NewPage(ui.PageConfig{Root: "wrap-lines", View: view})
	drawPage(page, benchCanvas(), view)
	if zero.Rect().Height <= one.Rect().Height {
		t.Fatalf("Paragraph(0) height %v is not more than Paragraph(1) height %v", zero.Rect().Height, one.Rect().Height)
	}
	if wrap.Rect().Height != zero.Rect().Height {
		t.Fatalf("Wrap height %v != Paragraph(0) height %v", wrap.Rect().Height, zero.Rect().Height)
	}
}
