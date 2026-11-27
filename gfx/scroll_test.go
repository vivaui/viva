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
	// what moves: whole rows that fit in the region, not the window's.
	cases := []struct {
		name  string
		event ScrollEvent
		unit  float32
		view  float32
		want  float32
	}{
		{"a page down in a short list", ScrollEvent{DeltaY: 900, Pages: 1}, 30, 300, 300},
		{"a page up in a short list", ScrollEvent{DeltaY: -900, Pages: -1}, 30, 300, -300},
		{"rows that do not fit whole are left out", ScrollEvent{DeltaY: 900, Pages: 1}, 30, 305, 300},
		{"two pages", ScrollEvent{DeltaY: 1800, Pages: 2}, 24, 200, 384},
		{"half a page of a trackpad setting", ScrollEvent{DeltaY: 450, Pages: 0.5}, 24, 200, 96},
		{"a region shorter than one row still moves a row", ScrollEvent{DeltaY: 900, Pages: 1}, 30, 10, 30},
		{"a line notch is not a page", ScrollEvent{DeltaY: 48}, 30, 300, 48},
		{"a zero viewport keeps a row", ScrollEvent{DeltaY: 900, Pages: 1}, 30, 0, 30},
	}
	for _, c := range cases {
		if got := c.event.StepYIn(c.unit, c.view); got != c.want {
			t.Errorf("%s: StepYIn(%v, %v) = %v, want %v", c.name, c.unit, c.view, got, c.want)
		}
	}
	if got := (ScrollEvent{DeltaX: 900, Pages: 1}).StepXIn(10, 50); got != 50 {
		t.Errorf("StepXIn = %v, want 50", got)
	}
	// A page setting does not turn a horizontal delta into a vertical page.
	if got := (ScrollEvent{DeltaY: 900, Pages: 1}).StepXIn(10, 50); got != 0 {
		t.Errorf("StepXIn on a vertical page = %v, want 0", got)
	}
}
