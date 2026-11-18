package ui

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// Badge adds a presentational badge label, as components.PaintBadge paints it: a
// small chip dressed by its variant, chosen with the Button variants' methods.
// It takes no room beyond its text and responds to nothing unless given an
// identity with Hit, which makes a click on it report that identity (a removable
// tag's remove button, a link).
//
//	ui.Badge(c, "beta").Secondary()
//	ui.Badge(c, tag+" ×").Secondary().Hit(tagID, tag)
func (f *frame) Badge(label string) Box {
	b := f.box(kindBadge)
	n := &f.Nodes[b.i]
	n.text, n.label = label, label
	return b
}

// Badge adds a badge to a page.
func Badge(c C, label string) Box {
	if c == nil || c.page == nil || c.f == nil {
		panic("ui: Badge is for a page")
	}
	return c.f.Badge(label)
}

// badgeVariant is the chip's look, from the variant the Button methods chose:
// the two recipes share the names, and an outline destructive button has no
// separate chip.
func badgeVariant(variant components.ButtonVariant) components.BadgeVariant {
	switch variant {
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
func (f *frame) paintBadge(v ControlView) {
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
