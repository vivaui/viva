package app

import (
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
)

// A key handler that closes an overlay and asks the same key to keep traversing
// moves focus into the page the overlay was covering. That needs a canvas
// painted without the overlay and routing rebuilt from it, which is two paints
// for one event: the event still owes the platform exactly one frame, and the
// frame it publishes has to carry everything both paints changed. Publishing the
// first one and then dropping it left the backend holding the overlay's
// rectangle count while the second frame described the smaller page, which the
// Windows backend rejects with "canvas rectangle count shrank without rebuild".
func TestContinueTraversalKeyPublishesOneConsistentFrame(t *testing.T) {
	var canvas gfx.Canvas
	theme := gfx.LightTheme()
	popup := true

	config := Config{Title: "traversal", Width: 640, Height: 480}
	config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
		components.BeginButton(canvas, theme, gfx.Item(1), gfx.Rect{X: 10, Y: 10, Width: 90}).Default("One")
		components.BeginButton(canvas, theme, gfx.Item(2), gfx.Rect{X: 110, Y: 10, Width: 90}).Secondary("Two")
		canvas.HitWithLabel(gfx.Item(3), gfx.Rect{X: 10, Y: 60, Width: 320, Height: 30}, "Group")
		if !popup {
			return
		}
		// The open candidate list: sixty rows the close has to take away.
		for row := 0; row < 60; row++ {
			bounds := gfx.Rect{X: 10, Y: float32(100 + row*16), Width: 320, Height: 15}
			canvas.Fill(bounds, gfx.Color{Red: 0.95, Green: 0.95, Blue: 0.95, Alpha: 1})
			canvas.HitWithLabel(gfx.Item(100+row), bounds, "Candidate")
		}
	}
	config.OnKey = func(event gfx.KeyEvent) gfx.EventResult {
		if event.Key == gfx.KeyTab && popup {
			popup = false
			// What SingleSelectState and MultiSelectState return for Tab.
			return gfx.EventResult{Handled: true, Repaint: true, ContinueTraversal: true}
		}
		return gfx.EventResult{}
	}

	painter := newTestPainter(t, &canvas, config)
	base := len(painter.renderer.Rectangles())
	frame, err := painter.key(platform.KeyEvent{Key: input.KeyTab})
	if err != nil {
		t.Fatal(err)
	}
	count := len(painter.renderer.Rectangles())
	if count >= base {
		t.Fatalf("the popup did not close: %d -> %d", base, count)
	}
	// The platform still holds base rectangles. A frame that carries a different
	// number of them has to say so with a rebuild.
	if len(frame.Rectangles) != base && !frame.Update.Rebuild {
		t.Fatalf("frame does not match the platform's base: base=%d frame=%d update=%+v", base, len(frame.Rectangles), frame.Update)
	}
	if !frame.Update.Rebuild || frame.Update.Count != count {
		t.Fatalf("update = %+v, want a rebuild of all %d rectangles", frame.Update, count)
	}
}
