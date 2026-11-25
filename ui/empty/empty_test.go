package empty_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/empty"
)

var icon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{gfx.MoveTo(4, 20), gfx.LineTo(20, 4)}}

func measure(value string, size, weight float32) float32 { return float32(len(value)) * size * 0.55 }

func draw(page *ui.Page, canvas *gfx.Canvas, view func(ui.C)) {
	canvas.BeginFrame()
	canvas.Reset()
	page.Draw(canvas, gfx.Rect{Width: 600, Height: 400}, view)
	canvas.EndFrame()
}

func open(t testing.TB, view func(ui.C)) (*ui.Page, *gfx.Canvas) {
	t.Helper()
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetMeasurer(measure)
	canvas.SetReduceMotion(true)
	draw(page, canvas, view)
	return page, canvas
}

func textAt(canvas *gfx.Canvas, value string) (gfx.Text, bool) {
	for _, text := range canvas.Texts() {
		if text.Value == value {
			return text, true
		}
	}
	return gfx.Text{}, false
}

// The words are centered in the room given, with the actions under them.
func TestStateIsCenteredWithActionsBelow(t *testing.T) {
	var c0 ui.C
	actions := func() { ui.Button(c0, "Create project") }
	view := func(c ui.C) {
		c0 = c
		empty.View(c, empty.Spec{Icon: icon, Title: "No projects yet", Description: "Create one."}, actions).Fill()
	}
	_, canvas := open(t, view)
	title, ok := textAt(canvas, "No projects yet")
	if !ok {
		t.Fatal("the empty state did not paint its title")
	}
	middle := title.Bounds.X + title.Bounds.Width/2
	if middle < 295 || middle > 305 {
		t.Fatalf("the title is centered at %v in a 600 wide window, want about 300", middle)
	}
	description, _ := textAt(canvas, "Create one.")
	button, ok := textAt(canvas, "Create project")
	if !ok || !(title.Bounds.Y < description.Bounds.Y && description.Bounds.Y < button.Bounds.Y) {
		t.Fatalf("title %v, description %v, action %v are not in order", title.Bounds.Y, description.Bounds.Y, button.Bounds.Y)
	}
}

// Boxed puts a muted square behind the icon; bare does not.
func TestBoxedIconHasAMutedSquare(t *testing.T) {
	theme := gfx.LightTheme()
	squares := func(boxed bool) int {
		view := func(c ui.C) { empty.View(c, empty.Spec{Icon: icon, Boxed: boxed, Title: "None"}, nil).Fill() }
		_, canvas := open(t, view)
		n := 0
		for _, rectangle := range canvas.Rectangles() {
			if rectangle.Color == theme.Muted && rectangle.Bounds.Width == rectangle.Bounds.Height {
				n++
			}
		}
		return n
	}
	if squares(true) != 1 || squares(false) != 0 {
		t.Fatalf("muted squares: boxed %d, bare %d; want 1 and 0", squares(true), squares(false))
	}
}

func TestWarmPaintAllocatesNothing(t *testing.T) {
	var c0 ui.C
	actions := func() { ui.Button(c0, "Create") }
	view := func(c ui.C) {
		c0 = c
		empty.View(c, empty.Spec{Icon: icon, Boxed: true, Title: "None", Description: "Create one."}, actions).Fill()
	}
	page, canvas := open(t, view)
	if allocations := testing.AllocsPerRun(100, func() { draw(page, canvas, view) }); allocations != 0 {
		t.Fatalf("warm paint allocated %v times, want 0", allocations)
	}
}

func BenchmarkViewPaint(b *testing.B) {
	view := func(c ui.C) {
		empty.View(c, empty.Spec{Icon: icon, Title: "None", Description: "Create one."}, nil).Fill()
	}
	page, canvas := open(b, view)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		draw(page, canvas, view)
	}
}
