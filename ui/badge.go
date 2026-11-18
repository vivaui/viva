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
	n := &f.nodes[b.i]
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

// paintBadge paints the chip the element measured, and its hit when one was
// declared.
func (f *frame) paintBadge(n *node) {
	rect := n.rect
	components.PaintBadge(f.canvas, f.theme, Rect{X: rect.X, Y: rect.Y}, n.text, badgeVariant(n.variant))
	if n.flags&flagHasHit != 0 && !n.id.IsZero() {
		options := f.exts[n.ext].hit
		if options.Role == 0 {
			options.Role = gfx.RoleButton
		}
		f.canvas.HitWithOptions(n.id, rect, n.label, options)
	}
}
