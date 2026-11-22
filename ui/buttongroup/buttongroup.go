// Package buttongroup is the opt-in page control for the joined button
// groups of package components. A page declares one with New and chained
// options, then views it:
//
//	group := m.group
//	group.Reset().
//		Item(ui.ID(c), "Left").Outline().
//		Item(ui.ID(c), "Center").Outline().
//		Item(ui.ID(c), "Right").Outline().
//		Selected(m.align)
//	group.OnClick(m.selectAlignment)
//	group.View(c, "Text alignment").FillWidth()
//
// The actions are painted by components.BeginButtonGroup, so they share their
// adjoining borders and only the group's outer corners are rounded. Each
// enabled action is its own Tab stop and a button to assistive technology; the
// runtime activates the focused action with Enter or Space. The group itself is
// a non-focusable group region named by View's label.
//
// The caller owns everything mutable. Item(...).Icon(icon) adds a leading glyph;
// IconOnly keeps the item's label as its accessible name and tooltip without
// painting it. Selected names the action painted with the secondary treatment
// and exposes that selection to assistive technology. A click does not change it:
// OnClick receives the action's index and identity, like the form control a page
// updates itself.
// Clicked reports the same pair for a caller that reads after the paint.
//
// Declarations are rebuilt every paint: call Reset, declare the actions in
// order, then View. The slices keep their capacity, so a warm paint allocates
// nothing. OnClick and the draw function are made once and kept. Do not copy a
// Group after View has painted it.
//
// It is a separate package so an application that does not import it pays
// nothing for it.
package buttongroup

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Variant is components.ButtonVariant, named here so a page can set a look
// dynamically with Variant without spelling the components package.
type Variant = components.ButtonVariant

// Group is one button group's declarations and its page control. Configure it
// with the chainable methods every paint, then call View.
type Group struct {
	items   []action
	options []components.ButtonGroupOption

	vertical    bool
	disabled    bool
	selected    int
	selectedSet bool

	onClick func(index int, id gfx.ID)

	drawFn   func(*ui.DrawContext)
	prepared bool

	clicked   int
	clickedID gfx.ID
	hadClick  bool
}

// action is one declared item. The zero Variant is the default look.
type action struct {
	id       gfx.ID
	label    string
	variant  components.ButtonVariant
	icon     gfx.Icon
	iconOnly bool
}

// New returns a button group with no actions and no selection.
func New() *Group { return new(Group) }

// Reset forgets the actions and the horizontal, enabled, unselected state of the
// last paint. It keeps OnClick and the prepared draw function, so a page that
// reconfigures the group every paint allocates nothing after the first.
func (g *Group) Reset() *Group {
	g.items = g.items[:0]
	g.options = g.options[:0]
	g.vertical = false
	g.disabled = false
	g.selected, g.selectedSet = 0, false
	g.clicked, g.clickedID, g.hadClick = 0, gfx.ID{}, false
	return g
}

// Item declares the next action. id is the caller-owned hit identity; a zero
// id declares an inert surface with no pointer or keyboard target.
func (g *Group) Item(id gfx.ID, label string) *Group {
	g.items = append(g.items, action{id: id, label: label, variant: components.ButtonDefault})
	return g
}

// Icon sets the leading glyph of the action declared last.
func (g *Group) Icon(icon gfx.Icon) *Group {
	if n := len(g.items); n > 0 {
		g.items[n-1].icon = icon
	}
	return g
}

// IconOnly hides the visible label of the action declared last, keeping it as
// the accessible name and tooltip. Give that action an Icon.
func (g *Group) IconOnly() *Group {
	if n := len(g.items); n > 0 {
		g.items[n-1].iconOnly = true
	}
	return g
}

// Variant sets the look of the action declared last.
func (g *Group) Variant(variant Variant) *Group {
	if n := len(g.items); n > 0 {
		g.items[n-1].variant = variant
	}
	return g
}

// Default sets the action declared last to the default look.
func (g *Group) Default() *Group { return g.Variant(components.ButtonDefault) }

// Destructive sets the action declared last to the quiet destructive look.
func (g *Group) Destructive() *Group { return g.Variant(components.ButtonDestructive) }

// Outline sets the action declared last to the outline look.
func (g *Group) Outline() *Group { return g.Variant(components.ButtonOutline) }

// Secondary sets the action declared last to the secondary look.
func (g *Group) Secondary() *Group { return g.Variant(components.ButtonSecondary) }

// Ghost sets the action declared last to the ghost look.
func (g *Group) Ghost() *Group { return g.Variant(components.ButtonGhost) }

// Link sets the action declared last to the link look.
func (g *Group) Link() *Group { return g.Variant(components.ButtonLink) }

// DestructiveOutline sets the action declared last to the destructive outline
// look.
func (g *Group) DestructiveOutline() *Group {
	return g.Variant(components.ButtonDestructiveOutline)
}

// Vertical stacks the actions, collapsing their adjoining horizontal borders.
func (g *Group) Vertical() *Group {
	g.vertical = true
	return g
}

// Horizontal lays the actions side by side. It is the default, and clears
// Vertical.
func (g *Group) Horizontal() *Group {
	g.vertical = false
	return g
}

// Disabled paints every action inert and takes the whole group out of the Tab
// order.
func (g *Group) Disabled() *Group {
	g.disabled = true
	return g
}

// Enabled clears Disabled, for a page that enables the group after a first
// paint.
func (g *Group) Enabled() *Group {
	g.disabled = false
	return g
}

// Selected paints the action at index with the secondary treatment, as the
// active choice of a toolbar. A negative index clears the selection. The
// wrapper does not change it on a click; OnClick or Clicked tells the page what
// was chosen, and the page declares Selected again on its next paint.
func (g *Group) Selected(index int) *Group {
	if index < 0 {
		g.selected, g.selectedSet = 0, false
		return g
	}
	g.selected, g.selectedSet = index, true
	return g
}

// SelectedIndex returns the index Selected named, or -1 when the selection is
// clear or names no declared action.
func (g *Group) SelectedIndex() int {
	if !g.selectedSet || g.selected < 0 || g.selected >= len(g.items) {
		return -1
	}
	return g.selected
}

// OnClick sets the callback a completed click on an action runs, with the
// action's index and identity. It runs after the paint has read the click, in
// the event pass, so it may change the page's state. A disabled group and a
// zero identity never run it.
func (g *Group) OnClick(fn func(index int, id gfx.ID)) *Group {
	g.onClick = fn
	return g
}

// Clicked reports the action whose click the last paint delivered, and false in
// every other paint or when the click was not on an enabled action.
func (g *Group) Clicked() (index int, id gfx.ID, ok bool) {
	if !g.hadClick {
		return -1, gfx.ID{}, false
	}
	return g.clicked, g.clickedID, true
}

// View declares the group at c. label names the group for assistive technology.
// Give the box a width; it is one control row high, or one row per action when
// the group is Vertical.
//
// Declare the view on its own, not inside a Memo.
func (g *Group) View(c ui.C, label string) ui.Box {
	if g == nil {
		panic("buttongroup: View needs its Group")
	}
	g.prepare()
	return ui.Slot(c).Label(label).Height(g.height(*c.Theme())).Draw(g.drawFn)
}

func (g *Group) prepare() {
	if g.prepared {
		return
	}
	g.prepared = true
	g.drawFn = g.draw
}

func (g *Group) height(theme gfx.Theme) float32 {
	height := theme.Metrics.ControlHeight
	if g.vertical && len(g.items) > 0 {
		height *= float32(len(g.items))
	}
	return height
}

func (g *Group) draw(d *ui.DrawContext) {
	theme := *d.Theme
	options := g.options[:0]
	selected := g.SelectedIndex()
	for index := range g.items {
		item := &g.items[index]
		variant := item.variant
		if index == selected {
			variant = components.ButtonSecondary
		}
		options = append(options, components.ButtonGroupOption{
			ID: item.id, Label: item.label, Variant: variant,
			Icon: item.icon, IconOnly: item.iconOnly, Selected: index == selected,
		})
	}
	g.options = options

	// The buttons are the group's Tab stops, so the group region itself is
	// semantic only: it carries the label and takes no focus from them.
	d.Canvas.ContainerWithOptions(d.Rect, gfx.RoleGroup, d.Label, gfx.HitOptions{})
	group := components.BeginButtonGroup(d.Canvas, theme, d.Rect, options)
	if g.vertical {
		group = group.Vertical()
	}
	if g.disabled {
		group = group.Disabled()
	}
	result := group.Paint()

	g.clicked, g.clickedID, g.hadClick = 0, gfx.ID{}, false
	if g.disabled {
		return
	}
	id, ok := result.Clicked()
	if !ok {
		return
	}
	index := g.indexOf(id)
	if index < 0 {
		return
	}
	g.clicked, g.clickedID, g.hadClick = index, id, true
	if g.onClick != nil {
		g.onClick(index, id)
	}
}

func (g *Group) indexOf(id gfx.ID) int {
	for index := range g.items {
		if g.items[index].id == id {
			return index
		}
	}
	return -1
}
