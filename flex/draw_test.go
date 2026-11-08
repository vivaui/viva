package flex

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

var (
	hookRow   = tree.ID(page, "hook-row")
	hookOther = tree.ID(page, "hook-other")

	hookUnder = ui.Color{Red: 1, Alpha: 1}
	hookOver  = ui.Color{Green: 1, Alpha: 1}
	hookFill  = ui.Color{Blue: 1, Alpha: 1}
	hookChild = ui.Color{Red: .5, Green: .5, Alpha: 1}
)

// colorIndex is where the first rectangle of that color was painted, or -1.
func colorIndex(canvas *ui.Canvas, color ui.Color) int {
	for i, rect := range canvas.Rectangles() {
		if rect.Color == color {
			return i
		}
	}
	return -1
}

// Draw paints after the element's own fill and before its children; DrawOver after
// them: the order a page is painted in, which a Slot filled after End cannot give.
func TestDrawAndDrawOverPaintBetweenTheBackgroundAndTheChildren(t *testing.T) {
	var view View
	canvas := testCanvas()
	var box Box
	view.Draw(canvas, ui.LightTheme(), ui.Rect{Width: 200, Height: 100}, func(f *Frame) {
		box = f.Row().Size(100, 40).Background(hookFill).Children(func() {
			f.Row().Size(10, 10).Background(hookChild)
		}).Draw(func(d *DrawContext) {
			d.Canvas.Fill(d.Rect, hookUnder)
		}).DrawOver(func(d *DrawContext) {
			d.Canvas.Fill(d.Rect, hookOver)
		})
	})
	fill, under, child, over := colorIndex(canvas, hookFill), colorIndex(canvas, hookUnder), colorIndex(canvas, hookChild), colorIndex(canvas, hookOver)
	if fill < 0 || under < 0 || child < 0 || over < 0 {
		t.Fatalf("a layer is missing: fill %d, draw %d, child %d, hookOver %d", fill, under, child, over)
	}
	if !(fill < under && under < child && child < over) {
		t.Fatalf("paint order fill %d, Draw %d, child %d, DrawOver %d, want increasing", fill, under, child, over)
	}
	if got := canvas.Rectangles()[under].Bounds; got != box.Rect() {
		t.Fatalf("Draw got %+v, want the laid-out element %+v", got, box.Rect())
	}
}

// The rectangle is known when the callback runs, wherever layout put the element, and
// the callback sees the element it was declared on and no other.
func TestDrawGetsTheLaidOutRectangleOfItsOwnElement(t *testing.T) {
	var view View
	canvas := testCanvas()
	var got [2]ui.Rect
	var boxes [2]Box
	view.Draw(canvas, ui.LightTheme(), ui.Rect{Width: 300, Height: 100}, func(f *Frame) {
		f.Row().Gap(10).Pad(6).Children(func() {
			boxes[0] = f.Row().Width(80).Height(20).Draw(func(d *DrawContext) { got[0] = d.Rect })
			f.Spacer()
			boxes[1] = f.Row().Width(50).Height(30).Draw(func(d *DrawContext) { got[1] = d.Rect })
		})
	})
	for i := range got {
		if got[i] != boxes[i].Rect() || got[i].Width == 0 {
			t.Errorf("element %d: callback saw %+v, layout put it at %+v", i, got[i], boxes[i].Rect())
		}
	}
}

// What is drawn is inside the element's Clip and faded with it: the properties an
// overlay painted after End does not have.
func TestDrawIsClippedAndFadedWithItsElement(t *testing.T) {
	var view View
	canvas := testCanvas()
	var seen float32
	var inside ui.Color
	view.Draw(canvas, ui.LightTheme(), ui.Rect{Width: 200, Height: 100}, func(f *Frame) {
		f.Row().Size(100, 40).Clip().Opacity(0.5).Draw(func(d *DrawContext) {
			seen = d.Opacity
			inside = d.Fade(hookUnder)
			d.Canvas.Fill(d.Rect, inside)
		})
	})
	if seen != 0.5 || inside.Alpha != 0.5 {
		t.Fatalf("opacity %v and faded alpha %v, want 0.5", seen, inside.Alpha)
	}
	i := colorIndex(canvas, inside)
	if i < 0 || !canvas.Rectangles()[i].HasClip {
		t.Fatalf("the drawing was not clipped to its element: %+v", canvas.Rectangles())
	}
}

// State reads the pointer and keyboard from the element's identity, so a custom drawing
// follows hover, press and focus without Viva knowing what it draws.
func TestDrawStateFollowsTheElementsIdentity(t *testing.T) {
	run := func(hover, pressed, focus ui.ID) (state ui.State, other ui.State) {
		var view View
		canvas := testCanvas()
		canvas.SetInteraction(hover, pressed, focus, true)
		view.Draw(canvas, ui.LightTheme(), ui.Rect{Width: 200, Height: 100}, func(f *Frame) {
			f.Column().Children(func() {
				f.Row().Size(80, 20).Hit(hookRow, "row").Draw(func(d *DrawContext) { state = d.State })
				f.Row().Size(80, 20).Hit(hookOther, "other").Draw(func(d *DrawContext) { other = d.State })
			})
		})
		return
	}
	if s, o := run(hookRow, ui.ID{}, ui.ID{}); !s.Hovered || s.Pressed || s.Focused || o.Hovered {
		t.Errorf("hover: row %+v, other %+v", s, o)
	}
	if s, o := run(hookRow, hookRow, ui.ID{}); !s.Pressed || o.Pressed {
		t.Errorf("press: row %+v, other %+v", s, o)
	}
	if s, o := run(ui.ID{}, ui.ID{}, hookRow); !s.Focused || !s.FocusVisible || o.Focused {
		t.Errorf("focus: row %+v, other %+v", s, o)
	}
	if s, _ := run(ui.ID{}, ui.ID{}, ui.ID{}); s != (ui.State{}) {
		t.Errorf("rest: %+v, want the zero state", s)
	}
}

func TestBoxStateIsAvailableWhileDeclaring(t *testing.T) {
	var view View
	canvas := testCanvas()
	canvas.SetInteraction(hookRow, ui.ID{}, ui.ID{}, false)
	var state ui.State
	view.Draw(canvas, ui.LightTheme(), ui.Rect{Width: 100, Height: 40}, func(f *Frame) {
		state = f.Row().Size(80, 20).Hit(hookRow, "row").Disabled(true).State()
	})
	if !state.Hovered || !state.Disabled {
		t.Fatalf("state = %+v, want hovered and disabled", state)
	}
}

// A callback is not kept past the frame it was declared in, so what it captured can be
// collected, and a frame that declares none paints none.
func TestDrawCallbacksDoNotOutliveTheirFrame(t *testing.T) {
	var view View
	canvas := testCanvas()
	calls := 0
	declare := func(withDraw bool) {
		canvas.Reset()
		view.Draw(canvas, ui.LightTheme(), ui.Rect{Width: 100, Height: 40}, func(f *Frame) {
			b := f.Row().Size(80, 20)
			if withDraw {
				b.Draw(func(*DrawContext) { calls++ })
			}
		})
	}
	declare(true)
	declare(false)
	if calls != 1 {
		t.Fatalf("callback ran %d times over two frames, want once", calls)
	}
}

func TestDrawIsAllocationFreeAfterWarmup(t *testing.T) {
	var view View
	canvas := testCanvas()
	fn := func(d *DrawContext) { d.Canvas.Fill(d.Rect, d.Fade(hookUnder)) }
	draw := func() {
		canvas.Reset()
		view.Draw(canvas, ui.LightTheme(), ui.Rect{Width: 200, Height: 100}, func(f *Frame) {
			f.Row().Size(100, 40).Draw(fn).DrawOver(fn)
		})
	}
	draw()
	if allocs := testing.AllocsPerRun(100, draw); allocs != 0 {
		t.Fatalf("allocs = %v, want 0", allocs)
	}
}

func BenchmarkDraw(b *testing.B) {
	var view View
	canvas := testCanvas()
	fn := func(d *DrawContext) { d.Canvas.Fill(d.Rect, d.Fade(hookUnder)) }
	b.ReportAllocs()
	for range b.N {
		canvas.Reset()
		view.Draw(canvas, ui.LightTheme(), ui.Rect{Width: 200, Height: 100}, func(f *Frame) {
			f.Row().Size(100, 40).Draw(fn).DrawOver(fn)
		})
	}
}
