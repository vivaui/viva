package base_test

import (
	"math"
	"testing"

	. "github.com/vivaui/viva/base"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
)

type tabsProbe struct {
	box   Rect
	rects []gfx.Rectangle
	texts []gfx.Text
	hits  []gfx.Hit
}

func probeTabs(t *testing.T, declare func(f *Frame) Box) tabsProbe {
	t.Helper()
	canvas := testCanvas()
	var view View
	var box Box
	view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 400, Height: 160}, func(f *Frame) {
		f.Column().Children(func() { box = declare(f) })
	})
	return tabsProbe{box: box.Rect(), rects: canvas.Rectangles(), texts: canvas.Texts(), hits: canvas.Hits()}
}

var tabsNodeLabels = tabLabels

// A Tabs strip with no calls of its own is the theme's: the muted strip behind
// the triggers, one tab role per label, the selected one filled.
func TestTabsDefaultsAreTheRecipe(t *testing.T) {
	theme := gfx.LightTheme()
	p := probeTabs(t, func(f *Frame) Box { return f.Tabs(tabsScope, tabsNodeLabels, 1) })
	if len(p.hits) != len(tabsNodeLabels) {
		t.Fatalf("hits = %d, want one tab per label", len(p.hits))
	}
	for index, hit := range p.hits {
		if hit.Role != gfx.RoleTab || hit.Label != tabsNodeLabels[index] {
			t.Errorf("hit %d = %+v, want a tab named %q", index, hit, tabsNodeLabels[index])
		}
		if hit.Selected != (index == 1) {
			t.Errorf("hit %d selected=%v, want %v", index, hit.Selected, index == 1)
		}
	}
	list := gfx.Rectangle{}
	found := false
	for _, rect := range p.rects {
		if rect.Bounds == p.box {
			list, found = rect, true
		}
	}
	if !found {
		t.Fatalf("no strip rectangle for %v: %+v", p.box, p.rects)
	}
	look := components.TabsListRecipe(theme, components.TabsDefault)
	if list.Color != look.Background || list.Radius != look.Radius {
		t.Errorf("strip = %v radius %v, want %v radius %v", list.Color, list.Radius, look.Background, look.Radius)
	}
}

// The calls every element takes reach the strip and its triggers: Pad insets the
// triggers, Gap separates line triggers, FontSize and Weight style the labels,
// Color their ink, Radius the strip, Background its fill, Height its height.
func TestTabsPropertiesReachTheStrip(t *testing.T) {
	theme := gfx.LightTheme()
	ink := gfx.Color{Red: 1, Alpha: 1}
	fill := gfx.Color{Blue: 1, Alpha: 1}
	_, defPadding, _, _, _, _, _ := style.TabsGeometry(theme.Metrics)
	p := probeTabs(t, func(f *Frame) Box {
		return f.Tabs(tabsScope, tabsNodeLabels, 0).FontSize(18).Weight(gfx.WeightBold).Color(ink).
			Pad(7).Radius(4).Background(fill).Height(44)
	})
	if p.box.Height != 44 {
		t.Errorf("strip height = %v, want 44", p.box.Height)
	}
	if len(p.texts) != len(tabsNodeLabels) {
		t.Fatalf("texts = %d, want one per label", len(p.texts))
	}
	dimmed := ink
	dimmed.Alpha *= 0.60
	for index, text := range p.texts {
		if text.Style.Size != 18 || text.Style.Weight != gfx.WeightBold {
			t.Errorf("label %d style = %+v, want 18, bold", index, text.Style)
		}
		want := dimmed
		if index == 0 {
			want = ink // the selected tab keeps the ink whole
		}
		if text.Style.Color != want {
			t.Errorf("label %d color = %v, want %v", index, text.Style.Color, want)
		}
	}
	list := gfx.Rectangle{}
	for _, rect := range p.rects {
		if rect.Bounds == p.box {
			list = rect
		}
	}
	if list.Color != fill || list.Radius != 4 {
		t.Errorf("strip = %v radius %v, want %v radius 4", list.Color, list.Radius, fill)
	}
	// Pad 7 puts the first trigger's left edge seven pixels inside the strip.
	first := p.hits[0].Bounds
	if first.X != p.box.X+7 {
		t.Errorf("first trigger x = %v, want the strip's x+7 = %v (default padding %v)", first.X, p.box.X+7, defPadding)
	}

	line := probeTabs(t, func(f *Frame) Box {
		return f.Tabs(tabsScope, tabsNodeLabels, 0).Line().Gap(10)
	})
	if len(line.hits) < 2 {
		t.Fatalf("line strip hits = %d", len(line.hits))
	}
	if gap := line.hits[1].Bounds.X - (line.hits[0].Bounds.X + line.hits[0].Bounds.Width); gap < 9.99 || gap > 10.01 {
		t.Errorf("line trigger gap = %v, want the explicit 10", gap)
	}
}

// Pad reaches the strip on every side: it insets the triggers and the strip's
// thickness, rather than only the top.
func TestTabsPadOnEverySide(t *testing.T) {
	horizontal := probeTabs(t, func(f *Frame) Box {
		return f.Tabs(tabsScope, tabsNodeLabels, 0).PadSides(2, 8, 4, 12).Height(44)
	})
	first, last := horizontal.hits[0].Bounds, horizontal.hits[len(horizontal.hits)-1].Bounds
	if math.Abs(float64(first.X-(horizontal.box.X+12))) > 0.01 || math.Abs(float64(first.Y-(horizontal.box.Y+2))) > 0.01 {
		t.Errorf("first trigger at %v, want the strip's left 12 and top 2", first)
	}
	if math.Abs(float64(first.Height-(44-2-4))) > 0.01 {
		t.Errorf("trigger height = %v, want the strip height less top and bottom", first.Height)
	}
	if math.Abs(float64(last.X+last.Width-(horizontal.box.X+horizontal.box.Width-8))) > 0.01 {
		t.Errorf("last trigger ends at %v, want 8 from the right edge %v", last.X+last.Width, horizontal.box.X+horizontal.box.Width)
	}

	vertical := probeTabs(t, func(f *Frame) Box {
		return f.Tabs(tabsScope, tabsNodeLabels, 0).Vertical().PadSides(2, 8, 4, 12).Height(140)
	})
	first, last = vertical.hits[0].Bounds, vertical.hits[len(vertical.hits)-1].Bounds
	if math.Abs(float64(first.X-(vertical.box.X+12))) > 0.01 || math.Abs(float64(first.Width-(vertical.box.Width-12-8))) > 0.01 {
		t.Errorf("vertical trigger at %v width %v, want 12 in and the width less both sides", first, first.Width)
	}
	if math.Abs(float64(first.Y-(vertical.box.Y+2))) > 0.01 || math.Abs(float64(last.Y+last.Height-(vertical.box.Y+vertical.box.Height-4))) > 0.01 {
		t.Errorf("vertical triggers from %v to %v, want 2 from top and 4 from bottom", first.Y, last.Y+last.Height)
	}
}

// Without an explicit height a strip is exactly its triggers and its Pad: PadY(0)
// no longer falls back to the theme's 36.
func TestTabsPadYShrinksTheAutomaticHeight(t *testing.T) {
	theme := gfx.LightTheme()
	_, _, triggerHeight, _, _, _, _ := style.TabsGeometry(theme.Metrics)
	for _, tc := range []struct {
		name    string
		declare func(f *Frame) Box
		want    float32
	}{
		{"padY 0", func(f *Frame) Box { return f.Tabs(tabsScope, tabsNodeLabels, 0).PadY(0) }, triggerHeight},
		{"padY 2", func(f *Frame) Box { return f.Tabs(tabsScope, tabsNodeLabels, 0).PadY(2) }, triggerHeight + 4},
		{"line padY 2", func(f *Frame) Box { return f.Tabs(tabsScope, tabsNodeLabels, 0).Line().PadY(2) }, triggerHeight + 4 + TabIndicator},
	} {
		p := probeTabs(t, tc.declare)
		if p.box.Height != tc.want {
			t.Errorf("%s: strip height = %v, want %v", tc.name, p.box.Height, tc.want)
		}
	}
	_ = theme
}
