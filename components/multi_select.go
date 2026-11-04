package components

import (
	"slices"
	"strconv"
	"strings"

	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

// MultiSelectOption is a stable selectable value. Empty labels display Value.
type MultiSelectOption struct {
	Value, Label string
	Disabled     bool
}

// MultiSelectChoice is one filtered row. A Create row adds the query as a value.
type MultiSelectChoice struct {
	MultiSelectOption
	Selected, Create bool
}

// MultiSelectState is application-owned selection and native query-editor state.
// Use SetOptions/SetValues rather than editing their slices; Choices caches its
// filtered rows and steady paints reuse their storage. Values remain in selection
// order. Query can be bound directly to the platform text service.
type MultiSelectState struct {
	Query  text.Editor
	Open   bool
	Active int
	// AllowCreate offers the query as a new value when no option matches it.
	AllowCreate bool
	// MatchCaseInsensitive compares values and their labels with Unicode case
	// folding, so "Development" and "development" are one value. It does not
	// control chip overflow: chips always fold into +N when the field is narrow.
	MatchCaseInsensitive bool
	Limit                int // zero means unlimited
	// validateCreate, set with SetValidateCreate, can reject a query without
	// removing its disabled create row.
	validateCreate      func(string) bool
	options             []MultiSelectOption
	values              []string
	choices             []MultiSelectChoice
	scroll              MenuScroll
	revision, built     uint64
	lastQuery           string
	lastAllow, lastCase bool
	lastLimit           int
	prepared            bool
	// openPainted is the popup state of the previous paint, so closing it can
	// hand focus back to a query input the popup's scope had taken.
	openPainted        bool
	focusInput, reveal bool
	// hover is the row the pointer last crossed onto: hovering that row moves
	// Active there, so the highlight and Enter name one row instead of two.
	hover                                 pointerRow
	createText, createPrefix, createValue string
	overflowText                          string
	overflowCount                         int
}

func (s *MultiSelectState) SetOptions(options []MultiSelectOption) {
	s.options = append(s.options[:0], options...)
	s.revision++
}

func (s *MultiSelectState) SetValues(values []string) {
	s.values = s.values[:0]
	for _, value := range values {
		if !s.contains(value) {
			s.values = append(s.values, value)
		}
	}
	s.revision++
}

// Values is a read-only view until the next selection change.
func (s *MultiSelectState) Values() []string { return s.values }

// SetValidateCreate replaces the create validator; nil accepts every query. It
// re-filters at the next paint, so a host that changes its rules between forms
// never shows a create row judged by the previous rules.
func (s *MultiSelectState) SetValidateCreate(validate func(string) bool) {
	s.validateCreate = validate
	s.revision++
}

func (s *MultiSelectState) equal(a, b string) bool {
	return a == b || s.MatchCaseInsensitive && strings.EqualFold(a, b)
}

func (s *MultiSelectState) contains(value string) bool {
	return slices.ContainsFunc(s.values, func(other string) bool { return s.equal(other, value) })
}

func (s *MultiSelectState) label(value string) string {
	for _, option := range s.options {
		if s.equal(option.Value, value) && option.Label != "" {
			return option.Label
		}
	}
	return value
}

// Choices returns filtered options, including selected custom values that no
// longer occur in Options. Creating never shadows an existing label or value.
func (s *MultiSelectState) Choices() []MultiSelectChoice {
	query := strings.TrimSpace(s.Query.String())
	if s.prepared && s.built == s.revision && query == s.lastQuery && s.AllowCreate == s.lastAllow && s.MatchCaseInsensitive == s.lastCase && s.Limit == s.lastLimit {
		return s.choices
	}
	if query != s.lastQuery && query != "" {
		s.Open = true
	}
	s.choices = s.choices[:0]
	exact := false
	for _, option := range s.options {
		label := option.Label
		if label == "" {
			label = option.Value
		}
		exact = exact || strings.EqualFold(query, label) || s.equal(query, option.Value)
		if query != "" && !strings.Contains(strings.ToLower(label), strings.ToLower(query)) {
			continue
		}
		option.Label = label
		selected := s.contains(option.Value)
		option.Disabled = option.Disabled || !selected && s.Limit > 0 && len(s.values) >= s.Limit
		s.choices = append(s.choices, MultiSelectChoice{MultiSelectOption: option, Selected: selected})
	}
	for _, value := range s.values {
		exact = exact || s.equal(query, value)
		if slices.ContainsFunc(s.options, func(o MultiSelectOption) bool { return s.equal(o.Value, value) }) {
			continue
		}
		if query == "" || strings.Contains(strings.ToLower(value), strings.ToLower(query)) {
			s.choices = append(s.choices, MultiSelectChoice{MultiSelectOption: MultiSelectOption{Value: value, Label: value}, Selected: true})
		}
	}
	if s.AllowCreate && query != "" && !exact {
		disabled := s.Limit > 0 && len(s.values) >= s.Limit || s.validateCreate != nil && !s.validateCreate(query)
		s.choices = append(s.choices, MultiSelectChoice{})
		copy(s.choices[1:], s.choices[:len(s.choices)-1])
		s.choices[0] = MultiSelectChoice{MultiSelectOption: MultiSelectOption{Value: query, Label: query, Disabled: disabled}, Create: true}
	}
	s.lastQuery, s.built, s.prepared = query, s.revision, true
	s.lastAllow, s.lastCase, s.lastLimit = s.AllowCreate, s.MatchCaseInsensitive, s.Limit
	s.Active = 0
	for s.Active < len(s.choices)-1 && s.choices[s.Active].Disabled {
		s.Active++
	}
	s.scroll.Offset = 0
	s.reveal = true
	// A rebuilt list is a new set of rows: let the pointer adopt the row it is
	// over again, or a pointer parked on a row would lose it to the reset above.
	s.hover = pointerRow{}
	return s.choices
}

// Choose toggles a row while keeping the popup open for further selections.
func (s *MultiSelectState) Choose(index int) bool {
	choices := s.Choices()
	if index < 0 || index >= len(choices) || choices[index].Disabled {
		return false
	}
	choice := choices[index]
	if choice.Selected {
		s.values = slices.DeleteFunc(s.values, func(value string) bool { return s.equal(value, choice.Value) })
	} else {
		s.values = append(s.values, choice.Value)
	}
	s.Query = text.Editor{}
	s.Open = true
	s.revision++
	s.focusInput = true
	return true
}

func (s *MultiSelectState) Remove(index int) bool {
	if index < 0 || index >= len(s.values) {
		return false
	}
	s.values = slices.Delete(s.values, index, index+1)
	s.revision++
	s.focusInput = true
	return true
}

// Key keeps keyboard navigation in the editable input. Enter chooses the first
// matching/create row; IME confirmation never creates a value. Tab closes the
// popup without consuming normal focus traversal.
func (s *MultiSelectState) Key(event ui.KeyEvent) ui.EventResult {
	if event.Release || event.Modifiers != 0 && !(event.Key == ui.KeyTab && event.Modifiers == ui.ModifierShift) {
		return ui.EventResult{}
	}
	if _, composing := s.Query.Composition(); composing {
		return ui.EventResult{Handled: event.Key == ui.KeyEnter}
	}
	choices := s.Choices()
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
				return ui.EventResult{Handled: true, Repaint: true}
			}
			if len(choices) > 0 {
				if s.Query.String() != "" && choices[s.Active].Selected {
					s.Query = text.Editor{} // an exact duplicate keeps its selection
				} else {
					s.Choose(s.Active)
				}
			}
		}
		return ui.EventResult{Handled: true, Repaint: true}
	case ui.KeyEscape:
		if s.Open {
			s.Open = false
			return ui.EventResult{Handled: true, Repaint: true}
		}
	case ui.KeyTab:
		if s.Open {
			s.Open = false
			return ui.EventResult{Handled: true, Repaint: true, ContinueTraversal: true}
		}
	case ui.KeyBackspace:
		if s.Query.String() == "" && !event.Repeat && s.Remove(len(s.values)-1) {
			return ui.EventResult{Handled: true, Repaint: true}
		}
	}
	return ui.EventResult{}
}

func (s *MultiSelectState) Wheel(event ui.ScrollEvent) bool { return s.scroll.Wheel(event) }

// Pointer routes a configured popup scrollbar; other presses remain ordinary clicks.
func (s *MultiSelectState) Pointer(event ui.PointerEvent) ui.EventResult {
	if s.scroll.BarID.IsZero() || event.Target != s.scroll.BarID && !s.scroll.Dragging() {
		return ui.EventResult{}
	}
	return ui.EventResult{Handled: true, Repaint: s.scroll.Drag(event)}
}

// MultiSelectOptions supplies stable action IDs and localized presentation.
// OptionIDs and RemoveIDs are indexed by Choices and Values respectively.
type MultiSelectOptions struct {
	InputID, TriggerID, DismissID, Scope         ui.ID
	ScrollID                                     ui.ID
	OptionIDs, RemoveIDs                         ui.Scope
	Label, Placeholder, EmptyLabel, CreatePrefix string
	// Viewport bounds the popup and backdrop independently of the input's
	// ancestor scroll clips. Set it to the window or a narrower overlay region.
	Viewport          ui.Rect
	Layer             uint16
	Disabled, Invalid bool
	MaxRows           int
}

type MultiSelectLayout struct {
	Bounds, Input, Popup ui.Rect
	Hidden               int
}

// PaintMultiSelect paints a compact editable multi-select. Chips that cannot fit
// beside the native query input collapse to +N; every value remains available
// in the popup for deselection. The popup traps focus around the input as well
// as the list, so typing and IME stay native while it is open.
func PaintMultiSelect(canvas *ui.Canvas, theme ui.Theme, bounds ui.Rect, s *MultiSelectState, o MultiSelectOptions) MultiSelectLayout {
	if bounds.Height <= 0 {
		bounds.Height = theme.Metrics.ControlHeight
	}
	layout := MultiSelectLayout{Bounds: bounds}
	if canvas == nil || bounds.Width <= 0 || bounds.Height <= 0 {
		return layout
	}
	choices := s.Choices()
	open := s.Open && !o.Disabled
	floating := o.Viewport.Width > 0 && o.Viewport.Height > 0
	// The popup takes focus into its own scope and the runtime restores the
	// target from before that scope opened, which is not the query input when
	// the list was opened by the trigger instead of a press on the input. Hand
	// focus back when the input is the one losing it, and only then: a press on
	// another control has already moved focus and keeps it.
	if s.openPainted && !open && canvas.Focused() == o.InputID {
		canvas.RequestFocus(o.InputID)
	}
	s.openPainted = open
	if open {
		canvas.PushLayer(o.Layer)
		canvas.PushFocusScope(o.Scope)
		defer canvas.PopFocusScope()
		defer canvas.PopLayer()
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
	paintTextEntrySurface(canvas, theme, bounds, textEntryState{Focused: canvas.Focused() == o.InputID || open, Invalid: o.Invalid, Disabled: o.Disabled})
	canvas.HitWithOptions(o.TriggerID, bounds, o.Label, ui.HitOptions{Role: ui.RoleButton, Expanded: open, Disabled: o.Disabled, SkipFocus: true})
	trigger := ui.Rect{X: bounds.X + bounds.Width - 28, Y: bounds.Y + (bounds.Height-24)/2, Width: 24, Height: 24}
	x, right := bounds.X+7, trigger.X-6
	for i, value := range s.values {
		label := s.label(value)
		width := min(canvas.TextWidthWeight(label, theme.Metrics.FontSizeS, ui.WeightMedium)+34, max(bounds.Width/2, 32))
		reserve := float32(72)
		if i < len(s.values)-1 {
			reserve += 34
		}
		if x+width+reserve > right {
			layout.Hidden = len(s.values) - i
			break
		}
		chip := ui.Rect{X: x, Y: bounds.Y + (bounds.Height-22)/2, Width: width, Height: 22}
		paintComboboxChip(canvas, theme, chip, ComboboxChip{RemoveID: o.RemoveIDs.At(i), Label: canvas.FitLine(label, theme.Metrics.FontSizeS, ui.WeightMedium, width-30), Value: value, Disabled: o.Disabled})
		x += width + 5
	}
	if layout.Hidden > 0 {
		badge := ui.Rect{X: x, Y: bounds.Y + (bounds.Height-22)/2, Width: 30, Height: 22}
		canvas.Rounded(badge, theme.Metrics.RadiusItem(), theme.Muted)
		if s.overflowCount != layout.Hidden {
			s.overflowText = "+" + strconv.Itoa(layout.Hidden)
			s.overflowCount = layout.Hidden
		}
		canvas.LabelCenter(s.overflowText, badge, ui.TextStyle{Size: theme.Metrics.FontSizeS, Color: theme.MutedForeground})
		x += 35
	}
	layout.Input = ui.Rect{X: x, Y: bounds.Y + 1, Width: max(right-x, 1), Height: bounds.Height - 2}
	query := s.Query.String()
	focused := canvas.Focused() == o.InputID
	if !focused || query == "" {
		placeholder := o.Placeholder
		if len(s.values) > 0 {
			placeholder = ""
		}
		inputPaintText(canvas, theme, layout.Input, InputSnapshot{Value: query, Placeholder: placeholder, Disabled: o.Disabled}, false)
	}
	canvas.HitWithOptions(o.InputID, layout.Input, o.Label, ui.HitOptions{Role: ui.RoleCombobox, Value: query, Expanded: open, Disabled: o.Disabled, Cursor: ui.PointerCursorText, TextBounds: layout.Input, TextSize: theme.Metrics.FontSizeM, TextColor: theme.Foreground, NavigationKeys: true})
	paintChevron(canvas, &chevronDownIcon, trigger, theme.MutedForeground)
	if s.focusInput {
		canvas.RequestFocus(o.InputID)
		s.focusInput = false
	}
	if !open {
		return layout
	}
	rows := o.MaxRows
	if rows <= 0 {
		rows = 7
	}
	height := float32(max(1, min(len(choices), rows))*32 + 8)
	layout.Popup = ui.Rect{X: bounds.X, Y: bounds.Y + bounds.Height + 6, Width: bounds.Width, Height: height}
	if o.Viewport.Height > 0 {
		top, bottom := o.Viewport.Y+1, o.Viewport.Y+o.Viewport.Height-1
		if layout.Popup.Y+layout.Popup.Height > bottom {
			// Prefer room above for at least one usable row. Otherwise scroll
			// inside the space left below, even when that is less than a row.
			if above := bounds.Y - 6 - top; above >= min(height, 40) {
				layout.Popup.Height = min(height, above)
				layout.Popup.Y = bounds.Y - layout.Popup.Height - 6
			}
		}
		layout.Popup.Y = max(top, min(layout.Popup.Y, bottom-1))
		layout.Popup.Height = max(1, min(layout.Popup.Height, bottom-layout.Popup.Y))
	}
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
			label = canvas.FitLine(label, theme.Metrics.FontSizeM, ui.WeightRegular, max(row.Width-38, 0))
			canvas.Label(label, ui.Rect{X: row.X + 8, Y: row.Y, Width: max(row.Width-38, 0), Height: row.Height}, ui.TextStyle{Size: theme.Metrics.FontSizeM, Color: color})
			if choice.Selected {
				paintCheck(canvas, ui.Rect{X: row.X + row.Width - 25, Y: row.Y, Width: 16, Height: row.Height}, 14, color)
			}
			canvas.HitWithOptions(id, row, choice.Label, ui.HitOptions{Role: ui.RoleOption, Value: choice.Value, Selected: choice.Selected, Disabled: choice.Disabled, SkipFocus: true})
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
