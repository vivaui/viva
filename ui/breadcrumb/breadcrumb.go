// Package breadcrumb is the opt-in page control for a breadcrumb: the path to
// where the user is, each place a link back, the last one the current place. A
// page keeps a Breadcrumb in its model, because a collapsed path opens a menu of
// the places it hides:
//
//	m.path = breadcrumb.New()
//	// in the page:
//	m.path.View(c, "Location", breadcrumb.Spec{Items: m.places, Max: 4})
//	if i, ok := m.path.Clicked(); ok {
//		m.goTo(i)
//	}
//
// The items are in order from the root; every item but the last is a link, and
// the last is the current place and is not. Max, when above two, keeps the first
// place and the last Max-1 and puts the ones between behind an ellipsis that opens
// a menu: choosing a place there reports it as a click on that item. A Disabled
// item is shown but cannot be chosen. Separator is the word between places, "/"
// by default.
//
// It is a separate package so an application that does not import it pays
// nothing for it. Keep a Breadcrumb in the model: its menu state is made once, so
// a warm paint allocates nothing. Do not copy a Breadcrumb after it has been
// declared.
package breadcrumb

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var dotsIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
	gfx.MoveTo(5, 12), gfx.LineTo(5.01, 12), gfx.MoveTo(12, 12), gfx.LineTo(12.01, 12), gfx.MoveTo(19, 12), gfx.LineTo(19.01, 12),
}}

// Item is one place on the path.
type Item struct {
	// Label names the place.
	Label string
	// Disabled shows the place but takes no press.
	Disabled bool
}

// Spec declares the path.
type Spec struct {
	// Items are the places, from the root to where the user is. Keep the slice in
	// the model: a slice made in each paint allocates in each paint.
	Items []Item
	// Max is how many places are shown before the ones in between collapse behind
	// an ellipsis; zero, or any number below three, shows them all.
	Max int
	// Separator is the word between places; "/" when empty.
	Separator string
	// MoreLabel is the accessible name of the ellipsis, "More" when empty.
	MoreLabel string
}

// Breadcrumb is one path and the menu of the places it collapses.
type Breadcrumb struct {
	open    bool
	clicked int // 1 + the item chosen in the paint being declared, or 0
}

// New returns a breadcrumb.
func New() *Breadcrumb { return &Breadcrumb{} }

// Clicked reports which place the user chose in the paint just made, whether from
// the path or from the menu behind the ellipsis.
func (b *Breadcrumb) Clicked() (int, bool) { return b.clicked - 1, b.clicked > 0 }

// View declares the path at c. label names it for assistive technology. The path
// is one line high. Declare the view on its own, not inside a Memo.
func (b *Breadcrumb) View(c ui.C, label string, spec Spec) ui.Box {
	if b == nil {
		panic("breadcrumb: View needs its Breadcrumb")
	}
	b.clicked = 0
	theme := c.Theme()
	metrics := theme.Metrics
	separator := spec.Separator
	if separator == "" {
		separator = "/"
	}
	more := spec.MoreLabel
	if more == "" {
		more = "More"
	}
	last := len(spec.Items) - 1
	// The places shown: all of them, or the first, an ellipsis and the last Max-1.
	hideFrom, hideTo := 0, 0
	if spec.Max >= 3 && len(spec.Items) > spec.Max {
		hideFrom, hideTo = 1, len(spec.Items)-(spec.Max-1)
	}
	root := ui.Row(c).Items(ui.Center).Gap(metrics.GapItem * 0.5)
	root = root.HitWith(ui.ID(c), label, gfx.HitOptions{Role: gfx.RoleGroup, SkipFocus: true})
	root.Children(func() {
		for i := range spec.Items {
			if i >= hideFrom && i < hideTo {
				if i == hideFrom {
					b.ellipsis(c, spec, hideFrom, hideTo, more)
					ui.Text(c, separator).Muted().Shrink(0)
				}
				continue
			}
			item := spec.Items[i]
			text := ui.Text(c.KeyInt(i), item.Label).Shrink(0)
			switch {
			case i == last:
				text.Weight(gfx.WeightMedium)
			case item.Disabled:
				text.Muted().Opacity(0.5)
			default:
				text.Muted()
				text.HitWith(ui.ID(c), item.Label, gfx.HitOptions{Role: gfx.RoleLink})
				text.Hover(ui.Look{}.Color(theme.Foreground))
				if text.Clicked() {
					b.clicked = i + 1
				}
			}
			if i != last {
				ui.Text(c, separator).Muted().Shrink(0)
			}
		}
	})
	return root
}

// ellipsis declares the button for the hidden places and the menu that lists them.
func (b *Breadcrumb) ellipsis(c ui.C, spec Spec, from, to int, label string) {
	trigger := ui.Button(c, label).Icon(dotsIcon).IconOnly().Ghost().Shrink(0)
	if trigger.Clicked() {
		b.open = !b.open
	}
	ui.Menu(c, trigger, &b.open, func(menu *ui.MenuItems) {
		for i := from; i < to; i++ {
			item := menu.Item(spec.Items[i].Label)
			if spec.Items[i].Disabled {
				item.Disabled(true)
			}
			if item.Clicked() {
				b.clicked = i + 1
			}
		}
	})
}
