package ui

import (
	"time"
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
//		menu.Sub("Move to", func(to *ui.MenuItems) {
//			for _, g := range m.groups {
//				if to.Item(g.Name).Clicked() {
//					m.move(g)
//				}
//			}
//		})
//		menu.Separator()
//		if menu.Item("Delete").Destructive().Clicked() {
//			m.delete()
//		}
//	})
//
// An item may carry a second line with Description; WidthRange bounds the menu's
// width; Above opens it over the anchor, for a trigger along the bottom of a panel,
// and Beside to its right, for a row of a list whose actions should not cover the
// rows under it.
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
	p.declareMenu(c, menu, scope, open, anchor.i, nil, body)
}

// ContextMenu shows the items body declares at the pointer when target is pressed
// with the secondary button, or at the target when it has the focus and Shift+F10
// is pressed, as shadcn's ContextMenu. It behaves as a Menu then, but Tab does not
// close it. Whether it is open is its own state. target is a control, a
// container given Hit, or a LabelFor element, whose menu opens on the control
// it names.
//
//	row := ui.Row(c.KeyInt(h.ID)).Hit(ui.ID(c), h.Name).Children(...)
//	ui.ContextMenu(c, row, func(menu *ui.MenuItems) {
//		if menu.Item("Connect").Clicked() { m.connect(h) }
//	})
//
// Not inside a Memo.
func ContextMenu(c C, target Box, body func(*MenuItems)) {
	contextMenu(c, target, gfx.Scope(0), nil, body)
}

// ContextMenuAt shows a context menu at a point the application captured on its
// own surface: a canvas cell, a column header, or a text editor. Unlike Menu, its
// row layout is a ContextMenu's, and a menu that reaches the window edge clamps
// its origin rather than flipping to the other side of the pointer. target names
// the control whose focus and secondary-press token the menu takes; open owns
// dismissal, just as it does for Menu.
//
// Not inside a Memo.
func ContextMenuAt(c C, target Box, open *bool, at gfx.Point, body func(*MenuItems)) {
	if !*open {
		return
	}
	contextMenu(c, target, c.page.boundScope(pkMenu, unsafe.Pointer(open)), &menuOrigin{open: open, at: Rect{X: at.X, Y: at.Y}}, body)
}

func (m *MenuItems) RowHeight(height float32) { m.rowSize = height }

func contextMenu(c C, target Box, bound gfx.Scope, opened *menuOrigin, body func(*MenuItems)) {
	p := c.page
	p.forbidInMemo("ContextMenu")
	id := target.n().id
	if id.IsZero() {
		panic("ui: ContextMenu needs a target with an identity: a control, or a container given Hit")
	}
	scope := p.tree.Sub(id.Scope(), id.Name()) // the target's own: a ContextMenu per target
	if !bound.IsZero() {
		scope = bound
	}
	menu := stateIn[MenuItems](p, scope, "context")
	menu.context = true
	p.contexts = append(p.contexts, contextTarget{id: id, menu: menu, f: c.f, target: target.i})
	if p.menuRoute == nil {
		p.keyRoute, p.menuRoute = (*Page).routeKey, (*Page).menuKey
		p.wheelRoute = (*Page).overlayWheel
	}
	if canvas := c.f.canvas; canvas != nil && opened == nil {
		if event, ok := canvas.PointerOn(id); ok && event.Kind == gfx.PointerDown && event.Button == gfx.PointerButtonSecondary {
			menu.shown, menu.at = true, Rect{X: event.X, Y: event.Y}
			canvas.Consume()
		}
	}
	if opened != nil {
		menu.open, menu.at = opened.open, opened.at
	}
	if opened != nil || menu.shown {
		open := menu.open
		if open == nil {
			// A context menu the target opened has no *bool of its own: the
			// pointer opened it, and the chosen row (or a dismissal) closes it.
			open = &menu.shown
		}
		if bound.IsZero() {
			// The target's scope holds the identities inside it: the parts of a
			// Control (Handlers.Parts) and the children of a container. The rows, the
			// backdrop, the scroll bar and the focus trap of the menu take a place of
			// their own under it, or a press on a row would reach the control as a
			// press on one of its parts.
			scope = p.tree.Sub(scope, p.menuName)
		}
		p.declareMenu(c, menu, scope, open, -1, opened, body)
	}
}

// menuOrigin is the captured open state of a context menu on a custom surface.
// The application owns when it opens; MenuItems owns the rows and dismissal.
type menuOrigin struct {
	open *bool
	at   Rect
}

// contextTarget is a ContextMenu declared in the last paint, which Shift+F10 on its
// target opens.
type contextTarget struct {
	id     gfx.ID
	menu   *MenuItems
	f      *frame
	target int32
}

// declareMenu declares an open menu: its rows, its size, and the popup that places
// it, under the anchor element or, with anchor -1, at menu.at.
func (p *Page) declareMenu(c C, menu *MenuItems, scope gfx.Scope, open *bool, anchor int32, opened *menuOrigin, body func(*MenuItems)) {
	p.overlay = true
	if menu.paint == nil {
		menu.paint = menu.draw // made once: a method value made in each paint allocates
	}
	if menu.itemPainter == nil {
		menu.itemPainter, menu.panelPainter = menu.paintItem, menu.paintPanel
	}
	f := c.f
	menu.page, menu.f, menu.open, menu.anchor = p, f, open, anchor
	menu.scope, menu.root, menu.parent = scope, menu, nil
	if opened != nil {
		menu.at = opened.at
	}
	menu.layer = f.widgetLayer() + 2 // over the page, and over a dialog it is declared in
	menu.side = menuBelow            // unless body says otherwise again in this paint
	menu.fresh()
	menu.declare(body)
	if f.canvas != nil && f.canvas.Clicked(scope.At(menuDismiss)) {
		*open = false
		return
	}
	width, height := menu.size()
	popup := f.Popup()
	if anchor >= 0 {
		anchorBox := Box{f: f, i: anchor, gen: f.gen}
		switch menu.side {
		case menuAbove:
			popup.AboveBox(anchorBox, 4, Start)
		case menuBeside:
			popup.BesideBox(anchorBox, 4, Start)
		default:
			popup.BelowBox(anchorBox, 4, Start)
		}
	} else {
		height = min(height, max(f.bounds.Height-2*components.DropdownMenuPadding, 0))
		clamped := components.ClampOverlay(gfx.Point{X: menu.at.X, Y: menu.at.Y}, gfx.Size{Width: width, Height: height}, f.bounds, components.DropdownMenuPadding)
		popup.At(clamped.X, clamped.Y)
	}
	popup.Size(width, height).Within(f.bounds.Inset(gfx.All(components.DropdownMenuPadding)))
	popup.Draw(menu.paint)
	if menu.hasPageContent() {
		// The page row is a child of the popup: it paints and takes hits in the
		// menu's layer and focus scope, over the canvas rows the menu drew under it.
		if menu.layer > f.popupLayer {
			f.popupLayer = menu.layer
		}
		f.nodes[popup.i].focusScope = menu.scope.At(menuFocus)
		popup.Children(func() { menu.declarePageRows(c, width, height) })
	}
	p.menus = append(p.menus, menu)
	if p.menuRoute == nil {
		p.keyRoute, p.menuRoute = (*Page).routeKey, (*Page).menuKey
		p.wheelRoute = (*Page).overlayWheel
	}
}

// hasPageContent reports whether any row is declared in the page.
func (m *MenuItems) hasPageContent() bool {
	for i := range m.items {
		if m.items[i].kind == menuRowPageContent && m.items[i].pageContent != nil {
			return true
		}
	}
	return false
}

// declarePageRows declares the page controls of every page content row at the
// place the menu's rows and scroll put them.
func (m *MenuItems) declarePageRows(c C, width, height float32) {
	pad := float32(components.DropdownMenuPadding)
	y := pad - m.scroll.Offset
	for i := range m.items {
		row := &m.items[i]
		if row.kind == menuRowPageContent && row.pageContent != nil && row.height > 0 &&
			y+row.height > 0 && y < height {
			pageY, pageHeight := y, row.height
			bounds := Rect{X: pad, Y: pageY, Width: max(width-2*pad, 0), Height: pageHeight}
			Column(c).Absolute().Left(bounds.X).Top(bounds.Y).Width(bounds.Width).Height(bounds.Height).Children(func() {
				row.pageContent(c, bounds)
			})
		}
		y += m.rowExtent(i)
	}
}

// fresh forgets the submenu a menu had open the last time it was: one that opens
// again opens with none.
func (m *MenuItems) fresh() {
	if m.seen+1 != m.page.epoch {
		m.sub, m.subMenu, m.intent = -1, nil, components.HoverIntent{}
	}
	m.unpainted, m.drawPanel, m.drawItem = false, nil, nil
	m.seen = m.page.epoch
}

// declare declares the menu's rows with body, and, as the pointer rests on its
// rows, opens or closes its submenu.
func (m *MenuItems) declare(body func(*MenuItems)) {
	m.items = m.items[:0]
	m.subMenu = nil
	body(m)
	if m.sub >= 0 && (m.sub >= len(m.items) || !m.items[m.sub].submenu || m.items[m.sub].disabled) {
		m.sub, m.subMenu = -1, nil // its row is gone, or disabled after Sub
	}
	m.hover()
}

// submenuDelay is how long the pointer rests on a row before the submenu it opens or
// the one it closes follows, Base UI's: a pointer on its way to an open submenu
// crosses other rows without closing it.
const submenuDelay = 100 * time.Millisecond

// hover opens the submenu of the row the pointer rests on, or closes the open one
// when it rests on another row.
func (m *MenuItems) hover() {
	canvas := m.f.canvas
	if canvas == nil {
		return
	}
	at, inside := canvas.Hovered().In(m.scope)
	row := at - menuRows
	if !inside || row < 0 || row >= len(m.items) || row == m.sub || m.items[row].disabled ||
		!m.items[row].submenu && m.sub < 0 {
		m.intent.Reset()
		return
	}
	now := clockEpoch.Add(canvas.Now())
	if _, wait := m.intent.Rest(m.scope.At(at), now, submenuDelay); wait > 0 {
		canvas.RequestFrameIn(wait)
		return
	}
	m.intent.Reset()
	m.sub, m.subMenu = -1, nil
	if m.items[row].submenu {
		m.sub = row
	}
	canvas.RequestFrame() // the submenu is declared by the next paint
}

// size is the menu's: as wide as its widest row and as tall as its rows.
func (m *MenuItems) size() (width, height float32) {
	f := m.f
	theme := &f.theme
	width, height = m.minWidth(), 2*components.DropdownMenuPadding
	for i := range m.items {
		item := &m.items[i]
		height += m.rowExtent(i)
		switch item.kind {
		case menuRowSeparator, menuRowLabel, menuRowContent, menuRowPageContent:
		default:
			row := m.row(i)
			if m.context {
				if f.canvas != nil {
					width = max(width, components.ContextMenuItemWidth(f.canvas, *theme, contextItem(row)))
				}
				continue
			}
			if f.canvas != nil {
				width = max(width, components.DropdownMenuItemWidth(f.canvas, *theme, row))
			}
		}
	}
	if m.widthMin > 0 && width < m.widthMin {
		width = m.widthMin
	}
	if m.widthMax > 0 && width > m.widthMax {
		width = m.widthMax
	}
	return width, height
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
	menuRowContent
	menuRowPageContent
)

// MenuItems is an open Menu or ContextMenu being declared: what its rows are, and,
// between paints, where it is scrolled to.
type MenuItems struct {
	items     []menuRow
	scroll    components.MenuScroll
	typeahead components.MenuTypeahead // the letters typed to reach an item
	paint     func(*DrawContext)
	keys      [6]string // the keycaps of the row being measured or painted
	page      *Page
	f         *frame
	open      *bool
	anchor    int32 // the element a Menu hangs off; -1 for a ContextMenu
	scope     gfx.Scope
	layer     uint16

	context            bool // a ContextMenu: its own look, and Tab does not close it
	shown              bool // a ContextMenu opened from its target
	at                 Rect // where a ContextMenu opened
	rowSize            float32
	side               menuSide // the edge of the anchor a Menu opens on
	widthMin, widthMax float32  // the least and greatest surface width; zero keeps the default

	// A submenu's menu, and the menu of the one open in it: root is the Menu or
	// ContextMenu the chain hangs off, itself for that one, and parent the menu its
	// row is in, nil for the root.
	root, parent *MenuItems
	sub          int        // the row whose submenu is open, or -1
	subMenu      *MenuItems // that submenu, once declared in this paint
	intent       components.HoverIntent
	seen         uint32 // the page's epoch the menu was last declared in
	focusSub     bool   // the keyboard opened the submenu: its first row takes the focus
	focusFirst   bool   // this submenu's first row takes the focus as it paints

	// An unpainted menu declares its behavior and rows but paints none of them:
	// drawPanel draws the surface and drawItem each row. itemPainter and
	// panelPainter are their method values, made once.
	unpainted    bool
	drawPanel    func(*gfx.Canvas, gfx.Rect, gfx.Theme)
	drawItem     func(MenuItemParts)
	panelPainter func(*gfx.Canvas, gfx.Rect, gfx.Theme)
	itemPainter  func(components.MenuItemPaint)
}

// MenuItemParts is one row of a menu declared Unpainted. The application paints
// it, and the menu keeps the row's hit, its focus and keyboard behavior, and its
// place in a submenu chain. Separator and GroupLabel say the row is not an
// action; Active is the row the pointer or the keyboard is on; Foreground is the
// color the menu's own row would give its label.
type MenuItemParts struct {
	Canvas      *gfx.Canvas
	Theme       gfx.Theme
	Rect        gfx.Rect
	Label       string
	Description string
	Shortcut    string
	Checked     bool
	Selected    bool
	Expanded    bool
	Disabled    bool
	Active      bool
	Destructive bool
	Separator   bool
	GroupLabel  bool
	Foreground  gfx.Color
}

// Unpainted makes the menu declare its behavior, size and rows but paint none
// of them: DrawPanel draws the surface and DrawItem each row, inside the menu's
// layer and clipping. Hits, focus, the keyboard, dismissal and submenu opening
// stay the menu's, so an application's own menu cannot behave differently. A
// menu that is not unpainted keeps the theme's look.
//
// Call it in the body, before the rows, on every paint: the setting lasts one
// paint.
func (m *MenuItems) Unpainted() *MenuItems {
	m.unpainted = true
	return m
}

// DrawPanel paints an Unpainted menu's surface, inside the menu's layer and
// clipping and within its entrance. Nil paints no surface.
func (m *MenuItems) DrawPanel(draw func(canvas *gfx.Canvas, bounds gfx.Rect, theme gfx.Theme)) *MenuItems {
	m.drawPanel = draw
	return m
}

// DrawItem paints each row of an Unpainted menu, actions, group labels and
// separators alike, after the menu has declared the row's hit.
func (m *MenuItems) DrawItem(draw func(MenuItemParts)) *MenuItems {
	m.drawItem = draw
	return m
}

// paintItem is the unpainted menu's row painter; it is a method value so
// painting a menu allocates nothing.
func (m *MenuItems) paintItem(parts components.MenuItemPaint) {
	if m.drawItem == nil {
		return
	}
	m.drawItem(MenuItemParts{
		Canvas: parts.Canvas, Theme: parts.Theme, Rect: parts.Bounds,
		Label: parts.Label, Description: parts.Description, Shortcut: parts.Shortcut,
		Checked: parts.Checked, Selected: parts.Selected, Expanded: parts.Expanded,
		Disabled: parts.Disabled, Active: parts.Active, Destructive: parts.Destructive,
		Separator: parts.Separator, GroupLabel: parts.GroupLabel, Foreground: parts.Foreground,
	})
}

// paintPanel is DrawPanel as the components take it.
func (m *MenuItems) paintPanel(canvas *gfx.Canvas, bounds gfx.Rect, theme gfx.Theme) {
	if m.drawPanel != nil {
		m.drawPanel(canvas, bounds, theme)
	}
}

type menuRow struct {
	kind        menuRowKind
	label       string
	description string
	icon        gfx.Icon
	// The shortcut the row shows, a keymap.Chord's fields: the Page's type would
	// keep that type's methods in every program, Menu or not.
	modifiers     gfx.Modifiers
	char          byte
	key           gfx.Key
	destructive   bool
	disabled      bool
	submenu       bool // the row opens a submenu
	trailingWidth float32
	trailing      func(canvas *gfx.Canvas, bounds gfx.Rect, theme gfx.Theme)
	height        float32                                                    // a content row's height
	content       func(canvas *gfx.Canvas, bounds gfx.Rect, theme gfx.Theme) // what paints a content row
	pageContent   func(c C, bounds Rect)                                     // what a page content row declares
	checked       int8                                                       // 1 a checked checkbox, 2 an unchecked one, 3 a chosen radio, 4 an unchosen one
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
	item := components.DropdownMenuItem{ID: m.scope.At(menuRows + i), Label: row.label, Description: row.description, Disabled: row.disabled, Icon: row.icon, TrailingWidth: row.trailingWidth}
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
	if row.submenu {
		item.Kind, item.Expanded = components.DropdownMenuSubmenu, i == m.sub
	}
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

// Sub adds an item that opens a submenu with the items body declares, beside it:
// as the pointer rests on it, as it is clicked, or as Right, Enter or Space is
// pressed on it, Left going back. body runs only while the submenu is open, in
// the paint it is declared in. Choosing an item in it closes the whole menu.
func (m *MenuItems) Sub(label string, body func(*MenuItems)) MenuItem {
	item := m.Item(label)
	m.items[item.i].submenu = true
	if canvas := m.f.canvas; canvas != nil && canvas.Clicked(m.scope.At(menuRows+item.i)) {
		m.sub, m.subMenu = item.i, nil // a click opens it, as the pointer resting there does
	}
	if m.sub != item.i {
		return item
	}
	p := m.page
	child := stateIn[MenuItems](p, p.tree.Sub(m.scope, gfx.Index(item.i)), "sub")
	child.page, child.f, child.open, child.anchor = p, m.f, m.open, -1
	child.scope, child.root, child.parent, child.context, child.rowSize = p.tree.Sub(m.scope, gfx.Index(item.i)), m.root, m, m.context, m.rowSize
	child.fresh()
	if m.focusSub {
		m.focusSub, child.focusFirst = false, true
	}
	child.declare(body)
	m.subMenu = child
	p.menus = append(p.menus, child)
	return item
}

// menuSide is the edge of its anchor a Menu opens on.
type menuSide uint8

const (
	menuBelow  menuSide = iota // under the anchor, flipping over it when there is no room
	menuAbove                  // over the anchor, flipping under it when there is no room
	menuBeside                 // right of the anchor, flipping to its left when there is no room
)

// Above opens the menu over its anchor instead of under it, flipping under when
// there is no room over: the menu of a trigger along the bottom of a panel, which
// should cover what is above it. A submenu opens beside its row and a ContextMenu
// opens at the pointer, so neither is affected.
func (m *MenuItems) Above() { m.side = menuAbove }

// Beside opens the menu to the right of its anchor, its top at the anchor's,
// flipping to the left when there is no room on the right: the menu of a row in a
// list or a popup, which should leave the rows under it in sight, as a submenu
// leaves its menu. A submenu and a ContextMenu are not affected.
func (m *MenuItems) Beside() { m.side = menuBeside }

// WidthRange sets the least and greatest width of the menu's surface. Zero keeps
// the menu's own minimum (128 for a Menu, 144 for a ContextMenu) or leaves that end
// unbounded. A row wider than the greatest is ellipsized to fit.
func (m *MenuItems) WidthRange(least, greatest float32) {
	if least < 0 || greatest < 0 || greatest > 0 && greatest < least {
		panic("ui: MenuItems.WidthRange needs 0 <= least <= greatest, or greatest 0 for no limit")
	}
	m.widthMin, m.widthMax = least, greatest
}

// Content adds a row height tall that body paints: the room for what a menu row cannot say,
// such as a strip of color swatches. It is not an item, so the keyboard passes over it; body
// takes the row's rectangle and declares its own hits, inside the menu's clip, and is not
// called while scrolling has the row out of sight.
func (m *MenuItems) Content(height float32, body func(canvas *gfx.Canvas, bounds gfx.Rect, theme gfx.Theme)) {
	m.items = append(m.items, menuRow{kind: menuRowContent, height: height, content: body})
}

// ContentPage adds a row height tall whose controls body declares in the page:
// the room for what a menu row cannot say and a canvas painter cannot host, such
// as a strip of color swatches. It is not an item, so the keyboard passes over it.
// The row is placed where the menu's rows put it; it is for a menu that does not
// scroll its rows.
func (m *MenuItems) ContentPage(height float32, body func(c C, bounds Rect)) {
	m.items = append(m.items, menuRow{kind: menuRowPageContent, height: height, pageContent: body})
}

// rowExtent is how tall row i is, as size and the page rows' placement both need.
func (m *MenuItems) rowExtent(i int) float32 {
	row := &m.items[i]
	switch row.kind {
	case menuRowSeparator:
		return components.DropdownMenuSeparatorHeight
	case menuRowLabel:
		return components.DropdownMenuLabelHeight
	case menuRowContent, menuRowPageContent:
		return row.height
	}
	if m.context {
		return components.ContextMenuItemHeight(m.f.theme, contextItem(m.row(i)), m.rowSize)
	}
	return components.DropdownMenuItemHeight(m.f.theme, m.row(i))
}

// paintContent calls a content row's painter over its row, inside the menu's clip.
func (m *MenuItems) paintContent(canvas *gfx.Canvas, theme gfx.Theme, row *menuRow, bounds gfx.Rect, begin, end func(), menu gfx.Rect) {
	if bounds.Width <= 0 || row.content == nil {
		return
	}
	begin()
	canvas.Clip(gfx.Rect{X: menu.X, Y: menu.Y + components.DropdownMenuPadding/2, Width: menu.Width, Height: menu.Height - components.DropdownMenuPadding})
	row.content(canvas, bounds, theme)
	canvas.Unclip()
	end()
}

// paintTrailing calls a row's trailing painter over the slot the menu reserved at its end.
func (m *MenuItems) paintTrailing(canvas *gfx.Canvas, theme gfx.Theme, row *menuRow, slot gfx.Rect, begin, end func()) {
	if slot.Width <= 0 || row.trailing == nil {
		return
	}
	begin()
	row.trailing(canvas, slot, theme)
	end()
}

// Label adds a heading over the items that follow it.
func (m *MenuItems) Label(label string) {
	m.items = append(m.items, menuRow{kind: menuRowLabel, label: label})
}

// Separator adds a line between groups of items.
func (m *MenuItems) Separator() {
	m.items = append(m.items, menuRow{kind: menuRowSeparator})
}

// Trailing reserves width pixels at the item's end and has paint fill them: a small control
// of the row's own, such as the eye that hides a shell from a list. The control declares its
// own hit, over the row's, so pressing it does not choose the row; it is not called while
// scrolling has the row out of sight. A menu row has either Trailing or a shortcut.
func (item MenuItem) Trailing(width float32, paint func(canvas *gfx.Canvas, bounds gfx.Rect, theme gfx.Theme)) MenuItem {
	row := &item.menu.items[item.i]
	row.trailingWidth, row.trailing = width, paint
	return item
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

// Description adds a muted second line under the item's label, for an action whose
// meaning needs a word more: what a model answers, which folder a file goes to.
func (item MenuItem) Description(text string) MenuItem {
	item.menu.items[item.i].description = text
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
	if canvas == nil || m.items[item.i].disabled || m.items[item.i].submenu || !canvas.Clicked(m.scope.At(menuRows+item.i)) {
		return false
	}
	*m.open = false
	return true
}

// draw paints the open menu in the popup the page placed for it.
func (m *MenuItems) draw(d *DrawContext) {
	m.paintAt(d.Canvas, d.Theme, d.Rect, max(m.layer, m.f.popupLayer))
}

// paintAt paints the menu over bounds in layer, then the submenu open in it beside
// its row, a layer over.
func (m *MenuItems) paintAt(canvas *gfx.Canvas, theme *Theme, bounds Rect, layer uint16) {
	f, scope := m.f, m.scope
	m.scroll.BarID = scope.At(menuBar)
	// A submenu is in its root's focus scope, which keeps the keyboard in the chain
	// and lets the pointer reach every menu of it; Escape and a press outside are
	// the root's, and close them all.
	focus, dismiss, id := m.root.scope.At(menuFocus), gfx.ID{}, scope.At(menuFocus)
	if m.parent == nil {
		dismiss = scope.At(menuDismiss)
	}
	var trigger Rect // the row of the open submenu
	if m.context {
		menu := components.BeginContextMenu(canvas, *theme, bounds, components.ContextMenuOptions{
			Layer: layer, Scope: focus, DismissID: dismiss, Viewport: f.bounds, ID: id,
			Scroll: &m.scroll, Submenu: m.parent != nil, RowHeight: m.rowSize,
			Unpainted: m.unpainted, PaintPanel: m.panelPainter, PaintItem: m.itemPainter,
		})
		for i := range m.items {
			switch row := &m.items[i]; row.kind {
			case menuRowSeparator:
				menu.AddSeparator()
			case menuRowLabel:
				menu.AddLabel(row.label, false)
			case menuRowContent:
				m.paintContent(canvas, *theme, row, menu.AddContent(row.height), func() { menu.BeginContent() }, func() { menu.EndContent() }, menu.Bounds())
			case menuRowPageContent:
				menu.AddContent(row.height) // the page declares the row's controls
			default:
				if row.trailing != nil {
					r, slot := menu.AddItemSlot(contextItem(m.row(i)))
					if i == m.sub {
						trigger = r
					}
					m.paintTrailing(canvas, *theme, row, slot, menu.BeginContent, menu.EndContent)
				} else if r := menu.AddItem(contextItem(m.row(i))); i == m.sub {
					trigger = r
				}
			}
		}
		menu.End()
	} else {
		side := components.OverlayRight // the edge the menu slides in from, its anchor's
		if m.parent == nil {
			anchor := f.nodes[m.anchor].rect
			switch {
			case m.side == menuBeside:
				if bounds.X+bounds.Width <= anchor.X+anchor.Width/2 {
					side = components.OverlayLeft // flipped for want of room
				}
			case bounds.Y+bounds.Height <= anchor.Y+anchor.Height/2:
				side = components.OverlayTop
			default:
				side = components.OverlayBottom
			}
		}
		menu := components.BeginDropdownMenu(canvas, *theme, bounds, components.DropdownMenuOptions{
			Layer: layer, Scope: focus, DismissID: dismiss, Viewport: f.bounds, ID: id,
			Scroll: &m.scroll, Side: side, Submenu: m.parent != nil,
			Unpainted: m.unpainted, PaintPanel: m.panelPainter, PaintItem: m.itemPainter,
		})
		for i := range m.items {
			switch row := &m.items[i]; row.kind {
			case menuRowSeparator:
				menu.AddSeparator()
			case menuRowLabel:
				menu.AddLabel(row.label, false)
			case menuRowContent:
				m.paintContent(canvas, *theme, row, menu.AddContent(row.height), func() { menu.BeginContent() }, func() { menu.EndContent() }, menu.Bounds())
			case menuRowPageContent:
				menu.AddContent(row.height) // the page declares the row's controls
			default:
				if row.trailing != nil {
					r, slot := menu.AddItemSlot(m.row(i))
					if i == m.sub {
						trigger = r
					}
					m.paintTrailing(canvas, *theme, row, slot, menu.BeginContent, menu.EndContent)
				} else if r := menu.AddItem(m.row(i)); i == m.sub {
					trigger = r
				}
			}
		}
		menu.End()
	}
	if m.focusFirst {
		m.focusFirst = false
		for i := range m.items {
			if row := &m.items[i]; row.kind == menuRowItem && !row.disabled {
				canvas.RequestFocus(scope.At(menuRows + i))
				break
			}
		}
	}
	if child := m.subMenu; child != nil && trigger.Width > 0 { // a row scrolled out of sight shows none
		width, height := child.size()
		placed := components.SubmenuPlacement(trigger, gfx.Size{Width: width, Height: height}, f.bounds.Inset(gfx.All(components.DropdownMenuPadding)))
		child.paintAt(canvas, theme, placed, layer+1)
	}
}

// contextItem is a dropdown row as a context menu's: the two describe one row.
func contextItem(item components.DropdownMenuItem) components.ContextMenuItem {
	context := components.ContextMenuItem{
		ID: item.ID, Label: item.Label, Description: item.Description, Keys: item.Keys, Icon: item.Icon, Disabled: item.Disabled, TrailingWidth: item.TrailingWidth,
		Checked: item.Checked, Selected: item.Selected,
	}
	switch item.Kind {
	case components.DropdownMenuCheckbox:
		context.Kind = components.ContextMenuCheckbox
	case components.DropdownMenuRadio:
		context.Kind = components.ContextMenuRadio
	case components.DropdownMenuSubmenu:
		context.Kind, context.Expanded = components.ContextMenuSubmenu, item.Expanded
	}
	if item.Variant == components.DropdownMenuItemDestructive {
		context.Variant = components.ContextMenuItemDestructive
	}
	return context
}

// menuKey moves between the items of an open Menu with the arrows and to the one
// whose label begins with the letters typed, and closes a Menu on Tab, from its
// items or from its anchor; Shift+F10 on the target of a ContextMenu opens it.
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
		// A context menu is modal: its arrows move through it from wherever the focus is,
		// as a menu opened at a point of the window has no control of its own to be on.
		last := i == len(p.menus)-1 && (event.Target.IsZero() || m.context)
		if !inside && !last && (event.Target != trigger || trigger.IsZero()) {
			continue
		}
		if event.Key == gfx.KeyTab && !event.Release && !m.context {
			*m.open = false
			return gfx.EventResult{Handled: true, Repaint: true, ContinueTraversal: true}, true
		}
		if result, ok := m.submenuKey(event, inside); ok {
			return result, true
		}
		if text := event.Text; !event.Release && event.Modifiers&^gfx.ModifierShift == 0 && (text != "" || event.Key == gfx.KeySpace) {
			if text == "" {
				text = " "
			}
			now := clockEpoch.Add(m.f.canvas.Now())
			typing := text == " " && m.typeahead.Typing(now) // a Space in the middle of a label
			if id, ok := m.typeahead.Type(text, now, &m.scroll, event.Target); ok {
				return gfx.EventResult{Handled: true, Repaint: true, Focus: id, FocusVisible: true}, true
			}
			if typing {
				return gfx.EventResult{Handled: true}, true
			}
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

// submenuKey opens the submenu of the row the keyboard is on with Right, Enter or
// Space, its first row taking the focus, and goes back to that row with Left.
func (m *MenuItems) submenuKey(event gfx.KeyEvent, inside bool) (gfx.EventResult, bool) {
	if event.Release || event.Modifiers != 0 || !inside {
		return gfx.EventResult{}, false
	}
	switch event.Key {
	case gfx.KeyRight, gfx.KeyEnter, gfx.KeySpace:
		at, _ := event.Target.In(m.scope)
		if row := at - menuRows; row >= 0 && row < len(m.items) && m.items[row].submenu && !m.items[row].disabled {
			m.sub, m.subMenu, m.focusSub = row, nil, true
			return gfx.EventResult{Handled: true, Repaint: true}, true
		}
	case gfx.KeyLeft:
		if parent := m.parent; parent != nil {
			row := parent.sub
			parent.sub, parent.subMenu = -1, nil
			return gfx.EventResult{Handled: true, Repaint: true, Focus: parent.scope.At(menuRows + row), FocusVisible: true}, true
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
