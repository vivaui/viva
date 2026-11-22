package components

import (
	"reflect"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
)

// A toggle of the page's own, dressed from ToggleRecipe with the canvas calls
// the engine makes, is the toggle the component paints: for every theme,
// variant, size and state the surface is the same shapes in the same colors.
func TestToggleRecipePaintsTheToggleAsTheComponentDoes(t *testing.T) {
	variants := map[string]ToggleVariant{"default": ToggleDefault, "outline": ToggleOutline}
	sizes := map[string]struct {
		small, large bool
		height       float32
	}{"default": {height: 32}, "small": {small: true, height: 28}, "large": {large: true, height: 36}}
	states := map[string]gfx.State{
		"rest": {}, "active": {Checked: true}, "hover": {Hovered: true},
		"pressed": {Hovered: true, Pressed: true}, "focus": {FocusVisible: true},
		"invalid": {Invalid: true}, "disabled": {Disabled: true},
		"disabled active": {Disabled: true, Checked: true}, "disabled invalid": {Disabled: true, Invalid: true},
	}
	for themeName, theme := range recipeThemes() {
		for variantName, variant := range variants {
			for sizeName, size := range sizes {
				for stateName, state := range states {
					t.Run(themeName+"/"+variantName+"/"+sizeName+"/"+stateName, func(t *testing.T) {
						id := gfx.Item(1)
						bounds := gfx.Rect{X: 4, Y: 4, Width: 90, Height: size.height}
						want := measured()
						if !state.Disabled {
							hovered, pressed, focused := gfx.ID{}, gfx.ID{}, gfx.ID{}
							if state.Hovered {
								hovered = id
							}
							if state.Pressed {
								pressed = id
							}
							if state.FocusVisible {
								focused = id
							}
							want.SetInteraction(hovered, pressed, focused, state.FocusVisible)
						}
						toggle := BeginToggle(want, theme, id, bounds, state.Checked)
						if size.small {
							toggle = toggle.Small()
						}
						if size.large {
							toggle = toggle.Large()
						}
						if state.Disabled {
							toggle = toggle.Disabled()
						}
						if state.Invalid {
							toggle = toggle.Invalid()
						}
						if variant == ToggleOutline {
							toggle.Outline("Bold")
						} else {
							toggle.Default("Bold")
						}

						got := measured()
						look := ToggleRecipe(theme, variant, state)
						if look.Shadow != gfx.ShadowNone {
							got.Elevate(bounds, look.Radius, look.Shadow)
						}
						if look.RingWidth > 0 {
							got.Rounded(expand(bounds, look.RingWidth), look.RingRadius, look.Ring)
						}
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
							paintBordered(got, theme, bounds, look.Radius, look.Border, background)
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
	}
}

// The recipe gives the same geometry the component fits: the size's height,
// 10px either side and the medium font, never shorter than 20.
func TestToggleGeometryMatchesTheComponent(t *testing.T) {
	for name, theme := range recipeThemes() {
		for sizeName, size := range map[string]ToggleSize{"default": ToggleSizeDefault, "small": ToggleSizeSmall, "large": ToggleSizeLarge} {
			height, padding, fontSize := style.ToggleGeometry(theme.Metrics, toggleStyleSize(size))
			if got := toggleControlHeight(theme, size); got != height {
				t.Errorf("%s/%s: component height %v, geometry %v", name, sizeName, got, height)
			}
			if padding != 10 {
				t.Errorf("%s/%s: padding %v, want 10", name, sizeName, padding)
			}
			if fontSize != theme.Metrics.FontSizeM {
				t.Errorf("%s/%s: font size %v, want %v", name, sizeName, fontSize, theme.Metrics.FontSizeM)
			}
			if height < 20 {
				t.Errorf("%s/%s: height %v below 20", name, sizeName, height)
			}
		}
	}
}
