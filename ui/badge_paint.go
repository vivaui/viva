package ui

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
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

// paintBadge paints the chip the view measured, and its hit when one was
// declared.
func paintBadge(f *Frame, v ControlView) {
	rect := v.Rect
	components.PaintBadge(f.Surface, f.Palette, Rect{X: rect.X, Y: rect.Y}, v.Text, badgeVariant(v.Variant))
	if v.HasHit && !v.ID.IsZero() {
		options := v.Hit
		if options.Role == 0 {
			options.Role = gfx.RoleButton
		}
		f.Surface.HitWithOptions(v.ID, rect, v.Label, options)
	}
}
