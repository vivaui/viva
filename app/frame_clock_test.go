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

// A zero interval means presenting a frame blocks; the timer asks again as soon
// as the frame is painted, with no display-sized pause of its own.
func TestFrameTimerWithPresentPacedInterval(t *testing.T) {
	var wakes int
	frames := newFrameTimer(func() { wakes++ })
	start := time.Unix(200, 0)
	now := start
	frames.now = func() time.Time { return now }
	frames.setInterval(0)
	frames.redrawing()
	frames.request(0)
	want := start.Add(minFrameInterval)
	if !frames.due.Equal(want) {
		t.Fatalf("due = %v, want %v (the safety floor)", frames.due, want)
	}
	if frames.interval != 0 {
		t.Fatalf("interval = %v, want present-paced zero", frames.interval)
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
