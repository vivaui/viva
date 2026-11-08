package interaction

import "github.com/vivaui/viva/ui"

// Pointer is the press state shared by the native and browser painters. A press
// captures moves and releases on the target it began on even when hover moves
// elsewhere; a click completes only when the press is still on the hovered
// target and the application did not consume it.
type Pointer struct {
	hovered      ui.ID
	pressed      ui.ID
	target       ui.ID
	button       ui.PointerButton
	pressHandled bool
}

// SetHovered records the hit under the pointer, or zero when there is none.
func (p *Pointer) SetHovered(target ui.ID) { p.hovered = target }

// Hovered returns the hit under the pointer.
func (p *Pointer) Hovered() ui.ID { return p.hovered }

// Pressed returns the target of the current press, or zero.
func (p *Pointer) Pressed() ui.ID { return p.pressed }

// PressTarget returns the captured target: moves and releases go to it until
// the press ends.
func (p *Pointer) PressTarget() ui.ID { return p.target }

// Down starts a press on the hovered target.
func (p *Pointer) Down(button ui.PointerButton) {
	p.pressed = p.hovered
	p.target = p.hovered
	p.button = button
	if p.button == ui.PointerButtonNone {
		p.button = ui.PointerButtonPrimary
	}
}

// Delivery says which target an event is offered to and whether the press
// captured it. A press itself goes to the hit under the pointer; later moves and
// releases go to the press target.
func (p *Pointer) Delivery(kind ui.PointerEventKind) (target ui.ID, captured bool) {
	if kind == ui.PointerDown || p.target.IsZero() {
		return p.hovered, false
	}
	return p.target, true
}

// NoteHandled records whether the application consumed an event. A consumed
// press keeps its release from also producing a click.
func (p *Pointer) NoteHandled(kind ui.PointerEventKind, handled bool) {
	if kind == ui.PointerDown {
		p.pressHandled = handled
	} else if !p.target.IsZero() && handled {
		p.pressHandled = true
	}
}

// Click returns the target a release should click, if the press is still on the
// hovered target, is a primary press, and was not consumed.
func (p *Pointer) Click() (ui.ID, bool) {
	click := p.button == ui.PointerButtonPrimary && !p.pressHandled &&
		!p.pressed.IsZero() && p.pressed == p.hovered && p.pressed == p.target
	return p.pressed, click
}

// Up ends the press and keeps hover where it is.
func (p *Pointer) Up() { p.reset() }

// Exit ends the press and clears hover.
func (p *Pointer) Exit() {
	p.reset()
	p.hovered = ui.ID{}
}

func (p *Pointer) reset() {
	p.pressed = ui.ID{}
	p.target = ui.ID{}
	p.button = ui.PointerButtonNone
	p.pressHandled = false
}
