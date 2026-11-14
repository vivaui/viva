package components

import (
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/vivaui/viva/gfx"
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

// End paints the scrollbar of a menu whose items did not all fit, after the last
// item, and ends the menu's entrance and exit: call it once the last item is
// painted, or the menu appears and disappears at once.
func (menu *DropdownMenu) End() { menu.surface.end() }

// MenuScroll is the caller-owned scroll position of a menu or command list whose
// items do not all fit its bounds, as `overflow-y-auto` is the browser's. Keep one
// per open list, hand its address to the list's options, and feed wheel gestures
// that happen over the list to Wheel:
//
//	var scroll components.MenuScroll // reset to its zero value when the list opens
//	...
//	menu := components.BeginDropdownMenu(canvas, theme, bounds, components.DropdownMenuOptions{Scroll: &scroll})
//	... AddItem ...
//	menu.End() // paints the scrollbar
//	...
//	config.OnScroll = func(event gfx.ScrollEvent) gfx.EventResult {
//		if menuOpen && pointInRect(event.X, event.Y, menuBounds) {
//			return gfx.EventResult{Handled: true, Repaint: scroll.Wheel(event)}
//		}
//		...
//	}
//
// Rows that are scrolled out declare no hit, so Tab and the pointer cannot reach
// them; the arrow keys can, through Navigate, and a row the keyboard focuses is
// scrolled into view by the next paint. Set BarID to make the scrollbar draggable.
//
// The list measures its content as it paints, so Offset is clamped to what the
// last paint found. Set Offset to zero when the items change.
type MenuScroll struct {
	// Offset is how far the content has moved up, in pixels.
	Offset float32
	// BarID, when set, makes the scrollbar a hit of that identity so the thumb can
	// be dragged (see Drag). The rows' hits then end where the bar starts.
	BarID gfx.ID
	max   float32
	// measured says a paint has found max, so Offset can be held to it; before
	// that an Offset the caller set stands.
	measured bool
	// view is the height of the room the rows show in; items are the rows with
	// an identity, as the last paint laid them out in content coordinates.
	view  float32
	items []menuScrollItem
	// TabCloses makes Tab close the list and continue through the page, as a
	// dropdown or select does. A context menu leaves it false, because Base UI
	// keeps Tab inside a context menu.
	TabCloses bool
	// followed is the keyboard focus the list last scrolled to.
	followed gfx.ID
	// barTop and thumb place the thumb of the last paint; grab is where in the
	// thumb a drag holds it.
	barTop, thumb, grab float32
	dragging            bool
}

// menuScrollItem is a selectable row: what it says and where it lies in the
// content, so the keyboard can also match it by name.
type menuScrollItem struct {
	id          gfx.ID
	label       string
	top, height float32
	disabled    bool
}

// The draggable scrollbar track is menuBarWidth wide and ends 2px inside the
// surface, over the thumb; the rows' hits end where it starts, menuBarInset short
// of their paint.
const (
	menuBarWidth = float32(6)
	menuBarInset = float32(4)
)

// Max is how far the list can scroll: how much of its content the last paint
// did not fit. It is zero until the list has painted, and for a list that fits.
func (scroll *MenuScroll) Max() float32 { return scroll.max }

// Wheel scrolls by one scroll gesture and reports whether the position changed.
// A notch of a wheel (a gesture that is not Precise) moves at least 24 pixels.
func (scroll *MenuScroll) Wheel(event gfx.ScrollEvent) bool {
	step := event.DeltaY
	if !event.Precise {
		if step > 0 && step < 24 {
			step = 24
		} else if step < 0 && step > -24 {
			step = -24
		}
	}
	return scroll.By(step)
}

// By scrolls by step pixels (positive reveals what is below) and reports whether
// the position changed.
func (scroll *MenuScroll) By(step float32) bool {
	before := scroll.Offset
	scroll.Offset = max(min(before+step, scroll.max), 0)
	return scroll.Offset != before
}

// Reveal scrolls the least that brings the item with that identity fully into
// view, as the last paint laid the list out, and reports whether the position
// changed. The first item scrolls to the top, so a group label above it shows,
// and the last to the end. An identity the list did not paint changes nothing.
func (scroll *MenuScroll) Reveal(id gfx.ID) bool {
	if id.IsZero() {
		return false
	}
	for index := range scroll.items {
		if scroll.items[index].id == id {
			return scroll.revealAt(index)
		}
	}
	return false
}

func (scroll *MenuScroll) revealAt(index int) bool {
	item := scroll.items[index]
	target := scroll.Offset
	switch {
	case index == 0:
		target = 0
	case item.top < scroll.Offset:
		target = item.top
	case item.top+item.height > scroll.Offset+scroll.view:
		target = item.top + item.height - scroll.view
		if index == len(scroll.items)-1 {
			target = scroll.max
		}
	}
	before := scroll.Offset
	scroll.Offset = max(min(target, scroll.max), 0)
	return scroll.Offset != before
}

// Navigate is the arrow-key movement of a list: Up and Down step to the previous
// and next enabled item, Home and End go to the first and last, PageUp and
// PageDown move a viewport's height. It returns the identity of the item the key
// leads to from the item from (the zero identity when none has the keyboard) and
// scrolls it into view; ask the runtime to focus it:
//
//	if id, ok := scroll.Navigate(event.Key, event.Target, true); ok {
//		return gfx.EventResult{Handled: true, Repaint: true, Focus: id, FocusVisible: true}
//	}
//
// loop makes Down past the last item arrive at the first, and Up past the first
// at the last, as a menu does; a list that does not loop stays on its edge. It
// reports false for any other key, and for a list without an enabled item.
func (scroll *MenuScroll) Navigate(key gfx.Key, from gfx.ID, loop bool) (gfx.ID, bool) {
	switch key {
	case gfx.KeyUp, gfx.KeyDown, gfx.KeyHome, gfx.KeyEnd, gfx.KeyPageUp, gfx.KeyPageDown:
	default:
		return gfx.ID{}, false
	}
	first, last, current := -1, -1, -1
	for index, item := range scroll.items {
		if item.id == from {
			current = index
		}
		if item.disabled {
			continue
		}
		if first < 0 {
			first = index
		}
		last = index
	}
	if first < 0 {
		return gfx.ID{}, false
	}
	next := func(from, step int) int {
		for index := from + step; index >= 0 && index < len(scroll.items); index += step {
			if !scroll.items[index].disabled {
				return index
			}
		}
		return -1
	}
	target := -1
	switch {
	case key == gfx.KeyHome:
		target = first
	case key == gfx.KeyEnd:
		target = last
	case current < 0:
		target = first
		if key == gfx.KeyUp {
			target = last
		}
	case key == gfx.KeyDown:
		if target = next(current, 1); target < 0 {
			target = current
			if loop {
				target = first
			}
		}
	case key == gfx.KeyUp:
		if target = next(current, -1); target < 0 {
			target = current
			if loop {
				target = last
			}
		}
	case key == gfx.KeyPageDown:
		reach := scroll.items[current].top + scroll.view
		target = current
		for index := next(current, 1); index >= 0 && scroll.items[index].top <= reach; index = next(index, 1) {
			target = index
		}
		if target == current {
			target = next(current, 1)
		}
		if target < 0 {
			target = last
		}
	case key == gfx.KeyPageUp:
		reach := scroll.items[current].top - scroll.view
		target = current
		for index := next(current, -1); index >= 0 && scroll.items[index].top >= reach; index = next(index, -1) {
			target = index
		}
		if target == current {
			target = next(current, -1)
		}
		if target < 0 {
			target = first
		}
	}
	scroll.revealAt(target)
	return scroll.items[target].id, true
}

// Match returns the first enabled row whose label starts with prefix, ignoring
// case, searching after from and wrapping once, and scrolls it into view. It is
// Base UI's menu and select typeahead: the caller owns the typed text and when a
// session starts or expires (Base UI resets it after 500ms), then hands each
// printable key to Match. Passing the item the keyboard or selection started on
// as from starts at the next row, as a new session does; passing the previous
// match continues after it, which is how a repeated first letter cycles through
// same-initial rows. It reports false for an empty prefix or when nothing among
// the last paint's rows matches.
func (scroll *MenuScroll) Match(prefix string, from gfx.ID) (gfx.ID, bool) {
	if prefix == "" || len(scroll.items) == 0 {
		return gfx.ID{}, false
	}
	start := 0
	if !from.IsZero() {
		for index := range scroll.items {
			if scroll.items[index].id == from {
				start = index + 1
				break
			}
		}
	}
	for offset := 0; offset < len(scroll.items); offset++ {
		index := (start + offset) % len(scroll.items)
		item := scroll.items[index]
		if item.id.IsZero() || item.disabled || item.label == "" {
			continue
		}
		if menuLabelHasPrefix(item.label, prefix) {
			scroll.revealAt(index)
			return item.id, true
		}
	}
	return gfx.ID{}, false
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

// Dragging reports whether the thumb is held, so pointer events that follow it
// out of the scrollbar still belong to Drag.
func (scroll *MenuScroll) Dragging() bool { return scroll.dragging }

// Drag drags the thumb of a list whose BarID is set: feed it the pointer events
// whose Target is BarID, and, while Dragging, every pointer event. A press on the
// thumb holds it where it was grabbed; a press on the track elsewhere centers the
// thumb on the pointer. It reports whether the position changed, to repaint.
func (scroll *MenuScroll) Drag(event gfx.PointerEvent) bool {
	switch event.Kind {
	case gfx.PointerDown:
		if scroll.BarID.IsZero() || event.Target != scroll.BarID || scroll.max <= 0 {
			return false
		}
		at := scroll.barTop + (scroll.view-scroll.thumb)*scroll.Offset/scroll.max
		scroll.grab = scroll.thumb / 2
		if event.Y >= at && event.Y < at+scroll.thumb {
			scroll.grab = event.Y - at
		}
		scroll.dragging = true
		return scroll.dragTo(event.Y)
	case gfx.PointerMove:
		return scroll.dragging && scroll.dragTo(event.Y)
	default:
		scroll.dragging = false
		return false
	}
}

func (scroll *MenuScroll) dragTo(y float32) bool {
	travel := scroll.view - scroll.thumb
	if travel <= 0 {
		return false
	}
	before := scroll.Offset
	scroll.Offset = max(min((y-scroll.grab-scroll.barTop)/travel*scroll.max, scroll.max), 0)
	return scroll.Offset != before
}

// follow scrolls the item the keyboard has just moved to into view, before the
// list is laid out. A wheel gesture that leaves the focus where it was is not
// undone.
func (scroll *MenuScroll) follow(canvas *gfx.Canvas) {
	focus := canvas.FocusVisible()
	if focus == scroll.followed {
		return
	}
	scroll.followed = focus
	scroll.Reveal(focus)
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
}

// pointerRow remembers which option row the pointer is over so a list can move
// its keyboard cursor onto that row. It follows the pointer only when the
// pointer crosses onto a different row: a pointer that rests where it already
// was never takes the cursor back from the keyboard, so arrow keys keep
// working while the pointer is parked over the list.
type pointerRow struct{ row gfx.ID }

// entered records row as the row under the pointer and reports whether the
// pointer has just crossed onto it. Pass the zero ID, once per paint, to record
// that no option is under the pointer.
func (p *pointerRow) entered(row gfx.ID) bool {
	if row == p.row {
		return false
	}
	p.row = row
	return !row.IsZero()
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
	menu.canvas.Block(bounds)
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
	paintPopupSurface(canvas, &theme, bounds, level)
	menu.popOverlay()
	return menu
}

// beginScroll starts a paint of the list's scroll position: it forgets the rows of
// the last paint, scrolls to the row the keyboard moved to, and takes the offset.
func (menu *menuSurface) beginScroll() {
	if scroll := menu.options.scroll; scroll != nil {
		if menu.canvas != nil {
			scroll.follow(menu.canvas)
		}
		scroll.items = scroll.items[:0]
		scroll.view = menu.viewHeight
		if scroll.measured {
			scroll.Offset = max(min(scroll.Offset, scroll.max), 0)
		}
		menu.offset = max(scroll.Offset, 0)
		if scroll.max > 0 && !scroll.BarID.IsZero() {
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
		scroll.items = append(scroll.items, menuScrollItem{id: id, label: label, top: menu.used, height: row.Height, disabled: disabled})
	}
	return menu.take(row)
}

// take reserves a row and reports whether any of it shows.
func (menu *menuSurface) take(row gfx.Rect) bool {
	menu.y += row.Height
	menu.used += row.Height
	if scroll := menu.options.scroll; scroll != nil {
		scroll.max, scroll.measured = max(menu.used-menu.viewHeight, 0), true
	}
	return !menu.hidden && row.Height > 0 && row.Y < menu.bottom && row.Y+row.Height > menu.top
}

// paintRow runs paint for a row on the menu's layer and scope, clipped to the
// visible part of the surface when the row is cut by an edge.
func (menu *menuSurface) paintRow(row gfx.Rect, paint func()) {
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
	menu.paintRow(row, func() {
		menu.canvas.Label(label, gfx.Rect{X: row.X + left, Y: row.Y, Width: max(0, row.Width-left-8), Height: row.Height}, gfx.TextStyle{
			Size: size, Weight: weight, Color: color,
		})
	})
	return row
}

func (menu *menuSurface) addSeparator() gfx.Rect {
	row := gfx.Rect{X: menu.bounds.X, Y: menu.y, Width: menu.bounds.Width, Height: menuSeparatorHeight}
	if !menu.take(row) {
		return gfx.Rect{}
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

func (menu *menuSurface) paintItem(bounds gfx.Rect, item menuItem) {
	canvas, theme := menu.canvas, menu.theme
	enabled := !item.ID.IsZero() && !item.Disabled
	// The pointer moves the menu's current row the way Radix does: entering a
	// row focuses it, so the tint and Enter name one row instead of two. A
	// pointer that rests in place never moves the row back, so the arrow keys
	// keep working under it.
	hovered := canvas.Hovered()
	claim := canvas.HoverCrossed() && !hovered.IsZero()
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
	active := enabled && (claim && hovered == item.ID || !claim && canvas.Focused() == item.ID)
	if item.Kind == menuSubmenu && item.Expanded {
		active = true
	}
	foreground := theme.PopoverForeground
	if item.Destructive {
		foreground = theme.Danger
	}
	if active {
		fill := theme.Accent
		if item.Destructive {
			fill = theme.Danger
			fill.Alpha = 0.10
		} else {
			foreground = theme.AccentForeground
		}
		canvas.Rounded(bounds, theme.Metrics.RadiusItem(), fill)
	}
	if item.Disabled {
		foreground.Alpha *= 0.50
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
	if !item.ID.IsZero() {
		hit := bounds
		hit.Width -= menu.hitInset
		canvas.HitWithOptions(item.ID, hit, item.Label, gfx.HitOptions{
			Role: gfx.RoleMenuItem, Value: item.Value, Hint: item.Hint, Description: item.Description,
			Disabled: item.Disabled, Checked: item.Kind == menuCheckbox && item.Checked,
			Selected: item.Kind == menuRadio && item.Selected,
			Expanded: item.Kind == menuSubmenu && item.Expanded, Expandable: item.Kind == menuSubmenu,
		})
	}
}

// end paints a thin scrollbar thumb on the right edge when rows are scrolled out,
// and ends what the menu animates in and out.
func (menu *menuSurface) end() {
	if menu.hidden {
		return
	}
	if scroll := menu.options.scroll; scroll != nil && scroll.max > 0 {
		view := menu.viewHeight
		thumb := max(view*view/(view+scroll.max), 18)
		at := menu.top + (view-thumb)*scroll.Offset/scroll.max
		scroll.barTop, scroll.thumb = menu.top, thumb
		menu.pushOverlay()
		menu.canvas.Rounded(gfx.Rect{X: menu.bounds.X + menu.bounds.Width - 6, Y: at, Width: 4, Height: thumb}, 2, menu.theme.Border)
		if !scroll.BarID.IsZero() {
			menu.canvas.HitWithOptions(scroll.BarID, gfx.Rect{X: menu.bounds.X + menu.bounds.Width - 2 - menuBarWidth, Y: menu.top, Width: menuBarWidth, Height: view},
				"Scroll", gfx.HitOptions{Role: gfx.RoleSlider, SkipFocus: true})
		}
		menu.popOverlay()
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
