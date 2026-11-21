package ui

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
)

// toggleVariant is the component's treatment for the variant the Button
// methods chose: an outline toggle is the bordered one, anything else the
// borderless default.
func toggleVariant(Variant style.Variant) components.ToggleVariant {
	if Variant == style.VariantOutline {
		return components.ToggleOutline
	}
	return components.ToggleDefault
}

// frameToggleLook is the recipe a toggle is dressed with: the page's
// Styles.Toggle when it set one, else the theme's.
func frameToggleLook(f *Frame, n *Node, Palette *Theme, state gfx.State) components.ButtonLook {
	if f.GetStyles() != nil && f.GetStyles().Toggle != nil {
		return f.GetStyles().Toggle(*Palette, n.Variant, state)
	}
	return components.ToggleRecipe(*Palette, toggleVariant(n.Variant), state)
}

// paintToggle paints the default label of a toggle; the engine paints the
// surface, the ring and the hit around it, and clips the content area.
func paintToggle(f *Frame, n *Node) {
	if n.Flags&FlagChildren != 0 {
		return
	}
	_, _, fontSize := style.ToggleGeometry(f.Palette.Metrics, n.ButtonSize)
	if n.FontSize > 0 {
		fontSize = n.FontSize
	}
	weight := gfx.WeightMedium
	if n.WeightSet || n.Weight != 0 {
		weight = n.Weight
	}
	area := buttonContentArea(f, n)
	room := f.ContentRect(n)
	if room.Width > 0 && f.Surface.TextWidthWeight(n.Label, fontSize, weight) > room.Width {
		f.Surface.Clip(room)
		defer f.Surface.Unclip()
	}
	components.PaintButtonContent(f.Surface, area, components.ButtonContent{
		Label: n.Label, FontSize: fontSize, Weight: weight, Color: n.Color,
	})
}
