package ui

import "github.com/vivaui/viva/gfx"

// FocusBind binds the keyboard focus of box to a comparable value in field.
// While *field equals value the box asks for the focus; when the user focuses
// the box, *field is set to value; when the user moves the focus away, the field
// follows the focus if it lands on another element bound to the same field.
//
// The field is the application's: set it to a value to move the focus there.
// The request is edge-triggered -- changing *field is what asks -- and waits
// while the element is not declared (a hidden page, a control not shown yet).
// An element that disappears and comes back asks again if the field still names
// it. Values must be unique among the elements bound to one field; the field
// belongs to one window at a time, so give each window its own. The element
// needs an identity: a control, or a container after Hit.
//
//	a := ui.FocusBind(ui.TextField(c, &m.query, "Search", ""), &m.focus, FocusSearch)
//
// Not inside a Memo, whose copies are not declared. Use FocusedValue to read the
// focus that is committed, rather than the request.
func FocusBind[T comparable](box Box, field *T, value T) Box {
	n := box.n()
	if box.f.memoDepth > 0 {
		panic("ui: FocusBind in a Memo: a copied part is not declared, so its binding would disappear; declare it outside the Memo")
	}
	if field == nil {
		panic("ui: FocusBind needs a non-nil field")
	}
	if n.id.IsZero() {
		panic("ui: FocusBind needs an element with an identity: a control, or a container given Hit before FocusBind")
	}
	p := box.f.page
	if p == nil {
		panic("ui: FocusBind is for a page, not a Frame of the application's")
	}
	if p.focusFields == nil {
		p.focusFields = map[any]any{}
	}
	state, ok := p.focusFields[field].(*focusBindings[T])
	if !ok {
		if p.focusFields[field] != nil {
			panic("ui: FocusBind: one field is bound with two value types")
		}
		state = &focusBindings[T]{field: field}
		p.focusFields[field] = state
	}
	if state.epoch != p.epoch {
		if state.epoch != 0 && state.epoch+1 != p.epoch {
			state.missing = true // the page was not painted for a while; ask again
		}
		state.epoch = p.epoch
		state.bindings = state.bindings[:0]
		p.focusPaint = append(p.focusPaint, state)
	}
	state.bindings = append(state.bindings, focusBinding[T]{id: n.id, value: value})
	return box
}

// FocusedValue returns the value bound to the element that has the keyboard
// focus, or the zero value of T if the focus is on no element bound to field.
// It is the committed focus, not a request waiting for a hidden element; call it
// after the elements of the field were declared in this paint.
func FocusedValue[T comparable](c C, field *T) T {
	var zero T
	if c == nil || c.page == nil || field == nil {
		return zero
	}
	state, ok := c.page.focusFields[field].(*focusBindings[T])
	if !ok || state.epoch != c.page.epoch {
		return zero
	}
	canvas := c.f.canvas
	if canvas == nil {
		return zero
	}
	focused := canvas.Focused()
	for i := range state.bindings {
		if state.bindings[i].id == focused {
			return state.bindings[i].value
		}
	}
	return zero
}

// focusState is a field's bindings without the value type, so the page can
// settle them after a paint.
type focusState interface {
	settle(canvas *gfx.Canvas, focused gfx.ID)
	seen() uint32
}

type focusBinding[T comparable] struct {
	id    gfx.ID
	value T
}

type focusBindings[T comparable] struct {
	field     *T
	last      T
	set       bool
	pending   bool // the desired value is not declared yet
	missing   bool // the desired value's element was not declared since the page last painted
	actual    gfx.ID
	actualSet bool
	epoch     uint32
	bindings  []focusBinding[T]
}

func (s *focusBindings[T]) seen() uint32 { return s.epoch }

// settle moves the focus to the value the application asked for, or writes the
// focus the user moved to back into the field.
func (s *focusBindings[T]) settle(canvas *gfx.Canvas, focused gfx.ID) {
	desired := *s.field
	focusChanged := !s.actualSet || focused != s.actual
	s.actual, s.actualSet = focused, true

	var focusedValue T
	focusedBound := false
	for i := range s.bindings {
		if s.bindings[i].id == focused {
			focusedValue, focusedBound = s.bindings[i].value, true
			break
		}
	}
	desiredID := gfx.ID{}
	desiredBound := false
	for i := range s.bindings {
		if s.bindings[i].value == desired {
			desiredID, desiredBound = s.bindings[i].id, true
			break
		}
	}

	if !s.set || desired != s.last {
		s.last, s.set = desired, true
		s.pending, s.missing = !desiredBound, false
		if desiredBound && focused != desiredID {
			canvas.RequestFocus(desiredID)
		}
		return
	}
	if desiredBound && (s.pending || s.missing) {
		s.pending, s.missing = false, false
		if focused != desiredID {
			canvas.RequestFocus(desiredID)
		}
		return
	}
	if !desiredBound {
		s.missing = true
	}
	if focusChanged && focusedBound && focusedValue != desired {
		*s.field = focusedValue
		s.last, s.set = focusedValue, true
		s.pending, s.missing = false, false
	}
}

// settleFocus applies the focus bindings of the paint that just ended: a request
// the application made, or the focus the user moved back into its field.
func (p *Page) settleFocus() {
	canvas := p.ctx.f.canvas
	if canvas != nil {
		focused := canvas.Focused()
		for _, state := range p.focusPaint {
			state.settle(canvas, focused)
		}
	}
	for field, anyState := range p.focusFields {
		if anyState.(focusState).seen() != p.epoch {
			delete(p.focusFields, field)
		}
	}
}
