//go:build linux

package linux

import (
	"errors"
	"testing"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

func TestWaylandPointerCoordinates(t *testing.T) {
	// Scale 1.0, margin 24
	win1 := &waylandWindow{margin: 24}
	win1.storeScale(1.0)

	// Origin of content box: devX = 24, devY = 24 -> (0, 0)
	x, y := win1.pointerCoords(float64ToWlFixed(24), float64ToWlFixed(24))
	if x != 0 || y != 0 {
		t.Errorf("expected (0, 0), got (%g, %g)", x, y)
	}

	// Inside content box: devX = 124, devY = 74 -> (100, 50)
	x, y = win1.pointerCoords(float64ToWlFixed(124), float64ToWlFixed(74))
	if x != 100 || y != 50 {
		t.Errorf("expected (100, 50), got (%g, %g)", x, y)
	}

	// Scale 2.0, margin 48 (logical margin = 24)
	win2 := &waylandWindow{margin: 48}
	win2.storeScale(2.0)

	// Origin of content box: surfaceX = 24, surfaceY = 24 -> (0, 0)
	x, y = win2.pointerCoords(float64ToWlFixed(24), float64ToWlFixed(24))
	if x != 0 || y != 0 {
		t.Errorf("expected (0, 0) at 2x, got (%g, %g)", x, y)
	}

	// Inside content box: surfaceX = 124, surfaceY = 74 -> (100, 50)
	x, y = win2.pointerCoords(float64ToWlFixed(124), float64ToWlFixed(74))
	if x != 100 || y != 50 {
		t.Errorf("expected (100, 50) at 2x, got (%g, %g)", x, y)
	}
}

func TestWaylandPointerButtonMapping(t *testing.T) {
	cases := []struct {
		evdev uint32
		want  ui.PointerButton
	}{
		{btnLeft, ui.PointerButtonPrimary},
		{btnRight, ui.PointerButtonSecondary},
		{btnMiddle, ui.PointerButtonMiddle},
		{btnSide, ui.PointerButtonBack},
		{btnExtra, ui.PointerButtonForward},
		{0x999, ui.PointerButtonNone},
	}

	for _, c := range cases {
		got := waylandPointerButton(c.evdev)
		if got != c.want {
			t.Errorf("waylandPointerButton(%x) = %v, want %v", c.evdev, got, c.want)
		}
	}
}

func TestWaylandPointerTitlebarInteraction(t *testing.T) {
	var movedSerial uint32
	var resizedSerial uint32
	var resizedXdgEdge uint32
	var maximizedToggleCount int

	api := &waylandAPI{
		proxyMarshalFlags: func(proxy uintptr, opcode uint32, interfacePtr uintptr, version uint32, flags uint32, args ...any) uintptr {
			if opcode == xdgToplevelMoveOpcode && len(args) >= 2 {
				if s, ok := args[1].(uint32); ok {
					movedSerial = s
				}
			} else if opcode == xdgToplevelResizeOpcode && len(args) >= 3 {
				if s, ok := args[1].(uint32); ok {
					resizedSerial = s
				}
				if e, ok := args[2].(uint32); ok {
					resizedXdgEdge = e
				}
			}
			return 0
		},
	}
	app := &waylandApplication{
		api: api,
		globals: waylandGlobals{
			seat: 99,
		},
	}

	win := &waylandWindow{
		app:         app,
		xdgToplevel: 10,
		width:       848, // 800 + 2*24
		height:      648, // 600 + 2*24
		margin:      24,
	}
	win.storeScale(1.0)

	state := &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			WindowMode:     platform.WindowCustomTitlebar,
			TitlebarHeight: 40,
			PressAt: func(x, y float32) platform.PressKind {
				if y < 40 && x > 700 {
					return platform.PressControl // Close/min/max buttons
				}
				if y < 40 {
					return platform.PressMove
				}
				return platform.PressClient
			},
		},
	}
	win.state = state

	// 1. Press on titlebar move area -> should be handled and trigger startMove immediately (GTK4 / Qt6 best practice)
	handled := win.beginTitlebarInteraction(1001, 100, 20, 1000)
	if !handled || movedSerial != 1001 {
		t.Fatalf("expected titlebar press to trigger startMove immediately, handled=%v, movedSerial=%d", handled, movedSerial)
	}

	// 2. Press on top edge resize (x=400, y=2) -> triggers startResize immediately on press (GTK4 / Qt6 best practice)
	handled = win.beginTitlebarInteraction(1002, 400, 2, 2000)
	if !handled || resizedSerial != 1002 || resizedXdgEdge != xdgToplevelResizeEdgeTop {
		t.Fatalf("expected edge resize to trigger startResize immediately on press, handled=%v, resizedSerial=%d, edge=%v", handled, resizedSerial, resizedXdgEdge)
	}

	// 3. Press on shadow/margin (x=400, y=-5) -> should be intercepted as edge resize immediately, NEVER move!
	resizedSerial = 0
	handled = win.beginTitlebarInteraction(1005, 400, -5, 2500)
	if !handled || resizedSerial != 1005 || resizedXdgEdge != xdgToplevelResizeEdgeTop {
		t.Fatalf("expected margin press to trigger edge resize immediately, handled=%v, resizedSerial=%d, edge=%v", handled, resizedSerial, resizedXdgEdge)
	}

	// 4. Press on controls area (x=750, y=20) -> should NOT be intercepted by titlebar
	handled = win.beginTitlebarInteraction(1003, 750, 20, 3000)
	if handled {
		t.Fatal("press on control button should not be intercepted as move/resize")
	}

	// 5. Double-click on titlebar (time diff <= 400ms) -> should trigger toggleMaximize instead of move
	movedSerial = 0
	win.lastClickTime = 4000
	handled = win.beginTitlebarInteraction(1004, 200, 20, 4200)
	if !handled || movedSerial != 0 {
		t.Fatalf("expected double-click to be handled without starting move, handled=%v, movedSerial=%d", handled, movedSerial)
	}
	_ = maximizedToggleCount
}

func TestWaylandPressMoveOnDragDeliversPressBeforeMoving(t *testing.T) {
	var movedSerial uint32
	var moves, maximizes int
	api := &waylandAPI{
		proxyMarshalFlags: func(proxy uintptr, opcode uint32, interfacePtr uintptr, version uint32, flags uint32, args ...any) uintptr {
			switch opcode {
			case xdgToplevelMoveOpcode:
				moves++
				if len(args) >= 2 {
					if serial, ok := args[1].(uint32); ok {
						movedSerial = serial
					}
				}
			case xdgToplevelSetMaximizedOpcode:
				maximizes++
			}
			return 0
		},
	}
	app := &waylandApplication{
		api:     api,
		globals: waylandGlobals{seat: 99},
	}
	win := &waylandWindow{
		app:         app,
		xdgToplevel: 10,
		width:       848, // 800 + 2*24
		height:      648, // 600 + 2*24
		margin:      24,
	}
	win.storeScale(1.0)

	var events []platform.PointerEvent
	state := &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			WindowMode:     platform.WindowCustomTitlebar,
			TitlebarHeight: 40,
			PressAt: func(x, y float32) platform.PressKind {
				if y >= 0 && y < 40 {
					if x >= 400 && x < 700 {
						return platform.PressMoveOnDrag
					}
					return platform.PressMove
				}
				return platform.PressClient
			},
			OnPointer: func(event platform.PointerEvent) (platform.Frame, error) {
				events = append(events, event)
				return platform.Frame{}, nil
			},
		},
	}
	state.router = eventRouter{
		config: state.config,
		scale:  win.displayScale,
		clicks: &input.ClickCounter{Interval: 500 * time.Millisecond, Distance: 4},
	}
	win.state = state
	app.windows = []*waylandWindowState{state}
	app.pointerFocus = win
	initPointerListeners()

	press := func(serial, timeMs uint32) {
		purego.SyscallN(rawPointerListener[3], uintptr(unsafe.Pointer(app)), 0,
			uintptr(serial), uintptr(timeMs), uintptr(btnLeft), 1)
	}
	release := func(serial, timeMs uint32) {
		purego.SyscallN(rawPointerListener[3], uintptr(unsafe.Pointer(app)), 0,
			uintptr(serial), uintptr(timeMs), uintptr(btnLeft), 0)
	}
	// Motion callbacks carry surface coordinates; the content sits at the
	// margin offset inside the surface.
	motion := func(x, y float32) {
		puregoInvokeCallback5(rawPointerListener[2], unsafe.Pointer(app), 0, 0,
			float64ToWlFixed(float64(x)+24), float64ToWlFixed(float64(y)+24))
	}
	lastEvent := func() platform.PointerEvent {
		if len(events) == 0 {
			t.Fatal("no pointer event reached the application")
		}
		return events[len(events)-1]
	}

	// A press on a move region under a backdrop returns false to the caller,
	// so the application receives the press and no move starts yet.
	win.pointerX, win.pointerY = 500, 20
	press(300, 1000)
	if !win.pendingMove || win.pendingMoveSerial != 300 {
		t.Fatalf("press did not leave a pending move: pending=%v serial=%d", win.pendingMove, win.pendingMoveSerial)
	}
	if moves != 0 {
		t.Fatalf("a PressMoveOnDrag press started %d moves", moves)
	}
	if ev := lastEvent(); ev.Kind != platform.PointerDown || ev.Button != ui.PointerButtonPrimary || ev.X != 500 || ev.Y != 20 {
		t.Fatalf("application did not receive the press: %+v", ev)
	}

	// Motion within the slop stays the application's, and the window stays put.
	motion(502, 21)
	if moves != 0 || !win.pendingMove {
		t.Fatalf("motion within the slop moved the window: moves=%d pending=%v", moves, win.pendingMove)
	}
	if ev := lastEvent(); ev.Kind != platform.PointerMove || ev.X != 502 || ev.Y != 21 {
		t.Fatalf("motion within the slop was not delivered to the application: %+v", ev)
	}

	// Crossing the slop starts the move with the press serial and ends the
	// application's press, which will not see the release.
	motion(520, 21)
	if moves != 1 || movedSerial != 300 || win.pendingMove {
		t.Fatalf("drag did not start the move: moves=%d serial=%d pending=%v", moves, movedSerial, win.pendingMove)
	}
	if ev := lastEvent(); ev.Kind != platform.PointerExit {
		t.Fatalf("application was not told its press ended: %+v", ev)
	}

	// The move is asked for once; later motion is still the application's.
	motion(540, 40)
	if moves != 1 {
		t.Fatalf("a started move was asked for again: %d moves", moves)
	}
	if ev := lastEvent(); ev.Kind != platform.PointerMove {
		t.Fatalf("motion after the move started was not delivered: %+v", ev)
	}

	// A press released without dragging never moves the window.
	win.pointerX, win.pointerY = 500, 20
	press(301, 5000)
	release(301, 5001)
	if win.pendingMove {
		t.Fatal("release left a pending move")
	}
	motion(540, 40)
	if moves != 1 {
		t.Fatalf("a released press still started a move: %d moves", moves)
	}

	// A double click on the same region still toggles maximize and does not
	// start a move or reach the application.
	before := len(events)
	win.pointerX, win.pointerY = 500, 20
	press(302, 5100)
	if maximizes != 1 || moves != 1 || win.pendingMove {
		t.Fatalf("double click: maximizes=%d moves=%d pending=%v", maximizes, moves, win.pendingMove)
	}
	if len(events) != before {
		t.Fatalf("double click reached the application: %d new events", len(events)-before)
	}
}

func TestWaylandPointerEventsDispatch(t *testing.T) {
	app := &waylandApplication{}
	win := &waylandWindow{
		app:     app,
		surface: 100,
		width:   800,
		height:  600,
	}
	win.storeScale(1.0)

	var pointerEvents []platform.PointerEvent
	var scrollEvents []platform.ScrollEvent

	state := &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			OnPointer: func(event platform.PointerEvent) (platform.Frame, error) {
				pointerEvents = append(pointerEvents, event)
				return platform.Frame{}, nil
			},
			OnScroll: func(event platform.ScrollEvent) (platform.Frame, error) {
				scrollEvents = append(scrollEvents, event)
				return platform.Frame{}, nil
			},
		},
	}
	state.router = eventRouter{
		config: state.config,
		scale:  win.displayScale,
		clicks: &input.ClickCounter{Interval: 500 * time.Millisecond, Distance: 4},
	}
	win.state = state
	app.windows = []*waylandWindowState{state}

	initPointerListeners()

	// 1. Enter
	// Simulate enter via direct pointerCoords & dispatch
	x, y := win.pointerCoords(float64ToWlFixed(50), float64ToWlFixed(60))
	win.pointerX, win.pointerY = x, y
	_ = state.router.dispatchPointer(platform.PointerEvent{Kind: platform.PointerMove, X: x, Y: y})

	if len(pointerEvents) != 1 || pointerEvents[0].X != 50 || pointerEvents[0].Y != 60 {
		t.Fatalf("expected pointer event at (50, 60), got %v", pointerEvents)
	}

	// 2. Button Down
	_ = state.router.dispatchPointer(platform.PointerEvent{Kind: platform.PointerDown, Button: ui.PointerButtonPrimary, X: x, Y: y, ClickCount: 1})
	if len(pointerEvents) != 2 || pointerEvents[1].Kind != platform.PointerDown || pointerEvents[1].Button != ui.PointerButtonPrimary {
		t.Fatalf("expected PointerDown event, got %v", pointerEvents)
	}

	// 3. Scroll
	_ = state.router.dispatchScroll(platform.ScrollEvent{DeltaY: 48})
	if len(scrollEvents) != 1 || scrollEvents[0].DeltaY != 48 {
		t.Fatalf("expected scroll event with DeltaY=48, got %v", scrollEvents)
	}
}

func TestWaylandPointerAndScrollErrorsReachApplication(t *testing.T) {
	t.Run("pointer", func(t *testing.T) {
		expectedErr := errors.New("pointer failed")
		app := &waylandApplication{}
		win := &waylandWindow{app: app}
		state := &waylandWindowState{
			app: app,
			win: win,
			config: platform.Config{
				OnPointer: func(platform.PointerEvent) (platform.Frame, error) {
					return platform.Frame{}, expectedErr
				},
			},
		}
		state.router = eventRouter{config: state.config}
		win.state = state

		app.dispatchPointerEvent(win, platform.PointerEvent{Kind: platform.PointerMove})
		if !errors.Is(app.firstErr, expectedErr) {
			t.Fatalf("application error = %v, want %v", app.firstErr, expectedErr)
		}
	})

	t.Run("scroll", func(t *testing.T) {
		expectedErr := errors.New("scroll failed")
		app := &waylandApplication{}
		win := &waylandWindow{app: app}
		state := &waylandWindowState{
			app: app,
			win: win,
			config: platform.Config{
				OnScroll: func(platform.ScrollEvent) (platform.Frame, error) {
					return platform.Frame{}, expectedErr
				},
			},
		}
		state.router = eventRouter{config: state.config}
		win.state = state

		app.dispatchScrollEvent(win, platform.ScrollEvent{DeltaY: 1})
		if !errors.Is(app.firstErr, expectedErr) {
			t.Fatalf("application error = %v, want %v", app.firstErr, expectedErr)
		}
	})
}

func TestWaylandPointerScrollFrameAndV120Deduplication(t *testing.T) {
	app := &waylandApplication{
		modifiers: input.ModifierControl,
	}
	win := &waylandWindow{
		app:      app,
		surface:  101,
		pointerX: 120,
		pointerY: 240,
	}
	win.storeScale(1.0)

	var scrollEvents []platform.ScrollEvent
	state := &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			OnScroll: func(event platform.ScrollEvent) (platform.Frame, error) {
				scrollEvents = append(scrollEvents, event)
				return platform.Frame{}, nil
			},
		},
	}
	state.router = eventRouter{config: state.config, scale: win.displayScale}
	win.state = state
	app.windows = []*waylandWindowState{state}
	app.pointerFocus = win

	initPointerListeners()

	// axis_value120 (120 units = 1 notch = 48.0 delta)
	puregoInvokeCallback4(rawPointerListener[9], unsafe.Pointer(app), 0, 0, 120)

	// Fallback axis event in the same frame: must be ignored because axis_v120 was already processed!
	puregoInvokeCallback5(rawPointerListener[4], unsafe.Pointer(app), 0, 100, 0, float64ToWlFixed(10))

	// Before frame(): no scroll should be dispatched yet
	if len(scrollEvents) != 0 {
		t.Fatalf("expected 0 scroll events before frame(), got %d", len(scrollEvents))
	}

	// frame()
	puregoInvokeCallback2(rawPointerListener[5], unsafe.Pointer(app), 0)

	if len(scrollEvents) != 1 {
		t.Fatalf("expected exactly 1 scroll event after frame(), got %d", len(scrollEvents))
	}
	ev := scrollEvents[0]
	if ev.DeltaY != 48.0 || ev.X != 120 || ev.Y != 240 || ev.Modifiers != input.ModifierControl {
		t.Fatalf("unexpected scroll event: %+v", ev)
	}
}

func TestWaylandPointerCoordsWithScaleAndMargin(t *testing.T) {
	win := &waylandWindow{
		margin: 48, // physical margin at 2.0x scale -> 24 logical margin
	}
	win.storeScale(2.0)

	// Surface coords (24, 24) should map to content logical coords (0, 0)
	x, y := win.pointerCoords(float64ToWlFixed(24), float64ToWlFixed(24))
	if x != 0 || y != 0 {
		t.Fatalf("expected (0, 0), got (%f, %f)", x, y)
	}

	// Surface coords (124, 224) should map to content logical coords (100, 200)
	x, y = win.pointerCoords(float64ToWlFixed(124), float64ToWlFixed(224))
	if x != 100 || y != 200 {
		t.Fatalf("expected (100, 200), got (%f, %f)", x, y)
	}

	// When margin is 0 (maximized or frameless)
	win.margin = 0
	x, y = win.pointerCoords(float64ToWlFixed(50), float64ToWlFixed(60))
	if x != 50 || y != 60 {
		t.Fatalf("expected (50, 60), got (%f, %f)", x, y)
	}
}
