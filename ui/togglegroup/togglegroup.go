// Package togglegroup is the opt-in page control for the compact toggle
// groups of package components. A page keeps one Group in its model and
// configures it with chained options:
//
//	var m struct {
//		group     togglegroup.Group
//		alignment int
//	}
//
//	m.group.Items("Left", "Center", "Right").Single(&m.alignment).Outline()
//	m.group.View(c, "Text alignment").SizeToFit()
//
// Items declares the visible labels in one call, so configuring a group in each
// paint does not accumulate items. Item-level options (Disable, ItemID) are
// called after Items. Single binds a caller-owned selected index; Multiple
// binds a caller-owned slice of active bits. OnChange reports the item that
// changed, after the bound state has been written, for a caller that keeps its
// state somewhere else.
//
// Outline selects the bordered treatment, Small and Large the compact sizes,
// Spacing the logical pixels between items, SizeToFit sizes each item to its
// label, and Vertical lays the group out in a column. The default is a joined
// horizontal group: Spacing(0) shares borders and rounds only the outer
// corners, matching components.BeginToggleGroup. Disabled paints the whole
// group inert; Disable(index) disables one item.
//
// View declares a ui.Control whose parts are the items. The arrow keys move
// the roving focus, Home and End choose the first and last enabled item, and
// Enter and Space activate the focused item. In single-choice mode an arrow
// also selects the item it lands on, as a radio group does; in multiple-choice
// mode only activation toggles a bit. Keys aimed at an item reach the control,
// so an application that configures the page's OnKey as usual needs no
// group-specific key code.
//
// It is a separate package so an application that does not import it pays
// nothing for it. Keep a Group in the model: its handlers, draw function and
// option scratch are made once, so a warm paint allocates nothing. Do not copy
// a Group after it has been declared.
package togglegroup

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Group is one toggle group and its page control. Configure it with the
// chainable methods before each View, or once when the items are static.
type Group struct {
	labels   []string
	disabled []bool
	itemIDs  []gfx.ID

	multiple bool
	selected *int
	active   []bool
	// ownSelected and ownActive keep a callback-only group's state when the
	// caller did not bind a pointer or a slice.
	ownSelected int
	ownActive   []bool
	onChange    func(index int, active bool)

	outline  bool
	size     components.ToggleSize
	vertical bool
	spacing  float32
	fit      bool
	groupOff bool
	handlers ui.Handlers
	drawFn   func(*ui.DrawContext)
	prepared bool
	options  []components.ToggleGroupOption
}

// New returns an empty group. Add its items with Items; the group is joined and
// horizontal until another option says otherwise.
func New() *Group {
	return &Group{ownSelected: -1}
}

// Items sets the visible labels, in order. It replaces a previous item list
// rather than appending, so calling it in a paint is safe; item-level options
// called before it (Disable, ItemID, Active) are reset.
func (g *Group) Items(labels ...string) *Group {
	g.labels = append(g.labels[:0], labels...)
	count := len(g.labels)
	g.disabled = resizeBool(g.disabled, count)
	g.itemIDs = resizeIDs(g.itemIDs, count)
	g.ownActive = resizeBool(g.ownActive, count)
	clear(g.disabled)
	clear(g.itemIDs)
	clear(g.ownActive)
	return g
}

// Single selects single-choice mode and binds the selected index. A value
// outside the item list (including -1) means no selection. The group writes the
// index on a click or activation. A nil pointer leaves the selection in the
// group itself, which OnChange still reports.
func (g *Group) Single(selected *int) *Group {
	g.multiple, g.selected = false, selected
	return g
}

// Multiple selects multiple-choice mode and binds the active bits. The group
// toggles active[index] on a click or activation; a slice shorter than the item
// list leaves its missing entries inactive and is not grown.
func (g *Group) Multiple(active []bool) *Group {
	g.multiple, g.active = true, active
	return g
}

// OnChange keeps a handler for an item that changed: its index and its new
// active state. Single-choice mode reports true; multiple-choice mode reports
// the bit's new value. The handler runs after the bound index or slice was
// written, and is not called when an activation leaves the state unchanged.
func (g *Group) OnChange(handler func(index int, active bool)) *Group {
	g.onChange = handler
	return g
}

// Active marks item index active in the group's own state. It is for a group
// with no bound pointer or slice (a callback-only group); a bound value takes
// precedence. In single-choice mode it selects the item, in multiple-choice
// mode it sets one bit. Call Items first.
func (g *Group) Active(index int) *Group {
	if g.multiple {
		if index >= 0 && index < len(g.ownActive) {
			g.ownActive[index] = true
		}
		return g
	}
	g.ownSelected = index
	return g
}

// Disable paints item index inert: the pointer and the keys skip it. Call Items
// first.
func (g *Group) Disable(index int) *Group {
	if index >= 0 && index < len(g.disabled) {
		g.disabled[index] = true
	}
	return g
}

// Disabled paints the whole group inert, including every item. A disabled
// group cannot be selected or toggled.
func (g *Group) Disabled() *Group {
	g.groupOff = true
	return g
}

// Outline selects the bordered item variant. With the default joined spacing,
// neighbouring outline items share a border and only the group's outer corners
// are rounded.
func (g *Group) Outline() *Group {
	g.outline = true
	return g
}

// Small selects the compact item size.
func (g *Group) Small() *Group {
	g.size = components.ToggleSizeSmall
	return g
}

// Large selects the large item size.
func (g *Group) Large() *Group {
	g.size = components.ToggleSizeLarge
	return g
}

// Vertical lays the items out in a column instead of a row.
func (g *Group) Vertical() *Group {
	g.vertical = true
	return g
}

// Spacing sets the logical pixels between items. Zero, the default, joins the
// items into one connected control; a positive spacing paints each item with
// its own rounded surface.
func (g *Group) Spacing(spacing float32) *Group {
	if spacing < 0 {
		spacing = 0
	}
	g.spacing = spacing
	return g
}

// SizeToFit sizes every item to its label instead of splitting the width View
// was given, matching components.ToggleGroup.SizeToFit. View then declares the
// group as wide as its items need.
func (g *Group) SizeToFit() *Group {
	g.fit = true
	return g
}

// ItemID names item index's hit explicitly instead of the control's own part.
// It is for an embedding runner that resolves items by an application identity,
// such as a component preview. Such an identity is outside the control's parts,
// so the page does not route its keys to the control; a runner that wants key
// handling for explicitly named items calls Key itself. Call Items first.
func (g *Group) ItemID(index int, id gfx.ID) *Group {
	if index >= 0 && index < len(g.itemIDs) {
		g.itemIDs[index] = id
	}
	return g
}

// View declares the group at c. label names the control for assistive
// technology. The box is one item tall, or as tall as the column for a vertical
// group; a horizontal group fills the width unless SizeToFit gives it a natural
// one. Chain Width or MaxWidth after View to bound a vertical group.
//
// Declare the view on its own, not inside a Memo.
func (g *Group) View(c ui.C, label string) ui.Box {
	if g == nil {
		panic("togglegroup: View needs its Group")
	}
	g.prepare()
	box := ui.Control(c, label, &g.handlers)
	// The items are the focusable stops; the group itself is a semantic
	// container, not another stop of the Tab order.
	box = box.HitWith(box.N().ID, label, gfx.HitOptions{Role: gfx.RoleGroup, SkipFocus: true})
	height := controlHeight(*c.Theme(), g.size)
	count := len(g.labels)
	switch {
	case g.vertical && count > 0:
		box = box.Height(height*float32(count) + g.spacing*float32(count-1))
	case !g.vertical && count > 0:
		box = box.Height(height)
	}
	if g.fit && count > 0 && c.Canvas() != nil {
		box = box.Width(g.fittedWidth(c.Canvas(), *c.Theme()))
	} else {
		box = box.FillWidth()
	}
	return box.Draw(g.drawFn)
}

// Key routes a key event to the group: arrows, Home and End move the roving
// focus, and Enter and Space activate the item the event targets. View's
// control calls it for keys aimed at the group's parts; an embedding runner
// that names items with ItemID may call it directly.
func (g *Group) Key(event gfx.KeyEvent) gfx.EventResult {
	if g == nil || event.Release || event.Target.IsZero() {
		return gfx.EventResult{}
	}
	g.syncOptions()
	index := g.index(event.Target)
	if index < 0 {
		return gfx.EventResult{}
	}
	if event.Key == gfx.KeyEnter || event.Key == gfx.KeySpace {
		if g.groupOff || g.itemDisabled(index) {
			return gfx.EventResult{}
		}
		return gfx.EventResult{Handled: true, Repaint: g.activate(index)}
	}
	orientation := components.ToggleGroupHorizontal
	if g.vertical {
		orientation = components.ToggleGroupVertical
	}
	next, handled := components.ToggleGroupKeyDown(event.Target, g.options, orientation, components.DirectionLTR, event.Key)
	if !handled {
		return gfx.EventResult{}
	}
	result := gfx.EventResult{Handled: true, Focus: next, FocusVisible: true}
	if next != event.Target && !g.multiple {
		if nextIndex := g.index(next); nextIndex >= 0 && g.activate(nextIndex) {
			result.Repaint = true
		}
	}
	return result
}

func (g *Group) prepare() {
	if g.prepared {
		return
	}
	g.prepared = true
	g.handlers.Role = gfx.RoleGroup
	g.handlers.Key = g.key
	g.drawFn = g.draw
}

func (g *Group) key(event gfx.KeyEvent) gfx.EventResult { return g.Key(event) }

func (g *Group) draw(d *ui.DrawContext) {
	g.syncOptions()
	group := components.BeginToggleGroup(d.Canvas, *d.Theme, d.Rect, g.options)
	if g.outline {
		group = group.Outline()
	}
	switch g.size {
	case components.ToggleSizeSmall:
		group = group.Small()
	case components.ToggleSizeLarge:
		group = group.Large()
	}
	group = group.Spacing(g.spacing)
	if g.vertical {
		group = group.Vertical()
	}
	if g.fit {
		group = group.SizeToFit()
	}
	if g.groupOff {
		group = group.Disabled()
	}
	if id, ok := group.Paint().Clicked(); ok {
		if index := g.index(id); index >= 0 {
			g.activate(index)
		}
	}
}

// syncOptions fills the reusable option slice from the caller's state. It
// allocates only when the item count grows.
func (g *Group) syncOptions() {
	count := len(g.labels)
	if cap(g.options) < count {
		g.options = make([]components.ToggleGroupOption, count)
	} else {
		g.options = g.options[:count]
	}
	for index := range g.labels {
		option := &g.options[index]
		option.ID = g.itemID(index)
		option.Label = g.labels[index]
		option.Active = g.isActive(index)
		option.Disabled = g.itemDisabled(index)
	}
}

func (g *Group) itemID(index int) gfx.ID {
	if index < len(g.itemIDs) && !g.itemIDs[index].IsZero() {
		return g.itemIDs[index]
	}
	return g.handlers.Parts().At(index)
}

func (g *Group) itemDisabled(index int) bool {
	return index >= 0 && index < len(g.disabled) && g.disabled[index]
}

func (g *Group) isActive(index int) bool {
	if g.multiple {
		return g.activeAt(index)
	}
	return g.selectedIndex() == index
}

func (g *Group) selectedIndex() int {
	if g.selected != nil {
		return *g.selected
	}
	return g.ownSelected
}

func (g *Group) activeAt(index int) bool {
	if g.active != nil {
		return index >= 0 && index < len(g.active) && g.active[index]
	}
	return index >= 0 && index < len(g.ownActive) && g.ownActive[index]
}

func (g *Group) setSelected(index int) {
	if g.selected != nil {
		*g.selected = index
		return
	}
	g.ownSelected = index
}

func (g *Group) setActive(index int, active bool) {
	if g.active != nil {
		if index >= 0 && index < len(g.active) {
			g.active[index] = active
		}
		return
	}
	if index >= 0 && index < len(g.ownActive) {
		g.ownActive[index] = active
	}
}

// activate selects or toggles index, then reports it. It returns false when the
// group or item is disabled, or when the single-choice selection would not
// change.
func (g *Group) activate(index int) bool {
	if g.groupOff || index < 0 || index >= len(g.labels) || g.itemDisabled(index) {
		return false
	}
	if g.multiple {
		active := !g.activeAt(index)
		g.setActive(index, active)
		if g.onChange != nil {
			g.onChange(index, active)
		}
		return true
	}
	if g.selectedIndex() == index {
		return false
	}
	g.setSelected(index)
	if g.onChange != nil {
		g.onChange(index, true)
	}
	return true
}

func (g *Group) index(id gfx.ID) int {
	if id.IsZero() {
		return -1
	}
	for index := range g.options {
		if g.options[index].ID == id {
			return index
		}
	}
	return -1
}

// controlHeight mirrors the component's item height for View's layout.
func controlHeight(theme gfx.Theme, size components.ToggleSize) float32 {
	height := theme.Metrics.ControlHeight
	switch size {
	case components.ToggleSizeSmall:
		height -= 4
	case components.ToggleSizeLarge:
		height += 4
	}
	if height < 20 {
		height = 20
	}
	return height
}

// fittedWidth is the width the component's SizeToFit gives the group.
func (g *Group) fittedWidth(canvas *gfx.Canvas, theme gfx.Theme) float32 {
	height := controlHeight(theme, g.size)
	padding := float32(10)
	if g.spacing == 0 {
		padding = 8
	}
	itemWidth := func(label string) float32 {
		return max(canvas.TextWidthWeight(label, theme.Metrics.FontSizeM, gfx.WeightMedium)+padding*2, height)
	}
	if g.vertical {
		var widest float32
		for _, label := range g.labels {
			widest = max(widest, itemWidth(label))
		}
		return widest
	}
	total := g.spacing * float32(len(g.labels)-1)
	for _, label := range g.labels {
		total += itemWidth(label)
	}
	return total
}

func resizeBool(slice []bool, length int) []bool {
	if cap(slice) < length {
		return make([]bool, length)
	}
	return slice[:length]
}

func resizeIDs(slice []gfx.ID, length int) []gfx.ID {
	if cap(slice) < length {
		return make([]gfx.ID, length)
	}
	return slice[:length]
}
