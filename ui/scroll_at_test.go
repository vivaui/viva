package ui_test

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// ScrollAt sets the area's offset when the page declares it, and the layout
// clamps it to the content; the settled value is readable from the same paint's
// Box, so an application never keeps the ScrollState between paints.
func TestScrollAtStartsAtItsOffset(t *testing.T) {
	var area ui.Box
	var page *ui.Page
	view := func(c ui.C) {
		ui.Column(c).Fill().Children(func() {
			area = ui.ScrollAt(c, 80).Grow(1).Children(func() {
				ui.Slot(c).Height(1600)
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
