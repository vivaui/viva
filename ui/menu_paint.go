package ui

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/keymap"
)

// declareMenu declares an open menu: its rows, its size, and the popup that places
// it, under the anchor element or, with anchor -1, at menu.at.
func (p *Page) DeclareMenu(c C, menu *MenuItems, scope gfx.Scope, open *bool, Anchor int32, opened *menuOrigin, body func(*MenuItems)) {
	p.overlay = true
	if menu.paint == nil {
		menu.paint = menu.draw // made once: a method value made in each paint allocates
	}
	if menu.itemPainter == nil {
		menu.itemPainter, menu.panelPainter = menu.paintItem, menu.paintPanel
	}
	f := c.f
	menu.page, menu.f, menu.open, menu.Anchor = p, f, open, Anchor
	menu.scope, menu.root, menu.Parent = scope, menu, nil
	if opened != nil {
		menu.at = opened.at
	}
	menu.layer = f.WidgetLayer() + 2 // over the page, and over a dialog it is declared in
	menu.side = menuBelow            // unless body says otherwise again in this paint
	menu.fresh()
	menu.declare(body)
	if f.Surface != nil && f.Surface.Clicked(scope.At(menuDismiss)) {
		*open = false
		return
	}
	width, height := menu.Size()
	popup := f.Popup()
	if Anchor >= 0 {
		anchorBox := Box{f: f, i: Anchor, gen: f.gen}
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
		f.Nodes[popup.i].FocusScope = menu.scope.At(menuFocus)
		popup.Children(func() { menu.declarePageRows(c, width, height) })
	}
	p.menus = append(p.menus, menu)
	if p.menuRoute == nil {
		p.keyRoute, p.menuRoute = (*Page).RouteKey, (*Page).MenuKey
		p.wheelRoute = (*Page).OverlayWheel
	}
}

// declarePageRows declares the page controls of every page content row at the
// place the menu's rows and scroll put them.
func (m *MenuItems) declarePageRows(c C, width, height float32) {
	pad := float32(components.DropdownMenuPadding)
	y := pad - m.Scroll.Offset
	for i := range m.Items {
		row := &m.Items[i]
		if row.Kind == menuRowPageContent && row.pageContent != nil && row.height > 0 &&
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

// size is the menu's: as wide as its widest row and as tall as its rows.
func (m *MenuItems) Size() (width, height float32) {
	f := m.f
	theme := &f.Palette
	width, height = m.minWidth(), 2*components.DropdownMenuPadding
	for i := range m.Items {
		item := &m.Items[i]
		height += m.rowExtent(i)
		switch item.Kind {
		case menuRowSeparator, menuRowLabel, menuRowContent, menuRowPageContent:
		default:
			row := m.row(i)
			if m.context {
				if f.Surface != nil {
					width = max(width, components.ContextMenuItemWidth(f.Surface, *theme, contextItem(row)))
				}
				continue
			}
			if f.Surface != nil {
				width = max(width, components.DropdownMenuItemWidth(f.Surface, *theme, row))
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

// row is row i as the components paint it. Its Keys are the MenuItems' scratch,
// good until the next row.
func (m *MenuItems) row(i int) components.DropdownMenuItem {
	row := &m.Items[i]
	item := components.DropdownMenuItem{ID: m.scope.At(menuRows + i), Label: row.Label, Description: row.description, Disabled: row.disabled, Icon: row.icon, TrailingWidth: row.trailingWidth}
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

// rowExtent is how tall row i is, as size and the page rows' placement both need.
func (m *MenuItems) rowExtent(i int) float32 {
	row := &m.Items[i]
	switch row.Kind {
	case menuRowSeparator:
		return components.DropdownMenuSeparatorHeight
	case menuRowLabel:
		return components.DropdownMenuLabelHeight
	case menuRowContent, menuRowPageContent:
		return row.height
	}
	if m.context {
		return components.ContextMenuItemHeight(m.f.Palette, contextItem(m.row(i)), m.rowSize)
	}
	return components.DropdownMenuItemHeight(m.f.Palette, m.row(i))
}

// paintContent calls a content row's painter over its row, inside the menu's clip.
func (m *MenuItems) paintContent(Surface *gfx.Canvas, Palette gfx.Theme, row *menuRow, bounds gfx.Rect, begin, end func(), menu gfx.Rect) {
	if bounds.Width <= 0 || row.Content == nil {
		return
	}
	begin()
	Surface.Clip(gfx.Rect{X: menu.X, Y: menu.Y + components.DropdownMenuPadding/2, Width: menu.Width, Height: menu.Height - components.DropdownMenuPadding})
	row.Content(Surface, bounds, Palette)
	Surface.Unclip()
	end()
}

// paintAt paints the menu over bounds in layer, then the submenu open in it beside
// its row, a layer over.
func (m *MenuItems) paintAt(Surface *gfx.Canvas, Palette *Theme, bounds Rect, layer uint16) {
	f, scope := m.f, m.scope
	m.Scroll.BarID = scope.At(menuBar)
	// A submenu is in its root's focus scope, which keeps the keyboard in the chain
	// and lets the pointer reach every menu of it; Escape and a press outside are
	// the root's, and close them all.
	focus, dismiss, id := m.root.scope.At(menuFocus), gfx.ID{}, scope.At(menuFocus)
	if m.Parent == nil {
		dismiss = scope.At(menuDismiss)
	}
	var trigger Rect // the row of the open submenu
	if m.context {
		menu := components.BeginContextMenu(Surface, *Palette, bounds, components.ContextMenuOptions{
			Layer: layer, Scope: focus, DismissID: dismiss, Viewport: f.bounds, ID: id,
			Scroll: &m.Scroll, Submenu: m.Parent != nil, RowHeight: m.rowSize,
			Unpainted: m.unpainted, PaintPanel: m.panelPainter, PaintItem: m.itemPainter,
		})
		for i := range m.Items {
			switch row := &m.Items[i]; row.Kind {
			case menuRowSeparator:
				menu.AddSeparator()
			case menuRowLabel:
				menu.AddLabel(row.Label, false)
			case menuRowContent:
				m.paintContent(Surface, *Palette, row, menu.AddContent(row.height), func() { menu.BeginContent() }, func() { menu.EndContent() }, menu.Bounds())
			case menuRowPageContent:
				menu.AddContent(row.height) // the page declares the row's controls
			default:
				if row.trailing != nil {
					r, slot := menu.AddItemSlot(contextItem(m.row(i)))
					if i == m.sub {
						trigger = r
					}
					m.paintTrailing(Surface, *Palette, row, slot, menu.BeginContent, menu.EndContent)
				} else if r := menu.AddItem(contextItem(m.row(i))); i == m.sub {
					trigger = r
				}
			}
		}
		menu.End()
	} else {
		side := components.OverlayRight // the edge the menu slides in from, its anchor's
		if m.Parent == nil {
			anchor := f.Nodes[m.Anchor].Rect
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
		menu := components.BeginDropdownMenu(Surface, *Palette, bounds, components.DropdownMenuOptions{
			Layer: layer, Scope: focus, DismissID: dismiss, Viewport: f.bounds, ID: id,
			Scroll: &m.Scroll, Side: side, Submenu: m.Parent != nil,
			Unpainted: m.unpainted, PaintPanel: m.panelPainter, PaintItem: m.itemPainter,
		})
		for i := range m.Items {
			switch row := &m.Items[i]; row.Kind {
			case menuRowSeparator:
				menu.AddSeparator()
			case menuRowLabel:
				menu.AddLabel(row.Label, false)
			case menuRowContent:
				m.paintContent(Surface, *Palette, row, menu.AddContent(row.height), func() { menu.BeginContent() }, func() { menu.EndContent() }, menu.Bounds())
			case menuRowPageContent:
				menu.AddContent(row.height) // the page declares the row's controls
			default:
				if row.trailing != nil {
					r, slot := menu.AddItemSlot(m.row(i))
					if i == m.sub {
						trigger = r
					}
					m.paintTrailing(Surface, *Palette, row, slot, menu.BeginContent, menu.EndContent)
				} else if r := menu.AddItem(m.row(i)); i == m.sub {
					trigger = r
				}
			}
		}
		menu.End()
	}
	if m.focusFirst {
		m.focusFirst = false
		for i := range m.Items {
			if row := &m.Items[i]; row.Kind == menuRowItem && !row.disabled {
				Surface.RequestFocus(scope.At(menuRows + i))
				break
			}
		}
	}
	if child := m.subMenu; child != nil && trigger.Width > 0 { // a row scrolled out of sight shows none
		width, height := child.Size()
		placed := components.SubmenuPlacement(trigger, gfx.Size{Width: width, Height: height}, f.bounds.Inset(gfx.All(components.DropdownMenuPadding)))
		child.paintAt(Surface, Palette, placed, layer+1)
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
