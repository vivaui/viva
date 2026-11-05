package ui

import (
	"math"
	"testing"
	"time"
)

func TestEasingMatchesCubicBezierReference(t *testing.T) {
	// Reference values solve x(s)=progress by 100-step bisection.
	cases := []struct {
		easing Easing
		points [5]float32
	}{
		{EaseInOut, [5]float32{0.025863, 0.236587, 0.775561, 0.959368, 0.994354}},
		{EaseOut, [5]float32{0.303848, 0.577573, 0.839245, 0.964216, 0.994601}},
		{EaseIn, [5]float32{0.018373, 0.098627, 0.324815, 0.630085, 0.84375}},
	}
	progress := [5]float32{0.1, 0.25, 0.5, 0.75, 0.9}
	for _, test := range cases {
		for index, want := range test.points {
			if got := test.easing.At(progress[index]); math.Abs(float64(got-want)) > 1e-4 {
				t.Errorf("easing %d at %v = %v, want %v", test.easing, progress[index], got, want)
			}
		}
	}
}

func TestEasingEndpointsAreExactAndCurvesMonotonic(t *testing.T) {
	for _, easing := range []Easing{EaseInOut, EaseOut, EaseIn, EaseLinear} {
		if easing.At(-1) != 0 || easing.At(0) != 0 || easing.At(1) != 1 || easing.At(2) != 1 {
			t.Errorf("easing %d end points = %v %v %v %v", easing, easing.At(-1), easing.At(0), easing.At(1), easing.At(2))
		}
		previous := float32(0)
		for step := 1; step <= 1000; step++ {
			value := easing.At(float32(step) / 1000)
			if value < previous-1e-5 || value < 0 || value > 1 {
				t.Fatalf("easing %d is not monotonic within [0,1] at step %d: %v after %v", easing, step, value, previous)
			}
			previous = value
		}
	}
	if EaseLinear.At(0.3) != 0.3 {
		t.Fatalf("linear easing = %v", EaseLinear.At(0.3))
	}
}

func TestColorMixReturnsEndpointsExactlyAndBlendsPremultiplied(t *testing.T) {
	opaque := Color{Red: 1, Alpha: 1}
	clear := Color{Blue: 1, Alpha: 0}
	if opaque.Mix(clear, 0) != opaque || opaque.Mix(clear, 1) != clear || opaque.Mix(clear, -3) != opaque || opaque.Mix(clear, 4) != clear {
		t.Fatal("mix end points must be returned exactly")
	}
	// Fading to a transparent color keeps the hue of the visible side: a
	// non-premultiplied blend would drift toward blue as alpha falls.
	half := opaque.Mix(clear, 0.5)
	if half.Alpha != 0.5 || half.Red != 1 || half.Blue != 0 {
		t.Fatalf("premultiplied mix = %+v", half)
	}
	black, white := Color{Alpha: 1}, Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	if mid := black.Mix(white, 0.5); mid != (Color{Red: 0.5, Green: 0.5, Blue: 0.5, Alpha: 1}) {
		t.Fatalf("opaque mix = %+v", mid)
	}
	if both := (Color{Red: 1}).Mix(Color{Blue: 1}, 0.5); both != (Color{}) {
		t.Fatalf("fully transparent mix = %+v", both)
	}
}

// testFrames drives a canvas on a fake clock, one frame per call.
type testFrames struct {
	canvas *Canvas
	now    time.Duration
}

func newTestFrames(canvas *Canvas) *testFrames {
	frames := &testFrames{canvas: canvas}
	canvas.SetClock(func() time.Duration { return frames.now })
	return frames
}

func (frames *testFrames) at(now time.Duration, paint func()) {
	frames.now = now
	frames.canvas.BeginFrame()
	paint()
	frames.canvas.EndFrame()
}

func TestAnimatePassesThroughWithoutFrameClock(t *testing.T) {
	var canvas Canvas
	motion := DefaultMotion()
	if got := canvas.Animate(Item(1), 0, 1, motion); got != 1 || canvas.Animating() || len(canvas.motion.slots) != 0 {
		t.Fatalf("Animate outside a frame = %v animating=%v slots=%d", got, canvas.Animating(), len(canvas.motion.slots))
	}
	// A frame without a clock cannot animate either.
	canvas.BeginFrame()
	if got := canvas.Animate(Item(1), 0, 1, motion); got != 1 || len(canvas.motion.slots) != 0 {
		t.Fatalf("Animate without a clock = %v slots=%d", got, len(canvas.motion.slots))
	}
	canvas.EndFrame()

	frames := newTestFrames(&canvas)
	frames.at(0, func() {
		if got := canvas.Animate(ID{}, 0, 1, motion); got != 1 || len(canvas.motion.slots) != 0 {
			t.Fatalf("static preview ID must not retain state: %v slots=%d", got, len(canvas.motion.slots))
		}
	})
	if got := canvas.Animate(Item(1), 0, 0, motion); got != 0 || len(canvas.motion.slots) != 0 {
		t.Fatalf("Animate after EndFrame = %v", got)
	}
}

func TestAnimateFirstSightingIsSettledAndZeroDurationIsInstant(t *testing.T) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	var got float32
	frames.at(0, func() { got = canvas.Animate(Item(1), 0, 1, DefaultMotion()) })
	if got != 1 || canvas.Animating() {
		t.Fatalf("first sighting must start at its target, got %v animating=%v", got, canvas.Animating())
	}
	frames.at(10*time.Millisecond, func() { got = canvas.Animate(Item(1), 0, 0, Motion{}) })
	if got != 0 || canvas.Animating() {
		t.Fatalf("zero duration must be instant, got %v animating=%v", got, canvas.Animating())
	}
}

func TestAnimateRunsToTargetAndSettles(t *testing.T) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	motion := Motion{Duration: 100 * time.Millisecond}
	sample := func(at time.Duration, target float32) (float32, bool) {
		var got float32
		frames.at(at, func() { got = canvas.Animate(Item(7), 2, target, motion) })
		return got, canvas.Animating()
	}
	if got, animating := sample(0, 0); got != 0 || animating {
		t.Fatalf("rest = %v animating=%v", got, animating)
	}
	// The frame that observes the new target begins the timeline.
	if got, animating := sample(1000*time.Millisecond, 1); got != 0 || !animating {
		t.Fatalf("start = %v animating=%v", got, animating)
	}
	if got, animating := sample(1050*time.Millisecond, 1); math.Abs(float64(got)-0.775561) > 1e-4 || !animating {
		t.Fatalf("midpoint = %v animating=%v, want eased 0.775561", got, animating)
	}
	if got, animating := sample(1099*time.Millisecond, 1); got >= 1 || !animating {
		t.Fatalf("just before end = %v animating=%v", got, animating)
	}
	if got, animating := sample(1100*time.Millisecond, 1); got != 1 || animating {
		t.Fatalf("end = %v animating=%v", got, animating)
	}
	if got, animating := sample(5*time.Second, 1); got != 1 || animating {
		t.Fatalf("rest after end = %v animating=%v", got, animating)
	}
}

func TestAnimateRetargetContinuesFromDisplayedValue(t *testing.T) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	motion := Motion{Duration: 100 * time.Millisecond, Easing: EaseLinear}
	value := func(at time.Duration, target float32) float32 {
		var got float32
		frames.at(at, func() { got = canvas.Animate(Item(3), 0, target, motion) })
		return got
	}
	value(0, 0)
	value(100*time.Millisecond, 1)
	before := value(160*time.Millisecond, 1)
	if before != 0.6 {
		t.Fatalf("value before reversing = %v", before)
	}
	if reversed := value(160*time.Millisecond, 0); math.Abs(float64(reversed-0.6)) > 1e-6 {
		t.Fatalf("reversing jumped from 0.6 to %v", reversed)
	}
	if got := value(210*time.Millisecond, 0); math.Abs(float64(got-0.3)) > 1e-6 {
		t.Fatalf("halfway back = %v, want 0.3", got)
	}
	if got := value(260*time.Millisecond, 0); got != 0 || canvas.Animating() {
		t.Fatalf("back at rest = %v animating=%v", got, canvas.Animating())
	}
}

func TestAnimateSlotsAreIndependentPerIDAndSlot(t *testing.T) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	motion := Motion{Duration: 100 * time.Millisecond, Easing: EaseLinear}
	frames.at(0, func() {
		canvas.Animate(Item(1), 0, 0, motion)
		canvas.Animate(Item(1), 1, 0, motion)
		canvas.Animate(Item(2), 0, 0, motion)
	})
	var a, b, c float32
	frames.at(10*time.Millisecond, func() {
		a = canvas.Animate(Item(1), 0, 1, motion)
		b = canvas.Animate(Item(1), 1, 0, motion)
		c = canvas.Animate(Item(2), 0, 0, motion)
	})
	frames.at(60*time.Millisecond, func() {
		a = canvas.Animate(Item(1), 0, 1, motion)
		b = canvas.Animate(Item(1), 1, 0, motion)
		c = canvas.Animate(Item(2), 0, 0, motion)
	})
	if a != 0.5 || b != 0 || c != 0 {
		t.Fatalf("slots leaked into each other: %v %v %v", a, b, c)
	}
}

func TestEndFrameForgetsUnpaintedWidgets(t *testing.T) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	motion := Motion{Duration: 100 * time.Millisecond, Easing: EaseLinear}
	frames.at(0, func() { canvas.Animate(Item(1), 0, 0, motion) })
	frames.at(10*time.Millisecond, func() { canvas.Animate(Item(1), 0, 1, motion) })
	if !canvas.Animating() {
		t.Fatal("widget should be mid-transition")
	}
	frames.at(20*time.Millisecond, func() {})
	if len(canvas.motion.slots) != 0 || canvas.Animating() {
		t.Fatalf("hidden widget kept state: slots=%d animating=%v", len(canvas.motion.slots), canvas.Animating())
	}
	var got float32
	frames.at(30*time.Millisecond, func() { got = canvas.Animate(Item(1), 0, 1, motion) })
	if got != 1 || canvas.Animating() {
		t.Fatalf("a widget shown again must not replay its transition, got %v", got)
	}
}

func TestAnimateInsertionKeepsPaintOrder(t *testing.T) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	motion := DefaultMotion()
	paint := func(ids ...uint32) {
		for _, id := range ids {
			canvas.Animate(Item(int(id)), 0, 0, motion)
		}
	}
	frames.at(0, func() { paint(1, 2, 3) })
	frames.at(1, func() { paint(1, 9, 2, 3) })
	order := make([]int, 0, 4)
	for _, slot := range canvas.motion.slots {
		item, _ := slot.id.Name().Index()
		order = append(order, item)
	}
	want := []int{1, 9, 2, 3}
	for index := range want {
		if order[index] != want[index] {
			t.Fatalf("slot order = %v, want %v", order, want)
		}
	}
}

func TestAnimateNeverRunsBackwardsInTime(t *testing.T) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	motion := Motion{Duration: 100 * time.Millisecond, Easing: EaseLinear}
	frames.at(500*time.Millisecond, func() { canvas.Animate(Item(1), 0, 0, motion) })
	frames.at(500*time.Millisecond, func() { canvas.Animate(Item(1), 0, 1, motion) })
	var got float32
	frames.at(400*time.Millisecond, func() { got = canvas.Animate(Item(1), 0, 1, motion) })
	if got != 0 || canvas.Now() != 500*time.Millisecond {
		t.Fatalf("clock regression changed output: value=%v now=%v", got, canvas.Now())
	}
}

func TestReduceMotionJumpsToTarget(t *testing.T) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	canvas.SetReduceMotion(true)
	motion := DefaultMotion()
	frames.at(0, func() { canvas.Animate(Item(1), 0, 0, motion) })
	var got float32
	frames.at(time.Millisecond, func() { got = canvas.Animate(Item(1), 0, 1, motion) })
	if !canvas.ReduceMotion() || got != 1 || canvas.Animating() {
		t.Fatalf("reduced motion = %v animating=%v", got, canvas.Animating())
	}
}

func TestRequestFrameSurvivesEndFrameUntilNextBegin(t *testing.T) {
	var canvas Canvas
	canvas.BeginFrame()
	canvas.RequestFrame()
	canvas.EndFrame()
	if !canvas.Animating() {
		t.Fatal("RequestFrame should leave the frame unfinished")
	}
	canvas.BeginFrame()
	canvas.EndFrame()
	if canvas.Animating() {
		t.Fatal("a new frame must start at rest")
	}
}

func TestPlayOnceRunsOneGestureAndSettles(t *testing.T) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	var progress float32
	var playing bool
	sample := func(at time.Duration, trigger bool) {
		frames.at(at, func() { progress, playing = canvas.PlayOnce(Item(1), 0, trigger, 200*time.Millisecond) })
	}
	sample(0, false)
	if progress != 1 || playing {
		t.Fatalf("rest = %v playing=%v", progress, playing)
	}
	sample(100*time.Millisecond, true)
	if progress != 0 || !playing || !canvas.Animating() {
		t.Fatalf("trigger = %v playing=%v animating=%v", progress, playing, canvas.Animating())
	}
	sample(200*time.Millisecond, true)
	if progress != 0.5 || !playing {
		t.Fatalf("half way = %v playing=%v", progress, playing)
	}
	sample(300*time.Millisecond, true)
	if progress != 1 || playing || canvas.Animating() {
		t.Fatalf("end = %v playing=%v animating=%v", progress, playing, canvas.Animating())
	}
	sample(400*time.Millisecond, false)
	if progress != 1 || playing {
		t.Fatalf("settled = %v playing=%v", progress, playing)
	}
}

func TestPlayOnceDoesNotLoopWhileTriggerStaysTrue(t *testing.T) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	for step := range 8 {
		at := time.Duration(step) * 50 * time.Millisecond
		var progress float32
		frames.at(at, func() {
			progress, _ = canvas.PlayOnce(Item(1), 0, true, 200*time.Millisecond)
		})
		if step == 5 && progress != 1 {
			t.Fatalf("a held trigger restarted at %v after the gesture ended", progress)
		}
	}
}

func TestPlayOnceRestartsAfterRest(t *testing.T) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	var progress float32
	frames.at(0, func() { progress, _ = canvas.PlayOnce(Item(1), 0, true, 100*time.Millisecond) })
	frames.at(100*time.Millisecond, func() { progress, _ = canvas.PlayOnce(Item(1), 0, false, 100*time.Millisecond) })
	if progress != 1 {
		t.Fatalf("after the first pass = %v", progress)
	}
	frames.at(150*time.Millisecond, func() { progress, _ = canvas.PlayOnce(Item(1), 0, true, 100*time.Millisecond) })
	if progress != 0 {
		t.Fatalf("a trigger after rest began at %v", progress)
	}
}

func TestPlayOnceFinishesWhenTheTriggerIsReleased(t *testing.T) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	var progress float32
	var playing bool
	sample := func(at time.Duration, trigger bool) {
		frames.at(at, func() { progress, playing = canvas.PlayOnce(Item(1), 0, trigger, 200*time.Millisecond) })
	}
	sample(0, true)
	sample(50*time.Millisecond, false)
	if progress != 0.25 || !playing {
		t.Fatalf("after releasing at a quarter = %v playing=%v", progress, playing)
	}
	sample(150*time.Millisecond, false)
	if progress != 0.75 || !playing {
		t.Fatalf("after releasing at three quarters = %v playing=%v", progress, playing)
	}
	sample(200*time.Millisecond, false)
	if progress != 1 || playing {
		t.Fatalf("after the released pass = %v playing=%v", progress, playing)
	}
}

func TestPlayOnceSlotsAreIndependentPerIDAndSlot(t *testing.T) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	var first, second, other float32
	frames.at(0, func() {
		first, _ = canvas.PlayOnce(Item(1), 0, true, 200*time.Millisecond)
		second, _ = canvas.PlayOnce(Item(1), 1, false, 200*time.Millisecond)
		other, _ = canvas.PlayOnce(Item(2), 0, true, 200*time.Millisecond)
	})
	frames.at(100*time.Millisecond, func() {
		first, _ = canvas.PlayOnce(Item(1), 0, true, 200*time.Millisecond)
		second, _ = canvas.PlayOnce(Item(1), 1, true, 200*time.Millisecond)
		other, _ = canvas.PlayOnce(Item(2), 0, false, 200*time.Millisecond)
	})
	if first != 0.5 || second != 0 || other != 0.5 {
		t.Fatalf("slots leaked into each other: %v %v %v", first, second, other)
	}
}

func TestPlayOnceHonoursReducedMotionAndMissingClock(t *testing.T) {
	var canvas Canvas
	if progress, playing := canvas.PlayOnce(Item(1), 0, true, time.Second); progress != 1 || playing {
		t.Fatalf("without a clock = %v playing=%v", progress, playing)
	}
	frames := newTestFrames(&canvas)
	canvas.SetReduceMotion(true)
	frames.at(0, func() {
		if progress, playing := canvas.PlayOnce(Item(1), 0, true, time.Second); progress != 1 || playing {
			t.Fatalf("reduced motion = %v playing=%v", progress, playing)
		}
	})
	if len(canvas.motion.onces) != 1 || canvas.motion.onces[0].active {
		t.Fatalf("reduced motion kept an active pass: %+v", canvas.motion.onces)
	}
}

func TestPlayOnceForgetsUnpaintedWidgets(t *testing.T) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	frames.at(0, func() { canvas.PlayOnce(Item(1), 0, true, time.Second) })
	frames.at(10*time.Millisecond, func() {})
	if len(canvas.motion.onces) != 0 {
		t.Fatalf("a widget that stopped painting kept %d passes", len(canvas.motion.onces))
	}
}

func TestClockIsReadOnlyWhenSomethingAnimatesAndOncePerFrame(t *testing.T) {
	var canvas Canvas
	reads := 0
	canvas.SetClock(func() time.Duration {
		reads++
		return time.Duration(reads) * time.Millisecond
	})
	motion := DefaultMotion()
	for range 3 {
		canvas.BeginFrame()
		canvas.EndFrame()
	}
	if reads != 0 {
		t.Fatalf("frames without animated widgets read the clock %d times", reads)
	}
	canvas.BeginFrame()
	canvas.Animate(Item(1), 0, 0, motion)
	canvas.Animate(Item(2), 0, 1, motion)
	canvas.Animate(Item(3), 0, 1, motion)
	now := canvas.Now()
	canvas.EndFrame()
	if reads != 1 || now != time.Millisecond {
		t.Fatalf("one frame read the clock %d times (now=%v)", reads, now)
	}
}

func TestMotionStateShrinksAfterSpike(t *testing.T) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	motion := DefaultMotion()
	frames.at(0, func() {
		for id := uint32(1); id <= 4096; id++ {
			canvas.Animate(Item(int(id)), 0, 0, motion)
		}
	})
	frames.at(1, func() { canvas.Animate(Item(1), 0, 0, motion) })
	if cap(canvas.motion.slots) > 8 {
		t.Fatalf("retained %d slots of capacity after a spike", cap(canvas.motion.slots))
	}
}

func TestWarmAnimateDoesNotAllocate(t *testing.T) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	motion := DefaultMotion()
	paint := func(target float32) {
		for id := uint32(1); id <= 64; id++ {
			canvas.Animate(Item(int(id)), 0, target, motion)
		}
	}
	frames.at(0, func() { paint(0) })
	now := time.Duration(0)
	allocations := testing.AllocsPerRun(100, func() {
		now += 8 * time.Millisecond
		frames.at(now, func() { paint(float32(now / (150 * time.Millisecond) % 2)) })
	})
	if allocations != 0 {
		t.Fatalf("warm animated frames allocate %v times", allocations)
	}
}

func BenchmarkAnimateSettled64(b *testing.B) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	motion := DefaultMotion()
	frames.at(0, func() {
		for id := uint32(1); id <= 64; id++ {
			canvas.Animate(Item(int(id)), 0, 1, motion)
		}
	})
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		frames.now += 8 * time.Millisecond
		canvas.BeginFrame()
		for id := uint32(1); id <= 64; id++ {
			canvas.Animate(Item(int(id)), 0, 1, motion)
		}
		canvas.EndFrame()
	}
}

func BenchmarkAnimateTransitioning64(b *testing.B) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	motion := DefaultMotion()
	frames.at(0, func() {
		for id := uint32(1); id <= 64; id++ {
			canvas.Animate(Item(int(id)), 0, 0, motion)
		}
	})
	b.ReportAllocs()
	b.ResetTimer()
	for index := range b.N {
		frames.now += 8 * time.Millisecond
		target := float32(index & 1)
		canvas.BeginFrame()
		for id := uint32(1); id <= 64; id++ {
			canvas.Animate(Item(int(id)), 0, target, motion)
		}
		canvas.EndFrame()
	}
}

func BenchmarkEasingInOut(b *testing.B) {
	b.ReportAllocs()
	var sink float32
	for index := range b.N {
		sink += EaseInOut.At(float32(index&1023) / 1024)
	}
	_ = sink
}

// BenchmarkFrameBracketWithoutAnimation is the cost every repaint pays for
// the capability when no widget animates.
func BenchmarkFrameBracketWithoutAnimation(b *testing.B) {
	var canvas Canvas
	canvas.SetClock(func() time.Duration { return 0 })
	b.ReportAllocs()
	for range b.N {
		canvas.BeginFrame()
		canvas.EndFrame()
	}
}

func TestColorOverCompositesLikeTheGPU(t *testing.T) {
	backdrop := Color{Red: 0.2, Green: 0.4, Blue: 0.6, Alpha: 1}
	if got := (Color{Red: 1, Alpha: 1}).Over(backdrop); got != (Color{Red: 1, Alpha: 1}) {
		t.Fatalf("opaque color changed: %+v", got)
	}
	if got := (Color{Red: 1, Alpha: 0}).Over(backdrop); got != backdrop {
		t.Fatalf("transparent color must leave the backdrop: %+v", got)
	}
	got := (Color{Red: 1, Green: 1, Blue: 1, Alpha: 0.25}).Over(backdrop)
	want := Color{Red: 0.4, Green: 0.55, Blue: 0.7, Alpha: 1}
	for name, pair := range map[string][2]float32{"red": {got.Red, want.Red}, "green": {got.Green, want.Green}, "blue": {got.Blue, want.Blue}, "alpha": {got.Alpha, want.Alpha}} {
		if d := pair[0] - pair[1]; d > 1e-6 || d < -1e-6 {
			t.Fatalf("%s = %v, want %v", name, pair[0], pair[1])
		}
	}
}

func TestNextFrameSaysWhenToPaintAgain(t *testing.T) {
	var canvas Canvas
	frame := func(paint func()) {
		canvas.BeginFrame()
		paint()
		canvas.EndFrame()
	}

	frame(func() {})
	if delay, ok := canvas.NextFrame(); ok || delay != 0 {
		t.Fatalf("a frame that asked for nothing scheduled %v, %v", delay, ok)
	}

	frame(func() { canvas.RequestFrameIn(250 * time.Millisecond) })
	if delay, ok := canvas.NextFrame(); !ok || delay != 250*time.Millisecond {
		t.Fatalf("RequestFrameIn(250ms) scheduled %v, %v", delay, ok)
	}
	if canvas.Animating() {
		t.Fatal("a delayed request is not a transition in flight")
	}

	// The soonest of several requests wins.
	frame(func() {
		canvas.RequestFrameIn(300 * time.Millisecond)
		canvas.RequestFrameIn(100 * time.Millisecond)
		canvas.RequestFrameIn(200 * time.Millisecond)
	})
	if delay, _ := canvas.NextFrame(); delay != 100*time.Millisecond {
		t.Fatalf("soonest request = %v, want 100ms", delay)
	}

	// A transition in flight paints at display cadence whatever else was asked.
	frame(func() {
		canvas.RequestFrameIn(100 * time.Millisecond)
		canvas.RequestFrame()
	})
	if delay, ok := canvas.NextFrame(); !ok || delay != 0 || !canvas.Animating() {
		t.Fatalf("with a transition in flight: %v, %v, animating %v", delay, ok, canvas.Animating())
	}
	frame(func() { canvas.RequestFrameIn(0) })
	if delay, ok := canvas.NextFrame(); !ok || delay != 0 {
		t.Fatalf("RequestFrameIn(0) scheduled %v, %v, want an immediate frame", delay, ok)
	}

	// A frame has to ask again: nothing carries over.
	frame(func() {})
	if _, ok := canvas.NextFrame(); ok {
		t.Fatal("a request outlived its frame")
	}
}

// The built-in easings are these curves; the constructor has to agree with them
// within rounding, or a motion that mixes the two would see its curve segments
// step against the ones Animate eases.
func TestCubicBezierMatchesTheBuiltInCurves(t *testing.T) {
	cases := []struct {
		easing Easing
		curve  Curve
	}{
		{EaseInOut, CubicBezier(0.4, 0, 0.2, 1)},
		{EaseOut, CubicBezier(0, 0, 0.2, 1)},
		{EaseIn, CubicBezier(0.4, 0, 1, 1)},
		{EaseCSS, CubicBezier(0.25, 0.1, 0.25, 1)},
		{EaseEmphasized, CubicBezier(0.22, 1, 0.36, 1)},
	}
	for _, test := range cases {
		for step := range 101 {
			progress := float32(step) / 100
			// The constructor takes its control points as float32, so the same
			// curve built from the same numbers differs from the built-in one by
			// the width of that conversion and by the solver's tolerance.
			if got, want := test.curve(progress), test.easing.At(progress); math.Abs(float64(got-want)) > 1e-6 {
				t.Fatalf("%v at %.2f = %v, want %v", test.easing, progress, got, want)
			}
		}
	}
}

// A curve is asked for progress beyond its ends whenever a motion overshoots a
// keyframe: the ends themselves must be exact, not solved for.
func TestCubicBezierClampsItsEnds(t *testing.T) {
	curve := CubicBezier(0.2, 0, 0.3, 1)
	if got := curve(-2); got != 0 {
		t.Fatalf("progress -2 = %v, want 0", got)
	}
	if got := curve(0); got != 0 {
		t.Fatalf("progress 0 = %v, want 0", got)
	}
	if got := curve(1); got != 1 {
		t.Fatalf("progress 1 = %v, want 1", got)
	}
	if got := curve(3); got != 1 {
		t.Fatalf("progress 3 = %v, want 1", got)
	}
	// y is free, which is how a curve overshoots: a control point above 1 has to
	// leave the eased value above the straight line.
	overshoot := CubicBezier(0.1, 1.8, 0.9, 1)
	if got := overshoot(0.15); got <= 0.3 {
		t.Fatalf("an overshooting curve reached only %v at 0.15", got)
	}
}

// A spring overshoots the value it moves to and settles back onto it, and a
// calm spring does not overshoot at all.
func TestSpringOvershootsAndSettles(t *testing.T) {
	bouncy := Spring(1)
	if got := bouncy(0); got != 0 {
		t.Fatalf("a spring starts at %v, want 0", got)
	}
	if got := bouncy(1); got != 1 {
		t.Fatalf("a spring ends at %v, want 1", got)
	}
	peak := float32(0)
	for step := 0; step <= 200; step++ {
		value := bouncy(float32(step) / 200)
		if value < -0.05 {
			t.Fatalf("a spring dips to %.3f at %.2f, want no visible undershoot", value, float32(step)/200)
		}
		peak = max(peak, value)
	}
	if peak < 1.05 {
		t.Fatalf("a bouncy spring peaks at %.3f, want an overshoot past 1", peak)
	}
	calm := Spring(0)
	for step := 0; step <= 200; step++ {
		value := calm(float32(step) / 200)
		if value > 1.001 || value < -0.001 {
			t.Fatalf("a critically damped spring reaches %.4f at %.2f, want it to stay inside [0, 1]", value, float32(step)/200)
		}
	}
}
