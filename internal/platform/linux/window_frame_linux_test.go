//go:build linux

package linux

import (
	"errors"
	"os"
	"testing"
	"unsafe"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

func openFramedWindow(t *testing.T, config platform.Config) (*linuxApplication, *nativeWindow) {
	t.Helper()
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no X display")
	}
	t.Setenv("VIVA_LINUX_FRAME", "force")
	// These tests check the Vulkan frame margin, so they keep the software Vulkan
	// device that a window would otherwise replace with the CPU renderer.
	t.Setenv("VIVA_LINUX_RENDERER", "vulkan")
	application := &linuxApplication{}
	opened, err := application.OpenWindow(config)
	if errors.Is(err, platform.ErrNoGraphics) {
		t.Skipf("no Vulkan device: %v", err)
	}
	if err != nil {
		t.Skipf("no window: %v", err)
	}
	state := opened.(*nativeWindow).state
	if _, ok := state.renderer.(*cpuWindowRenderer); ok {
		t.Skip("the CPU renderer has no alpha surface, so the window has no drawn frame")
	}
	t.Cleanup(application.closeAll)
	return application, opened.(*nativeWindow)
}

// The application sees the content: its size, and where it is on the screen. The
// margin around it, which holds the shadow, is the window manager's and Viva's.
func TestFramedWindowReportsItsContentNotItsMargin(t *testing.T) {
	_, window := openFramedWindow(t, platform.Config{
		Title: "framed", Width: 300, Height: 200, WindowMode: platform.WindowCustomTitlebar, Actions: &platform.WindowActions{},
	})
	win := window.state.win
	if win.margin == 0 {
		t.Fatal("the window has no margin")
	}
	scale := win.displayScale()
	if want := deviceLength(300, scale) + 2*win.margin; win.width != want {
		t.Fatalf("X window is %d wide, want content plus both margins = %d", win.width, want)
	}
	size := win.logicalSize()
	if size.Width != 300 || size.Height != 200 {
		t.Fatalf("logical size = %vx%v, want the 300x200 asked for", size.Width, size.Height)
	}
	frame, ok := window.Frame()
	if !ok || frame.Width != 300 || frame.Height != 200 {
		t.Fatalf("Frame = %+v (%v), want the content's 300x200", frame, ok)
	}
	originX, originY := win.rootOrigin()
	if want := float32(originX+int32(win.margin)) / scale; frame.X != want {
		t.Fatalf("Frame.X = %v, want the content's %v (window at %d, margin %d)", frame.X, want, originX, win.margin)
	}
	if want := float32(originY+int32(win.margin)) / scale; frame.Y != want {
		t.Fatalf("Frame.Y = %v, want %v", frame.Y, want)
	}
}

// Pointer positions are the content's: a press at the content's corner is at 0,0, and
// one in the grip of the margin is outside it.
func TestFramedWindowPointerPositionsStartAtTheContent(t *testing.T) {
	_, window := openFramedWindow(t, platform.Config{
		Title: "framed", Width: 300, Height: 200, WindowMode: platform.WindowCustomTitlebar, Actions: &platform.WindowActions{},
	})
	win := window.state.win
	for _, test := range []struct {
		kind         uint8
		windowX      int16
		windowY      int16
		wantX, wantY int16
	}{
		{xcbButtonPress, int16(win.margin), int16(win.margin), 0, 0},
		{xcbMotionNotify, int16(win.margin) + 40, int16(win.margin) + 7, 40, 7},
		{xcbButtonRelease, int16(win.margin) - 3, int16(win.margin) - 3, -3, -3},
	} {
		event := xcbInputEvent{ResponseType: test.kind, EventX: test.windowX, EventY: test.windowY}
		win.shiftPointer(test.kind, unsafe.Pointer(&event))
		if event.EventX != test.wantX || event.EventY != test.wantY {
			t.Fatalf("kind %d: %d,%d, want %d,%d", test.kind, event.EventX, event.EventY, test.wantX, test.wantY)
		}
	}
	// A key event has no pointer position to move.
	key := xcbInputEvent{ResponseType: xcbKeyPress, EventX: 9, EventY: 9}
	win.shiftPointer(xcbKeyPress, unsafe.Pointer(&key))
	if key.EventX != 9 || key.EventY != 9 {
		t.Fatal("a key event was moved")
	}
}

// A window that is not drawn with a frame has no margin and reports the whole window.
func TestWindowWithoutAFrameHasNoMargin(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no X display")
	}
	t.Setenv("VIVA_LINUX_FRAME", "force")
	application := &linuxApplication{}
	opened, err := application.OpenWindow(platform.Config{Title: "titled", Width: 300, Height: 200, WindowMode: platform.WindowTitled})
	if err != nil {
		t.Skipf("no window: %v", err)
	}
	defer application.closeAll()
	if win := opened.(*nativeWindow).state.win; win.margin != 0 || win.frameWanted {
		t.Fatalf("a titled window has margin %d", win.margin)
	}
}

// A move to a display of another scale keeps the content's logical size and
// gives the margin a whole number of pixels at the new scale.
func TestFramedWindowKeepsItsContentWhenTheScaleChanges(t *testing.T) {
	_, window := openFramedWindow(t, platform.Config{
		Title: "framed", Width: 300, Height: 200, WindowMode: platform.WindowCustomTitlebar, Actions: &platform.WindowActions{},
	})
	state := window.state
	if err := state.applyScale(2); err != nil {
		t.Fatal(err)
	}
	win := state.win
	if win.margin != frameMargin(2) || state.renderer.frameMargin() != frameMargin(2) {
		t.Fatalf("margin %d (renderer %d), want %d at 2x", win.margin, state.renderer.frameMargin(), frameMargin(2))
	}
	if size := win.logicalSize(); size.Scale != 2 {
		t.Fatalf("scale %v", size.Scale)
	}
}

func TestFramedWindowSuppressesFrameChangeWhileMaximized(t *testing.T) {
	var frames []gfx.Rect
	var maximizes []bool
	_, window := openFramedWindow(t, platform.Config{
		Title: "framed", Width: 300, Height: 200, WindowMode: platform.WindowCustomTitlebar, Actions: &platform.WindowActions{},
		OnFrameChange: func(frame gfx.Rect) { frames = append(frames, frame) },
		OnMaximize:    func(maximized bool) { maximizes = append(maximizes, maximized) },
	})
	state := window.state
	win := state.win
	state.reportFrame()
	if len(frames) != 1 {
		t.Fatalf("initial frames reported = %d, want 1", len(frames))
	}
	initialFrame := frames[0]
	if initialFrame.Width != 300 || initialFrame.Height != 200 {
		t.Fatalf("initial frame = %+v, want 300x200", initialFrame)
	}

	// When maximized, reportFrame must not overwrite the unmaximized frame geometry.
	win.frame.maximized = true
	win.width, win.height = 2048, 1236
	win.margin = 0
	state.reportFrame()
	if len(frames) != 1 {
		t.Fatalf("frames reported while maximized = %d, want OnFrameChange suppressed", len(frames))
	}

	// When restored, reportFrame reports the restored frame.
	win.frame.maximized = false
	scale := win.displayScale()
	win.margin = frameMargin(scale)
	win.width = deviceLength(300, scale) + 2*win.margin
	win.height = deviceLength(200, scale) + 2*win.margin
	// Simulate move to a different position
	win.lastFrame = gfx.Rect{}
	state.reportFrame()
	if len(frames) != 2 {
		t.Fatalf("frames reported after restore = %d, want 2", len(frames))
	}
}

func TestConfigureNotifySynchronizesMarginAndGeometry(t *testing.T) {
	var resizes []platform.Size
	var maximizes []bool
	_, window := openFramedWindow(t, platform.Config{
		Title: "framed", Width: 300, Height: 200, WindowMode: platform.WindowCustomTitlebar, Actions: &platform.WindowActions{},
		OnResize: func(size platform.Size) (platform.Frame, error) {
			resizes = append(resizes, size)
			return platform.Frame{}, nil
		},
		OnMaximize: func(maximized bool) { maximizes = append(maximizes, maximized) },
	})
	state := window.state
	win := state.win
	scale := win.displayScale()

	// Set _NET_WM_STATE to maximized on the X server, simulating what the WM does
	states := [2]uint32{win.frame.maximizedVertical, win.frame.maximizedHorizontal}
	win.api.changeProperty(win.connection, xcbPropertyReplace, win.id, win.frame.stateAtom, xcbAtomAtom, 32, 2, unsafe.Pointer(&states[0]))
	win.api.flush(win.connection)

	configure := xcbConfigureEvent{
		ResponseType: xcbConfigureNotify,
		Width:        2048,
		Height:       1236,
	}
	closeWin, err := state.handleEvent(unsafe.Pointer(&configure))
	if closeWin || err != nil {
		t.Fatalf("handleEvent configure: close=%v, err=%v", closeWin, err)
	}

	// Verify margin was atomically set to 0 and logicalSize computed without margin
	if win.margin != 0 {
		t.Fatalf("win.margin = %d, want 0 when maximized", win.margin)
	}
	if !win.frame.maximized {
		t.Fatal("win.frame.maximized = false, want true")
	}
	if len(maximizes) != 1 || !maximizes[0] {
		t.Fatalf("maximizes = %v, want [true]", maximizes)
	}
	if len(resizes) == 0 {
		t.Fatal("OnResize was not called")
	}
	lastResize := resizes[len(resizes)-1]
	if wantW := float32(2048) / scale; lastResize.Width != wantW {
		t.Fatalf("last resize width = %v, want %v (no margin subtracted)", lastResize.Width, wantW)
	}

	// Now simulate restore: WM clears _NET_WM_STATE and configures window back to floating size
	empty := [0]uint32{}
	win.api.changeProperty(win.connection, xcbPropertyReplace, win.id, win.frame.stateAtom, xcbAtomAtom, 32, 0, unsafe.Pointer(&empty))
	win.api.flush(win.connection)

	resizesCountBeforeRestore := len(resizes)
	restoreWidth := deviceLength(300, scale) + 2*frameMargin(scale)
	restoreHeight := deviceLength(200, scale) + 2*frameMargin(scale)
	restoreConfigure := xcbConfigureEvent{
		ResponseType: xcbConfigureNotify,
		Width:        uint16(restoreWidth),
		Height:       uint16(restoreHeight),
	}
	closeWin, err = state.handleEvent(unsafe.Pointer(&restoreConfigure))
	if closeWin || err != nil {
		t.Fatalf("handleEvent restore configure: close=%v, err=%v", closeWin, err)
	}

	if win.margin != frameMargin(scale) {
		t.Fatalf("win.margin = %d, want %d after restore", win.margin, frameMargin(scale))
	}
	if win.frame.maximized {
		t.Fatal("win.frame.maximized = true, want false")
	}
	if len(maximizes) != 2 || maximizes[1] {
		t.Fatalf("maximizes = %v, want [true, false]", maximizes)
	}
	if len(resizes) != resizesCountBeforeRestore+1 {
		t.Fatalf("resizes on restore = %d, want exactly 1 resize", len(resizes)-resizesCountBeforeRestore)
	}
	restoreResize := resizes[len(resizes)-1]
	if restoreResize.Width != 300 || restoreResize.Height != 200 {
		t.Fatalf("restored size = %vx%v, want 300x200", restoreResize.Width, restoreResize.Height)
	}
}

func TestPropertyNotifyBeforeConfigureNotifyDoesNotGlitch(t *testing.T) {
	var resizes []platform.Size
	var frames []gfx.Rect
	var maximizes []bool
	_, window := openFramedWindow(t, platform.Config{
		Title: "framed", Width: 300, Height: 200, WindowMode: platform.WindowCustomTitlebar, Actions: &platform.WindowActions{},
		OnResize: func(size platform.Size) (platform.Frame, error) {
			resizes = append(resizes, size)
			return platform.Frame{}, nil
		},
		OnFrameChange: func(frame gfx.Rect) { frames = append(frames, frame) },
		OnMaximize:    func(maximized bool) { maximizes = append(maximizes, maximized) },
	})
	state := window.state
	win := state.win
	scale := win.displayScale()

	// Maximize first
	states := [2]uint32{win.frame.maximizedVertical, win.frame.maximizedHorizontal}
	win.api.changeProperty(win.connection, xcbPropertyReplace, win.id, win.frame.stateAtom, xcbAtomAtom, 32, 2, unsafe.Pointer(&states[0]))
	win.api.flush(win.connection)

	configure := xcbConfigureEvent{
		ResponseType: xcbConfigureNotify,
		Width:        2048,
		Height:       1236,
	}
	state.handleEvent(unsafe.Pointer(&configure))

	initialFramesCount := len(frames)
	resizesBeforeRestore := len(resizes)

	// Now simulate restore: WM clears _NET_WM_STATE on X server
	empty := [0]uint32{}
	win.api.changeProperty(win.connection, xcbPropertyReplace, win.id, win.frame.stateAtom, xcbAtomAtom, 32, 0, unsafe.Pointer(&empty))
	win.api.flush(win.connection)

	// Deliver PropertyNotify first
	propEvent := xcbPropertyEvent{
		ResponseType: xcbPropertyNotify,
		Window:       win.id,
		Atom:         win.frame.stateAtom,
	}
	closeWin, err := state.handleEvent(unsafe.Pointer(&propEvent))
	if closeWin || err != nil {
		t.Fatalf("handleEvent property: close=%v, err=%v", closeWin, err)
	}
	if len(resizes) != resizesBeforeRestore {
		t.Fatalf("PropertyNotify resized before final geometry: got %d resize events", len(resizes)-resizesBeforeRestore)
	}

	// Now deliver ConfigureNotify
	restoreWidth := deviceLength(300, scale) + 2*frameMargin(scale)
	restoreHeight := deviceLength(200, scale) + 2*frameMargin(scale)
	restoreConfigure := xcbConfigureEvent{
		ResponseType: xcbConfigureNotify,
		Width:        uint16(restoreWidth),
		Height:       uint16(restoreHeight),
	}
	closeWin, err = state.handleEvent(unsafe.Pointer(&restoreConfigure))
	if closeWin || err != nil {
		t.Fatalf("handleEvent configure: close=%v, err=%v", closeWin, err)
	}

	// Final check: restored size is 300x200
	restoreResize := resizes[len(resizes)-1]
	if restoreResize.Width != 300 || restoreResize.Height != 200 {
		t.Fatalf("restored size = %vx%v, want 300x200", restoreResize.Width, restoreResize.Height)
	}
	if len(resizes) != resizesBeforeRestore+1 {
		t.Fatalf("resizes on property-first restore = %d, want exactly 1", len(resizes)-resizesBeforeRestore)
	}
	// And OnFrameChange must NOT have been called with maximized 2048x1236 dimensions
	for _, f := range frames[initialFramesCount:] {
		if f.Width > 1000 || f.Height > 1000 {
			t.Fatalf("OnFrameChange emitted maximized frame: %+v", f)
		}
	}
}

func TestConfigureNotifyBeforePropertyNotifySynchronizesMargin(t *testing.T) {
	var resizes []platform.Size
	var frames []gfx.Rect
	var maximizes []bool
	_, window := openFramedWindow(t, platform.Config{
		Title: "framed", Width: 300, Height: 200, WindowMode: platform.WindowCustomTitlebar, Actions: &platform.WindowActions{},
		OnResize: func(size platform.Size) (platform.Frame, error) {
			resizes = append(resizes, size)
			return platform.Frame{}, nil
		},
		OnFrameChange: func(frame gfx.Rect) { frames = append(frames, frame) },
		OnMaximize:    func(maximized bool) { maximizes = append(maximizes, maximized) },
	})
	state := window.state
	win := state.win
	scale := win.displayScale()

	win.toggleMaximize()
	resizesBeforeMaximize := len(resizes)
	framesBeforeMaximize := len(frames)

	// Step 1: WM sends ConfigureNotify with maximized dimensions FIRST,
	// while _NET_WM_STATE is not yet set on the X server.
	configure := xcbConfigureEvent{
		ResponseType: xcbConfigureNotify,
		Width:        2048,
		Height:       1236,
	}
	closeWin, err := state.handleEvent(unsafe.Pointer(&configure))
	if closeWin || err != nil {
		t.Fatalf("handleEvent configure: close=%v, err=%v", closeWin, err)
	}

	// The request target supplies the state until PropertyNotify confirms it, so
	// geometry, margin and application callbacks are committed together.
	if !win.frame.maximized {
		t.Fatal("win.frame.maximized = false after requested maximize ConfigureNotify")
	}
	if len(resizes) != resizesBeforeMaximize+1 {
		t.Fatalf("resizes on configure-first maximize = %d, want exactly 1", len(resizes)-resizesBeforeMaximize)
	}
	if len(frames) != framesBeforeMaximize {
		t.Fatalf("OnFrameChange emitted maximized geometry: %v", frames[framesBeforeMaximize:])
	}

	// Step 2: WM now sets _NET_WM_STATE to maximized on X server and sends PropertyNotify SECOND
	states := [2]uint32{win.frame.maximizedVertical, win.frame.maximizedHorizontal}
	win.api.changeProperty(win.connection, xcbPropertyReplace, win.id, win.frame.stateAtom, xcbAtomAtom, 32, 2, unsafe.Pointer(&states[0]))
	win.api.flush(win.connection)

	propEvent := xcbPropertyEvent{
		ResponseType: xcbPropertyNotify,
		Window:       win.id,
		Atom:         win.frame.stateAtom,
	}
	closeWin, err = state.handleEvent(unsafe.Pointer(&propEvent))
	if closeWin || err != nil {
		t.Fatalf("handleEvent property: close=%v, err=%v", closeWin, err)
	}
	if len(resizes) != resizesBeforeMaximize+1 {
		t.Fatalf("PropertyNotify caused a second maximize resize: %d", len(resizes)-resizesBeforeMaximize)
	}

	// PropertyNotify must have zeroed the margin and resized with margin = 0
	if win.margin != 0 {
		t.Fatalf("win.margin = %d, want 0 when PropertyNotify arrives", win.margin)
	}
	if !win.frame.maximized {
		t.Fatal("win.frame.maximized = false, want true")
	}
	if len(maximizes) != 1 || !maximizes[0] {
		t.Fatalf("maximizes = %v, want [true]", maximizes)
	}
	lastResize := resizes[len(resizes)-1]
	if wantW := float32(2048) / scale; lastResize.Width != wantW {
		t.Fatalf("last resize width = %v, want %v (no margin subtracted)", lastResize.Width, wantW)
	}

	// Now test restore with ConfigureNotify FIRST, PropertyNotify SECOND:
	win.toggleMaximize()
	resizesBeforeRestore := len(resizes)
	restoreWidth := deviceLength(300, scale) + 2*frameMargin(scale)
	restoreHeight := deviceLength(200, scale) + 2*frameMargin(scale)
	restoreConfigure := xcbConfigureEvent{
		ResponseType: xcbConfigureNotify,
		Width:        uint16(restoreWidth),
		Height:       uint16(restoreHeight),
	}
	// WM sends restore ConfigureNotify before clearing _NET_WM_STATE
	closeWin, err = state.handleEvent(unsafe.Pointer(&restoreConfigure))
	if closeWin || err != nil {
		t.Fatalf("handleEvent restore configure: close=%v, err=%v", closeWin, err)
	}
	if len(resizes) != resizesBeforeRestore+1 {
		t.Fatalf("resizes on configure-first restore = %d, want exactly 1", len(resizes)-resizesBeforeRestore)
	}

	// Now WM clears _NET_WM_STATE on X server and sends PropertyNotify
	empty := [0]uint32{}
	win.api.changeProperty(win.connection, xcbPropertyReplace, win.id, win.frame.stateAtom, xcbAtomAtom, 32, 0, unsafe.Pointer(&empty))
	win.api.flush(win.connection)

	restorePropEvent := xcbPropertyEvent{
		ResponseType: xcbPropertyNotify,
		Window:       win.id,
		Atom:         win.frame.stateAtom,
	}
	closeWin, err = state.handleEvent(unsafe.Pointer(&restorePropEvent))
	if closeWin || err != nil {
		t.Fatalf("handleEvent restore property: close=%v, err=%v", closeWin, err)
	}
	if len(resizes) != resizesBeforeRestore+1 {
		t.Fatalf("PropertyNotify caused a second restore resize: %d", len(resizes)-resizesBeforeRestore)
	}

	// Verify margin was restored and final size is 300x200
	if win.margin != frameMargin(scale) {
		t.Fatalf("win.margin = %d, want %d after restore", win.margin, frameMargin(scale))
	}
	if win.frame.maximized {
		t.Fatal("win.frame.maximized = true, want false after restore")
	}
	if len(maximizes) != 2 || maximizes[1] {
		t.Fatalf("maximizes = %v, want [true, false]", maximizes)
	}
	restoreResize := resizes[len(resizes)-1]
	if restoreResize.Width != 300 || restoreResize.Height != 200 {
		t.Fatalf("restored size = %vx%v, want 300x200", restoreResize.Width, restoreResize.Height)
	}
}
