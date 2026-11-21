package base_test

import (
	"testing"

	. "github.com/vivaui/viva/base"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
)

// A Table's element-level FontSize and Color style its header and its cells,
// replacing the spec's font size and the theme's foreground.
func TestTableElementTextStyleReachesHeaderAndCells(t *testing.T) {
	theme := gfx.LightTheme()
	ink := gfx.Color{Red: 1, Alpha: 1}
	spec := TableSpec{
		Columns: []style.TableColumn{{Width: 120}},
		Headers: []string{"Name"},
		Source:  &countingRows{n: 2},
	}
	paint := func(declare func(f *Frame) Box) ([]gfx.Text, Rect) {
		canvas := testCanvas()
		var view View
		var box Box
		view.Draw(canvas, theme, gfx.Rect{Width: 300, Height: 200}, func(f *Frame) {
			f.Column().Pad(10).Children(func() { box = declare(f) })
		})
		return canvas.Texts(), box.Rect()
	}

	plain, _ := paint(func(f *Frame) Box { return f.Table(&TableState{}, spec) })
	styled, _ := paint(func(f *Frame) Box {
		return f.Table(&TableState{}, spec).FontSize(18).Color(ink)
	})
	if len(plain) == 0 || len(styled) == 0 {
		t.Fatalf("painted %d and %d texts, want the header and cells", len(plain), len(styled))
	}
	if len(plain) != len(styled) {
		t.Fatalf("text count changed with the element's style: %d vs %d", len(plain), len(styled))
	}
	for _, text := range styled {
		if text.Style.Size != 18 {
			t.Errorf("text %q size = %v, want the element's 18", text.Value, text.Style.Size)
		}
		if text.Style.Color != ink {
			t.Errorf("text %q color = %v, want the element's %v", text.Value, text.Style.Color, ink)
		}
	}
	if plain[0].Style.Size == 18 {
		t.Error("the unstyled table already painted at 18; the test cannot tell the override apart")
	}
}

// The element's FontSize replaces the spec's, rather than adding to it.
func TestTableElementFontSizeWinsOverTheSpec(t *testing.T) {
	spec := TableSpec{
		Columns:  []style.TableColumn{{Width: 120}},
		Headers:  []string{"Name"},
		Source:   &countingRows{n: 1},
		FontSize: 12,
	}
	canvas := testCanvas()
	var view View
	view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 300, Height: 200}, func(f *Frame) {
		f.Column().Pad(10).Children(func() { f.Table(&TableState{}, spec).FontSize(16) })
	})
	texts := canvas.Texts()
	if len(texts) == 0 {
		t.Fatal("no table text painted")
	}
	for _, text := range texts {
		if text.Style.Size != 16 {
			t.Errorf("text %q size = %v, want the element's 16, not the spec's 12", text.Value, text.Style.Size)
		}
	}
}
