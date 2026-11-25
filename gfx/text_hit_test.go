package gfx

import (
	"math"
	"testing"
)

func TestTextHitAtKeepsTwoPositionsForOneOffset(t *testing.T) {
	const prefix, rtl, suffix = "ab ", "\u05d0\u05b7\u05d1", " cd"
	const value = prefix + rtl + suffix
	const size = 20
	for _, engine := range []TextEngine{nil, ReferenceTextEngine()} {
		var canvas Canvas
		canvas.SetTextEngine(engine)
		left := canvas.TextWidth(prefix, size)
		right := canvas.TextWidth(prefix+rtl, size)
		first := canvas.TextHitAt(value, size, 0, left-0.5)
		second := canvas.TextHitAt(value, size, 0, right-0.5)
		if first.Offset != len(prefix) || second.Offset != first.Offset {
			t.Fatalf("engine %T: mixed hits %+v and %+v; both should name byte %d", engine, first, second, len(prefix))
		}
		if math.Abs(float64(first.X-left)) > 0.01 || math.Abs(float64(second.X-right)) > 0.01 {
			t.Errorf("engine %T: mixed hits %+v and %+v; want positions %g and %g", engine, first, second, left, right)
		}
		if got := canvas.TextOffsetAt(value, size, 0, right-0.5); got == second.Offset {
			t.Errorf("engine %T: primary-only offset unexpectedly picked the alternate edge", engine)
		}
	}
}
