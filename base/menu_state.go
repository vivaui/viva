package base

import (
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

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
	BarID    gfx.ID
	MaxValue float32
	// measured says a paint has found max, so Offset can be held to it; before
	// that an Offset the caller set stands.
	Measured bool
	// view is the height of the room the rows show in; items are the rows with
	// an identity, as the last paint laid them out in content coordinates.
	View  float32
	Items []MenuScrollItem
	// TabCloses makes Tab close the list and continue through the page, as a
	// dropdown or select does. A context menu leaves it false, because Base UI
	// keeps Tab inside a context menu.
	TabCloses bool
	// followed is the keyboard focus the list last scrolled to.
	Followed gfx.ID
	// barTop and thumb place the thumb of the last paint; grab is where in the
	// thumb a drag holds it.
	BarTop, Thumb, Grab float32
	Held                bool
}

// menuScrollItem is a selectable row: what it says and where it lies in the
// content, so the keyboard can also match it by name.
type MenuScrollItem struct {
	ID          gfx.ID
	Label       string
	Top, Height float32
	Disabled    bool
}

// PointerRow remembers which option row the pointer is over so a list can move
// its keyboard cursor onto that row. It follows the pointer only when the
// pointer crosses onto a different row: a pointer that rests where it already
// was never takes the cursor back from the keyboard, so arrow keys keep
// working while the pointer is parked over the list.
type PointerRow struct{ Row gfx.ID }

// Max is how far the list can scroll: how much of its content the last paint
// did not fit. It is zero until the list has painted, and for a list that fits.
func (scroll *MenuScroll) Max() float32 { return scroll.MaxValue }

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
	scroll.Offset = max(min(before+step, scroll.MaxValue), 0)
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
	for index := range scroll.Items {
		if scroll.Items[index].ID == id {
			return scroll.revealAt(index)
		}
	}
	return false
}

func (scroll *MenuScroll) revealAt(index int) bool {
	item := scroll.Items[index]
	target := scroll.Offset
	switch {
	case index == 0:
		target = 0
	case item.Top < scroll.Offset:
		target = item.Top
	case item.Top+item.Height > scroll.Offset+scroll.View:
		target = item.Top + item.Height - scroll.View
		if index == len(scroll.Items)-1 {
			target = scroll.MaxValue
		}
	}
	before := scroll.Offset
	scroll.Offset = max(min(target, scroll.MaxValue), 0)
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
	for index, item := range scroll.Items {
		if item.ID == from {
			current = index
		}
		if item.Disabled {
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
		for index := from + step; index >= 0 && index < len(scroll.Items); index += step {
			if !scroll.Items[index].Disabled {
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
		reach := scroll.Items[current].Top + scroll.View
		target = current
		for index := next(current, 1); index >= 0 && scroll.Items[index].Top <= reach; index = next(index, 1) {
			target = index
		}
		if target == current {
			target = next(current, 1)
		}
		if target < 0 {
			target = last
		}
	case key == gfx.KeyPageUp:
		reach := scroll.Items[current].Top - scroll.View
		target = current
		for index := next(current, -1); index >= 0 && scroll.Items[index].Top >= reach; index = next(index, -1) {
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
	return scroll.Items[target].ID, true
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
	if prefix == "" || len(scroll.Items) == 0 {
		return gfx.ID{}, false
	}
	start := 0
	if !from.IsZero() {
		for index := range scroll.Items {
			if scroll.Items[index].ID == from {
				start = index + 1
				break
			}
		}
	}
	for offset := 0; offset < len(scroll.Items); offset++ {
		index := (start + offset) % len(scroll.Items)
		item := scroll.Items[index]
		if item.ID.IsZero() || item.Disabled || item.Label == "" {
			continue
		}
		if menuLabelHasPrefix(item.Label, prefix) {
			scroll.revealAt(index)
			return item.ID, true
		}
	}
	return gfx.ID{}, false
}

// CanCycle reports whether a repeated first letter may cycle through matches:
// no row's first two letters are the same, so a doubled letter is a real query.
func (scroll *MenuScroll) CanCycle() bool { return menuTypeaheadCanCycle(scroll) }

// Dragging reports whether the thumb is held, so pointer events that follow it
// out of the scrollbar still belong to Drag.
func (scroll *MenuScroll) Dragging() bool { return scroll.Held }

// Drag drags the thumb of a list whose BarID is set: feed it the pointer events
// whose Target is BarID, and, while Dragging, every pointer event. A press on the
// thumb holds it where it was grabbed; a press on the track elsewhere centers the
// thumb on the pointer. It reports whether the position changed, to repaint.
func (scroll *MenuScroll) Drag(event gfx.PointerEvent) bool {
	switch event.Kind {
	case gfx.PointerDown:
		if scroll.BarID.IsZero() || event.Target != scroll.BarID || scroll.MaxValue <= 0 {
			return false
		}
		at := scroll.BarTop + (scroll.View-scroll.Thumb)*scroll.Offset/scroll.MaxValue
		scroll.Grab = scroll.Thumb / 2
		if event.Y >= at && event.Y < at+scroll.Thumb {
			scroll.Grab = event.Y - at
		}
		scroll.Held = true
		return scroll.dragTo(event.Y)
	case gfx.PointerMove:
		return scroll.Held && scroll.dragTo(event.Y)
	default:
		scroll.Held = false
		return false
	}
}

func (scroll *MenuScroll) dragTo(y float32) bool {
	travel := scroll.View - scroll.Thumb
	if travel <= 0 {
		return false
	}
	before := scroll.Offset
	scroll.Offset = max(min((y-scroll.Grab-scroll.BarTop)/travel*scroll.MaxValue, scroll.MaxValue), 0)
	return scroll.Offset != before
}

// follow scrolls the item the keyboard has just moved to into view, before the
// list is laid out. A wheel gesture that leaves the focus where it was is not
// undone.
func (scroll *MenuScroll) Follow(canvas *gfx.Canvas) {
	focus := canvas.FocusVisible()
	if focus == scroll.Followed {
		return
	}
	scroll.Followed = focus
	scroll.Reveal(focus)
}

// entered records row as the row under the pointer and reports whether the
// pointer has just crossed onto it. Pass the zero ID, once per paint, to record
// that no option is under the pointer.
func (p *PointerRow) Entered(row gfx.ID) bool {
	if row == p.Row {
		return false
	}
	p.Row = row
	return !row.IsZero()
}

// HierarchyMenuItem is one path value of a HierarchyMenu.
type HierarchyMenuItem struct {
	Value    string
	Disabled bool
}

// HierarchyMenuAction is a root-level command, independent of the path values.
// Its nonzero ID must be outside OptionIDs. A click reports that ID directly;
// keyboard activation returns it in HierarchyMenuKeyResult.ActionID.
type HierarchyMenuAction struct {
	ID       gfx.ID
	Label    string
	Disabled bool
}

// hierarchyNode is one segment of a value path. Children are a singly linked
// list in first-seen order; node 0 is the invisible root.
type hierarchyNode struct {
	Name, Path        string
	First, Last, Next int32
	Count             int32
	Disabled          bool
}

// HierarchyMenuState owns the segment tree of path values and which fly-outs are
// open. Level l of the menu lists the children of trail[l-1] (the root for level
// 0); depth is the number of open fly-outs. A cascade shows levels 0..depth;
// compact navigation shows depth alone. It holds no open flag: the caller
// decides when the menu is shown.
type HierarchyMenuState struct {
	Nodes []hierarchyNode
	OK    bool
	Sep   string
	Equal func(a, b string) bool
	Value string
	Trail [hierarchyMaxLevels]int32
	// Active is the keyboard row of each level; Level is the panel the cursor
	// is in.
	Active     [hierarchyMaxLevels]int
	DepthValue int
	Level      int
	// Action is the root command with the cursor, or zero for a path row.
	Action    gfx.ID
	OptionIDs gfx.Scope
	Initial   bool
	Typeahead MenuTypeahead
	TypeKeys  bool
	Intent    HoverIntent
	Hover     PointerRow
	// Panels and Triggers are where the last paint put each open panel and the
	// row that opened it.
	Panels, Triggers [hierarchyMaxLevels]gfx.Rect
	Scrolls          [hierarchyMaxLevels]MenuScroll
	Reveal           bool
	Rebuilt          uint64
	Compact          bool
	// Reserve the whole tree's column widths before any fly-out opens. Moving
	// a hovered parent to fit its new child can put that child under the press
	// that was meant to select the parent.
	Widest struct {
		Rebuilt uint64
		Size    float32
		Widths  [hierarchyMaxLevels]float32
		Levels  int
	}
}

// HierarchyMenuKeyResult is what a key did to the menu.
type HierarchyMenuKeyResult struct {
	gfx.EventResult
	// Picked says Chosen is the value of the row Enter chose.
	Picked bool
	Chosen string
	// ActionID is a root command activated with Enter or Space. It never also
	// sets Picked: commands are not path values.
	ActionID gfx.ID
	// Opened says the key asks the caller to show the menu, as Down and Enter
	// do on a closed one.
	Opened bool
}

// HierarchyMenuOptions names the menu's hit identities and its overlay.
type HierarchyMenuOptions struct {
	// OptionIDs identifies rows: level<<16 plus row, which Choose takes as is;
	// the first level's rows are the plain row numbers. ScrollID is the first
	// level's scrollbar, and each fly-out's lies in OptionIDs.
	// Compact navigation buttons also use OptionIDs; pass their index to Choose
	// unchanged. They navigate without selecting a value.
	OptionIDs gfx.Scope
	ScrollID  gfx.ID
	// Leading and Trailing are commands before and after the root's path rows.
	// Trailing is separated from preceding rows. They scroll with the root and
	// are reached by returning to the root in a compact menu.
	Leading, Trailing []HierarchyMenuAction
	// Typeahead enables typing to reach rows by their labels. Leave it false
	// when an editable input owns the text, as in SingleSelect.
	Typeahead bool
	// Scope, Layer and Label name the popup's focus scope, layer and list.
	Scope gfx.ID
	Layer uint16
	Label string
	// EmptyLabel is shown when there is nothing to list.
	EmptyLabel string
	// Viewport bounds the popup independently of the anchor's ancestor clips.
	Viewport gfx.Rect
	MaxRows  int
	// Overlay makes the menu its own modal overlay: it takes the layer and
	// focus scope, and a press outside it, or Escape, reports DismissID.
	// Leave it false when the caller is already inside an overlay, as the
	// SingleSelect popup is.
	Overlay   bool
	DismissID gfx.ID
}

// Build fills the tree from n values at separator and seeks value, so the menu
// opens where it is selected. equal compares segments and values (nil is exact).
// It reports false, and the menu stays unusable, when a value is nested deeper
// than the menu supports.
func (m *HierarchyMenuState) Build(separator, value string, equal func(a, b string) bool, n int, item func(i int) HierarchyMenuItem) bool {
	if equal == nil {
		equal = func(a, b string) bool { return a == b }
	}
	m.Sep, m.Value, m.Equal = separator, value, equal
	m.Rebuilt++
	m.Nodes = append(m.Nodes[:0], hierarchyNode{})
	m.OK = separator != ""
	for i := 0; i < n && m.OK; i++ {
		m.add(item(i))
	}
	for i := range m.Scrolls {
		m.Scrolls[i] = MenuScroll{Items: m.Scrolls[i].Items[:0]}
	}
	m.Hover = PointerRow{}
	m.Seek(value)
	return m.OK
}

// Usable reports whether Build succeeded.
func (m *HierarchyMenuState) Usable() bool { return m.OK }

func (m *HierarchyMenuState) add(item HierarchyMenuItem) {
	parent, rest, offset := int32(0), item.Value, 0
	for depth := 1; ; depth++ {
		if depth > hierarchyMaxLevels {
			m.OK = false
			return
		}
		end := strings.Index(rest, m.Sep)
		segment := rest
		if end >= 0 {
			segment = rest[:end]
		}
		name := strings.TrimSpace(segment)
		child := m.child(parent, name)
		if child == 0 {
			child = int32(len(m.Nodes))
			m.Nodes = append(m.Nodes, hierarchyNode{Name: name, Path: item.Value[:offset+len(segment)]})
			if p := &m.Nodes[parent]; p.Count == 0 {
				p.First = child
			} else {
				m.Nodes[p.Last].Next = child
			}
			m.Nodes[parent].Last = child
			m.Nodes[parent].Count++
		}
		parent = child
		if end < 0 {
			break
		}
		rest = rest[end+len(m.Sep):]
		offset += len(segment) + len(m.Sep)
	}
	if parent != 0 {
		m.Nodes[parent].Disabled = item.Disabled
	}
}

func (m *HierarchyMenuState) child(parent int32, name string) int32 {
	for n := m.Nodes[parent].First; n != 0; n = m.Nodes[n].Next {
		if m.Equal(m.Nodes[n].Name, name) {
			return n
		}
	}
	return 0
}

// Seek opens the fly-outs down to value, with the cursor on it. An empty or
// unknown value closes them all.
func (m *HierarchyMenuState) Seek(value string) {
	m.Value = value
	m.DepthValue, m.Level, m.Active = 0, 0, [hierarchyMaxLevels]int{}
	m.Action, m.Initial = gfx.ID{}, true
	m.Typeahead.Reset()
	m.Intent.Reset()
	if !m.OK || value == "" {
		return
	}
	parent, rest := int32(0), value
	for level := 0; ; level++ {
		end := strings.Index(rest, m.Sep)
		segment := rest
		if end >= 0 {
			segment = rest[:end]
		}
		row, n := 0, m.Nodes[parent].First
		for ; n != 0; n, row = m.Nodes[n].Next, row+1 {
			if m.Equal(m.Nodes[n].Name, strings.TrimSpace(segment)) {
				break
			}
		}
		if n == 0 {
			return
		}
		m.Active[level], m.Level = row, level
		if end < 0 {
			return
		}
		m.Trail[level], m.DepthValue = n, level+1
		parent, rest = n, rest[end+len(m.Sep):]
	}
}

// Reset closes every fly-out and puts the cursor on the first level.
func (m *HierarchyMenuState) Reset() {
	m.DepthValue, m.Level = 0, 0
	m.Action, m.Initial = gfx.ID{}, true
	m.Typeahead.Reset()
	m.Intent.Reset()
}

// Depth is the number of open fly-outs; Escape closes the deepest.
func (m *HierarchyMenuState) Depth() int { return m.DepthValue }

func (m *HierarchyMenuState) ParentOf(level int) int32 {
	if level == 0 {
		return 0
	}
	return m.Trail[level-1]
}

// rowNode is the node of row of level, or 0 when there is none.
func (m *HierarchyMenuState) RowNode(level, row int) int32 {
	if level < 0 || level > m.DepthValue || row < 0 {
		return 0
	}
	parent := m.ParentOf(level)
	if row >= int(m.Nodes[parent].Count) {
		return 0
	}
	n := m.Nodes[parent].First
	for ; row > 0; row-- {
		n = m.Nodes[n].Next
	}
	return n
}

// Choose returns the value of the row an identity number (level<<16 | row)
// names; false for a scrollbar, a missing row or a disabled one. Compact menus
// also send their navigation buttons here: they change the displayed level
// and return false, leaving the caller's selection and open state unchanged.
func (m *HierarchyMenuState) Choose(index int) (string, bool) {
	if !m.OK {
		return "", false
	}
	if m.Compact {
		switch {
		case index == hierarchyBackIndex && m.DepthValue > 0:
			m.DepthValue--
			m.Level, m.Reveal = m.DepthValue, true
			m.Typeahead.Reset()
			m.Intent.Reset()
			return "", false
		case index>>16 >= hierarchyOpenLevel && index>>16 < 2*hierarchyOpenLevel:
			level, row := (index>>16)-hierarchyOpenLevel, index&0xFFFF
			if level == m.DepthValue {
				if n := m.RowNode(level, row); n != 0 && !m.Nodes[n].Disabled && m.Nodes[n].Count > 0 && level+1 < hierarchyMaxLevels {
					m.Active[level] = row
					m.Trail[level], m.DepthValue = n, level+1
					m.Level, m.Active[m.DepthValue], m.Reveal = m.DepthValue, 0, true
					m.Scrolls[m.DepthValue].Offset = 0
					m.Scrolls[m.DepthValue].Items = m.Scrolls[m.DepthValue].Items[:0]
					m.Action = gfx.ID{}
					m.Typeahead.Reset()
					m.Intent.Reset()
				}
			}
			return "", false
		}
	}
	n := m.RowNode(index>>16, index&0xFFFF)
	if n == 0 || m.Nodes[n].Disabled {
		return "", false
	}
	return m.Nodes[n].Path, true
}

func (m *HierarchyMenuState) CurrentID() gfx.ID {
	if m.Level == 0 && !m.Action.IsZero() {
		return m.Action
	}
	return m.OptionIDs.At(m.Level<<16 | m.Active[m.Level])
}

func (m *HierarchyMenuState) ActionEnabled(id gfx.ID) bool {
	if id.IsZero() {
		return false
	}
	if _, row := id.In(m.OptionIDs); row {
		return false
	}
	for _, item := range m.Scrolls[0].Items {
		if item.ID == id {
			return !item.Disabled
		}
	}
	return false
}

func (m *HierarchyMenuState) cursorAt(id gfx.ID) {
	if index, ok := id.In(m.OptionIDs); ok {
		m.Level, m.Active[index>>16], m.Action = index>>16, index&0xFFFF, gfx.ID{}
	} else if m.ActionEnabled(id) {
		m.Level, m.Action = 0, id
	}
	m.Reveal = true
}

// moveUnpainted keeps keyboard-only callers usable before the first paint of a
// level. Once painted, MenuScroll owns the complete sequence, including commands.
func (m *HierarchyMenuState) moveUnpainted(key gfx.Key) {
	count := int(m.Nodes[m.ParentOf(m.Level)].Count)
	step, next := 1, m.Active[m.Level]
	switch key {
	case gfx.KeyUp, gfx.KeyPageUp:
		step = -1
	case gfx.KeyHome:
		next = -1
	case gfx.KeyEnd:
		step, next = -1, count
	}
	for i := 0; i < count; i++ {
		next = (next + step + count) % count
		if n := m.RowNode(m.Level, next); n != 0 && !m.Nodes[n].Disabled {
			m.Active[m.Level] = next
			break
		}
	}
	m.Reveal = true
}

// Key moves within the current level with Up/Down, Home/End and PageUp/PageDown,
// Right into a fly-out, and Left out of it. Enter chooses; Escape closes the
// deepest fly-out. Typeahead and Space activation are enabled by the paint's
// Typeahead option, so editable controls retain their text input. open says
// whether the menu is shown; Down, Up and Enter on a hidden one ask to show it.
// Releases, modified navigation and keys it does not handle are left unhandled.
func (m *HierarchyMenuState) Key(event gfx.KeyEvent, open bool) HierarchyMenuKeyResult {
	if !m.OK || event.Release || event.Modifiers&(gfx.ModifierControl|gfx.ModifierAlt|gfx.ModifierMeta) != 0 {
		return HierarchyMenuKeyResult{}
	}
	// Keyboard navigation owns the next paint, even while the pointer remains
	// parked on a row that used to open a deeper panel.
	m.Intent.Reset()
	handled := HierarchyMenuKeyResult{EventResult: gfx.EventResult{Handled: true, Repaint: true}}
	if open && m.TypeKeys && (event.Text != "" || event.Key == gfx.KeySpace) {
		text := event.Text
		if text == "" && event.Key == gfx.KeySpace {
			text = " "
		}
		now := time.Now()
		continues := text == " " && m.Typeahead.Typing(now)
		if id, ok := m.Typeahead.Type(text, now, &m.Scrolls[m.Level], m.CurrentID()); ok {
			m.cursorAt(id)
			return handled
		}
		if continues {
			return handled
		}
	}
	if event.Modifiers != 0 {
		return HierarchyMenuKeyResult{}
	}
	switch event.Key {
	case gfx.KeyDown, gfx.KeyUp, gfx.KeyHome, gfx.KeyEnd, gfx.KeyPageUp, gfx.KeyPageDown:
		if !open {
			if event.Key != gfx.KeyDown && event.Key != gfx.KeyUp {
				return HierarchyMenuKeyResult{}
			}
			handled.Opened = true
			return handled
		}
		m.Typeahead.Reset()
		if id, ok := m.Scrolls[m.Level].Navigate(event.Key, m.CurrentID(), true); ok {
			m.cursorAt(id)
		} else if len(m.Scrolls[m.Level].Items) == 0 {
			m.moveUnpainted(event.Key)
		}
		return handled
	case gfx.KeyRight:
		if !open {
			return HierarchyMenuKeyResult{}
		}
		m.Typeahead.Reset()
		if m.Level == 0 && !m.Action.IsZero() {
			return handled
		}
		if n := m.RowNode(m.Level, m.Active[m.Level]); n != 0 && m.Nodes[n].Count > 0 && m.Level+1 < hierarchyMaxLevels {
			m.Trail[m.Level], m.DepthValue = n, m.Level+1
			m.Level++
			m.Active[m.Level] = 0
			m.Scrolls[m.Level].Offset = 0
			m.Scrolls[m.Level].Items = m.Scrolls[m.Level].Items[:0]
			m.Reveal = true
		}
		return handled
	case gfx.KeyLeft:
		if !open || m.Level == 0 {
			return HierarchyMenuKeyResult{}
		}
		m.Level--
		m.DepthValue = m.Level
		m.Typeahead.Reset()
		m.Reveal = true
		return handled
	case gfx.KeyEnter, gfx.KeySpace:
		if event.Key == gfx.KeySpace && (!open || !m.TypeKeys) {
			return HierarchyMenuKeyResult{}
		}
		if event.Repeat {
			return handled
		}
		if !open {
			handled.Opened = true
		} else if m.Level == 0 && !m.Action.IsZero() {
			if m.ActionEnabled(m.Action) {
				handled.ActionID = m.Action
			}
		} else if value, ok := m.Choose(m.Level<<16 | m.Active[m.Level]); ok {
			handled.Picked, handled.Chosen = true, value
		}
		return handled
	case gfx.KeyEscape:
		if open && m.DepthValue > 0 {
			m.DepthValue--
			m.Level = min(m.Level, m.DepthValue)
			m.Typeahead.Reset()
			return handled
		}
	}
	return HierarchyMenuKeyResult{}
}

// Wheel scrolls the panel under the pointer: the deepest open one that holds it.
func (m *HierarchyMenuState) Wheel(event gfx.ScrollEvent) bool {
	for level := m.DepthValue; level >= 0; level-- {
		if pointInRect(event.X, event.Y, m.Panels[level]) {
			return m.Scrolls[level].Wheel(event)
		}
	}
	return false
}

// Pointer drags the scrollbar of whichever panel the event belongs to.
func (m *HierarchyMenuState) Pointer(event gfx.PointerEvent) gfx.EventResult {
	for level := 0; level <= m.DepthValue; level++ {
		scroll := &m.Scrolls[level]
		if !scroll.BarID.IsZero() && (event.Target == scroll.BarID || scroll.Dragging()) {
			return gfx.EventResult{Handled: true, Repaint: scroll.Drag(event)}
		}
	}
	return gfx.EventResult{}
}

// SingleSelectOption is a stable selectable value; an empty Label displays Value.
type SingleSelectOption struct {
	Value, Label string
	Disabled     bool
	// Icon, when it has any geometry, is painted before the label in the flat
	// list, and before the field's text while it is that option's value. A
	// single-color icon takes the text color; a multi-color one keeps its own.
	// The hierarchical list (Separator) paints names only.
	Icon gfx.Icon
	// Search is extra text the query matches besides the label and the value,
	// for what the row does not show, such as an address or a group. It is never
	// painted.
	Search string
	// Detail is secondary text shown after the label in a muted, smaller face, such as a
	// host's address, in the list and while the option is the field's value. The query
	// matches it too, as it matches Search.
	Detail string
	// Leading and Trailing reserve room before the label and at the end of the row, which
	// their Paint fills: a flag, a status dot, a badge, a shortcut. Content, when it has a
	// Paint, paints the row's text itself in place of the label and detail, in the room
	// between them. Each is drawn over its slot and takes no input: a press on it is the
	// option's. The field shows the chosen option's label and detail only.
	Leading, Trailing, Content SingleSelectPaint
}

// SingleSelectPaint is room an option reserves in its row, Width pixels wide, and what
// paints it. A zero Width with a Paint takes what is left of the row (Content only).
type SingleSelectPaint struct {
	Width float32
	Paint func(canvas *gfx.Canvas, bounds gfx.Rect, theme gfx.Theme)
}

// SingleSelectChoice is one filtered existing or creatable option.
type SingleSelectChoice struct {
	SingleSelectOption
	Selected, Create bool
}

// SingleSelectState owns one selection and a separate native search editor.
// Selecting replaces the previous value and closes the list. Bind the native
// text input to Query, and route option clicks and input keys to Choose and Key.
type SingleSelectState struct {
	Query                             text.Editor
	Open                              bool
	Active                            int
	AllowCreate, MatchCaseInsensitive bool
	// LabelsOnly makes the query match the options' labels (and Search), not their
	// values: for values that are keys of the caller's, which the user never sees.
	LabelsOnly bool
	// Separator, when set, makes the popup hierarchical: values are paths whose
	// segments are split at Separator, the list shows the first segment and a
	// row with children opens a fly-out of the next. Choosing any row, a parent
	// included, selects the path up to it. Typing falls back to a flat list of
	// full paths. Rows show their own segment, not the option's Label.
	Separator       string
	ValueText       string
	Options, Custom []SingleSelectOption
	ChoicesCache    []SingleSelectChoice
	ValidateCreate  func(string) bool
	Revision, Built uint64
	// rebuilt counts list rebuilds; widest caches the widest existing label
	// for the list and font size it was measured at.
	Rebuilt, WidestRebuilt          uint64
	WidestSize, WidestValue         float32
	LastQuery, LastSep              string
	Tree                            HierarchyMenuState
	TreeOK                          bool
	LastAllow, LastCase, Prepared   bool
	Scroll                          MenuScroll
	OpenPainted, FocusInput, Reveal bool
	// hover is the row the pointer last crossed onto: hovering that row moves
	// Active there, so the highlight and Enter name one row instead of two.
	Hover                                 PointerRow
	CreatePrefix, CreateValue, CreateText string
}

// SingleSelectOptions names the control's hit IDs, its popup layer and the
// strings the caller supplies for its labels and create row.
type SingleSelectOptions struct {
	InputID, TriggerID, ClearID, DismissID, Scope, ScrollID  gfx.ID
	OptionIDs                                                gfx.Scope
	Label, Placeholder, EmptyLabel, CreatePrefix, ClearLabel string
	// Viewport bounds the popup and backdrop independently of the input's
	// ancestor scroll clips. Set it to the window or a narrower overlay region.
	Viewport          gfx.Rect
	Layer             uint16
	Disabled, Invalid bool
	MaxRows           int
}

// SingleSelectLayout is where the control, its native input and its popup were
// painted, for callers that route text input or place overlays.
type SingleSelectLayout struct{ Bounds, Input, Popup gfx.Rect }

func (s *SingleSelectState) SetOptions(options []SingleSelectOption) {
	s.Options = append(s.Options[:0], options...)
	s.Revision++
}

func (s *SingleSelectState) Value() string { return s.ValueText }

func (s *SingleSelectState) SetValue(value string) {
	s.ValueText = value
	s.Query = text.Editor{}
	s.Revision++
}

// ForgetCreated drops the values the user created with AllowCreate, for a caller
// that keeps them among the options it sets itself: the list would show each twice.
func (s *SingleSelectState) ForgetCreated() {
	if len(s.Custom) == 0 {
		return
	}
	clear(s.Custom)
	s.Custom = s.Custom[:0]
	s.Revision++
}

func (s *SingleSelectState) SetValidateCreate(validate func(string) bool) {
	s.ValidateCreate = validate
	s.Revision++
}

// Dismiss closes the list and drops the search text without changing the
// value. Call it when focus or a press leaves the field.
func (s *SingleSelectState) Dismiss() {
	if !s.Open && s.Query.String() == "" {
		return
	}
	s.Query = text.Editor{}
	s.Open = false
	s.Active = 0
	s.Tree.Reset()
}

func (s *SingleSelectState) equal(a, b string) bool {
	return a == b || s.MatchCaseInsensitive && strings.EqualFold(a, b)
}

func (s *SingleSelectState) Label(value string) string {
	for _, option := range s.Options {
		if s.equal(option.Value, value) && option.Label != "" {
			return option.Label
		}
	}
	return value
}

// detail is the detail of the option with value, or "".
func (s *SingleSelectState) Detail(value string) string {
	for _, option := range s.Options {
		if s.equal(option.Value, value) {
			return option.Detail
		}
	}
	return ""
}

// selectedIcon is the icon of the chosen option, when it has one.
func (s *SingleSelectState) SelectedIcon() gfx.Icon {
	for _, option := range s.Options {
		if s.equal(option.Value, s.ValueText) {
			return option.Icon
		}
	}
	return gfx.Icon{}
}

// Choices returns a read-only view until the next query or selection change.
// It caches filtered rows, preserving selected and newly created values
// even when the caller's options do not contain them. Exact labels and disabled
// values cannot be bypassed through a duplicate create row.
func (s *SingleSelectState) Choices() []SingleSelectChoice {
	query := strings.TrimSpace(s.Query.String())
	if s.Prepared && s.Built == s.Revision && query == s.LastQuery && s.LastAllow == s.AllowCreate && s.LastCase == s.MatchCaseInsensitive && s.LastSep == s.Separator {
		return s.ChoicesCache
	}
	if query != s.LastQuery && query != "" {
		s.Open = true
	}
	s.ChoicesCache = s.ChoicesCache[:0]
	exact, haveValue := false, s.ValueText == ""
	match := strings.ToLower(query)
	add := func(option SingleSelectOption) {
		label := option.Label
		if label == "" {
			label = option.Value
		}
		exact = exact || s.equal(query, option.Value) || s.equal(query, label)
		haveValue = haveValue || s.equal(s.ValueText, option.Value)
		if query != "" && !strings.Contains(strings.ToLower(label), match) &&
			(s.LabelsOnly || !strings.Contains(strings.ToLower(option.Value), match)) &&
			!strings.Contains(strings.ToLower(option.Search), match) &&
			!strings.Contains(strings.ToLower(option.Detail), match) {
			return
		}
		option.Label = label
		s.ChoicesCache = append(s.ChoicesCache, SingleSelectChoice{SingleSelectOption: option, Selected: s.equal(s.ValueText, option.Value)})
	}
	for _, option := range s.Options {
		add(option)
	}
	for _, option := range s.Custom {
		if !slices.ContainsFunc(s.Options, func(o SingleSelectOption) bool { return s.equal(o.Value, option.Value) }) {
			add(option)
		}
	}
	if !haveValue {
		add(SingleSelectOption{Value: s.ValueText, Label: s.Label(s.ValueText)})
	}
	if s.AllowCreate && query != "" && !exact {
		s.ChoicesCache = append(s.ChoicesCache, SingleSelectChoice{})
		copy(s.ChoicesCache[1:], s.ChoicesCache[:len(s.ChoicesCache)-1])
		s.ChoicesCache[0] = SingleSelectChoice{SingleSelectOption: SingleSelectOption{Value: query, Label: query, Disabled: s.ValidateCreate != nil && !s.ValidateCreate(query)}, Create: true}
	}
	s.Active = 0
	for i, choice := range s.ChoicesCache {
		if !choice.Disabled && (s.ChoicesCache[s.Active].Disabled || query == "" && choice.Selected) {
			s.Active = i
		}
	}
	s.LastQuery, s.Built, s.Prepared = query, s.Revision, true
	s.Rebuilt++
	s.LastAllow, s.LastCase, s.LastSep = s.AllowCreate, s.MatchCaseInsensitive, s.Separator
	s.TreeOK = false
	if s.Separator != "" && query == "" {
		s.TreeOK = s.Tree.Build(s.Separator, s.ValueText, s.equal, len(s.ChoicesCache), func(i int) HierarchyMenuItem {
			return HierarchyMenuItem{Value: s.ChoicesCache[i].Value, Disabled: s.ChoicesCache[i].Disabled}
		})
	}
	s.Scroll.Offset, s.Reveal = 0, true
	// A rebuilt list is a new set of rows: let the pointer adopt the row it is
	// over again, or a pointer parked on a row would lose it to the reset above.
	s.Hover = PointerRow{}
	return s.ChoicesCache
}

// Choose always selects a single value, including when the row was selected.
//
// In a hierarchical list index is the number of the row's identity: its level
// shifted left by 16 plus its row, which is the plain row number in the first
// level and in a flat list.
func (s *SingleSelectState) Choose(index int) bool {
	choices := s.Choices()
	if s.TreeActive() {
		value, ok := s.Tree.Choose(index)
		if ok {
			s.SetValue(value)
			s.Open, s.FocusInput = false, true
		}
		return ok
	}
	if index < 0 || index >= len(choices) || choices[index].Disabled {
		return false
	}
	choice := choices[index]
	if choice.Create {
		s.Custom = append(s.Custom, choice.SingleSelectOption)
	}
	s.SetValue(choice.Value)
	s.Open, s.FocusInput = false, true
	return true
}

func (s *SingleSelectState) Clear() {
	s.SetValue("")
	s.Open, s.FocusInput = false, true
}

// CommitQuery confirms an exact existing value or a valid new value. A form can
// call it on submit to preserve ordinary text-field save behavior.
func (s *SingleSelectState) CommitQuery() bool {
	if _, composing := s.Query.Composition(); composing {
		return false
	}
	query := strings.TrimSpace(s.Query.String())
	if query == "" {
		return true
	}
	choices := s.Choices()
	for i, choice := range choices {
		if !choice.Create && (s.equal(query, choice.Value) || s.equal(query, choice.Label)) {
			return s.Choose(i)
		}
	}
	for i, choice := range choices {
		if choice.Create {
			return s.Choose(i)
		}
	}
	return false
}

func (s *SingleSelectState) Key(event gfx.KeyEvent) gfx.EventResult {
	if event.Release || event.Modifiers != 0 && !(event.Key == gfx.KeyTab && event.Modifiers == gfx.ModifierShift) {
		return gfx.EventResult{}
	}
	if _, composing := s.Query.Composition(); composing {
		return gfx.EventResult{Handled: event.Key == gfx.KeyEnter}
	}
	choices := s.Choices()
	if s.TreeActive() {
		result := s.Tree.Key(event, s.Open)
		if result.Opened {
			s.Open = true
		}
		if result.Picked {
			s.SetValue(result.Chosen)
			s.Open, s.FocusInput = false, true
		}
		if result.Handled {
			return result.EventResult
		}
	}
	switch event.Key {
	case gfx.KeyDown, gfx.KeyUp:
		if !s.Open {
			s.Open = true
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		step := 1
		if event.Key == gfx.KeyUp {
			step = -1
		}
		for i, next := 0, s.Active; i < len(choices); i++ {
			next = (next + step + len(choices)) % len(choices)
			if !choices[next].Disabled {
				s.Active = next
				break
			}
		}
		s.Reveal = true
		return gfx.EventResult{Handled: true, Repaint: true}
	case gfx.KeyEnter:
		if !event.Repeat {
			if !s.Open && s.Query.String() == "" {
				s.Open = true
			} else {
				s.Choose(s.Active)
			}
		}
		return gfx.EventResult{Handled: true, Repaint: true}
	case gfx.KeyEscape:
		if s.Open || s.Query.String() != "" {
			s.Dismiss()
			return gfx.EventResult{Handled: true, Repaint: true}
		}
	case gfx.KeyTab:
		if s.Open {
			// Leaving the field drops the search text: a closed field paints the
			// selected label, never the query it did not commit.
			s.Dismiss()
			return gfx.EventResult{Handled: true, Repaint: true, ContinueTraversal: true}
		}
	case gfx.KeyBackspace:
		if !event.Repeat && s.Query.String() == "" && s.ValueText != "" {
			s.Clear()
			return gfx.EventResult{Handled: true, Repaint: true}
		}
	}
	return gfx.EventResult{}
}

// Wheel scrolls the list, or in a hierarchical one the panel under the pointer.
func (s *SingleSelectState) Wheel(event gfx.ScrollEvent) bool {
	if s.TreeActive() {
		return s.Tree.Wheel(event)
	}
	return s.Scroll.Wheel(event)
}

func (s *SingleSelectState) Pointer(event gfx.PointerEvent) gfx.EventResult {
	if s.TreeActive() {
		return s.Tree.Pointer(event)
	}
	if s.Scroll.BarID.IsZero() || event.Target != s.Scroll.BarID && !s.Scroll.Dragging() {
		return gfx.EventResult{}
	}
	return gfx.EventResult{Handled: true, Repaint: s.Scroll.Drag(event)}
}

// treeActive reports whether the popup is the hierarchical one: a separator is
// set, no search text is typed, and every value fits the supported depth.
func (s *SingleSelectState) TreeActive() bool {
	s.Choices()
	return s.TreeOK && s.Separator != "" && s.LastQuery == ""
}

// The menu's fixed sizes, with the meaning they have in components.

const hierarchyMaxLevels = 8

const hierarchySwitchDelay = 60 * time.Millisecond

const hierarchyBarRow = 0xFFFF

const (
	hierarchyOpenLevel = hierarchyMaxLevels
	hierarchyBackIndex = 2 * hierarchyMaxLevels << 16
	hierarchyMinColumn = 96
)

// menuTypeaheadCanCycle reports whether a repeated first letter may cycle
// through matches.
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
