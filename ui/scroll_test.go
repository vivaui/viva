package ui

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
)

// logPage draws a scroll area of n rows of 30 in a window 100 tall, the last row's top
// edge being what the area shows at its end.
func logPage(view *View, follow bool, rows int) (last Box) {
	f := view.Begin(testCanvas(), gfx.LightTheme(), gfx.Rect{Width: 100, Height: 100})
	f.Column().Fill().Children(func() {
		f.ScrollArea(listID).Grow(1).Follow(follow).Children(func() {
			for range rows {
				last = f.Slot().Height(30)
			}
		})
	})
	f.End()
	return last
}

func TestScrollToEndHoldsInTheFrameThatAddsContent(t *testing.T) {
	var view View
	logPage(&view, false, 10)
	state := view.ScrollState(listID)
	state.ScrollToEnd()
	last := logPage(&view, false, 11) // one more row, added in the frame that scrolls
	if got := last.Rect().Y + last.Rect().Height; got != 100 {
		t.Errorf("the last row ends at %v, want the bottom of the area, 100", got)
	}
	if state.Offset != 230 || state.Offset != state.Max() {
		t.Errorf("offset %v, max %v, want both 230", state.Offset, state.Max())
	}
	state.ScrollTo(40)
	if last := logPage(&view, false, 11); last.Rect().Y != 330-40-30 {
		t.Errorf("after ScrollTo(40) the last row is at %v, want %v", last.Rect().Y, 330-40-30)
	}
}

func TestFollowStaysAtTheEndUntilTheUserScrollsUpAndReturns(t *testing.T) {
	var view View
	if last := logPage(&view, true, 10); last.Rect().Y+last.Rect().Height != 100 {
		t.Fatalf("a log starts at its end, last row at %v", last.Rect())
	}
	state := view.ScrollState(listID)
	if last := logPage(&view, true, 12); last.Rect().Y+last.Rect().Height != 100 || state.Offset != state.Max() {
		t.Errorf("new lines did not carry the area along: last row %v offset %v", last.Rect(), state.Offset)
	}
	state.Wheel(gfx.ScrollEvent{DeltaY: -60, Precise: true}) // the user reads older lines
	before := state.Offset
	logPage(&view, true, 15)
	if state.Offset != before {
		t.Errorf("the area moved from %v to %v under a user who scrolled up", before, state.Offset)
	}
	state.Wheel(gfx.ScrollEvent{DeltaY: 5000, Precise: true}) // and comes back to the bottom
	if last := logPage(&view, true, 17); last.Rect().Y+last.Rect().Height != 100 {
		t.Errorf("back at the end it did not follow again: last row %v", last.Rect())
	}
}

func TestAViewGivesTheWheelToTheAreaUnderThePointer(t *testing.T) {
	var view View
	left, right, inner, outer := tree.ID(page, "left"), tree.ID(page, "right"), tree.ID(page, "inner"), tree.ID(page, "outer")
	offset := func(id gfx.ID) float32 { return view.ScrollState(id).Offset }
	draw := func() {
		f := view.Begin(testCanvas(), gfx.LightTheme(), gfx.Rect{Width: 400, Height: 100})
		f.Row().Fill().Children(func() {
			f.ScrollArea(left).Width(100).Children(func() {
				for range 10 {
					f.Slot().Height(30)
				}
			})
			f.ScrollArea(right).Width(100).Children(func() {
				for range 10 {
					f.Slot().Height(30)
				}
			})
			f.ScrollArea(outer).Width(200).Children(func() {
				f.Slot().Height(60)
				f.ScrollArea(inner).Height(60).Children(func() {
					for range 10 {
						f.Slot().Height(30)
					}
				})
				f.Slot().Height(400)
			})
		})
		f.End()
	}
	draw()
	wheel := func(x, y, dy float32) gfx.EventResult {
		return view.Wheel(gfx.ScrollEvent{X: x, Y: y, DeltaY: dy, Precise: true})
	}
	if r := wheel(150, 50, 10); !r.Handled || offset(right) != 10 || offset(left) != 0 || offset(outer) != 0 {
		t.Errorf("over the right area: left %v right %v outer %v handled %v", offset(left), offset(right), offset(outer), r.Handled)
	}
	if wheel(50, 50, 20); offset(left) != 20 || offset(right) != 10 {
		t.Errorf("over the left area: left %v right %v", offset(left), offset(right))
	}
	if wheel(250, 30, 15); offset(inner) != 0 || offset(outer) != 15 {
		t.Errorf("over the outer area only: inner %v outer %v", offset(inner), offset(outer))
	}
	view.ScrollState(outer).Offset = 0
	draw()
	if wheel(250, 90, 25); offset(inner) != 25 || offset(outer) != 0 {
		t.Errorf("over the inner area: inner %v outer %v", offset(inner), offset(outer))
	}
	wheel(250, 90, 5000)
	if offset(inner) != view.ScrollState(inner).Max() {
		t.Fatalf("inner at %v, want its end %v", offset(inner), view.ScrollState(inner).Max())
	}
	if wheel(250, 90, 10); offset(outer) != 10 {
		t.Errorf("an inner area at its end hands the wheel to the outer: outer %v", offset(outer))
	}
	wheel(350, 30, -10)
	if r := wheel(350, 30, -10); offset(outer) != 0 || r.Handled {
		t.Errorf("a wheel up over an area at its top: offset %v handled %v, want 0 and not handled", offset(outer), r.Handled)
	}
}

func TestMaxWidthPercentCapsABubbleAndItsParagraphWrapsInsideIt(t *testing.T) {
	var view View
	canvas := runeCanvas()
	var short, long Box
	f := view.Begin(canvas, gfx.LightTheme(), gfx.Rect{Width: 300, Height: 400})
	f.Column().Pad(10).Gap(6).Children(func() { // 280 wide inside the padding
		short = f.Column().Self(End).MaxWidthPercent(70).Pad(8).Children(func() { f.Wrap("hi").FontSize(10) })
		long = f.Column().Self(End).MaxWidthPercent(70).Pad(8).Children(func() {
			f.Wrap("aaaa bbbb cccc dddd eeee ffff gggg hhhh").FontSize(10)
		})
	})
	f.End()
	if w := short.Rect().Width; w != 36 {
		t.Errorf("a short message is %v wide, want 36 (its text and padding)", w)
	}
	if w := long.Rect().Width; w < 195.9 || w > 196.1 {
		t.Errorf("a long message is %v wide, want 70%% of 280 = 196", w)
	}
	if got := long.Rect().X + long.Rect().Width; got != 290 {
		t.Errorf("the bubble ends at %v, want the right edge, 290", got)
	}
	if h := long.Rect().Height; h <= 17+16 {
		t.Errorf("the long message is %v tall: its text did not wrap inside the cap", h)
	}
}

// A row child, where the cap acts on the flow axis.
func TestMaxWidthPercentOnAFlowChild(t *testing.T) {
	var view View
	var box Box
	f := view.Begin(testCanvas(), gfx.LightTheme(), gfx.Rect{Width: 200, Height: 40})
	f.Row().Fill().Children(func() {
		box = f.Slot().Width(180).MaxWidthPercent(50)
	})
	f.End()
	if w := box.Rect().Width; w != 100 {
		t.Errorf("width %v, want 100", w)
	}
}

func TestScrollingAllocatesNothingOnceWarm(t *testing.T) {
	var view View
	canvas := testCanvas()
	draw := func() {
		view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 100, Height: 100}, func(f *Frame) {
			f.Column().Fill().Children(func() {
				f.ScrollArea(listID).Grow(1).Follow(true).Children(func() {
					for range 10 {
						f.Slot().Height(30).MaxWidthPercent(80)
					}
				})
			})
		})
	}
	draw()
	if got := testing.AllocsPerRun(50, func() { draw(); view.Wheel(gfx.ScrollEvent{X: 5, Y: 5, DeltaY: -3}) }); got != 0 {
		t.Errorf("%v allocations per frame, want 0", got)
	}
}

// A page that never keeps a scroll position of its own still keeps where each area is
// scrolled to, by identity, and forgets an area that a frame does not declare: it comes
// back at the top, and the View does not grow with the areas a page has had.
func TestAScrollAreaKeepsItsOffsetInTheViewAndDropsItWhenNotDeclared(t *testing.T) {
	var view View
	draw := func(ids ...gfx.ID) {
		f := view.Begin(testCanvas(), gfx.LightTheme(), gfx.Rect{Width: 100, Height: 100})
		f.Column().Fill().Children(func() {
			for _, id := range ids {
				f.ScrollArea(id).Height(50).Children(func() { f.Slot().Height(300) })
			}
		})
		f.End()
	}
	a, b := tree.ID(page, "a"), tree.ID(page, "b")
	draw(a, b)
	view.ScrollState(a).ScrollTo(70)
	draw(a, b)
	if view.ScrollState(a).Offset != 70 || view.ScrollState(b).Offset != 0 {
		t.Fatalf("offsets a %v b %v, want 70 and 0", view.ScrollState(a).Offset, view.ScrollState(b).Offset)
	}
	draw(b) // a's tab is not shown
	draw(a, b)
	if got := view.ScrollState(a).Offset; got != 0 {
		t.Errorf("an area that was not declared for a frame came back at %v, want the top", got)
	}
	for i := range 500 { // a page that makes up a new area every frame
		draw(gfx.Item(i + 1))
	}
	if n := len(view.frame.scrolls.slots) + len(view.frame.scrolls.free); n > 4 {
		t.Errorf("the view holds %d scroll slots after 500 frames of one area each, want the few it needs", n)
	}
}

func TestScrollAreaWithoutAnIdentityFailsLoudly(t *testing.T) {
	defer func() {
		if message, _ := recover().(string); !strings.Contains(message, "identity") || !strings.Contains(message, "ScrollArea") {
			t.Errorf("panic %q does not say what to pass", message)
		}
	}()
	var view View
	view.Draw(testCanvas(), gfx.LightTheme(), gfx.Rect{Width: 100, Height: 100}, func(f *Frame) { f.ScrollArea(gfx.ID{}) })
}

// BenchmarkTenScrollAreas is what keying the state by identity costs: a page of ten
// areas looks each one up and sweeps the store every frame.
func BenchmarkTenScrollAreas(b *testing.B) {
	var view View
	canvas := testCanvas()
	ids := make([]gfx.ID, 10)
	for i := range ids {
		ids[i] = gfx.Item(i + 1)
	}
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 400, Height: 300}, func(f *Frame) {
			f.Row().Fill().Children(func() {
				for _, id := range ids {
					f.ScrollArea(id).Width(40).Children(func() { f.Slot().Height(500) })
				}
			})
		})
	}
}
