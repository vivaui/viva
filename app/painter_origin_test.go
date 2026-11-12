package app

import "testing"

// A label's frame is on whole logical pixels, and the text keeps the rest of its position:
// the two add up to where layout put it, so a backend that places glyphs at device pixels
// loses nothing to the rounding.
func TestLabelOriginIsWhatRoundingTookOff(t *testing.T) {
	for _, value := range []float32{-99.9, -20.75, -10.3, -3.49, -0.5, -0.25, 0, 0.25, 0.5, 3.49, 10.3, 20.75, 99.9} {
		got := float32(roundPosition(value)) + roundingRemainder(value)
		if diff := got - value; diff > 1e-4 || diff < -1e-4 {
			t.Errorf("%v: frame %d + origin %v = %v", value, roundPosition(value), roundingRemainder(value), got)
		}
		if r := roundingRemainder(value); r < -0.5 || r > 0.5 {
			t.Errorf("%v: origin %v is further than half a pixel from the frame", value, r)
		}
	}
}
