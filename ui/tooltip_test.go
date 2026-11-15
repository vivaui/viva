package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// tooltipShown reports whether the frame shows text in a tooltip: a label with
// that text and no control of that name.
func tooltipShown(d *pageDriver, text string) bool {
	for _, label := range d.driver.Snapshot().Labels {
		if label.Value == text {
			return true
		}
	}
	return false
}

// A Tooltip appears while the pointer rests on its control, is the control's
// description, hides on a press until the pointer leaves, and a Memo's copy keeps it.
func TestTooltip(t *testing.T) {
	for _, memo := range []bool{false, true} {
		d := openPage(t, func(c ui.C) {
			ui.Column(c).Pad(40).Gap(40).Children(func() {
				row := func() { ui.Button(c, "Delete").Tooltip("Delete the host") }
				if memo {
					ui.Memo(c, 1, row)
				} else {
					row()
				}
				ui.Button(c, "Other")
			})
		})
		if hit := d.hit("Delete"); hit.Hint != "Delete the host" {
			t.Fatalf("memo=%v: description %q", memo, hit.Hint)
		}
		if tooltipShown(d, "Delete the host") {
			t.Fatalf("memo=%v: the tip shows with the pointer elsewhere", memo)
		}
		if err := d.driver.Move(center(d.hit("Delete").Bounds)); err != nil {
			t.Fatal(err)
		}
		if !tooltipShown(d, "Delete the host") {
			t.Fatalf("memo=%v: the tip did not appear over its control", memo)
		}
		d.click("Delete")
		if tooltipShown(d, "Delete the host") {
			t.Fatalf("memo=%v: the tip stayed through a press", memo)
		}
		if err := d.driver.Move(center(d.hit("Other").Bounds)); err != nil {
			t.Fatal(err)
		}
		if tooltipShown(d, "Delete the host") {
			t.Fatalf("memo=%v: the tip follows the pointer away", memo)
		}
	}
}

// tooltipLabel is the label of the shown tooltip saying text.
func tooltipLabel(t *testing.T, d *pageDriver, text string) gfx.Rect {
	t.Helper()
	for _, label := range d.driver.Snapshot().Labels {
		if label.Value == text {
			return label.Bounds
		}
	}
	t.Fatalf("no tooltip says %q", text)
	return gfx.Rect{}
}

// TooltipOn opens the tip beside the side it names, below the control by
// default, and a Memo's copy keeps the side.
func TestTooltipOn(t *testing.T) {
	for _, memo := range []bool{false, true} {
		d := openPage(t, func(c ui.C) {
			ui.Column(c).Pad(40).Gap(40).Children(func() {
				row := func() {
					ui.Button(c, "Info").Tooltip("Opens to the right").TooltipOn(ui.TooltipRight)
					ui.Button(c, "Plain").Tooltip("Opens below")
				}
				if memo {
					ui.Memo(c, 1, row)
				} else {
					row()
				}
			})
		})
		info := d.hit("Info").Bounds
		if err := d.driver.Move(center(info)); err != nil {
			t.Fatal(err)
		}
		if tip := tooltipLabel(t, d, "Opens to the right"); tip.X < info.X+info.Width || tip.Y >= info.Y+info.Height {
			t.Fatalf("memo=%v: the tip %v is not to the right of its control %v", memo, tip, info)
		}
		plain := d.hit("Plain").Bounds
		if err := d.driver.Move(center(plain)); err != nil {
			t.Fatal(err)
		}
		if tip := tooltipLabel(t, d, "Opens below"); tip.Y < plain.Y+plain.Height {
			t.Fatalf("memo=%v: the tip %v is not below its control %v", memo, tip, plain)
		}
	}
}
