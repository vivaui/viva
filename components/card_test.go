package components

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestPaintCardLaysOutSectionsInsideSpacingAndRules(t *testing.T) {
	theme := gfx.LightTheme()
	bounds := gfx.Rect{X: 10, Y: 20, Width: 300, Height: 260}
	for _, size := range []CardSize{CardSizeDefault, CardSizeSmall} {
		var canvas gfx.Canvas
		layout := PaintCard(&canvas, theme, bounds, CardOptions{Size: size, HeaderHeight: 40, FooterHeight: 32, HeaderBorder: true, FooterBorder: true})
		spacing := CardSpacing(theme, size)
		if layout.Header.X != bounds.X+spacing || layout.Header.Y != bounds.Y+spacing || layout.Header.Width != bounds.Width-2*spacing {
			t.Fatalf("size %d header = %+v", size, layout.Header)
		}
		if layout.Footer.Y+layout.Footer.Height != bounds.Y+bounds.Height-spacing {
			t.Fatalf("size %d footer = %+v", size, layout.Footer)
		}
		if layout.Content.Y <= layout.Header.Y+layout.Header.Height || layout.Content.Y+layout.Content.Height >= layout.Footer.Y {
			t.Fatalf("size %d content %+v is not between header %+v and footer %+v", size, layout.Content, layout.Header, layout.Footer)
		}
		rules := 0
		for _, rectangle := range canvas.Rectangles() {
			if rectangle.Bounds.Height == 1 && rectangle.Color == theme.Border {
				rules++
			}
		}
		if rules != 2 {
			t.Fatalf("size %d painted %d rules, want header border-b and footer border-t", size, rules)
		}
	}
}

func TestPaintCardWithoutSectionsGivesAllTheInnerArea(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	layout := PaintCard(&canvas, theme, gfx.Rect{Width: 200, Height: 120}, CardOptions{Size: CardSizeSmall})
	spacing := CardSpacing(theme, CardSizeSmall)
	want := gfx.Rect{X: spacing, Y: spacing, Width: 200 - 2*spacing, Height: 120 - 2*spacing}
	if layout.Content != want || layout.Header != (gfx.Rect{}) || layout.Footer != (gfx.Rect{}) {
		t.Fatalf("layout = %+v, want content %+v only", layout, want)
	}
}
