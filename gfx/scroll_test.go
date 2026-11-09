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
