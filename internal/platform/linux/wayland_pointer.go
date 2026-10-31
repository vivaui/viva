//go:build linux

package linux

import (
	"runtime"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

const (
	defaultDoubleClickMillis = 400

	btnLeft   uint32 = 0x110
	btnRight  uint32 = 0x111
	btnMiddle uint32 = 0x112
	btnSide   uint32 = 0x113
	btnExtra  uint32 = 0x114
)

var (
	pointerListenerOnce sync.Once
	rawPointerListener  [10]uintptr
	pinnerPointer       runtime.Pinner
)

func initPointerListeners() {
	pointerListenerOnce.Do(func() {
		// 0: enter(serial, surface, sx, sy)
		rawPointerListener[0] = purego.NewCallback(func(data unsafe.Pointer, pointer uintptr, serial uint32, surface uintptr, surfaceX, surfaceY wlFixed) {
			if data == nil {
				return
			}
			app := (*waylandApplication)(data)
			win := app.findWindowBySurface(surface)
			app.pointerFocus = win
			app.pointerSerial = serial
			app.lastSerial = serial
			app.setCursor(app.currentCursorShape)

			if win != nil && win.state != nil {
				x, y := win.pointerCoords(surfaceX, surfaceY)
				win.pointerX, win.pointerY = x, y
				event := platform.PointerEvent{
					Kind:      platform.PointerMove,
					X:         x,
					Y:         y,
					Modifiers: app.modifiers,
				}
				app.dispatchPointerEvent(win, event)
			}
		})

		// 1: leave(serial, surface)
		rawPointerListener[1] = purego.NewCallback(func(data unsafe.Pointer, pointer uintptr, serial uint32, surface uintptr) {
			if data == nil {
				return
			}
			app := (*waylandApplication)(data)
			if app.pointerFocus != nil && app.pointerFocus.state != nil {
				// A press that left the surface can no longer drag the window.
				app.pointerFocus.pendingMove = false
				event := platform.PointerEvent{
					Kind:      platform.PointerExit,
					X:         app.pointerFocus.pointerX,
					Y:         app.pointerFocus.pointerY,
					Modifiers: app.modifiers,
				}
				app.dispatchPointerEvent(app.pointerFocus, event)
			}
			app.pointerFocus = nil
		})

		// 2: motion(time, sx, sy)
		rawPointerListener[2] = purego.NewCallback(func(data unsafe.Pointer, pointer uintptr, timeMs uint32, surfaceX, surfaceY wlFixed) {
			if data == nil {
				return
			}
			app := (*waylandApplication)(data)
			if app.pointerFocus == nil || app.pointerFocus.state == nil {
				return
			}
			win := app.pointerFocus
			x, y := win.pointerCoords(surfaceX, surfaceY)
			win.pointerX, win.pointerY = x, y

			if win.dragPendingMove(x, y) {
				// The compositor takes the pointer for the move, so the
				// application never sees this motion or the release.
				app.fail(win.state.router.pointerExit())
				return
			}

			event := platform.PointerEvent{
				Kind:      platform.PointerMove,
				X:         x,
				Y:         y,
				Modifiers: app.modifiers,
			}
			app.dispatchPointerEvent(win, event)
		})

		// 3: button(serial, time, button, state)
		rawPointerListener[3] = purego.NewCallback(func(data unsafe.Pointer, pointer uintptr, serial uint32, timeMs uint32, button uint32, state uint32) {
			if data == nil {
				return
			}
			app := (*waylandApplication)(data)
			app.pointerSerial = serial
			app.lastSerial = serial
			if app.pointerFocus == nil || app.pointerFocus.state == nil {
				return
			}
			win := app.pointerFocus
			btn := waylandPointerButton(button)
			isDown := (state == 1)
			if btn == ui.PointerButtonPrimary {
				// A press or release starts a fresh interaction. Clearing here
				// also disarms a press whose release was lost with the pointer.
				win.pendingMove = false
			}

			if isDown && btn == ui.PointerButtonPrimary && win.state.config.WindowMode == platform.WindowCustomTitlebar {
				if win.beginTitlebarInteraction(serial, win.pointerX, win.pointerY, timeMs) {
					return
				}
			}

			count := 1
			if win.state.router.clicks != nil && isDown {
				win.state.router.applyClickTiming()
				win.state.router.clicks.Press(time.Duration(timeMs)*time.Millisecond, uint8(btn), win.pointerX, win.pointerY)
				count = win.state.router.clicks.Current()
			}

			kind := platform.PointerUp
			if isDown {
				kind = platform.PointerDown
			}

			event := platform.PointerEvent{
				Kind:       kind,
				Button:     btn,
				X:          win.pointerX,
				Y:          win.pointerY,
				ClickCount: count,
				Modifiers:  app.modifiers,
			}
			app.dispatchPointerEvent(win, event)
		})

		// 4: axis(time, axis, value)
		rawPointerListener[4] = purego.NewCallback(func(data unsafe.Pointer, pointer uintptr, timeMs uint32, axis uint32, value wlFixed) {
			if data == nil {
				return
			}
			app := (*waylandApplication)(data)
			if app.pointerFocus == nil || app.pointerFocus.state == nil {
				return
			}
			// If axis_value120 already provided high-resolution delta for this axis in this frame, skip fallback
			if (axis == 0 && app.scrollV120Axis0) || (axis == 1 && app.scrollV120Axis1) {
				return
			}
			val := float32(wlFixedToFloat64(value)) * 4.8
			if axis == 0 {
				app.pendingScroll.DeltaY += val
			} else if axis == 1 {
				app.pendingScroll.DeltaX += val
			}
			app.scrollPending = true
		})

		// 5: frame()
		rawPointerListener[5] = purego.NewCallback(func(data unsafe.Pointer, pointer uintptr) {
			if data == nil {
				return
			}
			app := (*waylandApplication)(data)
			if app.scrollPending {
				app.scrollPending = false
				app.scrollV120Axis0 = false
				app.scrollV120Axis1 = false
				if app.pointerFocus != nil && app.pointerFocus.state != nil {
					scroll := app.pendingScroll
					app.pendingScroll = platform.ScrollEvent{}
					scroll.X = app.pointerFocus.pointerX
					scroll.Y = app.pointerFocus.pointerY
					scroll.Modifiers = app.modifiers
					app.dispatchScrollEvent(app.pointerFocus, scroll)
				} else {
					app.pendingScroll = platform.ScrollEvent{}
				}
			}
		})

		// 6: axis_source(axis_source)
		rawPointerListener[6] = purego.NewCallback(func(data unsafe.Pointer, pointer uintptr, axisSource uint32) {
		})

		// 7: axis_stop(time, axis)
		rawPointerListener[7] = purego.NewCallback(func(data unsafe.Pointer, pointer uintptr, timeMs uint32, axis uint32) {
		})

		// 8: axis_discrete(axis, discrete)
		rawPointerListener[8] = purego.NewCallback(func(data unsafe.Pointer, pointer uintptr, axis uint32, discrete int32) {
		})

		// 9: axis_value120(axis, value120)
		rawPointerListener[9] = purego.NewCallback(func(data unsafe.Pointer, pointer uintptr, axis uint32, value120 int32) {
			if data == nil {
				return
			}
			app := (*waylandApplication)(data)
			if app.pointerFocus == nil || app.pointerFocus.state == nil {
				return
			}
			val := float32(value120) / 120.0 * 48.0
			if axis == 0 {
				app.scrollV120Axis0 = true
				app.pendingScroll.DeltaY += val
			} else if axis == 1 {
				app.scrollV120Axis1 = true
				app.pendingScroll.DeltaX += val
			}
			app.scrollPending = true
		})

		pinnerPointer.Pin(&rawPointerListener)
	})
}

func (app *waylandApplication) dispatchPointerEvent(win *waylandWindow, event platform.PointerEvent) {
	if win == nil || win.state == nil {
		return
	}
	app.fail(win.state.router.dispatchPointer(event))
}

func (app *waylandApplication) dispatchScrollEvent(win *waylandWindow, event platform.ScrollEvent) {
	if win == nil || win.state == nil {
		return
	}
	app.fail(win.state.router.dispatchScroll(event))
}

func (app *waylandApplication) setupPointer() {
	if app.pointer != 0 || app.globals.seat == 0 || app.api == nil {
		return
	}
	pointer := app.api.seatGetPointer(app.globals.seat)
	if pointer == 0 {
		return
	}
	app.pointer = pointer
	initPointerListeners()
	app.api.proxyAddListener(pointer, uintptr(unsafe.Pointer(&rawPointerListener[0])), unsafe.Pointer(app))
}

func (app *waylandApplication) teardownPointer() {
	if app.pointer != 0 && app.api != nil {
		app.api.pointerRelease(app.pointer)
		app.pointer = 0
		if app.pointerFocus != nil {
			// No release will arrive for a press on a pointer that is gone.
			app.pointerFocus.pendingMove = false
		}
		app.pointerFocus = nil
	}
}

// waylandPointerButton maps Linux input event button codes to Viva PointerButton.
func waylandPointerButton(button uint32) ui.PointerButton {
	switch button {
	case btnLeft:
		return ui.PointerButtonPrimary
	case btnRight:
		return ui.PointerButtonSecondary
	case btnMiddle:
		return ui.PointerButtonMiddle
	case btnSide:
		return ui.PointerButtonBack
	case btnExtra:
		return ui.PointerButtonForward
	default:
		return ui.PointerButtonNone
	}
}

func (win *waylandWindow) pointerCoords(surfaceX, surfaceY wlFixed) (float32, float32) {
	scale := win.displayScale()
	logicalMargin := float32(0)
	if scale > 0 {
		logicalMargin = float32(win.margin) / scale
	}
	logicalX := float32(wlFixedToFloat64(surfaceX)) - logicalMargin
	logicalY := float32(wlFixedToFloat64(surfaceY)) - logicalMargin
	return logicalX, logicalY
}

func (win *waylandWindow) checkDoubleClick(timeMs uint32) bool {
	if win.lastClickTime > 0 && timeMs >= win.lastClickTime && (timeMs-win.lastClickTime) <= defaultDoubleClickMillis {
		win.lastClickTime = 0
		return true
	}
	win.lastClickTime = timeMs
	return false
}

// dragPendingMove starts the move a PressMoveOnDrag press asked for once the
// pointer has moved past the drag slop. x and y are logical content
// coordinates, so the slop needs no display scaling.
func (win *waylandWindow) dragPendingMove(x, y float32) bool {
	if !win.pendingMove {
		return false
	}
	dx, dy := x-win.pendingMoveX, y-win.pendingMoveY
	if dx > -defaultDragSlop && dx < defaultDragSlop && dy > -defaultDragSlop && dy < defaultDragSlop {
		return false
	}
	win.pendingMove = false
	win.startMove(win.pendingMoveSerial)
	return true
}

func (win *waylandWindow) beginTitlebarInteraction(serial uint32, x, y float32, timeMs uint32) bool {
	if win.state == nil || win.state.config.WindowMode != platform.WindowCustomTitlebar {
		return false
	}
	scale := win.displayScale()
	contentW := float32(contentSize(win.width, win.margin)) / scale
	contentH := float32(contentSize(win.height, win.margin)) / scale

	inMargin := (x < 0 || y < 0 || x >= contentW || y >= contentH)

	// 1. Edge and corner resizing for non-maximized windows (outer grip or inner border)
	if !win.maximized && !win.fullscreen {
		overControl := false
		if !inMargin && win.state.config.PressAt != nil {
			overControl = (win.state.config.PressAt(x, y) == platform.PressControl)
		}
		edge := platform.EdgeAt(x, y, contentW, contentH, overControl)
		if edge != platform.EdgeNone {
			win.startResize(serial, edge)
			return true
		}
	}

	// In margin, never trigger move or pass through to client
	if inMargin {
		return true
	}

	// 3. Check application-defined titlebar dragging
	if win.state.config.PressAt != nil {
		press := win.state.config.PressAt(x, y)
		if press == platform.PressControl || press == platform.PressClient {
			return false
		}
		if press == platform.PressMove || press == platform.PressMoveOnDrag {
			if win.checkDoubleClick(timeMs) {
				win.toggleMaximize()
				return true
			}
			if press == platform.PressMoveOnDrag {
				// The press belongs to the application: it may dismiss the
				// backdrop. Remember it so motion past the slop can still move
				// the window with this press's serial.
				win.pendingMove = true
				win.pendingMoveSerial = serial
				win.pendingMoveX, win.pendingMoveY = x, y
				return false
			}
			win.startMove(serial)
			return true
		}
	}

	// 4. Fallback default titlebar height
	tbHeight := win.state.config.TitlebarHeight
	if tbHeight <= 0 {
		tbHeight = 32
	}
	if y >= 0 && y < tbHeight && x >= 0 && x < contentW {
		if win.checkDoubleClick(timeMs) {
			win.toggleMaximize()
			return true
		}
		win.startMove(serial)
		return true
	}

	return false
}
