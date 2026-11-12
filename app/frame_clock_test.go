package app

import (
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

// A frame's interval is measured from the start of its paint, so a slow paint
// does not push every later frame further behind.
func TestFrameTimerMeasuresFromTheFrameStart(t *testing.T) {
	var wakes int
	frames := newFrameTimer(func() { wakes++ })
	start := time.Unix(100, 0)
	now := start
	frames.now = func() time.Time { return now }
	frames.setInterval(20 * time.Millisecond)
	frames.redrawing() // the frame starts at start
	now = start.Add(15 * time.Millisecond)
	frames.request(0)
	want := start.Add(20 * time.Millisecond)
	if !frames.due.Equal(want) {
		t.Fatalf("due = %v, want the frame start + 20ms, %v", frames.due, want)
	}
	frames.stop()
}

// A zero interval means presenting a frame blocks; the timer asks for the next
// frame only once the adapter reports the current one presented, so the event
// loop is left idle (and reads input) between frames.
func TestFrameTimerWithPresentPacedInterval(t *testing.T) {
	var wakes int
	frames := newFrameTimer(func() { wakes++ })
	start := time.Unix(200, 0)
	now := start
	frames.now = func() time.Time { return now }
	frames.setInterval(0)
	frames.redrawing()
	frames.request(0)
	if frames.armed.Load() {
		t.Fatal("a present-paced frame asked for its successor before presenting")
	}
	now = start.Add(16 * time.Millisecond)
	frames.presented()
	want := now.Add(minFrameInterval)
	if !frames.due.Equal(want) {
		t.Fatalf("due = %v, want %v (after the present, with the safety floor)", frames.due, want)
	}
	if frames.interval != 0 {
		t.Fatalf("interval = %v, want present-paced zero", frames.interval)
	}
	frames.stop()
}

// A request made while no frame is painting arms the timer at once: a pointer
// event that coalesced its repaint must not wait for a present that will only
// follow the frame it asked for.
func TestFrameTimerArmsOutsideAFrame(t *testing.T) {
	frames := newFrameTimer(func() {})
	start := time.Unix(300, 0)
	frames.now = func() time.Time { return start }
	frames.setInterval(0)
	frames.request(0)
	if want := start.Add(minFrameInterval); !frames.due.Equal(want) {
		t.Fatalf("due = %v, want %v", frames.due, want)
	}
	frames.stop()
}

// A frame that presented but asked for nothing wakes nothing: a settled canvas
// owns no timer.
func TestFrameTimerPresentedWithoutRequest(t *testing.T) {
	frames := newFrameTimer(func() {})
	frames.now = func() time.Time { return time.Unix(400, 0) }
	frames.setInterval(0)
	frames.redrawing()
	frames.presented()
	if frames.armed.Load() {
		t.Fatal("a frame that asked for nothing armed the timer")
	}
	frames.stop()
}

// A delayed request (a spinner stepping) is measured from the present, and the
// soonest of several requests in one frame wins.
func TestFrameTimerDelayedRequestWaitsFromThePresent(t *testing.T) {
	frames := newFrameTimer(func() {})
	start := time.Unix(500, 0)
	frames.now = func() time.Time { return start }
	frames.setInterval(0)
	frames.redrawing()
	frames.request(125 * time.Millisecond)
	frames.request(0)
	frames.presented()
	if want := start.Add(minFrameInterval); !frames.due.Equal(want) {
		t.Fatalf("due = %v, want the soonest request, %v", frames.due, want)
	}
	frames.stop()

	frames = newFrameTimer(func() {})
	frames.now = func() time.Time { return start }
	frames.setInterval(0)
	frames.redrawing()
	frames.request(125 * time.Millisecond)
	frames.presented()
	if want := start.Add(125 * time.Millisecond); !frames.due.Equal(want) {
		t.Fatalf("due = %v, want the delayed request from the present, %v", frames.due, want)
	}
	frames.stop()
}

// A window paced by a display interval keeps measuring from the frame start and
// arms while painting: its frames are not paced by the present.
func TestFrameTimerWithDisplayIntervalArmsWhilePainting(t *testing.T) {
	frames := newFrameTimer(func() {})
	start := time.Unix(600, 0)
	now := start
	frames.now = func() time.Time { return now }
	frames.setInterval(20 * time.Millisecond)
	frames.redrawing()
	now = start.Add(5 * time.Millisecond)
	frames.request(0)
	if want := start.Add(20 * time.Millisecond); !frames.due.Equal(want) {
		t.Fatalf("due = %v, want the frame start + 20ms, %v", frames.due, want)
	}
	frames.stop()
}

// BenchmarkFramePresentedFrame measures one present-paced frame's bookkeeping:
// the request a paint makes and the arm its presentation causes.
func BenchmarkFramePresentedFrame(b *testing.B) {
	frames := newFrameTimer(func() {})
	now := time.Unix(700, 0)
	frames.now = func() time.Time { return now }
	frames.setInterval(0)
	b.ReportAllocs()
	for range b.N {
		frames.redrawing()
		frames.request(0)
		now = now.Add(time.Millisecond)
		frames.presented()
		frames.timer.Stop()
		frames.armed.Store(false)
	}
	frames.stop()
}

// A coalescing window leaves a pointer repaint to its frame clock.
func TestCoalescedPointerEventLeavesThePaintToTheFrameClock(t *testing.T) {
	config := Config{Title: "coalesce", Width: 200, Height: 100}
	config.Paint = func(canvas *gfx.Canvas, _ gfx.Size) {
		canvas.HitWithLabel(gfx.Item(1), gfx.Rect{Width: 100, Height: 50}, "Save")
	}
	var canvas gfx.Canvas
	painter := newTestPainter(t, &canvas, config)
	painter.coalesceInput = true
	requests := 0
	painter.scheduleFrame = func(time.Duration) { requests++ }

	frame, err := painter.pointerEvent(platform.PointerEvent{Kind: platform.PointerMove, X: 10, Y: 10})
	if err != nil {
		t.Fatal(err)
	}
	if !frame.Update.Empty() || frame.TextLabels != nil || frame.Accessibility != nil {
		t.Fatalf("a coalesced event presented a frame: %+v", frame)
	}
	if requests != 1 {
		t.Fatalf("frame requests = %d, want one", requests)
	}
	if painter.pointer.Hovered() != gfx.Item(1) {
		t.Fatalf("hovered = %v, want the declared control", painter.pointer.Hovered())
	}

	// The frame clock's repaint publishes the hover.
	if _, err := painter.repaint(); err != nil {
		t.Fatal(err)
	}
	if got := painter.canvas.Hovered(); got != gfx.Item(1) {
		t.Fatalf("painted hover = %v, want the declared control", got)
	}
}
