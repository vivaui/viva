package base_test

import (
	"testing"

	. "github.com/vivaui/viva/base"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

type fieldProbe struct {
	box   Rect
	rects []gfx.Rectangle
	texts []gfx.Text
	hits  []gfx.Hit
}

func probeField(t *testing.T, declare func(f *Frame) Box) fieldProbe {
	t.Helper()
	canvas := testCanvas()
	var view View
	var box Box
	view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 400, Height: 240}, func(f *Frame) {
		f.Column().Pad(6).Children(func() { box = declare(f) })
	})
	return fieldProbe{box: box.Rect(), rects: canvas.Rectangles(), texts: canvas.Texts(), hits: canvas.Hits()}
}

// A themed Input is the text-entry recipe's control: the fill and line of the
// recipe, the value at the element's text style, and no width of its own.
func TestInputDefaultsAreTheRecipe(t *testing.T) {
	theme := gfx.LightTheme()
	look := components.TextEntryRecipe(theme, gfx.State{})
	p := probeField(t, func(f *Frame) Box {
		return f.Input(InputSnapshot{ID: gfx.Item(1), Label: "Name", Value: "Gopher"}).Width(200)
	})
	if p.box.Height != theme.Metrics.ControlHeight {
		t.Errorf("input height = %v, want the control height %v", p.box.Height, theme.Metrics.ControlHeight)
	}
	if len(p.texts) == 0 || p.texts[0].Value != "Gopher" {
		t.Fatalf("texts = %+v, want the value", p.texts)
	}
	if p.texts[0].Style.Size != theme.Metrics.FontSizeM || p.texts[0].Style.Color != look.Fill.Over(theme.Background) && p.texts[0].Style.Color != theme.Foreground {
		// The recipe's foreground is the page's; only the size is worth pinning here.
		if p.texts[0].Style.Size != theme.Metrics.FontSizeM {
			t.Errorf("value size = %v, want %v", p.texts[0].Style.Size, theme.Metrics.FontSizeM)
		}
	}
	found := false
	for _, rect := range p.rects {
		if rect.Bounds == p.box {
			found = true
			if rect.Radius != look.Radius {
				t.Errorf("input radius = %v, want %v", rect.Radius, look.Radius)
			}
		}
	}
	if !found {
		t.Errorf("no input surface for %v: %+v", p.box, p.rects)
	}
}

// FontSize and Weight style the value; a larger size makes the field taller
// rather than clipping the text, and explicit Pad moves the text inside.
func TestInputTextStyleAndPaddingReachTheValue(t *testing.T) {
	theme := gfx.LightTheme()
	baseLine := probeField(t, func(f *Frame) Box {
		return f.Input(InputSnapshot{ID: gfx.Item(1), Label: "Name", Value: "Gopher"}).Width(200)
	})
	big := probeField(t, func(f *Frame) Box {
		return f.Input(InputSnapshot{ID: gfx.Item(1), Label: "Name", Value: "Gopher"}).
			Width(200).FontSize(30).Weight(gfx.WeightBold)
	})
	if big.box.Height <= baseLine.box.Height {
		t.Errorf("FontSize 30 input height = %v, want taller than %v", big.box.Height, baseLine.box.Height)
	}
	if len(big.texts) == 0 {
		t.Fatal("the larger input painted no value")
	}
	if got := big.texts[0].Style; got.Size != 30 || got.Weight != gfx.WeightBold {
		t.Errorf("value style = %+v, want size 30, bold", got)
	}

	padded := probeField(t, func(f *Frame) Box {
		return f.Input(InputSnapshot{ID: gfx.Item(1), Label: "Name", Value: "Gopher"}).Width(200).PadSides(9, 24, 9, 24)
	})
	if len(padded.texts) == 0 {
		t.Fatal("the padded input painted no value")
	}
	if padded.texts[0].Bounds.X < padded.box.X+24-0.01 {
		t.Errorf("value x = %v, want at least %v (explicit left padding)", padded.texts[0].Bounds.X, padded.box.X+24)
	}
	_ = theme
}

// A focused or invalid field draws the recipe's focus or danger line.
func TestInputStatesFollowTheRecipe(t *testing.T) {
	theme := gfx.LightTheme()
	focused := probeField(t, func(f *Frame) Box {
		return f.Input(InputSnapshot{ID: gfx.Item(1), Label: "Name", Value: "Gopher", Focused: true}).Width(200)
	})
	want := components.TextEntryRecipe(theme, gfx.State{Focused: true})
	ok := false
	for _, rect := range focused.rects {
		ok = ok || rect.Color == want.Border.Over(theme.Background)
	}
	if !ok {
		t.Errorf("focused input paints no focus line %v: %+v", want.Border, focused.rects)
	}
	invalid := probeField(t, func(f *Frame) Box {
		return f.Input(InputSnapshot{ID: gfx.Item(1), Label: "Name", Value: "Gopher", Invalid: true}).Width(200)
	})
	wantInvalid := components.TextEntryRecipe(theme, gfx.State{Invalid: true})
	danger := false
	for _, rect := range invalid.rects {
		danger = danger || rect.Color == wantInvalid.Border.Over(theme.Background)
	}
	if !danger {
		t.Errorf("invalid input paints no dangerous line: %+v", invalid.rects)
	}
}

// A themed Textarea is at least 64 tall, grows with its text size, and its
// value takes the element's FontSize and Weight.
func TestTextareaTextStyleAndHeight(t *testing.T) {
	theme := gfx.LightTheme()
	p := probeField(t, func(f *Frame) Box {
		return f.Textarea(TextareaSnapshot{ID: gfx.Item(2), Label: "Notes", Value: "line one"}).FontSize(24).Weight(gfx.WeightBold)
	})
	if p.box.Height < 64 {
		t.Errorf("textarea height = %v, want at least 64", p.box.Height)
	}
	if len(p.texts) == 0 {
		t.Fatal("the textarea painted no row")
	}
	if got := p.texts[0].Style; got.Size != 24 || got.Weight != gfx.WeightBold {
		t.Errorf("row style = %+v, want size 24, bold", got)
	}
	tall := probeField(t, func(f *Frame) Box {
		return f.Textarea(TextareaSnapshot{ID: gfx.Item(2), Label: "Notes", Value: "line one"}).Height(140)
	})
	if tall.box.Height != 140 {
		t.Errorf("explicit textarea height = %v, want 140", tall.box.Height)
	}
	_ = theme
}
