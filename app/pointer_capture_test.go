package app

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

// A press owns the pointer until the button comes up, as on every desktop:
// dragging out of the window must keep reaching the widget that was pressed.
// The platform reports leaving the window, and a widget that ends its drag there
// stops following the pointer exactly when the user is dragging past the edge.
func TestAPressKeepsThePointerUntilTheButtonComesUp(t *testing.T) {
	var canvas gfx.Canvas
	var events []gfx.PointerEvent
	config := Config{Title: "capture", Width: 100, Height: 100}
	config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
		canvas.HitWithLabel(gfx.Item(7), gfx.Rect{Width: 40, Height: 20}, "Terminal")
	}
	config.OnPointer = func(event gfx.PointerEvent) gfx.EventResult {
		events = append(events, event)
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	painter := newTestPainter(t, &canvas, config)
	send := func(event platform.PointerEvent) {
		if _, err := painter.pointerEvent(event); err != nil {
			t.Fatal(err)
		}
	}

	send(platform.PointerEvent{Kind: platform.PointerDown, Button: gfx.PointerButtonPrimary, X: 10, Y: 10})
	// The pointer leaves the window with the button still down, then keeps
	// moving out there.
	send(platform.PointerEvent{Kind: platform.PointerExit})
	send(platform.PointerEvent{Kind: platform.PointerMove, X: 120, Y: 140})

	for _, event := range events[1:] {
		if event.Kind == gfx.PointerExit {
			t.Fatalf("the press was cancelled while the button was down: %+v", events)
		}
		if event.Target != gfx.Item(7) || !event.Captured {
			t.Fatalf("a move outside the window did not reach the pressed widget: %+v", events)
		}
	}
	outside := events[len(events)-1]
	if outside.Kind != gfx.PointerMove || outside.X != 120 || outside.Y != 140 {
		t.Fatalf("the move outside the window was reported as %+v", outside)
	}

	// Releasing outside ends the press, and only then does the widget hear that
	// the pointer is gone, so hover state and links do not stay stuck.
	send(platform.PointerEvent{Kind: platform.PointerUp, Button: gfx.PointerButtonPrimary, X: 120, Y: 140})
	if len(events) < 2 {
		t.Fatalf("release outside produced %+v", events)
	}
	released := events[len(events)-2]
	if released.Kind != gfx.PointerUp || released.Target != gfx.Item(7) || !released.Captured {
		t.Fatalf("the release outside the window was reported as %+v", released)
	}
	if gone := events[len(events)-1]; gone.Kind != gfx.PointerExit {
		t.Fatalf("the pointer leaving after the release was reported as %+v", gone)
	}
}

// A press the platform takes for itself ends for the application: its release
// goes to whoever holds the pointer now, so the application is told the pointer
// is gone instead of keeping a drag it can never finish. Every platform reports
// the same thing as a cancel: the system's window drag, a window manager's grab.
func TestACancelledPressEndsForTheApplication(t *testing.T) {
	var canvas gfx.Canvas
	var events []gfx.PointerEvent
	config := Config{Title: "cancel", Width: 100, Height: 100}
	config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
		canvas.HitWithLabel(gfx.Item(7), gfx.Rect{Width: 100, Height: 100}, "Terminal")
	}
	config.OnPointer = func(event gfx.PointerEvent) gfx.EventResult {
		events = append(events, event)
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	painter := newTestPainter(t, &canvas, config)
	send := func(event platform.PointerEvent) {
		t.Helper()
		if _, err := painter.pointerEvent(event); err != nil {
			t.Fatal(err)
		}
	}

	send(platform.PointerEvent{Kind: platform.PointerDown, Button: gfx.PointerButtonPrimary, X: 10, Y: 10})
	send(platform.PointerEvent{Kind: platform.PointerMove, X: 20, Y: 20})
	if last := events[len(events)-1]; last.Kind != gfx.PointerMove || !last.Captured || last.Target != gfx.Item(7) {
		t.Fatalf("a held press was reported as %+v", last)
	}
	events = events[:0]
	send(platform.PointerEvent{Kind: platform.PointerCancel, X: 20, Y: 20})
	if len(events) != 1 || events[0].Kind != gfx.PointerExit {
		t.Fatalf("cancelling the press told the application %+v, want one pointer exit", events)
	}
	events = events[:0]
	send(platform.PointerEvent{Kind: platform.PointerMove, X: 30, Y: 30})
	if last := events[len(events)-1]; last.Kind != gfx.PointerMove || last.Captured {
		t.Fatalf("a move after the cancel still reports a press: %+v", last)
	}
}

// The activation going away takes the pointer with it. A press held when the
// user switches to another window never gets its release here, so the
// application must be told the press ended; otherwise a pane that keeps its
// layout while a drag is in progress (a terminal resizing its shell after the
// drag) stays where it was until the user clicks again.
func TestADeactivatedWindowEndsItsPress(t *testing.T) {
	var canvas gfx.Canvas
	var events []gfx.PointerEvent
	config := Config{Title: "deactivate", Width: 100, Height: 100}
	config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
		canvas.HitWithLabel(gfx.Item(7), gfx.Rect{Width: 100, Height: 100}, "Terminal")
	}
	config.OnPointer = func(event gfx.PointerEvent) gfx.EventResult {
		events = append(events, event)
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	painter := newTestPainter(t, &canvas, config)
	send := func(event platform.PointerEvent) {
		t.Helper()
		if _, err := painter.pointerEvent(event); err != nil {
			t.Fatal(err)
		}
	}

	send(platform.PointerEvent{Kind: platform.PointerDown, Button: gfx.PointerButtonPrimary, X: 10, Y: 10})
	send(platform.PointerEvent{Kind: platform.PointerMove, X: 20, Y: 20})
	if last := events[len(events)-1]; !last.Captured {
		t.Fatalf("a held press was reported as %+v", last)
	}
	events = events[:0]
	if _, err := painter.deactivate(); err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Kind != gfx.PointerExit {
		t.Fatalf("deactivating told the application %+v, want one pointer exit", events)
	}
	events = events[:0]
	send(platform.PointerEvent{Kind: platform.PointerMove, X: 30, Y: 30})
	if last := events[len(events)-1]; last.Captured {
		t.Fatalf("a move after deactivation still reports a press: %+v", last)
	}
}

// A window manager's grab (Alt+Tab) takes the keyboard and the held button's
// release together, so the press ends with the focus.
func TestLosingFocusEndsAPress(t *testing.T) {
	var canvas gfx.Canvas
	var events []gfx.PointerEvent
	config := Config{Title: "focus", Width: 100, Height: 100, OnKey: func(gfx.KeyEvent) gfx.EventResult { return gfx.EventResult{} }}
	config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
		canvas.HitWithLabel(gfx.Item(7), gfx.Rect{Width: 100, Height: 100}, "Terminal")
	}
	config.OnPointer = func(event gfx.PointerEvent) gfx.EventResult {
		events = append(events, event)
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	painter := newTestPainter(t, &canvas, config)
	send := func(event platform.PointerEvent) {
		t.Helper()
		if _, err := painter.pointerEvent(event); err != nil {
			t.Fatal(err)
		}
	}

	send(platform.PointerEvent{Kind: platform.PointerDown, Button: gfx.PointerButtonPrimary, X: 10, Y: 10})
	send(platform.PointerEvent{Kind: platform.PointerMove, X: 20, Y: 20})
	events = events[:0]
	if _, err := painter.focusLost(); err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Kind != gfx.PointerExit {
		t.Fatalf("losing focus told the application %+v, want one pointer exit", events)
	}
}
