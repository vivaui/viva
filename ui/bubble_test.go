package ui_test

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

// A page Bubble is a row-sized surface: it measures its wrapped text, stays
// inside 80% of its row and carries a hit named by the caller.
func TestPageBubbleSizesToItsText(t *testing.T) {
	d := openPage(t, func(c ui.C) {
		ui.Column(c).FillWidth().Children(func() {
			ui.Bubble(c, "A short message", ui.BubbleMuted).
				Hit(ui.ID(c), "Short").Self(ui.Start)
			ui.Bubble(c, "A much longer message that has to wrap onto more than one line in a narrow window", ui.BubbleTinted).
				Hit(ui.ID(c), "Long").Self(ui.End)
		})
	})
	short, long := d.hit("Short"), d.hit("Long")
	if short.Bounds.Width >= long.Bounds.Width {
		t.Fatalf("short bubble %g is not narrower than long %g", short.Bounds.Width, long.Bounds.Width)
	}
	if long.Bounds.Width > 480*0.8+1 {
		t.Fatalf("long bubble width %g exceeds 80%% of the page", long.Bounds.Width)
	}
	if long.Bounds.Height <= short.Bounds.Height {
		t.Fatalf("long bubble height %g is not taller than short %g", long.Bounds.Height, short.Bounds.Height)
	}
	if short.Bounds.X != 0 {
		t.Fatalf("start bubble x = %g, want the row start", short.Bounds.X)
	}
	if long.Bounds.X+long.Bounds.Width < 479 {
		t.Fatalf("end bubble ends at %g, want the row end", long.Bounds.X+long.Bounds.Width)
	}
}
