package components

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

func TestColorSwatchSelectionKeyboardAndDisabled(t *testing.T) {
	var tree gfx.Tree
	feature := tree.Feature("colors")
	red, blue, green := tree.ID(feature, "red"), tree.ID(feature, "blue"), tree.ID(feature, "green")
	options := [...]RadioOption{{ID: red}, {ID: blue}, {ID: green, Disabled: true}}
	selected := red
	driver, err := app.NewDriver(app.Config{Width: 180, Height: 80, Tree: &tree, Paint: func(c *gfx.Canvas, size gfx.Size) {
		if BeginColorSwatch(c, gfx.LightTheme(), red, gfx.Rect{X: 10, Y: 10, Width: 32, Height: 32}, gfx.Color{Red: 1, Alpha: 1}).Label("Red").Selected(selected == red).TabStop(selected == red).Paint().Clicked() {
			selected = red
		}
		if BeginColorSwatch(c, gfx.LightTheme(), blue, gfx.Rect{X: 52, Y: 10, Width: 32, Height: 32}, gfx.Color{Blue: 1, Alpha: 1}).Label("Blue").Selected(selected == blue).TabStop(selected == blue).Paint().Clicked() {
			selected = blue
		}
		BeginColorSwatch(c, gfx.LightTheme(), green, gfx.Rect{X: 94, Y: 10, Width: 32, Height: 32}, gfx.Color{Green: 1, Alpha: 1}).Label("Green").Selected(selected == green).TabStop(selected == green).Disabled().Paint()
	}, OnKey: func(event gfx.KeyEvent) gfx.EventResult {
		next, handled := RadioGroupKeyDown(event.Target, options[:], DirectionLTR, event.Key)
		if !handled {
			return gfx.EventResult{}
		}
		selected = next
		return gfx.EventResult{Focus: next, FocusVisible: true, Handled: true, Repaint: true}
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Key(gfx.KeyTab, 0); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyRight, 0); err != nil {
		t.Fatal(err)
	}
	if selected != blue || driver.Snapshot().Focused != blue {
		t.Fatalf("right arrow selected %v with focus %v, want blue", selected, driver.Snapshot().Focused)
	}
	if err := driver.Key(gfx.KeyRight, 0); err != nil {
		t.Fatal(err)
	}
	if selected != red || driver.Snapshot().Focused != red {
		t.Fatalf("right arrow did not skip disabled green: selected %v, focus %v", selected, driver.Snapshot().Focused)
	}
	if err := driver.Click(68, 26); err != nil {
		t.Fatal(err)
	}
	if selected != blue {
		t.Fatal("pointer did not select blue")
	}
	tabStops := 0
	for _, h := range driver.Snapshot().Hits {
		if h.Hint != "" {
			t.Fatalf("%v hint = %q, want empty", h.ID, h.Hint)
		}
		if h.Focusable && !h.SkipTab {
			tabStops++
		}
		if h.ID == green && (!h.Disabled || h.Focusable) {
			t.Fatal("disabled swatch remains focusable")
		}
	}
	if tabStops != 1 {
		t.Fatalf("tab stops = %d, want 1", tabStops)
	}
}

func TestColorSwatchIgnoresBoundsTooSmallForItsDisc(t *testing.T) {
	var canvas gfx.Canvas
	BeginColorSwatch(&canvas, gfx.LightTheme(), gfx.ID{}, gfx.Rect{Width: 9, Height: 9}, gfx.Color{}).Paint()
	if len(canvas.Rectangles()) != 0 || len(canvas.Hits()) != 0 {
		t.Fatal("undersized swatch declared negative-size geometry")
	}
}

func BenchmarkColorSwatch(b *testing.B) {
	var canvas gfx.Canvas
	var tree gfx.Tree
	id := tree.ID(tree.Feature("colors"), "color")
	paint := func() {
		canvas.Reset()
		BeginColorSwatch(&canvas, gfx.LightTheme(), id, gfx.Rect{Width: 32, Height: 32}, gfx.Color{Red: 1, Alpha: 1}).Label("Red").Selected(true).Paint()
	}
	paint()
	b.ReportAllocs()
	for b.Loop() {
		paint()
	}
}

func TestColorSwatchFlushFillsItsBorder(t *testing.T) {
	var canvas gfx.Canvas
	color := gfx.Color{Red: 1, Alpha: 1}
	BeginColorSwatch(&canvas, gfx.LightTheme(), gfx.ID{}, gfx.Rect{Width: 20, Height: 20}, color).Flush().Paint()
	var disc gfx.Rect
	found := false
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Color == color {
			disc, found = rectangle.Bounds, true
		}
	}
	if !found || disc.X != 1 || disc.Y != 1 || disc.Width != 18 || disc.Height != 18 {
		t.Fatalf("flush disc = %+v found=%v, want a 1px border and an 18x18 disc", disc, found)
	}
}
