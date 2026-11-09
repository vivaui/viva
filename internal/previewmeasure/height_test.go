package previewmeasure

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

var viewport = gfx.Size{Width: 760, Height: 600}

func TestContentIgnoresThePageBackground(t *testing.T) {
	var canvas gfx.Canvas
	canvas.Fill(gfx.Rect{Width: viewport.Width, Height: viewport.Height}, gfx.Color{Alpha: 1})
	if got := Content(&canvas, viewport); got != 0 {
		t.Fatalf("background-only canvas measured %d, want 0", got)
	}
}

func TestContentMeasuresThePaintedExtent(t *testing.T) {
	var canvas gfx.Canvas
	canvas.Label("Hello", gfx.Rect{Y: 100, Height: 200}, gfx.TextStyle{Size: 14, Color: gfx.Color{Alpha: 1}})
	if got, want := Content(&canvas, viewport), 200+2*16; got != want {
		t.Fatalf("content measured %d, want %d", got, want)
	}
}

func TestContentTreatsAFullHeightPanelAsTallContent(t *testing.T) {
	var canvas gfx.Canvas
	canvas.Fill(gfx.Rect{Y: 0, Width: 200, Height: viewport.Height}, gfx.Color{Alpha: 1})
	if got := Content(&canvas, viewport); got != 520 {
		t.Fatalf("full-height panel measured %d, want the 520 ceiling", got)
	}
}

func TestContentRespectsAComponentClip(t *testing.T) {
	var canvas gfx.Canvas
	canvas.Clip(gfx.Rect{Width: 100, Height: 100})
	canvas.Fill(gfx.Rect{Y: 50, Width: 100, Height: 500}, gfx.Color{Alpha: 1})
	canvas.Unclip()
	if got := Content(&canvas, viewport); got != 160 {
		t.Fatalf("clipped content measured %d, want the 160 floor", got)
	}
}

func TestContentIgnoresTheViewportClip(t *testing.T) {
	var canvas gfx.Canvas
	canvas.Clip(gfx.Rect{Width: viewport.Width, Height: viewport.Height})
	canvas.Fill(gfx.Rect{Y: 100, Width: 100, Height: 500}, gfx.Color{Alpha: 1})
	canvas.Unclip()
	if got := Content(&canvas, viewport); got != 520 {
		t.Fatalf("overflowing content measured %d, want the 520 ceiling", got)
	}
}
