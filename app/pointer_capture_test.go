package app

import (
	"testing"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

// A press owns the pointer until the button comes up, as on every desktop:
// dragging out of the window must keep reaching the widget that was pressed.
// The platform reports leaving the window, and a widget that ends its drag there
// stops following the pointer exactly when the user is dragging past the edge.
func TestAPressKeepsThePointerUntilTheButtonComesUp(t *testing.T) {
	var canvas ui.Canvas
	var events []ui.PointerEvent
	config := Config{Title: "capture", Width: 100, Height: 100}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.HitWithLabel(ui.Item(7), ui.Rect{Width: 40, Height: 20}, "Terminal")
	}
	config.OnPointer = func(event ui.PointerEvent) ui.EventResult {
		events = append(events, event)
		return ui.EventResult{Handled: true, Repaint: true}
	}
	painter := newTestPainter(t, &canvas, config)
	send := func(event platform.PointerEvent) {
		if _, err := painter.pointerEvent(event); err != nil {
			t.Fatal(err)
		}
	}

	send(platform.PointerEvent{Kind: platform.PointerDown, Button: ui.PointerButtonPrimary, X: 10, Y: 10})
	// The pointer leaves the window with the button still down, then keeps
	// moving out there.
	send(platform.PointerEvent{Kind: platform.PointerExit})
	send(platform.PointerEvent{Kind: platform.PointerMove, X: 120, Y: 140})

	for _, event := range events[1:] {
		if event.Kind == ui.PointerExit {
			t.Fatalf("the press was cancelled while the button was down: %+v", events)
		}
		if event.Target != ui.Item(7) || !event.Captured {
			t.Fatalf("a move outside the window did not reach the pressed widget: %+v", events)
		}
	}
	outside := events[len(events)-1]
	if outside.Kind != ui.PointerMove || outside.X != 120 || outside.Y != 140 {
		t.Fatalf("the move outside the window was reported as %+v", outside)
	}

	// Releasing outside ends the press, and only then does the widget hear that
	// the pointer is gone, so hover state and links do not stay stuck.
	send(platform.PointerEvent{Kind: platform.PointerUp, Button: ui.PointerButtonPrimary, X: 120, Y: 140})
	if len(events) < 2 {
		t.Fatalf("release outside produced %+v", events)
	}
	released := events[len(events)-2]
	if released.Kind != ui.PointerUp || released.Target != ui.Item(7) || !released.Captured {
		t.Fatalf("the release outside the window was reported as %+v", released)
	}
	if gone := events[len(events)-1]; gone.Kind != ui.PointerExit {
		t.Fatalf("the pointer leaving after the release was reported as %+v", gone)
	}
}
