package indicator_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/indicator"
)

func texts(t *testing.T, spec indicator.Spec) (values map[string]bool, button gfx.Rect) {
	t.Helper()
	view := func(c ui.C) {
		ui.Column(c).Fill().Pad(16).Children(func() {
			indicator.View(c, "Inbox", spec, func() { ui.Button(c, "Inbox") })
		})
	}
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.BeginFrame()
	canvas.Reset()
	page.Draw(canvas, gfx.Rect{Width: 320, Height: 200}, view)
	canvas.EndFrame()
	values = map[string]bool{}
	for _, x := range canvas.Texts() {
		values[x.Value] = true
	}
	for _, h := range canvas.Hits() {
		if h.Label == "Inbox" && !h.IsLabel {
			button = h.Bounds
		}
	}
	return values, button
}

func TestCountShowsAsNumberOrCapped(t *testing.T) {
	for _, c := range []struct {
		spec indicator.Spec
		want string
	}{
		{indicator.Spec{Count: 7}, "7"},
		{indicator.Spec{Count: 120}, "99+"},
		{indicator.Spec{Count: 12, Max: 9}, "9+"},
		{indicator.Spec{Count: 0, ShowZero: true}, "0"},
		{indicator.Spec{Count: 1234, Max: 1000}, "1000+"},
	} {
		got, _ := texts(t, c.spec)
		if !got[c.want] {
			t.Errorf("%+v: %q is not shown (have %v)", c.spec, c.want, got)
		}
	}
}

func TestNothingShownForZeroAndDotShowsNoNumber(t *testing.T) {
	got, _ := texts(t, indicator.Spec{Count: 0})
	if len(got) != 1 {
		t.Errorf("a zero count shows %v", got)
	}
	got, _ = texts(t, indicator.Spec{Count: 5, Dot: true})
	if got["5"] {
		t.Error("a dot shows the number")
	}
}

func TestTheMarkTakesNoRoom(t *testing.T) {
	_, plain := texts(t, indicator.Spec{})
	_, marked := texts(t, indicator.Spec{Count: 5})
	if plain != marked {
		t.Fatalf("the control moved or resized: %+v then %+v", plain, marked)
	}
}

func TestWarmPaintDoesNotAllocate(t *testing.T) {
	view := func(c ui.C) {
		ui.Column(c).Fill().Pad(16).Children(func() {
			indicator.View(c, "Inbox", indicator.Spec{Count: 12}, func() { ui.Button(c, "Inbox") })
		})
	}
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	draw := func() {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 320, Height: 200}, view)
		canvas.EndFrame()
	}
	draw()
	draw()
	if n := testing.AllocsPerRun(100, draw); n != 0 {
		t.Fatalf("a warm paint allocates %v times", n)
	}
}
