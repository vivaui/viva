package ui_test

import (
	"strings"
	"testing"

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
		case strings.HasPrefix(long, strings.TrimSuffix(tx.Value, "…")) && first.Value == "":
			first = tx
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
