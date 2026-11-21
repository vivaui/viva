package base_test

import (
	"testing"

	. "github.com/vivaui/viva/base"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

type selectProbe struct {
	box   Rect
	rects []gfx.Rectangle
	texts []gfx.Text
	hits  []gfx.Hit
}

func probeSelect(t *testing.T, interaction func(*gfx.Canvas), declare func(f *Frame) Box) selectProbe {
	t.Helper()
	canvas := testCanvas()
	if interaction != nil {
		interaction(canvas)
	}
	var view View
	var box Box
	view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 400, Height: 300}, func(f *Frame) {
		f.Column().Pad(10).Children(func() { box = declare(f) })
	})
	return selectProbe{box: box.Rect(), rects: canvas.Rectangles(), texts: canvas.Texts(), hits: canvas.Hits()}
}

func selectValue(t *testing.T, p selectProbe) gfx.Text {
	t.Helper()
	if len(p.texts) == 0 {
		t.Fatalf("no value painted")
	}
	return p.texts[0]
}

// A Select trigger with no calls of its own is the theme's text-entry surface:
// the chosen value, the arrow, and one combobox hit named by the trigger.
func TestSelectDefaultsAreTheRecipe(t *testing.T) {
	theme := gfx.LightTheme()
	look := components.SelectRecipe(theme, gfx.State{}, false)
	var state SelectState
	p := probeSelect(t, nil, func(f *Frame) Box {
		return f.Select(selScope, &state, selLabels, 1).Label("Size").Placeholder("Size").Width(200)
	})
	if p.box.Width != 200 {
		t.Errorf("trigger width = %v, want 200", p.box.Width)
	}
	if len(p.hits) != 1 || p.hits[0].Role != gfx.RoleCombobox || p.hits[0].Label != "Size" || p.hits[0].Value != selLabels[1] {
		t.Fatalf("hit = %+v, want one combobox named Size with the chosen value", p.hits)
	}
	value := selectValue(t, p)
	if value.Value != selLabels[1] || value.Style.Size != theme.Metrics.FontSizeM {
		t.Errorf("value = %+v, want %q at the body size", value, selLabels[1])
	}
	if value.Style.Color != look.Foreground {
		t.Errorf("value color = %v, want %v", value.Style.Color, look.Foreground)
	}
	found := false
	for _, rect := range p.rects {
		if rect.Bounds == p.box {
			found = true
			if rect.Radius != look.Surface.Radius {
				t.Errorf("trigger radius = %v, want %v", rect.Radius, look.Surface.Radius)
			}
		}
	}
	if !found {
		t.Errorf("no trigger surface for %v: %+v", p.box, p.rects)
	}
}

// The calls every element takes reach the trigger: FontSize and Weight the
// value, Color its ink, Pad the content, Height the trigger, Radius its corners,
// Background its fill.
func TestSelectPropertiesReachTheTrigger(t *testing.T) {
	ink := gfx.Color{Red: 1, Alpha: 1}
	fill := gfx.Color{Blue: 1, Alpha: 1}
	var state SelectState
	p := probeSelect(t, nil, func(f *Frame) Box {
		return f.Select(selScope, &state, selLabels, 0).Label("Size").Width(200).Height(48).
			FontSize(18).Weight(gfx.WeightBold).Color(ink).Radius(5).Background(fill)
	})
	if p.box.Height != 48 {
		t.Errorf("trigger height = %v, want 48", p.box.Height)
	}
	value := selectValue(t, p)
	if value.Style.Size != 18 || value.Style.Weight != gfx.WeightBold || value.Style.Color != ink {
		t.Errorf("value style = %+v, want 18, bold, red", value.Style)
	}
	found, radius := false, float32(0)
	for _, rect := range p.rects {
		if rect.Color == fill {
			found = true
		}
		if rect.Bounds == p.box && rect.Radius > radius {
			radius = rect.Radius
		}
	}
	if found && radius != 5 {
		t.Errorf("trigger radius = %v, want 5", radius)
	}
	if !found {
		t.Errorf("no trigger fill %v for %v: %+v", fill, p.box, p.rects)
	}
}

// A focused or invalid trigger draws the recipe's ring and line, and hover
// lightens the fill, exactly as the component does.
func TestSelectStatesFollowTheRecipe(t *testing.T) {
	theme := gfx.LightTheme()
	var state SelectState
	trigger := selScope.At(len(selLabels))
	focused := probeSelect(t, func(c *gfx.Canvas) { c.SetInteraction(gfx.ID{}, gfx.ID{}, trigger, true) },
		func(f *Frame) Box { return f.Select(selScope, &state, selLabels, 0).Label("Size").Width(200) })
	focusLook := components.SelectRecipe(theme, gfx.State{Focused: true}, false)
	ring := false
	for _, rect := range focused.rects {
		ring = ring || rect.Color == focusLook.Surface.Border
	}
	if !ring {
		t.Errorf("focused select paints no focus line %v: %+v", focusLook.Surface.Border, focused.rects)
	}
	invalid := probeSelect(t, nil, func(f *Frame) Box {
		return f.Select(selScope, &state, selLabels, 0).Label("Size").Width(200).Invalid(true)
	})
	invalidLook := components.SelectRecipe(theme, gfx.State{Invalid: true}, false)
	danger := false
	for _, rect := range invalid.rects {
		danger = danger || rect.Color == invalidLook.Surface.Border
	}
	if !danger {
		t.Errorf("invalid select paints no dangerous line: %+v", invalid.rects)
	}
	if len(invalid.hits) != 1 || !invalid.hits[0].Invalid {
		t.Errorf("invalid select hit = %+v, want Invalid", invalid.hits)
	}
}

// Children replace the value and icon while the arrow and the trigger's hit
// stay the control's; a disabled select's content is inert.
func TestSelectChildrenReplaceTheValue(t *testing.T) {
	var state SelectState
	p := probeSelect(t, nil, func(f *Frame) Box {
		return f.Select(selScope, &state, selLabels, 0).Label("Size").Width(200).Children(func() {
			f.Text("own")
		})
	})
	for _, text := range p.texts {
		if text.Value == selLabels[0] {
			t.Errorf("the value was painted over the children: %+v", p.texts)
		}
	}
	if len(p.hits) != 1 || p.hits[0].Role != gfx.RoleCombobox {
		t.Errorf("hits = %+v, want the trigger's one combobox", p.hits)
	}

	var disabled SelectState
	d := probeSelect(t, nil, func(f *Frame) Box {
		return f.Select(selScope, &disabled, selLabels, 0).Label("Size").Width(200).Disabled(true).Children(func() {
			f.Button(gfx.Item(9), "inner")
		})
	})
	for _, hit := range d.hits {
		if !hit.Disabled || hit.Focusable {
			t.Errorf("hit %+v is not inert under the disabled select", hit)
		}
	}
}
