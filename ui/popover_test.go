package ui_test

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// A Popover's Title and Description head the body, the description wrapped to the
// panel's width, and a body with a Height of its own fixes the panel's height.
func TestPopoverHeaderAndFixedBody(t *testing.T) {
	open := true
	long := "A description long enough that it has to wrap onto a second line inside a narrow panel."
	var body ui.Box
	d := openPage(t, func(c ui.C) {
		more := ui.Button(c, "More")
		ui.PopoverWith(c, more, ui.PopoverSpec{Width: 200, Title: "Transfers", Description: long}, &open, func() {
			body = ui.Column(c).FillWidth().Height(120).Children(func() { ui.Text(c, "Body") })
		})
	})
	var title, first, last gfx.Text
	for _, tx := range d.driver.Snapshot().Texts {
		switch {
		case tx.Value == "Transfers":
			title = tx
		case tx.Value == "Body":
			last = tx
		case strings.Contains(long, strings.TrimSuffix(tx.Value, "…")) && tx.Value != "Body":
			if first.Value == "" {
				first = tx
			}
		}
	}
	if title.Value == "" || first.Value == "" || last.Value == "" {
		t.Fatalf("title %q, description %q, body %q: not all painted", title.Value, first.Value, last.Value)
	}
	if !(title.Bounds.Y < first.Bounds.Y && first.Bounds.Y < last.Bounds.Y) {
		t.Fatalf("order: title %v, description %v, body %v", title.Bounds, first.Bounds, last.Bounds)
	}
	if first.Value == long {
		t.Fatal("the description did not wrap to the panel's width")
	}
	if got := body.Rect().Height; got != 120 {
		t.Fatalf("a Height on the body made the panel's body %v tall, not 120", got)
	}
}

// The body starts a surface padding under the header, as components.Popover leaves
// it: the space between description and body follows the theme's PadSurface, not
// the panel's item gap.
func TestPopoverHeaderToBodyFollowsSurfacePadding(t *testing.T) {
	gapFor := func(Pad float32) float32 {
		theme := pageTheme
		theme.Metrics.PadSurface = Pad
		open := true
		var body ui.Box
		d := openPageWith(t, ui.PageConfig{Theme: &theme, View: func(c ui.C) {
			more := ui.Button(c, "More")
			ui.PopoverWith(c, more, ui.PopoverSpec{Width: 300, Title: "Transfers", Description: "Active transfers"}, &open, func() {
				body = ui.Column(c).FillWidth().Height(40).Children(func() { ui.Text(c, "Body") })
			})
		}}, app.Config{})
		var description gfx.Text
		for _, tx := range d.driver.Snapshot().Texts {
			if tx.Value == "Active transfers" {
				description = tx
			}
		}
		if description.Value == "" {
			t.Fatal("the description is not painted")
		}
		return body.Rect().Y - (description.Bounds.Y + description.Bounds.Height)
	}
	if small, large := gapFor(16), gapFor(24); large-small < 7.5 || large-small > 8.5 {
		t.Fatalf("raising PadSurface by 8 moved the body %v away from the description", large-small)
	}
}

// AlignOffset moves a Popover along its anchor from where Align put it, as the
// hover-card option of the same name does.
func TestPopoverAlignOffset(t *testing.T) {
	x := func(shift float32) float32 {
		open := true
		var body ui.Box
		d := openPage(t, func(c ui.C) {
			ui.Column(c).Pad(20).Children(func() {
				more := ui.Button(c, "More").Width(200)
				ui.PopoverWith(c, more, ui.PopoverSpec{Side: components.PopoverBottom, Align: ui.Start, AlignOffset: shift, Width: 100}, &open, func() {
					body = ui.Column(c).FillWidth().Height(40)
				})
			})
		})
		_ = d
		return body.Rect().X
	}
	base, shifted := x(0), x(40)
	if shifted != base+40 {
		t.Fatalf("an AlignOffset of 40 moved the panel from %v to %v, want %v", base, shifted, base+40)
	}
}

// A hover preview can leave out the dismiss target: no press-outside hit and no
// Escape target, as a hand-painted hover card had. Radius is how it keeps the
// popup recipe's corners rather than a popover's.
func TestPopoverCanBeAHoverPreview(t *testing.T) {
	open := true
	d := openPage(t, func(c ui.C) {
		anchor := ui.Button(c, "Anchor")
		ui.PopoverWith(c, anchor, ui.PopoverSpec{Title: "Details", Width: 200, Radius: 10, NoDismiss: true}, &open, func() {
			ui.Text(c, "Body")
		})
	})
	for _, hit := range d.driver.Snapshot().Hits {
		if hit.Label == "Dismiss popover" {
			t.Fatal("a hover preview declared a dismiss target")
		}
	}
	if err := d.driver.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if !open {
		t.Fatal("Escape closed a hover preview")
	}
}

// Margin is the room the panel keeps from the window's edges as it shifts to fit:
// a page that does not own the title strip asks for more than the default 8.
func TestPopoverMarginKeepsClearOfTheWindowEdge(t *testing.T) {
	x := func(margin float32) float32 {
		open := true
		var body ui.Box
		openPage(t, func(c ui.C) {
			ui.Row(c).FillWidth().Justify(ui.End).Children(func() {
				anchor := ui.Button(c, "Anchor").Width(100)
				ui.PopoverWith(c, anchor, ui.PopoverSpec{Side: components.PopoverBottom, Align: ui.Start, Width: 200, Margin: margin}, &open, func() {
					body = ui.Column(c).FillWidth().Height(40)
				})
			})
		})
		return body.Rect().X
	}
	if tight, roomy := x(8), x(40); roomy != tight-32 {
		t.Fatalf("a margin of 40 put the panel at %v against %v at 8, want 32 to the left", roomy, tight)
	}
}

// Padding insets the panel's content from its surface: a menu-like panel gives a
// smaller one so its rows line up with the menus around it.
func TestPopoverPaddingInsetsTheBody(t *testing.T) {
	x := func(padding float32) float32 {
		open := true
		var body ui.Box
		openPage(t, func(c ui.C) {
			anchor := ui.Button(c, "Anchor")
			ui.PopoverWith(c, anchor, ui.PopoverSpec{Width: 200, Padding: padding}, &open, func() {
				body = ui.Column(c).FillWidth().Height(40)
			})
		})
		return body.Rect().X
	}
	if tight, roomy := x(4), x(20); roomy != tight+16 {
		t.Fatalf("a padding of 20 put the body at %v against %v at 4, want 16 further in", roomy, tight)
	}
}

// Instant only removes the entrance motion: the panel and its body keep the same
// geometry as an animated one.
func TestPopoverInstantKeepsTheLayout(t *testing.T) {
	rect := func(instant bool) gfx.Rect {
		open := true
		var body ui.Box
		openPage(t, func(c ui.C) {
			anchor := ui.Button(c, "Anchor")
			ui.PopoverWith(c, anchor, ui.PopoverSpec{Width: 200, Instant: instant}, &open, func() {
				body = ui.Column(c).FillWidth().Height(40)
			})
		})
		return body.Rect()
	}
	if animated, instant := rect(false), rect(true); instant != animated {
		t.Fatalf("instant body %+v, animated %+v", instant, animated)
	}
}
