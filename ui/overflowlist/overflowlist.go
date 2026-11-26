// Package overflowlist is the opt-in page control for a row of icon actions that
// shows as many as fit and puts the rest in a "More" menu: a toolbar that keeps
// working when its window or panel is narrowed. A page keeps an OverflowList in
// its model and gives it the actions each paint:
//
//	m.tools = overflowlist.New()
//	// in the page:
//	m.tools.View(c, "Actions", overflowlist.Spec{Items: m.actions}).FillWidth()
//	if i, ok := m.tools.Clicked(); ok {
//		m.run(m.actions[i])
//	}
//
// An Item is an icon, the label that names it (its tooltip and accessible name
// when shown, its words in the menu when not) and whether it is Disabled. Items
// leave the row from the end, last first, and appear in the menu in their own
// order; a Disabled item stays disabled in the menu. When every item fits there is
// no menu button. Min keeps at least that many items in the row however narrow it
// gets, and the row never leaves room for fewer than one button besides More.
//
// The number that fit is measured from the width the control is placed in, never
// from what it holds, so it settles in the paint that first sees the width and
// again as the window is resized; it asks for nothing between resizes.
//
// It is a separate package so an application that does not import it pays
// nothing for it. Keep an OverflowList in the model: its menu state is made once.
// Do not copy one after it has been declared.
package overflowlist

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var moreIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
	gfx.MoveTo(5, 12), gfx.LineTo(5.01, 12), gfx.MoveTo(12, 12), gfx.LineTo(12.01, 12), gfx.MoveTo(19, 12), gfx.LineTo(19.01, 12),
}}

// Item is one action.
type Item struct {
	// Label names the action: its accessible name and tooltip in the row, its
	// words in the menu.
	Label string
	// Icon is the glyph of the action's button.
	Icon gfx.Icon
	// Disabled leaves the action inert, in the row and in the menu.
	Disabled bool
}

// Spec declares the row.
type Spec struct {
	// Items are the actions, in order. Keep the slice in the model.
	Items []Item
	// Min is how many items stay in the row however narrow it gets; one when
	// zero.
	Min int
	// Gap is the space between buttons; the theme's item gap when zero.
	Gap float32
	// MoreLabel is the accessible name of the menu button, "More" when empty.
	MoreLabel string
}

// OverflowList is one row and its menu.
type OverflowList struct {
	open    bool
	clicked int // 1 + the item pressed in the paint being declared, or 0
}

// New returns a row.
func New() *OverflowList { return &OverflowList{} }

// Clicked reports which action the user chose in the paint just made, from the row
// or from the menu.
func (o *OverflowList) Clicked() (int, bool) { return o.clicked - 1, o.clicked > 0 }

// View declares the row at c. label names it for assistive technology. The row is
// one button high and as wide as the room it is given; give it FillWidth. Declare
// the view on its own, not inside a Memo.
func (o *OverflowList) View(c ui.C, label string, spec Spec) ui.Box {
	if o == nil {
		panic("overflowlist: View needs its OverflowList")
	}
	o.clicked = 0
	theme := c.Theme()
	gap := spec.Gap
	if gap <= 0 {
		gap = theme.Metrics.GapItem
	}
	button := theme.Metrics.ControlHeight
	more := spec.MoreLabel
	if more == "" {
		more = "More"
	}
	n := len(spec.Items)
	root := ui.Container(c).FillWidth().Label(label)
	root.Children(func() {
		shown := n
		// The widest row that fits: all of the items, or as many as leave room for the
		// menu button. Each question is answered from the width the container had in the
		// last layout.
		if n > 0 && !c.AtLeast(ui.Width(span(n, button, gap))) {
			floor := max(min(max(spec.Min, 1), n), 1)
			shown = floor
			for k := n - 1; k > floor; k-- {
				if c.AtLeast(ui.Width(span(k+1, button, gap))) {
					shown = k
					break
				}
			}
		}
		ui.Row(c).FillWidth().Items(ui.Center).Gap(gap).Children(func() {
			for i := 0; i < shown; i++ {
				item := &spec.Items[i]
				b := ui.Button(c.KeyInt(i), item.Label).Icon(item.Icon).IconOnly().Ghost().Disabled(item.Disabled).Shrink(0)
				if b.Clicked() {
					o.clicked = i + 1
				}
			}
			if shown < n {
				ui.Spacer(c).Grow(1)
				trigger := ui.Button(c, more).Icon(moreIcon).IconOnly().Ghost().Shrink(0)
				if trigger.Clicked() {
					o.open = !o.open
				}
				ui.Menu(c, trigger, &o.open, func(menu *ui.MenuItems) {
					for i := shown; i < n; i++ {
						entry := menu.Item(spec.Items[i].Label)
						if spec.Items[i].Disabled {
							entry.Disabled(true)
						}
						if entry.Clicked() {
							o.clicked = i + 1
						}
					}
				})
			}
		})
	})
	return root
}

// span is the width of count buttons with a gap between each.
func span(count int, button, gap float32) float32 {
	return float32(count)*button + float32(max(count-1, 0))*gap
}
