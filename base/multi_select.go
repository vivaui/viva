package base

import (
	"slices"
	"strings"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
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
	hover                                 PointerRow
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
	s.hover = PointerRow{}
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
func (s *MultiSelectState) Key(event gfx.KeyEvent) gfx.EventResult {
	if event.Release || event.Modifiers != 0 && !(event.Key == gfx.KeyTab && event.Modifiers == gfx.ModifierShift) {
		return gfx.EventResult{}
	}
	if _, composing := s.Query.Composition(); composing {
		return gfx.EventResult{Handled: event.Key == gfx.KeyEnter}
	}
	choices := s.Choices()
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
		s.reveal = true
		return gfx.EventResult{Handled: true, Repaint: true}
	case gfx.KeyEnter:
		if !event.Repeat {
			if !s.Open && s.Query.String() == "" {
				s.Open = true
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			if len(choices) > 0 {
				if s.Query.String() != "" && choices[s.Active].Selected {
					s.Query = text.Editor{} // an exact duplicate keeps its selection
				} else {
					s.Choose(s.Active)
				}
			}
		}
		return gfx.EventResult{Handled: true, Repaint: true}
	case gfx.KeyEscape:
		if s.Open {
			s.Open = false
			return gfx.EventResult{Handled: true, Repaint: true}
		}
	case gfx.KeyTab:
		if s.Open {
			s.Open = false
			return gfx.EventResult{Handled: true, Repaint: true, ContinueTraversal: true}
		}
	case gfx.KeyBackspace:
		if s.Query.String() == "" && !event.Repeat && s.Remove(len(s.values)-1) {
			return gfx.EventResult{Handled: true, Repaint: true}
		}
	}
	return gfx.EventResult{}
}

func (s *MultiSelectState) Wheel(event gfx.ScrollEvent) bool { return s.scroll.Wheel(event) }

// Pointer routes a configured popup scrollbar; other presses remain ordinary clicks.
func (s *MultiSelectState) Pointer(event gfx.PointerEvent) gfx.EventResult {
	if s.scroll.BarID.IsZero() || event.Target != s.scroll.BarID && !s.scroll.Dragging() {
		return gfx.EventResult{}
	}
	return gfx.EventResult{Handled: true, Repaint: s.scroll.Drag(event)}
}

// The fields below are the painting layer's bridge to the state: the component
// that paints the trigger, chips and popup reads and writes them, while the page
// and the application use the state's methods and its exported fields.

// ValueLabel is the label of value, as the trigger and the popup show it.
func (s *MultiSelectState) ValueLabel(value string) string { return s.label(value) }

// ScrollArea is the open popup's scrollbar and row window.
func (s *MultiSelectState) ScrollArea() *MenuScroll { return &s.scroll }

// OpenPainted reports whether the popup was open in the paint before this one.
func (s *MultiSelectState) OpenPainted() bool { return s.openPainted }

// SetOpenPainted records whether this paint left the popup open.
func (s *MultiSelectState) SetOpenPainted(open bool) { s.openPainted = open }

// FocusInput reports whether the query input takes the focus as it paints.
func (s *MultiSelectState) FocusInput() bool { return s.focusInput }

// SetFocusInput makes the next paint hand the focus to the query input.
func (s *MultiSelectState) SetFocusInput(focus bool) { s.focusInput = focus }

// AdoptHover moves the active row to the one the pointer crossed onto, and
// reports whether it did.
func (s *MultiSelectState) AdoptHover(hovered gfx.ID) bool { return s.hover.Entered(hovered) }

// Overflow is the +N text and the number of values it hides.
func (s *MultiSelectState) Overflow() (text string, hidden int) {
	return s.overflowText, s.overflowCount
}

// SetOverflow records the +N text and the number of values it hides.
func (s *MultiSelectState) SetOverflow(text string, hidden int) {
	s.overflowText, s.overflowCount = text, hidden
}

// RevealActive reports whether the active row is scrolled into sight next paint.
func (s *MultiSelectState) RevealActive() bool { return s.reveal }

// SetRevealActive asks the next paint to scroll the active row into sight.
func (s *MultiSelectState) SetRevealActive(reveal bool) { s.reveal = reveal }

// CreateText is the prefixed label of the create row, and the prefix and query
// it was built from.
func (s *MultiSelectState) CreateText() (text, prefix, value string) {
	return s.createText, s.createPrefix, s.createValue
}

// SetCreateText records the prefixed label of the create row and its parts.
func (s *MultiSelectState) SetCreateText(text, prefix, value string) {
	s.createText, s.createPrefix, s.createValue = text, prefix, value
}
