//go:build js && wasm

package web

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/interaction"
)

// touchState carries out what interaction.Touch reads a finger as, as the native
// runtime does (app/touch.go).
type touchState struct {
	interaction.Touch
	id        int32
	modifiers gfx.Modifiers
	advancing bool // a timed step is being carried out: repaint must not start another
}

// touchEvent reads one event of a finger and reports whether rendering state
// changed.
func (p *painter) touchEvent(event gfx.PointerEvent) bool {
	t := &p.touch
	now := p.canvas.Now()
	var step interaction.TouchStep
	switch event.Kind {
	case gfx.PointerDown:
		if !t.Active() || event.PointerID == t.id {
			t.id, t.modifiers = event.PointerID, event.Modifiers
		}
		step = t.Down(event.PointerID, event.X, event.Y, now, p.claimsDrag(event.X, event.Y))
	case gfx.PointerMove:
		step = t.Move(event.PointerID, event.X, event.Y, now)
	case gfx.PointerUp:
		step = t.Up(event.PointerID, event.X, event.Y, now)
	case gfx.PointerExit:
		step = t.Cancel()
	}
	return p.carryTouch(step)
}

// claimsDrag reports whether what is under a finger takes a moving finger as a
// drag rather than letting it scroll: a slider, a split view's divider.
func (p *painter) claimsDrag(x, y float32) bool {
	region, ok, _ := p.focus.Router().PointerAt(x, y, false)
	return ok && (region.Role == gfx.RoleSlider || region.Role == gfx.RoleSeparator)
}

// carryTouch carries out a step in the order TouchStep gives, as the presses of
// a mouse at the finger, and asks for the frame the step wants.
func (p *painter) carryTouch(step interaction.TouchStep) bool {
	t := &p.touch
	event := gfx.PointerEvent{Button: gfx.PointerButtonPrimary, Modifiers: t.modifiers, X: step.X, Y: step.Y, Device: gfx.PointerTouch, PointerID: t.id}
	changed := false
	send := func(kind gfx.PointerEventKind, button gfx.PointerButton, clicks int) {
		event.Kind, event.Button, event.ClickCount = kind, button, clicks
		changed = p.pointerInput(event) || changed
	}
	if step.Cancel {
		changed = p.endPress(event) || changed
	}
	if step.Press {
		send(gfx.PointerDown, gfx.PointerButtonPrimary, step.Clicks)
	}
	if step.Move {
		send(gfx.PointerMove, gfx.PointerButtonNone, 0)
	}
	if step.Release {
		send(gfx.PointerUp, gfx.PointerButtonPrimary, step.Clicks)
	}
	if step.Secondary {
		send(gfx.PointerDown, gfx.PointerButtonSecondary, 1)
		send(gfx.PointerUp, gfx.PointerButtonSecondary, 1)
	}
	if step.Lift {
		send(gfx.PointerExit, gfx.PointerButtonNone, 0)
	}
	if step.Scrolls() {
		_, moved := p.scrollBy(gfx.ScrollEvent{Modifiers: t.modifiers, X: step.ScrollAtX, Y: step.ScrollAtY, DeltaX: step.ScrollX, DeltaY: step.ScrollY, Precise: true})
		if !moved {
			t.FlingBlocked() // at the end of what it scrolls
			step.Wake = false
		} else if !t.advancing {
			p.repaint() // a fling's step is painted by the repaint it runs in
		}
		changed = changed || moved
	}
	if step.Wake && p.scheduleFrame != nil {
		p.scheduleFrame(step.WakeAfter)
	}
	return changed
}

// advanceTouch carries out what a finger's time brings, at the start of a
// repaint: a still finger's press, its secondary press, the next step of a fling.
func (p *painter) advanceTouch() {
	t := &p.touch
	if t.advancing || !t.Active() {
		return
	}
	t.advancing = true
	defer func() { t.advancing = false }()
	p.carryTouch(t.Advance(p.canvas.Now()))
}
