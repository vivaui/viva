package base_test

import (
	"testing"

	. "github.com/vivaui/viva/base"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
)

func TestButtonChildrenInheritTypographyWithoutLosingIdentity(t *testing.T) {
	plain := probeButton(t, nil, func(f *Frame) Box { return f.Button(save, "Label").FontSize(24).Weight(gfx.WeightBold) })
	children := probeButton(t, nil, func(f *Frame) Box {
		return f.Button(save, "Label").FontSize(24).Weight(gfx.WeightBold).Children(func() { f.Text("Label") })
	})
	if plain.box != children.box || plain.label.Style.Size != children.label.Style.Size || plain.label.Style.Weight != children.label.Style.Weight {
		t.Fatalf("default and Children disagree: plain %v %+v, children %v %+v", plain.box, plain.label.Style, children.box, children.label.Style)
	}
	own := probeButton(t, nil, func(f *Frame) Box {
		return f.Button(save, "Label").FontSize(24).Weight(gfx.WeightBold).Children(func() { f.Text("Label").FontSize(12).Weight(gfx.WeightRegular) })
	})
	if own.label.Style.Size != 12 || own.label.Style.Weight != gfx.WeightRegular {
		t.Fatalf("explicit child typography ignored: %+v", own.label.Style)
	}
}

func TestButtonRecipeDoesNotDependOnChildrenOrder(t *testing.T) {
	green := gfx.Color{Green: 1, Alpha: 1}
	recipe := func(theme gfx.Theme, v style.Variant, s gfx.State) style.ButtonLook {
		look := components.ButtonRecipe(theme, v, s)
		look.Background = green
		return look
	}
	a := probeButton(t, nil, func(f *Frame) Box {
		return f.Button(save, "Label").ButtonRecipe(recipe).Children(func() { f.Text("Label") })
	})
	b := probeButton(t, nil, func(f *Frame) Box {
		return f.Button(save, "Label").Children(func() { f.Text("Label") }).ButtonRecipe(recipe)
	})
	if a.canvas.Rectangles()[0].Color != green || b.canvas.Rectangles()[0].Color != green {
		t.Fatal("Children changed the button's recipe eligibility")
	}
}

func TestButtonCanExplicitlyUseRegularWeight(t *testing.T) {
	p := probeButton(t, nil, func(f *Frame) Box { return f.Button(save, "Label").Weight(gfx.WeightRegular) })
	if p.label.Style.Weight != gfx.WeightRegular {
		t.Fatalf("regular weight is a choice, got %v", p.label.Style.Weight)
	}
}

func TestButtonHitOptionsSurviveDefaultAndCustomContent(t *testing.T) {
	for _, children := range []bool{false, true} {
		p := probeButton(t, nil, func(f *Frame) Box {
			b := f.Button(save, "Label").HitWith(save, "Label", gfx.HitOptions{Role: gfx.RoleButton, Selected: true, Hint: "details"})
			if children {
				b = b.Children(func() { f.Text("Label") })
			}
			return b
		})
		hits := p.canvas.Hits()
		if len(hits) != 1 || !hits[0].Selected || hits[0].Hint != "details" {
			t.Fatalf("children=%v: %+v", children, hits)
		}
	}
}

func TestButtonClipsOverflowInsideItsContentArea(t *testing.T) {
	for _, children := range []bool{false, true} {
		p := probeButton(t, nil, func(f *Frame) Box {
			b := f.Button(save, "Long label").Width(20).PadX(4)
			if children {
				b = b.Children(func() { f.Text("Long label") })
			}
			return b
		})
		for _, text := range p.canvas.Texts() {
			if !text.HasClip || text.ClipOf.X <= p.box.X || text.ClipOf.Right() >= p.box.Right() {
				t.Fatalf("children=%v: overflow not clipped inside padding: %+v", children, text)
			}
		}
	}
}

func TestButtonNarrowerThanPaddingRetainsLeadingIcon(t *testing.T) {
	icon := gfx.Icon{ViewBox: 24, Stroke: 2, Commands: []gfx.PathCommand{gfx.MoveTo(4, 4), gfx.LineTo(20, 20)}}
	p := probeButton(t, nil, func(f *Frame) Box {
		return f.Button(save, "Edit").Icon(icon).Width(12).PadX(10)
	})
	if len(p.canvas.Triangles()) == 0 {
		t.Fatal("the icon disappeared when the content area became narrower than the padding")
	}
	for _, triangle := range p.canvas.Triangles() {
		for _, point := range triangle.Points {
			if point.X < p.box.X-0.001 || point.X > p.box.Right()+0.001 {
				t.Fatalf("icon escaped its button: %v outside %v", point, p.box)
			}
		}
	}
}

func TestDisabledButtonDoesNotExposeActiveDescendants(t *testing.T) {
	p := probeButton(t, nil, func(f *Frame) Box {
		return f.Button(save, "Outer").Disabled(true).Children(func() { f.Button(ctlFirst, "Inner") })
	})
	for _, hit := range p.canvas.Hits() {
		if !hit.Disabled || hit.Focusable {
			t.Fatalf("active descendant in disabled button: %+v", hit)
		}
	}
	var view View
	canvas := testCanvas()
	canvas.BeginEventPass(ctlFirst, gfx.ClickInfo{})
	defer canvas.EndEventPass()
	clicked := false
	view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 400, Height: 160}, func(f *Frame) {
		f.Button(save, "Outer").Disabled(true).Children(func() { clicked = f.Button(ctlFirst, "Inner").Clicked() })
	})
	if clicked {
		t.Fatal("a queued child click escaped the disabled ancestor")
	}
}
