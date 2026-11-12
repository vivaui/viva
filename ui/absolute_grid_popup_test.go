package ui

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// topHit is the region a pointer at (x, y) would reach: the last declared that holds it.
func topHit(canvas *gfx.Canvas, x, y float32) gfx.ID {
	var id gfx.ID
	for _, hit := range canvas.Hits() {
		b := hit.Bounds
		if x >= b.X && x < b.X+b.Width && y >= b.Y && y < b.Y+b.Height {
			id = hit.ID
		}
	}
	return id
}

func TestGridChildSpansItsColumnsAndGrowsRowsOnlyWhenNeeded(t *testing.T) {
	var view View
	var grid, wide, a, b, tall, next Box
	f := view.Begin(testCanvas(), gfx.LightTheme(), gfx.Rect{Width: 320, Height: 400})
	f.Column().Children(func() {
		grid = f.Grid(100).Gap(10).Children(func() { // 3 columns of 100
			wide = f.Slot().Height(30).ColSpan(2)
			a = f.Slot().Height(20)
			b = f.Slot().Height(20)
			tall = f.Slot().Height(90).RowSpan(2) // rows 2 and 3 hold 20 and 0 and a 10 gap: they lack 60
			next = f.Slot().Height(10)
		})
	})
	f.End()
	near(t, "wide covers two columns and the gap", wide.Rect(), gfx.Rect{X: 0, Y: 0, Width: 210, Height: 30})
	near(t, "a after the span", a.Rect(), gfx.Rect{X: 220, Y: 0, Width: 100, Height: 30})
	near(t, "tall covers its rows", tall.Rect(), gfx.Rect{X: 110, Y: 40, Width: 100, Height: 90})
	if b.Rect().Height != 50 || next.Rect().Height != 50 {
		t.Errorf("the row the span crosses is %v and %v tall, want it grown to 50 (20 and 0 plus 30 each)", b.Rect().Height, next.Rect().Height)
	}
	if grid.Rect().Height != 130 {
		t.Errorf("the grid is %v tall, want 130", grid.Rect().Height)
	}
}

func TestGridRowSpanLeavesRowsAloneWhenTheyAreTallEnough(t *testing.T) {
	var view View
	var tall, a, b, c Box
	f := view.Begin(testCanvas(), gfx.LightTheme(), gfx.Rect{Width: 210, Height: 400})
	f.Grid(100).Gap(10).Children(func() { // 2 columns of 100
		tall = f.Slot().Height(50).RowSpan(2)
		a = f.Slot().Height(40)
		b = f.Slot().Height(40) // under a: the first column is taken by tall
		c = f.Slot().Height(5)  // the next row, first column
	})
	f.End()
	near(t, "tall", tall.Rect(), gfx.Rect{X: 0, Y: 0, Width: 100, Height: 90})
	near(t, "a", a.Rect(), gfx.Rect{X: 110, Y: 0, Width: 100, Height: 40})
	near(t, "b skips the taken cell", b.Rect(), gfx.Rect{X: 110, Y: 50, Width: 100, Height: 40})
	near(t, "c", c.Rect(), gfx.Rect{X: 0, Y: 100, Width: 100, Height: 5})
}

func TestGridSpanWiderThanTheGridIsAsWideAsTheGrid(t *testing.T) {
	var view View
	var wide, after Box
	f := view.Begin(testCanvas(), gfx.LightTheme(), gfx.Rect{Width: 100, Height: 100})
	f.Grid(100).Children(func() {
		wide = f.Slot().Height(10).ColSpan(5)
		after = f.Slot().Height(10)
	})
	f.End()
	near(t, "wide", wide.Rect(), gfx.Rect{X: 0, Y: 0, Width: 100, Height: 10})
	near(t, "after", after.Rect(), gfx.Rect{X: 0, Y: 10, Width: 100, Height: 10})
}

func TestAbsoluteChildDoesNotChangeItsSiblingsIsAboveThemAndWinsTheHit(t *testing.T) {
	layout := func(withBadge bool) (card, badge, body, content Box, canvas *gfx.Canvas) {
		var view View
		canvas = testCanvas()
		f := view.Begin(canvas, gfx.LightTheme(), gfx.Rect{Width: 200, Height: 200})
		f.Column().Pad(10).Gap(4).Children(func() {
			card = f.Stack().Size(100, 50).Children(func() {
				body = f.Slot().Fill().Self(Stretch)
				f.Column().Fill().Hit(rowID, "card")
				if withBadge {
					badge = f.Button(save, "x").IconOnly().Absolute().Top(-6).Right(-6)
				}
			})
			content = f.Slot().Height(30)
		})
		f.End()
		return
	}
	card, _, _, content, _ := layout(false)
	card2, badge, body, content2, canvas := layout(true)
	if card.Rect() != card2.Rect() || content.Rect() != content2.Rect() {
		t.Errorf("the absolute child moved its siblings: %+v %+v against %+v %+v", card.Rect(), content.Rect(), card2.Rect(), content2.Rect())
	}
	if r := badge.Rect(); r.X+r.Width != card2.Rect().X+card2.Rect().Width+6 || r.Y != card2.Rect().Y-6 {
		t.Errorf("badge %+v is not 6 beyond the card's top right corner %+v", r, card2.Rect())
	}
	// Where the badge overlaps the card, the badge is what the pointer reaches.
	r := badge.Rect()
	if id := topHit(canvas, r.X+r.Width-2, r.Y+r.Height-2); id != save {
		t.Errorf("the pointer over the badge reaches %v, want the badge", id)
	}
	if id := topHit(canvas, body.Rect().X+5, body.Rect().Y+30); id != rowID {
		t.Errorf("the pointer on the card reaches %v, want the card", id)
	}
}

func TestAbsoluteInsetFillsAndTwoEdgesStretch(t *testing.T) {
	var view View
	var overlay, bar, corner Box
	f := view.Begin(testCanvas(), gfx.LightTheme(), gfx.Rect{Width: 200, Height: 120})
	f.Column().Fill().Pad(10).Border(2, gfx.Color{Alpha: 1}).Children(func() {
		f.Slot().Height(20)
		overlay = f.Slot().Absolute().Inset(0)
		bar = f.Slot().Absolute().Left(5).Right(15).Bottom(8).Height(12)
		corner = f.Slot().Absolute().Size(30, 10).Margin(1)
	})
	f.End()
	// The padding box: the rectangle less its border, not less its padding.
	near(t, "overlay", overlay.Rect(), gfx.Rect{X: 2, Y: 2, Width: 196, Height: 116})
	near(t, "bar", bar.Rect(), gfx.Rect{X: 7, Y: 98, Width: 176, Height: 12})
	near(t, "corner with no edges sits at the start", corner.Rect(), gfx.Rect{X: 3, Y: 3, Width: 30, Height: 10})
}

func TestAbsoluteChildIsAboveInFlowSiblingsDeclaredAfterIt(t *testing.T) {
	var view View
	canvas := testCanvas()
	first := tree.ID(page, "first")
	second := tree.ID(page, "second")
	f := view.Begin(canvas, gfx.LightTheme(), gfx.Rect{Width: 100, Height: 100})
	f.Stack().Fill().Children(func() {
		f.Column().Hit(first, "first").Absolute().Inset(0)
		f.Column().Size(50, 50).Hit(second, "second") // in flow, declared after, still under the absolute one
	})
	f.End()
	if id := topHit(canvas, 10, 10); id != first {
		t.Errorf("the pointer reaches %v, want the absolute element above the in-flow one", id)
	}
}

func TestPopupFlipsOnEachAxisAndStaysInsideTheFrame(t *testing.T) {
	var view View
	frame := gfx.Rect{Width: 400, Height: 300}
	place := func(w, h float32, fn func(Box) Box) gfx.Rect {
		f := view.Begin(testCanvas(), gfx.LightTheme(), frame)
		p := fn(f.Popup().Size(w, h))
		f.End()
		return p.Rect()
	}
	anchor := gfx.Rect{X: 150, Y: 130, Width: 100, Height: 30}
	cases := []struct {
		name string
		got  gfx.Rect
		x, y float32
	}{
		{"below fits", place(80, 50, func(b Box) Box { return b.Below(anchor, 4, Start) }), 150, 164},
		{"below flips above", place(80, 100, func(b Box) Box { return b.Below(gfx.Rect{X: 150, Y: 200, Width: 100, Height: 30}, 4, Start) }), 150, 96},
		{"above flips below", place(80, 130, func(b Box) Box { return b.Above(gfx.Rect{X: 150, Y: 100, Width: 100, Height: 30}, 4, Start) }), 150, 134},
		{"beside fits", place(60, 50, func(b Box) Box { return b.Beside(anchor, 4, Start) }), 254, 130},
		{"beside flips left", place(60, 50, func(b Box) Box { return b.Beside(gfx.Rect{X: 300, Y: 130, Width: 90, Height: 30}, 4, Start) }), 236, 130},
		{"beside-left fits", place(60, 50, func(b Box) Box { return b.BesideLeft(anchor, 4, Start) }), 86, 130},
		{"beside-left flips right", place(60, 50, func(b Box) Box { return b.BesideLeft(gfx.Rect{X: 30, Y: 130, Width: 40, Height: 30}, 4, Start) }), 74, 130},
		{"cross axis centers on the anchor", place(60, 50, func(b Box) Box { return b.Below(anchor, 4, Center) }), 170, 164},
		{"cross axis shifts, never flips", place(120, 50, func(b Box) Box { return b.Below(gfx.Rect{X: 360, Y: 10, Width: 30, Height: 20}, 4, Start) }), 280, 34},
		{"beside, aligned to the anchor's bottom, kept in the frame", place(60, 100, func(b Box) Box { return b.Beside(gfx.Rect{X: 10, Y: 20, Width: 30, Height: 30}, 4, End) }), 44, 0},
		{"beside, centered on the anchor", place(60, 50, func(b Box) Box { return b.Beside(anchor, 4, Center) }), 254, 120},
	}
	for _, c := range cases {
		if c.got.X != c.x || c.got.Y != c.y {
			t.Errorf("%s: at (%v, %v), want (%v, %v)", c.name, c.got.X, c.got.Y, c.x, c.y)
		}
	}
}

func TestPopupThatFitsNeitherSideTakesTheLargerAndIsClamped(t *testing.T) {
	var view View
	f := view.Begin(testCanvas(), gfx.LightTheme(), gfx.Rect{Width: 400, Height: 300})
	// 140 above the anchor, 110 below it: a 200 high popup fits neither, is put on the roomier
	// (above) and then moved down until it is inside the frame, covering the anchor.
	p := f.Popup().Size(80, 200).Below(gfx.Rect{X: 100, Y: 140, Width: 50, Height: 30}, 6, Start)
	f.End()
	near(t, "popup", p.Rect(), gfx.Rect{X: 100, Y: 0, Width: 80, Height: 200})
	// Taller than the frame itself: as tall as the frame, at its top, and its content gives way.
	f = view.Begin(testCanvas(), gfx.LightTheme(), gfx.Rect{Width: 400, Height: 300})
	var inner Box
	p = f.Popup().Width(80).Height(500).Below(gfx.Rect{X: 100, Y: 140, Width: 50, Height: 30}, 6, Start).Children(func() {
		inner = f.Slot().Grow(1)
	})
	f.End()
	near(t, "tall popup", p.Rect(), gfx.Rect{X: 100, Y: 0, Width: 80, Height: 300})
	if inner.Rect().Height != 300 {
		t.Errorf("the content of a limited popup is %v tall, want 300", inner.Rect().Height)
	}
}

func TestWithinReplacesTheFrameForAPopup(t *testing.T) {
	var view View
	f := view.Begin(testCanvas(), gfx.LightTheme(), gfx.Rect{Width: 400, Height: 300})
	p := f.Popup().Size(100, 40).At(395, 5).Within(gfx.Rect{X: 8, Y: 8, Width: 384, Height: 284})
	f.End()
	near(t, "popup", p.Rect(), gfx.Rect{X: 292, Y: 8, Width: 100, Height: 40})
}

func TestAbsoluteGridSpanAndPopupAllocateNothingOnceWarm(t *testing.T) {
	var view View
	canvas := testCanvas()
	theme := gfx.LightTheme()
	run := func() {
		canvas.Reset()
		view.Draw(canvas, theme, gfx.Rect{Width: 400, Height: 300}, func(f *Frame) {
			f.Column().Fill().Pad(8).Children(func() {
				f.Grid(90).Gap(6).Children(func() {
					f.Text("title").ColSpan(2)
					f.Text("tall").RowSpan(2)
					for range 7 {
						f.Column().Pad(4).Children(func() { f.Text("cell").FontSize(11) })
					}
				})
				f.Stack().Size(60, 60).Children(func() {
					f.Slot().Fill()
					f.Button(save, "x").IconOnly().Absolute().Top(-4).Right(-4)
				})
			})
			f.Column().Absolute().Right(8).Bottom(8).Gap(4).Children(func() { f.Text("toast"); f.Text("toast") })
			f.Popup().BesideLeft(gfx.Rect{X: 10, Y: 10, Width: 50, Height: 20}, 4, Center).Within(gfx.Rect{Width: 400, Height: 300}).Children(func() { f.Text("menu") })
		})
	}
	run()
	if allocs := testing.AllocsPerRun(200, run); allocs != 0 {
		t.Fatalf("a warm page with spans, absolute elements and popups allocates %v times a frame, want 0", allocs)
	}
}

func BenchmarkSpansAbsoluteAndPopups(b *testing.B) {
	var view View
	canvas := testCanvas()
	theme := gfx.LightTheme()
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		view.Draw(canvas, theme, gfx.Rect{Width: 400, Height: 300}, func(f *Frame) {
			f.Column().Fill().Pad(8).Children(func() {
				f.Grid(90).Gap(6).Children(func() {
					f.Text("title").ColSpan(2)
					f.Text("tall").RowSpan(2)
					for range 7 {
						f.Column().Pad(4).Children(func() { f.Text("cell").FontSize(11) })
					}
				})
			})
			f.Column().Absolute().Right(8).Bottom(8).Gap(4).Children(func() { f.Text("toast") })
			f.Popup().Below(gfx.Rect{X: 10, Y: 10, Width: 50, Height: 20}, 4, Start).Children(func() { f.Text("menu") })
		})
	}
}
