package interaction

import (
	"math"

	"github.com/vivaui/viva/gfx"
)

// MaxSliderSteps bounds how many key presses one assistive-technology request
// to set a slider's value may make, so a fine step and a far target do not hold
// the event loop.
const MaxSliderSteps = 200

// SliderSteps returns the key and repeat count that move a slider from its
// current value to target, using only the keys a person would press: Home and
// End for the ends, and arrow steps between. The application's own slider logic
// remains the only source of truth.
func SliderSteps(value gfx.ValueRange, target float64) (gfx.Key, int) {
	if math.IsNaN(target) {
		return gfx.KeyUnknown, 0
	}
	switch {
	case target <= value.Min:
		return gfx.KeyHome, 1
	case target >= value.Max:
		return gfx.KeyEnd, 1
	}
	step := value.Step
	if step <= 0 {
		step = 1 // what a slider moves by when it states no step
	}
	steps := int(math.Round((target - value.Now) / step))
	key := gfx.KeyUp
	if steps < 0 {
		key, steps = gfx.KeyDown, -steps
	}
	return key, min(steps, MaxSliderSteps)
}
