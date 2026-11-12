package components

import (
	"math"
	"slices"
	"strings"
	"time"

	"github.com/vivaui/viva/gfx"
)

// hierarchyMaxLevels is how many panels (the list plus its fly-outs) a
// HierarchyMenu can show at once. Values nested deeper make Build fail, so no
// value becomes unreachable behind a limit.
const hierarchyMaxLevels = 8

// hierarchySwitchDelay is how long the pointer rests on another row before an
// open fly-out gives way to that row's, or closes. A fly-out opens at once when
// none is open. The rest keeps the pointer's diagonal across the rows beside the
// trigger on its way into the fly-out from switching it; the canvas has no
// pointer position to aim with instead.
const hierarchySwitchDelay = 60 * time.Millisecond

// hierarchyBarRow is the reserved row number, within a level, of that level's
// scrollbar hit.
const hierarchyBarRow = 0xFFFF

// A compact menu keeps the current level readable instead of squeezing all
// its ancestors into blank columns. Its navigation buttons use reserved levels
// above the real tree, and still travel through the caller's Choose route.
const (
	hierarchyOpenLevel = hierarchyMaxLevels
	hierarchyBackIndex = 2 * hierarchyMaxLevels << 16
	hierarchyMinColumn = 96
)

var hierarchyLeftIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{gfx.MoveTo(15, 18), gfx.LineTo(9, 12), gfx.LineTo(15, 6)}}

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
	name, path        string
	first, last, next int32
	count             int32
	disabled          bool
}

// HierarchyMenuState owns the segment tree of path values and which fly-outs are
// open. Level l of the menu lists the children of trail[l-1] (the root for level
// 0); depth is the number of open fly-outs. A cascade shows levels 0..depth;
// compact navigation shows depth alone. It holds no open flag: the caller
// decides when the menu is shown.
type HierarchyMenuState struct {
	nodes []hierarchyNode
	ok    bool
	sep   string
	equal func(a, b string) bool
	value string
	trail [hierarchyMaxLevels]int32
	// active is the keyboard row of each level; level is the panel the cursor
	// is in.
	active [hierarchyMaxLevels]int
	depth  int
	level  int
	// action is the root command with the cursor, or zero for a path row.
	action    gfx.ID
	optionIDs gfx.Scope
	initial   bool
	typeahead MenuTypeahead
	typeKeys  bool
	intent    HoverIntent
	hover     pointerRow
	// panels and triggers are where the last paint put each open panel and the
	// row that opened it.
	panels, triggers [hierarchyMaxLevels]gfx.Rect
	scrolls          [hierarchyMaxLevels]MenuScroll
	reveal           bool
	rebuilt          uint64
	compact          bool
	// Reserve the whole tree's column widths before any fly-out opens. Moving
	// a hovered parent to fit its new child can put that child under the press
	// that was meant to select the parent.
	widest struct {
		rebuilt uint64
		size    float32
		widths  [hierarchyMaxLevels]float32
		levels  int
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
	m.sep, m.value, m.equal = separator, value, equal
	m.rebuilt++
	m.nodes = append(m.nodes[:0], hierarchyNode{})
	m.ok = separator != ""
	for i := 0; i < n && m.ok; i++ {
		m.add(item(i))
	}
	for i := range m.scrolls {
		m.scrolls[i] = MenuScroll{items: m.scrolls[i].items[:0]}
	}
	m.hover = pointerRow{}
	m.Seek(value)
	return m.ok
}

// Usable reports whether Build succeeded.
func (m *HierarchyMenuState) Usable() bool { return m.ok }

func (m *HierarchyMenuState) add(item HierarchyMenuItem) {
	parent, rest, offset := int32(0), item.Value, 0
	for depth := 1; ; depth++ {
		if depth > hierarchyMaxLevels {
			m.ok = false
			return
		}
		end := strings.Index(rest, m.sep)
		segment := rest
		if end >= 0 {
			segment = rest[:end]
		}
		name := strings.TrimSpace(segment)
		child := m.child(parent, name)
		if child == 0 {
			child = int32(len(m.nodes))
			m.nodes = append(m.nodes, hierarchyNode{name: name, path: item.Value[:offset+len(segment)]})
			if p := &m.nodes[parent]; p.count == 0 {
				p.first = child
			} else {
				m.nodes[p.last].next = child
			}
			m.nodes[parent].last = child
			m.nodes[parent].count++
		}
		parent = child
		if end < 0 {
			break
		}
		rest = rest[end+len(m.sep):]
		offset += len(segment) + len(m.sep)
	}
	if parent != 0 {
		m.nodes[parent].disabled = item.Disabled
	}
}

func (m *HierarchyMenuState) child(parent int32, name string) int32 {
	for n := m.nodes[parent].first; n != 0; n = m.nodes[n].next {
		if m.equal(m.nodes[n].name, name) {
			return n
		}
	}
	return 0
}

// Seek opens the fly-outs down to value, with the cursor on it. An empty or
// unknown value closes them all.
func (m *HierarchyMenuState) Seek(value string) {
	m.value = value
	m.depth, m.level, m.active = 0, 0, [hierarchyMaxLevels]int{}
	m.action, m.initial = gfx.ID{}, true
	m.typeahead.Reset()
	m.intent.Reset()
	if !m.ok || value == "" {
		return
	}
	parent, rest := int32(0), value
	for level := 0; ; level++ {
		end := strings.Index(rest, m.sep)
		segment := rest
		if end >= 0 {
			segment = rest[:end]
		}
		row, n := 0, m.nodes[parent].first
		for ; n != 0; n, row = m.nodes[n].next, row+1 {
			if m.equal(m.nodes[n].name, strings.TrimSpace(segment)) {
				break
			}
		}
		if n == 0 {
			return
		}
		m.active[level], m.level = row, level
		if end < 0 {
			return
		}
		m.trail[level], m.depth = n, level+1
		parent, rest = n, rest[end+len(m.sep):]
	}
}

// Reset closes every fly-out and puts the cursor on the first level.
func (m *HierarchyMenuState) Reset() {
	m.depth, m.level = 0, 0
	m.action, m.initial = gfx.ID{}, true
	m.typeahead.Reset()
	m.intent.Reset()
}

// Depth is the number of open fly-outs; Escape closes the deepest.
func (m *HierarchyMenuState) Depth() int { return m.depth }

func (m *HierarchyMenuState) parentOf(level int) int32 {
	if level == 0 {
		return 0
	}
	return m.trail[level-1]
}

// rowNode is the node of row of level, or 0 when there is none.
func (m *HierarchyMenuState) rowNode(level, row int) int32 {
	if level < 0 || level > m.depth || row < 0 {
		return 0
	}
	parent := m.parentOf(level)
	if row >= int(m.nodes[parent].count) {
		return 0
	}
	n := m.nodes[parent].first
	for ; row > 0; row-- {
		n = m.nodes[n].next
	}
	return n
}

// Choose returns the value of the row an identity number (level<<16 | row)
// names; false for a scrollbar, a missing row or a disabled one. Compact menus
// also send their navigation buttons here: they change the displayed level
// and return false, leaving the caller's selection and open state unchanged.
func (m *HierarchyMenuState) Choose(index int) (string, bool) {
	if !m.ok {
		return "", false
	}
	if m.compact {
		switch {
		case index == hierarchyBackIndex && m.depth > 0:
			m.depth--
			m.level, m.reveal = m.depth, true
			m.typeahead.Reset()
			m.intent.Reset()
			return "", false
		case index>>16 >= hierarchyOpenLevel && index>>16 < 2*hierarchyOpenLevel:
			level, row := (index>>16)-hierarchyOpenLevel, index&0xFFFF
			if level == m.depth {
				if n := m.rowNode(level, row); n != 0 && !m.nodes[n].disabled && m.nodes[n].count > 0 && level+1 < hierarchyMaxLevels {
					m.active[level] = row
					m.trail[level], m.depth = n, level+1
					m.level, m.active[m.depth], m.reveal = m.depth, 0, true
					m.scrolls[m.depth].Offset = 0
					m.scrolls[m.depth].items = m.scrolls[m.depth].items[:0]
					m.action = gfx.ID{}
					m.typeahead.Reset()
					m.intent.Reset()
				}
			}
			return "", false
		}
	}
	n := m.rowNode(index>>16, index&0xFFFF)
	if n == 0 || m.nodes[n].disabled {
		return "", false
	}
	return m.nodes[n].path, true
}

func (m *HierarchyMenuState) currentID() gfx.ID {
	if m.level == 0 && !m.action.IsZero() {
		return m.action
	}
	return m.optionIDs.At(m.level<<16 | m.active[m.level])
}

func (m *HierarchyMenuState) actionEnabled(id gfx.ID) bool {
	if id.IsZero() {
		return false
	}
	if _, row := id.In(m.optionIDs); row {
		return false
	}
	for _, item := range m.scrolls[0].items {
		if item.id == id {
			return !item.disabled
		}
	}
	return false
}

func (m *HierarchyMenuState) cursorAt(id gfx.ID) {
	if index, ok := id.In(m.optionIDs); ok {
		m.level, m.active[index>>16], m.action = index>>16, index&0xFFFF, gfx.ID{}
	} else if m.actionEnabled(id) {
		m.level, m.action = 0, id
	}
	m.reveal = true
}

// moveUnpainted keeps keyboard-only callers usable before the first paint of a
// level. Once painted, MenuScroll owns the complete sequence, including commands.
func (m *HierarchyMenuState) moveUnpainted(key gfx.Key) {
	count := int(m.nodes[m.parentOf(m.level)].count)
	step, next := 1, m.active[m.level]
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
		if n := m.rowNode(m.level, next); n != 0 && !m.nodes[n].disabled {
			m.active[m.level] = next
			break
		}
	}
	m.reveal = true
}

// Key moves within the current level with Up/Down, Home/End and PageUp/PageDown,
// Right into a fly-out, and Left out of it. Enter chooses; Escape closes the
// deepest fly-out. Typeahead and Space activation are enabled by the paint's
// Typeahead option, so editable controls retain their text input. open says
// whether the menu is shown; Down, Up and Enter on a hidden one ask to show it.
// Releases, modified navigation and keys it does not handle are left unhandled.
func (m *HierarchyMenuState) Key(event gfx.KeyEvent, open bool) HierarchyMenuKeyResult {
	if !m.ok || event.Release || event.Modifiers&(gfx.ModifierControl|gfx.ModifierAlt|gfx.ModifierMeta) != 0 {
		return HierarchyMenuKeyResult{}
	}
	// Keyboard navigation owns the next paint, even while the pointer remains
	// parked on a row that used to open a deeper panel.
	m.intent.Reset()
	handled := HierarchyMenuKeyResult{EventResult: gfx.EventResult{Handled: true, Repaint: true}}
	if open && m.typeKeys && (event.Text != "" || event.Key == gfx.KeySpace) {
		text := event.Text
		if text == "" && event.Key == gfx.KeySpace {
			text = " "
		}
		now := time.Now()
		continues := text == " " && m.typeahead.Typing(now)
		if id, ok := m.typeahead.Type(text, now, &m.scrolls[m.level], m.currentID()); ok {
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
		m.typeahead.Reset()
		if id, ok := m.scrolls[m.level].Navigate(event.Key, m.currentID(), true); ok {
			m.cursorAt(id)
		} else if len(m.scrolls[m.level].items) == 0 {
			m.moveUnpainted(event.Key)
		}
		return handled
	case gfx.KeyRight:
		if !open {
			return HierarchyMenuKeyResult{}
		}
		m.typeahead.Reset()
		if m.level == 0 && !m.action.IsZero() {
			return handled
		}
		if n := m.rowNode(m.level, m.active[m.level]); n != 0 && m.nodes[n].count > 0 && m.level+1 < hierarchyMaxLevels {
			m.trail[m.level], m.depth = n, m.level+1
			m.level++
			m.active[m.level] = 0
			m.scrolls[m.level].Offset = 0
			m.scrolls[m.level].items = m.scrolls[m.level].items[:0]
			m.reveal = true
		}
		return handled
	case gfx.KeyLeft:
		if !open || m.level == 0 {
			return HierarchyMenuKeyResult{}
		}
		m.level--
		m.depth = m.level
		m.typeahead.Reset()
		m.reveal = true
		return handled
	case gfx.KeyEnter, gfx.KeySpace:
		if event.Key == gfx.KeySpace && (!open || !m.typeKeys) {
			return HierarchyMenuKeyResult{}
		}
		if event.Repeat {
			return handled
		}
		if !open {
			handled.Opened = true
		} else if m.level == 0 && !m.action.IsZero() {
			if m.actionEnabled(m.action) {
				handled.ActionID = m.action
			}
		} else if value, ok := m.Choose(m.level<<16 | m.active[m.level]); ok {
			handled.Picked, handled.Chosen = true, value
		}
		return handled
	case gfx.KeyEscape:
		if open && m.depth > 0 {
			m.depth--
			m.level = min(m.level, m.depth)
			m.typeahead.Reset()
			return handled
		}
	}
	return HierarchyMenuKeyResult{}
}

// Wheel scrolls the panel under the pointer: the deepest open one that holds it.
func (m *HierarchyMenuState) Wheel(event gfx.ScrollEvent) bool {
	for level := m.depth; level >= 0; level-- {
		if pointInRect(gfx.Point{X: event.X, Y: event.Y}, m.panels[level]) {
			return m.scrolls[level].Wheel(event)
		}
	}
	return false
}

// Pointer drags the scrollbar of whichever panel the event belongs to.
func (m *HierarchyMenuState) Pointer(event gfx.PointerEvent) gfx.EventResult {
	for level := 0; level <= m.depth; level++ {
		scroll := &m.scrolls[level]
		if !scroll.BarID.IsZero() && (event.Target == scroll.BarID || scroll.Dragging()) {
			return gfx.EventResult{Handled: true, Repaint: scroll.Drag(event)}
		}
	}
	return gfx.EventResult{}
}

// measureWidths visits the tree only after its values or font size change.
// Per-depth widths keep every ancestor in place when another branch opens.
func (m *HierarchyMenuState) measureWidths(canvas *gfx.Canvas, size float32, parent int32, level int) {
	m.widest.levels = max(m.widest.levels, level+1)
	for n := m.nodes[parent].first; n != 0; n = m.nodes[n].next {
		width := float32(math.Ceil(float64(canvas.TextWidthWeight(m.nodes[n].name, size, gfx.WeightRegular)))) + 47
		m.widest.widths[level] = max(m.widest.widths[level], width)
		if m.nodes[n].count > 0 {
			m.measureWidths(canvas, size, n, level+1)
		}
	}
}

// PaintHierarchyMenu paints the first list under anchor and a fly-out beside
// each open row, and returns the box around every panel. Its complete
// path is placed before any fly-out opens, so hovering never moves a parent out
// from under the pointer. Narrow viewports show one readable level with parent
// and child navigation buttons instead. Hovering a row in a cascade opens its
// fly-out at once; switching rows waits a short rest on the frame clock.
func PaintHierarchyMenu(canvas *gfx.Canvas, theme gfx.Theme, anchor gfx.Rect, m *HierarchyMenuState, o HierarchyMenuOptions) gfx.Rect {
	if canvas == nil || !m.ok {
		return gfx.Rect{}
	}
	floating := o.Viewport.Width > 0 && o.Viewport.Height > 0
	if o.Overlay {
		canvas.PushLayer(o.Layer)
		defer canvas.PopLayer()
		canvas.PushFocusScope(o.Scope)
		defer canvas.PopFocusScope()
		canvas.TrapFocus(o.Scope)
		canvas.SetEscapeTarget(o.DismissID)
		if floating {
			canvas.PushOverlayClip(o.Viewport)
		}
		canvas.HitWithOptions(o.DismissID, o.Viewport, o.Label, gfx.HitOptions{Role: gfx.RoleButton, SkipFocus: true, Backdrop: true, PassSecondary: true})
		if floating {
			canvas.PopOverlayClip()
		}
	}
	if floating {
		canvas.PushOverlayClip(o.Viewport)
		defer canvas.PopOverlayClip()
	}
	m.optionIDs, m.typeKeys = o.OptionIDs, o.Typeahead
	if m.initial {
		if m.value == "" && m.level == 0 && m.active[0] == 0 {
			for _, action := range o.Leading {
				if !action.Disabled && !action.ID.IsZero() {
					m.action = action.ID
					break
				}
			}
			if m.action.IsZero() && m.nodes[0].count == 0 {
				for _, action := range o.Trailing {
					if !action.Disabled && !action.ID.IsZero() {
						m.action = action.ID
						break
					}
				}
			}
		}
		m.initial = false
	}
	widths, left, reverse := m.panelWidths(canvas, theme, anchor, o)
	if m.compact {
		m.level = m.depth
	}
	// Move the cursor before any row is painted, as the flat list does.
	hovered := canvas.Hovered()
	level, row, onRow := 0, 0, false
	if index, ok := hovered.In(o.OptionIDs); ok && index&0xFFFF != hierarchyBarRow {
		level, row, onRow = index>>16, index&0xFFFF, true
	}
	entered := m.hover.entered(hovered)
	onAction := false
	if !m.compact || m.depth == 0 {
		for _, actions := range [][]HierarchyMenuAction{o.Leading, o.Trailing} {
			for _, action := range actions {
				if !action.ID.IsZero() && action.ID == hovered && !action.Disabled {
					onAction = true
					if entered {
						m.level, m.action = 0, action.ID
						m.typeahead.Reset()
					}
				}
			}
		}
	}
	if entered && onRow && (!m.compact || level == m.depth) {
		if n := m.rowNode(level, row); n != 0 && !m.nodes[n].disabled {
			m.level, m.active[level] = level, row
			m.action = gfx.ID{}
			m.typeahead.Reset()
		}
	}
	// A fly-out opens, or a deeper one closes, only after the pointer rests on a
	// row when another fly-out is open, so crossing rows on the way to one
	// changes nothing.
	n := int32(0)
	if onRow {
		n = m.rowNode(level, row)
	}
	opens := n != 0 && m.nodes[n].count > 0 && level+1 < hierarchyMaxLevels && !(m.depth > level && m.trail[level] == n)
	closes := (n != 0 && m.nodes[n].count == 0 || onAction) && m.depth > level
	if !m.compact && (opens || closes) && (entered || m.intent.target == hovered) {
		now, delay := time.Time{}.Add(canvas.Now()), time.Duration(0)
		if m.depth > level {
			delay = hierarchySwitchDelay
		}
		if id, wait := m.intent.Rest(hovered, now, delay); !id.IsZero() {
			m.intent.Reset()
			m.depth = level
			if opens {
				m.trail[level], m.depth = n, level+1
				m.scrolls[level+1].Offset = 0
				m.scrolls[level+1].items = m.scrolls[level+1].items[:0]
			}
		} else if wait > 0 {
			canvas.RequestFrameIn(wait)
		}
	} else {
		m.intent.Reset()
	}

	rows := o.MaxRows
	if rows <= 0 {
		rows = 7
	}
	var union gfx.Rect
	first := 0
	if m.compact {
		first = m.depth
		clear(m.panels[:first])
	}
	for l := first; l <= m.depth; l++ {
		parent := m.parentOf(l)
		count := int(m.nodes[parent].count)
		rowCount := count
		separator := l == 0 && len(o.Trailing) > 0 && count+len(o.Leading) > 0
		if l == 0 {
			rowCount += len(o.Leading) + len(o.Trailing)
		}
		height := float32(max(1, min(rowCount, rows))*32 + 8)
		if separator {
			height += menuSeparatorHeight
		}
		back := m.compact && l > 0
		if back {
			height += 32
		}
		popup := gfx.Rect{X: left, Y: anchor.Y + anchor.Height + 6, Width: widths[l], Height: height}
		if l == first {
			placePopupBelow(&popup, anchor, height, o.Viewport)
		} else {
			prev, trigger := m.panels[l-1], m.triggers[l-1]
			popup.X, popup.Y = prev.X+prev.Width-4, trigger.Y-4
			if reverse {
				popup.X = prev.X - popup.Width + 4
			}
			if o.Viewport.Height > 0 {
				top, bottom := o.Viewport.Y+1, o.Viewport.Y+o.Viewport.Height-1
				popup.Y = max(top, min(popup.Y, bottom-popup.Height))
				popup.Height = min(popup.Height, bottom-popup.Y)
			}
		}
		m.panels[l] = popup
		union = unionRect(union, popup, l == first)

		canvas.Block(popup)
		canvas.Container(popup, gfx.RoleListBox, o.Label)
		paintPopupSurface(canvas, &theme, popup, gfx.ShadowMD)
		if m.compact {
			canvas.Clip(popup)
		}
		scroll := &m.scrolls[l]
		scroll.BarID = o.ScrollID
		if l > 0 {
			scroll.BarID = o.OptionIDs.At(l<<16 | hierarchyBarRow)
		}
		menu := menuSurface{canvas: canvas, theme: theme, bounds: popup, options: menuSurfaceOptions{layer: o.Layer, scope: o.Scope, scroll: scroll}, top: popup.Y + 4, bottom: popup.Y + popup.Height - 4, viewHeight: popup.Height - 8}
		if back {
			// Keep parent navigation outside the scrolling list, so it remains
			// reachable even after scrolling to the last child.
			bounds := gfx.Rect{X: popup.X + 4, Y: popup.Y + 4, Width: max(popup.Width-8, 0), Height: min(float32(32), max(popup.Height-8, 0))}
			label := m.nodes[parent].name
			look := menuItemRecipe(&theme, gfx.State{Hovered: canvas.Hovered() == o.OptionIDs.At(hierarchyBackIndex)})
			if look.Fill.Alpha > 0 {
				canvas.Rounded(bounds, look.Radius, look.Fill)
			}
			paintChevron(canvas, &hierarchyLeftIcon, gfx.Rect{X: bounds.X + 8, Y: bounds.Y, Width: 16, Height: bounds.Height}, look.Foreground)
			text := gfx.Rect{X: bounds.X + 32, Y: bounds.Y, Width: max(bounds.Width-40, 0), Height: bounds.Height}
			canvas.Label(canvas.FitLine(label, theme.Metrics.FontSizeM, gfx.WeightRegular, text.Width), text, gfx.TextStyle{Size: theme.Metrics.FontSizeM, Color: look.Foreground})
			canvas.HitWithOptions(o.OptionIDs.At(hierarchyBackIndex), bounds, label, gfx.HitOptions{Role: gfx.RoleButton, Description: m.nodes[parent].path, SkipFocus: true})
			menu.top += bounds.Height
			menu.viewHeight = max(menu.viewHeight-bounds.Height, 0)
		}
		menu.beginScroll()
		if l == 0 {
			for _, action := range o.Leading {
				m.paintAction(&menu, action)
			}
		}
		i := 0
		for node := m.nodes[parent].first; node != 0; node, i = m.nodes[node].next, i+1 {
			info := m.nodes[node]
			id := o.OptionIDs.At(l<<16 | i)
			rowBounds := menu.rowBounds(32)
			open := l < m.depth && m.trail[l] == node
			if open {
				m.triggers[l] = rowBounds
			}
			if !menu.takeItem(rowBounds, id, info.name, info.disabled) {
				continue
			}
			selected := m.equal(m.value, info.path)
			menu.paintRow(rowBounds, func() {
				look := menuItemRecipe(&theme, gfx.State{Hovered: open || l == m.level && i == m.active[l] && m.action.IsZero(), Disabled: info.disabled})
				if look.Fill.Alpha > 0 {
					canvas.Rounded(rowBounds, look.Radius, look.Fill)
				}
				color := look.Foreground
				reserve := float32(38)
				if selected && info.count > 0 {
					reserve += 16
				}
				labelWidth := max(rowBounds.Width-reserve, 0)
				canvas.Label(canvas.FitLine(info.name, theme.Metrics.FontSizeM, gfx.WeightRegular, labelWidth), gfx.Rect{X: rowBounds.X + 8, Y: rowBounds.Y, Width: labelWidth, Height: rowBounds.Height}, gfx.TextStyle{Size: theme.Metrics.FontSizeM, Color: color})
				switch {
				case info.count > 0 && rowBounds.Width >= reserve:
					chevron := &chevronRightIcon
					if reverse {
						chevron = &hierarchyLeftIcon
					}
					paintChevron(canvas, chevron, gfx.Rect{X: rowBounds.X + rowBounds.Width - 25, Y: rowBounds.Y, Width: 16, Height: rowBounds.Height}, color)
					if selected {
						paintCheck(canvas, gfx.Rect{X: rowBounds.X + rowBounds.Width - 41, Y: rowBounds.Y, Width: 16, Height: rowBounds.Height}, 14, color)
					}
				case selected && rowBounds.Width >= reserve:
					paintCheck(canvas, gfx.Rect{X: rowBounds.X + rowBounds.Width - 25, Y: rowBounds.Y, Width: 16, Height: rowBounds.Height}, 14, color)
				}
				canvas.HitWithOptions(id, rowBounds, info.name, gfx.HitOptions{Role: gfx.RoleOption, Value: info.path, Selected: selected, Expanded: open, Disabled: info.disabled, SkipFocus: true})
				if m.compact && info.count > 0 {
					button := gfx.Rect{X: rowBounds.X + rowBounds.Width - 30, Y: rowBounds.Y, Width: 30, Height: rowBounds.Height}
					canvas.HitWithOptions(o.OptionIDs.At((hierarchyOpenLevel+l)<<16|i), button, info.name, gfx.HitOptions{Role: gfx.RoleButton, Value: info.path, Expandable: true, Disabled: info.disabled, SkipFocus: true})
				}
			})
		}
		if l == 0 {
			if separator {
				menu.addSeparator()
			}
			for _, action := range o.Trailing {
				m.paintAction(&menu, action)
			}
			if !m.action.IsZero() && !m.actionEnabled(m.action) {
				m.action = gfx.ID{}
			}
		}
		if rowCount == 0 {
			canvas.LabelCenter(o.EmptyLabel, popup, gfx.TextStyle{Size: theme.Metrics.FontSizeM, Color: theme.MutedForeground})
		}
		menu.end()
		if m.compact {
			canvas.Unclip()
		}
	}
	if m.reveal {
		if m.scrolls[m.level].Reveal(m.currentID()) {
			canvas.RequestFrame()
		}
		m.reveal = false
	}
	return union
}

func (m *HierarchyMenuState) paintAction(menu *menuSurface, action HierarchyMenuAction) {
	row := menu.rowBounds(32)
	if !menu.takeItem(row, action.ID, action.Label, action.Disabled) {
		return
	}
	menu.paintRow(row, func() {
		canvas, theme := menu.canvas, menu.theme
		look := menuItemRecipe(&theme, gfx.State{Hovered: m.level == 0 && m.action == action.ID, Disabled: action.Disabled})
		if look.Fill.Alpha > 0 {
			canvas.Rounded(row, look.Radius, look.Fill)
		}
		color := look.Foreground
		label := gfx.Rect{X: row.X + 8, Y: row.Y, Width: max(row.Width-16, 0), Height: row.Height}
		canvas.Label(canvas.FitLine(action.Label, theme.Metrics.FontSizeM, gfx.WeightRegular, label.Width), label, gfx.TextStyle{Size: theme.Metrics.FontSizeM, Color: color})
		row.Width = max(row.Width-menu.hitInset, 0)
		canvas.HitWithOptions(action.ID, row, action.Label, gfx.HitOptions{Role: gfx.RoleMenuItem, Disabled: action.Disabled, SkipFocus: true})
	})
}

// panelWidths reserves the complete cascade before the first hover. There are
// at most hierarchyMaxLevels columns. A common width cap gives long labels the
// room left by short ones. Below a readable column width it uses one panel with
// explicit navigation; shrinking eight columns to blank slivers is not useful.
func (m *HierarchyMenuState) panelWidths(canvas *gfx.Canvas, theme gfx.Theme, anchor gfx.Rect, o HierarchyMenuOptions) (widths [hierarchyMaxLevels]float32, left float32, reverse bool) {
	if m.widest.rebuilt != m.rebuilt || m.widest.size != theme.Metrics.FontSizeM {
		m.widest.widths, m.widest.levels = [hierarchyMaxLevels]float32{}, 0
		m.measureWidths(canvas, theme.Metrics.FontSizeM, 0, 0)
		m.widest.rebuilt, m.widest.size = m.rebuilt, theme.Metrics.FontSizeM
	}
	count := max(m.widest.levels, 1)
	rootWidth := float32(0)
	for _, actions := range [][]HierarchyMenuAction{o.Leading, o.Trailing} {
		for _, action := range actions {
			rootWidth = max(rootWidth, float32(math.Ceil(float64(canvas.TextWidthWeight(action.Label, theme.Metrics.FontSizeM, gfx.WeightRegular))))+24)
		}
	}
	span := float32(-4 * (count - 1))
	for level := 0; level < count; level++ {
		widths[level] = max(m.widest.widths[level], 144)
		if level == 0 {
			widths[level] = max(widths[level], anchor.Width, rootWidth)
		}
		span += widths[level]
	}
	left = anchor.X
	m.compact = false
	viewport := o.Viewport
	if viewport.Width <= 0 {
		return widths, left, false
	}
	start, end := viewport.X+1, viewport.X+viewport.Width-1
	rootStart := max(start, min(anchor.X, end))
	rootEnd := max(start, min(anchor.X+anchor.Width, end))
	available := max(end-rootStart, 0)
	rootMinimum := max(float32(hierarchyMinColumn), anchor.Width)
	minimum := rootMinimum + float32(hierarchyMinColumn*(count-1)-4*(count-1))
	if available >= minimum {
		left = rootStart
	} else if rootEnd-start >= minimum {
		// Keep the root at its anchor, and all fly-outs on the same side.
		// Flipping individual panels covers ancestors after the next turn.
		reverse, available = true, rootEnd-start
	} else {
		m.compact = true
		width := min(max(slices.Max(widths[:count]), anchor.Width), max(viewport.Width-2, 0))
		for level := range widths {
			widths[level] = width
		}
		left = max(start, min(left, end-width))
		return widths, left, false
	}
	if span > available {
		ordered := widths
		budget := available + float32(4*(count-1))
		first := 0
		if rootMinimum > budget/float32(count) {
			widths[0], budget, first = rootMinimum, budget-rootMinimum, 1
		}
		slices.Sort(ordered[first:count])
		limit := budget / float32(count-first)
		for index, width := range ordered[first : count-1] {
			if width >= limit {
				break
			}
			budget -= width
			limit = budget / float32(count-first-index-1)
		}
		for level := first; level < count; level++ {
			widths[level] = min(widths[level], limit)
		}
	}
	if reverse {
		left = rootEnd - widths[0]
	}
	return widths, left, reverse
}

// placePopupBelow keeps a popup inside the viewport: above the anchor when that
// holds a usable row and below does not fit, shortened to what room is left.
func placePopupBelow(popup *gfx.Rect, anchor gfx.Rect, height float32, viewport gfx.Rect) {
	if viewport.Height > 0 {
		top, bottom := viewport.Y+1, viewport.Y+viewport.Height-1
		if popup.Y+popup.Height > bottom {
			// Prefer the room above the anchor, but only when it holds a usable
			// row; otherwise the list scrolls inside what is left below.
			if above := anchor.Y - 6 - top; above >= min(height, 40) {
				popup.Height = min(height, above)
				popup.Y = anchor.Y - popup.Height - 6
			}
		}
		popup.Y = max(top, min(popup.Y, bottom-1))
		popup.Height = max(1, min(popup.Height, bottom-popup.Y))
	}
}

// unionRect is the smallest rect holding both; the first rect starts it.
func unionRect(a, b gfx.Rect, first bool) gfx.Rect {
	if first {
		return b
	}
	x, y := min(a.X, b.X), min(a.Y, b.Y)
	return gfx.Rect{X: x, Y: y, Width: max(a.X+a.Width, b.X+b.Width) - x, Height: max(a.Y+a.Height, b.Y+b.Height) - y}
}
