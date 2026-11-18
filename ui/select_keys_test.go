package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// selectPage opens a page with one Select and puts the keyboard on its trigger,
// as Tab does.
func selectPage(t *testing.T, size *string) *pageDriver {
	t.Helper()
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			ui.Select(c, size, []string{"S", "M", "L"}, nil).Placeholder("Size")
		})
	})
	d.key(gfx.KeyTab)
	if got := d.focused(); got != "Size" {
		t.Fatalf("Tab focused %q, want the select's trigger", got)
	}
	return d
}

// A Select is usable from the keyboard: Enter opens its list on the first option,
// the arrows move, Enter chooses, and the trigger takes the focus back.
func TestSelectOpensAndChoosesFromTheKeyboard(t *testing.T) {
	size := ""
	d := selectPage(t, &size)
	d.key(gfx.KeyEnter)
	if got := d.focused(); got != "S" {
		t.Fatalf("Enter opened the list on %q, want the first option", got)
	}
	d.key(gfx.KeyDown)
	if got := d.focused(); got != "M" {
		t.Fatalf("Down focused %q", got)
	}
	d.key(gfx.KeyEnter)
	if size != "M" {
		t.Fatalf("Enter chose %q", size)
	}
	if d.has("S") {
		t.Fatal("the list stayed open after a keyboard choice")
	}
	if got := d.focused(); got != "Size" {
		t.Fatalf("the focus went to %q, want the trigger", got)
	}
}

// Up on the closed trigger opens on the last option, as a select does.
func TestSelectArrowUpFromTheTriggerStartsAtTheLastOption(t *testing.T) {
	size := ""
	d := selectPage(t, &size)
	d.key(gfx.KeyUp)
	if got := d.focused(); got != "L" {
		t.Fatalf("Up focused %q, want the last option", got)
	}
	d.key(gfx.KeyEnter)
	if size != "L" {
		t.Fatalf("Enter chose %q", size)
	}
}

// Escape closes the list and leaves the value alone.
func TestSelectEscapeClosesTheListFromTheKeyboard(t *testing.T) {
	size := "S"
	d := selectPage(t, &size)
	d.key(gfx.KeyEnter)
	d.key(gfx.KeyEscape)
	if size != "S" {
		t.Fatalf("Escape changed the value to %q", size)
	}
	if d.has("M") {
		t.Fatal("Escape left the list open")
	}
	if got := d.focused(); got != "Size" {
		t.Fatalf("Escape left the focus on %q, want the trigger", got)
	}
}
