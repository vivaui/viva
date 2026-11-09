package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// A page declares into the rectangle the window gave it: the driver paints the
// page over its whole 480x480 window.
func TestPageBounds(t *testing.T) {
	var bounds gfx.Rect
	openPage(t, func(c ui.C) { bounds = c.Bounds() })
	want := gfx.Rect{Width: 480, Height: 480}
	if bounds != want {
		t.Fatalf("Bounds = %+v, want the driver's page rectangle %+v", bounds, want)
	}
}

// The page has the window's canvas while it declares, and can measure text on
// it before declaring anything.
func TestPageCanvas(t *testing.T) {
	var canvas *gfx.Canvas
	var textWidth float32
	openPage(t, func(c ui.C) {
		canvas = c.Canvas()
		textWidth = canvas.TextWidth("x", 12)
	})
	if canvas == nil {
		t.Fatal("Canvas is nil while the page declares")
	}
	if textWidth <= 0 {
		t.Fatalf("Canvas().TextWidth(\"x\", 12) = %v, want a positive width", textWidth)
	}
}
