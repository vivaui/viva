package components

import (
	"math"
	"slices"
	"strings"

	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

// SingleSelectOption is a stable selectable value; an empty Label displays Value.
type SingleSelectOption struct {
	Value, Label string
	Disabled     bool
	// Search is extra text the query matches besides the label and the value,
	// for what the row does not show, such as an address or a group. It is never
	// painted.
	Search string
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
	tree                            HierarchyMenuState
	treeOK                          bool
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
	s.tree.Reset()
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
		if query != "" && !strings.Contains(strings.ToLower(label), match) &&
			!strings.Contains(strings.ToLower(option.Value), match) &&
			!strings.Contains(strings.ToLower(option.Search), match) {
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
	s.treeOK = false
	if s.Separator != "" && query == "" {
		s.treeOK = s.tree.Build(s.Separator, s.value, s.equal, len(s.choices), func(i int) HierarchyMenuItem {
			return HierarchyMenuItem{Value: s.choices[i].Value, Disabled: s.choices[i].Disabled}
		})
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
		value, ok := s.tree.Choose(index)
		if ok {
			s.SetValue(value)
			s.Open, s.focusInput = false, true
		}
		return ok
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
		result := s.tree.Key(event, s.Open)
		if result.Opened {
			s.Open = true
		}
		if result.Picked {
			s.SetValue(result.Chosen)
			s.Open, s.focusInput = false, true
		}
		if result.Handled {
			return result.EventResult
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

// Wheel scrolls the list, or in a hierarchical one the panel under the pointer.
func (s *SingleSelectState) Wheel(event ui.ScrollEvent) bool {
	if s.treeActive() {
		return s.tree.Wheel(event)
	}
	return s.scroll.Wheel(event)
}

func (s *SingleSelectState) Pointer(event ui.PointerEvent) ui.EventResult {
	if s.treeActive() {
		return s.tree.Pointer(event)
	}
	if s.scroll.BarID.IsZero() || event.Target != s.scroll.BarID && !s.scroll.Dragging() {
		return ui.EventResult{}
	}
	return ui.EventResult{Handled: true, Repaint: s.scroll.Drag(event)}
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
	paintTextEntrySurface(canvas, &theme, bounds, ui.State{Focused: canvas.Focused() == o.InputID || open, Disabled: o.Disabled, Invalid: o.Invalid})
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
		layout.Popup = PaintHierarchyMenu(canvas, theme, bounds, &s.tree, HierarchyMenuOptions{
			OptionIDs: o.OptionIDs, ScrollID: o.ScrollID, Scope: o.Scope, Layer: o.Layer, Label: o.Label,
			EmptyLabel: o.EmptyLabel, Viewport: o.Viewport, MaxRows: o.MaxRows,
		})
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
	placePopupBelow(&layout.Popup, bounds, height, o.Viewport)
	if floating {
		canvas.PushOverlayClip(o.Viewport)
		defer canvas.PopOverlayClip()
	}
	canvas.Block(layout.Popup)
	canvas.Container(layout.Popup, ui.RoleListBox, o.Label)
	paintPopupSurface(canvas, &theme, layout.Popup, ui.ShadowMD)
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
			look := menuItemRecipe(&theme, ui.State{Hovered: i == s.Active, Disabled: choice.Disabled})
			if look.Fill.Alpha > 0 {
				canvas.Rounded(row, look.Radius, look.Fill)
			}
			color := look.Foreground
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

// treeActive reports whether the popup is the hierarchical one: a separator is
// set, no search text is typed, and every value fits the supported depth.
func (s *SingleSelectState) treeActive() bool {
	s.Choices()
	return s.treeOK && s.Separator != "" && s.lastQuery == ""
}
