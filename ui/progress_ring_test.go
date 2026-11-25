package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func ringTriangles(d *pageDriver, color gfx.Color) int {
	n := 0
	for _, triangle := range d.driver.Snapshot().Primitives {
		if triangle.Color == color {
			n++
		}
	}
	return n
}

// A circular Progress is a ring that keeps its size in a Column instead of
// stretching like a bar, fills with its fraction, and takes its status color.
func TestProgressCircleKeepsItsSizeAndFillsWithItsFraction(t *testing.T) {
	var box ui.Box
	fraction := float32(0.25)
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(8).Children(func() { box = ui.Progress(c, fraction).Circle().Status(ui.StatusSuccess) })
	})
	r := box.Rect()
	if r.Width != 64 || r.Height != 64 {
		t.Fatalf("a ring given no size is %vx%v, want 64x64", r.Width, r.Height)
	}
	quarter := ringTriangles(d, pageTheme.Success)
	if quarter == 0 {
		t.Fatal("the ring's fill is not painted in the status color")
	}
	fraction = 0.75
	d.repaint()
	if three := ringTriangles(d, pageTheme.Success); three <= quarter {
		t.Fatalf("a ring three quarters full has %d fill triangles, not above a quarter full (%d)", three, quarter)
	}
	fraction = 0
	d.repaint()
	if ringTriangles(d, pageTheme.Success) != 0 {
		t.Fatal("an empty ring paints a fill")
	}
}

// The ring is a progress bar to assistive technology, with the fraction as its value.
func TestProgressCircleIsAProgressBar(t *testing.T) {
	d := openPage(t, func(c ui.C) { ui.Progress(c, 0.5).Circle().Label("Upload") })
	hit := d.hit("Upload")
	if hit.Role != gfx.RoleProgressBar || hit.Range.Now != 50 || hit.Range.Max != 100 {
		t.Fatalf("hit = role %v, range %+v; want a progress bar at 50 of 100", hit.Role, hit.Range)
	}
}

func TestProgressCircleRepaintsWithoutAllocating(t *testing.T) {
	d := openPage(t, func(c ui.C) { ui.Progress(c, 0.6).Circle().Size(80, 80) })
	d.repaint()
	d.repaint()
	if allocs := testing.AllocsPerRun(100, d.repaint); allocs != 0 {
		t.Fatalf("%v allocations a repaint", allocs)
	}
}
