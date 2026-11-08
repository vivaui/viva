package interaction

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

var (
	trigger   = ui.Item(1)
	elsewhere = ui.Item(2)
	popScope  = ui.Item(10)
	inPopover = ui.Item(11)
)

// paintPopover paints the page's two buttons and, when open, a non-modal popover
// with one button that gives the focus back as it closes.
func paintPopover(canvas *ui.Canvas, open bool) {
	canvas.Reset()
	canvas.BeginFrame()
	button := ui.HitOptions{Role: ui.RoleButton}
	canvas.HitWithOptions(trigger, ui.Rect{Width: 40, Height: 20}, "", button)
	canvas.HitWithOptions(elsewhere, ui.Rect{Y: 30, Width: 40, Height: 20}, "", button)
	if open {
		canvas.PushLayer(2)
		canvas.PushFocusScope(popScope)
		canvas.ReturnFocus(popScope)
		canvas.HitWithOptions(inPopover, ui.Rect{X: 60, Width: 40, Height: 20}, "", button)
		canvas.PopFocusScope()
		canvas.PopLayer()
	}
	canvas.EndFrame()
}

func TestReturnFocusGivesTheFocusBackToTheTrigger(t *testing.T) {
	var canvas ui.Canvas
	var focus Focus
	paintPopover(&canvas, false)
	focus.Sync(&canvas)
	focus.Apply(ui.EventResult{Focus: trigger})
	paintPopover(&canvas, true)
	focus.Sync(&canvas)
	focus.Apply(ui.EventResult{Focus: inPopover})
	paintPopover(&canvas, true)
	focus.Sync(&canvas)
	paintPopover(&canvas, false)
	focus.Sync(&canvas)
	if focus.Focused() != trigger {
		t.Fatalf("focus after closing = %v, want the trigger", focus.Focused())
	}
}

func TestReturnFocusLeavesFocusTheUserMovedOutside(t *testing.T) {
	var canvas ui.Canvas
	var focus Focus
	paintPopover(&canvas, false)
	focus.Sync(&canvas)
	focus.Apply(ui.EventResult{Focus: trigger})
	paintPopover(&canvas, true)
	focus.Sync(&canvas)
	focus.Apply(ui.EventResult{Focus: elsewhere}) // a press outside that went to another control
	paintPopover(&canvas, true)
	focus.Sync(&canvas)
	paintPopover(&canvas, false)
	focus.Sync(&canvas)
	if focus.Focused() != elsewhere {
		t.Fatalf("focus after closing = %v, want the control the user chose", focus.Focused())
	}
}

func TestReleaseFocusClearsOnlyTheNamedTarget(t *testing.T) {
	var canvas ui.Canvas
	var focus Focus
	paintPopover(&canvas, false)
	focus.Sync(&canvas)
	focus.Apply(ui.EventResult{Focus: trigger})

	paintPopover(&canvas, false)
	canvas.ReleaseFocus(elsewhere)
	focus.Sync(&canvas)
	if focus.Focused() != trigger {
		t.Fatalf("releasing another target moved the focus to %v", focus.Focused())
	}
	paintPopover(&canvas, false)
	canvas.ReleaseFocus(trigger)
	focus.Sync(&canvas)
	if !focus.Focused().IsZero() {
		t.Fatalf("focus = %v, want none after releasing it", focus.Focused())
	}
	paintPopover(&canvas, false)
	focus.Sync(&canvas)
	focus.Apply(ui.EventResult{Focus: trigger})
	paintPopover(&canvas, false) // the request is gone with the paint that made it
	focus.Sync(&canvas)
	if focus.Focused() != trigger {
		t.Fatalf("a later paint released the focus again: %v", focus.Focused())
	}
}
