package flex

import (
	"testing"
	"time"

	"github.com/vivaui/viva/ui"
)

var (
	styleRow   = tree.ID(page, "style-row")
	styleChild = tree.ID(page, "style-child")

	colorRest    = ui.Color{Red: 0.1, Green: 0.2, Blue: 0.3, Alpha: 1}
	colorHover   = ui.Color{Red: 0.9, Green: 0.8, Blue: 0.1, Alpha: 1}
	colorPressed = ui.Color{Red: 0.2, Green: 0.9, Blue: 0.2, Alpha: 1}
	colorRing    = ui.Color{Red: 1, Green: 0, Blue: 1, Alpha: 0.5}
)

// styleCanvas is a canvas whose pointer and focus are as the test says.
func styleCanvas(hovered, pressed, focused ui.ID, focusVisible bool) *ui.Canvas {
	canvas := testCanvas()
	canvas.SetInteraction(hovered, pressed, focused, focusVisible)
	return canvas
}

// drawRow paints a 100x40 row that has all three looks and returns it.
func drawRow(canvas *ui.Canvas, build func(f *Frame, row Box)) Box {
	var view View
	var row Box
	view.Draw(canvas, ui.LightTheme(), ui.Rect{Width: 300, Height: 200}, func(f *Frame) {
		f.Column().Pad(20).Children(func() {
			row = f.Row().Size(100, 40).Radius(8).Background(colorRest).Hit(styleRow, "row").
				Hover(Look{}.Background(colorHover)).
				Pressed(Look{}.Background(colorPressed)).
				Focus(Look{}.Ring(3, colorRing))
			if build != nil {
				build(f, row)
			}
		})
	})
	return row
}

// fillOf is the color the row's rectangle was painted with.
func fillOf(t *testing.T, canvas *ui.Canvas, row Box) ui.Color {
	t.Helper()
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Bounds == row.Rect() {
			return rectangle.Color
		}
	}
	t.Fatalf("nothing was painted at the row's rectangle %+v: %+v", row.Rect(), canvas.Rectangles())
	return ui.Color{}
}

// ringTriangles counts the triangles of a ring of the ring color.
func ringTriangles(canvas *ui.Canvas) (count int, left, top float32) {
	left, top = 1e9, 1e9
	for _, triangle := range canvas.Triangles() {
		if triangle.Color != colorRing {
			continue
		}
		count++
		for _, point := range triangle.Points {
			left, top = min(left, point.X), min(top, point.Y)
		}
	}
	return count, left, top
}

func TestLooksApplyWhileTheirStateHolds(t *testing.T) {
	none := ui.ID{}
	for _, test := range []struct {
		name                      string
		hovered, pressed, focused ui.ID
		focusVisible              bool
		fill                      ui.Color
		ring                      bool
	}{
		{"rest", none, none, none, false, colorRest, false},
		{"hover", styleRow, none, none, false, colorHover, false},
		{"pressed wins over hover", styleRow, styleRow, none, false, colorPressed, false},
		{"another element's hover is not ours", styleChild, none, none, false, colorRest, false},
		{"keyboard focus draws the ring", none, none, styleRow, true, colorRest, true},
		{"a click's focus does not", none, none, styleRow, false, colorRest, false},
		{"focus and hover combine", styleRow, none, styleRow, true, colorHover, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			canvas := styleCanvas(test.hovered, test.pressed, test.focused, test.focusVisible)
			row := drawRow(canvas, nil)
			if got := fillOf(t, canvas, row); got != test.fill {
				t.Errorf("fill is %+v, want %+v", got, test.fill)
			}
			if count, _, _ := ringTriangles(canvas); (count > 0) != test.ring {
				t.Errorf("ring painted: %v, want %v", count > 0, test.ring)
			}
		})
	}
}

func TestAnElementWithoutIdentityIgnoresItsLooks(t *testing.T) {
	canvas := styleCanvas(styleRow, styleRow, styleRow, true)
	var view View
	var box Box
	view.Draw(canvas, ui.LightTheme(), ui.Rect{Width: 100, Height: 100}, func(f *Frame) {
		box = f.Row().Size(50, 50).Background(colorRest).Hover(Look{}.Background(colorHover)).Focus(Look{}.Ring(3, colorRing))
	})
	if got := fillOf(t, canvas, box); got != colorRest {
		t.Errorf("fill is %+v, want the resting one", got)
	}
	if count, _, _ := ringTriangles(canvas); count != 0 {
		t.Error("a ring was painted")
	}
}

func TestStateFromMakesWhatIsInsideFollowTheContainer(t *testing.T) {
	text := ui.Color{Red: 0.5, Alpha: 1}
	for _, hovered := range []bool{false, true} {
		var id ui.ID
		if hovered {
			id = styleRow
		}
		canvas := styleCanvas(id, ui.ID{}, ui.ID{}, false)
		var view View
		view.Draw(canvas, ui.LightTheme(), ui.Rect{Width: 300, Height: 100}, func(f *Frame) {
			row := f.Row().Size(200, 40).Hit(styleRow, "row")
			row.Children(func() {
				f.Text("name").Color(colorRest).FontSize(10)
				f.Text("edit").Color(text).FontSize(10).Opacity(0).StateFrom(row).Hover(Look{}.Opacity(1).Color(colorHover))
			})
		})
		var edit *ui.Text
		for index, shown := range canvas.Texts() {
			if shown.Value == "edit" {
				edit = &canvas.Texts()[index]
			}
		}
		switch {
		case hovered && (edit == nil || edit.Style.Color != colorHover):
			t.Fatalf("hovering the row, the text inside it is %+v, want %+v", edit, colorHover)
		case !hovered && edit != nil && edit.Style.Color.Alpha != 0:
			t.Fatalf("at rest the text inside it is %+v, want it invisible", edit)
		}
	}
}

// drawFrames paints the row once per entry of times, with the hover as given, and
// returns the fill after each.
func TestTransitionFadesBetweenLooksAndSettles(t *testing.T) {
	canvas := testCanvas()
	var view View
	now := time.Duration(0)
	canvas.SetClock(func() time.Duration { return now })
	frame := func(at time.Duration, hovered bool) (fill ui.Color, animating bool) {
		now = at
		canvas.Reset()
		var id ui.ID
		if hovered {
			id = styleRow
		}
		canvas.SetInteraction(id, ui.ID{}, ui.ID{}, false)
		canvas.BeginFrame()
		var row Box
		view.Draw(canvas, ui.LightTheme(), ui.Rect{Width: 300, Height: 100}, func(f *Frame) {
			row = f.Row().Size(100, 40).Background(colorRest).Hit(styleRow, "row").
				Hover(Look{}.Background(colorHover)).TransitionWith(ui.Motion{Duration: 100 * time.Millisecond, Easing: ui.EaseLinear})
		})
		canvas.EndFrame()
		return fillOf(t, canvas, row), canvas.Animating()
	}
	if fill, animating := frame(0, false); fill != colorRest || animating {
		t.Fatalf("at rest the fill is %+v and animating is %v", fill, animating)
	}
	if fill, animating := frame(10*time.Millisecond, true); !animating || fill != colorRest {
		t.Fatalf("as the hover begins the fill is %+v (animating %v), want the resting one", fill, animating)
	}
	mid, animating := frame(60*time.Millisecond, true)
	if !animating {
		t.Fatal("the transition is over halfway through")
	}
	for name, parts := range map[string][3]float32{
		"red":   {colorRest.Red, mid.Red, colorHover.Red},
		"green": {colorRest.Green, mid.Green, colorHover.Green},
		"blue":  {colorRest.Blue, mid.Blue, colorHover.Blue},
	} {
		if !(parts[1] > min(parts[0], parts[2]) && parts[1] < max(parts[0], parts[2])) {
			t.Errorf("%s is %v halfway, want it between %v and %v", name, parts[1], parts[0], parts[2])
		}
	}
	if fill, animating := frame(300*time.Millisecond, true); fill != colorHover || animating {
		t.Fatalf("afterwards the fill is %+v and animating is %v, want the hover look at rest", fill, animating)
	}
	// Leaving reverses from what is shown.
	if fill, animating := frame(310*time.Millisecond, false); !animating || fill != colorHover {
		t.Fatalf("as the pointer leaves the fill is %+v (animating %v)", fill, animating)
	}
	if fill, animating := frame(600*time.Millisecond, false); fill != colorRest || animating {
		t.Fatalf("after leaving the fill is %+v and animating is %v", fill, animating)
	}
}

func TestOpacityFadesAnElementAndWhatIsInsideIt(t *testing.T) {
	canvas := testCanvas()
	var view View
	var inner, button Box
	view.Draw(canvas, ui.LightTheme(), ui.Rect{Width: 300, Height: 200}, func(f *Frame) {
		f.Column().Opacity(0.5).Children(func() {
			inner = f.Row().Size(100, 40).Opacity(0.5).Background(colorRest).Shadow(ui.ShadowMD).Children(func() {
				f.Text("faded").Color(colorHover).FontSize(10)
			})
			button = f.Button(save, "Save").Opacity(0.5)
		})
	})
	if got, want := fillOf(t, canvas, inner).Alpha, colorRest.Alpha*0.25; got != want {
		t.Errorf("the fill's alpha is %v, want %v (half of half)", got, want)
	}
	for _, shown := range canvas.Texts() {
		if shown.Value == "faded" && shown.Style.Color.Alpha != colorHover.Alpha*0.25 {
			t.Errorf("the text's alpha is %v, want a quarter", shown.Style.Color.Alpha)
		}
	}
	if len(canvas.Shadows()) != 2 {
		t.Fatalf("%d shadow layers, want the 2 of ShadowMD", len(canvas.Shadows()))
	}
	var plain ui.Canvas
	plain.Elevate(inner.Rect(), 0, ui.ShadowMD)
	for index, shadow := range canvas.Shadows() {
		if want := plain.Shadows()[index].Color.Alpha * 0.25; shadow.Color.Alpha != want {
			t.Errorf("shadow layer %d has alpha %v, want %v", index, shadow.Color.Alpha, want)
		}
	}
	// The button is a component painted with the theme the element hands it.
	var buttonFill ui.Color
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Bounds.X == button.Rect().X && rectangle.Bounds.Y == button.Rect().Y {
			buttonFill = rectangle.Color
		}
	}
	if buttonFill.Alpha == 0 || buttonFill.Alpha > ui.LightTheme().Primary.Alpha*0.25+1e-4 {
		t.Errorf("the button's fill has alpha %v, want at most a quarter of %v", buttonFill.Alpha, ui.LightTheme().Primary.Alpha)
	}
}

func TestAnElementAtNoOpacityIsStillLaidOutAndHit(t *testing.T) {
	var view View
	var plain, hidden Box
	draw := func(opacity float32) (*ui.Canvas, Box) {
		canvas := testCanvas()
		var row Box
		view.Draw(canvas, ui.LightTheme(), ui.Rect{Width: 300, Height: 100}, func(f *Frame) {
			f.Row().Gap(8).Children(func() {
				row = f.Row().Size(80, 30).Opacity(opacity).Hit(styleRow, "row").Background(colorRest)
				f.Slot().Size(10, 10)
			})
		})
		return canvas, row
	}
	var visible, invisible *ui.Canvas
	visible, plain = draw(1)
	invisible, hidden = draw(0)
	if plain.Rect() != hidden.Rect() {
		t.Errorf("fading moved the element from %+v to %+v", plain.Rect(), hidden.Rect())
	}
	if len(invisible.Hits()) != len(visible.Hits()) || len(invisible.Hits()) == 0 {
		t.Errorf("%d hit regions when faded, %d when not", len(invisible.Hits()), len(visible.Hits()))
	}
	for _, rectangle := range invisible.Rectangles() {
		if rectangle.Bounds == hidden.Rect() && rectangle.Color.Alpha != 0 {
			t.Error("an element at no opacity was painted")
		}
	}
}

func TestRingIsOutsideTheElementAndTakesNoRoom(t *testing.T) {
	const width = 3
	var view View
	var with, without Box
	canvas := testCanvas()
	view.Draw(canvas, ui.LightTheme(), ui.Rect{Width: 300, Height: 100}, func(f *Frame) {
		f.Row().Gap(10).Pad(20).Children(func() {
			with = f.Row().Size(80, 30).Radius(6).Background(colorRest).Ring(width, colorRing)
			without = f.Row().Size(80, 30)
		})
	})
	if gap := without.Rect().X - (with.Rect().X + with.Rect().Width); gap != 10 {
		t.Errorf("the ring took room: the next element is %v away, want the gap of 10", gap)
	}
	count, left, top := ringTriangles(canvas)
	if count == 0 {
		t.Fatal("no ring was painted")
	}
	if left != with.Rect().X-width || top != with.Rect().Y-width {
		t.Errorf("the ring starts at (%v, %v), want %v and %v: its width outside the element", left, top, with.Rect().X-width, with.Rect().Y-width)
	}
}

func TestGradientFillsTheElementUnderItsBorderAndFades(t *testing.T) {
	const border = 2
	var view View
	var box Box
	paint := func(opacity float32) *ui.Canvas {
		canvas := testCanvas()
		view.Draw(canvas, ui.LightTheme(), ui.Rect{Width: 300, Height: 100}, func(f *Frame) {
			f.Row().Pad(10).Children(func() {
				box = f.Row().Size(120, 40).Radius(8).Border(border, colorRest).Opacity(opacity).Gradient(ToRight, colorHover, colorPressed)
			})
		})
		return canvas
	}
	canvas := paint(1)
	if len(canvas.Triangles()) == 0 {
		t.Fatal("no gradient was painted")
	}
	inner := box.Rect()
	left, right := float32(1e9), float32(-1e9)
	for _, triangle := range canvas.Triangles() {
		for _, point := range triangle.Points {
			left, right = min(left, point.X), max(right, point.X)
		}
	}
	if left != inner.X || right != inner.X+inner.Width {
		t.Errorf("the gradient spans %v to %v, want %v to %v: the whole element", left, right, inner.X, inner.X+inner.Width)
	}
	for _, triangle := range paint(0.5).Triangles() {
		if triangle.Color.Alpha > 0.5+1e-4 {
			t.Fatalf("a faded gradient has a vertex of alpha %v", triangle.Color.Alpha)
		}
	}
}

// styled declares a page with every look, ring, gradient and fade, to count what
// it allocates.
func styled(view *View, canvas *ui.Canvas, theme ui.Theme) {
	view.Draw(canvas, theme, ui.Rect{Width: 800, Height: 600}, func(f *Frame) {
		f.Column().Pad(24).Gap(12).Children(func() {
			for range 12 {
				row := f.Row().Gap(8).Pad(6).Radius(8).Background(theme.Card).Shadow(ui.ShadowSM).Opacity(0.9).
					Hit(styleRow, "row").Transition().
					Hover(Look{}.Background(theme.Hover).Shadow(ui.ShadowMD)).
					Pressed(Look{}.Opacity(0.7)).
					Focus(Look{}.Ring(3, theme.Ring))
				row.Children(func() {
					f.Row().Size(24, 24).Radius(12).Gradient(ToBottomRight, theme.Primary, theme.Accent)
					f.Text("a row of the list").Grow(1)
					f.Icon(ui.Icon{ViewBox: 24}).Size(16, 16).Opacity(0).StateFrom(row).Hover(Look{}.Opacity(1))
				})
			}
		})
	})
}

func TestStyledPageAllocatesNothingOnceWarm(t *testing.T) {
	var view View
	canvas := styleCanvas(styleRow, ui.ID{}, styleRow, true)
	theme := ui.LightTheme()
	run := func() { canvas.Reset(); styled(&view, canvas, theme) }
	run()
	if allocs := testing.AllocsPerRun(200, run); allocs != 0 {
		t.Fatalf("a warm styled page allocates %v times a frame, want 0", allocs)
	}
}

func BenchmarkStyledPage(b *testing.B) {
	var view View
	canvas := styleCanvas(styleRow, ui.ID{}, styleRow, true)
	theme := ui.LightTheme()
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		styled(&view, canvas, theme)
	}
}

// A faded border around a faded fill is a line over the fill: the color of the
// border is not a solid shape under it that the fill lets show through.
func TestAFadedBorderIsOnlyALineOverItsFill(t *testing.T) {
	canvas := testCanvas()
	var view View
	var box Box
	view.Draw(canvas, ui.LightTheme(), ui.Rect{Width: 100, Height: 100}, func(f *Frame) {
		box = f.Row().Size(60, 40).Radius(8).Background(colorHover).Border(2, colorRest).Opacity(0.5)
	})
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Bounds == box.Rect() && rectangle.Color.Red == colorRest.Red {
			t.Fatal("the border color was painted as a solid shape over the whole element")
		}
	}
	if len(canvas.Triangles()) == 0 {
		t.Fatal("the border was not painted at all")
	}
}
