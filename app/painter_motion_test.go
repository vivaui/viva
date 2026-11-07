package app

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

// motionHarness drives a painter with a fake frame clock and counts the
// frames the painter asks the runtime for.
type motionHarness struct {
	painter  *painter
	canvas   *ui.Canvas
	now      time.Duration
	requests int
	checked  bool
	theme    ui.Theme
}

func newMotionHarness(t *testing.T) *motionHarness {
	t.Helper()
	harness := &motionHarness{canvas: &ui.Canvas{}, now: time.Second, theme: ui.LightTheme()}
	config := Config{Title: "motion", Width: 120, Height: 60}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		components.BeginSwitch(canvas, harness.theme, ui.Item(1), ui.Rect{X: 10, Y: 10}, harness.checked).Paint()
	}
	onClick(&config, func(id ui.ID) { harness.checked = !harness.checked })
	harness.painter = newTestPainter(t, harness.canvas, config)
	harness.canvas.SetClock(func() time.Duration { return harness.now })
	harness.painter.scheduleFrame = func(time.Duration) { harness.requests++ }
	return harness
}

// frame plays the role of the platform's redraw callback.
func (harness *motionHarness) frame(t *testing.T, advance time.Duration) platform.Frame {
	t.Helper()
	harness.now += advance
	frame, err := harness.painter.repaint()
	if err != nil {
		t.Fatal(err)
	}
	return frame
}

func (harness *motionHarness) click(t *testing.T) platform.Frame {
	t.Helper()
	var frame platform.Frame
	for _, kind := range []platform.PointerEventKind{platform.PointerMove, platform.PointerDown, platform.PointerUp} {
		var err error
		frame, err = harness.painter.pointerEvent(platform.PointerEvent{Kind: kind, X: 26, Y: 19})
		if err != nil {
			t.Fatal(err)
		}
	}
	return frame
}

func TestIdlePainterSchedulesNoFrames(t *testing.T) {
	harness := newMotionHarness(t)
	for range 5 {
		if frame := harness.frame(t, 16*time.Millisecond); !frame.Update.Empty() {
			t.Fatalf("an unchanged repaint produced renderer work: %+v", frame.Update)
		}
	}
	if harness.requests != 0 || harness.canvas.Animating() {
		t.Fatalf("idle painter requested %d frames, animating=%v", harness.requests, harness.canvas.Animating())
	}
}

func TestClickSchedulesFramesUntilTransitionSettles(t *testing.T) {
	harness := newMotionHarness(t)
	duration := harness.theme.Transition.Duration
	offThumb := harness.canvas.Rectangles()[1].Bounds.X

	harness.click(t)
	if !harness.checked {
		t.Fatal("click did not toggle the application state")
	}
	if harness.requests == 0 || !harness.canvas.Animating() {
		t.Fatalf("a state change must start the frame loop: requests=%d animating=%v", harness.requests, harness.canvas.Animating())
	}
	if harness.canvas.Rectangles()[1].Bounds.X != offThumb {
		t.Fatal("the first frame of a transition must still show the old position")
	}

	scheduled := harness.requests
	frame := harness.frame(t, duration/2)
	if frame.Update.Empty() {
		t.Fatal("a mid-flight frame must update the moving thumb")
	}
	if got := harness.canvas.Rectangles()[1].Bounds.X; got <= offThumb {
		t.Fatalf("thumb did not move: %v", got)
	}
	if harness.requests != scheduled+1 {
		t.Fatalf("mid-flight frame requested %d more frames, want 1", harness.requests-scheduled)
	}

	scheduled = harness.requests
	frame = harness.frame(t, duration)
	if frame.Update.Empty() || harness.canvas.Animating() {
		t.Fatalf("final frame: empty=%v animating=%v", frame.Update.Empty(), harness.canvas.Animating())
	}
	if harness.requests != scheduled {
		t.Fatal("the settling frame must not request another frame")
	}

	frame = harness.frame(t, 500*time.Millisecond)
	if !frame.Update.Empty() || harness.requests != scheduled {
		t.Fatalf("after settling the painter must be idle: update=%+v requests=%d", frame.Update, harness.requests-scheduled)
	}
}

func TestMidFlightFramesDoNotAllocate(t *testing.T) {
	harness := newMotionHarness(t)
	harness.theme.Transition.Duration = 10 * time.Minute
	harness.click(t)
	harness.frame(t, time.Second)
	allocations := testing.AllocsPerRun(200, func() {
		harness.now += 8 * time.Millisecond
		if _, err := harness.painter.repaint(); err != nil {
			t.Fatal(err)
		}
	})
	if allocations != 0 || !harness.canvas.Animating() {
		t.Fatalf("mid-flight frames allocated %v times (animating=%v)", allocations, harness.canvas.Animating())
	}
}

func TestReducedMotionPaintsDestinationWithoutFrames(t *testing.T) {
	harness := newMotionHarness(t)
	harness.canvas.SetReduceMotion(true)
	harness.click(t)
	if harness.requests != 0 || harness.canvas.Animating() {
		t.Fatalf("reduced motion requested %d frames", harness.requests)
	}
	var onCanvas ui.Canvas
	components.BeginSwitch(&onCanvas, harness.theme, ui.Item(1), ui.Rect{X: 10, Y: 10}, true).Paint()
	if harness.canvas.Rectangles()[1] != onCanvas.Rectangles()[1] {
		t.Fatal("reduced motion must paint the destination state immediately")
	}
}

func TestFrameTimerCoalescesRequestsIntoOneWake(t *testing.T) {
	var wakes atomic.Int32
	woken := make(chan struct{}, 8)
	frames := newFrameTimer(func() {
		wakes.Add(1)
		woken <- struct{}{}
	})
	defer frames.stop()

	for range 10 {
		frames.request(0)
	}
	waitForWake(t, woken)
	if wakes.Load() != 1 {
		t.Fatalf("10 coalesced requests produced %d wakes", wakes.Load())
	}

	// The loop continues only because the repaint asks again.
	select {
	case <-woken:
		t.Fatal("timer re-armed itself without a request")
	case <-time.After(4 * frameInterval):
	}
	frames.request(0)
	waitForWake(t, woken)
	if wakes.Load() != 2 {
		t.Fatalf("second request produced %d total wakes", wakes.Load())
	}
}

func TestFrameTimerStopCancelsPendingWake(t *testing.T) {
	var wakes atomic.Int32
	frames := newFrameTimer(func() { wakes.Add(1) })
	frames.request(0)
	frames.stop()
	frames.request(0)
	time.Sleep(6 * frameInterval)
	if wakes.Load() != 0 {
		t.Fatalf("stopped timer woke the loop %d times", wakes.Load())
	}
}

func TestFrameLoopRunsThroughRealTimerAndSettles(t *testing.T) {
	harness := newMotionHarness(t)
	woken := make(chan struct{}, 1)
	frames := newFrameTimer(func() { woken <- struct{}{} })
	defer frames.stop()
	harness.painter.scheduleFrame = frames.request
	started := time.Now()
	harness.canvas.SetClock(func() time.Duration { return harness.now + time.Since(started) })

	harness.click(t)
	redraws := 0
	for harness.canvas.Animating() {
		waitForWake(t, woken)
		if _, err := harness.painter.repaint(); err != nil {
			t.Fatal(err)
		}
		redraws++
		if redraws > 200 {
			t.Fatal("frame loop failed to settle")
		}
	}
	// 150ms at 120 Hz is at most ~19 wakes; allow generous timer slack but
	// require that the loop is bounded by wall time, not spinning.
	if redraws < 3 || redraws > 40 {
		t.Fatalf("transition took %d redraws", redraws)
	}
	select {
	case <-woken:
		t.Fatal("frame loop kept waking after the transition settled")
	case <-time.After(4 * frameInterval):
	}
}

func waitForWake(t *testing.T, woken <-chan struct{}) {
	t.Helper()
	select {
	case <-woken:
	case <-time.After(2 * time.Second):
		t.Fatal("frame timer never woke the loop")
	}
}

func BenchmarkPainterAnimatedSwitchFrame(b *testing.B) {
	harness := &motionHarness{canvas: &ui.Canvas{}, now: time.Second, theme: ui.DarkTheme()}
	config := Config{Title: "motion", Width: 120, Height: 60}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		components.BeginSwitch(canvas, harness.theme, ui.Item(1), ui.Rect{X: 10, Y: 10}, harness.checked).Paint()
	}
	normalized, err := config.normalize()
	if err != nil {
		b.Fatal(err)
	}
	harness.painter, err = newPainter(harness.canvas, &normalized)
	if err != nil {
		b.Fatal(err)
	}
	harness.canvas.SetClock(func() time.Duration { return harness.now })
	harness.painter.scheduleFrame = func(time.Duration) {}
	b.ReportAllocs()
	b.ResetTimer()
	for index := range b.N {
		harness.now += 8 * time.Millisecond
		if index%20 == 0 {
			harness.checked = !harness.checked
		}
		if _, err := harness.painter.repaint(); err != nil {
			b.Fatal(err)
		}
	}
}

// A frame that asks for a later frame schedules it with that delay, and the
// application then sits idle until it is painted: no per-frame cadence.
func TestPainterSchedulesADelayedFrameWithItsDelay(t *testing.T) {
	var canvas ui.Canvas
	asked := true
	config := Config{Title: "pulse", Width: 100, Height: 50}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.Fill(ui.Rect{Width: 10, Height: 10}, ui.Color{Alpha: 1})
		if asked {
			canvas.RequestFrameIn(125 * time.Millisecond)
		}
	}
	painter := newTestPainter(t, &canvas, config)
	var delays []time.Duration
	painter.scheduleFrame = func(delay time.Duration) { delays = append(delays, delay) }

	if _, err := painter.repaint(); err != nil {
		t.Fatal(err)
	}
	asked = false
	if _, err := painter.repaint(); err != nil {
		t.Fatal(err)
	}
	if len(delays) != 1 || delays[0] != 125*time.Millisecond {
		t.Fatalf("scheduled delays = %v, want one of 125ms and none once the paint stops asking", delays)
	}
}

func TestFrameTimerWaitsForTheRequestedDelay(t *testing.T) {
	woken := make(chan struct{}, 8)
	frames := newFrameTimer(func() { woken <- struct{}{} })
	defer frames.stop()

	const delay = 60 * time.Millisecond
	start := time.Now()
	frames.request(delay)
	waitForWake(t, woken)
	if elapsed := time.Since(start); elapsed < delay {
		t.Fatalf("woke after %v, before the %v asked for", elapsed, delay)
	}
}

// An animation that begins while a slow tick is pending is not held back by it.
func TestFrameTimerBringsAPendingWakeForward(t *testing.T) {
	woken := make(chan struct{}, 8)
	frames := newFrameTimer(func() { woken <- struct{}{} })
	defer frames.stop()

	start := time.Now()
	frames.request(5 * time.Second)
	frames.request(0)
	waitForWake(t, woken)
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("the immediate request waited %v behind a 5s tick", elapsed)
	}
	// And a later request does not push a sooner pending wake back.
	frames.request(0)
	frames.request(5 * time.Second)
	start = time.Now()
	waitForWake(t, woken)
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("a slow request delayed a pending frame by %v", elapsed)
	}
}
