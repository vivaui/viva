// Package transfer is the opt-in page control for moving items between two
// lists: the ones on the left are available, the ones on the right are chosen.
// A page keeps the items in its model and declares the two lists with a Spec:
//
//	m.moves = transfer.New()
//	// in the page:
//	m.moves.View(c, "Groups", transfer.Spec{Items: m.groups, Titles: [2]string{"Available", "Selected"}})
//	if m.moves.Changed() {
//		// items moved; m.groups[i].Target says which side each is on
//	}
//
// An Item says on which side it is (Target is true for the right list) and whether
// it is ticked (Checked); the control moves the ticked items of a list to the
// other list with the button between them, and writes Target and Checked in the
// caller's slice, so the page reads the result where it keeps the items. A
// Disabled item can be neither ticked nor moved. The tick above a list ticks or
// clears every item in it. Each list scrolls when it is longer than its Height.
//
// Every item of a list is declared in each paint, so it is for lists of some
// hundreds of items, not for a table of thousands.
//
// It is a separate package so an application that does not import it pays
// nothing for it. Keep a Transfer in the model: it holds the state of the two
// ticks above the lists, so a warm paint allocates nothing. Do not copy a Transfer
// after it has been declared.
package transfer

import (
	"strconv"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var (
	rightIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(9, 18), gfx.LineTo(15, 12), gfx.LineTo(9, 6),
	}}
	leftIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(15, 18), gfx.LineTo(9, 12), gfx.LineTo(15, 6),
	}}
)

// Item is one thing that is in one of the two lists.
type Item struct {
	// Title is what the list shows.
	Title string
	// Target is whether the item is in the right list; false is the left one.
	Target bool
	// Checked is whether the item is ticked, to be moved with the button.
	Checked bool
	// Disabled makes the item neither tickable nor movable.
	Disabled bool
}

// Spec declares the two lists.
type Spec struct {
	// Items are everything, on either side, in the order the lists show them.
	// Keep the slice in the model: the control writes Target and Checked in it.
	Items []Item
	// Titles head the left and the right list; they are not drawn when empty.
	Titles [2]string
	// Height is the height of a list; 240 when zero or less.
	Height float32
	// MoveLabels are the accessible names of the buttons that move the ticked
	// items to the right and to the left; "Move right" and "Move left" when empty.
	MoveLabels [2]string
}

// Transfer is one pair of lists.
type Transfer struct {
	all      [2]bool // the tick above each list, as of the last paint
	counts   [2]countText
	changed  bool
	reported bool
}

// countText is the "ticked/total" words above a list, made when the counts change.
type countText struct {
	ticked, total int
	text          string
}

// New returns a pair of lists.
func New() *Transfer { return &Transfer{} }

// Changed reports whether items moved in the paint that delivered it.
func (t *Transfer) Changed() bool { return t.reported }

func (c *countText) set(ticked, total int) string {
	if c.text == "" || c.ticked != ticked || c.total != total {
		c.ticked, c.total = ticked, total
		c.text = strconv.Itoa(ticked) + "/" + strconv.Itoa(total)
	}
	return c.text
}

// stats counts the items of a side and how many of them are ticked, and whether
// every tickable one is.
func stats(items []Item, target bool) (total, ticked int, every bool) {
	tickable := 0
	for i := range items {
		item := &items[i]
		if item.Target != target {
			continue
		}
		total++
		if item.Disabled {
			continue
		}
		tickable++
		if item.Checked {
			ticked++
		}
	}
	return total, ticked, tickable > 0 && ticked == tickable
}

// move moves the ticked items of a side to the other and unticks them.
func (t *Transfer) move(items []Item, from bool) {
	for i := range items {
		item := &items[i]
		if item.Target == from && item.Checked && !item.Disabled {
			item.Target, item.Checked = !from, false
			t.changed = true
		}
	}
}

// View declares the two lists and the buttons between them at c. label names the
// control for assistive technology. The control is as wide as the room it is given
// and as tall as Height plus the heads; give it FillWidth. Declare the view on its
// own, not inside a Memo.
func (t *Transfer) View(c ui.C, label string, spec Spec) ui.Box {
	if t == nil {
		panic("transfer: View needs its Transfer")
	}
	t.reported, t.changed = t.changed, false
	height := spec.Height
	if height <= 0 {
		height = 240
	}
	moves := spec.MoveLabels
	if moves[0] == "" {
		moves[0] = "Move right"
	}
	if moves[1] == "" {
		moves[1] = "Move left"
	}
	theme := c.Theme()
	root := ui.Row(c).Items(ui.Center).Gap(theme.Metrics.GapItem)
	// The lists are made of controls of their own, so the row names the group
	// without a hit that would overlap theirs.
	root = root.Label(label)
	root.Children(func() {
		t.panel(c, spec, false, height)
		ui.Column(c).Gap(theme.Metrics.GapItem).Shrink(0).Children(func() {
			_, leftTicked, _ := stats(spec.Items, false)
			_, rightTicked, _ := stats(spec.Items, true)
			if ui.Button(c, moves[0]).Icon(rightIcon).IconOnly().Outline().Disabled(leftTicked == 0).Clicked() {
				t.move(spec.Items, false)
			}
			if ui.Button(c, moves[1]).Icon(leftIcon).IconOnly().Outline().Disabled(rightTicked == 0).Clicked() {
				t.move(spec.Items, true)
			}
		})
		t.panel(c, spec, true, height)
	})
	return root
}

// panel declares one list: its head with the tick for all, the title and the
// count, and the scrolling rows.
func (t *Transfer) panel(c ui.C, spec Spec, target bool, height float32) {
	theme := c.Theme()
	side := 0
	if target {
		side = 1
	}
	total, ticked, every := stats(spec.Items, target)
	t.all[side] = every
	box := ui.Column(c).Grow(1).Basis(0).Border(1, theme.Border).Radius(theme.Metrics.RadiusControl).Clip()
	box.Children(func() {
		head := ui.Row(c).FillWidth().Items(ui.Center).Gap(theme.Metrics.GapItem).Pad(theme.Metrics.GapItem)
		head.Children(func() {
			// The title is a label of the tick, as a row of words beside a control is.
			id := ui.ID(c)
			all := ui.Checkbox(c, &t.all[side], "").Hit(id, spec.Titles[side])
			if spec.Titles[side] != "" {
				ui.Text(c, spec.Titles[side]).LabelFor(id)
			}
			if all.Changed() {
				for i := range spec.Items {
					item := &spec.Items[i]
					if item.Target == target && !item.Disabled {
						item.Checked = t.all[side]
					}
				}
			}
			ui.Spacer(c).Grow(1)
			ui.Text(c, t.counts[side].set(ticked, total)).FontSize(theme.Metrics.FontSizeS).Muted()
		})
		ui.Spacer(c).FillWidth().Height(1).Grow(0).Shrink(0).Background(theme.Border)
		ui.Scroll(c).FillWidth().Height(height).Children(func() {
			ui.Column(c).FillWidth().Pad(theme.Metrics.GapItem).Gap(theme.Metrics.GapItem).Children(func() {
				for i := range spec.Items {
					item := &spec.Items[i]
					if item.Target != target {
						continue
					}
					// Keyed by the item's place in the caller's slice, so an item that moves
					// to the other list keeps its name.
					id := ui.ID(c.KeyInt(i))
					ui.Row(c).FillWidth().Items(ui.Center).Gap(theme.Metrics.GapItem).Children(func() {
						ui.Checkbox(c, &item.Checked, "").Hit(id, item.Title).Disabled(item.Disabled)
						title := ui.Text(c, item.Title).LabelFor(id).Grow(1)
						if item.Disabled {
							title.Muted()
						}
					})
				}
			})
		})
	})
}
