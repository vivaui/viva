package ui

import (
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

// Handlers is how a control of the application's own takes the window's input: a
// terminal, an editor, a canvas the user draws on. The page gives each event whose
// target is the control to it while it is declared; a nil function leaves that
// kind of event to the page as for any element. Keep one in the model, next to the
// control's state: the page holds the pointer for a paint, and functions made in
// each paint would allocate.
//
// A press on the control focuses it, as on any element, so its keys follow. Return Handled for
// what the control acted on; a key it leaves unhandled goes on to the page (Tab
// moves the focus on, Escape closes the overlay it is in).
type Handlers struct {
	// Role is what assistive technology and the platform's text service take the
	// control for. Zero is a group. RoleTextField or RoleTerminal make it a target
	// of typing, given to Text.
	Role gfx.Role
	// Key receives the keys pressed while the control has the focus.
	Key func(gfx.KeyEvent) gfx.EventResult
	// Pointer receives the presses, moves and releases over the control, and the
	// moves and release of a press it took while the pointer is held.
	Pointer func(gfx.PointerEvent) gfx.EventResult
	// Scroll receives the wheel over the control; what it leaves unhandled scrolls
	// what the control is in.
	Scroll func(gfx.ScrollEvent) gfx.EventResult
	// Text gives the platform's text service the control's editor while it has the
	// focus: typing, input methods composing at the caret (TextInput.Bounds places
	// their window) and the clipboard. It is asked only for a control whose Role is
	// RoleTextField or RoleTerminal. What it leaves unset is the control's: its
	// rectangle, and the page's body text size and color.
	Text func(app.TextInputContext) *app.TextInput
}

// controlEntry is a Control declared in the last paint.
type controlEntry struct {
	id       gfx.ID
	handlers *Handlers
}

// Control declares an element that takes its input through h: a control of the
// application's own, named by its place (or the Key or Part before it) like any
// element, and focusable. It is an empty box: give it a size or let it grow, and
// paint it with Draw, where DrawContext.State says whether it has the focus.
//
//	ui.Control(c, "Terminal", &m.term.input).Grow(1).FillWidth().Draw(m.term.draw)
//
// Not inside a Memo: a copied part is not declared, and its control would take no input.
func Control(c C, label string, h *Handlers) Box {
	p := c.page
	p.forbidInMemo("Control")
	if h == nil {
		panic("ui: Control needs its Handlers")
	}
	id := p.controlID(pkControl)
	role := h.Role
	if role == 0 {
		role = gfx.RoleGroup
	}
	p.controls = append(p.controls, controlEntry{id: id, handlers: h})
	if p.pointerRoute == nil {
		p.keyRoute, p.pointerRoute = (*Page).routeKey, (*Page).routePointer
	}
	theme := &c.f.theme
	// What the text service is given when Text leaves it unset: the page's body text.
	return c.f.Slot().HitWith(id, label, gfx.HitOptions{Role: role,
		TextSize: theme.Metrics.FontSizeM, TextColor: theme.Foreground})
}

// control returns the Handlers of the control named id in the last paint, or nil.
func (p *Page) control(id gfx.ID) *Handlers {
	if id.IsZero() {
		return nil
	}
	for i := range p.controls {
		if p.controls[i].id == id {
			return p.controls[i].handlers
		}
	}
	return nil
}
