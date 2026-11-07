package interaction

import (
	"math"

	"github.com/vivaui/viva/ui"
)

// MaxSliderSteps bounds how many key presses one assistive-technology request
// to set a slider's value may make, so a fine step and a far target do not hold
// the event loop.
const MaxSliderSteps = 200

// SliderSteps returns the key and repeat count that move a slider from its
// current value to target, using only the keys a person would press: Home and
// End for the ends, and arrow steps between. The application's own slider logic
// remains the only source of truth.
func SliderSteps(value ui.ValueRange, target float64) (ui.Key, int) {
	if math.IsNaN(target) {
		return ui.KeyUnknown, 0
	}
	switch {
	case target <= value.Min:
		return ui.KeyHome, 1
	case target >= value.Max:
		return ui.KeyEnd, 1
	}
	step := value.Step
	if step <= 0 {
		step = 1 // what a slider moves by when it states no step
	}
	steps := int(math.Round((target - value.Now) / step))
	key := ui.KeyUp
	if steps < 0 {
		key, steps = ui.KeyDown, -steps
	}
	return key, min(steps, MaxSliderSteps)
}
