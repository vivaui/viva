package ui

import (
	"unsafe"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/keymap"
)

// Menu shows a list of actions hanging off anchor while *open, as shadcn's
// DropdownMenu: the items body declares on m, one under the other, with the
// arrows moving between them, Enter or a click choosing one, and Escape, Tab or
// a press outside closing it. Choosing an item closes the menu; the focus goes
// back to the anchor.
//
//	more := ui.Button(c, "More").Outline()
//	if more.Clicked() {
//		m.menu = !m.menu
//	}
//	ui.Menu(c, more, &m.menu, func(menu *ui.MenuItems) {
//		if menu.Item("Rename").Clicked() {
//			m.rename()
//		}
//		menu.Separator()
//		if menu.Item("Delete").Destructive().Clicked() {
//			m.delete()
//		}
//	})
//
// Not inside a Memo.
func Menu(c C, anchor Box, open *bool, body func(*MenuItems)) {
	p := c.page
	p.forbidInMemo("Menu")
	if !*open {
		return
	}
	scope := p.boundScope(pkMenu, unsafe.Pointer(open))
	menu := stateIn[MenuItems](p, scope, "menu")
	menu.context = false
	p.declareMenu(c, menu, scope, open, anchor.i, body)
}

// ContextMenu shows the items body declares at the pointer when target is pressed
// with the secondary button, or at the target when it has the focus and Shift+F10
// is pressed, as shadcn's ContextMenu. It behaves as a Menu then, but Tab does not
// close it. Whether it is open is its own state. target is a control, or a
// container given Hit.
//
//	row := ui.Row(c.KeyInt(h.ID)).Hit(ui.ID(c), h.Name).Children(...)
//	ui.ContextMenu(c, row, func(menu *ui.MenuItems) {
//		if menu.Item("Connect").Clicked() { m.connect(h) }
//	})
//
// Not inside a Memo.
func ContextMenu(c C, target Box, body func(*MenuItems)) {
	p := c.page
	p.forbidInMemo("ContextMenu")
	id := target.n().id
	if id.IsZero() {
		panic("ui: ContextMenu needs a target with an identity: a control, or a container given Hit")
	}
	scope := p.tree.Sub(id.Scope(), id.Name()) // the target's own: a ContextMenu per target
	menu := stateIn[MenuItems](p, scope, "context")
	menu.context = true
	if canvas := c.f.canvas; canvas != nil {
		if event, ok := canvas.PointerOn(id); ok && event.Kind == gfx.PointerDown && event.Button == gfx.PointerButtonSecondary {
			menu.shown, menu.at = true, Rect{X: event.X, Y: event.Y}
			canvas.Consume()
		}
	}
	p.contexts = append(p.contexts, contextTarget{id: id, menu: menu, f: c.f, target: target.i})
	if p.menuRoute == nil {
		p.keyRoute, p.menuRoute = (*Page).routeKey, (*Page).menuKey
		p.wheelRoute = (*Page).overlayWheel
	}
	if menu.shown {
		p.declareMenu(c, menu, scope, &menu.shown, -1, body)
	}
}

// contextTarget is a ContextMenu declared in the last paint, which Shift+F10 on its
// target opens.
type contextTarget struct {
	id     gfx.ID
	menu   *MenuItems
	f      *Frame
	target int32
}

// declareMenu declares an open menu: its rows, its size, and the popup that places
// it, under the anchor element or, with anchor -1, at menu.at.
func (p *Page) declareMenu(c C, menu *MenuItems, scope gfx.Scope, open *bool, anchor int32, body func(*MenuItems)) {
	p.overlay = true
	if menu.paint == nil {
		menu.paint = menu.draw // made once: a method value made in each paint allocates
	}
	f := c.f
	menu.page, menu.f, menu.open, menu.anchor = p, f, open, anchor
	menu.scope = scope
	menu.layer = f.widgets.layer + 2 // over the page, and over a dialog it is declared in
	menu.items = menu.items[:0]
	body(menu)
	if f.canvas != nil && f.canvas.Clicked(scope.At(menuDismiss)) {
		*open = false
		return
	}
	theme := &f.theme
	width, height := menu.minWidth(), 2*components.DropdownMenuPadding
	for i := range menu.items {
		item := &menu.items[i]
		switch item.kind {
		case menuRowSeparator:
			height += components.DropdownMenuSeparatorHeight
		case menuRowLabel:
			height += components.DropdownMenuLabelHeight
		default:
			row := menu.row(i)
			if menu.context {
				context := contextItem(row)
				height += components.ContextMenuItemHeight(*theme, context, 0)
				if f.canvas != nil {
					width = max(width, components.ContextMenuItemWidth(f.canvas, *theme, context))
				}
				continue
			}
			height += components.DropdownMenuItemHeight(*theme, row)
			if f.canvas != nil {
				width = max(width, components.DropdownMenuItemWidth(f.canvas, *theme, row))
			}
		}
	}
	popup := f.Popup()
	if anchor >= 0 {
		popup.BelowBox(Box{f: f, i: anchor}, 4, Start)
	} else {
		popup.At(menu.at.X, menu.at.Y)
	}
	popup.Size(width, height).Within(f.bounds.Inset(gfx.All(components.DropdownMenuPadding)))
	popup.Draw(menu.paint)
	p.menus = append(p.menus, menu)
	if p.menuRoute == nil {
		p.keyRoute, p.menuRoute = (*Page).routeKey, (*Page).menuKey
		p.wheelRoute = (*Page).overlayWheel
	}
}

// The identities of a Menu, in the scope its open bool names: its focus scope,
// its dismissal, its scrollbar, then its rows.
const (
	menuFocus = iota
	menuDismiss
	menuBar
	menuRows
)

type menuRowKind uint8

const (
	menuRowItem menuRowKind = iota
	menuRowLabel
	menuRowSeparator
)

// MenuItems is an open Menu or ContextMenu being declared: what its rows are, and,
// between paints, where it is scrolled to.
type MenuItems struct {
	items  []menuRow
	scroll components.MenuScroll
	paint  func(*DrawContext)
	keys   [6]string // the keycaps of the row being measured or painted
	page   *Page
	f      *Frame
	open   *bool
	anchor int32 // the element a Menu hangs off; -1 for a ContextMenu
	scope  gfx.Scope
	layer  uint16

	context bool // a ContextMenu: its own look, and Tab does not close it
	shown   bool // a ContextMenu is open
	at      Rect // where a ContextMenu opened
}

type menuRow struct {
	kind  menuRowKind
	label string
	icon  gfx.Icon
	// The shortcut the row shows, a keymap.Chord's fields: the Page's type would
	// keep that type's methods in every program, Menu or not.
	modifiers   gfx.Modifiers
	char        byte
	key         gfx.Key
	destructive bool
	disabled    bool
	checked     int8 // 1 a checked checkbox, 2 an unchecked one, 3 a chosen radio, 4 an unchosen one
}

// minWidth is shadcn's: 8rem for a DropdownMenu, 9rem for a ContextMenu.
func (m *MenuItems) minWidth() float32 {
	if m.context {
		return 144
	}
	return 128
}

// row is row i as the components paint it. Its Keys are the MenuItems' scratch,
// good until the next row.
func (m *MenuItems) row(i int) components.DropdownMenuItem {
	row := &m.items[i]
	item := components.DropdownMenuItem{ID: m.scope.At(menuRows + i), Label: row.label, Disabled: row.disabled, Icon: row.icon}
	if row.destructive {
		item.Variant = components.DropdownMenuItemDestructive
	}
	if chord := (keymap.Chord{Modifiers: row.modifiers, Char: row.char, Key: row.key}); chord.Valid() {
		item.Keys = chord.Keycaps(m.keys[:0])
	}
	switch row.checked {
	case 1, 2:
		item.Kind, item.Checked = components.DropdownMenuCheckbox, row.checked == 1
	case 3, 4:
		item.Kind, item.Checked = components.DropdownMenuRadio, row.checked == 3
	}
	item.Selected = item.Checked
	return item
}

// MenuItem is an item of a Menu. Set its look and state before Clicked.
type MenuItem struct {
	menu *MenuItems
	i    int
}

// Item adds an action named label.
func (m *MenuItems) Item(label string) MenuItem {
	m.items = append(m.items, menuRow{label: label})
	return MenuItem{menu: m, i: len(m.items) - 1}
}

// Label adds a heading over the items that follow it.
func (m *MenuItems) Label(label string) {
	m.items = append(m.items, menuRow{kind: menuRowLabel, label: label})
}

// Separator adds a line between groups of items.
func (m *MenuItems) Separator() {
	m.items = append(m.items, menuRow{kind: menuRowSeparator})
}

// Destructive paints the item in the destructive color: an action that deletes.
func (item MenuItem) Destructive() MenuItem {
	item.menu.items[item.i].destructive = true
	return item
}

// Disabled dims the item and keeps it from being chosen; the arrows pass over it.
func (item MenuItem) Disabled(disabled bool) MenuItem {
	item.menu.items[item.i].disabled = disabled
	return item
}

// Icon paints icon before the item's label.
func (item MenuItem) Icon(icon gfx.Icon) MenuItem {
	item.menu.items[item.i].icon = icon
	return item
}

// Keys shows the keyboard shortcut of the item's action at its end, in the
// platform's keycaps (⌘S on macOS, Ctrl+S elsewhere). It only shows it: the menu
// is not declared while closed, so the shortcut itself is a Shortcut declared
// where the action is.
func (item MenuItem) Keys(modifiers gfx.Modifiers, key string) MenuItem {
	chord, ok := keymap.Parse(modifiers, key)
	if !ok {
		panic("ui: MenuItem.Keys " + key + ": not a key keymap.Parse names, nor one character")
	}
	row := &item.menu.items[item.i]
	row.modifiers, row.char, row.key = chord.Modifiers, chord.Char, chord.Key
	return item
}

// Checked makes the item a checkbox, checked or not: a setting it toggles.
func (item MenuItem) Checked(on bool) MenuItem {
	item.menu.items[item.i].checked = 2
	if on {
		item.menu.items[item.i].checked = 1
	}
	return item
}

// Chosen makes the item one of a radio group, chosen or not: one value of several.
func (item MenuItem) Chosen(on bool) MenuItem {
	item.menu.items[item.i].checked = 4
	if on {
		item.menu.items[item.i].checked = 3
	}
	return item
}

// Clicked reports whether this paint delivers the choice of the item, by a click
// or Enter, and closes the menu then.
func (item MenuItem) Clicked() bool {
	m := item.menu
	canvas := m.f.canvas
	if canvas == nil || m.items[item.i].disabled || !canvas.Clicked(m.scope.At(menuRows+item.i)) {
		return false
	}
	*m.open = false
	return true
}

// draw paints the open menu in the popup the page placed for it.
func (m *MenuItems) draw(d *DrawContext) {
	f, scope := m.f, m.scope
	m.scroll.BarID = scope.At(menuBar)
	layer := max(m.layer, f.popupLayer)
	if m.context {
		menu := components.BeginContextMenu(d.Canvas, *d.Theme, d.Rect, components.ContextMenuOptions{
			Layer: layer, Scope: scope.At(menuFocus), DismissID: scope.At(menuDismiss), Viewport: f.bounds,
			Scroll: &m.scroll,
		})
		for i := range m.items {
			switch row := &m.items[i]; row.kind {
			case menuRowSeparator:
				menu.AddSeparator()
			case menuRowLabel:
				menu.AddLabel(row.label, false)
			default:
				menu.AddItem(contextItem(m.row(i)))
			}
		}
		menu.End()
		return
	}
	side := components.OverlayBottom
	if anchor := f.nodes[m.anchor].rect; d.Rect.Y+d.Rect.Height <= anchor.Y+anchor.Height/2 {
		side = components.OverlayTop
	}
	menu := components.BeginDropdownMenu(d.Canvas, *d.Theme, d.Rect, components.DropdownMenuOptions{
		Layer: layer, Scope: scope.At(menuFocus), DismissID: scope.At(menuDismiss), Viewport: f.bounds,
		Scroll: &m.scroll, Side: side,
	})
	for i := range m.items {
		switch row := &m.items[i]; row.kind {
		case menuRowSeparator:
			menu.AddSeparator()
		case menuRowLabel:
			menu.AddLabel(row.label, false)
		default:
			menu.AddItem(m.row(i))
		}
	}
	menu.End()
}

// contextItem is a dropdown row as a context menu's: the two describe one row.
func contextItem(item components.DropdownMenuItem) components.ContextMenuItem {
	context := components.ContextMenuItem{
		ID: item.ID, Label: item.Label, Keys: item.Keys, Icon: item.Icon, Disabled: item.Disabled,
		Checked: item.Checked, Selected: item.Selected,
	}
	switch item.Kind {
	case components.DropdownMenuCheckbox:
		context.Kind = components.ContextMenuCheckbox
	case components.DropdownMenuRadio:
		context.Kind = components.ContextMenuRadio
	}
	if item.Variant == components.DropdownMenuItemDestructive {
		context.Variant = components.ContextMenuItemDestructive
	}
	return context
}

// menuKey moves between the items of an open Menu with the arrows and closes a
// Menu on Tab, from its items or from its anchor; Shift+F10 on the target of a
// ContextMenu opens it.
func (p *Page) menuKey(event gfx.KeyEvent) (gfx.EventResult, bool) {
	for i, m := range p.menus {
		// The keys are the menu's from its items, from its anchor, and, for the
		// last one opened, from nowhere: its focus trap takes the focus from the
		// anchor as it opens, and the first arrow goes into it.
		var trigger gfx.ID
		if m.anchor >= 0 {
			trigger = m.f.nodes[m.anchor].id
		}
		_, inside := event.Target.In(m.scope)
		last := i == len(p.menus)-1 && event.Target.IsZero()
		if !inside && !last && (event.Target != trigger || trigger.IsZero()) {
			continue
		}
		if event.Key == gfx.KeyTab && !event.Release && !m.context {
			*m.open = false
			return gfx.EventResult{Handled: true, Repaint: true, ContinueTraversal: true}, true
		}
		if id, ok := m.scroll.Navigate(event.Key, event.Target, true); ok {
			return gfx.EventResult{Handled: true, Repaint: true, Focus: id, FocusVisible: true}, true
		}
	}
	if event.Key == gfx.KeyF10 && event.Modifiers == gfx.ModifierShift && !event.Release {
		for _, target := range p.contexts {
			if target.id == event.Target && !target.menu.shown {
				r := target.f.nodes[target.target].rect
				target.menu.shown, target.menu.at = true, Rect{X: r.X, Y: r.Y + r.Height}
				return gfx.EventResult{Handled: true, Repaint: true}, true
			}
		}
	}
	return gfx.EventResult{}, false
}

// menuWheel scrolls an open Menu the wheel is over.
func (p *Page) menuWheel(event gfx.ScrollEvent) (gfx.EventResult, bool) {
	for _, m := range p.menus {
		if _, inside := event.Target.In(m.scope); inside {
			return gfx.EventResult{Handled: true, Repaint: m.scroll.Wheel(event)}, true
		}
	}
	return gfx.EventResult{}, false
}
