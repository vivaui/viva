package ui_test

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

// A focus request made while handling a click is carried into the repaint that
// follows, the one that shows the result of the click.
func TestPageFocusRequestInAClickHandlerTakesEffect(t *testing.T) {
	var search ui.Field
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Gap(8).Children(func() {
			field := ui.TextField(c, &search, "Search", "")
			if ui.Button(c, "Focus search").Clicked() {
				field.AutoFocus()
			}
		})
	})
	d.click("Focus search")
	if got := d.focused(); got != "Search" {
		t.Fatalf("focus after the click = %q, want the field the handler asked for", got)
	}
	d.typeText("find")
	if search.Text() != "find" {
		t.Fatalf("typing after the click reached %q, want the carried focus", search.Text())
	}
}

// A request the repaint makes itself wins over the one carried from the event
// pass: the frame the application sees is the one the repaint asked for.
func TestPageFocusRequestInTheRepaintWinsOverTheClick(t *testing.T) {
	var first, second ui.Field
	wantSecond := false
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Gap(8).Children(func() {
			// Declared before the click handler sets wantSecond, so the event
			// pass does not make this request: only the repaint does.
			b := ui.TextField(c, &second, "Second", "")
			a := ui.TextField(c, &first, "First", "")
			if wantSecond {
				b.AutoFocus()
			}
			if ui.Button(c, "Focus first").Clicked() {
				wantSecond = true
				a.AutoFocus()
			}
		})
	})
	d.click("Focus first")
	if got := d.focused(); got != "Second" {
		t.Fatalf("focus = %q, want the repaint's request to win", got)
	}
}

// The carried request is applied once: a later click moves the focus where the
// user put it, and the request made by the first click does not come back.
func TestPageCarriedFocusRequestIsAppliedOnce(t *testing.T) {
	var search ui.Field
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Gap(8).Children(func() {
			field := ui.TextField(c, &search, "Search", "")
			if ui.Button(c, "Focus search").Clicked() {
				field.AutoFocus()
			}
			ui.Button(c, "After")
		})
	})
	d.click("Focus search")
	d.click("After")
	if got := d.focused(); got != "After" {
		t.Fatalf("focus = %q, want the button the user clicked", got)
	}
}

// A focus request made while handling a shortcut is carried the same way.
func TestPageFocusRequestInAShortcutHandlerTakesEffect(t *testing.T) {
	var search ui.Field
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Gap(8).Children(func() {
			field := ui.TextField(c, &search, "Search", "")
			if ui.Shortcut(c, ui.Primary, "f") {
				field.AutoFocus()
			}
		})
	})
	if _, err := d.driver.ShortcutChord(ui.Primary, 'f'); err != nil {
		t.Fatal(err)
	}
	if got := d.focused(); got != "Search" {
		t.Fatalf("focus after the shortcut = %q, want the field the handler asked for", got)
	}
}
