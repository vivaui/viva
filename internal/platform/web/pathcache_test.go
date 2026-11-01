//go:build js && wasm

package web

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

func triangleRun(dx, dy float32) []ui.Triangle {
	shape := [][3]ui.Point{
		{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 0, Y: 10}},
		{{X: 10, Y: 0}, {X: 10, Y: 10}, {X: 0, Y: 10}},
	}
	run := make([]ui.Triangle, len(shape))
	for index, points := range shape {
		for corner := range points {
			points[corner].X += dx
			points[corner].Y += dy
		}
		run[index] = ui.Triangle{Points: points}
	}
	return run
}

// A shape is one cached path wherever it is drawn: scrolling a list of icons must
// not rebuild them.
func TestRunKeyIsTheSameWhereverTheShapeIs(t *testing.T) {
	at := triangleRun(100, 200)
	for _, offset := range [][2]float32{{0, 0}, {0, 26}, {13.37, 421.0625}, {-8, 1000.1}} {
		moved := triangleRun(100+offset[0], 200+offset[1])
		if runKey(moved, moved[0].Points[0]) != runKey(at, at[0].Points[0]) {
			t.Errorf("shape moved by %v is a different path", offset)
		}
	}
}

func TestRunKeyDiffersWhenTheShapeDoes(t *testing.T) {
	a := triangleRun(0, 0)
	b := triangleRun(0, 0)
	b[1].Points[1].X += 1
	if runKey(a, a[0].Points[0]) == runKey(b, b[0].Points[0]) {
		t.Error("different geometry shares a key")
	}
	if runKey(a[:1], a[0].Points[0]) == runKey(a, a[0].Points[0]) {
		t.Error("a shorter run shares a key with the longer one")
	}
}

// Triangles join a run only when one fill can draw them all as drawn separately.
func TestSameRunStopsAtAnyChangeOfHowItIsPainted(t *testing.T) {
	first := ui.Triangle{Color: ui.Color{Red: 1, Alpha: 1}, Layer: 1}
	if !sameRun(first, first) {
		t.Fatal("identical triangles must share a run")
	}
	for name, change := range map[string]func(*ui.Triangle){
		"color":  func(next *ui.Triangle) { next.Color.Green = 1 },
		"layer":  func(next *ui.Triangle) { next.Layer = 2 },
		"shaded": func(next *ui.Triangle) { next.Shaded = true },
		"clip":   func(next *ui.Triangle) { next.HasClip, next.ClipOf = true, ui.Rect{Width: 5, Height: 5} },
	} {
		next := first
		change(&next)
		if sameRun(first, next) {
			t.Errorf("a change of %s must end the run", name)
		}
	}
}
