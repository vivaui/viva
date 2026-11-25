package ui_test

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

// A busy button delivers no press, so work cannot be started twice, and is not
// dimmed as a Disabled one is.
func TestLoadingButtonDeliversNoPress(t *testing.T) {
	loading, clicks := false, 0
	d := openPage(t, func(c ui.C) {
		if ui.Button(c, "Connect").Loading(loading).Clicked() {
			clicks++
		}
	})
	d.click("Connect")
	if clicks != 1 {
		t.Fatalf("clicks = %d after a press on an idle button, want 1", clicks)
	}
	loading = true
	d.repaint()
	d.click("Connect")
	if clicks != 1 {
		t.Fatalf("clicks = %d after a press on a busy button, want it still 1", clicks)
	}
	if d.hit("Connect").Disabled {
		t.Fatal("a busy button is announced as disabled")
	}
}

// The spinner stands in the icon's place, or before the label when there is no
// icon, so the busy button is wider than the idle one by an icon.
func TestLoadingButtonPaintsASpinnerAndHoldsRoomForIt(t *testing.T) {
	width := func(loading bool) (float32, int) {
		var box ui.Box
		d := openPage(t, func(c ui.C) { box = ui.Button(c, "Connect").Loading(loading) })
		return box.Rect().Width, len(d.driver.Snapshot().Primitives)
	}
	idleWidth, idlePrimitives := width(false)
	busyWidth, busyPrimitives := width(true)
	if busyWidth <= idleWidth {
		t.Fatalf("busy width %v is not above idle width %v", busyWidth, idleWidth)
	}
	if busyPrimitives <= idlePrimitives {
		t.Fatalf("a busy button paints %d primitives, an idle one %d; want the spinner's spokes in addition", busyPrimitives, idlePrimitives)
	}
}

func TestLoadingButtonRepaintsWithoutAllocating(t *testing.T) {
	d := openPage(t, func(c ui.C) { ui.Button(c, "Connect").Loading(true) })
	d.repaint()
	d.repaint()
	if allocs := testing.AllocsPerRun(100, d.repaint); allocs != 0 {
		t.Fatalf("%v allocations a repaint", allocs)
	}
}
