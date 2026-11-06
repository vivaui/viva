package components

import (
	"math"
	"slices"
	"strings"
	"time"

	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

// SingleSelectOption is a stable selectable value; an empty Label displays Value.
type SingleSelectOption struct {
	Value, Label string
	Disabled     bool
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
	// Separator, when set, makes the popup hierarchical: values are paths whose
	// segments are split at Separator, the list shows the first segment and a
	// row with children opens a fly-out of the next. Choosing any row, a parent
	// included, selects the path up to it. Typing falls back to a flat list of
	// full paths. Rows show their own segment, not the option's Label.
	Separator       string
	value           string
	options, custom []SingleSelectOption
	choices         []SingleSelectChoice
	validateCreate  func(string) bool
	revision, built uint64
	// rebuilt counts list rebuilds; widest caches the widest existing label
	// for the list and font size it was measured at.
	rebuilt, widestRebuilt          uint64
	widestSize, widest              float32
	lastQuery, lastSep              string
	tree                            selectTree
	lastAllow, lastCase, prepared   bool
	scroll                          MenuScroll
	openPainted, focusInput, reveal bool
	// hover is the row the pointer last crossed onto: hovering that row moves
	// Active there, so the highlight and Enter name one row instead of two.
	hover                                 pointerRow
	createPrefix, createValue, createText string
}

func (s *SingleSelectState) SetOptions(options []SingleSelectOption) {
	s.options = append(s.options[:0], options...)
	s.revision++
}

func (s *SingleSelectState) Value() string { return s.value }

func (s *SingleSelectState) SetValue(value string) {
	s.value = value
	s.Query = text.Editor{}
	s.revision++
}

func (s *SingleSelectState) SetValidateCreate(validate func(string) bool) {
	s.validateCreate = validate
	s.revision++
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
	s.tree.depth, s.tree.level = 0, 0
}

func (s *SingleSelectState) equal(a, b string) bool {
	return a == b || s.MatchCaseInsensitive && strings.EqualFold(a, b)
}

func (s *SingleSelectState) label(value string) string {
	for _, option := range s.options {
		if s.equal(option.Value, value) && option.Label != "" {
			return option.Label
		}
	}
	return value
}

// Choices returns a read-only view until the next query or selection change.
// It caches filtered rows, preserving selected and newly created values
// even when the caller's options do not contain them. Exact labels and disabled
// values cannot be bypassed through a duplicate create row.
func (s *SingleSelectState) Choices() []SingleSelectChoice {
	query := strings.TrimSpace(s.Query.String())
	if s.prepared && s.built == s.revision && query == s.lastQuery && s.lastAllow == s.AllowCreate && s.lastCase == s.MatchCaseInsensitive && s.lastSep == s.Separator {
		return s.choices
	}
	if query != s.lastQuery && query != "" {
		s.Open = true
	}
	s.choices = s.choices[:0]
	exact, haveValue := false, s.value == ""
	match := strings.ToLower(query)
	add := func(option SingleSelectOption) {
		label := option.Label
		if label == "" {
			label = option.Value
		}
		exact = exact || s.equal(query, option.Value) || s.equal(query, label)
		haveValue = haveValue || s.equal(s.value, option.Value)
		if query != "" && !strings.Contains(strings.ToLower(label), match) && !strings.Contains(strings.ToLower(option.Value), match) {
			return
		}
		option.Label = label
		s.choices = append(s.choices, SingleSelectChoice{SingleSelectOption: option, Selected: s.equal(s.value, option.Value)})
	}
	for _, option := range s.options {
		add(option)
	}
	for _, option := range s.custom {
		if !slices.ContainsFunc(s.options, func(o SingleSelectOption) bool { return s.equal(o.Value, option.Value) }) {
			add(option)
		}
	}
	if !haveValue {
		add(SingleSelectOption{Value: s.value})
	}
	if s.AllowCreate && query != "" && !exact {
		s.choices = append(s.choices, SingleSelectChoice{})
		copy(s.choices[1:], s.choices[:len(s.choices)-1])
		s.choices[0] = SingleSelectChoice{SingleSelectOption: SingleSelectOption{Value: query, Label: query, Disabled: s.validateCreate != nil && !s.validateCreate(query)}, Create: true}
	}
	s.Active = 0
	for i, choice := range s.choices {
		if !choice.Disabled && (s.choices[s.Active].Disabled || query == "" && choice.Selected) {
			s.Active = i
		}
	}
	s.lastQuery, s.built, s.prepared = query, s.revision, true
	s.rebuilt++
	s.lastAllow, s.lastCase, s.lastSep = s.AllowCreate, s.MatchCaseInsensitive, s.Separator
	s.tree.ok = false
	if s.Separator != "" && query == "" {
		s.buildTree()
	}
	s.scroll.Offset, s.reveal = 0, true
	// A rebuilt list is a new set of rows: let the pointer adopt the row it is
	// over again, or a pointer parked on a row would lose it to the reset above.
	s.hover = pointerRow{}
	return s.choices
}

// Choose always selects a single value, including when the row was selected.
//
// In a hierarchical list index is the number of the row's identity: its level
// shifted left by 16 plus its row, which is the plain row number in the first
// level and in a flat list.
func (s *SingleSelectState) Choose(index int) bool {
	choices := s.Choices()
	if s.treeActive() {
		return s.treeChoose(index)
	}
	if index < 0 || index >= len(choices) || choices[index].Disabled {
		return false
	}
	choice := choices[index]
	if choice.Create {
		s.custom = append(s.custom, choice.SingleSelectOption)
	}
	s.SetValue(choice.Value)
	s.Open, s.focusInput = false, true
	return true
}

func (s *SingleSelectState) Clear() {
	s.SetValue("")
	s.Open, s.focusInput = false, true
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

func (s *SingleSelectState) Key(event ui.KeyEvent) ui.EventResult {
	if event.Release || event.Modifiers != 0 && !(event.Key == ui.KeyTab && event.Modifiers == ui.ModifierShift) {
		return ui.EventResult{}
	}
	if _, composing := s.Query.Composition(); composing {
		return ui.EventResult{Handled: event.Key == ui.KeyEnter}
	}
	choices := s.Choices()
	if s.treeActive() {
		if result, handled := s.treeKey(event); handled {
			return result
		}
	}
	switch event.Key {
	case ui.KeyDown, ui.KeyUp:
		if !s.Open {
			s.Open = true
			return ui.EventResult{Handled: true, Repaint: true}
		}
		step := 1
		if event.Key == ui.KeyUp {
			step = -1
		}
		for i, next := 0, s.Active; i < len(choices); i++ {
			next = (next + step + len(choices)) % len(choices)
			if !choices[next].Disabled {
				s.Active = next
				break
			}
		}
		s.reveal = true
		return ui.EventResult{Handled: true, Repaint: true}
	case ui.KeyEnter:
		if !event.Repeat {
			if !s.Open && s.Query.String() == "" {
				s.Open = true
			} else {
				s.Choose(s.Active)
			}
		}
		return ui.EventResult{Handled: true, Repaint: true}
	case ui.KeyEscape:
		if s.Open || s.Query.String() != "" {
			s.Dismiss()
			return ui.EventResult{Handled: true, Repaint: true}
		}
	case ui.KeyTab:
		if s.Open {
			// Leaving the field drops the search text: a closed field paints the
			// selected label, never the query it did not commit.
			s.Dismiss()
			return ui.EventResult{Handled: true, Repaint: true, ContinueTraversal: true}
		}
	case ui.KeyBackspace:
		if !event.Repeat && s.Query.String() == "" && s.value != "" {
			s.Clear()
			return ui.EventResult{Handled: true, Repaint: true}
		}
	}
	return ui.EventResult{}
}

// Wheel scrolls the panel under the pointer: the deepest open one that holds it.
func (s *SingleSelectState) Wheel(event ui.ScrollEvent) bool {
	if s.treeActive() {
		for level := s.tree.depth; level >= 0; level-- {
			if pointInRect(ui.Point{X: event.X, Y: event.Y}, s.tree.panels[level]) {
				return s.treeScroll(level).Wheel(event)
			}
		}
		return false
	}
	return s.scroll.Wheel(event)
}

func (s *SingleSelectState) Pointer(event ui.PointerEvent) ui.EventResult {
	levels := 1
	if s.treeActive() {
		levels += s.tree.depth
	}
	for level := 0; level < levels; level++ {
		scroll := s.treeScroll(level)
		if !scroll.BarID.IsZero() && (event.Target == scroll.BarID || scroll.Dragging()) {
			return ui.EventResult{Handled: true, Repaint: scroll.Drag(event)}
		}
	}
	return ui.EventResult{}
}

// SingleSelectOptions names the control's hit IDs, its popup layer and the
// strings the caller supplies for its labels and create row.
type SingleSelectOptions struct {
	InputID, TriggerID, ClearID, DismissID, Scope, ScrollID  ui.ID
	OptionIDs                                                ui.Scope
	Label, Placeholder, EmptyLabel, CreatePrefix, ClearLabel string
	// Viewport bounds the popup and backdrop independently of the input's
	// ancestor scroll clips. Set it to the window or a narrower overlay region.
	Viewport          ui.Rect
	Layer             uint16
	Disabled, Invalid bool
	MaxRows           int
}

// popupWidth is the width that shows every label of choices untruncated.
func (s *SingleSelectState) popupWidth(canvas *ui.Canvas, theme ui.Theme, choices []SingleSelectChoice, o SingleSelectOptions) float32 {
	size := theme.Metrics.FontSizeM
	if s.widestRebuilt != s.rebuilt || s.widestSize != size {
		s.widest = 0
		for _, choice := range choices {
			if !choice.Create {
				s.widest = max(s.widest, canvas.TextWidthWeight(choice.Label, size, ui.WeightRegular))
			}
		}
		s.widestRebuilt, s.widestSize = s.rebuilt, size
	}
	widest := s.widest
	if len(choices) > 0 && choices[0].Create {
		widest = max(widest, canvas.TextWidthWeight(o.CreatePrefix+choices[0].Label, size, ui.WeightRegular))
	}
	// A row spends 8 on the left and 30 on the check before the menu padding.
	return float32(math.Ceil(float64(widest))) + 46 + 1
}

// SingleSelectLayout is where the control, its native input and its popup were
// painted, for callers that route text input or place overlays.
type SingleSelectLayout struct{ Bounds, Input, Popup ui.Rect }

// PaintSingleSelect shares text-entry and menu primitives with Combobox and
// MultiSelect. The query stays a native editor; no chip or multi-selection is
// introduced when the selected value changes.
func PaintSingleSelect(canvas *ui.Canvas, theme ui.Theme, bounds ui.Rect, s *SingleSelectState, o SingleSelectOptions) SingleSelectLayout {
	if bounds.Height <= 0 {
		bounds.Height = theme.Metrics.ControlHeight
	}
	layout := SingleSelectLayout{Bounds: bounds}
	if canvas == nil || bounds.Width <= 0 || bounds.Height <= 0 {
		return layout
	}
	choices := s.Choices()
	open := s.Open && !o.Disabled
	floating := o.Viewport.Width > 0 && o.Viewport.Height > 0
	if s.openPainted && !open && canvas.Focused() == o.InputID {
		canvas.RequestFocus(o.InputID)
	}
	s.openPainted = open
	if open {
		canvas.PushLayer(o.Layer)
		defer canvas.PopLayer()
		canvas.PushFocusScope(o.Scope)
		defer canvas.PopFocusScope()
		canvas.TrapFocus(o.Scope)
		canvas.SetInitialFocus(o.InputID)
		canvas.SetEscapeTarget(o.DismissID)
		if floating {
			canvas.PushOverlayClip(o.Viewport)
		}
		canvas.HitWithOptions(o.DismissID, o.Viewport, o.Label, ui.HitOptions{Role: ui.RoleButton, SkipFocus: true, Backdrop: true, PassSecondary: true})
		if floating {
			canvas.PopOverlayClip()
		}
	}
	paintTextEntrySurface(canvas, theme, bounds, textEntryState{Focused: canvas.Focused() == o.InputID || open, Disabled: o.Disabled, Invalid: o.Invalid})
	canvas.HitWithOptions(o.TriggerID, bounds, o.Label, ui.HitOptions{Role: ui.RoleButton, Expanded: open, Disabled: o.Disabled, SkipFocus: true})
	trigger := ui.Rect{X: bounds.X + bounds.Width - 28, Y: bounds.Y + (bounds.Height-24)/2, Width: 24, Height: 24}
	right := trigger.X - 4
	query := s.Query.String()
	iconColor := theme.MutedForeground
	if o.Disabled {
		iconColor.Alpha *= .5
	}
	if !o.ClearID.IsZero() && (s.value != "" || query != "") {
		clear := ui.Rect{X: right - 24, Y: trigger.Y, Width: 24, Height: 24}
		if !o.Disabled && canvas.Hovered() == o.ClearID {
			canvas.Rounded(clear, theme.Metrics.RadiusItem(), theme.Muted)
		}
		paintCross(canvas, clear, 12, iconColor)
		label := o.ClearLabel
		if label == "" {
			label = "Clear selection"
		}
		canvas.HitWithOptions(o.ClearID, clear, label, ui.HitOptions{Role: ui.RoleButton, Disabled: o.Disabled})
		right = clear.X - 4
	}
	inset := theme.Metrics.TextInsetX()
	layout.Input = ui.Rect{X: bounds.X + inset, Y: bounds.Y + 1, Width: max(1, right-bounds.X-inset), Height: bounds.Height - 2}
	display, color := query, theme.Foreground
	if display == "" {
		display = s.label(s.value)
		if display == "" {
			display, color = o.Placeholder, theme.MutedForeground
		} else if open {
			color = theme.MutedForeground
		}
	}
	if o.Disabled {
		color.Alpha *= .5
	}
	if canvas.Focused() != o.InputID || query == "" {
		canvas.Label(canvas.FitLine(display, theme.Metrics.FontSizeM, ui.WeightRegular, layout.Input.Width), layout.Input, ui.TextStyle{Size: theme.Metrics.FontSizeM, Color: color})
	}
	accessibleValue := query
	if accessibleValue == "" {
		accessibleValue = s.label(s.value)
	}
	canvas.HitWithOptions(o.InputID, layout.Input, o.Label, ui.HitOptions{Role: ui.RoleCombobox, Value: accessibleValue, Expanded: open, Disabled: o.Disabled, Cursor: ui.PointerCursorText, TextBounds: layout.Input, TextSize: theme.Metrics.FontSizeM, TextColor: theme.Foreground, NavigationKeys: true})
	paintChevron(canvas, &chevronDownIcon, trigger, iconColor)
	if s.focusInput {
		canvas.RequestFocus(o.InputID)
		s.focusInput = false
	}
	if !open {
		return layout
	}
	if s.treeActive() {
		layout.Popup = s.paintTree(canvas, theme, bounds, o)
		return layout
	}
	rows := o.MaxRows
	if rows <= 0 {
		rows = 7
	}
	height := float32(max(1, min(len(choices), rows))*32 + 8)
	layout.Popup = ui.Rect{X: bounds.X, Y: bounds.Y + bounds.Height + 6, Width: bounds.Width, Height: height}
	// Like Radix's popper content, the list is at least as wide as the field and
	// grows to its longest label so options that share a long prefix stay
	// distinguishable, until the viewport stops it.
	if want := s.popupWidth(canvas, theme, choices, o); want > layout.Popup.Width {
		layout.Popup.Width = want
		if o.Viewport.Width > 0 {
			layout.Popup.Width = min(want, max(o.Viewport.Width-2, bounds.Width))
			layout.Popup.X = max(o.Viewport.X+1, min(layout.Popup.X, o.Viewport.X+o.Viewport.Width-1-layout.Popup.Width))
		}
	}
	s.placePopup(&layout.Popup, bounds, height, o)
	if floating {
		canvas.PushOverlayClip(o.Viewport)
		defer canvas.PopOverlayClip()
	}
	canvas.Block(layout.Popup)
	canvas.Container(layout.Popup, ui.RoleListBox, o.Label)
	canvas.Elevate(layout.Popup, theme.Metrics.RadiusPopup(), ui.ShadowMD)
	canvas.RoundedBorder(layout.Popup, theme.Metrics.RadiusPopup(), 1, theme.Border, theme.Popover)
	s.scroll.BarID = o.ScrollID
	menu := menuSurface{canvas: canvas, theme: theme, bounds: layout.Popup, options: menuSurfaceOptions{layer: o.Layer, scope: o.Scope, scroll: &s.scroll}, top: layout.Popup.Y + 4, bottom: layout.Popup.Y + layout.Popup.Height - 4, viewHeight: layout.Popup.Height - 8}
	menu.beginScroll()
	// Move the cursor before any row is painted: syncing it while the rows are
	// drawn would tint the previous row and the new one in the same frame.
	hovered := canvas.Hovered()
	if s.hover.entered(hovered) && !hovered.IsZero() {
		for i := range choices {
			if o.OptionIDs.At(i) == hovered && !choices[i].Disabled {
				s.Active = i
				break
			}
		}
	}
	for i, choice := range choices {
		id := o.OptionIDs.At(i)
		row := menu.rowBounds(32)
		if !menu.takeItem(row, id, choice.Label, choice.Disabled) {
			continue
		}
		menu.paintRow(row, func() {
			if !choice.Disabled && i == s.Active {
				canvas.Rounded(row, theme.Metrics.RadiusItem(), theme.Accent)
			}
			color := theme.PopoverForeground
			if choice.Disabled {
				color.Alpha *= .5
			}
			label := choice.Label
			if choice.Create {
				if s.createPrefix != o.CreatePrefix || s.createValue != label {
					s.createPrefix, s.createValue = o.CreatePrefix, label
					s.createText = o.CreatePrefix + label
				}
				label = s.createText
			}
			canvas.Label(canvas.FitLine(label, theme.Metrics.FontSizeM, ui.WeightRegular, max(row.Width-38, 0)), ui.Rect{X: row.X + 8, Y: row.Y, Width: max(row.Width-38, 0), Height: row.Height}, ui.TextStyle{Size: theme.Metrics.FontSizeM, Color: color})
			if choice.Selected {
				paintCheck(canvas, ui.Rect{X: row.X + row.Width - 25, Y: row.Y, Width: 16, Height: row.Height}, 14, color)
			}
			canvas.HitWithOptions(id, row, label, ui.HitOptions{Role: ui.RoleOption, Value: choice.Value, Selected: choice.Selected, Disabled: choice.Disabled, SkipFocus: true})
		})
	}
	if len(choices) == 0 {
		canvas.LabelCenter(o.EmptyLabel, layout.Popup, ui.TextStyle{Size: theme.Metrics.FontSizeM, Color: theme.MutedForeground})
	}
	menu.end()
	if s.reveal && s.scroll.Reveal(o.OptionIDs.At(s.Active)) {
		canvas.RequestFrame()
	}
	s.reveal = false
	return layout
}

// placePopup keeps the popup inside the viewport: above the field when that
// holds a usable row and below does not fit, shortened to what room is left.
func (s *SingleSelectState) placePopup(popup *ui.Rect, bounds ui.Rect, height float32, o SingleSelectOptions) {
	if o.Viewport.Height > 0 {
		top, bottom := o.Viewport.Y+1, o.Viewport.Y+o.Viewport.Height-1
		if popup.Y+popup.Height > bottom {
			// Prefer the room above the field, but only when it holds a usable
			// row; otherwise the list scrolls inside what is left below.
			if above := bounds.Y - 6 - top; above >= min(height, 40) {
				popup.Height = min(height, above)
				popup.Y = bounds.Y - popup.Height - 6
			}
		}
		popup.Y = max(top, min(popup.Y, bottom-1))
		popup.Height = max(1, min(popup.Height, bottom-popup.Y))
	}
}

// selectMaxLevels is how many panels (the list plus its fly-outs) a tree
// SingleSelect can show at once. Values nested deeper make the whole list fall
// back to the flat one, so no value becomes unreachable.
const selectMaxLevels = 8

// selectSwitchDelay is how long the pointer rests on another row before an open
// fly-out gives way to that row's, or closes. A fly-out opens at once when none
// is open. The rest keeps the pointer's diagonal across the rows beside the
// trigger on its way into the fly-out from switching it; the canvas has no
// pointer position to aim with instead.
const selectSwitchDelay = 60 * time.Millisecond

// selectBarRow is the reserved row number, within a level, of that level's
// scrollbar hit.
const selectBarRow = 0xFFFF

// selectNode is one segment of a value path. Children are a singly linked list
// in first-seen order; node 0 is the invisible root.
type selectNode struct {
	name, path        string
	first, last, next int32
	count             int32
	disabled          bool
}

// selectTree is the segment tree of the current options, and which fly-outs
// are open. Level l of the popup lists the children of trail[l-1] (the root for
// level 0); depth is the number of open fly-outs, so levels 0..depth show.
type selectTree struct {
	nodes  []selectNode
	ok     bool
	trail  [selectMaxLevels]int32
	active [selectMaxLevels]int
	depth  int
	// level is the panel the keyboard cursor is in.
	level  int
	intent HoverIntent
	// panels and triggers are where the last paint put each open panel and
	// the row that opened it.
	panels, triggers [selectMaxLevels]ui.Rect
	sub              [selectMaxLevels - 1]MenuScroll
	// widest caches the widest label of one level's list: keyed by the parent
	// node, the rebuild and the font size it was measured at.
	widest [selectMaxLevels]struct {
		parent  int32
		rebuilt uint64
		size    float32
		width   float32
		valid   bool
	}
}

// treeActive reports whether the popup is the hierarchical one: a separator is
// set, no search text is typed, and every value fits the supported depth.
func (s *SingleSelectState) treeActive() bool {
	s.Choices()
	return s.tree.ok && s.Separator != "" && s.lastQuery == ""
}

func (s *SingleSelectState) treeScroll(level int) *MenuScroll {
	if level == 0 {
		return &s.scroll
	}
	return &s.tree.sub[level-1]
}

func (t *selectTree) parentOf(level int) int32 {
	if level == 0 {
		return 0
	}
	return t.trail[level-1]
}

// rowNode is the node of row of level, or 0 when there is none.
func (t *selectTree) rowNode(level, row int) int32 {
	if level < 0 || level > t.depth || row < 0 {
		return 0
	}
	parent := t.parentOf(level)
	if row >= int(t.nodes[parent].count) {
		return 0
	}
	n := t.nodes[parent].first
	for ; row > 0; row-- {
		n = t.nodes[n].next
	}
	return n
}

func (t *selectTree) child(parent int32, name string, equal func(a, b string) bool) int32 {
	for n := t.nodes[parent].first; n != 0; n = t.nodes[n].next {
		if equal(t.nodes[n].name, name) {
			return n
		}
	}
	return 0
}

// buildTree splits every existing choice's value at Separator into the segment
// tree. Labels are not used: each row shows its own segment of the value.
func (s *SingleSelectState) buildTree() {
	t := &s.tree
	t.nodes = append(t.nodes[:0], selectNode{})
	t.ok = true
	for _, choice := range s.choices {
		if choice.Create {
			continue
		}
		parent, rest, offset := int32(0), choice.Value, 0
		for depth := 1; ; depth++ {
			if depth > selectMaxLevels {
				t.ok = false
				return
			}
			end := strings.Index(rest, s.Separator)
			segment := rest
			if end >= 0 {
				segment = rest[:end]
			}
			name := strings.TrimSpace(segment)
			child := t.child(parent, name, s.equal)
			if child == 0 {
				child = int32(len(t.nodes))
				t.nodes = append(t.nodes, selectNode{name: name, path: choice.Value[:offset+len(segment)]})
				if p := &t.nodes[parent]; p.count == 0 {
					p.first = child
				} else {
					t.nodes[p.last].next = child
				}
				t.nodes[parent].last = child
				t.nodes[parent].count++
			}
			parent = child
			if end < 0 {
				break
			}
			rest = rest[end+len(s.Separator):]
			offset += len(segment) + len(s.Separator)
		}
		if parent != 0 {
			t.nodes[parent].disabled = choice.Disabled
		}
	}
	s.seekValue()
}

// seekValue opens the fly-outs down to the selected value, with the cursor on
// it, so the list opens where the selection is.
func (s *SingleSelectState) seekValue() {
	t := &s.tree
	t.depth, t.level, t.active = 0, 0, [selectMaxLevels]int{}
	if s.value == "" {
		return
	}
	parent, rest := int32(0), s.value
	for level := 0; ; level++ {
		end := strings.Index(rest, s.Separator)
		segment := rest
		if end >= 0 {
			segment = rest[:end]
		}
		row, n := 0, t.nodes[parent].first
		for ; n != 0; n, row = t.nodes[n].next, row+1 {
			if s.equal(t.nodes[n].name, strings.TrimSpace(segment)) {
				break
			}
		}
		if n == 0 {
			return
		}
		t.active[level], t.level = row, level
		if end < 0 {
			return
		}
		t.trail[level], t.depth = n, level+1
		parent, rest = n, rest[end+len(s.Separator):]
	}
}

// treeKey handles the keys that move through the levels. It reports false for
// keys the flat list handles identically.
func (s *SingleSelectState) treeKey(event ui.KeyEvent) (ui.EventResult, bool) {
	t := &s.tree
	repaint := ui.EventResult{Handled: true, Repaint: true}
	switch event.Key {
	case ui.KeyDown, ui.KeyUp:
		if !s.Open {
			s.Open = true
			return repaint, true
		}
		step := 1
		if event.Key == ui.KeyUp {
			step = -1
		}
		count := int(t.nodes[t.parentOf(t.level)].count)
		for i, next := 0, t.active[t.level]; i < count; i++ {
			next = (next + step + count) % count
			if n := t.rowNode(t.level, next); n != 0 && !t.nodes[n].disabled {
				t.active[t.level] = next
				break
			}
		}
		s.reveal = true
		return repaint, true
	case ui.KeyRight:
		if !s.Open {
			return ui.EventResult{}, false
		}
		if n := t.rowNode(t.level, t.active[t.level]); n != 0 && t.nodes[n].count > 0 && t.level+1 < selectMaxLevels {
			t.trail[t.level], t.depth = n, t.level+1
			t.level++
			t.active[t.level] = 0
			t.sub[t.level-1].Offset = 0
			s.reveal = true
		}
		return repaint, true
	case ui.KeyLeft:
		if !s.Open || t.level == 0 {
			return ui.EventResult{}, false
		}
		t.level--
		t.depth = t.level
		s.reveal = true
		return repaint, true
	case ui.KeyEnter:
		if event.Repeat {
			return repaint, true
		}
		if !s.Open {
			s.Open = true
		} else {
			s.Choose(t.level<<16 | t.active[t.level])
		}
		return repaint, true
	case ui.KeyEscape:
		if s.Open && t.depth > 0 {
			t.depth--
			t.level = min(t.level, t.depth)
			return repaint, true
		}
	}
	return ui.EventResult{}, false
}

// treeChoose selects the node a row identity (level<<16 | row) names.
func (s *SingleSelectState) treeChoose(index int) bool {
	t := &s.tree
	n := t.rowNode(index>>16, index&0xFFFF)
	if n == 0 || t.nodes[n].disabled {
		return false
	}
	s.SetValue(t.nodes[n].path)
	s.Open, s.focusInput = false, true
	return true
}

// levelWidth is the width that shows every row of level untruncated.
func (s *SingleSelectState) levelWidth(canvas *ui.Canvas, theme ui.Theme, level int) float32 {
	t := &s.tree
	parent, size := t.parentOf(level), theme.Metrics.FontSizeM
	cache := &t.widest[level]
	if !cache.valid || cache.parent != parent || cache.rebuilt != s.rebuilt || cache.size != size {
		cache.width = 0
		for n := t.nodes[parent].first; n != 0; n = t.nodes[n].next {
			cache.width = max(cache.width, canvas.TextWidthWeight(t.nodes[n].name, size, ui.WeightRegular))
		}
		cache.parent, cache.rebuilt, cache.size, cache.valid = parent, s.rebuilt, size, true
	}
	// 8 before the label and 30 after it, inside the menu padding on both sides.
	return float32(math.Ceil(float64(cache.width))) + 46 + 1
}

// paintTree paints the hierarchical popup: the first list under the field and a
// fly-out to the right of each open row. It returns the box around every panel.
func (s *SingleSelectState) paintTree(canvas *ui.Canvas, theme ui.Theme, bounds ui.Rect, o SingleSelectOptions) ui.Rect {
	t := &s.tree
	if floating := o.Viewport.Width > 0 && o.Viewport.Height > 0; floating {
		canvas.PushOverlayClip(o.Viewport)
		defer canvas.PopOverlayClip()
	}
	// Move the cursor before any row is painted, as the flat list does.
	hovered := canvas.Hovered()
	level, row, onRow := 0, 0, false
	if index, ok := hovered.In(o.OptionIDs); ok && index&0xFFFF != selectBarRow {
		level, row, onRow = index>>16, index&0xFFFF, true
	}
	if s.hover.entered(hovered) && onRow {
		if n := t.rowNode(level, row); n != 0 && !t.nodes[n].disabled {
			t.level, t.active[level] = level, row
		}
	}
	// A fly-out opens, or a deeper one closes, only after the pointer rests on a
	// row, so crossing other rows on the way to a fly-out changes nothing.
	n := int32(0)
	if onRow {
		n = t.rowNode(level, row)
	}
	opens := n != 0 && t.nodes[n].count > 0 && level+1 < selectMaxLevels && !(t.depth > level && t.trail[level] == n)
	closes := n != 0 && t.nodes[n].count == 0 && t.depth > level
	if opens || closes {
		now, delay := time.Time{}.Add(canvas.Now()), time.Duration(0)
		if t.depth > level {
			delay = selectSwitchDelay
		}
		if id, wait := t.intent.Rest(hovered, now, delay); !id.IsZero() {
			t.depth = level
			if opens {
				t.trail[level], t.depth = n, level+1
				t.sub[level].Offset = 0
			}
		} else if wait > 0 {
			canvas.RequestFrameIn(wait)
		}
	} else {
		t.intent.Reset()
	}

	rows := o.MaxRows
	if rows <= 0 {
		rows = 7
	}
	var union ui.Rect
	for l := 0; l <= t.depth; l++ {
		parent := t.parentOf(l)
		count := int(t.nodes[parent].count)
		height := float32(max(1, min(count, rows))*32 + 8)
		width := max(s.levelWidth(canvas, theme, l), 144)
		popup := ui.Rect{X: bounds.X, Y: bounds.Y + bounds.Height + 6, Width: max(width, bounds.Width), Height: height}
		if l == 0 {
			s.placePopup(&popup, bounds, height, o)
		} else {
			popup.Width = width
			prev, trigger := t.panels[l-1], t.triggers[l-1]
			popup.X, popup.Y = prev.X+prev.Width-4, trigger.Y-4
			if o.Viewport.Width > 0 {
				left, right, top, bottom := o.Viewport.X+1, o.Viewport.X+o.Viewport.Width-1, o.Viewport.Y+1, o.Viewport.Y+o.Viewport.Height-1
				if popup.X+popup.Width > right {
					popup.X = prev.X - popup.Width + 4
				}
				popup.X = max(left, min(popup.X, right-popup.Width))
				popup.Y = max(top, min(popup.Y, bottom-popup.Height))
				popup.Height = min(popup.Height, bottom-popup.Y)
			}
		}
		t.panels[l] = popup
		union = unionRect(union, popup, l == 0)

		canvas.Block(popup)
		canvas.Container(popup, ui.RoleListBox, o.Label)
		canvas.Elevate(popup, theme.Metrics.RadiusPopup(), ui.ShadowMD)
		canvas.RoundedBorder(popup, theme.Metrics.RadiusPopup(), 1, theme.Border, theme.Popover)
		scroll := s.treeScroll(l)
		scroll.BarID = o.ScrollID
		if l > 0 {
			scroll.BarID = o.OptionIDs.At(l<<16 | selectBarRow)
		}
		menu := menuSurface{canvas: canvas, theme: theme, bounds: popup, options: menuSurfaceOptions{layer: o.Layer, scope: o.Scope, scroll: scroll}, top: popup.Y + 4, bottom: popup.Y + popup.Height - 4, viewHeight: popup.Height - 8}
		menu.beginScroll()
		i := 0
		for node := t.nodes[parent].first; node != 0; node, i = t.nodes[node].next, i+1 {
			info := t.nodes[node]
			id := o.OptionIDs.At(l<<16 | i)
			rowBounds := menu.rowBounds(32)
			open := l < t.depth && t.trail[l] == node
			if open {
				t.triggers[l] = rowBounds
			}
			if !menu.takeItem(rowBounds, id, info.name, info.disabled) {
				continue
			}
			selected := s.equal(s.value, info.path)
			menu.paintRow(rowBounds, func() {
				if !info.disabled && (open || l == t.level && i == t.active[l]) {
					canvas.Rounded(rowBounds, theme.Metrics.RadiusItem(), theme.Accent)
				}
				color := theme.PopoverForeground
				if info.disabled {
					color.Alpha *= .5
				}
				reserve := float32(38)
				if selected && info.count > 0 {
					reserve += 16
				}
				labelWidth := max(rowBounds.Width-reserve, 0)
				canvas.Label(canvas.FitLine(info.name, theme.Metrics.FontSizeM, ui.WeightRegular, labelWidth), ui.Rect{X: rowBounds.X + 8, Y: rowBounds.Y, Width: labelWidth, Height: rowBounds.Height}, ui.TextStyle{Size: theme.Metrics.FontSizeM, Color: color})
				switch {
				case info.count > 0:
					paintChevron(canvas, &chevronRightIcon, ui.Rect{X: rowBounds.X + rowBounds.Width - 25, Y: rowBounds.Y, Width: 16, Height: rowBounds.Height}, color)
					if selected {
						paintCheck(canvas, ui.Rect{X: rowBounds.X + rowBounds.Width - 41, Y: rowBounds.Y, Width: 16, Height: rowBounds.Height}, 14, color)
					}
				case selected:
					paintCheck(canvas, ui.Rect{X: rowBounds.X + rowBounds.Width - 25, Y: rowBounds.Y, Width: 16, Height: rowBounds.Height}, 14, color)
				}
				canvas.HitWithOptions(id, rowBounds, info.name, ui.HitOptions{Role: ui.RoleOption, Value: info.path, Selected: selected, Expanded: open, Disabled: info.disabled, SkipFocus: true})
			})
		}
		if count == 0 {
			canvas.LabelCenter(o.EmptyLabel, popup, ui.TextStyle{Size: theme.Metrics.FontSizeM, Color: theme.MutedForeground})
		}
		menu.end()
	}
	if s.reveal {
		if s.treeScroll(t.level).Reveal(o.OptionIDs.At(t.level<<16 | t.active[t.level])) {
			canvas.RequestFrame()
		}
		s.reveal = false
	}
	return union
}

// unionRect is the smallest rect holding both; the first rect starts it.
func unionRect(a, b ui.Rect, first bool) ui.Rect {
	if first {
		return b
	}
	x, y := min(a.X, b.X), min(a.Y, b.Y)
	return ui.Rect{X: x, Y: y, Width: max(a.X+a.Width, b.X+b.Width) - x, Height: max(a.Y+a.Height, b.Y+b.Height) - y}
}
