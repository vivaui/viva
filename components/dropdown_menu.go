package components

import (
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/vivaui/viva/base"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
)

// DropdownMenuItemVariant selects the standard or destructive treatment.
type DropdownMenuItemVariant uint8

const (
	DropdownMenuItemDefault DropdownMenuItemVariant = iota
	DropdownMenuItemDestructive
)

// DropdownMenuItemKind selects the item's indicator and semantic state.
type DropdownMenuItemKind uint8

const (
	DropdownMenuAction DropdownMenuItemKind = iota
	DropdownMenuCheckbox
	DropdownMenuRadio
	DropdownMenuSubmenu
)

// DropdownMenuState is caller-owned popup visibility. Its zero value is closed.
// Transitions are synchronous value operations and never schedule work.
type DropdownMenuState bool

// IsOpen reports whether the dropdown menu should be painted.
func (state DropdownMenuState) IsOpen() bool { return bool(state) }

// Open returns the open state.
func (DropdownMenuState) Open() DropdownMenuState { return true }

// Close returns the closed state.
func (DropdownMenuState) Close() DropdownMenuState { return false }

// Toggle returns the opposite visibility state.
func (state DropdownMenuState) Toggle() DropdownMenuState { return !state }

// Dismiss closes the menu after selection or an application-owned dismiss action.
func (DropdownMenuState) Dismiss() DropdownMenuState { return false }

// Escape closes the menu after the runtime routes an Escape key press.
func (DropdownMenuState) Escape() DropdownMenuState { return false }

// OutsidePress closes the menu after the runtime identifies an outside press.
func (DropdownMenuState) OutsidePress() DropdownMenuState { return false }

// SubmenuState is caller-owned open submenu identity: the ID of the row whose
// submenu is open, or zero when none is. A submenu has its own state so Escape
// and Left can close it without closing the menu around it, and so a hover
// intent can move between siblings.
type SubmenuState gfx.ID

// IsOpen reports whether trigger's submenu is open.
func (state SubmenuState) IsOpen(trigger gfx.ID) bool {
	return !trigger.IsZero() && gfx.ID(state) == trigger
}

// IsAnyOpen reports whether any submenu is open.
func (state SubmenuState) IsAnyOpen() bool { return !gfx.ID(state).IsZero() }

// Trigger returns the row whose submenu is open, or zero.
func (state SubmenuState) Trigger() gfx.ID { return gfx.ID(state) }

// Open returns the state with trigger's submenu open. Zero is ignored.
func (state SubmenuState) Open(trigger gfx.ID) SubmenuState {
	if trigger.IsZero() {
		return state
	}
	return SubmenuState(trigger)
}

// Close returns the state with no submenu open.
func (SubmenuState) Close() SubmenuState { return SubmenuState{} }

// SubmenuSafe reports whether point is in the grace area between a submenu
// trigger and its open panel, so a diagonal pointer move does not close the
// submenu before it arrives. It is the box of the two rectangles with a small
// margin: more forgiving than a strict triangle, and it does not block pointer
// events outside the pair the way Base UI's safe polygon does.
func SubmenuSafe(trigger, submenu gfx.Rect, point gfx.Point) bool {
	if pointInRect(point, trigger) || pointInRect(point, submenu) {
		return true
	}
	const margin = float32(4)
	left := min(trigger.X, submenu.X) - margin
	right := max(trigger.X+trigger.Width, submenu.X+submenu.Width) + margin
	top := min(trigger.Y, submenu.Y) - margin
	bottom := max(trigger.Y+trigger.Height, submenu.Y+submenu.Height) + margin
	return point.X >= left && point.X <= right && point.Y >= top && point.Y <= bottom
}

func pointInRect(point gfx.Point, bounds gfx.Rect) bool {
	return point.X >= bounds.X && point.X < bounds.X+bounds.Width &&
		point.Y >= bounds.Y && point.Y < bounds.Y+bounds.Height
}

// SubmenuCommand is the level movement a key asks for.
type SubmenuCommand uint8

const (
	SubmenuCommandNone SubmenuCommand = iota
	// SubmenuCommandOpen opens the focused row's submenu (ArrowRight).
	SubmenuCommandOpen
	// SubmenuCommandClose closes the current submenu and returns to its row
	// (ArrowLeft).
	SubmenuCommandClose
)

// SubmenuCommandForKey maps the horizontal menu keys Base UI uses for submenu
// levels: Right opens, Left closes. The application moves focus after it acts:
// Right focuses the first row of the submenu, Left the row that opened it.
func SubmenuCommandForKey(key gfx.Key) (SubmenuCommand, bool) {
	switch key {
	case gfx.KeyRight:
		return SubmenuCommandOpen, true
	case gfx.KeyLeft:
		return SubmenuCommandClose, true
	default:
		return SubmenuCommandNone, false
	}
}

// DropdownMenuOptions supplies application-owned overlay identities.
type DropdownMenuOptions struct {
	Layer     uint16
	Scope     gfx.ID
	DismissID gfx.ID
	// ID names the menu for its entrance and exit when DismissID, the default,
	// does not tell it from another menu painted under the same Scope. Side is
	// the edge of its anchor the menu opens on (the zero value is under it), which
	// is the edge it slides in from. End has to be called for the menu to animate.
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
	// width (96) than a menu opened by a trigger (128).
	Submenu bool
	// Unpainted declares the menu's behavior, size and rows but paints none of
	// it: PaintPanel draws the surface and PaintItem draws each row, both inside
	// the menu's layer, clipping and focus scope. Hits, focus, the keyboard and
	// submenu expansion stay the menu's, so a menu of the application's own
	// cannot behave differently.
	Unpainted  bool
	PaintPanel func(canvas *gfx.Canvas, bounds gfx.Rect, theme gfx.Theme)
	PaintItem  func(MenuItemPaint)
}

// DropdownMenuItem describes one stable menu action.
type DropdownMenuItem struct {
	ID       gfx.ID
	Label    string
	Shortcut string
	// Keys, when set, replace Shortcut with one keycap per entry (for example
	// "⌘", "⌥", "H"), painted with Kbd at the row's trailing edge.
	Keys []string
	// TrailingWidth reserves that much of the row's trailing edge for content the
	// caller paints itself (a switch, a badge, a small icon) with AddItemSlot and
	// BeginContent. Keys and Shortcut are not drawn when it is set.
	TrailingWidth float32
	Value         string
	Hint          string
	// Description is an optional muted second line under the label. It grows the
	// row's height and is measured into the menu's width; Hint may carry the full
	// text for assistive technology.
	Description string
	Kind        DropdownMenuItemKind
	Variant     DropdownMenuItemVariant
	Checked     bool
	Selected    bool
	Disabled    bool
	Inset       bool
	Expanded    bool
	// Icon, when it has any geometry, is painted before the label (after the
	// checkbox or radio indicator, if the item has one). A single-color icon takes
	// the item's text color; a multi-color one keeps its own.
	Icon gfx.Icon
}

func (item DropdownMenuItem) common() menuItem {
	return menuItem{
		ID: item.ID, Label: item.Label, Shortcut: item.Shortcut, Keys: item.Keys, TrailingWidth: item.TrailingWidth,
		Value: item.Value, Hint: item.Hint, Description: item.Description, Kind: menuKind(item.Kind), Destructive: item.Variant == DropdownMenuItemDestructive,
		Checked: item.Checked, Selected: item.Selected, Disabled: item.Disabled, Inset: item.Inset, Expanded: item.Expanded, Icon: item.Icon,
	}
}

// DropdownMenuItemWidth is the width a dropdown menu needs for item's row to show
// its icon, label and trailing content whole: take the largest over the menu's
// items as the bounds' Width. The menu's own minimum still applies.
func DropdownMenuItemWidth(canvas *gfx.Canvas, theme gfx.Theme, item DropdownMenuItem) float32 {
	return menuItemWidth(canvas, theme, item.common(), false)
}

// DropdownMenuItemHeight is the height AddItem reserves for item.
func DropdownMenuItemHeight(theme gfx.Theme, item DropdownMenuItem) float32 {
	return menuItemHeight(theme, item.common(), 0)
}

// The heights AddLabel and AddSeparator reserve, and the padding of the surface
// around its rows: a menu's bounds are as tall as its rows and twice the padding.
const (
	DropdownMenuLabelHeight     = menuLabelHeight
	DropdownMenuSeparatorHeight = menuSeparatorHeight
	DropdownMenuPadding         = menuPadding
)

// DropdownMenu paints a source-owned menu surface. Selection, open state,
// placement, and keyboard routing remain application/runtime state.
type DropdownMenu struct {
	surface menuSurface
}

// BeginDropdownMenu paints an open menu using caller-owned layer, focus scope,
// and Escape action identities. Bounds are normalized to Vega's 8rem minimum
// (6rem for a submenu).
func BeginDropdownMenu(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, options DropdownMenuOptions) DropdownMenu {
	minWidth := float32(128)
	if options.Submenu {
		minWidth = 96
	}
	return DropdownMenu{surface: beginMenuSurface(canvas, theme, bounds, minWidth, menuSurfaceOptions{
		layer: options.Layer, scope: options.Scope, dismissID: options.DismissID, viewport: options.Viewport, scroll: options.Scroll,
		key: overlayKey(options.ID, options.DismissID, options.Scope), side: options.Side, submenu: options.Submenu,
		labelMuted: true, tabCloses: true,
		unpainted: options.Unpainted, paintPanel: options.PaintPanel, paintItem: options.PaintItem,
	})}
}

// Bounds returns the normalized overlay geometry.
func (menu DropdownMenu) Bounds() gfx.Rect { return menu.surface.bounds }

// NextItemBounds returns the next keyboard-navigation row, as tall as the
// theme's control height, without advancing the declaration cursor.
func (menu DropdownMenu) NextItemBounds() gfx.Rect {
	return menu.surface.rowBounds(menu.surface.itemHeight())
}

// AddLabel paints one medium-weight group label and returns its row geometry.
func (menu *DropdownMenu) AddLabel(label string, inset bool) gfx.Rect {
	return menu.surface.addLabel(label, inset)
}

// AddItem paints one action, checkbox, radio option, or submenu trigger. The
// rectangle is empty when the row is scrolled out of the menu.
func (menu *DropdownMenu) AddItem(item DropdownMenuItem) gfx.Rect {
	bounds, _ := menu.AddItemSlot(item)
	return bounds
}

// AddItemSlot paints an item like AddItem and also returns the rectangle its
// TrailingWidth reserved, vertically centered in the row and ending 8px from its
// edge. The slot is empty when the item has no TrailingWidth or is scrolled out
// of the menu. Paint into it between BeginContent and EndContent.
func (menu *DropdownMenu) AddItemSlot(item DropdownMenuItem) (row, slot gfx.Rect) {
	return menu.surface.addItem(item.common())
}

// BeginContent enters the menu's layer and focus scope so the caller's own
// components, painted until EndContent, belong to the menu: above the page, inside
// its focus trap, and hit-tested over the row beneath.
func (menu *DropdownMenu) BeginContent() { menu.surface.pushOverlay() }

// EndContent leaves the layer and focus scope BeginContent entered.
func (menu *DropdownMenu) EndContent() { menu.surface.popOverlay() }

// AddSeparator paints the official -mx-1/my-1 divider geometry.
func (menu *DropdownMenu) AddSeparator() gfx.Rect { return menu.surface.addSeparator() }

// AddContent reserves a row height tall, painted by the caller between BeginContent and
// EndContent. It is not an item: it takes no focus and no hit of its own.
func (menu *DropdownMenu) AddContent(height float32) gfx.Rect { return menu.surface.addContent(height) }

// End paints the scrollbar of a menu whose items did not all fit, after the last
// item, and ends the menu's entrance and exit: call it once the last item is
// painted, or the menu appears and disappears at once.
func (menu *DropdownMenu) End() { menu.surface.end() }

// The draggable scrollbar track is menuBarWidth wide and ends 2px inside the
// surface, over the thumb; the rows' hits end where it starts, menuBarInset short
// of their paint.
const (
	menuBarWidth = float32(6)
	menuBarInset = float32(4)
)

// menuTypeaheadCanCycle reports whether a repeated first letter may cycle
// through matches. Base UI does not cycle when a row's first two letters are
// the same ("Aaron", "llama"), because then a doubled letter is a real query.
func menuTypeaheadCanCycle(scroll *MenuScroll) bool {
	for _, item := range scroll.Items {
		if item.Disabled || item.ID.IsZero() {
			continue
		}
		first, firstSize := utf8.DecodeRuneInString(item.Label)
		if firstSize == 0 {
			continue
		}
		second, secondSize := utf8.DecodeRuneInString(item.Label[firstSize:])
		if secondSize > 0 && unicode.ToLower(first) == unicode.ToLower(second) {
			return false
		}
	}
	return true
}

// menuLabelHasPrefix reports whether label starts with prefix, comparing one
// rune at a time without lowercasing either string. Typeahead runs on a key
// press, so it should not allocate.
func menuLabelHasPrefix(label, prefix string) bool {
	for _, want := range prefix {
		got, size := utf8.DecodeRuneInString(label)
		if size == 0 || unicode.ToLower(got) != unicode.ToLower(want) {
			return false
		}
		label = label[size:]
	}
	return true
}

const (
	// menuRowHeight is the height of an item row in a theme without a control
	// height; a theme with one sizes its rows by it (menuThemeRowHeight).
	menuRowHeight       = float32(32)
	menuLabelHeight     = float32(28)
	menuSeparatorHeight = float32(9)
	menuPadding         = float32(4)
	// menuDescriptionGap is what separates an item's title from its description.
	// It is a line leading, not a row gap: the two lines belong to one row.
	menuDescriptionGap = float32(2)
	// menuDescriptionPadding is the air above the title and below the description
	// of a two-line item. It is larger than the gap between the lines so the pair
	// reads as one item and not as two rows that happen to be close.
	menuDescriptionPadding = float32(6)
	// menuIconSize and menuIconGap place an item icon: the label moves right by
	// their sum.
	menuIconSize = 16
	menuIconGap  = 8
)

// menuKind is the indicator and semantic state of a menu row; the Kind types of
// the dropdown menu, the context menu and the menubar all convert to it.
type menuKind uint8

const (
	menuAction menuKind = iota
	menuCheckbox
	menuRadio
	menuSubmenu
)

// menuItem is a menu row, whichever menu it belongs to.
type menuItem struct {
	ID            gfx.ID
	Label         string
	Description   string
	Shortcut      string
	Keys          []string
	TrailingWidth float32
	Value         string
	Hint          string
	Kind          menuKind
	Destructive   bool
	Checked       bool
	Selected      bool
	Disabled      bool
	Inset         bool
	Expanded      bool
	Icon          gfx.Icon
}

type menuSurfaceOptions struct {
	layer     uint16
	scope     gfx.ID
	dismissID gfx.ID
	// key names the menu for its entrance and exit, side is the edge of its anchor
	// it opens on, and submenu says it opens beside another menu of the same key.
	key      gfx.ID
	side     OverlaySide
	submenu  bool
	viewport gfx.Rect
	scroll   *MenuScroll
	// instant makes the menu appear and disappear at once, with no presence group.
	instant bool
	// labelMuted makes group labels the small muted ones of the dropdown and the
	// context menu; the menubar's are body text.
	labelMuted bool
	// indicatorLeft puts the check of a checkbox or radio row on the left, as the
	// menubar does; the other menus put it on the right.
	indicatorLeft bool
	// tabCloses is MenuScroll.TabCloses for this surface.
	tabCloses bool
	// rowHeight is the height of an item row; zero is the theme's
	// (menuThemeRowHeight).
	rowHeight float32
	// unpainted leaves the surface and rows to paintPanel and paintItem; see
	// DropdownMenuOptions.Unpainted.
	unpainted  bool
	paintPanel func(canvas *gfx.Canvas, bounds gfx.Rect, theme gfx.Theme)
	paintItem  func(MenuItemPaint)
}

// menuSurface is what the dropdown menu, the context menu and the menubar's popup
// have in common: the bordered surface, the rows laid out one under the other
// (scrolled, when the caller gave a MenuScroll), and the painting of a row.
type menuSurface struct {
	canvas  *gfx.Canvas
	theme   gfx.Theme
	bounds  gfx.Rect
	options menuSurfaceOptions
	// top and bottom are the edges of the visible rows; y is where the next row
	// goes, already moved up by the scroll offset; used is the rows' height so far.
	top, bottom float32
	// Keep the local height: subtracting absolute coordinates loses precision
	// at fractional origins and can create a scrollbar for an exactly fitting list.
	viewHeight float32
	y          float32
	used       float32
	offset     float32
	hidden     bool
	// animated says beginMenuSurface opened the menu's presence group, which end
	// has to close. A command list shares the surface and has none.
	animated bool
	// hitInset is how much shorter than their paint the rows' hits are, to leave
	// the scrollbar's own.
	hitInset float32
	// viewport, when it has an area, is the region the rows and the scrollbar are
	// painted and hit-tested in place of the clips around the caller: a surface
	// declared inside a scroll area is not cut by it, as Canvas.PushOverlayClip
	// suspends them for the duration of each row. The control's owner sets it; the
	// menus leave it empty and keep their ancestor clips.
	viewport gfx.Rect
}

func beginMenuSurface(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, minWidth float32, options menuSurfaceOptions) menuSurface {
	bounds.Width = max(bounds.Width, minWidth)
	if options.viewport.Width > 0 && options.viewport.Height > 0 {
		bounds = overlayClamp(bounds, options.viewport, menuPadding)
	}
	menu := menuSurface{canvas: canvas, theme: theme, bounds: bounds, options: options, hidden: canvas == nil || bounds.Height <= 0}
	menu.top, menu.bottom = bounds.Y+menuPadding, bounds.Y+bounds.Height-menuPadding
	menu.viewHeight = max(bounds.Height-2*menuPadding, 0)
	menu.beginScroll()
	if scroll := options.scroll; scroll != nil {
		scroll.TabCloses = options.tabCloses
	}
	if menu.hidden {
		return menu
	}
	menu.pushOverlay()
	// Claim after pushing the menu's scope: a menu inside a dialog is deeper
	// than the dialog and keeps the trap and Escape target when the dialog ends.
	if !options.scope.IsZero() {
		canvas.TrapFocus(options.scope)
	}
	if !options.dismissID.IsZero() {
		canvas.SetEscapeTarget(options.dismissID)
	}
	menu.paintDismissBackdrop()
	// The surface itself is inside the menu: a press on its padding, a label, a
	// separator, or a disabled row must not reach the dismiss backdrop behind it.
	// Saying the pointer is on the surface also tells a row whether the pointer is
	// on the menu at all: over the padding and the separators the pointer is on the
	// menu but on no row, and treating that as "outside" relit the row that kept
	// focus.
	menu.canvas.BlockSurface(bounds)
	menu.canvas.Container(bounds, gfx.RoleMenu, "")
	part := overlaySurfacePart
	if options.submenu {
		part = overlaySubmenuPart
	}
	if !options.instant {
		menu.animated = true
		canvas.BeginPresence(options.key, part, popupPresence(theme, overlayDuration, options.side), popupPivot(bounds, options.side, gfx.Rect{}))
	}
	level := gfx.ShadowMD // shadow-md; a submenu floats higher, shadow-lg
	if options.submenu {
		level = gfx.ShadowLG
	}
	if options.unpainted {
		if options.paintPanel != nil {
			options.paintPanel(canvas, bounds, theme)
		}
	} else {
		paintPopupSurface(canvas, &theme, bounds, level)
	}
	menu.popOverlay()
	return menu
}

// beginScroll starts a paint of the list's scroll position: it forgets the rows of
// the last paint, scrolls to the row the keyboard moved to, and takes the offset.
func (menu *menuSurface) beginScroll() {
	if scroll := menu.options.scroll; scroll != nil {
		if menu.canvas != nil {
			scroll.Follow(menu.canvas)
		}
		scroll.Items = scroll.Items[:0]
		scroll.View = menu.viewHeight
		if scroll.Measured {
			scroll.Offset = max(min(scroll.Offset, scroll.MaxValue), 0)
		}
		menu.offset = max(scroll.Offset, 0)
		if scroll.MaxValue > 0 && !scroll.BarID.IsZero() {
			menu.hitInset = menuBarInset
		}
	}
	menu.y = menu.top - menu.offset
}

// rowBounds is where a row of that height goes next.
func (menu menuSurface) rowBounds(height float32) gfx.Rect {
	return gfx.Rect{X: menu.bounds.X + menuPadding, Y: menu.y, Width: max(0, menu.bounds.Width-2*menuPadding), Height: height}
}

// takeItem reserves a row like take, and lists it with the scroll position as
// one the keyboard can move to and match by label.
func (menu *menuSurface) takeItem(row gfx.Rect, id gfx.ID, label string, disabled bool) bool {
	if scroll := menu.options.scroll; scroll != nil && !id.IsZero() {
		scroll.Items = append(scroll.Items, base.MenuScrollItem{ID: id, Label: label, Top: menu.used, Height: row.Height, Disabled: disabled})
	}
	return menu.take(row)
}

// take reserves a row and reports whether any of it shows.
func (menu *menuSurface) take(row gfx.Rect) bool {
	menu.y += row.Height
	menu.used += row.Height
	if scroll := menu.options.scroll; scroll != nil {
		scroll.MaxValue, scroll.Measured = max(menu.used-menu.viewHeight, 0), true
	}
	return !menu.hidden && row.Height > 0 && row.Y < menu.bottom && row.Y+row.Height > menu.top
}

// pushClip suspends the clips around the surface for a viewport-anchored one, so a
// row is not cut by an ancestor scroll area, as Select sets one for its popup. A
// surface without a viewport keeps its ancestor clips, as the menus do.
func (menu *menuSurface) pushClip() {
	if menu.viewport.Width > 0 && menu.viewport.Height > 0 {
		menu.canvas.PushOverlayClip(menu.viewport)
	}
}

// popClip closes the clip pushClip opened.
func (menu *menuSurface) popClip() {
	if menu.viewport.Width > 0 && menu.viewport.Height > 0 {
		menu.canvas.PopOverlayClip()
	}
}

// paintRow runs paint for a row on the menu's layer and scope, clipped to the
// visible part of the surface when the row is cut by an edge.
func (menu *menuSurface) paintRow(row gfx.Rect, paint func()) {
	menu.pushClip()
	defer menu.popClip()
	menu.pushOverlay()
	cut := row.Y < menu.top || row.Y+row.Height > menu.bottom
	if cut {
		menu.canvas.Clip(gfx.Rect{X: menu.bounds.X, Y: menu.top, Width: menu.bounds.Width, Height: menu.bottom - menu.top})
	}
	paint()
	if cut {
		menu.canvas.Unclip()
	}
	menu.popOverlay()
}

// itemHeight is the height of an item row.
func (menu *menuSurface) itemHeight() float32 {
	if menu.options.rowHeight > 0 {
		return menu.options.rowHeight
	}
	return menuThemeRowHeight(menu.theme)
}

// menuThemeRowHeight is the height of an item row in theme: its control
// height, so a compact theme packs its menus as densely as its buttons and
// inputs (32 at the default density, 26 at the compact one), and menuRowHeight
// for a theme without one.
func menuThemeRowHeight(theme gfx.Theme) float32 {
	if height := theme.Metrics.ControlHeight; height > 0 {
		return height
	}
	return menuRowHeight
}

func (menu *menuSurface) addLabel(label string, inset bool) gfx.Rect {
	height := menu.itemHeight()
	size, weight, color := menu.theme.Metrics.FontSizeM, gfx.WeightMedium, menu.theme.PopoverForeground
	if menu.options.labelMuted {
		height, size, color = menuLabelHeight, menu.theme.Metrics.FontSizeS, menu.theme.MutedForeground
	}
	row := menu.rowBounds(height)
	if !menu.take(row) {
		return gfx.Rect{}
	}
	left := float32(8)
	if inset {
		left = 32
	}
	if menu.options.unpainted {
		menu.paintRow(row, func() {
			if paint := menu.options.paintItem; paint != nil {
				paint(MenuItemPaint{Canvas: menu.canvas, Theme: menu.theme, Bounds: row, Label: label, GroupLabel: true, Foreground: color})
			}
		})
		return row
	}
	menu.paintRow(row, func() {
		menu.canvas.Label(label, gfx.Rect{X: row.X + left, Y: row.Y, Width: max(0, row.Width-left-8), Height: row.Height}, gfx.TextStyle{
			Size: size, Weight: weight, Color: color,
		})
	})
	return row
}

// addContent reserves a row height tall for the caller to paint into, which the menu
// paints nothing of; it is not an item and takes no focus. It returns the row, or the zero
// rectangle when scrolling has it out of sight.
func (menu *menuSurface) addContent(height float32) gfx.Rect {
	row := menu.rowBounds(height)
	if !menu.take(row) {
		return gfx.Rect{}
	}
	return row
}

func (menu *menuSurface) addSeparator() gfx.Rect {
	row := gfx.Rect{X: menu.bounds.X, Y: menu.y, Width: menu.bounds.Width, Height: menuSeparatorHeight}
	if !menu.take(row) {
		return gfx.Rect{}
	}
	if menu.options.unpainted {
		menu.paintRow(row, func() {
			if paint := menu.options.paintItem; paint != nil {
				paint(MenuItemPaint{Canvas: menu.canvas, Theme: menu.theme, Bounds: row, Separator: true, Foreground: menu.theme.Border})
			}
		})
		return row
	}
	menu.paintRow(row, func() {
		menu.canvas.Fill(gfx.Rect{X: row.X, Y: row.Y + 4, Width: row.Width, Height: 1}, menu.theme.Border)
	})
	return row
}

// addItem lays a row out and paints it. The slot is the room TrailingWidth
// reserved, when the whole row shows.
func (menu *menuSurface) addItem(item menuItem) (row, slot gfx.Rect) {
	row = menu.rowBounds(menuItemHeight(menu.theme, item, menu.options.rowHeight))
	if !menu.takeItem(row, item.ID, item.Label, item.Disabled) {
		return gfx.Rect{}, gfx.Rect{}
	}
	menu.paintRow(row, func() { menu.paintItem(row, item) })
	if item.TrailingWidth > 0 && row.Y >= menu.top && row.Y+row.Height <= menu.bottom {
		slot = gfx.Rect{X: row.X + row.Width - menu.trailingEdge(item) - item.TrailingWidth, Y: row.Y, Width: item.TrailingWidth, Height: row.Height}
	}
	return row, slot
}

// trailingEdge is how far from the row's right edge the trailing content (a
// shortcut, keys, a caller's slot) ends: the row's padding, and room for the check
// that sits there in the menus that put it on the right.
func (menu *menuSurface) trailingEdge(item menuItem) float32 {
	if (item.Kind == menuCheckbox || item.Kind == menuRadio) && !menu.options.indicatorLeft {
		return 32
	}
	return 8
}

// menuItemWidth is the width of a menu whose row shows item with nothing cut,
// mirroring the layout of paintItem: the left offset, the icon, the label, then
// what is reserved at the trailing edge, between the surface's padding.
func menuItemWidth(canvas *gfx.Canvas, theme gfx.Theme, item menuItem, indicatorLeft bool) float32 {
	indicator := item.Kind == menuCheckbox || item.Kind == menuRadio
	left := float32(8)
	if item.Inset || indicator && indicatorLeft {
		left = 32
	}
	right := float32(8)
	if indicator && !indicatorLeft {
		right = 32
	}
	switch {
	case item.TrailingWidth > 0:
		right += item.TrailingWidth + 8
	case len(item.Keys) > 0:
		right += menuKeysWidth(canvas, theme, item.Keys) + 12
	case item.Shortcut != "":
		right += canvas.TextWidth(item.Shortcut, theme.Metrics.FontSizeS) + 12
	case item.Kind == menuSubmenu:
		right += menuIconSize + 4
	}
	if len(item.Icon.Commands) > 0 || len(item.Icon.Layers) > 0 {
		left += menuIconSize + menuIconGap
	}
	textWidth := canvas.TextWidth(item.Label, theme.Metrics.FontSizeM)
	if item.Description != "" {
		textWidth = max(textWidth, canvas.TextWidth(item.Description, theme.Metrics.FontSizeS))
	}
	return left + textWidth + right + 2*menuPadding
}

// menuLineHeight is the height one line of type occupies at size, using the
// FontSize+7 line the field, alert and dialog components already share.
func menuLineHeight(size float32) float32 { return size + 7 }

func menuItemHeight(theme gfx.Theme, item menuItem, rowHeight float32) float32 {
	if rowHeight <= 0 {
		rowHeight = menuThemeRowHeight(theme)
	}
	if item.Description != "" {
		lines := menuLineHeight(theme.Metrics.FontSizeM) + menuDescriptionGap + menuLineHeight(theme.Metrics.FontSizeS)
		rowHeight = max(rowHeight, lines+2*menuDescriptionPadding)
	}
	return rowHeight
}

// itemState is what a row does before it is painted: the pointer moves the
// menu's current row the way Radix does, entering a row focuses it, and the
// returned colors are the menu's own for the label. Both the painted and the
// unpainted row use it, so a menu of the application's own behaves the same.
func (menu *menuSurface) itemState(item menuItem) (active bool, foreground gfx.Color) {
	canvas, theme := menu.canvas, menu.theme
	enabled := !item.ID.IsZero() && !item.Disabled
	hovered := canvas.Hovered()
	claim := canvas.HoverCrossed() && !hovered.IsZero()
	// No row is the menu's current one while the pointer is on the menu but not on
	// a row. That covers the pointer over caller content inside the menu (a colour
	// row, a switch), which declares a role that is not a row's, and the pointer
	// over the menu's own padding and separators, which report the surface rather
	// than a hit. Without the second case the row that kept focus relit whenever the
	// pointer came back into the menu anywhere but on a row.
	//
	// Visible keyboard focus outranks a resting pointer, wherever it rests: a key
	// press moves the menu's current row while the pointer sits on the padding or on
	// a control of the caller's, and that row must stay lit. Focus that the pointer
	// itself moved is not visible, so the row it entered still gives way to whatever
	// is under the pointer.
	inContent := canvas.FocusVisible().IsZero() &&
		(canvas.PointerOnSurface() ||
			!hovered.IsZero() && canvas.HoveredRole() != 0 && canvas.HoveredRole() != gfx.RoleMenuItem)
	if claim && hovered == item.ID {
		// A disabled row cannot take focus, but the frame is still repainted so
		// the row that kept focus is tinted again on the next paint.
		if enabled {
			canvas.RequestFocus(item.ID)
		}
		canvas.RequestFrame()
	}
	// Focused, not FocusVisible: a menu row the pointer focused is the current
	// row even though no key press made its focus ring visible.
	active = enabled && !inContent && (claim && hovered == item.ID || !claim && canvas.Focused() == item.ID)
	if item.Kind == menuSubmenu && item.Expanded {
		active = true
	}
	foreground = theme.PopoverForeground
	if item.Destructive {
		foreground = theme.Danger
	} else if active {
		foreground = theme.AccentForeground
	}
	if item.Disabled {
		foreground.Alpha *= 0.50
	}
	return active, foreground
}

// declareItemHit gives a row the pointer, keyboard and assistive-technology
// behavior each path needs, painted or not.
func (menu *menuSurface) declareItemHit(bounds gfx.Rect, item menuItem) {
	if item.ID.IsZero() {
		return
	}
	hit := bounds
	hit.Width -= menu.hitInset
	menu.canvas.HitWithOptions(item.ID, hit, item.Label, gfx.HitOptions{
		Role: gfx.RoleMenuItem, Value: item.Value, Hint: item.Hint, Description: item.Description,
		Disabled: item.Disabled, Checked: item.Kind == menuCheckbox && item.Checked,
		Selected: item.Kind == menuRadio && item.Selected,
		Expanded: item.Kind == menuSubmenu && item.Expanded, Expandable: item.Kind == menuSubmenu,
	})
}

func (menu *menuSurface) paintItem(bounds gfx.Rect, item menuItem) {
	canvas, theme := menu.canvas, menu.theme
	active, foreground := menu.itemState(item)
	if menu.options.unpainted {
		if paint := menu.options.paintItem; paint != nil {
			paint(MenuItemPaint{
				Canvas: canvas, Theme: theme, Bounds: bounds,
				Label: item.Label, Description: item.Description, Shortcut: item.Shortcut,
				Checked:  item.Kind == menuCheckbox && item.Checked,
				Selected: item.Kind == menuRadio && item.Selected,
				Expanded: item.Kind == menuSubmenu && item.Expanded,
				Disabled: item.Disabled, Active: active, Destructive: item.Destructive,
				Foreground: foreground,
			})
		}
		menu.declareItemHit(bounds, item)
		return
	}
	if active {
		fill := theme.Accent
		if item.Destructive {
			fill = theme.Danger
			fill.Alpha = 0.10
		}
		canvas.Rounded(bounds, theme.Metrics.RadiusItem(), fill)
	}
	// A row with a description is two lines of type, not a taller one-line row:
	// the title keeps the system line height, the description follows one small
	// gap below it, and the pair is centered in the row. The indicator, icon and
	// trailing content anchor to the title line, so a check mark never floats
	// between the two lines.
	labelY, labelHeight := bounds.Y, bounds.Height
	descriptionY, descriptionHeight := bounds.Y+bounds.Height, float32(0)
	if item.Description != "" {
		titleLine := menuLineHeight(theme.Metrics.FontSizeM)
		detailLine := menuLineHeight(theme.Metrics.FontSizeS)
		pad := max((bounds.Height-titleLine-menuDescriptionGap-detailLine)/2, 0)
		labelY, labelHeight = bounds.Y+pad, titleLine
		descriptionY = labelY + titleLine + menuDescriptionGap
		descriptionHeight = min(detailLine, max(bounds.Height-(descriptionY-bounds.Y), 0))
	}
	indicator := item.Kind == menuCheckbox || item.Kind == menuRadio
	left := float32(8)
	if item.Inset || indicator && menu.options.indicatorLeft {
		left = 32
	}
	edge := menu.trailingEdge(item)
	right := edge
	switch {
	case item.TrailingWidth > 0:
		right += item.TrailingWidth + 8
	case len(item.Keys) > 0:
		keysWidth := menuKeysWidth(canvas, theme, item.Keys)
		paintMenuKeys(canvas, theme, gfx.Rect{X: bounds.X + bounds.Width - edge - keysWidth, Y: labelY, Width: keysWidth, Height: labelHeight}, item.Keys, item.Disabled)
		right += keysWidth + 12
	case item.Shortcut != "":
		shortcutWidth := canvas.TextWidth(item.Shortcut, theme.Metrics.FontSizeS)
		canvas.Label(item.Shortcut, gfx.Rect{X: bounds.X + bounds.Width - edge - shortcutWidth, Y: labelY, Width: shortcutWidth, Height: labelHeight}, gfx.TextStyle{
			Size: theme.Metrics.FontSizeS, Weight: gfx.WeightRegular, Color: dropdownMenuOpacity(theme.MutedForeground, item.Disabled),
		})
		right += shortcutWidth + 12
	case item.Kind == menuSubmenu:
		paintChevron(canvas, &chevronRightIcon, gfx.Rect{X: bounds.X + bounds.Width - 8 - menuIconSize, Y: labelY, Width: menuIconSize, Height: labelHeight}, foreground)
		right += menuIconSize + 4
	}
	if item.Kind == menuCheckbox && item.Checked || item.Kind == menuRadio && item.Selected {
		x := bounds.X + bounds.Width - 8 - menuIconSize
		if menu.options.indicatorLeft {
			x = bounds.X + 8
		}
		paintCheck(canvas, gfx.Rect{X: x, Y: labelY, Width: menuIconSize, Height: labelHeight}, menuIconSize, foreground)
	}
	if len(item.Icon.Commands) > 0 || len(item.Icon.Layers) > 0 {
		side := min(menuIconSize, labelHeight)
		canvas.DrawIcon(item.Icon, gfx.Rect{X: bounds.X + left, Y: labelY + (labelHeight-side)/2, Width: side, Height: side}, foreground)
		left += menuIconSize + menuIconGap
	}
	labelWidth := max(0, bounds.Width-left-right)
	label := item.Label
	if canvas.TextWidthWeight(label, theme.Metrics.FontSizeM, gfx.WeightRegular) > labelWidth {
		label = canvas.FitLine(label, theme.Metrics.FontSizeM, gfx.WeightRegular, labelWidth)
	}
	canvas.Label(label, gfx.Rect{X: bounds.X + left, Y: labelY, Width: labelWidth, Height: labelHeight}, gfx.TextStyle{
		Size: theme.Metrics.FontSizeM, Weight: gfx.WeightRegular, Color: foreground,
	})
	if item.Description != "" {
		color := theme.MutedForeground
		if active {
			color = foreground
		}
		color = dropdownMenuOpacity(color, item.Disabled)
		width := max(0, bounds.Width-left-right-menu.hitInset)
		value := canvas.FitLine(item.Description, theme.Metrics.FontSizeS, gfx.WeightRegular, width)
		canvas.Label(value, gfx.Rect{X: bounds.X + left, Y: descriptionY, Width: width, Height: descriptionHeight}, gfx.TextStyle{
			Size: theme.Metrics.FontSizeS, Weight: gfx.WeightRegular, Color: color,
		})
	}
	menu.declareItemHit(bounds, item)
}

// end paints a thin scrollbar thumb on the right edge when rows are scrolled out,
// and ends what the menu animates in and out.
func (menu *menuSurface) end() {
	if menu.hidden {
		return
	}
	if scroll := menu.options.scroll; scroll != nil && scroll.MaxValue > 0 {
		view := menu.viewHeight
		thumb := max(view*view/(view+scroll.MaxValue), 18)
		at := menu.top + (view-thumb)*scroll.Offset/scroll.MaxValue
		scroll.BarTop, scroll.Thumb = menu.top, thumb
		menu.pushClip()
		menu.pushOverlay()
		if !menu.options.unpainted {
			menu.canvas.Rounded(gfx.Rect{X: menu.bounds.X + menu.bounds.Width - 6, Y: at, Width: 4, Height: thumb}, 2, menu.theme.Border)
		}
		if !scroll.BarID.IsZero() {
			menu.canvas.HitWithOptions(scroll.BarID, gfx.Rect{X: menu.bounds.X + menu.bounds.Width - 2 - menuBarWidth, Y: menu.top, Width: menuBarWidth, Height: view},
				"Scroll", gfx.HitOptions{Role: gfx.RoleSlider, SkipFocus: true})
		}
		menu.popOverlay()
		menu.popClip()
	}
	if menu.animated {
		menu.canvas.EndPresence()
	}
}

func (menu menuSurface) pushOverlay() {
	menu.canvas.PushLayer(menu.options.layer)
	if !menu.options.scope.IsZero() {
		menu.canvas.PushFocusScope(menu.options.scope)
	}
}

func (menu menuSurface) popOverlay() {
	if !menu.options.scope.IsZero() {
		menu.canvas.PopFocusScope()
	}
	menu.canvas.PopLayer()
}

// paintDismissBackdrop declares the window-wide dismiss hit under the surface.
// It is not focusable and sits in the menu's layer and scope, so nothing behind
// the menu can be reached while it is open.
func (menu menuSurface) paintDismissBackdrop() {
	if menu.options.dismissID.IsZero() || menu.options.viewport.Width <= 0 || menu.options.viewport.Height <= 0 {
		return
	}
	menu.canvas.HitWithOptions(menu.options.dismissID, menu.options.viewport, "Dismiss menu", gfx.HitOptions{Role: gfx.RoleButton, SkipFocus: true, Backdrop: true, PassSecondary: true})
}

func dropdownMenuOpacity(color gfx.Color, disabled bool) gfx.Color {
	if disabled {
		color.Alpha *= 0.50
	}
	return color
}

// SubmenuHover advances one submenu's hover intent. state is the open submenu,
// pointer the current pointer, and triggerRect/panelRect their geometry when it
// is open. A pointer over another trigger row opens that row's submenu after
// delay; a pointer in the grace area between the open trigger and its panel (see
// SubmenuSafe) keeps it open; a pointer outside both closes it at once, as Base
// UI's default close delay does. It returns the state and how long to wait
// before asking again (zero when nothing is pending).
func SubmenuHover(state SubmenuState, hovered gfx.ID, pointer gfx.Point, hoveredRect, openTriggerRect, panelRect gfx.Rect, now time.Time, intent *HoverIntent, delay time.Duration) (SubmenuState, time.Duration) {
	if !state.IsAnyOpen() {
		intent.Reset()
		return state, 0
	}
	if !hovered.IsZero() && !state.IsOpen(hovered) {
		id, wait := intent.Rest(hovered, now, delay)
		if id.IsZero() {
			return state, wait
		}
		return state.Open(id), 0
	}
	if hovered.IsZero() && !SubmenuSafe(openTriggerRect, panelRect, pointer) {
		intent.Reset()
		return state.Close(), 0
	}
	intent.Reset()
	return state, 0
}

// MenuItemLook is the look of one row of a list in a popup: the highlight under the pointer or
// the keyboard, and the color of its label.
type MenuItemLook struct {
	// Fill is the highlight behind the row, transparent while the row is not active.
	Fill gfx.Color
	// Foreground is the color of the label and icon.
	Foreground gfx.Color
	// Radius is the corner radius of the highlight.
	Radius float32
}

// MenuItemRecipe resolves the look of a row of a popup list. The row is active, and takes the
// accent highlight, while the pointer is over it or presses it or the keyboard has focused it
// visibly, unless it is Disabled, which dims the label and never highlights. A pure function.
func MenuItemRecipe(theme gfx.Theme, state gfx.State) MenuItemLook {
	return menuItemRecipe(&theme, state)
}

// menuItemRecipe is the recipe for a theme that is not to be copied: the controls paint every frame,
// and a Theme is large.
func menuItemRecipe(theme *gfx.Theme, state gfx.State) MenuItemLook {
	look := MenuItemLook{Foreground: theme.PopoverForeground, Radius: theme.Metrics.RadiusItem()}
	if state.Disabled {
		look.Foreground.Alpha *= 0.50
	} else if state.Hovered || state.Pressed || state.FocusVisible {
		look.Fill = theme.Accent
	}
	return look
}

// Aliases into the style vocabulary, kept for callers of this component.
type MenuItemPaint = style.MenuItemPaint

// Aliases into the style vocabulary.
type HierarchyMenuItem = base.HierarchyMenuItem
type HierarchyMenuAction = base.HierarchyMenuAction
type HierarchyMenuState = base.HierarchyMenuState
type HierarchyMenuKeyResult = base.HierarchyMenuKeyResult
type HierarchyMenuOptions = base.HierarchyMenuOptions
type MenuScroll = base.MenuScroll
type MenuScrollItem = base.MenuScrollItem
type PointerRow = base.PointerRow
