package components

import (
	"reflect"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
)

// measured returns a canvas that measures text the way the test pages do.
func measured() *gfx.Canvas {
	canvas := &gfx.Canvas{}
	canvas.SetMeasurer(func(Value string, Size, Weight float32) float32 { return float32(len(Value)) * Size / 2 })
	return canvas
}

// A badge of the page's own, dressed from BadgeRecipe with the canvas calls the
// engine makes, is the chip PaintBadge paints: for every theme and variant the
// surface is the same shapes in the same colors. This is what the node-based
// Badge relies on.
func TestBadgeRecipePaintsTheSurfaceAsTheComponentDoes(t *testing.T) {
	variants := map[string]BadgeVariant{
		"default": BadgeDefault, "secondary": BadgeSecondary, "destructive": BadgeDestructive,
		"outline": BadgeOutline, "ghost": BadgeGhost, "link": BadgeLink,
	}
	for themeName, theme := range recipeThemes() {
		for variantName, variant := range variants {
			t.Run(themeName+"/"+variantName, func(t *testing.T) {
				label := "beta"
				width, height := BadgeExtent(measured(), theme, label)
				bounds := gfx.Rect{X: 4, Y: 4, Width: width, Height: height}
				want := measured()
				PaintBadge(want, theme, gfx.Rect{X: bounds.X, Y: bounds.Y}, label, variant)

				var got gfx.Canvas
				look := BadgeRecipe(theme, variant, gfx.State{})
				background := gfx.Color{}
				if look.Filled {
					background = look.Background
				}
				bordered := look.Bordered && look.Filled
				if bordered && theme.Background.Alpha >= 1 {
					background = background.Over(theme.Background)
					look.Border = look.Border.Over(theme.Background)
				}
				switch {
				case bordered:
					paintBordered(&got, theme, bounds, look.Radius, look.Border, background)
				case look.Filled:
					got.Rounded(bounds, look.Radius, background)
				}
				if !reflect.DeepEqual(noOrders(want.Rectangles()), noOrders(got.Rectangles())) {
					t.Errorf("recipe and component differ:\nwant %+v\n got %+v", want.Rectangles(), got.Rectangles())
				}
			})
		}
	}
}

// The recipe's pill radius is the geometry's: half the chip height, which is
// what makes a badge a pill rather than a rounded rectangle.
func TestBadgeRecipeRadiusIsHalfTheChip(t *testing.T) {
	theme := gfx.LightTheme()
	height, padding, radius, fontSize := style.BadgeGeometry(theme.Metrics)
	if radius != height/2 {
		t.Errorf("radius %v, want half of height %v", radius, height)
	}
	look := BadgeRecipe(theme, BadgeDefault, gfx.State{})
	if look.Radius != radius {
		t.Errorf("recipe radius %v, want %v", look.Radius, radius)
	}
	if padding <= 0 || fontSize <= 0 {
		t.Errorf("geometry padding %v, fontSize %v, want both positive", padding, fontSize)
	}
}
