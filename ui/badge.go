package ui

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// Badge adds a presentational badge label, as components.PaintBadge paints it:
// a small chip whose variant dresses it. It takes no room beyond its text and
// responds to nothing unless given an identity with Hit, which makes a click on
// it report that identity (a removable tag's remove button, a link).
//
//	ui.Badge(c, "beta", components.BadgeSecondary)
//	ui.Badge(c, tag+" ×", components.BadgeSecondary).Hit(tagID, tag)
func (f *frame) Badge(label string, variant components.BadgeVariant) Box {
	b := f.box(kindBadge)
	n := &f.nodes[b.i]
	n.text, n.label = label, label
	f.extra(b.i).badge = variant
	return b
}

// Badge adds a badge to a page.
func Badge(c C, label string, variant components.BadgeVariant) Box {
	if c == nil || c.page == nil || c.f == nil {
		panic("ui: Badge is for a page")
	}
	return c.f.Badge(label, variant)
}

// paintBadge paints the chip the element measured, and its hit when one was
// declared.
func (f *frame) paintBadge(n *node) {
	rect := n.rect
	components.PaintBadge(f.canvas, f.theme, Rect{X: rect.X, Y: rect.Y}, n.text, f.exts[n.ext].badge)
	if n.flags&flagHasHit != 0 && !n.id.IsZero() {
		options := f.exts[n.ext].hit
		if options.Role == 0 {
			options.Role = gfx.RoleButton
		}
		f.canvas.HitWithOptions(n.id, rect, n.label, options)
	}
}
