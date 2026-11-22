package base

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func finger(dy float32) gfx.ScrollEvent {
	return gfx.ScrollEvent{Phase: gfx.ScrollPhaseFinger, DeltaY: dy, Precise: true}
}

func tallArea() *ScrollState { return &ScrollState{Content: 2000, Viewport: 500} }

// A finger that drags content past its start carries it there at a fraction of its
// motion, the further the less, and never a whole viewport.
func TestAFingerGivesAtTheEndOfTheContent(t *testing.T) {
	s := tallArea()
	var last float32
	for i := 0; i < 40; i++ {
		s.Wheel(finger(-20)) // the finger moves down: the content goes down, past its start
		shown := -s.overscroll()
		if s.Offset != 0 {
			t.Fatalf("the offset left the start: %v", s.Offset)
		}
		if shown <= last && shown < s.Viewport*0.99 {
			t.Fatalf("step %d: content shown past the start %v did not grow from %v", i, shown, last)
		}
		if shown >= s.Viewport {
			t.Fatalf("content went a whole viewport past the start: %v", shown)
		}
		if i == 0 && shown >= 20 {
			t.Fatalf("the first 20 px of the finger moved the content %v, want it to give", shown)
		}
		last = shown
	}
}

// A wheel and a touchpad stop at the end: they never carry content past it.
func TestAWheelStopsAtTheEnd(t *testing.T) {
	s := tallArea()
	s.Wheel(gfx.ScrollEvent{DeltaY: -80, Precise: true})
	if s.overscroll() != 0 || s.pull != 0 {
		t.Fatalf("a touchpad pulled the content past its start: %v", s.pull)
	}
}

// What the finger holds stays held until it lets go; then it is home.
func TestContentHeldPastTheEndSpringsBackWhenReleased(t *testing.T) {
	s := tallArea()
	s.Wheel(finger(-60))
	s.relax(nil)
	if s.pull == 0 {
		t.Fatal("content let go while the finger held it")
	}
	s.Wheel(gfx.ScrollEvent{Phase: gfx.ScrollPhaseEnd})
	s.relax(nil)
	if s.pull != 0 || s.overscroll() != 0 {
		t.Fatalf("content is still %v past the start after the finger lifted", s.overscroll())
	}
}

// The finger moving back first takes the content back to its end, then scrolls it.
func TestTheFingerMovingBackReturnsThePullBeforeScrolling(t *testing.T) {
	s := tallArea()
	s.Wheel(finger(-100))
	s.Wheel(finger(100 + 30))
	if s.pull != 0 {
		t.Fatalf("pull = %v after the finger came back past where it began", s.pull)
	}
	if s.Offset != 30 {
		t.Fatalf("offset = %v, want the 30 px the finger went on", s.Offset)
	}
}

// A fling that reaches an end gives a little and springs back at once, however fast it was.
func TestAFlingReachingTheEndGivesALittle(t *testing.T) {
	s := tallArea()
	s.Offset = s.Max()
	s.Wheel(gfx.ScrollEvent{Phase: gfx.ScrollPhaseMomentum, DeltaY: 4000, Precise: true})
	if s.pull <= 0 || s.pull > s.Viewport/4 {
		t.Fatalf("pull = %v, want some, and no more than a quarter of the viewport", s.pull)
	}
	before := s.pull
	s.Wheel(gfx.ScrollEvent{Phase: gfx.ScrollPhaseMomentum, DeltaY: 4000, Precise: true})
	if s.pull != before {
		t.Fatalf("the rest of the fling carried the content further: %v", s.pull)
	}
}

// Content that fits has no end to give at.
func TestShortContentDoesNotGive(t *testing.T) {
	s := &ScrollState{Content: 300, Viewport: 500}
	s.Wheel(finger(-80))
	if s.pull != 0 {
		t.Fatalf("content shorter than its area gave: %v", s.pull)
	}
}
