package ui

import (
	"github.com/vivaui/viva/base"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/keymap"
)

// row is row i as the components paint it. Its Keys are the MenuItems' scratch,
// good until the next row.
func menuItemsRow(m *MenuItems, i int) components.DropdownMenuItem {
	row := &m.Items[i]
	item := components.DropdownMenuItem{ID: m.GetScope().At(MenuRowsPart + i), Label: row.Label, Description: row.GetDescription(), Disabled: row.GetDisabled(), Icon: row.GetIcon(), TrailingWidth: row.GetTrailingWidth()}
	if row.GetDestructive() {
		item.Variant = components.DropdownMenuItemDestructive
	}
	if chord := (keymap.Chord{Modifiers: row.GetModifiers(), Char: row.GetChar(), Key: row.GetKey()}); chord.Valid() {
		item.Keys = chord.Keycaps(m.KeysSlice()[:0])
	}
	switch row.GetChecked() {
	case 1, 2:
		item.Kind, item.Checked = components.DropdownMenuCheckbox, row.GetChecked() == 1
	case 3, 4:
		item.Kind, item.Checked = components.DropdownMenuRadio, row.GetChecked() == 3
	}
	item.Selected = item.Checked
	if row.GetSubmenu() {
		item.Kind, item.Expanded = components.DropdownMenuSubmenu, i == m.GetSub()
	}
	return item
}

// rowExtent is how tall row i is, as size and the page rows' placement both need.
func rowExtent(m *MenuItems, i int) float32 {
	row := &m.Items[i]
	switch row.Kind {
	case MenuRowSeparator:
		return components.DropdownMenuSeparatorHeight
	case MenuRowLabel:
		return components.DropdownMenuLabelHeight
	case MenuRowContent, MenuRowPageContent:
		return row.GetHeight()
	}
	if m.GetContext() {
		return components.ContextMenuItemHeight(m.GetF().Palette, contextItem(menuItemRow(m, i)), m.GetRowSize())
	}
	return components.DropdownMenuItemHeight(m.GetF().Palette, menuItemRow(m, i))
}

// paintContent calls a content row's painter over its row, inside the menu's clip.
func paintContent(m *MenuItems, Surface *gfx.Canvas, Palette gfx.Theme, row *MenuRow, bounds gfx.Rect, begin, end func(), menu gfx.Rect) {
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
func paintAt(m *MenuItems, Surface *gfx.Canvas, Palette *Theme, bounds Rect, layer uint16) {
	f, scope := m.GetF(), m.GetScope()
	m.Scroll.BarID = scope.At(MenuBar)
	// A submenu is in its root's focus scope, which keeps the keyboard in the chain
	// and lets the pointer reach every menu of it; Escape and a press outside are
	// the root's, and close them all.
	focus, dismiss, id := m.GetRoot().GetScope().At(MenuFocus), gfx.ID{}, scope.At(MenuFocus)
	if m.Parent == nil {
		dismiss = scope.At(MenuDismiss)
	}
	var trigger Rect // the row of the open submenu
	if m.GetContext() {
		menu := components.BeginContextMenu(Surface, *Palette, bounds, components.ContextMenuOptions{
			Layer: layer, Scope: focus, DismissID: dismiss, Viewport: f.GetBounds(), ID: id,
			Scroll: &m.Scroll, Submenu: m.Parent != nil, RowHeight: m.GetRowSize(),
			Unpainted: m.GetUnpainted(), PaintPanel: m.GetPanelPainter(), PaintItem: m.GetItemPainter(),
		})
		for i := range m.Items {
			switch row := &m.Items[i]; row.Kind {
			case MenuRowSeparator:
				menu.AddSeparator()
			case MenuRowLabel:
				menu.AddLabel(row.Label, false)
			case MenuRowContent:
				paintContent(m, Surface, *Palette, row, menu.AddContent(row.GetHeight()), func() { menu.BeginContent() }, func() { menu.EndContent() }, menu.Bounds())
			case MenuRowPageContent:
				menu.AddContent(row.GetHeight()) // the page declares the row's controls
			default:
				if row.GetTrailing() != nil {
					r, slot := menu.AddItemSlot(contextItem(menuItemRow(m, i)))
					if i == m.GetSub() {
						trigger = r
					}
					m.PaintTrailing(Surface, *Palette, row, slot, menu.BeginContent, menu.EndContent)
				} else if r := menu.AddItem(contextItem(menuItemRow(m, i))); i == m.GetSub() {
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
			case m.GetSide() == MenuBeside:
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
			Layer: layer, Scope: focus, DismissID: dismiss, Viewport: f.GetBounds(), ID: id,
			Scroll: &m.Scroll, Side: side, Submenu: m.Parent != nil,
			Unpainted: m.GetUnpainted(), PaintPanel: m.GetPanelPainter(), PaintItem: m.GetItemPainter(),
		})
		for i := range m.Items {
			switch row := &m.Items[i]; row.Kind {
			case MenuRowSeparator:
				menu.AddSeparator()
			case MenuRowLabel:
				menu.AddLabel(row.Label, false)
			case MenuRowContent:
				paintContent(m, Surface, *Palette, row, menu.AddContent(row.GetHeight()), func() { menu.BeginContent() }, func() { menu.EndContent() }, menu.Bounds())
			case MenuRowPageContent:
				menu.AddContent(row.GetHeight()) // the page declares the row's controls
			default:
				if row.GetTrailing() != nil {
					r, slot := menu.AddItemSlot(menuItemRow(m, i))
					if i == m.GetSub() {
						trigger = r
					}
					m.PaintTrailing(Surface, *Palette, row, slot, menu.BeginContent, menu.EndContent)
				} else if r := menu.AddItem(menuItemRow(m, i)); i == m.GetSub() {
					trigger = r
				}
			}
		}
		menu.End()
	}
	if m.GetFocusFirst() {
		m.SetFocusFirst(false)
		for i := range m.Items {
			if row := &m.Items[i]; row.Kind == MenuRowItem && !row.GetDisabled() {
				Surface.RequestFocus(scope.At(MenuRowsPart + i))
				break
			}
		}
	}
	if child := m.GetSubMenu(); child != nil && trigger.Width > 0 { // a row scrolled out of sight shows none
		width, height := child.Size()
		placed := components.SubmenuPlacement(trigger, gfx.Size{Width: width, Height: height}, f.GetBounds().Inset(gfx.All(components.DropdownMenuPadding)))
		paintAt(child, Surface, Palette, placed, layer+1)
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

// size is the menu's: as wide as its widest row and as tall as its rows.
func menuSize(m *MenuItems) (width, height float32) {
	f := m.GetF()
	theme := &f.Palette
	width, height = m.MinWidth(), 2*base.DropdownMenuPadding
	for i := range m.Items {
		item := &m.Items[i]
		height += rowExtent(m, i)
		switch item.Kind {
		case MenuRowSeparator, MenuRowLabel, MenuRowContent, MenuRowPageContent:
		default:
			row := menuItemRow(m, i)
			if m.GetContext() {
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
	if m.GetWidthMin() > 0 && width < m.GetWidthMin() {
		width = m.GetWidthMin()
	}
	if m.GetWidthMax() > 0 && width > m.GetWidthMax() {
		width = m.GetWidthMax()
	}
	return width, height
}

// menuItemRow is row i as the components paint it. Its Keys are the MenuItems'
// scratch, good until the next row.
func menuItemRow(m *MenuItems, i int) components.DropdownMenuItem {
	row := &m.Items[i]
	item := components.DropdownMenuItem{ID: m.GetScope().At(MenuRowsPart + i), Label: row.Label, Description: row.GetDescription(), Disabled: row.GetDisabled(), Icon: row.GetIcon(), TrailingWidth: row.GetTrailingWidth()}
	if row.GetDestructive() {
		item.Variant = components.DropdownMenuItemDestructive
	}
	if chord := (keymap.Chord{Modifiers: row.GetModifiers(), Char: row.GetChar(), Key: row.GetKey()}); chord.Valid() {
		item.Keys = chord.Keycaps(m.KeysSlice()[:0])
	}
	switch row.GetChecked() {
	case 1, 2:
		item.Kind, item.Checked = components.DropdownMenuCheckbox, row.GetChecked() == 1
	case 3, 4:
		item.Kind, item.Checked = components.DropdownMenuRadio, row.GetChecked() == 3
	}
	item.Selected = item.Checked
	if row.GetSubmenu() {
		item.Kind, item.Expanded = components.DropdownMenuSubmenu, i == m.GetSub()
	}
	return item
}
