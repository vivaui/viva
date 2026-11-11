package ui

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// Five 60 wide children in 200: three fit the first line exactly (60+10+60+10+60),
// the other two start the next one, a Gap below.
func TestFlowWrapsOntoTheLinesThatFit(t *testing.T) {
	var view View
	var cells [5]Box
	var flow, after Box
	f := view.Begin(testCanvas(), gfx.LightTheme(), gfx.Rect{Width: 200, Height: 300})
	f.Column().Children(func() {
		flow = f.Flow().Gap(10).Children(func() {
			for i := range cells {
				cells[i] = f.Slot().Width(60).Height(20)
			}
		})
		after = f.Slot().Height(10)
	})
	f.End()
	for i, want := range []gfx.Rect{
		{X: 0, Y: 0, Width: 60, Height: 20},
		{X: 70, Y: 0, Width: 60, Height: 20},
		{X: 140, Y: 0, Width: 60, Height: 20},
		{X: 0, Y: 30, Width: 60, Height: 20},
		{X: 70, Y: 30, Width: 60, Height: 20},
	} {
		near(t, "cell", cells[i].Rect(), want)
	}
	near(t, "flow", flow.Rect(), gfx.Rect{Width: 200, Height: 50})
	if after.Rect().Y != 50 {
		t.Errorf("the element after the flow starts at %v, want 50 (two lines and a gap)", after.Rect().Y)
	}
}

// LineGap sets the space between lines apart from the space between children, and
// Pad insets the lines as any container's content.
func TestFlowLineGapAndPad(t *testing.T) {
	var view View
	var first, second Box
	var flow Box
	f := view.Begin(testCanvas(), gfx.LightTheme(), gfx.Rect{Width: 146, Height: 300}) // 130 inside the Pad: two children a line
	f.Column().Children(func() {
		flow = f.Flow().Pad(8).Gap(10).LineGap(4).Children(func() {
			first = f.Slot().Width(60).Height(20)
			f.Slot().Width(60).Height(20)
			second = f.Slot().Width(60).Height(20)
		})
	})
	f.End()
	near(t, "first", first.Rect(), gfx.Rect{X: 8, Y: 8, Width: 60, Height: 20})
	near(t, "second line", second.Rect(), gfx.Rect{X: 8, Y: 32, Width: 60, Height: 20})
	if flow.Rect().Height != 8+20+4+20+8 {
		t.Errorf("flow %v tall, want 60", flow.Rect().Height)
	}
}

// A line is as tall as its tallest child, and Items lines the children up within
// their own line, not the whole flow.
func TestFlowLineIsAsTallAsItsTallestChild(t *testing.T) {
	var view View
	var short, tall, next Box
	f := view.Begin(testCanvas(), gfx.LightTheme(), gfx.Rect{Width: 100, Height: 300})
	f.Column().Children(func() {
		f.Flow().Gap(6).Items(Center).Children(func() {
			short = f.Slot().Width(40).Height(20)
			tall = f.Slot().Width(40).Height(40)
			next = f.Slot().Width(40).Height(10)
		})
	})
	f.End()
	near(t, "short", short.Rect(), gfx.Rect{X: 0, Y: 10, Width: 40, Height: 20})
	near(t, "tall", tall.Rect(), gfx.Rect{X: 46, Y: 0, Width: 40, Height: 40})
	near(t, "next line", next.Rect(), gfx.Rect{X: 0, Y: 46, Width: 40, Height: 10})
}

// Each line shares its own room: Grow takes what is left on its line, and Justify
// places every line alone.
func TestFlowGrowAndJustifyApplyToEachLine(t *testing.T) {
	var view View
	var a, b, c Box
	draw := func(justify Align, grow float32) {
		f := view.Begin(testCanvas(), gfx.LightTheme(), gfx.Rect{Width: 200, Height: 100})
		f.Column().Children(func() {
			f.Flow().Justify(justify).Children(func() {
				a = f.Slot().Width(80).Height(10)
				b = f.Slot().Width(80).Height(10).Grow(grow)
				c = f.Slot().Width(80).Height(10).Grow(grow)
			})
		})
		f.End()
	}
	draw(Start, 1)
	near(t, "grown b", b.Rect(), gfx.Rect{X: 80, Y: 0, Width: 120, Height: 10})
	near(t, "grown c", c.Rect(), gfx.Rect{X: 0, Y: 10, Width: 200, Height: 10})
	draw(End, 0)
	near(t, "a at the end", a.Rect(), gfx.Rect{X: 40, Y: 0, Width: 80, Height: 10})
	near(t, "c at the end", c.Rect(), gfx.Rect{X: 120, Y: 10, Width: 80, Height: 10})
}

// A child wider than the flow has a line to itself and gives way to it as in a Row:
// a paragraph wraps there, and its line is as tall as its lines of text.
func TestFlowGivesAWideChildALineOfItsOwn(t *testing.T) {
	var view View
	var before, text, after Box
	f := view.Begin(testCanvas(), gfx.LightTheme(), gfx.Rect{Width: 100, Height: 300})
	f.Column().Children(func() {
		f.Flow().Gap(4).Children(func() {
			before = f.Slot().Width(40).Height(20)
			text = f.Paragraph("one two three four five six seven", 5).FontSize(10) // 165 wide on one line
			after = f.Slot().Width(40).Height(20)
		})
	})
	f.End()
	got := text.Rect()
	if got.X != 0 || got.Y != 24 || got.Width != 100 || got.Height <= 17 {
		t.Errorf("the paragraph is at %+v, want its own 100 wide line at y=24, wrapped", got)
	}
	near(t, "after", after.Rect(), gfx.Rect{X: 0, Y: got.Y + got.Height + 4, Width: 40, Height: 20})
	near(t, "before", before.Rect(), gfx.Rect{X: 0, Y: 0, Width: 40, Height: 20})
}

// A flow asks for one line, as a Row does: where it has the room it does not wrap,
// and where a Row squeezes it the flow wraps instead.
func TestFlowWrapsOnlyWhereItIsGivenLess(t *testing.T) {
	var view View
	var cells [4]Box
	var flow, side Box
	draw := func(width float32) {
		f := view.Begin(testCanvas(), gfx.LightTheme(), gfx.Rect{Width: width, Height: 100})
		f.Row().Children(func() {
			flow = f.Flow().Children(func() {
				for i := range cells {
					cells[i] = f.Slot().Width(60).Height(20)
				}
			})
			side = f.Slot().Width(100).Height(20)
		})
		f.End()
	}
	draw(400)
	if flow.Rect().Width != 240 || flow.Rect().Height != 20 || cells[3].Rect().Y != 0 {
		t.Errorf("with room the flow is %+v and its last child at y=%v, want 240 by 20, one line", flow.Rect(), cells[3].Rect().Y)
	}
	draw(300) // the row takes 40 from the flow: three children a line
	if flow.Rect().Width != 200 || side.Rect().X != 200 || side.Rect().Width != 100 {
		t.Errorf("squeezed, the flow is %v wide and the slot beside it at x=%v, %v wide, want 200, 200 and 100", flow.Rect().Width, side.Rect().X, side.Rect().Width)
	}
	near(t, "wrapped", cells[3].Rect(), gfx.Rect{X: 0, Y: 20, Width: 60, Height: 20})
}

// Items(Baseline) lines up the text of a line on its first line of text.
func TestFlowLinesUpBaselinesWithinALine(t *testing.T) {
	var view View
	var small, large, below Box
	f := view.Begin(testCanvas(), gfx.LightTheme(), gfx.Rect{Width: 60, Height: 300}) // the 50 wide text does not fit after the 30 of the others
	f.Column().Children(func() {
		f.Flow().Items(Baseline).Children(func() {
			small = f.Text("ab").FontSize(10) // baseline 11.5 below its top
			large = f.Text("ab").FontSize(20) // baseline 19.5 below its top
			below = f.Text("abcdefghij").FontSize(10)
		})
	})
	f.End()
	if small.Rect().Y != 8 || large.Rect().Y != 0 {
		t.Errorf("texts at y=%v and %v, want 8 and 0: on one baseline", small.Rect().Y, large.Rect().Y)
	}
	if below.Rect().Y != 27 {
		t.Errorf("the second line starts at %v, want 27, under the 20 pt text", below.Rect().Y)
	}
}

func TestFlowAllocatesNothingOnceWarm(t *testing.T) {
	var view View
	canvas := testCanvas()
	theme := gfx.LightTheme()
	tags := []string{"main", "origin/main", "v1.2.0", "release", "HEAD", "feature/a-long-branch-name", "fix"}
	run := func() {
		canvas.Reset()
		view.Draw(canvas, theme, gfx.Rect{Width: 220, Height: 400}, func(f *Frame) {
			f.Column().Fill().Pad(8).Children(func() {
				f.Flow().Gap(6).LineGap(4).Items(Center).Children(func() {
					for _, tag := range tags {
						f.Row().PadX(6).Border(1, theme.Border).Children(func() { f.Text(tag).FontSize(11).Ellipsis() })
					}
				})
			})
		})
	}
	run()
	if allocs := testing.AllocsPerRun(200, run); allocs != 0 {
		t.Fatalf("a warm page with a flow allocates %v times a frame, want 0", allocs)
	}
}

func BenchmarkFlow(b *testing.B) {
	var view View
	canvas := testCanvas()
	theme := gfx.LightTheme()
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		view.Draw(canvas, theme, gfx.Rect{Width: 400, Height: 600}, func(f *Frame) {
			f.Flow().Gap(6).Children(func() {
				for range 200 {
					f.Row().PadX(6).Children(func() { f.Text("chip").FontSize(11) })
				}
			})
		})
	}
}
