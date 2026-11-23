package interaction

import (
	"math"
	"testing"
	"time"
)

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-4 }

// Two fingers held together are a pinch: it begins where they are, follows how far apart
// they go as a multiple of the start and a factor since the last step, and ends where they
// were when one lifts.
func TestTwoFingersPinch(t *testing.T) {
	var touch Touch
	now := time.Duration(0)
	touch.Down(1, 100, 200, now, false)
	begin := touch.Down(2, 200, 200, now, false)
	if begin.Pinch != PinchBegin || begin.PinchX != 150 || begin.PinchY != 200 || begin.PinchScale != 1 {
		t.Fatalf("begin = %+v, want a pinch at 150, 200 with scale 1", begin)
	}
	if !touch.Active() {
		t.Fatal("a pinch is a gesture in progress")
	}
	change := touch.Move(2, 300, 200, now) // 100 apart became 200
	if change.Pinch != PinchChange || !near(change.PinchScale, 2) || !near(change.PinchDelta, 2) || change.PinchX != 200 {
		t.Fatalf("change = %+v, want scale 2 and delta 2 around x 200", change)
	}
	again := touch.Move(1, 50, 200, now) // 250 apart: 1.25 of the 200
	if !near(again.PinchScale, 2.5) || !near(again.PinchDelta, 1.25) {
		t.Fatalf("again = %+v, want scale 2.5 and delta 1.25", again)
	}
	end := touch.Up(2, 300, 200, now)
	if end.Pinch != PinchEnd || !near(end.PinchScale, 2.5) {
		t.Fatalf("end = %+v, want the pinch to end at scale 2.5", end)
	}
	if touch.Active() {
		t.Fatal("the pinch is over")
	}
}

// What is left after a pinch is no gesture: the finger that stays down neither presses nor
// scrolls, and the next touch starts fresh.
func TestTheFingerLeftAfterAPinchIsNoGesture(t *testing.T) {
	var touch Touch
	now := time.Duration(0)
	touch.Down(1, 100, 200, now, false)
	touch.Down(2, 200, 200, now, false)
	touch.Up(2, 200, 200, now)
	for _, step := range []TouchStep{touch.Move(1, 100, 400, now), touch.Up(1, 100, 400, now), touch.Advance(now + time.Second)} {
		if step.Press || step.Release || step.Scrolls() || step.Pinch != PinchNone {
			t.Fatalf("the finger left after a pinch did something: %+v", step)
		}
	}
	if step := touch.Down(3, 10, 10, now+2*time.Second, false); !step.Wake {
		t.Fatalf("a new touch after a pinch did not start a gesture: %+v", step)
	}
}

// The first finger's gesture ends when the second comes: a press is released without a click,
// and a pan stops.
func TestAPinchEndsWhatTheFirstFingerWasDoing(t *testing.T) {
	var touch Touch
	now := time.Duration(0)
	touch.Down(1, 100, 100, now, true) // a slider: pressed at once
	step := touch.Down(2, 300, 100, now, false)
	if !step.Cancel || step.Release || step.Pinch != PinchBegin {
		t.Fatalf("second finger on a pressed control: %+v, want the press cancelled, no click, and a pinch", step)
	}
	touch = Touch{}
	touch.Down(1, 100, 100, now, false)
	touch.Move(1, 100, 180, now)
	step = touch.Down(2, 300, 180, now, false)
	if !step.ScrollEnds || step.Pinch != PinchBegin {
		t.Fatalf("second finger during a pan: %+v, want the pan ended and a pinch", step)
	}
}

// Two fingers that start too close have no ratio to speak of: they are not a pinch, and a
// third finger is nothing.
func TestFingersTooCloseAreNoPinchAndAThirdIsNothing(t *testing.T) {
	var touch Touch
	now := time.Duration(0)
	touch.Down(1, 100, 100, now, false)
	if step := touch.Down(2, 105, 100, now, false); step.Pinch != PinchNone {
		t.Fatalf("fingers 5 px apart began a pinch: %+v", step)
	}
	touch = Touch{}
	touch.Down(1, 100, 100, now, false)
	touch.Down(2, 200, 100, now, false)
	if step := touch.Down(3, 150, 150, now, false); step != (TouchStep{}) {
		t.Fatalf("a third finger did something: %+v", step)
	}
}

func TestCancelEndsAPinch(t *testing.T) {
	var touch Touch
	now := time.Duration(0)
	touch.Down(1, 100, 100, now, false)
	touch.Down(2, 200, 100, now, false)
	touch.Move(2, 250, 100, now)
	if step := touch.Cancel(); step.Pinch != PinchEnd || touch.Active() {
		t.Fatalf("cancel of a pinch: %+v active=%v", step, touch.Active())
	}
}
