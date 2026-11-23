//go:build js && wasm

package web

import (
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/interaction"
)

// The browser runtime reads a finger as the native one does: a tap clicks, a
// pan scrolls without pressing, and a held finger presses after its timeout.
func TestBrowserTouchTapsPansAndPresses(t *testing.T) {
	var canvas gfx.Canvas
	now := time.Duration(0)
	canvas.SetClock(func() time.Duration { return now })
	clicks, presses := 0, 0
	scrolled := float32(0)
	config := Config{
		Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
			canvas.HitWithOptions(gfx.Item(1), gfx.Rect{Width: 200, Height: 200}, "Button", gfx.HitOptions{Role: gfx.RoleButton})
			if canvas.Clicked(gfx.Item(1)) {
				clicks++
			}
		},
		OnPointer: func(event gfx.PointerEvent) gfx.EventResult {
			if event.Kind == gfx.PointerDown {
				presses++
			}
			return gfx.EventResult{}
		},
		OnScroll: func(event gfx.ScrollEvent) gfx.EventResult {
			scrolled += event.DeltaY
			return gfx.EventResult{Handled: true, Repaint: true}
		},
	}
	painter := newPainter(&canvas, &config)
	painter.width, painter.height = 400, 400
	painter.repaint()
	touch := func(kind gfx.PointerEventKind, x, y float32) {
		painter.pointerEvent(gfx.PointerEvent{Kind: kind, Button: gfx.PointerButtonPrimary, X: x, Y: y, Device: gfx.PointerTouch, PointerID: 7})
	}

	touch(gfx.PointerDown, 50, 50)
	touch(gfx.PointerUp, 50, 50)
	if clicks != 1 {
		t.Fatalf("a tap clicked %d times", clicks)
	}

	presses = 0
	touch(gfx.PointerDown, 50, 150)
	touch(gfx.PointerMove, 50, 120)
	touch(gfx.PointerMove, 50, 100)
	touch(gfx.PointerUp, 50, 100)
	if presses != 0 || scrolled < 40 {
		t.Fatalf("a pan pressed %d times and scrolled %v", presses, scrolled)
	}

	now += time.Second
	touch(gfx.PointerDown, 50, 50)
	now += interaction.TouchTapTimeout + time.Millisecond
	painter.repaint() // the frame the runtime asked for at the tap timeout
	if presses != 1 {
		t.Fatalf("a held finger pressed %d times after its tap timeout", presses)
	}
	touch(gfx.PointerUp, 50, 50)
	if clicks != 2 {
		t.Fatalf("lifting the held finger left %d clicks, want 2", clicks)
	}
}
