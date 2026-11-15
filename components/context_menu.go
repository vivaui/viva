package components

import "github.com/vivaui/viva/gfx"

// ContextMenuItemVariant selects the standard or destructive treatment.
type ContextMenuItemVariant uint8

const (
	ContextMenuItemDefault ContextMenuItemVariant = iota
	ContextMenuItemDestructive
)

// ContextMenuItemKind selects the item's indicator and semantic state.
type ContextMenuItemKind uint8

const (
	ContextMenuAction ContextMenuItemKind = iota
	ContextMenuCheckbox
	ContextMenuRadio
	ContextMenuSubmenu
)

// ContextMenuState is caller-owned popup visibility. Its zero value is closed.
// Transitions are synchronous value operations and never schedule work.
type ContextMenuState bool

// IsOpen reports whether the context menu should be painted.
func (state ContextMenuState) IsOpen() bool { return bool(state) }

// Open returns the open state.
func (ContextMenuState) Open() ContextMenuState { return true }

// Close returns the closed state.
func (ContextMenuState) Close() ContextMenuState { return false }

// Toggle returns the opposite visibility state.
func (state ContextMenuState) Toggle() ContextMenuState { return !state }

// Dismiss closes the menu after selection or an application-owned dismiss action.
func (ContextMenuState) Dismiss() ContextMenuState { return false }

// Escape closes the menu after the runtime routes an Escape key press.
func (ContextMenuState) Escape() ContextMenuState { return false }

// OutsidePress closes the menu after the runtime identifies an outside press.
func (ContextMenuState) OutsidePress() ContextMenuState { return false }

// ContextMenuOptions supplies application-owned overlay identities.
type ContextMenuOptions struct {
	Layer     uint16
	Scope     gfx.ID
	DismissID gfx.ID
	// ID and Side are those of DropdownMenuOptions. A context menu appears and
	// disappears at once, as a native one does, so neither changes how it moves.
	ID   gfx.ID
	Side OverlaySide
	// Viewport, with DismissID, makes the menu declare a hit over the whole
	// window behind its surface, so pressing anywhere outside it reports
	// DismissID (Escape reports it too). The menu is also kept inside it: shifted
	// back when it sticks out, and shortened, so that it scrolls, when it is taller
	// than the window. Leave it empty to dismiss only on Escape and handle outside
	// presses yourself.
	Viewport gfx.Rect
	// Scroll is the scroll position of a menu whose items do not all fit its
	// bounds (see MenuScroll). Without one such a menu shows only the items at
	// its top.
	Scroll *MenuScroll
	// Submenu makes this the content of a submenu, which has a smaller minimum
	// width (128) than a context menu (144).
	Submenu bool
	// RowHeight is the height of an item row. Zero is the theme's control height
	// (32 at the default density, 26 at the compact one); a menu with many items
	// can ask for a denser one (24, say). Group labels, separators and everything
	// else keep their size. An icon never grows past the row.
	RowHeight float32
	// Unpainted, PaintPanel and PaintItem are DropdownMenuOptions' and mean the
	// same: the menu keeps its behavior and the caller draws its surface and rows.
	Unpainted  bool
	PaintPanel func(canvas *gfx.Canvas, bounds gfx.Rect, theme gfx.Theme)
	PaintItem  func(MenuItemPaint)
}

// ContextMenuItem describes one stable menu action.
type ContextMenuItem struct {
	ID    gfx.ID
	Label string
	// Description is an optional muted second line. It increases the item height
	// and is ellipsized to the available width; Hint may carry the full text.
	Description string
	Shortcut    string
	// Keys, when set, replace Shortcut with one keycap per entry (for example
	// "⌘", "⌥", "H"), painted with Kbd at the row's trailing edge.
	Keys []string
	// TrailingWidth reserves that much of the row's trailing edge for content the
	// caller paints itself (a switch, a badge, a small icon) with AddItemSlot and
	// BeginContent. Keys and Shortcut are not drawn when it is set.
	TrailingWidth float32
	Value         string
	Hint          string
	Kind          ContextMenuItemKind
	Variant       ContextMenuItemVariant
	Checked       bool
	Selected      bool
	Disabled      bool
	Inset         bool
	Expanded      bool
	// Icon, when it has any geometry, is painted before the label (after the
	// checkbox or radio indicator, if the item has one). A single-color icon takes
	// the item's text color; a multi-color one keeps its own.
	Icon gfx.Icon
}

func (item ContextMenuItem) common() menuItem {
	return menuItem{
		Description: item.Description,
		ID:          item.ID, Label: item.Label, Shortcut: item.Shortcut, Keys: item.Keys, TrailingWidth: item.TrailingWidth,
		Value: item.Value, Hint: item.Hint, Kind: menuKind(item.Kind), Destructive: item.Variant == ContextMenuItemDestructive,
		Checked: item.Checked, Selected: item.Selected, Disabled: item.Disabled, Inset: item.Inset, Expanded: item.Expanded, Icon: item.Icon,
	}
}

// ContextMenuItemWidth is the width a context menu needs for item's row to show
// its icon, label and trailing content whole: take the largest over the menu's
// items as the bounds' Width. The menu's own minimum still applies.
func ContextMenuItemWidth(canvas *gfx.Canvas, theme gfx.Theme, item ContextMenuItem) float32 {
	return menuItemWidth(canvas, theme, item.common(), false)
}

// ContextMenuItemHeight is the height AddItem reserves for item. rowHeight is
// ContextMenuOptions.RowHeight (zero selects the theme's control height).
func ContextMenuItemHeight(theme gfx.Theme, item ContextMenuItem, rowHeight float32) float32 {
	return menuItemHeight(theme, item.common(), rowHeight)
}

// ContextMenu paints an application-positioned menu without owning pointer,
// keyboard, or popup lifecycle state. PlaceOverlay, given a zero-size anchor at
// the pointer, places it with the flip a context menu needs next to a window edge.
type ContextMenu struct {
	surface menuSurface
}

// BeginContextMenu paints an open menu at the caller's context-menu position,
// using caller-owned layer, focus scope, and Escape action identities. Bounds are normalized to Vega's 9rem minimum
// (8rem for a submenu).
func BeginContextMenu(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, options ContextMenuOptions) ContextMenu {
	minWidth := float32(144)
	if options.Submenu {
		minWidth = 128
	}
	return ContextMenu{surface: beginMenuSurface(canvas, theme, bounds, minWidth, menuSurfaceOptions{
		layer: options.Layer, scope: options.Scope, dismissID: options.DismissID, viewport: options.Viewport, scroll: options.Scroll,
		key: overlayKey(options.ID, options.DismissID, options.Scope), side: options.Side, submenu: options.Submenu,
		instant: true, labelMuted: true, rowHeight: options.RowHeight,
		unpainted: options.Unpainted, paintPanel: options.PaintPanel, paintItem: options.PaintItem,
	})}
}

// Bounds returns the normalized overlay geometry.
func (menu ContextMenu) Bounds() gfx.Rect { return menu.surface.bounds }

// NextItemBounds returns the next keyboard-navigation row (RowHeight high)
// without advancing the declaration cursor.
func (menu ContextMenu) NextItemBounds() gfx.Rect {
	return menu.surface.rowBounds(menu.surface.itemHeight())
}

// AddLabel paints one medium-weight group label and returns its row geometry.
func (menu *ContextMenu) AddLabel(label string, inset bool) gfx.Rect {
	return menu.surface.addLabel(label, inset)
}

// AddItem paints one action, checkbox, radio option, or submenu trigger. The
// rectangle is empty when the row is scrolled out of the menu.
func (menu *ContextMenu) AddItem(item ContextMenuItem) gfx.Rect {
	bounds, _ := menu.AddItemSlot(item)
	return bounds
}

// AddItemSlot paints an item like AddItem and also returns the rectangle its
// TrailingWidth reserved, vertically centered in the row and ending 8px from its
// edge. The slot is empty when the item has no TrailingWidth or is scrolled out
// of the menu. Paint into it between BeginContent and EndContent.
func (menu *ContextMenu) AddItemSlot(item ContextMenuItem) (row, slot gfx.Rect) {
	return menu.surface.addItem(item.common())
}

// BeginContent enters the menu's layer and focus scope so the caller's own
// components, painted until EndContent, belong to the menu: above the page, inside
// its focus trap, and hit-tested over the row beneath.
func (menu *ContextMenu) BeginContent() { menu.surface.pushOverlay() }

// EndContent leaves the layer and focus scope BeginContent entered.
func (menu *ContextMenu) EndContent() { menu.surface.popOverlay() }

// AddSeparator paints the official -mx-1/my-1 divider geometry.
func (menu *ContextMenu) AddSeparator() gfx.Rect { return menu.surface.addSeparator() }

// End paints the scrollbar of a menu whose items did not all fit, after the last
// item: call it once the last item is painted.
func (menu *ContextMenu) End() { menu.surface.end() }
