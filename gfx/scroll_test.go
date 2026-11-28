package gfx

import "testing"

func TestScrollStepRaisesClassicWheelToOneUnit(t *testing.T) {
	cases := []struct {
		name  string
		event ScrollEvent
		want  float32
	}{
		{"slow notch down", ScrollEvent{DeltaY: 1}, 18},
		{"slow notch up", ScrollEvent{DeltaY: -1}, -18},
		{"fast spin keeps its distance", ScrollEvent{DeltaY: 90}, 90},
		{"no movement stays none", ScrollEvent{}, 0},
		{"touchpad keeps pixels", ScrollEvent{DeltaY: 1, Precise: true}, 1},
	}
	for _, c := range cases {
		if got := c.event.StepY(18); got != c.want {
			t.Errorf("%s: StepY = %v, want %v", c.name, got, c.want)
		}
	}
	if got := (ScrollEvent{DeltaX: -2}).StepX(8); got != -8 {
		t.Errorf("StepX = %v, want -8", got)
	}
}

func TestPageStepMovesOnePageOfTheRegion(t *testing.T) {
	// A page notch reports the window as pixels and the region's own page is
	// what moves: whole rows of the region, not the window's, and fewer than it
	// shows, so the rows at the edge it leaves are still in view.
	cases := []struct {
		name  string
		event ScrollEvent
		unit  float32
		view  float32
	}{
		{"a page down in a short list", ScrollEvent{DeltaY: 900, Pages: 1}, 30, 300},
		{"a page up in a short list", ScrollEvent{DeltaY: -900, Pages: -1}, 30, 300},
		{"rows that do not fit whole", ScrollEvent{DeltaY: 900, Pages: 1}, 30, 305},
		{"a terminal screen", ScrollEvent{DeltaY: -900, Pages: -1}, 17, 40 * 17},
		{"a two-row region", ScrollEvent{DeltaY: 900, Pages: 1}, 30, 60},
		{"a pane in pixels", ScrollEvent{DeltaY: 900, Pages: 1}, 1, 420},
	}
	for _, c := range cases {
		got := c.event.StepYIn(c.unit, c.view)
		if (got < 0) != (c.event.DeltaY < 0) {
			t.Errorf("%s: StepYIn = %v, against the direction of %v", c.name, got, c.event.DeltaY)
		}
		step := got
		if step < 0 {
			step = -step
		}
		if rows := step / c.unit; rows != float32(int(rows)) {
			t.Errorf("%s: StepYIn = %v, not whole rows of %v", c.name, got, c.unit)
		}
		if step >= c.view || step < c.view/2 {
			t.Errorf("%s: StepYIn = %v, want most of the %v viewport with an overlap", c.name, got, c.view)
		}
		if step != float32(int(ScrollPage(c.view)/c.unit))*c.unit {
			t.Errorf("%s: StepYIn = %v, not the rows of ScrollPage(%v)", c.name, got, c.view)
		}
	}
	one := (ScrollEvent{DeltaY: 900, Pages: 1}).StepYIn(24, 200)
	if two := (ScrollEvent{DeltaY: 1800, Pages: 2}).StepYIn(24, 200); two < 2*one || two > 2*one+24 {
		t.Errorf("two pages = %v, want twice a page of %v", two, one)
	}
	if half := (ScrollEvent{DeltaY: 450, Pages: 0.5}).StepYIn(24, 200); half <= 0 || half > one {
		t.Errorf("half a page = %v, want a part of a page of %v", half, one)
	}
	if got := (ScrollEvent{DeltaY: 900, Pages: 1}).StepYIn(30, 10); got != 30 {
		t.Errorf("a region shorter than one row moved %v, want a row of 30", got)
	}
	if got := (ScrollEvent{DeltaY: 900, Pages: 1}).StepYIn(30, 0); got != 30 {
		t.Errorf("a zero viewport moved %v, want a row of 30", got)
	}
	if got := (ScrollEvent{DeltaY: 48}).StepYIn(30, 300); got != 48 {
		t.Errorf("a line notch moved %v, want its own 48", got)
	}
	if got := (ScrollEvent{DeltaX: 900, Pages: 1}).StepXIn(10, 80); got <= 0 || got >= 80 {
		t.Errorf("StepXIn = %v, want most of the 80 width", got)
	}
	// A page setting does not turn a horizontal delta into a vertical page.
	if got := (ScrollEvent{DeltaY: 900, Pages: 1}).StepXIn(10, 50); got != 0 {
		t.Errorf("StepXIn on a vertical page = %v, want 0", got)
	}
}
