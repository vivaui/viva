//go:build android

package android

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
)

// MotionEvent's actions, tools, buttons and meta state, as the host sends them.
const (
	actionDown        = 0
	actionUp          = 1
	actionMove        = 2
	actionCancel      = 3
	actionPointerDown = 5
	actionPointerUp   = 6

	toolFinger = 1
	toolStylus = 2
	toolMouse  = 3
	toolEraser = 4

	buttonSecondary = 2
	buttonTertiary  = 4

	metaShift = 0x1
	metaAlt   = 0x2
	metaCtrl  = 0x1000
	metaMeta  = 0x10000
)

// pointerKind is what an Android action is to the application, false for one the
// application has no use for.
func pointerKind(action int32) (platform.PointerEventKind, bool) {
	switch action {
	case actionDown, actionPointerDown:
		return platform.PointerDown, true
	case actionMove:
		return platform.PointerMove, true
	case actionUp, actionPointerUp:
		return platform.PointerUp, true
	case actionCancel:
		return platform.PointerCancel, true
	}
	return 0, false
}

func pointerDevice(tool int32) gfx.PointerDevice {
	switch tool {
	case toolMouse:
		return gfx.PointerMouse
	case toolStylus, toolEraser:
		return gfx.PointerPen
	}
	return gfx.PointerTouch
}

// pointerButton is the button a press of the tool is: a finger and a pen's tip press the
// primary one, a mouse the one it holds.
func pointerButton(tool, buttons int32) gfx.PointerButton {
	if tool == toolMouse {
		switch {
		case buttons&buttonSecondary != 0:
			return gfx.PointerButtonSecondary
		case buttons&buttonTertiary != 0:
			return gfx.PointerButtonMiddle
		}
	}
	return gfx.PointerButtonPrimary
}

func modifiers(meta int32) input.Modifiers {
	var result input.Modifiers
	if meta&metaShift != 0 {
		result |= input.ModifierShift
	}
	if meta&metaCtrl != 0 {
		result |= input.ModifierControl
	}
	if meta&metaAlt != 0 {
		result |= input.ModifierAlt
	}
	if meta&metaMeta != 0 {
		result |= input.ModifierMeta
	}
	return result
}

// touch gives the application one finger's event, and shows what it answers with.
func (application *application) touch(ev event) {
	win := application.window
	if win == nil || win.closed || win.config.OnPointer == nil || win.renderer == nil || win.native == nil || win.density <= 0 {
		return
	}
	kind, ok := pointerKind(ev.action)
	if !ok {
		return
	}
	defer application.guard()
	// The surface fills the window and starts at the screen's corner: the surface's
	// pixels over the density are the window's and the screen's logical ones.
	x, y := ev.x/win.density, ev.y/win.density
	event := platform.PointerEvent{
		Kind: kind, Modifiers: modifiers(ev.modifiers), X: x, Y: y, ScreenX: x, ScreenY: y,
		Device: pointerDevice(ev.tool), PointerID: ev.pointer,
	}
	if kind == platform.PointerDown || kind == platform.PointerUp {
		event.Button = pointerButton(ev.tool, ev.buttons)
		event.ClickCount = 1
	}
	frame, err := win.config.OnPointer(event)
	if err == nil {
		err = win.present(frame)
	}
	if err != nil {
		application.fail(err)
	}
}

// hover gives the application a mouse over the surface, as a pointer that moves with no
// button, and its leaving.
func (application *application) hover(ev event) {
	win := application.window
	if win == nil || win.closed || win.config.OnPointer == nil || win.renderer == nil || win.native == nil || win.density <= 0 {
		return
	}
	defer application.guard()
	event := platform.PointerEvent{Kind: platform.PointerMove, Device: gfx.PointerMouse}
	if ev.kind == eventHoverExit {
		if !win.hovering {
			return
		}
		win.hovering = false
		event.Kind = platform.PointerExit
	} else {
		win.hovering = true
		x, y := ev.x/win.density, ev.y/win.density
		event.X, event.Y, event.ScreenX, event.ScreenY, event.Modifiers = x, y, x, y, modifiers(ev.modifiers)
	}
	frame, err := win.config.OnPointer(event)
	if err == nil {
		err = win.present(frame)
	}
	if err != nil {
		application.fail(err)
	}
}

// scroll gives the application a wheel. Android reports a wheel up as positive; a scroll
// that moves the content up is a positive distance here, as on the other platforms.
func (application *application) scroll(ev event) {
	win := application.window
	if win == nil || win.closed || win.config.OnScroll == nil || win.renderer == nil || win.native == nil || win.density <= 0 {
		return
	}
	defer application.guard()
	x, y := ev.x/win.density, ev.y/win.density
	frame, err := win.config.OnScroll(platform.ScrollEvent{
		Modifiers: modifiers(ev.modifiers), X: x, Y: y, DeltaX: ev.dx / win.density, DeltaY: -ev.dy / win.density,
	})
	if err == nil {
		err = win.present(frame)
	}
	if err != nil {
		application.fail(err)
	}
}
