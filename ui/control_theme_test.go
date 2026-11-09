package ui

import (
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

var (
	ctlFirst  = tree.ID(page, "ctl-first")
	ctlSecond = tree.ID(page, "ctl-second")
	ctlScope  = tree.Sub(page, tree.Name("ctl-select"))
)

// Radius and FontSize on a control are the radius and font size of that control
// alone: it is painted exactly as the component is with a theme of the same
// overrides, and the control beside it keeps the theme's.
func TestRadiusAndFontSizeStyleTheControlTheyAreCalledOn(t *testing.T) {
	theme := gfx.LightTheme()
	snapshot := func(id gfx.ID) components.InputSnapshot {
		return components.InputSnapshot{ID: id, Label: "Name", Value: "Gopher", Focused: id == ctlFirst, ShowCaret: true, Caret: 2, SelectionStart: 0, SelectionEnd: 1}
	}
	var view View
	flexed := testCanvas()
	view.Draw(flexed, theme, gfx.Rect{Width: 300, Height: 200}, func(f *Frame) {
		f.Column().Gap(8).Children(func() {
			f.Input(snapshot(ctlFirst)).Radius(gfx.RadiusFull).FontSize(20).Height(48)
			f.Input(snapshot(ctlSecond))
		})
	})

	direct := testCanvas()
	components.PaintInput(direct, theme.WithRadius(gfx.RadiusFull).WithFontSize(20), gfx.Rect{Width: 300, Height: 48}, snapshot(ctlFirst))
	components.PaintInput(direct, theme, gfx.Rect{Y: 56, Width: 300}, snapshot(ctlSecond))
	sameCanvas(t, "a styled input beside a plain one", flexed, direct)
	if len(flexed.Rectangles()) == 0 || flexed.Rectangles()[0].Radius != gfx.RadiusFull {
		t.Fatalf("the first input's corners are %+v, want a full radius", flexed.Rectangles())
	}
}

// A radius of zero is a choice, not the absence of one: a square field.
func TestRadiusZeroSquaresAControl(t *testing.T) {
	var view View
	canvas := testCanvas()
	view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 200, Height: 60}, func(f *Frame) {
		f.Input(components.InputSnapshot{ID: ctlFirst, Label: "x"}).Radius(0)
	})
	if got := canvas.Rectangles()[0].Radius; got != 0 {
		t.Fatalf("radius = %v, want a square corner", got)
	}
}

// The size a control is laid out at follows its font size, so the box and the
// text in it agree: a bigger button is wider, and keeps its label on its baseline.
func TestFontSizeSizesAControlToItsText(t *testing.T) {
	theme := gfx.LightTheme()
	widthOf := func(declare func(f *Frame) Box) (float32, float32) {
		var view View
		canvas := testCanvas()
		var box Box
		view.Draw(canvas, theme, gfx.Rect{Width: 600, Height: 200}, func(f *Frame) {
			f.Row().Items(Start).Children(func() { box = declare(f) })
		})
		r := box.Rect()
		return r.Width, r.Height
	}
	plainW, _ := widthOf(func(f *Frame) Box { return f.Button(save, "Save changes") })
	bigW, _ := widthOf(func(f *Frame) Box { return f.Button(save, "Save changes").FontSize(24) })
	if bigW <= plainW {
		t.Fatalf("a 24px button is %v wide against %v at the theme's size", bigW, plainW)
	}
}

// A Select's open list is as wide as its labels at the Select's own size.
func TestFontSizeWidensASelectAndItsList(t *testing.T) {
	theme := gfx.LightTheme()
	width := func(size float32) (trigger, list float32) {
		state := SelectState{Open: true}
		var view View
		canvas := testCanvas()
		var box Box
		view.Draw(canvas, theme, gfx.Rect{Width: 800, Height: 600}, func(f *Frame) {
			f.Row().Items(Start).Children(func() {
				box = f.Select(ctlScope, &state, []string{"A fairly long option label", "Short"}, 0)
				if size > 0 {
					box.FontSize(size)
				}
			})
		})
		n := box.n()
		return n.rect.Width, box.f.nodes[box.f.widgets.selects[n.aux].popup].size[axisX].value
	}
	plainTrigger, plainList := width(0)
	bigTrigger, bigList := width(26)
	if bigTrigger <= plainTrigger || bigList <= plainList {
		t.Fatalf("at 26px the trigger is %v and the list %v wide, against %v and %v", bigTrigger, bigList, plainTrigger, plainList)
	}
}

func TestStyledControlsAllocateNothingOnceWarm(t *testing.T) {
	var view View
	canvas := testCanvas()
	theme := gfx.LightTheme()
	snapshot := components.InputSnapshot{ID: ctlFirst, Label: "Name", Value: "Gopher"}
	run := func() {
		canvas.Reset()
		view.Draw(canvas, theme, gfx.Rect{Width: 400, Height: 300}, func(f *Frame) {
			f.Column().Gap(8).Children(func() {
				f.Input(snapshot).Radius(gfx.RadiusFull).FontSize(18)
				f.Button(save, "Save").Radius(4).FontSize(16)
			})
		})
	}
	run()
	if allocs := testing.AllocsPerRun(200, run); allocs != 0 {
		t.Fatalf("a warm page of styled controls allocates %v times a frame, want 0", allocs)
	}
}

// A Height on a button is the button's height, painted and hit, not only the room
// it takes: the button fills it. Without one it is as tall as its size.
func TestButtonHeightIsThePaintedHeight(t *testing.T) {
	theme := gfx.LightTheme()
	draw := func(declare func(f *Frame) Box) (rect gfx.Rect, painted gfx.Rect, hit gfx.Rect) {
		var view View
		canvas := testCanvas()
		var box Box
		view.Draw(canvas, theme, gfx.Rect{Width: 400, Height: 300}, func(f *Frame) {
			f.Row().Items(Start).Children(func() { box = declare(f) })
		})
		return box.Rect(), canvas.Rectangles()[0].Bounds, canvas.Hits()[0].Bounds
	}
	rect, painted, hit := draw(func(f *Frame) Box { return f.Button(save, "Save") })
	if painted.Height != theme.Metrics.ControlHeight || rect.Height != painted.Height || hit.Height != painted.Height {
		t.Fatalf("a plain button: box %v, painted %v, hit %v tall", rect.Height, painted.Height, hit.Height)
	}
	rect, painted, hit = draw(func(f *Frame) Box { return f.Button(save, "Save").Height(48) })
	if rect.Height != 48 || painted.Height != 48 || hit.Height != 48 {
		t.Fatalf("Height(48): box %v, painted %v, hit %v tall", rect.Height, painted.Height, hit.Height)
	}
	rect, painted, _ = draw(func(f *Frame) Box { return f.Button(save, "").IconOnly().Height(44) })
	if rect.Width != 44 || painted.Width != 44 || painted.Height != 44 {
		t.Fatalf("a 44px icon button: box %vx%v, painted %vx%v, want square", rect.Width, rect.Height, painted.Width, painted.Height)
	}
}
