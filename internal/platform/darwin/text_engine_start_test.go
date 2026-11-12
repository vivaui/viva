//go:build darwin && vivahead

package darwin

import (
	"fmt"
	"testing"
)

// An engine starts at the weight the system's own text has — the user's font smoothing,
// read once at creation — and SetFontThickening is the application's choice from then on:
// asking for the weight the engine already has moves nothing, asking for the other one
// moves the generation, as pictures made with the old weight are drawn again. The
// argument domain stands in for the user's setting, as OverrideDefaultInteger writes it.
func TestTheEngineStartsAtTheSystemsOwnWeight(t *testing.T) {
	defer func() {
		OverrideDefaultInteger("AppleFontSmoothing", 0, true)
		SharedTextEngine().SetFontThickening(true)
	}()
	for _, test := range []struct {
		smoothing int
		ask       bool
	}{
		{0, false}, // the user turned smoothing off: the engine starts unthickened
		{0, true},  // asking for the thick stroke moves the generation
		{1, true},  // smoothing on: the engine starts thickened
		{1, false}, // asking for the font's own outlines moves it
	} {
		t.Run(fmt.Sprintf("smoothing %d ask %v", test.smoothing, test.ask), func(t *testing.T) {
			OverrideDefaultInteger("AppleFontSmoothing", test.smoothing, false)
			engine := newTextEngine()
			generation := engine.FontGeneration()
			engine.SetFontThickening(test.ask)
			want := uint32(0)
			if test.smoothing != 0 != test.ask {
				want = 1
			}
			if moved := engine.FontGeneration() - generation; moved != want {
				t.Fatalf("the generation moved %d for asking %v under smoothing %d, want %d", moved, test.ask, test.smoothing, want)
			}
		})
	}
}
