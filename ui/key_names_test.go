//go:build !dev && !vivadev

package ui_test

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

// A Key given to an element without identity (a Text) names nothing, and must
// not pass on to the container declared after it: that container would take a
// name that moves with the list, and lose its State whenever the list's length
// changes.
func TestAKeyGivenToATextDoesNotRenameTheNextElement(t *testing.T) {
	rows, painted := 2, 0
	d := openPage(t, func(c ui.C) {
		for i := range rows {
			ui.Text(c.KeyInt(i), "row")
		}
		ui.Row(c).Children(func() {
			count := ui.State[int](c, "paints")
			*count++
			painted = *count
		})
	})
	d.repaint()
	before := painted
	rows = 3
	d.repaint()
	if painted != before+1 {
		t.Fatalf("the row after the list lost its state when the list grew: %d paints after %d", painted, before)
	}
}
