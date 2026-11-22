package app

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/interaction"
	"github.com/vivaui/viva/internal/platform"
)

// touchRuntime is how the painter reads fingers. The platforms that report them
// install it (touch_install.go); where none does, macOS, it stays nil and none of
// the code is linked: a finger's event there is read as a mouse's.
var touchRuntime struct {
	event   func(*painter, platform.PointerEvent) (platform.Frame, error)
	advance func(*painter) (platform.Frame, bool, error)
}

// installTouch makes the painter read fingers.
func installTouch() {
	touchRuntime.event = (*painter).touchEvent
	touchRuntime.advance = (*painter).advanceTouch
}

// touchState carries out what interaction.Touch reads a finger as.
type touchState struct {
	interaction.Touch
	id                 int32
	modifiers          input.Modifiers
	screenDX, screenDY float32 // screen position less window position
	advancing          bool    // a timed step is being carried out: repaint must not start another
}

// touchEvent reads one event of a finger.
func (painter *painter) touchEvent(event platform.PointerEvent) (platform.Frame, error) {
	t := &painter.touch
	now := painter.canvas.Now()
	var step interaction.TouchStep
	switch event.Kind {
	case platform.PointerDown:
		if !t.Active() || event.PointerID == t.id {
			t.id, t.modifiers = event.PointerID, event.Modifiers
			t.screenDX, t.screenDY = event.ScreenX-event.X, event.ScreenY-event.Y
		}
		step = t.Down(event.PointerID, event.X, event.Y, now, painter.claimsDrag(event.X, event.Y))
	case platform.PointerMove:
		step = t.Move(event.PointerID, event.X, event.Y, now)
	case platform.PointerUp:
		step = t.Up(event.PointerID, event.X, event.Y, now)
	case platform.PointerCancel, platform.PointerExit:
		step = t.Cancel()
	}
	return painter.carryTouch(step)
}

// claimsDrag reports whether what is under a finger takes a moving finger as a
// drag rather than letting it scroll: a slider, a split view's divider.
func (painter *painter) claimsDrag(x, y float32) bool {
	region, ok, _ := painter.focus.Router().PointerAt(x, y, false)
	return ok && (region.Role == gfx.RoleSlider || region.Role == gfx.RoleSeparator)
}

// carryTouch carries out a step in the order TouchStep gives, as the presses of
// a mouse at the finger, and asks for the frame the step wants.
func (painter *painter) carryTouch(step interaction.TouchStep) (platform.Frame, error) {
	t := &painter.touch
	event := platform.PointerEvent{
		Button: gfx.PointerButtonPrimary, Modifiers: t.modifiers, X: step.X, Y: step.Y,
		ScreenX: step.X + t.screenDX, ScreenY: step.Y + t.screenDY, Device: gfx.PointerTouch, PointerID: t.id,
	}
	var frame platform.Frame
	var err error
	send := func(kind platform.PointerEventKind, button gfx.PointerButton, clicks int) {
		if err != nil {
			return
		}
		event.Kind, event.Button, event.ClickCount = kind, button, clicks
		frame, err = painter.pointerInput(event)
	}
	if step.Cancel {
		send(platform.PointerCancel, gfx.PointerButtonPrimary, 0)
	}
	if step.Press {
		send(platform.PointerDown, gfx.PointerButtonPrimary, step.Clicks)
	}
	if step.Move {
		send(platform.PointerMove, gfx.PointerButtonNone, 0)
	}
	if step.Release {
		send(platform.PointerUp, gfx.PointerButtonPrimary, step.Clicks)
	}
	if step.Secondary {
		send(platform.PointerDown, gfx.PointerButtonSecondary, 1)
		send(platform.PointerUp, gfx.PointerButtonSecondary, 1)
	}
	if step.Lift {
		send(platform.PointerExit, gfx.PointerButtonNone, 0)
	}
	if err != nil {
		return platform.Frame{}, err
	}
	if step.Scrolls() {
		moved := painter.applyScroll(platform.ScrollEvent{X: step.ScrollAtX, Y: step.ScrollAtY, DeltaX: step.ScrollX, DeltaY: step.ScrollY, Precise: true})
		if !moved {
			t.FlingBlocked() // at the end of what it scrolls
			step.Wake = false
		} else if !t.advancing {
			frame, err = painter.repaintAfterInput()
		}
	}
	if step.Wake && painter.scheduleFrame != nil {
		painter.scheduleFrame(step.WakeAfter)
	}
	return frame, err
}

// advanceTouch carries out what a finger's time brings, at the start of a
// repaint: a still finger's press, its secondary press, the next step of a
// fling. It reports a frame when the step repainted itself; a fling's step is
// painted by the repaint it runs in.
func (painter *painter) advanceTouch() (platform.Frame, bool, error) {
	t := &painter.touch
	if t.advancing || !t.Active() {
		return platform.Frame{}, false, nil
	}
	t.advancing = true
	defer func() { t.advancing = false }()
	step := t.Advance(painter.canvas.Now())
	if step.Cancel || step.Press || step.Secondary {
		frame, err := painter.carryTouch(step)
		return frame, true, err
	}
	_, err := painter.carryTouch(step)
	return platform.Frame{}, err != nil, err
}
