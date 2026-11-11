package ui_test

import (
	"testing"
	"time"

	"github.com/vivaui/viva/app"
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

// The page's theme sets how long the pointer rests before a tip appears; zero, the
// default, still shows it with the pointer.
func TestTooltipDelayComesFromTheTheme(t *testing.T) {
	theme := ui.LightTheme()
	theme.TooltipDelay = 500 * time.Millisecond
	d := openPageWith(t, ui.PageConfig{Theme: &theme, View: func(c ui.C) {
		ui.Column(c).Pad(40).Children(func() {
			ui.Button(c, "Delete").Tooltip("Delete the host")
		})
	}}, app.Config{})
	if err := d.driver.Move(center(d.hit("Delete").Bounds)); err != nil {
		t.Fatal(err)
	}
	if tooltipShown(d, "Delete the host") {
		t.Fatal("the tip appeared before the theme's delay")
	}
	if err := d.driver.Advance(600 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if !tooltipShown(d, "Delete the host") {
		t.Fatal("the tip did not appear after the theme's delay")
	}
}
