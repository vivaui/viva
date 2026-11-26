package gfx

import (
	"math"
	"testing"
)

// An arc starts on the circle at its start angle, ends on it at start plus sweep, and
// every point the curves pass through stays on the circle to within what a cubic
// approximation allows.
func TestPathBuilderArcFollowsTheCircle(t *testing.T) {
	for _, test := range []struct{ start, sweep float32 }{{-90, 90}, {-90, 180}, {0, 270}, {30, 360}, {10, 45}} {
		var builder PathBuilder
		builder.Arc(50, 40, 20, test.start, test.sweep)
		commands := builder.commands
		if commands[0].Kind != PathCommandMove {
			t.Fatalf("%+v: an arc does not begin with a move", test)
		}
		sx, sy := float64(commands[0].Point.X), float64(commands[0].Point.Y)
		wantX := 50 + 20*math.Cos(float64(test.start)*math.Pi/180)
		wantY := 40 + 20*math.Sin(float64(test.start)*math.Pi/180)
		if math.Abs(sx-wantX) > 1e-3 || math.Abs(sy-wantY) > 1e-3 {
			t.Fatalf("%+v: starts at (%v, %v), want (%v, %v)", test, sx, sy, wantX, wantY)
		}
		last := commands[len(commands)-1]
		endAngle := float64(test.start+min(test.sweep, 360)) * math.Pi / 180
		if math.Abs(float64(last.Point.X)-(50+20*math.Cos(endAngle))) > 1e-3 || math.Abs(float64(last.Point.Y)-(40+20*math.Sin(endAngle))) > 1e-3 {
			t.Fatalf("%+v: ends at (%v, %v), off the circle at the end angle", test, last.Point.X, last.Point.Y)
		}
		for _, c := range commands[1:] {
			if d := math.Hypot(float64(c.Point.X)-50, float64(c.Point.Y)-40); math.Abs(d-20) > 1e-3 {
				t.Fatalf("%+v: an end point is %v from the center, want 20", test, d)
			}
		}
	}
}

// A sweep of zero or less, or a radius of zero or less, adds nothing; and the
// builder's buffer is reused.
func TestPathBuilderArcOfNothingAddsNothing(t *testing.T) {
	var builder PathBuilder
	builder.Arc(0, 0, 10, 0, 0)
	builder.Arc(0, 0, 0, 0, 90)
	builder.Arc(0, 0, 10, 0, -5)
	if builder.Len() != 0 {
		t.Fatalf("%d commands for arcs of nothing, want none", builder.Len())
	}
	builder.Arc(0, 0, 10, 0, 360)
	builder.Reset()
	if allocs := testing.AllocsPerRun(100, func() { builder.Reset(); builder.Arc(0, 0, 10, -90, 270) }); allocs != 0 {
		t.Fatalf("a warm arc allocated %v times, want 0", allocs)
	}
}
