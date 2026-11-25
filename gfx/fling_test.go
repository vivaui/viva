package gfx

import (
	"math"
	"testing"
	"time"
)

func TestVelocityTrackerMeasuresASteadyDrag(t *testing.T) {
	var tracker VelocityTracker
	for step := range 6 {
		tracker.Add(float32(step)*10, time.Duration(step)*10*time.Millisecond) // 1000 units/s
	}
	if got := tracker.Velocity(); math.Abs(float64(got-1000)) > 1 {
		t.Fatalf("velocity = %v, want 1000", got)
	}
}

func TestVelocityTrackerIgnoresAPointerThatRestedBeforeRelease(t *testing.T) {
	var tracker VelocityTracker
	for step := range 5 {
		tracker.Add(float32(step)*30, time.Duration(step)*10*time.Millisecond)
	}
	tracker.Add(120, 400*time.Millisecond) // rested 360ms, then a single sample
	if got := tracker.Velocity(); got != 0 {
		t.Fatalf("velocity after resting = %v, want 0", got)
	}
}

func TestVelocityTrackerNeedsTwoSamplesAndForgetsOnReset(t *testing.T) {
	var tracker VelocityTracker
	if tracker.Velocity() != 0 {
		t.Fatal("empty tracker has velocity")
	}
	tracker.Add(5, 0)
	if tracker.Velocity() != 0 {
		t.Fatal("one sample has velocity")
	}
	tracker.Add(15, 10*time.Millisecond)
	tracker.Reset()
	if tracker.Velocity() != 0 {
		t.Fatal("reset tracker has velocity")
	}
}

func TestVelocityTrackerKeepsTheNewestSamplesWhenItWraps(t *testing.T) {
	var tracker VelocityTracker
	for step := range 40 { // far more than it holds; speed doubles at step 30
		value := float32(step) * 10
		if step >= 30 {
			value = 300 + float32(step-30)*20
		}
		tracker.Add(value, time.Duration(step)*5*time.Millisecond)
	}
	if got := tracker.Velocity(); got < 3000 {
		t.Fatalf("velocity = %v, want the recent 4000/s rather than the earlier 2000/s", got)
	}
}

func TestVelocityTrackerIgnoresTimeRunningBackward(t *testing.T) {
	var tracker VelocityTracker
	tracker.Add(0, 20*time.Millisecond)
	tracker.Add(10, 30*time.Millisecond)
	tracker.Add(999, 10*time.Millisecond)
	if got := tracker.Velocity(); math.Abs(float64(got-1000)) > 1 {
		t.Fatalf("velocity = %v, want 1000", got)
	}
}

func TestProjectIsTheDistanceAThrownValueCovers(t *testing.T) {
	if got := Project(100, 0, DecelerationNormal); got != 100 {
		t.Fatalf("at rest projects to %v", got)
	}
	slow, fast := Project(0, 1000, DecelerationNormal), Project(0, 2000, DecelerationNormal)
	if math.Abs(float64(fast-2*slow)) > 1e-2 || slow <= 0 {
		t.Fatalf("projections %v and %v are not proportional to speed", slow, fast)
	}
	if Project(0, -1000, DecelerationNormal) != -slow {
		t.Fatal("a throw backward does not mirror a throw forward")
	}
	if Project(0, 1000, DecelerationFast) >= slow {
		t.Fatal("a faster stop slides as far as a normal one")
	}
}

func TestNearestPicksTheClosestPoint(t *testing.T) {
	if got := Nearest(130, 0, 100, 200); got != 100 {
		t.Fatalf("nearest = %v", got)
	}
	if got := Nearest(150, 0, 100, 200); got != 100 {
		t.Fatalf("a tie takes the first: %v", got)
	}
	if got := Nearest(42); got != 42 {
		t.Fatalf("no points returns the value, got %v", got)
	}
}

func TestRubberBandResistsMoreTheFurtherItIsPulledAndNeverReachesTheLimit(t *testing.T) {
	const limit = 100
	if RubberBand(-20, limit) != -20 || RubberBand(0, limit) != 0 {
		t.Fatal("a value inside its range must pass through")
	}
	previous, previousStep := float32(0), float32(math.MaxFloat32)
	for offset := float32(10); offset <= 1000; offset += 10 {
		got := RubberBand(offset, limit)
		if got <= previous || got >= limit {
			t.Fatalf("band(%v) = %v: not increasing below the limit", offset, got)
		}
		if step := got - previous; step >= previousStep+1e-6 {
			t.Fatalf("band(%v) gains %v, more than the %v before it", offset, step, previousStep)
		}
		previous, previousStep = got, got-previous
	}
}

func TestThrowStartsTheNextAnimationAtTheReleaseSpeed(t *testing.T) {
	motion := SpringMotion(400*time.Millisecond, 0)
	after30ms := func(speed float32) (got float32) {
		var canvas Canvas
		frames := newTestFrames(&canvas)
		frames.at(0, func() { canvas.Animate(Item(1), 0, 0, motion) })
		canvas.Throw(Item(1), 0, 0, speed)
		frames.at(time.Second, func() { canvas.Animate(Item(1), 0, 100, motion) })
		frames.at(time.Second+30*time.Millisecond, func() { got = canvas.Animate(Item(1), 0, 100, motion) })
		return got
	}
	still, thrown := after30ms(0), after30ms(2000)
	if thrown <= still+1 {
		t.Fatalf("after 30ms a thrown value is at %v, one released at rest at %v", thrown, still)
	}
}

func TestThrownSpeedIsUsedOnceAndNotKeptForALaterMove(t *testing.T) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	motion := SpringMotion(400*time.Millisecond, 0)
	value := func(at time.Duration, target float32) (got float32) {
		frames.at(at, func() { got = canvas.Animate(Item(1), 0, target, motion) })
		return got
	}
	value(0, 0)
	canvas.Throw(Item(1), 0, 0, 5000)
	value(time.Second, 0) // released on its target: the speed is dropped
	value(2*time.Second, 100)
	if got := value(2*time.Second+10*time.Millisecond, 100); got > 5 {
		t.Fatalf("a later move starts with the old throw's speed: %v after 10ms", got)
	}
}

func TestThrowIsIgnoredWithoutAClockAndUnderReducedMotion(t *testing.T) {
	var canvas Canvas
	canvas.Throw(Item(1), 0, 0, 100) // no clock: nothing to keep
	frames := newTestFrames(&canvas)
	canvas.SetReduceMotion(true)
	canvas.Throw(Item(1), 0, 0, 100)
	var got float32
	frames.at(time.Second, func() { got = canvas.Animate(Item(1), 0, 50, SpringMotion(time.Second, 0)) })
	if got != 50 {
		t.Fatalf("reduced motion = %v, want the target", got)
	}
}

func BenchmarkVelocityTrackerAddAndRead(b *testing.B) {
	var tracker VelocityTracker
	at := time.Duration(0)
	b.ReportAllocs()
	for b.Loop() {
		at += 8 * time.Millisecond
		tracker.Add(float32(at/time.Millisecond), at)
		_ = tracker.Velocity()
	}
}
