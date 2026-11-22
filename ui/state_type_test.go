package ui_test

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

// Two States of one name and different types in one container are two states:
// each keeps its value, instead of one resetting the other every paint, and
// asking for them again allocates nothing.
func TestStatesOfOneNameAndTwoTypesAreTwoStates(t *testing.T) {
	var count *int
	var open *bool
	d := openPage(t, func(c ui.C) {
		count = ui.State[int](c, "x")
		open = ui.State[bool](c, "x")
		*count++
		*open = true
	})
	d.repaint()
	d.repaint()
	if *count != 3 || !*open {
		t.Fatalf("after three paints: count %d, open %v", *count, *open)
	}
	if allocations := testing.AllocsPerRun(20, d.repaint); allocations != 0 {
		t.Fatalf("a repaint asking for the States allocated %v times", allocations)
	}
}
