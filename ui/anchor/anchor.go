// Package anchor is the opt-in page control for an in-page table of contents: a
// vertical list of links to the sections of a long page, with a rail down its
// side that marks the section in view. A page keeps an Anchor in its model:
//
//	m.toc = anchor.New()
//	// in the page:
//	m.toc.View(c, "On this page", anchor.Spec{Items: m.sections, Active: m.section})
//	if i, ok := m.toc.Clicked(); ok {
//		m.scrollTo(m.sections[i]) // and m.section = i
//	}
//
// The control owns neither the scrolling nor which section is in view: the page
// knows where its sections are, so it scrolls to the one a click reports and
// tells the control which is Active as it scrolls. An Item with Level 1 or more
// is indented under the item before it, for the headings below a heading. Active
// below zero marks none.
//
// It is a separate package so an application that does not import it pays
// nothing for it. Keep an Anchor in the model; a warm paint allocates nothing.
// Do not copy an Anchor after it has been declared.
package anchor

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Item is one link.
type Item struct {
	// Title is the link's words.
	Title string
	// Level is how far the link is indented: zero for a top-level section.
	Level int
}

// Spec declares the list.
type Spec struct {
	// Items are the links, in the order of the page. Keep the slice in the
	// model: a slice made in each paint allocates in each paint.
	Items []Item
	// Active is the index of the section in view, or below zero for none.
	Active int
}

// Anchor is one list of links.
type Anchor struct {
	clicked int // 1 + the link pressed in the paint being declared, or 0
}

// New returns an anchor.
func New() *Anchor { return &Anchor{} }

// Clicked reports which link the user pressed in the paint just made.
func (a *Anchor) Clicked() (int, bool) { return a.clicked - 1, a.clicked > 0 }

// View declares the list at c. label names it for assistive technology. The list
// is as wide as its longest link and as tall as its links; give it a width to
// wrap or clip longer ones. Declare the view on its own, not inside a Memo.
func (a *Anchor) View(c ui.C, label string, spec Spec) ui.Box {
	if a == nil {
		panic("anchor: View needs its Anchor")
	}
	a.clicked = 0
	theme := c.Theme()
	metrics := theme.Metrics
	root := ui.Column(c).HitWith(ui.ID(c), label, gfx.HitOptions{Role: gfx.RoleGroup, SkipFocus: true})
	root.Children(func() {
		for i := range spec.Items {
			item := spec.Items[i]
			row := c.KeyInt(i)
			rail := theme.Border
			if i == spec.Active {
				rail = theme.Primary
			}
			ui.Row(row).FillWidth().Children(func() {
				ui.Spacer(row).Width(2).Grow(0).Shrink(0).Background(rail)
				text := ui.Text(row, item.Title).FontSize(metrics.FontSizeS).Ellipsis().Grow(1).Basis(0)
				text = text.MarginSides(4, 0, 4, metrics.GapItem+float32(item.Level)*metrics.GapItem*1.5)
				if i == spec.Active {
					text.Color(theme.Primary).Weight(gfx.WeightMedium)
				} else {
					text.Muted()
					text.Hover(ui.Look{}.Color(theme.Foreground))
				}
				text.HitWith(ui.ID(row), item.Title, gfx.HitOptions{Role: gfx.RoleLink})
				if text.Clicked() {
					a.clicked = i + 1
				}
			})
		}
	})
	return root
}
