package base_test

import (
	"reflect"
	"testing"

	. "github.com/vivaui/viva/base"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
)

type badgeProbe struct {
	box   Rect
	rects []gfx.Rectangle
	texts []gfx.Text
	hits  []gfx.Hit
}

func probeBadge(t *testing.T, state func(*gfx.Canvas), declare func(f *Frame) Box) badgeProbe {
	t.Helper()
	canvas := testCanvas()
	if state != nil {
		state(canvas)
	}
	var view View
	var box Box
	view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 400, Height: 120}, func(f *Frame) {
		f.Row().Items(Start).Children(func() { box = declare(f) })
	})
	return badgeProbe{box: box.Rect(), rects: canvas.Rectangles(), texts: canvas.Texts(), hits: canvas.Hits()}
}

// fontOf returns the style of the first painted text, which a badge's default
// content is.
func fontOf(t *testing.T, p badgeProbe) gfx.TextStyle {
	t.Helper()
	if len(p.texts) != 1 {
		t.Fatalf("texts = %+v, want the one label", p.texts)
	}
	return p.texts[0].Style
}

// A badge with no calls of its own is the chip the theme's recipe gives it: a
// pill the height of a badge, filled with the primary color, its label painted
// with the small font and the primary foreground.
func TestBadgeDefaultsAreTheRecipeChip(t *testing.T) {
	theme := gfx.LightTheme()
	look := components.BadgeRecipe(theme, components.BadgeDefault, gfx.State{})
	height, _, radius, fontSize := style.BadgeGeometry(theme.Metrics)
	p := probeBadge(t, nil, func(f *Frame) Box { return f.Badge("beta") })
	if len(p.rects) != 1 {
		t.Fatalf("rectangles = %+v, want the chip", p.rects)
	}
	if p.box.Height != height || p.rects[0].Bounds.Height != height {
		t.Errorf("chip height = %v/%v, want %v", p.box.Height, p.rects[0].Bounds.Height, height)
	}
	if p.rects[0].Radius != radius {
		t.Errorf("chip radius = %v, want the pill's %v", p.rects[0].Radius, radius)
	}
	if p.rects[0].Color != look.Background {
		t.Errorf("chip fill = %v, want %v", p.rects[0].Color, look.Background)
	}
	text := fontOf(t, p)
	if text.Size != fontSize || text.Weight != gfx.WeightMedium || text.Color != look.Foreground {
		t.Errorf("label style = %+v, want size %v, medium, %v", text, fontSize, look.Foreground)
	}
	if len(p.hits) != 0 {
		t.Errorf("a presentational badge declared hits: %+v", p.hits)
	}
}

// The calls every element takes reach the chip: FontSize and Weight the label,
// Color its ink, Pad the room inside the chip, Radius and Height the surface.
func TestBadgePropertiesReachTheChip(t *testing.T) {
	ink := gfx.Color{Red: 1, Alpha: 1}
	p := probeBadge(t, nil, func(f *Frame) Box {
		return f.Badge("beta").FontSize(20).Weight(gfx.WeightBold).Color(ink).Height(40).Radius(3)
	})
	if p.box.Height != 40 {
		t.Errorf("chip height = %v, want the explicit 40", p.box.Height)
	}
	text := fontOf(t, p)
	if text.Size != 20 || text.Weight != gfx.WeightBold || text.Color != ink {
		t.Errorf("label style = %+v, want size 20, bold, red", text)
	}
	if len(p.rects) == 0 || p.rects[0].Radius != 3 {
		t.Errorf("chip radius was not the explicit 3: %+v", p.rects)
	}

	_, padding, _, _ := style.BadgeGeometry(gfx.LightTheme().Metrics)
	plain := probeBadge(t, nil, func(f *Frame) Box { return f.Badge("beta") })
	padded := probeBadge(t, nil, func(f *Frame) Box { return f.Badge("beta").PadSides(1, 30, 1, 30) })
	if got, want := padded.box.Width, plain.box.Width-2*padding+60; got != want {
		t.Errorf("padded badge width = %v, want %v (pad replaces the recipe's)", got, want)
	}
}

// Explicit Height gives the chip a new height; the look keeps the recipe's
// radius unless the caller gives one, as a Button's corners do.
func TestBadgeExplicitHeightKeepsTheRecipeRadius(t *testing.T) {
	_, _, radius, _ := style.BadgeGeometry(gfx.LightTheme().Metrics)
	p := probeBadge(t, nil, func(f *Frame) Box { return f.Badge("beta").Height(48) })
	if p.box.Height != 48 {
		t.Fatalf("chip height = %v, want 48", p.box.Height)
	}
	if len(p.rects) == 0 || p.rects[0].Radius != radius {
		t.Errorf("chip radius = %+v, want the recipe's %v", p.rects, radius)
	}
}

// Children replace the label and are laid out inside the content rect; the
// badge is still one button to the pointer when it is given an identity.
func TestBadgeChildrenReplaceTheLabel(t *testing.T) {
	p := probeBadge(t, nil, func(f *Frame) Box {
		return f.Badge("beta").Hit(gfx.Item(7), "beta").Children(func() {
			f.Text("one")
			f.Text("two")
		})
	})
	if len(p.texts) != 2 {
		t.Fatalf("texts = %+v, want the two children", p.texts)
	}
	for _, text := range p.texts {
		if text.Value == "beta" {
			t.Errorf("the default label was painted over the children: %+v", p.texts)
		}
	}
	if len(p.hits) != 1 || p.hits[0].Role != gfx.RoleButton || p.hits[0].Label != "beta" || !p.hits[0].Focusable {
		t.Errorf("hits = %+v, want one focusable button named by the label", p.hits)
	}
	for _, text := range p.texts {
		if text.Bounds.X < p.box.X || text.Bounds.X+text.Bounds.Width > p.box.X+p.box.Width {
			t.Errorf("child %q at %v is outside the chip %v", text.Value, text.Bounds, p.box)
		}
	}
}

// A variant dresses the chip by the recipe: a secondary badge is the secondary
// colors, a ghost one paints no fill.
func TestBadgeVariantsFollowTheRecipe(t *testing.T) {
	theme := gfx.LightTheme()
	secondary := components.BadgeRecipe(theme, components.BadgeSecondary, gfx.State{})
	p := probeBadge(t, nil, func(f *Frame) Box { return f.Badge("beta").Secondary() })
	if len(p.rects) != 1 || p.rects[0].Color != secondary.Background {
		t.Errorf("secondary chip = %+v, want %v", p.rects, secondary.Background)
	}
	if text := fontOf(t, p); text.Color != secondary.Foreground {
		t.Errorf("secondary label = %v, want %v", text.Color, secondary.Foreground)
	}

	ghost := probeBadge(t, nil, func(f *Frame) Box { return f.Badge("beta").Ghost() })
	if len(ghost.rects) != 0 {
		t.Errorf("ghost badge painted a fill: %+v", ghost.rects)
	}
}

// A disabled badge's content is inert, as a disabled button's is: its nested
// controls take no focus and no click.
func TestDisabledBadgeContentsAreInert(t *testing.T) {
	ctl := gfx.Item(9)
	p := probeBadge(t, nil, func(f *Frame) Box {
		return f.Badge("outer").Disabled(true).Children(func() {
			f.Button(ctl, "inner")
		})
	})
	if len(p.hits) != 1 || !p.hits[0].Disabled || p.hits[0].Focusable {
		t.Errorf("hits = %+v, want the nested button inert under the disabled badge", p.hits)
	}
}

// A row that lines elements up by their baseline uses the badge's label size,
// so a badge and a text sit on the same row.
func TestBadgeBaselineMatchesItsText(t *testing.T) {
	var view View
	canvas := testCanvas()
	var sibling, badge Box
	view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 400, Height: 80}, func(f *Frame) {
		f.Row().Height(80).Items(Baseline).Children(func() {
			sibling = f.Text("Text").FontSize(20)
			badge = f.Badge("badge")
		})
	})
	_, _, _, fontSize := style.BadgeGeometry(gfx.LightTheme().Metrics)
	got := canvas.LabelBaseline(badge.Rect(), gfx.TextStyle{Size: fontSize})
	want := canvas.LabelBaseline(sibling.Rect(), gfx.TextStyle{Size: 20})
	if d := got - want; d > 0.01 || d < -0.01 {
		t.Fatalf("badge baseline %v, text baseline %v", got, want)
	}
}

// shapesOf normalizes painted shapes for comparison: two independent paints
// number their primitives differently.
func shapesOf(in []gfx.Rectangle) []gfx.Rectangle {
	out := append([]gfx.Rectangle(nil), in...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0; j-- {
			a, b := out[j-1], out[j]
			if a.Bounds.X < b.Bounds.X || a.Bounds.X == b.Bounds.X && (a.Bounds.Y < b.Bounds.Y || a.Bounds.Y == b.Bounds.Y && a.Radius < b.Radius) {
				break
			}
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

// A node badge paints what components.PaintBadge painted before it: for every
// variant the same chip shapes and the same label, at the same place.
func TestBadgeNodePaintsWhatTheComponentDid(t *testing.T) {
	theme := gfx.LightTheme()
	cases := []struct {
		name    string
		variant components.BadgeVariant
		declare func(f *Frame) Box
	}{
		{"default", components.BadgeDefault, func(f *Frame) Box { return f.Badge("beta") }},
		{"secondary", components.BadgeSecondary, func(f *Frame) Box { return f.Badge("beta").Secondary() }},
		{"outline", components.BadgeOutline, func(f *Frame) Box { return f.Badge("beta").Outline() }},
		{"ghost", components.BadgeGhost, func(f *Frame) Box { return f.Badge("beta").Ghost() }},
		{"link", components.BadgeLink, func(f *Frame) Box { return f.Badge("beta").Link() }},
		{"destructive", components.BadgeDestructive, func(f *Frame) Box { return f.Badge("beta").Destructive() }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			canvas := testCanvas()
			var view View
			var box Box
			view.Draw(canvas, theme, gfx.Rect{Width: 200, Height: 80}, func(f *Frame) {
				f.Row().Items(Start).Children(func() { box = tc.declare(f) })
			})
			wantCanvas := testCanvas()
			components.PaintBadge(wantCanvas, theme, gfx.Rect{X: box.Rect().X, Y: box.Rect().Y}, "beta", tc.variant)
			if !reflect.DeepEqual(shapesOf(wantCanvas.Rectangles()), shapesOf(canvas.Rectangles())) {
				t.Errorf("chip differs from the component:\nwant %+v\n got %+v", wantCanvas.Rectangles(), canvas.Rectangles())
			}
			wantTexts, gotTexts := wantCanvas.Texts(), canvas.Texts()
			if len(wantTexts) != len(gotTexts) {
				t.Fatalf("texts = %+v, want the component's %+v", gotTexts, wantTexts)
			}
			for i := range wantTexts {
				if wantTexts[i] != gotTexts[i] {
					t.Errorf("text %d differs: want %+v got %+v", i, wantTexts[i], gotTexts[i])
				}
			}
		})
	}
}
