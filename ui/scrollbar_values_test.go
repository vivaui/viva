package ui_test

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

// ScrollbarValues is the semantic value of a bar, as components.ScrollArea
// takes it: a percentage for a detail view's always-visible bar.
func TestScrollbarValuesAreAccessible(t *testing.T) {
	d := openPage(t, func(c ui.C) {
		ui.Scroll(c).Size(200, 100).AlwaysScrollbars().ScrollbarValues("42%", "").Children(func() {
			ui.Slot(c).Height(300)
		})
	})
	if got := d.hit("Vertical scrollbar").Value; got != "42%" {
		t.Fatalf("vertical bar value = %q, want 42%%", got)
	}
}
