package base_test

import (
	"testing"

	. "github.com/vivaui/viva/base"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// A themed Button is an element like any other: Pad, Weight and Color on it are the
// padding, weight and color of its content, and its surface, its hit region and the room
// it takes in the layout are one rectangle.

type buttonProbe struct {
	box     Rect
	surface Rect // the first rectangle painted
	hit     Rect
	label   gfx.Text
	texts   []gfx.Text
	canvas  *gfx.Canvas
}

func probeButton(t *testing.T, state func(*gfx.Canvas), declare func(f *Frame) Box) buttonProbe {
	t.Helper()
	var view View
	canvas := testCanvas()
	if state != nil {
		state(canvas)
	}
	var box Box
	view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 600, Height: 200}, func(f *Frame) {
		f.Row().Items(Start).Children(func() { box = declare(f) })
	})
	p := buttonProbe{box: box.Rect(), texts: canvas.Texts(), canvas: canvas}
	if rects := canvas.Rectangles(); len(rects) > 0 {
		p.surface = rects[0].Bounds
	}
	if hits := canvas.Hits(); len(hits) > 0 {
		p.hit = hits[0].Bounds
	}
	if len(p.texts) > 0 {
		p.label = p.texts[0]
	}
	return p
}

// textMidX is where a label's glyphs are centered across: the middle of the width the
// canvas measures, from the origin it placed them at.
func textMidX(canvas *gfx.Canvas, text gfx.Text) float32 {
	return text.Bounds.X + canvas.TextWidthWeight(text.Value, text.Style.Size, text.Style.Weight)/2
}

func TestButtonPadIsThePaddingOfItsContent(t *testing.T) {
	plain := probeButton(t, nil, func(f *Frame) Box { return f.Button(save, "Save") })
	padded := probeButton(t, nil, func(f *Frame) Box { return f.Button(save, "Save").PadX(30) })
	// The padding either side of the label is the caller's instead of the size's.
	if got, want := padded.box.Width-plain.box.Width, float32(2*(30-10)); got != want {
		t.Errorf("PadX(30) widened the button by %v, want %v", got, want)
	}
	for name, p := range map[string]buttonProbe{"plain": plain, "padded": padded} {
		if p.surface != p.box || p.hit != p.box {
			t.Errorf("%s: layout %v, surface %v, hit %v: want one rectangle", name, p.box, p.surface, p.hit)
		}
		if mid, want := textMidX(p.canvas, p.label), p.box.X+p.box.Width/2; mid < want-0.01 || mid > want+0.01 {
			t.Errorf("%s: the label is centered on %v, the button on %v", name, mid, want)
		}
	}
}

// Padding one side leaves the others as the size gives them: the recipe pads what the
// caller did not.
func TestButtonPaddedOnOneSideKeepsTheOthers(t *testing.T) {
	plain := probeButton(t, nil, func(f *Frame) Box { return f.Button(save, "Save") })
	top := probeButton(t, nil, func(f *Frame) Box { return f.Button(save, "Save").PadTop(2) })
	if top.box.Width != plain.box.Width {
		t.Errorf("PadTop widened the button from %v to %v", plain.box.Width, top.box.Width)
	}
	left := probeButton(t, nil, func(f *Frame) Box { return f.Button(save, "Save").PadLeft(40) })
	if got, want := left.box.Width-plain.box.Width, float32(40-10); got != want {
		t.Errorf("PadLeft(40) widened the button by %v, want %v", got, want)
	}
}

// A button grows to hold padding that its height does not, and keeps its size's height
// when the padding fits in it.
func TestButtonGrowsToItsVerticalPadding(t *testing.T) {
	theme := gfx.LightTheme()
	small := probeButton(t, nil, func(f *Frame) Box { return f.Button(save, "Save").PadY(2) })
	if small.box.Height != theme.Metrics.ControlHeight {
		t.Errorf("PadY(2) made the button %v tall, want its size's %v", small.box.Height, theme.Metrics.ControlHeight)
	}
	big := probeButton(t, nil, func(f *Frame) Box { return f.Button(save, "Save").PadY(20) })
	if big.box.Height <= theme.Metrics.ControlHeight {
		t.Errorf("PadY(20) left the button %v tall", big.box.Height)
	}
	if big.surface != big.box || big.hit != big.box {
		t.Errorf("layout %v, surface %v, hit %v: want one rectangle", big.box, big.surface, big.hit)
	}
}

func TestButtonWeightAndColorStyleItsLabel(t *testing.T) {
	plain := probeButton(t, nil, func(f *Frame) Box { return f.Button(save, "Save").Outline() })
	red := gfx.Color{Red: 1, Alpha: 1}
	styled := probeButton(t, nil, func(f *Frame) Box { return f.Button(save, "Save").Outline().Weight(gfx.WeightBold).Color(red) })
	if styled.label.Style.Weight != gfx.WeightBold || plain.label.Style.Weight == gfx.WeightBold {
		t.Errorf("weights: plain %v, styled %v", plain.label.Style.Weight, styled.label.Style.Weight)
	}
	if styled.label.Style.Color != red || plain.label.Style.Color == red {
		t.Errorf("colors: plain %v, styled %v", plain.label.Style.Color, styled.label.Style.Color)
	}
}

// A button is restyled with the looks any element takes, over what its recipe gives.
func TestButtonTakesHoverAndPressedLooks(t *testing.T) {
	theme := gfx.LightTheme()
	over := gfx.Color{Red: 0.2, Green: 0.9, Blue: 0.4, Alpha: 1}
	declare := func(f *Frame) Box {
		return f.Button(save, "Save").Outline().Hover(Look{}.Background(over))
	}
	rest := probeButton(t, nil, declare)
	hovered := probeButton(t, func(c *gfx.Canvas) { c.SetInteraction(save, gfx.ID{}, gfx.ID{}, false) }, declare)
	fill := func(p buttonProbe) gfx.Color {
		rects := p.canvas.Rectangles()
		return rects[len(rects)-1].Color // the fill is painted over the line it sits in
	}
	if fill(hovered) != over {
		t.Errorf("hovered fill = %v, want %v", fill(hovered), over)
	}
	if fill(rest) == over || fill(rest) != theme.Background {
		t.Errorf("resting fill = %v, want the recipe's %v", fill(rest), theme.Background)
	}
}

// What a button is given as Children is its content, in place of its label and icon; the
// button around it is the same one, as tall as its size and as wide as its content and Pad.
func TestButtonHoldsChildrenInPlaceOfItsLabel(t *testing.T) {
	theme := gfx.LightTheme()
	icon := gfx.Icon{ViewBox: 24, Stroke: 2, Commands: []gfx.PathCommand{gfx.MoveTo(4, 4), gfx.LineTo(20, 20)}}
	p := probeButton(t, nil, func(f *Frame) Box {
		return f.Button(save, "Deploy").Icon(icon).Children(func() {
			f.Text("Go")
			f.Text("⌘D")
		})
	})
	for _, text := range p.texts {
		if text.Value == "Deploy" {
			t.Errorf("the label was painted over the children: %+v", p.texts)
		}
	}
	if len(p.texts) != 2 {
		t.Fatalf("texts = %d, want the two children", len(p.texts))
	}
	if p.box.Height != theme.Metrics.ControlHeight {
		t.Errorf("the button is %v tall, want its size's %v", p.box.Height, theme.Metrics.ControlHeight)
	}
	if p.surface != p.box || p.hit != p.box {
		t.Errorf("layout %v, surface %v, hit %v: want one rectangle", p.box, p.surface, p.hit)
	}
	hits := p.canvas.Hits()
	if len(hits) != 1 || hits[0].Role != gfx.RoleButton || hits[0].Label != "Deploy" || !hits[0].Focusable {
		t.Errorf("hits = %+v, want one focusable button named by its label", hits)
	}
	// The content is inside the padding, in order, with the gap between.
	first, second := p.texts[0].Bounds, p.texts[1].Bounds
	if first.X < p.box.X+10 || second.X+second.Width > p.box.X+p.box.Width-10 || second.X < first.X+first.Width {
		t.Errorf("children at %v and %v do not sit in order inside the button %v", first, second, p.box)
	}
	if gap := second.X - (first.X + first.Width); gap != 8 {
		t.Errorf("the children are %v apart, want the button's gap of 8", gap)
	}
}

// Children take the color the recipe gives the label, in every state and variant, unless
// they have a color of their own; so a destructive or a disabled button needs nothing said
// of what is inside it.
func TestButtonChildrenTakeTheLabelColor(t *testing.T) {
	theme := gfx.LightTheme()
	own := gfx.Color{Blue: 1, Alpha: 1}
	cases := []struct {
		name    string
		variant func(Box) Box
		state   func(*gfx.Canvas)
		disable bool
	}{
		{"default", func(b Box) Box { return b }, nil, false},
		{"outline", func(b Box) Box { return b.Outline() }, nil, false},
		{"destructive", func(b Box) Box { return b.Destructive() }, nil, false},
		{"disabled", func(b Box) Box { return b }, nil, true},
	}
	for _, tc := range cases {
		p := probeButton(t, tc.state, func(f *Frame) Box {
			b := tc.variant(f.Button(save, "x"))
			if tc.disable {
				b = b.Disabled(true)
			}
			return b.Children(func() {
				f.Text("inherits")
				f.Text("own").Color(own)
			})
		})
		look := components.ButtonRecipe(theme, variantOf(tc.name), gfx.State{Disabled: tc.disable})
		if got := p.texts[0].Style.Color; got != look.Foreground {
			t.Errorf("%s: the child is %v, want the label color %v", tc.name, got, look.Foreground)
		}
		if got := p.texts[1].Style.Color; got != own {
			t.Errorf("%s: a child with a color of its own is %v, want %v", tc.name, got, own)
		}
	}
}

func variantOf(name string) components.ButtonVariant {
	switch name {
	case "outline":
		return components.ButtonOutline
	case "destructive":
		return components.ButtonDestructive
	}
	return components.ButtonDefault
}

// A button given Children and one that paints its label are the same button to the
// keyboard and to the page: the same surface, the same focus ring.
func TestButtonWithChildrenIsDressedAsOneWithALabel(t *testing.T) {
	focused := func(c *gfx.Canvas) { c.SetInteraction(gfx.ID{}, gfx.ID{}, save, true) }
	label := probeButton(t, focused, func(f *Frame) Box { return f.Button(save, "Save").Outline() })
	held := probeButton(t, focused, func(f *Frame) Box {
		return f.Button(save, "Save").Outline().Children(func() { f.Text("Save") })
	})
	a, b := label.canvas.Rectangles(), held.canvas.Rectangles()
	if len(a) != len(b) || len(a) == 0 {
		t.Fatalf("rectangles: %d with a label, %d with children", len(a), len(b))
	}
	for i := range a {
		if a[i].Color != b[i].Color || a[i].Radius != b[i].Radius {
			t.Errorf("rectangle %d: %+v with a label, %+v with children", i, a[i], b[i])
		}
	}
}

// Whatever a button is padded, sized and styled with, one rectangle is its layout, its
// surface and its hit region, and its label is inside it.
func TestButtonIsOneRectangleWhateverItIsGiven(t *testing.T) {
	icon := gfx.Icon{ViewBox: 24, Stroke: 2, Commands: []gfx.PathCommand{gfx.MoveTo(4, 4), gfx.LineTo(20, 20)}}
	declares := map[string]func(f *Frame) Box{
		"pad":        func(f *Frame) Box { return f.Button(save, "Save").Pad(14) },
		"padX":       func(f *Frame) Box { return f.Button(save, "Save").PadX(3) },
		"padLeft":    func(f *Frame) Box { return f.Button(save, "Save").PadLeft(25) },
		"icon+padX":  func(f *Frame) Box { return f.Button(save, "Save").Icon(icon).PadX(22) },
		"small":      func(f *Frame) Box { return f.Button(save, "Save").Small().Ghost() },
		"large pad":  func(f *Frame) Box { return f.Button(save, "Save").Large().PadX(5) },
		"height":     func(f *Frame) Box { return f.Button(save, "Save").Height(50) },
		"font+bold":  func(f *Frame) Box { return f.Button(save, "Save").FontSize(18).Bold() },
		"gap":        func(f *Frame) Box { return f.Button(save, "Save").Icon(icon).Gap(20) },
		"icon only":  func(f *Frame) Box { return f.Button(save, "").IconOnly().Icon(icon) },
		"link+pad":   func(f *Frame) Box { return f.Button(save, "Save").Link().Pad(9) },
		"empty pad":  func(f *Frame) Box { return f.Button(save, "").PadX(0) },
		"destructiv": func(f *Frame) Box { return f.Button(save, "Delete").Destructive().PadX(16) },
	}
	for name, declare := range declares {
		p := probeButton(t, nil, declare)
		if p.hit != p.box {
			t.Errorf("%s: layout %v, hit %v", name, p.box, p.hit)
		}
		if p.surface != (Rect{}) && p.surface != p.box {
			t.Errorf("%s: layout %v, surface %v", name, p.box, p.surface)
		}
		for _, text := range p.texts {
			if text.Bounds.X < p.box.X-0.01 || text.Bounds.X+p.canvas.TextWidthWeight(text.Value, text.Style.Size, text.Style.Weight) > p.box.X+p.box.Width+0.01 {
				t.Errorf("%s: the label %v is outside the button %v", name, text.Bounds, p.box)
			}
		}
	}
}
