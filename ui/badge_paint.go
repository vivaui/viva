package ui

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
)

// badgeVariant is the chip's look, from the variant the Button methods chose:
// the two recipes share the names, and an outline destructive button has no
// separate chip.
func badgeVariant(Variant components.ButtonVariant) components.BadgeVariant {
	switch Variant {
	case components.ButtonSecondary:
		return components.BadgeSecondary
	case components.ButtonOutline:
		return components.BadgeOutline
	case components.ButtonGhost:
		return components.BadgeGhost
	case components.ButtonLink:
		return components.BadgeLink
	case components.ButtonDestructive, components.ButtonDestructiveOutline:
		return components.BadgeDestructive
	}
	return components.BadgeDefault
}

// frameBadgeLook is the recipe a badge is dressed with: the page's Styles.Badge
// when it set one, else the theme's. A badge is presentational, so the state
// only reaches a page recipe that asks for it.
func frameBadgeLook(f *Frame, n *Node, Palette *Theme, state gfx.State) components.ButtonLook {
	if f.GetStyles() != nil && f.GetStyles().Badge != nil {
		return f.GetStyles().Badge(*Palette, n.Variant, state)
	}
	return components.BadgeRecipe(*Palette, badgeVariant(n.Variant), state)
}

// paintBadge paints the default label of a badge; the engine paints the chip's
// surface around it and clips it to the content area. Content of the caller's
// own is declared with Children and laid out by the engine instead.
func paintBadge(f *Frame, n *Node) {
	if n.Flags&FlagChildren != 0 {
		return
	}
	_, _, _, fontSize := style.BadgeGeometry(f.Palette.Metrics)
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
