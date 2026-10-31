//go:build linux

package linux

import (
	"errors"
	"math"
	"testing"
	"time"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/ui"
)

func TestWaylandSurfaceProviderExtensionsAndCreate(t *testing.T) {
	provider := newWaylandSurfaceProvider(0x1234, 0x5678)
	exts := provider.RequiredExtensions()
	if len(exts) != 2 || waylandGoString(exts[0]) != "VK_KHR_surface" || waylandGoString(exts[1]) != "VK_KHR_wayland_surface" {
		t.Fatalf("unexpected RequiredExtensions: %v, %v", waylandGoString(exts[0]), waylandGoString(exts[1]))
	}

	if unsafe.Sizeof(vkWaylandSurfaceCreateInfo{}) != 40 {
		t.Fatalf("vkWaylandSurfaceCreateInfo size = %d, want 40", unsafe.Sizeof(vkWaylandSurfaceCreateInfo{}))
	}
}

func TestWaylandWindowModesGeometryAndInputRegion(t *testing.T) {
	scales := []float32{1.0, 1.5, 2.0}

	for _, scale := range scales {
		margin := frameMargin(scale)
		grip := frameGrip(scale)

		wantMargin := uint32(math.Round(float64(24) * float64(scale)))
		wantGrip := uint32(math.Round(float64(8) * float64(scale)))

		if margin != wantMargin {
			t.Errorf("scale %g: margin = %d, want %d", scale, margin, wantMargin)
		}
		if grip != wantGrip {
			t.Errorf("scale %g: grip = %d, want %d", scale, grip, wantGrip)
		}

		logicalW, logicalH := 400, 300
		contentW := deviceLength(logicalW, scale)
		contentH := deviceLength(logicalH, scale)
		totalW := contentW + 2*margin
		totalH := contentH + 2*margin

		// Content box for set_window_geometry
		contentBoxW := contentSize(totalW, margin)
		contentBoxH := contentSize(totalH, margin)
		if contentBoxW != contentW || contentBoxH != contentH {
			t.Errorf("scale %g: contentBox (%d, %d), want (%d, %d)", scale, contentBoxW, contentBoxH, contentW, contentH)
		}

		// Input region calculation
		reach := min(grip, margin)
		rx := int32(margin - reach)
		ry := int32(margin - reach)
		rw := int32(contentBoxW + 2*reach)
		rh := int32(contentBoxH + 2*reach)

		// The input box must strictly encompass content plus grip, leaving outer (margin - reach) for click passthrough
		if rx <= 0 {
			t.Errorf("scale %g: rx = %d, expected > 0 for shadow click passthrough", scale, rx)
		}
		if rw >= int32(totalW) {
			t.Errorf("scale %g: rw = %d, expected < totalW (%d) for shadow click passthrough", scale, rw, totalW)
		}
		if ry <= 0 {
			t.Errorf("scale %g: ry = %d, expected > 0 for shadow click passthrough", scale, ry)
		}
		if rh >= int32(totalH) {
			t.Errorf("scale %g: rh = %d, expected < totalH (%d) for shadow click passthrough", scale, rh, totalH)
		}
	}
}

func TestWaylandFrameConcurrentGeometrySnapshots(t *testing.T) {
	t.Run("resize", func(t *testing.T) {
		win := &waylandWindow{surface: 1, width: 400, height: 300, savedLogicalW: 400, savedLogicalH: 300}
		win.storeScale(1)
		ready := make(chan struct{})
		wrote := make(chan struct{})
		done := make(chan struct{})
		go func() {
			<-ready
			for i := 0; i < 1000; i++ {
				if i%2 == 0 {
					win.configure.width, win.configure.height = 500, 350
				} else {
					win.configure.width, win.configure.height = 400, 300
				}
				win.applyPendingResize()
				if i == 0 {
					close(wrote)
				}
			}
			close(done)
		}()
		var bad ui.Rect
		failed := false
		reads := 0
		check := func() {
			frame, _ := win.Frame()
			reads++
			valid := (frame.Width == 400 && frame.Height == 300) || (frame.Width == 500 && frame.Height == 350)
			if !valid && !failed {
				bad, failed = frame, true
			}
		}
		check() // Read the initial generation before releasing the writer.
		close(ready)
		<-wrote
		check() // At least one read must follow a completed write.
		for {
			select {
			case <-done:
				if reads < 2 {
					t.Fatal("test completed without a read following a write")
				}
				if failed {
					t.Fatalf("Frame returned a mixed geometry generation: %+v", bad)
				}
				return
			default:
				check()
			}
		}
	})

	t.Run("scale", func(t *testing.T) {
		win := &waylandWindow{surface: 1, width: 400, height: 300, savedLogicalW: 400, savedLogicalH: 300}
		win.storeScale(1)
		ready := make(chan struct{})
		wrote := make(chan struct{})
		done := make(chan struct{})
		go func() {
			<-ready
			for i := 0; i < 1000; i++ {
				if i%2 == 0 {
					win.applyScale(2)
				} else {
					win.applyScale(1)
				}
				if i == 0 {
					close(wrote)
				}
			}
			close(done)
		}()
		var bad ui.Rect
		failed := false
		reads := 0
		check := func() {
			frame, _ := win.Frame()
			reads++
			if (frame.Width != 400 || frame.Height != 300) && !failed {
				bad, failed = frame, true
			}
		}
		check() // Read the initial generation before releasing the writer.
		close(ready)
		<-wrote
		check() // At least one read must follow a completed write.
		for {
			select {
			case <-done:
				if reads < 2 {
					t.Fatal("test completed without a read following a write")
				}
				if failed {
					t.Fatalf("Frame returned mixed scale and dimensions: %+v", bad)
				}
				return
			default:
				check()
			}
		}
	})
}

func TestWaylandInitialConfigureGeometryBeforeState(t *testing.T) {
	win := &waylandWindow{
		surface:       1,
		xdgSurface:    2,
		width:         400,
		height:        300,
		savedLogicalW: 400,
		savedLogicalH: 300,
	}
	win.storeScale(1)

	win.handleXdgToplevelConfigure(800, 600, nil)
	win.handleXdgSurfaceConfigure(99)

	if win.width != 800 || win.height != 600 {
		t.Fatalf("initial configure geometry = %dx%d, want 800x600", win.width, win.height)
	}
	if !win.geometryDirty {
		t.Fatal("initial configure did not mark the corrected geometry dirty")
	}
	if win.configure.serial != 99 {
		t.Fatal("initial configure serial was consumed before the first buffer was ready")
	}

	var acked []uint32
	var frameRequests int
	app := &waylandApplication{api: newWaylandPresentTestAPI(&acked, &frameRequests)}
	win.app = app
	win.surfaceState = waylandSurfaceAwaitInitialConfigure
	var reported platform.Size
	win.state = &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			OnResize: func(size platform.Size) (platform.Frame, error) {
				reported = size
				return platform.Frame{}, nil
			},
		},
	}
	win.applyPendingResize()
	if app.firstErr != nil {
		t.Fatalf("initial resize commit: %v", app.firstErr)
	}
	if reported.Width != 800 || reported.Height != 600 {
		t.Fatalf("application initial size = %gx%g, want 800x600", reported.Width, reported.Height)
	}
	if len(acked) != 1 || acked[0] != 99 || win.surfaceState != waylandSurfaceMapped {
		t.Fatalf("initial commit ACK=%v state=%v, want serial 99 and Mapped", acked, win.surfaceState)
	}
}

func TestWaylandFocusLossErrorsReachApplication(t *testing.T) {
	expectedErr := errors.New("focus loss failed")
	app := &waylandApplication{}
	win := &waylandWindow{app: app, activated: true}
	win.state = &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			OnFocusLost: func() (platform.Frame, error) {
				return platform.Frame{}, expectedErr
			},
		},
	}

	win.handleXdgToplevelConfigure(0, 0, nil)
	if !errors.Is(app.firstErr, expectedErr) {
		t.Fatalf("application error = %v, want %v", app.firstErr, expectedErr)
	}
}

func TestWaylandConfigureStateTransitions(t *testing.T) {
	win := &waylandWindow{
		width:       500,
		height:      400,
		margin:      24,
		frameWanted: true,
	}
	win.storeScale(1.0)

	var maximizeReports []bool
	win.state = &waylandWindowState{
		win: win,
		config: platform.Config{
			OnMaximize: func(maximized bool) {
				maximizeReports = append(maximizeReports, maximized)
			},
		},
	}

	// 1. Toplevel configure: Maximized state set
	statesData := []uint32{xdgToplevelStateMaximized, xdgToplevelStateActivated}
	statesArray := wlArray{
		Size:  uintptr(len(statesData) * 4),
		Alloc: uintptr(len(statesData) * 4),
		Data:  unsafe.Pointer(&statesData[0]),
	}

	win.handleXdgToplevelConfigure(800, 600, &statesArray)

	if !win.maximized {
		t.Fatal("expected window to be marked maximized")
	}
	if win.margin != 0 {
		t.Fatalf("expected margin to be stripped to 0 on maximize, got %d", win.margin)
	}
	if len(maximizeReports) != 1 || !maximizeReports[0] {
		t.Fatalf("expected OnMaximize(true), got %v", maximizeReports)
	}

	// 2. Surface configure with serial
	win.handleXdgSurfaceConfigure(1001)
	if win.configure.serial != 1001 {
		t.Fatalf("expected configure.serial 1001, got %d", win.configure.serial)
	}
	if win.configure.toplevelPending {
		t.Fatal("expected toplevel configure transaction to be complete")
	}

	// 3. Toplevel configure: Unmaximized (Normal state)
	emptyStates := wlArray{}
	win.handleXdgToplevelConfigure(400, 300, &emptyStates)

	if win.maximized {
		t.Fatal("expected window to be unmaximized")
	}
	if win.margin != 24 {
		t.Fatalf("expected margin to be restored to 24 on unmaximize, got %d", win.margin)
	}
	if len(maximizeReports) != 2 || maximizeReports[1] != false {
		t.Fatalf("expected OnMaximize(false), got %v", maximizeReports)
	}
}

func TestWaylandFrameMarginErrorsStopTransitions(t *testing.T) {
	for _, test := range []struct {
		name           string
		margin         uint32
		rendererMargin uint32
		maximized      bool
		frameWanted    bool
		customTitlebar bool
		configure      func(*waylandWindow)
	}{
		{
			name: "maximize", margin: 24, rendererMargin: 24, frameWanted: true,
			configure: func(win *waylandWindow) {
				states := []uint32{xdgToplevelStateMaximized}
				win.handleXdgToplevelConfigure(800, 600, &wlArray{Size: 4, Data: unsafe.Pointer(&states[0])})
			},
		},
		{
			name: "unmaximize", maximized: true, frameWanted: true,
			configure: func(win *waylandWindow) {
				win.handleXdgToplevelConfigure(400, 300, nil)
			},
		},
		{
			name: "server decoration", margin: 24, rendererMargin: 24, frameWanted: true,
			configure: func(win *waylandWindow) {
				win.handleDecorationConfigure(zxdgToplevelDecorationModeServerSide)
			},
		},
		{
			name: "client decoration", customTitlebar: true,
			configure: func(win *waylandWindow) {
				win.handleDecorationConfigure(zxdgToplevelDecorationModeClientSide)
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			app := &waylandApplication{}
			win := &waylandWindow{
				app:         app,
				margin:      test.margin,
				maximized:   test.maximized,
				frameWanted: test.frameWanted,
			}
			win.storeScale(1)
			mode := platform.WindowTitled
			if test.customTitlebar {
				mode = platform.WindowCustomTitlebar
			}
			renderer := &vulkanRenderer{
				device:      1,
				commandPool: 1,
				api: &vulkanAPI{
					deviceWaitIdle: func(uintptr) int32 { return vkErrorDeviceLost },
				},
			}
			renderer.frame.margin = test.rendererMargin
			win.state = &waylandWindowState{
				app: app, win: win, renderer: renderer,
				config: platform.Config{WindowMode: mode},
			}

			test.configure(win)
			if app.firstErr == nil {
				t.Fatal("frame-margin error did not reach the application")
			}
			if win.geometryDirty {
				t.Fatal("transition continued after frame-margin failure")
			}
		})
	}
}

func newInitialConfigureTestApplication(destroyed *[]uintptr) *waylandApplication {
	app := &waylandApplication{display: 1}
	app.globals.compositor = 1
	app.globals.xdgWmBase = 1
	app.api = &waylandAPI{
		proxyAddListener: func(uintptr, uintptr, unsafe.Pointer) int32 { return 0 },
		proxyMarshalFlags: func(_ uintptr, opcode uint32, _ uintptr, _, _ uint32, _ ...any) uintptr {
			switch opcode {
			case wlCompositorCreateSurfaceOpcode:
				return 10
			case xdgWmBaseGetXdgSurfaceOpcode:
				return 20
			case xdgSurfaceGetToplevelOpcode:
				return 30
			default:
				return 0
			}
		},
		proxyDestroy: func(proxy uintptr) { *destroyed = append(*destroyed, proxy) },
	}
	installWaylandFrameTestCallbacks(app.api)
	return app
}

func TestWaylandOpenWindowWaitsAsynchronouslyForInitialConfigure(t *testing.T) {
	originalRendererFactory := newWaylandWindowRenderer
	t.Cleanup(func() { newWaylandWindowRenderer = originalRendererFactory })
	presents := 0
	newWaylandWindowRenderer = func(_ vulkanSurfaceProvider, width, height uint32, _ float32, _ platform.Color, _ []render.Rectangle, _ []platform.TextLabel, _ uint32) (*vulkanRenderer, error) {
		renderer := newWaylandPresentTestRenderer(width, height)
		renderer.api.queuePresent = func(uintptr, *vkPresentInfo) int32 {
			presents++
			return vkSuccess
		}
		return renderer, nil
	}

	var destroyed []uintptr
	app := newInitialConfigureTestApplication(&destroyed)
	roundtrips := 0
	app.api.displayRoundtrip = func(uintptr) int32 { roundtrips++; return 0 }
	win, err := app.openWindow(platform.Config{Width: 400, Height: 300})
	if err != nil || win == nil {
		t.Fatalf("openWindow = (%v, %v), want asynchronous success", win, err)
	}
	defer win.destroy()
	if roundtrips != 0 {
		t.Fatalf("openWindow used %d blocking roundtrips", roundtrips)
	}
	if win.surfaceState != waylandSurfaceAwaitInitialConfigure || win.configure.serial != 0 {
		t.Fatalf("initial state=%v serial=%d, want AwaitInitialConfigure without a serial", win.surfaceState, win.configure.serial)
	}
	if presents != 0 || win.framePending || len(app.windows) != 1 {
		t.Fatalf("unconfigured open presents=%d framePending=%v windows=%d", presents, win.framePending, len(app.windows))
	}
}

func TestWaylandOpenWindowCallbackFailureRetriesCleanly(t *testing.T) {
	originalRendererFactory := newWaylandWindowRenderer
	t.Cleanup(func() { newWaylandWindowRenderer = originalRendererFactory })

	presents := 0
	newWaylandWindowRenderer = func(_ vulkanSurfaceProvider, width, height uint32, _ float32, _ platform.Color, _ []render.Rectangle, _ []platform.TextLabel, _ uint32) (*vulkanRenderer, error) {
		renderer := newWaylandPresentTestRenderer(width, height)
		// Keep presentation functional while making the failure cleanup a no-op
		// for real Vulkan resources.
		renderer.device = 0
		renderer.swapchain = 0
		renderer.fence = 0
		renderer.api.queuePresent = func(uintptr, *vkPresentInfo) int32 {
			presents++
			return vkSuccess
		}
		return renderer, nil
	}

	var acked []uint32
	callbackAttempts := 0
	newApp := func() (*waylandApplication, func()) {
		var destroyed []uintptr
		app := newInitialConfigureTestApplication(&destroyed)
		var configureData unsafe.Pointer
		app.api.proxyAddListener = func(proxy uintptr, _ uintptr, data unsafe.Pointer) int32 {
			if proxy == 20 {
				configureData = data
			}
			return 0
		}
		app.api.displayFlush = func(uintptr) int32 { return 0 }
		baseMarshal := app.api.proxyMarshalFlags
		app.api.proxyMarshalFlags = func(proxy uintptr, opcode uint32, interfacePtr uintptr, version uint32, flags uint32, args ...any) uintptr {
			if proxy == 10 && isWaylandFrameTestRequest(opcode, interfacePtr) {
				callbackAttempts++
				if callbackAttempts == 1 {
					return 0
				}
			}
			if opcode == xdgSurfaceAckConfigureOpcode {
				acked = append(acked, args[0].(uint32))
			}
			return baseMarshal(proxy, opcode, interfacePtr, version, flags, args...)
		}
		return app, func() {
			if configureData == nil {
				t.Fatal("xdg_surface listener was not installed before initial configure")
			}
			puregoInvokeCallback3(rawXdgSurfaceListener[0], configureData, 20, 41)
		}
	}

	app, configure := newApp()
	win, err := app.openWindow(platform.Config{Width: 400, Height: 300})
	if err != nil || win == nil {
		t.Fatalf("first openWindow = (%v, %v), want asynchronous success", win, err)
	}
	configure()
	if !errors.Is(app.firstErr, errWaylandFrameCallback) {
		t.Fatalf("first configure error = %v, want callback allocation failure", app.firstErr)
	}
	if len(acked) != 0 || presents != 0 || callbackAttempts != 1 {
		t.Fatalf("failed open advanced: ACK=%v presents=%d attempts=%d", acked, presents, callbackAttempts)
	}

	app, configure = newApp()
	win, err = app.openWindow(platform.Config{Width: 400, Height: 300})
	if err != nil {
		t.Fatalf("retry openWindow: %v", err)
	}
	configure()
	if app.firstErr != nil {
		t.Fatalf("retry initial configure: %v", app.firstErr)
	}
	defer win.destroy()
	if len(acked) != 1 || acked[0] != 41 || presents != 1 || callbackAttempts != 2 {
		t.Fatalf("retry open: ACK=%v presents=%d attempts=%d", acked, presents, callbackAttempts)
	}
	if win.configure.serial != 0 || !win.framePending || win.frameCallback != 99 || win.surfaceState != waylandSurfaceMapped {
		t.Fatalf("retry transaction: serial=%d pending=%v callback=%d state=%v", win.configure.serial, win.framePending, win.frameCallback, win.surfaceState)
	}
}

func TestWaylandHiddenRedrawWaitsForRemapConfigure(t *testing.T) {
	var commits int
	var nilAttachments int
	var acked []uint32
	api := &waylandAPI{
		proxyMarshalFlags: func(_ uintptr, opcode uint32, _ uintptr, _, _ uint32, args ...any) uintptr {
			switch opcode {
			case wlSurfaceCommitOpcode:
				commits++
			case wlSurfaceAttachOpcode:
				if len(args) > 0 && args[0] == uintptr(0) {
					nilAttachments++
				}
			case xdgSurfaceAckConfigureOpcode:
				if len(args) > 0 {
					acked = append(acked, args[0].(uint32))
				}
			}
			return 0
		},
		displayFlush: func(uintptr) int32 { return 0 },
	}
	installWaylandFrameTestCallbacks(api)
	app := &waylandApplication{api: api}
	win := &waylandWindow{
		app: app, surface: 1, xdgSurface: 2,
		width: 400, height: 300, savedLogicalW: 400, savedLogicalH: 300,
		surfaceState: waylandSurfaceMapped, needsRedraw: true,
	}
	win.storeScale(1)
	redraws := 0
	presents := 0
	state := &waylandWindowState{
		app: app, win: win, renderer: &vulkanRenderer{},
		config: platform.Config{
			OnRedraw: func() (platform.Frame, error) {
				redraws++
				return platform.Frame{}, nil
			},
		},
	}
	state.router.present = func(platform.Frame) error {
		presents++
		return nil
	}
	win.state = state

	win.Hide()
	win.renderFrame()
	if redraws != 0 || presents != 0 {
		t.Fatalf("hidden redraw called OnRedraw %d times and present %d times", redraws, presents)
	}
	if !win.needsRedraw {
		t.Fatal("hidden redraw discarded the pending content update")
	}

	win.Show()
	if commits != 2 || nilAttachments != 1 {
		t.Fatalf("Hide/Show commits = %d, nil attachments = %d; want 2 and 1", commits, nilAttachments)
	}
	if win.surfaceState != waylandSurfaceAwaitInitialConfigure {
		t.Fatal("Show did not enter the remap configure wait")
	}
	win.renderFrame() // A Waker must not bypass the new configure.
	if redraws != 0 || presents != 0 {
		t.Fatalf("Show presented before configure: redraws=%d presents=%d", redraws, presents)
	}

	win.handleXdgSurfaceConfigure(77)
	if redraws != 1 || presents != 1 {
		t.Fatalf("configure produced redraws=%d presents=%d, want one each", redraws, presents)
	}
	if len(acked) != 1 || acked[0] != 77 {
		t.Fatalf("acknowledged serials = %v, want [77]", acked)
	}
	if win.surfaceState != waylandSurfaceMapped {
		t.Fatal("configure did not complete the remap")
	}
	win.Show()
	if commits != 2 {
		t.Fatalf("repeated Show added a commit: commits = %d, want 2", commits)
	}
}

// A compositor may wait for a buffer before it configures a remapped surface.
// Show must not leave the window hidden forever when no configure arrives.
func TestWaylandShowRemapsWhenNoConfigureArrives(t *testing.T) {
	var acked []uint32
	api := &waylandAPI{
		proxyMarshalFlags: func(_ uintptr, opcode uint32, _ uintptr, _, _ uint32, args ...any) uintptr {
			if opcode == xdgSurfaceAckConfigureOpcode {
				acked = append(acked, args[0].(uint32))
			}
			return 0
		},
		proxyDestroy: func(uintptr) {},
		displayFlush: func(uintptr) int32 { return 0 },
	}
	installWaylandFrameTestCallbacks(api)
	app := &waylandApplication{api: api}
	app.globals.weston = true
	clock := time.Unix(1000, 0)
	app.now = func() time.Time { return clock }
	resizes := 0
	presents := 0
	win := &waylandWindow{
		app: app, surface: 1, xdgSurface: 2,
		width: 400, height: 300, savedLogicalW: 400, savedLogicalH: 300,
		configure:    waylandConfigureTransaction{width: 400, height: 300},
		surfaceState: waylandSurfaceMapped,
		mappedOnce:   true,
	}
	win.storeScale(1)
	win.state = &waylandWindowState{
		app: app, win: win, renderer: newWaylandPresentTestRenderer(400, 300),
		config: platform.Config{
			OnResize: func(platform.Size) (platform.Frame, error) {
				resizes++
				return platform.Frame{}, nil
			},
		},
	}
	win.state.router.present = func(platform.Frame) error {
		presents++
		return nil
	}
	app.windows = []*waylandWindowState{win.state}
	// processDeadlines also syncs the desktop watcher; keep it from dialing the
	// session bus from a unit test.
	platform.SetAppearanceObserver(nil)

	win.Hide()
	win.Show()
	if win.surfaceState != waylandSurfaceAwaitInitialConfigure || win.remapDeadline.IsZero() {
		t.Fatalf("Show did not start the bounded remap wait: state=%v deadline=%v", win.surfaceState, win.remapDeadline)
	}
	if got := app.pollTimeout(clock); got != int32(waylandRemapConfigureGrace/time.Millisecond) {
		t.Fatalf("poll timeout = %dms, want the remap grace", got)
	}

	app.processDeadlines(clock.Add(waylandRemapConfigureGrace / 2))
	if presents != 0 || win.surfaceState != waylandSurfaceAwaitInitialConfigure {
		t.Fatalf("remap expired before the grace period: presents=%d state=%v", presents, win.surfaceState)
	}

	app.processDeadlines(clock.Add(waylandRemapConfigureGrace + time.Millisecond))
	if presents != 1 || resizes != 1 || win.surfaceState != waylandSurfaceMapped {
		t.Fatalf("no-configure remap presents=%d resizes=%d state=%v; want 1, 1, mapped", presents, resizes, win.surfaceState)
	}
	if len(acked) != 0 {
		t.Fatalf("no-configure remap acknowledged a serial: %v", acked)
	}
	if got := app.pollTimeout(clock.Add(waylandRemapConfigureGrace + time.Millisecond)); got != -1 {
		t.Fatalf("poll timeout after the fallback = %dms, want none", got)
	}

	// A configure that arrives after the fallback still drives the normal
	// resize transaction and is acknowledged with its own serial.
	win.onFrameDone(99, 0)
	win.configure.width, win.configure.height = 800, 600
	win.handleXdgSurfaceConfigure(99)
	if presents != 2 || resizes != 2 || len(acked) != 1 || acked[0] != 99 {
		t.Fatalf("late configure presents=%d resizes=%d acked=%v; want 2, 2, [99]", presents, resizes, acked)
	}
	if win.width != 800 || win.height != 600 {
		t.Fatalf("late configure size = %dx%d, want 800x600", win.width, win.height)
	}
}

// A window that has never presented must still wait for its first configure:
// the fallback only applies to remaps of an already mapped surface.
func TestWaylandShowWithoutPresentDoesNotArmRemapDeadline(t *testing.T) {
	app := &waylandApplication{}
	app.globals.weston = true
	clock := time.Unix(1000, 0)
	app.now = func() time.Time { return clock }
	win := &waylandWindow{app: app, surfaceState: waylandSurfaceUnmapped}
	win.state = &waylandWindowState{app: app, win: win}
	app.windows = []*waylandWindowState{win.state}
	platform.SetAppearanceObserver(nil)

	win.Show()
	if win.surfaceState != waylandSurfaceAwaitInitialConfigure {
		t.Fatalf("Show state = %v, want awaiting the initial configure", win.surfaceState)
	}
	if !win.remapDeadline.IsZero() {
		t.Fatalf("never-presented Show armed a remap deadline: %v", win.remapDeadline)
	}
	if got := app.pollTimeout(clock); got != -1 {
		t.Fatalf("poll timeout = %dms, want none", got)
	}
	app.processDeadlines(clock.Add(time.Hour))
	if win.surfaceState != waylandSurfaceAwaitInitialConfigure {
		t.Fatalf("never-presented window mapped without a configure: %v", win.surfaceState)
	}
}

// Hide and Close must neutralize a remap that is still waiting for its grace.
func TestWaylandRemapDeadlineClearedByHideAndClose(t *testing.T) {
	app := &waylandApplication{}
	app.globals.weston = true
	clock := time.Unix(1000, 0)
	app.now = func() time.Time { return clock }
	win := &waylandWindow{app: app, surfaceState: waylandSurfaceUnmapped, mappedOnce: true}
	win.state = &waylandWindowState{app: app, win: win}
	app.windows = []*waylandWindowState{win.state}

	win.Show()
	if win.remapDeadline.IsZero() {
		t.Fatal("Show did not arm the remap deadline")
	}
	win.Hide()
	if win.surfaceState != waylandSurfaceUnmapped || !win.remapDeadline.IsZero() {
		t.Fatalf("Hide left state=%v deadline=%v", win.surfaceState, win.remapDeadline)
	}
	app.expireRemapDeadlines(clock.Add(time.Second))
	if win.surfaceState != waylandSurfaceUnmapped {
		t.Fatalf("expired deadline remapped a hidden window: %v", win.surfaceState)
	}

	win.Show()
	win.closed.Store(true)
	app.expireRemapDeadlines(clock.Add(time.Second))
	if win.surfaceState != waylandSurfaceAwaitInitialConfigure {
		t.Fatalf("expired deadline mapped a closed window: %v", win.surfaceState)
	}
	win.closed.Store(false)
}

// pollTimeout must wake for the earliest remap grace, and only that window may
// fall back when its deadline passes.
func TestWaylandRemapDeadlineUsesEarliestWindow(t *testing.T) {
	app := &waylandApplication{}
	app.globals.weston = true
	clock := time.Unix(1000, 0)
	app.now = func() time.Time { return clock }
	later := &waylandWindow{app: app, surfaceState: waylandSurfaceAwaitInitialConfigure, mappedOnce: true, remapDeadline: clock.Add(200 * time.Millisecond)}
	sooner := &waylandWindow{app: app, surfaceState: waylandSurfaceAwaitInitialConfigure, mappedOnce: true, remapDeadline: clock.Add(50 * time.Millisecond)}
	app.windows = []*waylandWindowState{{app: app, win: later}, {app: app, win: sooner}}
	platform.SetAppearanceObserver(nil)

	if got := app.pollTimeout(clock); got != 50 {
		t.Fatalf("poll timeout = %dms, want 50ms", got)
	}
	app.expireRemapDeadlines(clock.Add(60 * time.Millisecond))
	if sooner.surfaceState != waylandSurfaceMapped {
		t.Fatalf("due window state = %v, want mapped", sooner.surfaceState)
	}
	if later.surfaceState != waylandSurfaceAwaitInitialConfigure || later.remapDeadline.IsZero() {
		t.Fatalf("not-yet-due window state=%v deadline=%v", later.surfaceState, later.remapDeadline)
	}
}

// Conformant compositors (everything except Weston) must get their remap
// configure: attaching a buffer early is xdg_surface.unconfigured_buffer.
func TestWaylandRemapFallbackRequiresWeston(t *testing.T) {
	app := &waylandApplication{}
	clock := time.Unix(1000, 0)
	app.now = func() time.Time { return clock }
	win := &waylandWindow{app: app, surfaceState: waylandSurfaceUnmapped, mappedOnce: true}
	win.state = &waylandWindowState{app: app, win: win}
	app.windows = []*waylandWindowState{win.state}
	platform.SetAppearanceObserver(nil)

	win.Show()
	if win.surfaceState != waylandSurfaceAwaitInitialConfigure {
		t.Fatalf("Show state = %v, want awaiting the remap configure", win.surfaceState)
	}
	if !win.remapDeadline.IsZero() {
		t.Fatalf("non-Weston Show armed the remap fallback: %v", win.remapDeadline)
	}
	if got := app.pollTimeout(clock); got != -1 {
		t.Fatalf("poll timeout = %dms, want none", got)
	}
	app.processDeadlines(clock.Add(time.Hour))
	if win.surfaceState != waylandSurfaceAwaitInitialConfigure {
		t.Fatalf("non-Weston window mapped without a configure: %v", win.surfaceState)
	}
}

// Weston retains the configured toplevel state across an unmap, so the
// fallback must present at the retained configure, not re-derive a size: a
// smaller buffer would violate the configured maximized geometry.
func TestWaylandRemapFallbackKeepsRetainedConfigure(t *testing.T) {
	api := &waylandAPI{
		proxyMarshalFlags: func(uintptr, uint32, uintptr, uint32, uint32, ...any) uintptr { return 0 },
		proxyDestroy:      func(uintptr) {},
		displayFlush:      func(uintptr) int32 { return 0 },
	}
	installWaylandFrameTestCallbacks(api)
	app := &waylandApplication{api: api}
	app.globals.weston = true
	clock := time.Unix(1000, 0)
	app.now = func() time.Time { return clock }
	win := &waylandWindow{
		app: app, surface: 1, surfaceState: waylandSurfaceMapped, mappedOnce: true,
		maximized: true, width: 800, height: 600,
		savedLogicalW: 400, savedLogicalH: 300,
		configure: waylandConfigureTransaction{width: 800, height: 600},
	}
	win.storeScale(1)
	win.state = &waylandWindowState{app: app, win: win, renderer: newWaylandPresentTestRenderer(800, 600)}
	win.state.router.present = func(platform.Frame) error { return nil }
	app.windows = []*waylandWindowState{win.state}

	win.Hide()
	win.Show()
	win.remapWithoutConfigure()
	if !win.maximized || win.width != 800 || win.height != 600 {
		t.Fatalf("fallback changed the retained state: maximized=%v size=%dx%d", win.maximized, win.width, win.height)
	}
	if win.surfaceState != waylandSurfaceMapped {
		t.Fatalf("fallback state = %v, want mapped", win.surfaceState)
	}
}

func TestWaylandCloseWhileAwaitingRemapConfigure(t *testing.T) {
	api := &waylandAPI{
		proxyMarshalFlags: func(uintptr, uint32, uintptr, uint32, uint32, ...any) uintptr { return 0 },
		proxyDestroy:      func(uintptr) {},
		displayFlush:      func(uintptr) int32 { return 0 },
	}
	app := &waylandApplication{api: api}
	win := &waylandWindow{
		app: app, surface: 1, xdgSurface: 2, xdgToplevel: 3,
		surfaceState: waylandSurfaceUnmapped,
	}
	win.state = &waylandWindowState{app: app, win: win, renderer: &vulkanRenderer{}}

	win.Show()
	if win.surfaceState != waylandSurfaceAwaitInitialConfigure {
		t.Fatal("Show did not enter the remap configure wait")
	}
	win.Close()
	if !win.closed.Load() || win.state != nil {
		t.Fatal("Close did not destroy the window awaiting remap configure")
	}
	if win.framePending {
		t.Fatal("Close left a frame callback pending")
	}
}

func TestWaylandHiddenRemapResizeWaitsForShow(t *testing.T) {
	var acked []uint32
	commits := 0
	api := &waylandAPI{
		proxyMarshalFlags: func(_ uintptr, opcode uint32, _ uintptr, _, _ uint32, args ...any) uintptr {
			switch opcode {
			case wlSurfaceCommitOpcode:
				commits++
			case xdgSurfaceAckConfigureOpcode:
				acked = append(acked, args[0].(uint32))
			}
			return 0
		},
		displayFlush: func(uintptr) int32 { return 0 },
	}
	installWaylandFrameTestCallbacks(api)
	app := &waylandApplication{api: api}
	win := &waylandWindow{
		app: app, surface: 1, xdgSurface: 2,
		width: 400, height: 300, savedLogicalW: 400, savedLogicalH: 300,
		configure:    waylandConfigureTransaction{width: 400, height: 300},
		surfaceState: waylandSurfaceMapped,
	}
	win.storeScale(1)
	resizeCallbacks := 0
	presents := 0
	win.state = &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			OnResize: func(platform.Size) (platform.Frame, error) {
				resizeCallbacks++
				return platform.Frame{}, nil
			},
		},
	}
	win.state.router.present = func(platform.Frame) error {
		presents++
		return nil
	}

	win.Hide()
	win.Show()
	win.Hide() // The next configure must not remap this hidden surface.
	win.configure.width, win.configure.height = 800, 600
	win.handleXdgSurfaceConfigure(88)
	if resizeCallbacks != 0 || presents != 0 || len(acked) != 0 || commits != 3 {
		t.Fatalf("hidden configure advanced: resize=%d present=%d ack=%v commits=%d", resizeCallbacks, presents, acked, commits)
	}
	if win.configure.serial != 88 {
		t.Fatalf("hidden configure serial = %d, want retained serial 88", win.configure.serial)
	}

	win.Show()
	win.handleXdgSurfaceConfigure(89)
	if resizeCallbacks != 1 || presents != 1 {
		t.Fatalf("visible remap resize = %d callbacks, %d presents; want one each", resizeCallbacks, presents)
	}
	if len(acked) != 1 || acked[0] != 89 {
		t.Fatalf("acknowledged serials = %v, want [89]", acked)
	}
	if commits != 5 {
		t.Fatalf("commit count = %d, want 5 after the visible buffer commit", commits)
	}
	if win.width != 800 || win.height != 600 {
		t.Fatalf("remapped size = %dx%d, want 800x600", win.width, win.height)
	}
}

func newWaylandPresentTestRenderer(width, height uint32) *vulkanRenderer {
	renderer := &vulkanRenderer{
		device: 1, queue: 2, swapchain: 3, width: width, height: height,
		commands: []uintptr{4}, fence: 5,
		api: &vulkanAPI{
			waitForFences: func(uintptr, uint32, *uintptr, uint32, uint64) int32 { return vkSuccess },
			acquireNextImage: func(_, _ uintptr, _ uint64, _, _ uintptr, image *uint32) int32 {
				*image = 0
				return vkSuccess
			},
			resetFences:      func(uintptr, uint32, *uintptr) int32 { return vkSuccess },
			queueSubmit:      func(uintptr, uint32, *vkSubmitInfo, uintptr) int32 { return vkSuccess },
			queuePresent:     func(uintptr, *vkPresentInfo) int32 { return vkSuccess },
			deviceWaitIdle:   func(uintptr) int32 { return vkSuccess },
			destroySwapchain: func(uintptr, uintptr, unsafe.Pointer) {},
			destroyFence:     func(uintptr, uintptr, unsafe.Pointer) {},
			destroyDevice:    func(uintptr, unsafe.Pointer) {},
		},
	}
	installWaylandFakeSwapchain(renderer.api)
	return renderer
}

// installWaylandFakeSwapchain supplies the Vulkan calls swapchain recreation
// needs so scale and resize transactions can run without a driver.
func installWaylandFakeSwapchain(api *vulkanAPI) {
	api.getSurfaceCapabilities = func(_, _ uintptr, capabilities *vkSurfaceCapabilities) int32 {
		capabilities.Current = vkExtent2D{Width: math.MaxUint32, Height: math.MaxUint32}
		capabilities.Minimum = vkExtent2D{Width: 1, Height: 1}
		capabilities.Maximum = vkExtent2D{Width: 16384, Height: 16384}
		capabilities.MinImages = 2
		capabilities.MaxImages = 8
		capabilities.CompositeAlpha = vkCompositeOpaque
		capabilities.Usage = vkImageUsageColorAttachment
		return vkSuccess
	}
	api.getSurfaceFormats = func(_, _ uintptr, count *uint32, formats *vkSurfaceFormat) int32 {
		*count = 1
		if formats != nil {
			*formats = vkSurfaceFormat{Format: vkFormatBGRA8, ColorSpace: vkColorSpaceSRGB}
		}
		return vkSuccess
	}
	api.createSwapchain = func(_ uintptr, _ *vkSwapchainCreateInfo, _ unsafe.Pointer, swapchain *uintptr) int32 {
		*swapchain = 99
		return vkSuccess
	}
	api.getSwapchainImages = func(_, _ uintptr, count *uint32, images *uintptr) int32 {
		*count = 1
		if images != nil {
			*images = 101
		}
		return vkSuccess
	}
	api.createImageView = func(_ uintptr, _ *vkImageViewCreateInfo, _ unsafe.Pointer, view *uintptr) int32 {
		*view = 201
		return vkSuccess
	}
	api.createRenderPass = func(_ uintptr, _ *vkRenderPassCreateInfo, _ unsafe.Pointer, renderPass *uintptr) int32 {
		*renderPass = 301
		return vkSuccess
	}
	api.createFramebuffer = func(_ uintptr, _ *vkFramebufferCreateInfo, _ unsafe.Pointer, framebuffer *uintptr) int32 {
		*framebuffer = 401
		return vkSuccess
	}
	api.createCommandPool = func(_ uintptr, _ *vkCommandPoolCreateInfo, _ unsafe.Pointer, pool *uintptr) int32 {
		*pool = 501
		return vkSuccess
	}
	api.allocateCommandBuffers = func(_ uintptr, _ *vkCommandBufferAllocateInfo, buffers *uintptr) int32 {
		*buffers = 601
		return vkSuccess
	}
	api.createPipelineLayout = func(_ uintptr, _ *vkPipelineLayoutCreateInfo, _ unsafe.Pointer, layout *uintptr) int32 {
		*layout = 701
		return vkSuccess
	}
	api.createShaderModule = func(_ uintptr, _ *vkShaderModuleCreateInfo, _ unsafe.Pointer, module *uintptr) int32 {
		*module = 801
		return vkSuccess
	}
	api.createGraphicsPipelines = func(_, _ uintptr, _ uint32, _ *vkGraphicsPipelineCreateInfo, _ unsafe.Pointer, pipeline *uintptr) int32 {
		*pipeline = 901
		return vkSuccess
	}
	api.beginCommandBuffer = func(uintptr, *vkCommandBufferBeginInfo) int32 { return vkSuccess }
	api.cmdBeginRenderPass = func(uintptr, *vkRenderPassBeginInfo, uint32) {}
	api.cmdEndRenderPass = func(uintptr) {}
	api.endCommandBuffer = func(uintptr) int32 { return vkSuccess }
	api.destroyShaderModule = func(uintptr, uintptr, unsafe.Pointer) {}
	api.destroyPipeline = func(uintptr, uintptr, unsafe.Pointer) {}
	api.destroyPipelineLayout = func(uintptr, uintptr, unsafe.Pointer) {}
	api.destroyRenderPass = func(uintptr, uintptr, unsafe.Pointer) {}
	api.destroyFramebuffer = func(uintptr, uintptr, unsafe.Pointer) {}
	api.destroyImageView = func(uintptr, uintptr, unsafe.Pointer) {}
	api.destroyCommandPool = func(uintptr, uintptr, unsafe.Pointer) {}
}

func newWaylandPresentTestAPI(acked *[]uint32, frameRequests *int) *waylandAPI {
	return &waylandAPI{
		proxyMarshalFlags: func(_ uintptr, opcode uint32, interfacePtr uintptr, _, _ uint32, args ...any) uintptr {
			if isWaylandFrameTestRequest(opcode, interfacePtr) {
				*frameRequests++
				return 99
			}
			if opcode == xdgSurfaceAckConfigureOpcode {
				*acked = append(*acked, args[0].(uint32))
			}
			return 0
		},
		proxyAddListener: func(uintptr, uintptr, unsafe.Pointer) int32 { return 0 },
		proxyDestroy:     func(uintptr) {},
		displayFlush:     func(uintptr) int32 { return 0 },
	}
}

func isWaylandFrameTestRequest(opcode uint32, interfacePtr uintptr) bool {
	return opcode == wlSurfaceFrameOpcode && interfacePtr == uintptr(unsafe.Pointer(&wlCallbackInterface))
}

func installWaylandFrameTestCallbacks(api *waylandAPI) {
	if api == nil {
		return
	}
	previousMarshal := api.proxyMarshalFlags
	api.proxyMarshalFlags = func(proxy uintptr, opcode uint32, interfacePtr uintptr, version uint32, flags uint32, args ...any) uintptr {
		result := uintptr(0)
		if previousMarshal != nil {
			result = previousMarshal(proxy, opcode, interfacePtr, version, flags, args...)
		}
		if result == 0 && isWaylandFrameTestRequest(opcode, interfacePtr) {
			return 99
		}
		return result
	}
	if api.proxyAddListener == nil {
		api.proxyAddListener = func(uintptr, uintptr, unsafe.Pointer) int32 { return 0 }
	}
	if api.proxyDestroy == nil {
		api.proxyDestroy = func(uintptr) {}
	}
}

func TestWaylandInputPresentUsesFrameTransaction(t *testing.T) {
	var acked []uint32
	frameRequests := 0
	app := &waylandApplication{api: newWaylandPresentTestAPI(&acked, &frameRequests)}
	win := &waylandWindow{app: app, surface: 1, xdgSurface: 2, surfaceState: waylandSurfaceMapped}
	renderer := newWaylandPresentTestRenderer(400, 300)
	changed := true
	win.state = &waylandWindowState{
		app: app, win: win, renderer: renderer,
		applyFrame: func(platform.Frame) (bool, error) { return changed, nil },
	}

	if err := win.presentInputFrame(platform.Frame{}); err != nil {
		t.Fatalf("present input frame: %v", err)
	}
	if renderer.submissions != 1 || frameRequests != 1 || !win.framePending {
		t.Fatalf("dirty input submissions=%d frameRequests=%d pending=%v", renderer.submissions, frameRequests, win.framePending)
	}
	if win.needsRedraw {
		t.Fatal("presented input left a redraw request pending")
	}

	win.onFrameDone(99, 0)
	changed = false
	if err := win.presentInputFrame(platform.Frame{}); err != nil {
		t.Fatalf("present clean input frame: %v", err)
	}
	if renderer.submissions != 1 || frameRequests != 1 || win.framePending {
		t.Fatal("clean input produced another commit or callback")
	}
}

func TestWaylandInputPresentWaitsForWindowLifecycle(t *testing.T) {
	for _, test := range []struct {
		name         string
		state        waylandSurfaceState
		framePending bool
	}{
		{name: "hidden", state: waylandSurfaceUnmapped},
		{name: "awaiting remap configure", state: waylandSurfaceAwaitInitialConfigure},
		{name: "previous frame pending", state: waylandSurfaceMapped, framePending: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var acked []uint32
			frameRequests := 0
			app := &waylandApplication{api: newWaylandPresentTestAPI(&acked, &frameRequests)}
			win := &waylandWindow{
				app: app, surface: 1, xdgSurface: 2, surfaceState: test.state,
				framePending: test.framePending,
			}
			renderer := newWaylandPresentTestRenderer(400, 300)
			win.state = &waylandWindowState{
				app: app, win: win, renderer: renderer,
				applyFrame: func(platform.Frame) (bool, error) { return true, nil },
			}

			if err := win.presentInputFrame(platform.Frame{}); err != nil {
				t.Fatalf("present input frame: %v", err)
			}
			if renderer.submissions != 0 || frameRequests != 0 {
				t.Fatalf("blocked input committed: submissions=%d frameRequests=%d", renderer.submissions, frameRequests)
			}
			if !win.needsRedraw || win.framePending != test.framePending {
				t.Fatalf("blocked input state needsRedraw=%v framePending=%v", win.needsRedraw, win.framePending)
			}
		})
	}
}

func TestWaylandFocusLossDoesNotPresentWhileAwaitingRemap(t *testing.T) {
	var acked []uint32
	frameRequests := 0
	app := &waylandApplication{api: newWaylandPresentTestAPI(&acked, &frameRequests)}
	win := &waylandWindow{
		app: app, surface: 1, xdgSurface: 2, surfaceState: waylandSurfaceAwaitInitialConfigure,
		activated: true,
	}
	renderer := newWaylandPresentTestRenderer(400, 300)
	win.state = &waylandWindowState{
		app: app, win: win, renderer: renderer,
		config: platform.Config{OnFocusLost: func() (platform.Frame, error) {
			return platform.Frame{}, nil
		}},
		applyFrame: func(platform.Frame) (bool, error) { return true, nil },
	}
	win.state.router.present = win.presentInputFrame

	win.handleXdgToplevelConfigure(0, 0, nil)
	if renderer.submissions != 0 || frameRequests != 0 || len(acked) != 0 {
		t.Fatalf("focus loss bypassed remap: submissions=%d frameRequests=%d ack=%v", renderer.submissions, frameRequests, acked)
	}
	if !win.needsRedraw {
		t.Fatal("blocked focus-loss update did not request a later redraw")
	}
}

func TestWaylandInputPresentHonorsPendingConfigure(t *testing.T) {
	var acked []uint32
	frameRequests := 0
	app := &waylandApplication{api: newWaylandPresentTestAPI(&acked, &frameRequests)}
	win := &waylandWindow{
		app: app, surface: 1, xdgSurface: 2, surfaceState: waylandSurfaceMapped,
		configure: waylandConfigureTransaction{serial: 41}, forcePresent: true,
	}
	renderer := newWaylandPresentTestRenderer(400, 300)
	applyCalls := 0
	win.state = &waylandWindowState{
		app: app, win: win, renderer: renderer,
		config: platform.Config{OnRedraw: func() (platform.Frame, error) {
			return platform.Frame{}, nil
		}},
		applyFrame: func(platform.Frame) (bool, error) {
			applyCalls++
			return applyCalls == 1, nil
		},
	}

	if err := win.presentInputFrame(platform.Frame{}); err != nil {
		t.Fatalf("present input with configure: %v", err)
	}
	if applyCalls != 2 || renderer.submissions != 1 || frameRequests != 1 {
		t.Fatalf("configure input apply=%d submissions=%d frameRequests=%d", applyCalls, renderer.submissions, frameRequests)
	}
	if len(acked) != 1 || acked[0] != 41 || win.configure.serial != 0 {
		t.Fatalf("configure ACK = %v, pending serial = %d", acked, win.configure.serial)
	}
	if win.forcePresent || win.needsRedraw {
		t.Fatal("successful configure input retained pending flags")
	}
}

func TestWaylandCleanRedrawDoesNotRequestFrame(t *testing.T) {
	var acked []uint32
	frameRequests := 0
	app := &waylandApplication{api: newWaylandPresentTestAPI(&acked, &frameRequests)}
	win := &waylandWindow{app: app, surface: 1, xdgSurface: 2, surfaceState: waylandSurfaceMapped}
	renderer := newWaylandPresentTestRenderer(400, 300)
	win.state = &waylandWindowState{
		app: app, win: win, renderer: renderer,
		config:     platform.Config{OnRedraw: func() (platform.Frame, error) { return platform.Frame{}, nil }},
		applyFrame: func(platform.Frame) (bool, error) { return false, nil },
	}

	win.renderFrame()
	if renderer.submissions != 0 || frameRequests != 0 || win.framePending {
		t.Fatalf("clean redraw committed: submissions=%d frameRequests=%d pending=%v", renderer.submissions, frameRequests, win.framePending)
	}
	if win.needsRedraw {
		t.Fatal("clean redraw left a redraw request pending")
	}
}

func TestWaylandCleanGeometryChangePresents(t *testing.T) {
	var acked []uint32
	frameRequests := 0
	app := &waylandApplication{api: newWaylandPresentTestAPI(&acked, &frameRequests)}
	win := &waylandWindow{app: app, surface: 1, xdgSurface: 2, surfaceState: waylandSurfaceMapped, geometryDirty: true}
	renderer := newWaylandPresentTestRenderer(400, 300)
	win.state = &waylandWindowState{
		app: app, win: win, renderer: renderer,
		config:     platform.Config{OnRedraw: func() (platform.Frame, error) { return platform.Frame{}, nil }},
		applyFrame: func(platform.Frame) (bool, error) { return false, nil },
	}

	win.renderFrame()
	if renderer.submissions != 1 || frameRequests != 1 {
		t.Fatalf("geometry-only change submissions=%d frameRequests=%d, want one each", renderer.submissions, frameRequests)
	}
	if win.geometryDirty {
		t.Fatal("committed geometry change remained dirty")
	}
}

func TestWaylandForcedCleanConfigurePresents(t *testing.T) {
	var acked []uint32
	frameRequests := 0
	app := &waylandApplication{api: newWaylandPresentTestAPI(&acked, &frameRequests)}
	win := &waylandWindow{
		app: app, surface: 1, xdgSurface: 2, surfaceState: waylandSurfaceMapped,
		configure: waylandConfigureTransaction{serial: 41}, forcePresent: true,
	}
	renderer := newWaylandPresentTestRenderer(400, 300)
	win.state = &waylandWindowState{
		app: app, win: win, renderer: renderer,
		config:     platform.Config{OnRedraw: func() (platform.Frame, error) { return platform.Frame{}, nil }},
		applyFrame: func(platform.Frame) (bool, error) { return false, nil },
	}

	win.renderFrame()
	if renderer.submissions != 1 || frameRequests != 1 {
		t.Fatalf("forced clean configure submissions=%d frameRequests=%d, want one each", renderer.submissions, frameRequests)
	}
	if len(acked) != 1 || acked[0] != 41 || win.configure.serial != 0 {
		t.Fatalf("configure ACK = %v, pending serial = %d", acked, win.configure.serial)
	}
	if win.forcePresent {
		t.Fatal("successful present retained the force flag")
	}
	win.onFrameDone(99, 0)
	win.needsRedraw = true
	win.renderFrame()
	if renderer.submissions != 1 || frameRequests != 1 {
		t.Fatal("clean redraw after frame completion did not remain idle")
	}
}

func TestWaylandCleanResizePresentsOnce(t *testing.T) {
	var acked []uint32
	frameRequests := 0
	app := &waylandApplication{api: newWaylandPresentTestAPI(&acked, &frameRequests)}
	win := &waylandWindow{
		app: app, surface: 1, xdgSurface: 2, surfaceState: waylandSurfaceMapped,
		width: 400, height: 300, savedLogicalW: 400, savedLogicalH: 300,
		configure: waylandConfigureTransaction{width: 800, height: 600, serial: 42}, forcePresent: true,
	}
	win.storeScale(1)
	renderer := newWaylandPresentTestRenderer(800, 600)
	resizeCallbacks := 0
	win.state = &waylandWindowState{
		app: app, win: win, renderer: renderer,
		config: platform.Config{OnResize: func(platform.Size) (platform.Frame, error) {
			resizeCallbacks++
			return platform.Frame{}, nil
		}},
		applyFrame: func(platform.Frame) (bool, error) { return false, nil },
	}

	win.applyPendingResize()
	if resizeCallbacks != 1 || renderer.submissions != 1 || frameRequests != 1 {
		t.Fatalf("clean resize callbacks=%d submissions=%d frameRequests=%d, want one each", resizeCallbacks, renderer.submissions, frameRequests)
	}
	if len(acked) != 1 || acked[0] != 42 {
		t.Fatalf("resize ACK = %v, want [42]", acked)
	}
}

func TestWaylandCleanScalePresentsOnceWithoutRedraw(t *testing.T) {
	var acked []uint32
	frameRequests := 0
	app := &waylandApplication{api: newWaylandPresentTestAPI(&acked, &frameRequests)}
	win := &waylandWindow{
		app: app, surface: 1, xdgSurface: 2, surfaceState: waylandSurfaceMapped,
		width: 400, height: 300, savedLogicalW: 400, savedLogicalH: 300,
	}
	win.storeScale(1)
	renderer := newWaylandPresentTestRenderer(800, 600)
	resizeCallbacks := 0
	redraws := 0
	win.state = &waylandWindowState{
		app: app, win: win, renderer: renderer,
		config: platform.Config{
			OnResize: func(platform.Size) (platform.Frame, error) {
				resizeCallbacks++
				return platform.Frame{}, nil
			},
			OnRedraw: func() (platform.Frame, error) {
				redraws++
				return platform.Frame{}, nil
			},
		},
		applyFrame: func(platform.Frame) (bool, error) { return false, nil },
	}

	win.applyScale(2)
	if resizeCallbacks != 1 || redraws != 0 || renderer.submissions != 1 || frameRequests != 1 {
		t.Fatalf("clean scale resize=%d redraw=%d submissions=%d frameRequests=%d", resizeCallbacks, redraws, renderer.submissions, frameRequests)
	}
	if win.forcePresent {
		t.Fatal("scale present retained the force flag")
	}
}

func TestWaylandPendingResizePresentsLatestRedraw(t *testing.T) {
	var acked []uint32
	frameRequests := 0
	app := &waylandApplication{api: newWaylandPresentTestAPI(&acked, &frameRequests)}
	win := &waylandWindow{
		app: app, surface: 1, xdgSurface: 2, surfaceState: waylandSurfaceMapped,
		width: 400, height: 300, savedLogicalW: 400, savedLogicalH: 300,
		configure:    waylandConfigureTransaction{width: 800, height: 600, serial: 42},
		framePending: true, frameCallback: 99, needsRedraw: true, forcePresent: true,
	}
	win.storeScale(1)
	renderer := newWaylandPresentTestRenderer(800, 600)
	redraws := 0
	win.state = &waylandWindowState{
		app: app, win: win, renderer: renderer,
		config: platform.Config{OnRedraw: func() (platform.Frame, error) {
			redraws++
			return platform.Frame{}, nil
		}},
		applyFrame: func(platform.Frame) (bool, error) { return true, nil },
	}

	win.applyPendingResize()
	if !win.resizeDeferred || renderer.submissions != 0 || frameRequests != 0 || len(acked) != 0 {
		t.Fatalf("pending resize committed early: deferred=%v submissions=%d frameRequests=%d ack=%v", win.resizeDeferred, renderer.submissions, frameRequests, acked)
	}
	if win.configure.serial != 42 || !win.forcePresent || !win.needsRedraw {
		t.Fatal("pending resize discarded its configure or redraw state")
	}

	win.onFrameDone(99, 0)
	if win.resizeDeferred || renderer.submissions != 1 || frameRequests != 1 {
		t.Fatalf("deferred resize submissions=%d frameRequests=%d deferred=%v", renderer.submissions, frameRequests, win.resizeDeferred)
	}
	if len(acked) != 1 || acked[0] != 42 || redraws != 1 {
		t.Fatalf("deferred resize ack=%v redraws=%d, want [42] and one redraw", acked, redraws)
	}
	if win.needsRedraw {
		t.Fatal("deferred resize did not consume the applied redraw")
	}

	win.onFrameDone(99, 0)
	if redraws != 1 || renderer.submissions != 1 || frameRequests != 1 {
		t.Fatalf("completed resize caused another redraw or present: redraw=%d submissions=%d frameRequests=%d", redraws, renderer.submissions, frameRequests)
	}
}

func TestWaylandContinuousDeferredResizeUsesLatestRedraw(t *testing.T) {
	var acked []uint32
	frameRequests := 0
	app := &waylandApplication{api: newWaylandPresentTestAPI(&acked, &frameRequests)}
	win := &waylandWindow{
		app: app, surface: 1, xdgSurface: 2, surfaceState: waylandSurfaceMapped,
		width: 400, height: 300, savedLogicalW: 400, savedLogicalH: 300,
		framePending: true, frameCallback: 77, needsRedraw: true,
	}
	win.storeScale(1)
	recreations := 0
	renderer := newSuboptimalTestRenderer(vkSuccess, vkSuccess, &recreations)
	renderer.commandPool = 0
	renderer.api.destroyCommandPool = func(uintptr, uintptr, unsafe.Pointer) {}
	renderer.api.destroyPipeline = func(uintptr, uintptr, unsafe.Pointer) {}
	renderer.api.destroyPipelineLayout = func(uintptr, uintptr, unsafe.Pointer) {}
	renderer.api.destroyFramebuffer = func(uintptr, uintptr, unsafe.Pointer) {}
	renderer.api.destroyRenderPass = func(uintptr, uintptr, unsafe.Pointer) {}
	renderer.api.destroyImageView = func(uintptr, uintptr, unsafe.Pointer) {}
	redraws := 0
	var applied []int
	win.state = &waylandWindowState{
		app: app, win: win, renderer: renderer,
		config: platform.Config{OnRedraw: func() (platform.Frame, error) {
			redraws++
			return platform.Frame{}, nil
		}},
		applyFrame: func(frame platform.Frame) (bool, error) {
			applied = append(applied, redraws)
			return true, nil
		},
	}

	for i, size := range []int{800, 1000, 1200} {
		win.handleXdgToplevelConfigure(int32(size), 600, nil)
		win.handleXdgSurfaceConfigure(uint32(42 + i))
		if !win.resizeDeferred || renderer.submissions != uint64(i) || frameRequests != i {
			t.Fatalf("resize %d advanced before its callback: deferred=%v submissions=%d requests=%d", i, win.resizeDeferred, renderer.submissions, frameRequests)
		}
		win.needsRedraw = true // A Waker arrived while the preceding frame was pending.
		win.onFrameDone(77, 0)
		renderer.device = 0 // This mock owns no real library or GPU device.
		if redraws != i+1 || renderer.submissions != uint64(i+1) || frameRequests != i+1 {
			t.Fatalf("resize %d did not apply the latest scene: redraws=%d submissions=%d requests=%d", i, redraws, renderer.submissions, frameRequests)
		}
		if win.needsRedraw || win.resizeDeferred {
			t.Fatalf("resize %d retained completed transaction state", i)
		}
	}
	if len(applied) != 3 || applied[0] != 1 || applied[1] != 2 || applied[2] != 3 {
		t.Fatalf("applied scene generations = %v, want [1 2 3]", applied)
	}
	if len(acked) != 3 || acked[0] != 42 || acked[1] != 43 || acked[2] != 44 {
		t.Fatalf("continuous resize ACKs = %v, want [42 43 44]", acked)
	}
}

func TestWaylandDeferredResizeDoesNotDuplicateResizeRedraw(t *testing.T) {
	var acked []uint32
	frameRequests := 0
	app := &waylandApplication{api: newWaylandPresentTestAPI(&acked, &frameRequests)}
	win := &waylandWindow{
		app: app, surface: 1, xdgSurface: 2, surfaceState: waylandSurfaceMapped,
		width: 400, height: 300, savedLogicalW: 400, savedLogicalH: 300,
		configure:    waylandConfigureTransaction{width: 800, height: 600, serial: 42},
		framePending: true, frameCallback: 77, needsRedraw: true,
	}
	win.storeScale(1)
	renderer := newWaylandPresentTestRenderer(800, 600)
	resizes, redraws, applies := 0, 0, 0
	win.state = &waylandWindowState{
		app: app, win: win, renderer: renderer,
		config: platform.Config{
			OnResize: func(platform.Size) (platform.Frame, error) {
				resizes++
				return platform.Frame{}, nil
			},
			OnRedraw: func() (platform.Frame, error) {
				redraws++
				return platform.Frame{}, nil
			},
		},
		applyFrame: func(platform.Frame) (bool, error) { applies++; return false, nil },
	}

	win.applyPendingResize()
	win.onFrameDone(77, 0)
	if resizes != 1 || redraws != 0 || applies != 1 || renderer.submissions != 1 {
		t.Fatalf("resize scene selection resize=%d redraw=%d apply=%d submissions=%d", resizes, redraws, applies, renderer.submissions)
	}
	if win.needsRedraw {
		t.Fatal("successful resize frame did not consume the queued redraw")
	}
	win.onFrameDone(99, 0)
	if resizes != 1 || redraws != 0 || applies != 1 || renderer.submissions != 1 {
		t.Fatal("completed resize frame duplicated a scene or presentation")
	}
}

func TestWaylandDeferredResizeRedrawApplyFailureClosesWindow(t *testing.T) {
	var acked []uint32
	frameRequests := 0
	app := &waylandApplication{api: newWaylandPresentTestAPI(&acked, &frameRequests)}
	expectedErr := errors.New("deferred redraw apply failed")
	win := &waylandWindow{
		app: app, surface: 1, xdgSurface: 2, surfaceState: waylandSurfaceMapped,
		width: 400, height: 300, savedLogicalW: 400, savedLogicalH: 300,
		configure: waylandConfigureTransaction{width: 800, height: 600, serial: 42}, forcePresent: true,
		framePending: true, frameCallback: 77, needsRedraw: true,
	}
	win.storeScale(1)
	renderer := newWaylandPresentTestRenderer(800, 600)
	redraws := 0
	state := &waylandWindowState{
		app: app, win: win, renderer: renderer,
		config: platform.Config{OnRedraw: func() (platform.Frame, error) {
			redraws++
			return platform.Frame{}, nil
		}},
		applyFrame: func(platform.Frame) (bool, error) { return false, expectedErr },
	}
	win.state = state
	app.windows = []*waylandWindowState{state}
	renderer.device = 0 // This mock owns no real library or GPU device.

	win.applyPendingResize()
	win.onFrameDone(77, 0)
	if !errors.Is(app.firstErr, expectedErr) || redraws != 1 || renderer.submissions != 0 || frameRequests != 0 {
		t.Fatalf("failed deferred redraw err=%v redraws=%d submissions=%d requests=%d", app.firstErr, redraws, renderer.submissions, frameRequests)
	}
	if len(acked) != 0 || win.state != nil || !win.closed.Load() || win.framePending || win.frameCallback != 0 {
		t.Fatalf("failed redraw did not close cleanly: ack=%v state=%v closed=%v pending=%v callback=%d", acked, win.state != nil, win.closed.Load(), win.framePending, win.frameCallback)
	}
	win.onFrameDone(0, 0)
	if redraws != 1 || renderer.submissions != 0 || frameRequests != 0 {
		t.Fatal("a closed window retried its failed redraw")
	}
}

func TestWaylandDeferredResizeRedrawPresentFailureClosesWindow(t *testing.T) {
	var acked []uint32
	frameRequests := 0
	app := &waylandApplication{api: newWaylandPresentTestAPI(&acked, &frameRequests)}
	win := &waylandWindow{
		app: app, surface: 1, xdgSurface: 2, surfaceState: waylandSurfaceMapped,
		width: 400, height: 300, savedLogicalW: 400, savedLogicalH: 300,
		configure: waylandConfigureTransaction{width: 800, height: 600, serial: 42}, forcePresent: true,
		framePending: true, frameCallback: 77, needsRedraw: true,
	}
	win.storeScale(1)
	renderer := newWaylandPresentTestRenderer(800, 600)
	presentCalls := 0
	renderer.api.queuePresent = func(uintptr, *vkPresentInfo) int32 {
		presentCalls++
		if presentCalls == 1 {
			return vkErrorDeviceLost
		}
		return vkSuccess
	}
	redraws := 0
	state := &waylandWindowState{
		app: app, win: win, renderer: renderer,
		config: platform.Config{OnRedraw: func() (platform.Frame, error) {
			redraws++
			return platform.Frame{}, nil
		}},
		applyFrame: func(platform.Frame) (bool, error) { return true, nil },
	}
	win.state = state
	app.windows = []*waylandWindowState{state}
	renderer.device = 0 // This mock owns no real library or GPU device.

	win.applyPendingResize()
	win.onFrameDone(77, 0)
	if app.firstErr == nil || redraws != 1 || presentCalls != 1 || renderer.submissions != 0 {
		t.Fatalf("failed deferred present err=%v redraws=%d presents=%d submissions=%d", app.firstErr, redraws, presentCalls, renderer.submissions)
	}
	if win.state != nil || !win.closed.Load() || win.framePending || win.frameCallback != 0 {
		t.Fatalf("failed present did not close cleanly: state=%v closed=%v pending=%v callback=%d", win.state != nil, win.closed.Load(), win.framePending, win.frameCallback)
	}
	win.onFrameDone(0, 0)
	if redraws != 1 || presentCalls != 1 || renderer.submissions != 0 {
		t.Fatal("a closed window retried its failed presentation")
	}
}

func TestWaylandPendingScaleUsesResizeFrameOnce(t *testing.T) {
	var acked []uint32
	frameRequests := 0
	app := &waylandApplication{api: newWaylandPresentTestAPI(&acked, &frameRequests)}
	win := &waylandWindow{
		app: app, surface: 1, xdgSurface: 2, surfaceState: waylandSurfaceMapped,
		width: 400, height: 300, savedLogicalW: 400, savedLogicalH: 300,
		framePending: true, frameCallback: 99, needsRedraw: true,
	}
	win.storeScale(1)
	renderer := newWaylandPresentTestRenderer(800, 600)
	resizeCallbacks := 0
	redraws := 0
	win.state = &waylandWindowState{
		app: app, win: win, renderer: renderer,
		config: platform.Config{
			OnResize: func(platform.Size) (platform.Frame, error) {
				resizeCallbacks++
				return platform.Frame{}, nil
			},
			OnRedraw: func() (platform.Frame, error) {
				redraws++
				return platform.Frame{}, nil
			},
		},
		applyFrame: func(platform.Frame) (bool, error) { return false, nil },
	}

	win.applyScale(2)
	if !win.resizeDeferred || resizeCallbacks != 0 || renderer.submissions != 0 || frameRequests != 0 {
		t.Fatalf("pending scale advanced early: deferred=%v resize=%d submissions=%d frameRequests=%d", win.resizeDeferred, resizeCallbacks, renderer.submissions, frameRequests)
	}

	win.onFrameDone(99, 0)
	if resizeCallbacks != 1 || redraws != 0 || renderer.submissions != 1 || frameRequests != 1 {
		t.Fatalf("deferred scale resize=%d redraw=%d submissions=%d frameRequests=%d", resizeCallbacks, redraws, renderer.submissions, frameRequests)
	}
	if win.needsRedraw || win.forcePresent {
		t.Fatal("resize frame did not consume the queued update")
	}

	win.onFrameDone(99, 0)
	if resizeCallbacks != 1 || redraws != 0 || renderer.submissions != 1 || frameRequests != 1 {
		t.Fatal("completed scale frame caused a duplicate resize or redraw")
	}
}

func TestWaylandDeferredScaleCompletesAfterRemap(t *testing.T) {
	var acked []uint32
	frameRequests := 0
	app := &waylandApplication{api: newWaylandPresentTestAPI(&acked, &frameRequests)}
	win := &waylandWindow{
		app: app, surface: 1, xdgSurface: 2, surfaceState: waylandSurfaceMapped,
		width: 400, height: 300, savedLogicalW: 400, savedLogicalH: 300,
		framePending: true, frameCallback: 99, needsRedraw: true,
	}
	win.storeScale(1)
	renderer := newWaylandPresentTestRenderer(800, 600)
	resizeCallbacks := 0
	redraws := 0
	win.state = &waylandWindowState{
		app: app, win: win, renderer: renderer,
		config: platform.Config{
			OnResize: func(platform.Size) (platform.Frame, error) {
				resizeCallbacks++
				return platform.Frame{}, nil
			},
			OnRedraw: func() (platform.Frame, error) {
				redraws++
				return platform.Frame{}, nil
			},
		},
		applyFrame: func(platform.Frame) (bool, error) { return false, nil },
	}

	win.applyScale(2)
	win.Hide()
	win.Show()
	if !win.resizeDeferred || win.surfaceState != waylandSurfaceAwaitInitialConfigure {
		t.Fatal("remap did not retain the deferred scale transaction")
	}
	win.configure.width = 400
	win.configure.height = 300
	win.handleXdgSurfaceConfigure(43)
	if win.resizeDeferred || resizeCallbacks != 1 || redraws != 0 || renderer.submissions != 1 || frameRequests != 1 {
		t.Fatalf("remap resize deferred=%v resize=%d redraw=%d submissions=%d requests=%d", win.resizeDeferred, resizeCallbacks, redraws, renderer.submissions, frameRequests)
	}
	if len(acked) != 1 || acked[0] != 43 {
		t.Fatalf("remap ACK = %v, want [43]", acked)
	}

	win.onFrameDone(99, 0)
	if resizeCallbacks != 1 || redraws != 0 || renderer.submissions != 1 || frameRequests != 1 {
		t.Fatal("remap completion caused a duplicate resize or present")
	}
}

func TestWaylandDeferredConfigureKeepsGeometryAcrossScale(t *testing.T) {
	var acked []uint32
	frameRequests := 0
	app := &waylandApplication{api: newWaylandPresentTestAPI(&acked, &frameRequests)}
	win := &waylandWindow{
		app: app, surface: 1, xdgSurface: 2, surfaceState: waylandSurfaceMapped,
		width: 400, height: 300, savedLogicalW: 400, savedLogicalH: 300,
		framePending: true, frameCallback: 99,
	}
	win.storeScale(1)
	renderer := newWaylandPresentTestRenderer(1600, 1200)
	var resized platform.Size
	resizeCallbacks := 0
	win.state = &waylandWindowState{
		app: app, win: win, renderer: renderer,
		config: platform.Config{OnResize: func(size platform.Size) (platform.Frame, error) {
			resizeCallbacks++
			resized = size
			return platform.Frame{}, nil
		}},
		applyFrame: func(platform.Frame) (bool, error) { return false, nil },
	}

	win.handleXdgToplevelConfigure(800, 600, nil)
	win.handleXdgSurfaceConfigure(42)
	if !win.resizeDeferred || renderer.submissions != 0 || frameRequests != 0 {
		t.Fatal("pending configure did not defer its presentation")
	}
	win.applyScale(2)
	if win.configure.width != 800 || win.configure.height != 600 {
		t.Fatalf("scale replaced deferred geometry with %dx%d", win.configure.width, win.configure.height)
	}

	win.onFrameDone(99, 0)
	if win.width != 1600 || win.height != 1200 || resized.Width != 800 || resized.Height != 600 || resized.Scale != 2 {
		t.Fatalf("completed geometry device=%dx%d resize=%+v", win.width, win.height, resized)
	}
	if resizeCallbacks != 1 || renderer.submissions != 1 || frameRequests != 1 {
		t.Fatalf("completed scale configure resize=%d submissions=%d requests=%d", resizeCallbacks, renderer.submissions, frameRequests)
	}
	if len(acked) != 1 || acked[0] != 42 || win.configure.serial != 0 {
		t.Fatalf("scale configure ACK = %v, pending serial = %d", acked, win.configure.serial)
	}
}

func TestWaylandScaleBeforeConfigureSerialKeepsToplevelGeometry(t *testing.T) {
	for _, test := range []struct {
		name         string
		framePending bool
		trigger      string
	}{
		{name: "visible redraw", trigger: "redraw"},
		{name: "visible input", trigger: "input"},
		{name: "frame pending", framePending: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var acked []uint32
			frameRequests := 0
			app := &waylandApplication{api: newWaylandPresentTestAPI(&acked, &frameRequests)}
			win := &waylandWindow{
				app: app, surface: 1, xdgSurface: 2, surfaceState: waylandSurfaceMapped,
				width: 400, height: 300, savedLogicalW: 400, savedLogicalH: 300,
				framePending: test.framePending, frameCallback: 99,
			}
			win.storeScale(1)
			renderer := newWaylandPresentTestRenderer(1600, 1200)
			var resized platform.Size
			resizeCallbacks := 0
			redraws := 0
			win.state = &waylandWindowState{
				app: app, win: win, renderer: renderer,
				config: platform.Config{
					OnResize: func(size platform.Size) (platform.Frame, error) {
						resizeCallbacks++
						resized = size
						return platform.Frame{}, nil
					},
					OnRedraw: func() (platform.Frame, error) {
						redraws++
						return platform.Frame{}, nil
					},
				},
				applyFrame: func(platform.Frame) (bool, error) { return test.trigger == "input", nil },
			}

			win.handleXdgToplevelConfigure(800, 600, nil)
			win.applyScale(2)
			switch test.trigger {
			case "redraw":
				win.renderFrame()
			case "input":
				if err := win.presentInputFrame(platform.Frame{}); err != nil {
					t.Fatalf("present input before configure serial: %v", err)
				}
			}
			if win.configure.width != 800 || win.configure.height != 600 {
				t.Fatalf("scale replaced toplevel geometry with %dx%d", win.configure.width, win.configure.height)
			}
			if !win.resizeDeferred || resizeCallbacks != 0 || renderer.submissions != 0 || frameRequests != 0 || len(acked) != 0 {
				t.Fatalf("scale completed before its configure serial: deferred=%v resize=%d submissions=%d requests=%d ack=%v", win.resizeDeferred, resizeCallbacks, renderer.submissions, frameRequests, acked)
			}

			win.handleXdgSurfaceConfigure(42)
			if test.framePending {
				if resizeCallbacks != 0 || renderer.submissions != 0 || frameRequests != 0 {
					t.Fatal("frame-pending configure advanced before its callback")
				}
				win.onFrameDone(99, 0)
			}
			if win.width != 1600 || win.height != 1200 || resized.Width != 800 || resized.Height != 600 || resized.Scale != 2 {
				t.Fatalf("completed geometry device=%dx%d resize=%+v", win.width, win.height, resized)
			}
			if resizeCallbacks != 1 || redraws != 0 || renderer.submissions != 1 || frameRequests != 1 {
				t.Fatalf("completed configure resize=%d redraw=%d submissions=%d requests=%d", resizeCallbacks, redraws, renderer.submissions, frameRequests)
			}
			if len(acked) != 1 || acked[0] != 42 || win.configure.serial != 0 {
				t.Fatalf("configure ACK = %v, pending serial = %d", acked, win.configure.serial)
			}
			if win.configure.toplevelPending || win.resizeDeferred {
				t.Fatal("completed configure retained pending geometry state")
			}
		})
	}
}

func TestWaylandHiddenConfigureKeepsGeometryAcrossScaleAndRemap(t *testing.T) {
	var acked []uint32
	frameRequests := 0
	app := &waylandApplication{api: newWaylandPresentTestAPI(&acked, &frameRequests)}
	win := &waylandWindow{
		app: app, surface: 1, xdgSurface: 2, surfaceState: waylandSurfaceUnmapped,
		width: 400, height: 300, savedLogicalW: 400, savedLogicalH: 300,
	}
	win.storeScale(1)
	renderer := newWaylandPresentTestRenderer(1600, 1200)
	var resized platform.Size
	resizeCallbacks := 0
	redraws := 0
	win.state = &waylandWindowState{
		app: app, win: win, renderer: renderer,
		config: platform.Config{
			OnResize: func(size platform.Size) (platform.Frame, error) {
				resizeCallbacks++
				resized = size
				return platform.Frame{}, nil
			},
			OnRedraw: func() (platform.Frame, error) {
				redraws++
				return platform.Frame{}, nil
			},
		},
		applyFrame: func(platform.Frame) (bool, error) { return false, nil },
	}

	win.handleXdgToplevelConfigure(800, 600, nil)
	win.handleXdgSurfaceConfigure(42)
	win.applyScale(2)
	if win.configure.width != 800 || win.configure.height != 600 || !win.resizeDeferred {
		t.Fatalf("hidden scale lost configure geometry: %dx%d deferred=%v", win.configure.width, win.configure.height, win.resizeDeferred)
	}
	if resizeCallbacks != 0 || renderer.submissions != 0 || frameRequests != 0 || len(acked) != 0 {
		t.Fatal("hidden configure presented or acknowledged early")
	}

	win.Show()
	win.handleXdgSurfaceConfigure(43)
	if win.width != 1600 || win.height != 1200 || resized.Width != 800 || resized.Height != 600 || resized.Scale != 2 {
		t.Fatalf("remapped geometry device=%dx%d resize=%+v", win.width, win.height, resized)
	}
	if resizeCallbacks != 1 || redraws != 0 || renderer.submissions != 1 || frameRequests != 1 {
		t.Fatalf("remap resize=%d redraw=%d submissions=%d requests=%d", resizeCallbacks, redraws, renderer.submissions, frameRequests)
	}
	if len(acked) != 1 || acked[0] != 43 || win.configure.serial != 0 {
		t.Fatalf("remap ACK = %v, pending serial = %d", acked, win.configure.serial)
	}
}

func TestWaylandDeferredRestoreKeepsGeometryAcrossScale(t *testing.T) {
	var acked []uint32
	frameRequests := 0
	app := &waylandApplication{api: newWaylandPresentTestAPI(&acked, &frameRequests)}
	win := &waylandWindow{
		app: app, surface: 1, xdgSurface: 2, surfaceState: waylandSurfaceMapped,
		width: 1920, height: 1080, savedLogicalW: 500, savedLogicalH: 400,
		maximized: true, frameWanted: true, framePending: true, frameCallback: 99,
	}
	win.storeScale(1)
	renderer := newWaylandPresentTestRenderer(1096, 896)
	var resized platform.Size
	resizeCallbacks := 0
	win.state = &waylandWindowState{
		app: app, win: win, renderer: renderer,
		config: platform.Config{OnResize: func(size platform.Size) (platform.Frame, error) {
			resizeCallbacks++
			resized = size
			return platform.Frame{}, nil
		}},
		applyFrame: func(platform.Frame) (bool, error) { return false, nil },
	}

	win.handleXdgToplevelConfigure(0, 0, nil)
	win.handleXdgSurfaceConfigure(43)
	if !win.resizeDeferred || renderer.submissions != 0 || frameRequests != 0 {
		t.Fatal("zero-size unmaximize did not defer its restore")
	}
	win.applyScale(2)
	win.onFrameDone(99, 0)

	if win.width != 1096 || win.height != 896 || resized.Width != 500 || resized.Height != 400 || resized.Scale != 2 {
		t.Fatalf("restored geometry device=%dx%d resize=%+v", win.width, win.height, resized)
	}
	if resizeCallbacks != 1 || renderer.submissions != 1 || frameRequests != 1 {
		t.Fatalf("restored scale resize=%d submissions=%d requests=%d", resizeCallbacks, renderer.submissions, frameRequests)
	}
	if len(acked) != 1 || acked[0] != 43 || win.configure.serial != 0 {
		t.Fatalf("restore ACK = %v, pending serial = %d", acked, win.configure.serial)
	}
}

func TestWaylandApplyFailureDoesNotAckOrRequestFrame(t *testing.T) {
	var acked []uint32
	frameRequests := 0
	app := &waylandApplication{api: newWaylandPresentTestAPI(&acked, &frameRequests)}
	expectedErr := errors.New("apply failed")
	win := &waylandWindow{
		app: app, surface: 1, xdgSurface: 2, surfaceState: waylandSurfaceMapped,
		configure: waylandConfigureTransaction{serial: 43}, forcePresent: true,
	}
	renderer := newWaylandPresentTestRenderer(400, 300)
	win.state = &waylandWindowState{
		app: app, win: win, renderer: renderer,
		config:     platform.Config{OnRedraw: func() (platform.Frame, error) { return platform.Frame{}, nil }},
		applyFrame: func(platform.Frame) (bool, error) { return false, expectedErr },
	}

	win.renderFrame()
	if !errors.Is(app.firstErr, expectedErr) {
		t.Fatalf("application error = %v, want %v", app.firstErr, expectedErr)
	}
	if len(acked) != 0 || frameRequests != 0 || renderer.submissions != 0 || win.framePending {
		t.Fatalf("failed apply advanced: ack=%v frameRequests=%d submissions=%d pending=%v", acked, frameRequests, renderer.submissions, win.framePending)
	}
	if win.configure.serial != 43 {
		t.Fatalf("failed apply consumed serial %d, want 43", win.configure.serial)
	}
}

func TestWaylandFrameCallbackFailurePreservesTransaction(t *testing.T) {
	for _, test := range []struct {
		name  string
		apply func(*waylandWindow)
	}{
		{name: "redraw", apply: (*waylandWindow).renderFrame},
		{name: "resize", apply: (*waylandWindow).applyPendingResize},
		{name: "scale", apply: func(win *waylandWindow) { win.applyScale(2) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			var acked []uint32
			frameRequests := 0
			app := &waylandApplication{api: newWaylandPresentTestAPI(&acked, &frameRequests)}
			baseMarshal := app.api.proxyMarshalFlags
			callbackAttempts := 0
			app.api.proxyMarshalFlags = func(proxy uintptr, opcode uint32, interfacePtr uintptr, version uint32, flags uint32, args ...any) uintptr {
				if proxy == 1 && isWaylandFrameTestRequest(opcode, interfacePtr) && callbackAttempts == 0 {
					callbackAttempts++
					return 0
				}
				return baseMarshal(proxy, opcode, interfacePtr, version, flags, args...)
			}
			win := &waylandWindow{
				app: app, surface: 1, xdgSurface: 2, surfaceState: waylandSurfaceMapped,
				width: 400, height: 300, savedLogicalW: 400, savedLogicalH: 300,
				configure:    waylandConfigureTransaction{width: 800, height: 600, serial: 41},
				forcePresent: true, geometryDirty: true, needsRedraw: true,
			}
			win.storeScale(1)
			renderer := newWaylandPresentTestRenderer(800, 600)
			win.state = &waylandWindowState{
				app: app, win: win, renderer: renderer,
				config: platform.Config{
					OnResize: func(platform.Size) (platform.Frame, error) { return platform.Frame{}, nil },
					OnRedraw: func() (platform.Frame, error) { return platform.Frame{}, nil },
				},
				applyFrame: func(platform.Frame) (bool, error) { return false, nil },
			}

			test.apply(win)
			if !errors.Is(app.firstErr, errWaylandFrameCallback) {
				t.Fatalf("callback failure error = %v", app.firstErr)
			}
			if len(acked) != 0 || renderer.submissions != 0 || frameRequests != 0 || callbackAttempts != 1 {
				t.Fatalf("callback failure advanced: ack=%v submissions=%d requests=%d attempts=%d", acked, renderer.submissions, frameRequests, callbackAttempts)
			}
			if win.configure.serial != 41 || !win.forcePresent || !win.geometryDirty || !win.needsRedraw || win.framePending {
				t.Fatal("callback failure consumed or corrupted the pending transaction")
			}

			app.firstErr = nil
			win.renderFrame()
			if app.firstErr != nil || renderer.submissions != 1 || frameRequests != 1 {
				t.Fatalf("callback retry err=%v submissions=%d requests=%d", app.firstErr, renderer.submissions, frameRequests)
			}
			if len(acked) != 1 || acked[0] != 41 || win.configure.serial != 0 {
				t.Fatalf("callback retry ACK = %v, pending serial = %d", acked, win.configure.serial)
			}
			if win.forcePresent || win.geometryDirty || win.needsRedraw || !win.framePending {
				t.Fatal("callback retry did not complete the transaction")
			}
		})
	}
}

func TestWaylandFrameCallbackListenerFailureRetriesCleanly(t *testing.T) {
	var destroyed []uintptr
	listenerAttempts := 0
	api := &waylandAPI{
		proxyMarshalFlags: func(uintptr, uint32, uintptr, uint32, uint32, ...any) uintptr { return 99 },
		proxyAddListener: func(uintptr, uintptr, unsafe.Pointer) int32 {
			listenerAttempts++
			if listenerAttempts == 1 {
				return -1
			}
			return 0
		},
		proxyDestroy: func(proxy uintptr) { destroyed = append(destroyed, proxy) },
	}
	win := &waylandWindow{app: &waylandApplication{api: api}, surface: 1, surfaceState: waylandSurfaceMapped}
	acks := 0
	presents := 0
	present := func() error {
		presents++
		return nil
	}
	ack := func() { acks++ }

	err := win.presentWithFrameCallback(present, ack)
	if !errors.Is(err, errWaylandFrameCallback) {
		t.Fatalf("listener failure error = %v", err)
	}
	if acks != 0 || presents != 0 || listenerAttempts != 1 {
		t.Fatalf("listener failure advanced: ACK=%d presents=%d attempts=%d", acks, presents, listenerAttempts)
	}
	if win.framePending || win.frameCallback != 0 || len(destroyed) != 1 || destroyed[0] != 99 {
		t.Fatalf("listener failure cleanup: pending=%v callback=%d destroyed=%v", win.framePending, win.frameCallback, destroyed)
	}

	if err := win.presentWithFrameCallback(present, ack); err != nil {
		t.Fatalf("listener retry: %v", err)
	}
	if acks != 1 || presents != 1 || listenerAttempts != 2 || !win.framePending || win.frameCallback != 99 {
		t.Fatalf("listener retry: ACK=%d presents=%d attempts=%d pending=%v callback=%d", acks, presents, listenerAttempts, win.framePending, win.frameCallback)
	}
}

func TestWaylandSurfaceStateMachineCommitBoundaries(t *testing.T) {
	var acked []uint32
	frameRequests := 0
	app := &waylandApplication{api: newWaylandPresentTestAPI(&acked, &frameRequests)}
	win := &waylandWindow{app: app, surface: 1, xdgSurface: 2, xdgToplevel: 3}
	presents := 0
	present := func() error { presents++; return nil }

	for _, state := range []waylandSurfaceState{
		waylandSurfaceCreated,
		waylandSurfaceAwaitInitialConfigure,
		waylandSurfaceUnmapped,
	} {
		win.surfaceState = state
		if err := win.presentWithFrameCallback(present, win.ackConfigure); !errors.Is(err, errWaylandSurfaceNotReady) {
			t.Fatalf("state %v commit error = %v, want surface-not-ready", state, err)
		}
	}
	if presents != 0 || frameRequests != 0 || len(acked) != 0 {
		t.Fatalf("blocked states advanced: presents=%d requests=%d ACK=%v", presents, frameRequests, acked)
	}

	win.surfaceState = waylandSurfaceAwaitInitialConfigure
	win.configure.serial = 41
	if err := win.presentWithFrameCallback(present, win.ackConfigure); err != nil {
		t.Fatalf("initial configure commit: %v", err)
	}
	if win.surfaceState != waylandSurfaceMapped || len(acked) != 1 || acked[0] != 41 {
		t.Fatalf("initial commit state=%v ACK=%v", win.surfaceState, acked)
	}
	win.onFrameDone(99, 0)

	win.Hide()
	if win.surfaceState != waylandSurfaceUnmapped {
		t.Fatalf("Hide state=%v, want Unmapped", win.surfaceState)
	}
	if err := win.presentWithFrameCallback(present, win.ackConfigure); !errors.Is(err, errWaylandSurfaceNotReady) {
		t.Fatalf("hidden commit error = %v", err)
	}
	win.Show()
	if win.surfaceState != waylandSurfaceAwaitInitialConfigure || win.configure.serial != 0 {
		t.Fatalf("Show state=%v serial=%d, want fresh remap transaction", win.surfaceState, win.configure.serial)
	}
	win.handleXdgSurfaceConfigure(42)
	if err := win.presentWithFrameCallback(present, win.ackConfigure); err != nil {
		t.Fatalf("remap commit: %v", err)
	}
	if win.surfaceState != waylandSurfaceMapped || len(acked) != 2 || acked[1] != 42 || presents != 2 {
		t.Fatalf("remap state=%v ACK=%v presents=%d", win.surfaceState, acked, presents)
	}

	win.destroy()
	if win.surfaceState != waylandSurfaceClosed {
		t.Fatalf("destroy state=%v, want Closed", win.surfaceState)
	}
	if err := win.presentWithFrameCallback(present, win.ackConfigure); err != nil {
		t.Fatalf("closed commit was not a normal cancellation: %v", err)
	}
	if presents != 2 || frameRequests != 2 {
		t.Fatalf("closed state advanced: presents=%d requests=%d", presents, frameRequests)
	}
}

func TestWaylandCommitCloseCancellation(t *testing.T) {
	for _, test := range []struct {
		name   string
		resize bool
	}{
		{name: "redraw callback"},
		{name: "resize callback", resize: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var acked []uint32
			frameRequests := 0
			app := &waylandApplication{
				api:  newWaylandPresentTestAPI(&acked, &frameRequests),
				loop: &waylandLoop{wakeFD: -1},
			}
			renderer := newWaylandPresentTestRenderer(400, 300)
			win := &waylandWindow{
				app: app, surface: 1, xdgSurface: 2, surfaceState: waylandSurfaceMapped,
				width: 400, height: 300, savedLogicalW: 400, savedLogicalH: 300,
			}
			win.storeScale(1)
			win.state = &waylandWindowState{
				app: app, win: win, renderer: renderer,
				config: platform.Config{
					OnRedraw: func() (platform.Frame, error) { win.Close(); return platform.Frame{}, nil },
					OnResize: func(platform.Size) (platform.Frame, error) { win.Close(); return platform.Frame{}, nil },
				},
				applyFrame: func(platform.Frame) (bool, error) { return true, nil },
			}
			if test.resize {
				win.configure = waylandConfigureTransaction{width: 800, height: 600, serial: 41}
				win.applyPendingResize()
			} else {
				win.renderFrame()
			}
			if app.firstErr != nil {
				t.Fatalf("Close during commit became an application error: %v", app.firstErr)
			}
			if !win.closed.Load() || renderer.submissions != 0 || frameRequests != 0 || len(acked) != 0 {
				t.Fatalf("closed commit advanced: closed=%v submissions=%d requests=%d ACK=%v", win.closed.Load(), renderer.submissions, frameRequests, acked)
			}
			win.destroy()
		})
	}

	t.Run("close request before commit", func(t *testing.T) {
		var acked []uint32
		frameRequests := 0
		app := &waylandApplication{api: newWaylandPresentTestAPI(&acked, &frameRequests)}
		win := &waylandWindow{app: app, surface: 1, xdgSurface: 2, surfaceState: waylandSurfaceMapped}
		win.closed.Store(true)
		presents := 0
		if err := win.presentWithFrameCallback(func() error { presents++; return nil }, win.ackConfigure); err != nil {
			t.Fatalf("Close request before commit became an error: %v", err)
		}
		if presents != 0 || frameRequests != 0 || len(acked) != 0 {
			t.Fatalf("Close request advanced commit: presents=%d requests=%d ACK=%v", presents, frameRequests, acked)
		}
	})
}

func TestWaylandQueuedHideShowAfterDestroyStayClosed(t *testing.T) {
	api := &waylandAPI{
		proxyMarshalFlags: func(uintptr, uint32, uintptr, uint32, uint32, ...any) uintptr { return 0 },
		proxyDestroy:      func(uintptr) {},
		displayFlush:      func(uintptr) int32 { return 0 },
	}
	loop := &waylandLoop{
		wakeFD: 11,
		libc: &libcAPI{write: func(_ int32, _ unsafe.Pointer, size uintptr) int {
			return int(size)
		}},
	}
	app := &waylandApplication{api: api, loop: loop}
	win := &waylandWindow{
		app: app, surface: 1, xdgSurface: 2, xdgToplevel: 3,
		surfaceState: waylandSurfaceMapped,
	}
	win.Hide()
	win.Show()
	win.destroy()
	loop.drainCommands()
	if win.surfaceState != waylandSurfaceClosed {
		t.Fatalf("queued Hide/Show resurrected destroyed window: state=%v", win.surfaceState)
	}
}

func TestWaylandLegacyCommitCallbackFailureRetriesCleanly(t *testing.T) {
	var acked []uint32
	frameRequests := 0
	commits := 0
	app := &waylandApplication{api: newWaylandPresentTestAPI(&acked, &frameRequests)}
	baseMarshal := app.api.proxyMarshalFlags
	callbackAttempts := 0
	app.api.proxyMarshalFlags = func(proxy uintptr, opcode uint32, interfacePtr uintptr, version uint32, flags uint32, args ...any) uintptr {
		if proxy == 1 && isWaylandFrameTestRequest(opcode, interfacePtr) && callbackAttempts == 0 {
			callbackAttempts++
			return 0
		}
		if proxy == 1 && opcode == wlSurfaceCommitOpcode {
			commits++
		}
		return baseMarshal(proxy, opcode, interfacePtr, version, flags, args...)
	}
	win := &waylandWindow{
		app: app, surface: 1, xdgSurface: 2, surfaceState: waylandSurfaceMapped,
		configure: waylandConfigureTransaction{serial: 41}, forcePresent: true, needsRedraw: true,
	}
	win.state = &waylandWindowState{app: app, win: win}

	win.renderFrame()
	if !errors.Is(app.firstErr, errWaylandFrameCallback) {
		t.Fatalf("legacy callback failure error = %v", app.firstErr)
	}
	if len(acked) != 0 || commits != 0 || frameRequests != 0 || callbackAttempts != 1 {
		t.Fatalf("legacy failure advanced: ACK=%v commits=%d requests=%d attempts=%d", acked, commits, frameRequests, callbackAttempts)
	}
	if win.configure.serial != 41 || !win.forcePresent || !win.needsRedraw || win.framePending {
		t.Fatal("legacy failure consumed the pending transaction")
	}

	app.firstErr = nil
	win.renderFrame()
	if app.firstErr != nil || len(acked) != 1 || acked[0] != 41 || commits != 1 || frameRequests != 1 {
		t.Fatalf("legacy retry: err=%v ACK=%v commits=%d requests=%d", app.firstErr, acked, commits, frameRequests)
	}
	if win.configure.serial != 0 || win.forcePresent || win.needsRedraw || !win.framePending {
		t.Fatal("legacy retry did not complete the transaction")
	}
}

func TestWaylandPresentFailureCleansNewCallback(t *testing.T) {
	var acked []uint32
	frameRequests := 0
	app := &waylandApplication{api: newWaylandPresentTestAPI(&acked, &frameRequests)}
	var destroyed []uintptr
	app.api.proxyDestroy = func(proxy uintptr) { destroyed = append(destroyed, proxy) }
	win := &waylandWindow{
		app: app, surface: 1, xdgSurface: 2, surfaceState: waylandSurfaceMapped,
		configure: waylandConfigureTransaction{serial: 41}, forcePresent: true,
	}
	renderer := newWaylandPresentTestRenderer(400, 300)
	presentCalls := 0
	renderer.api.queuePresent = func(uintptr, *vkPresentInfo) int32 {
		presentCalls++
		if presentCalls == 1 {
			return vkErrorDeviceLost
		}
		return vkSuccess
	}
	win.state = &waylandWindowState{
		app: app, win: win, renderer: renderer,
		config:     platform.Config{OnRedraw: func() (platform.Frame, error) { return platform.Frame{}, nil }},
		applyFrame: func(platform.Frame) (bool, error) { return false, nil },
	}

	win.renderFrame()
	if app.firstErr == nil {
		t.Fatal("present failure did not reach the application")
	}
	if win.framePending || win.frameCallback != 0 {
		t.Fatalf("present failure left callback pending=%v handle=%d", win.framePending, win.frameCallback)
	}
	if len(destroyed) != 1 || destroyed[0] != 99 {
		t.Fatalf("destroyed callbacks = %v, want [99]", destroyed)
	}

	win.renderFrame()
	if presentCalls != 2 || renderer.submissions != 1 || frameRequests != 2 {
		t.Fatalf("retry presentCalls=%d submissions=%d frameRequests=%d", presentCalls, renderer.submissions, frameRequests)
	}
	if !win.framePending || win.frameCallback != 99 {
		t.Fatal("successful retry did not establish a new frame callback")
	}
}

func TestWaylandDeferredResizePresentFailureLifecycle(t *testing.T) {
	var acked []uint32
	frameRequests := 0
	app := &waylandApplication{api: newWaylandPresentTestAPI(&acked, &frameRequests)}
	var destroyed []uintptr
	app.api.proxyDestroy = func(proxy uintptr) { destroyed = append(destroyed, proxy) }
	win := &waylandWindow{
		app: app, surface: 1, xdgSurface: 2, surfaceState: waylandSurfaceMapped,
		width: 400, height: 300, savedLogicalW: 400, savedLogicalH: 300,
		configure:    waylandConfigureTransaction{width: 800, height: 600, serial: 42},
		forcePresent: true, framePending: true, frameCallback: 77,
	}
	win.storeScale(1)
	renderer := newWaylandPresentTestRenderer(800, 600)
	presentCalls := 0
	renderer.api.queuePresent = func(uintptr, *vkPresentInfo) int32 {
		presentCalls++
		if presentCalls == 1 {
			return vkErrorDeviceLost
		}
		return vkSuccess
	}
	win.state = &waylandWindowState{
		app: app, win: win, renderer: renderer,
		config: platform.Config{
			OnResize: func(platform.Size) (platform.Frame, error) { return platform.Frame{}, nil },
			OnRedraw: func() (platform.Frame, error) { return platform.Frame{}, nil },
		},
		applyFrame: func(platform.Frame) (bool, error) { return false, nil },
	}

	win.applyPendingResize()
	if app.firstErr != nil || !win.resizeDeferred || presentCalls != 0 {
		t.Fatalf("pending resize advanced early: err=%v deferred=%v presents=%d", app.firstErr, win.resizeDeferred, presentCalls)
	}
	if !win.framePending || win.frameCallback != 77 || len(destroyed) != 0 || frameRequests != 0 {
		t.Fatal("pending resize disturbed the previous callback")
	}

	win.onFrameDone(77, 0)
	if app.firstErr == nil || presentCalls != 1 || frameRequests != 1 {
		t.Fatalf("deferred resize failure err=%v presents=%d requests=%d", app.firstErr, presentCalls, frameRequests)
	}
	if win.framePending || win.frameCallback != 0 || len(destroyed) != 2 || destroyed[0] != 77 || destroyed[1] != 99 {
		t.Fatalf("failed resize callback state pending=%v handle=%d destroyed=%v", win.framePending, win.frameCallback, destroyed)
	}
	if len(acked) != 1 || acked[0] != 42 {
		t.Fatalf("failed resize ACK = %v, want [42]", acked)
	}

	win.renderFrame()
	if presentCalls != 2 || renderer.submissions != 1 || frameRequests != 2 {
		t.Fatalf("retry presentCalls=%d submissions=%d requests=%d", presentCalls, renderer.submissions, frameRequests)
	}
	if !win.framePending || win.frameCallback != 99 {
		t.Fatal("successful retry did not establish a new callback")
	}
}

func TestWaylandResizeScaleApplyFailurePreservesTransaction(t *testing.T) {
	for _, test := range []struct {
		name  string
		apply func(*waylandWindow)
	}{
		{name: "resize", apply: (*waylandWindow).applyPendingResize},
		{name: "scale", apply: func(win *waylandWindow) { win.applyScale(2) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			var acked []uint32
			frameRequests := 0
			app := &waylandApplication{api: newWaylandPresentTestAPI(&acked, &frameRequests)}
			geometryCalls := 0
			baseMarshal := app.api.proxyMarshalFlags
			app.api.proxyMarshalFlags = func(proxy uintptr, opcode uint32, interfacePtr uintptr, version uint32, flags uint32, args ...any) uintptr {
				if opcode == xdgSurfaceSetWindowGeometryOpcode || (proxy == 7 && opcode == 2) {
					geometryCalls++
				}
				return baseMarshal(proxy, opcode, interfacePtr, version, flags, args...)
			}
			expectedErr := errors.New("resize apply failed")
			win := &waylandWindow{
				app: app, surface: 1, xdgSurface: 2, viewport: 7, surfaceState: waylandSurfaceMapped,
				width: 400, height: 300, savedLogicalW: 400, savedLogicalH: 300,
				configure: waylandConfigureTransaction{width: 800, height: 600, serial: 44}, forcePresent: true,
			}
			win.storeScale(1)
			renderer := newWaylandPresentTestRenderer(800, 600)
			win.state = &waylandWindowState{
				app: app, win: win, renderer: renderer,
				config: platform.Config{OnResize: func(platform.Size) (platform.Frame, error) {
					return platform.Frame{}, nil
				}},
				applyFrame: func(platform.Frame) (bool, error) { return false, expectedErr },
			}

			test.apply(win)
			if !errors.Is(app.firstErr, expectedErr) {
				t.Fatalf("application error = %v, want %v", app.firstErr, expectedErr)
			}
			if geometryCalls != 0 || len(acked) != 0 || frameRequests != 0 || renderer.submissions != 0 {
				t.Fatalf("failed apply advanced: geometry=%d ack=%v requests=%d submissions=%d", geometryCalls, acked, frameRequests, renderer.submissions)
			}
			if !win.geometryDirty {
				t.Fatal("failed apply consumed geometryDirty")
			}
			if win.configure.serial != 44 {
				t.Fatalf("failed apply consumed serial %d, want 44", win.configure.serial)
			}
		})
	}
}

func TestWaylandRedrawErrorsReachApplication(t *testing.T) {
	callbackErr := errors.New("redraw callback failed")
	routerErr := errors.New("scene upload failed")
	tests := []struct {
		name     string
		renderer *vulkanRenderer
		onRedraw func() (platform.Frame, error)
		present  func(platform.Frame) error
		want     error
	}{
		{
			name:     "callback",
			renderer: &vulkanRenderer{},
			onRedraw: func() (platform.Frame, error) {
				return platform.Frame{}, callbackErr
			},
			want: callbackErr,
		},
		{
			name:     "router",
			renderer: &vulkanRenderer{},
			onRedraw: func() (platform.Frame, error) {
				return platform.Frame{}, nil
			},
			present: func(platform.Frame) error { return routerErr },
			want:    routerErr,
		},
		{
			name: "renderer",
			renderer: &vulkanRenderer{api: &vulkanAPI{
				waitForFences: func(uintptr, uint32, *uintptr, uint32, uint64) int32 {
					return vkErrorDeviceLost
				},
			}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := &waylandApplication{api: &waylandAPI{}}
			installWaylandFrameTestCallbacks(app.api)
			win := &waylandWindow{app: app, surface: 1, surfaceState: waylandSurfaceMapped}
			win.state = &waylandWindowState{
				app:      app,
				win:      win,
				renderer: test.renderer,
				config:   platform.Config{OnRedraw: test.onRedraw},
			}
			win.state.router.present = test.present

			win.renderFrame()
			if app.firstErr == nil {
				t.Fatal("redraw error did not reach the application")
			}
			if test.want != nil && !errors.Is(app.firstErr, test.want) {
				t.Fatalf("application error = %v, want %v", app.firstErr, test.want)
			}
		})
	}
}

func TestWaylandFrameClockBackpressure(t *testing.T) {
	redrawCount := 0
	win := &waylandWindow{
		surface:      1,
		xdgSurface:   2,
		surfaceState: waylandSurfaceMapped,
	}
	win.state = &waylandWindowState{
		win: win,
		config: platform.Config{
			OnRedraw: func() (platform.Frame, error) {
				redrawCount++
				return platform.Frame{}, nil
			},
		},
	}

	// When framePending is true, renderFrame must not render immediately, but set needsRedraw
	win.framePending = true
	win.renderFrame()

	if !win.needsRedraw {
		t.Fatal("expected needsRedraw to be set to true when framePending is active")
	}
	if redrawCount != 0 {
		t.Fatalf("expected redraw to be delayed by backpressure, got %d", redrawCount)
	}

	// When onFrameDone is called, framePending is cleared and deferred redraw runs
	win.onFrameDone(0, 0)
	if win.framePending {
		// Note: renderFrame requests a new frame callback when app/api are present;
		// here app is nil so framePending stays false
	}
	if win.needsRedraw {
		t.Fatal("expected deferred redraw to clear needsRedraw")
	}
}

func TestWaylandMultiWindowIndependence(t *testing.T) {
	app := &waylandApplication{}

	win1 := &waylandWindow{
		app:         app,
		surface:     10,
		xdgSurface:  20,
		xdgToplevel: 30,
		width:       400,
		height:      300,
	}
	state1 := &waylandWindowState{app: app, win: win1}
	win1.state = state1

	win2 := &waylandWindow{
		app:         app,
		surface:     11,
		xdgSurface:  21,
		xdgToplevel: 31,
		width:       600,
		height:      500,
	}
	state2 := &waylandWindowState{app: app, win: win2}
	win2.state = state2

	app.windows = []*waylandWindowState{state1, state2}

	// Close win1
	closed1 := false
	state1.config.OnClosed = func() {
		closed1 = true
	}
	win1.destroy()

	if !closed1 {
		t.Fatal("expected win1 OnClosed to be called")
	}
	if len(app.windows) != 1 || app.windows[0] != state2 {
		t.Fatalf("expected only state2 to remain in app.windows, got %d", len(app.windows))
	}
	if win2.closed.Load() {
		t.Fatal("win2 should remain open and unaffected")
	}
	if win2.width != 600 || win2.height != 500 {
		t.Fatalf("win2 dimensions corrupted: %dx%d", win2.width, win2.height)
	}

	// Close win2
	win2.destroy()
	if len(app.windows) != 0 {
		t.Fatalf("expected all windows closed, got %d", len(app.windows))
	}
}

func TestWaylandResizeEdgeMapping(t *testing.T) {
	cases := []struct {
		edge platform.WindowEdge
		want uint32
	}{
		{platform.EdgeTop, 1},
		{platform.EdgeBottom, 2},
		{platform.EdgeLeft, 4},
		{platform.EdgeTopLeft, 5},
		{platform.EdgeBottomLeft, 6},
		{platform.EdgeRight, 8},
		{platform.EdgeTopRight, 9},
		{platform.EdgeBottomRight, 10},
		{platform.EdgeNone, 0},
	}

	for _, c := range cases {
		got := xdgResizeEdge(c.edge)
		if got != c.want {
			t.Errorf("xdgResizeEdge(%d) = %d, want %d", c.edge, got, c.want)
		}
	}
}

func TestWaylandConfigureAckOnStateChange(t *testing.T) {
	var ackedSerials []uint32
	var committedSurfaces []uintptr

	api := &waylandAPI{
		proxyMarshalFlags: func(proxy uintptr, opcode uint32, interfacePtr uintptr, version uint32, flags uint32, args ...any) uintptr {
			if opcode == xdgSurfaceAckConfigureOpcode && len(args) > 0 {
				if serial, ok := args[0].(uint32); ok {
					ackedSerials = append(ackedSerials, serial)
				}
			} else if opcode == wlSurfaceCommitOpcode {
				committedSurfaces = append(committedSurfaces, proxy)
			}
			return 0
		},
		proxyAddListener: func(proxy uintptr, implementation uintptr, data unsafe.Pointer) int32 {
			return 0
		},
	}
	app := &waylandApplication{api: api}
	installWaylandFrameTestCallbacks(api)
	win := &waylandWindow{
		app:          app,
		surface:      10,
		xdgSurface:   20,
		xdgToplevel:  30,
		surfaceState: waylandSurfaceMapped,
		width:        400,
		height:       300,
	}
	state := &waylandWindowState{app: app, win: win}
	win.state = state

	// Compositor sends state change (e.g. activation) with 0, 0 (no size change)
	statesData := []uint32{xdgToplevelStateActivated}
	statesArray := wlArray{
		Size:  4,
		Alloc: 4,
		Data:  unsafe.Pointer(&statesData[0]),
	}
	win.handleXdgToplevelConfigure(0, 0, &statesArray)

	// Followed by surface configure with serial
	win.handleXdgSurfaceConfigure(5555)

	if len(ackedSerials) != 1 || ackedSerials[0] != 5555 {
		t.Fatalf("expected serial 5555 to be acknowledged on state change without resize, got %v", ackedSerials)
	}
	if len(committedSurfaces) != 1 || committedSurfaces[0] != 10 {
		t.Fatalf("expected surface commit on state change configure, got %v", committedSurfaces)
	}
	if win.configure.serial != 0 {
		t.Fatalf("expected configure.serial to be cleared after ack, got %d", win.configure.serial)
	}
}

func TestWaylandDestructorOrdering(t *testing.T) {
	var order []string

	app := &waylandApplication{}
	win := &waylandWindow{
		app:         app,
		surface:     10,
		xdgSurface:  20,
		xdgToplevel: 30,
		decoration:  40,
	}
	api := &waylandAPI{
		proxyMarshalFlags: func(proxy uintptr, opcode uint32, interfacePtr uintptr, version uint32, flags uint32, args ...any) uintptr {
			return 0
		},
		proxyDestroy: func(proxy uintptr) {
			switch proxy {
			case 40:
				order = append(order, "decoration")
			case 30:
				order = append(order, "xdgToplevel")
			case 20:
				order = append(order, "xdgSurface")
			case 10:
				order = append(order, "surface")
			}
		},
	}
	app.api = api

	state := &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			OnClosed: func() {
				order = append(order, "OnClosed")
			},
		},
	}
	win.state = state

	win.destroy()

	// Order must be: decoration -> xdgToplevel -> xdgSurface -> OnClosed (renderer teardown) -> surface
	expectedOrder := []string{"decoration", "xdgToplevel", "xdgSurface", "OnClosed", "surface"}
	if len(order) != len(expectedOrder) {
		t.Fatalf("unexpected destructor order: got %v, want %v", order, expectedOrder)
	}
	for i := range order {
		if order[i] != expectedOrder[i] {
			t.Fatalf("at index %d: got %s, want %s", i, order[i], expectedOrder[i])
		}
	}
}

func TestWaylandProtocolSignatures(t *testing.T) {
	tests := []struct {
		name     string
		msg      wlMessage
		wantName string
		wantSig  string
	}{
		{"xdg_toplevel.set_parent", xdgToplevelRequests[1], "set_parent", "?o"},
		{"xdg_toplevel.show_window_menu", xdgToplevelRequests[4], "show_window_menu", "ouii"},
		{"xdg_toplevel.move", xdgToplevelRequests[5], "move", "ou"},
		{"xdg_toplevel.resize", xdgToplevelRequests[6], "resize", "ouu"},
		{"xdg_toplevel.set_fullscreen", xdgToplevelRequests[11], "set_fullscreen", "?o"},
		{"xdg_surface.get_popup", xdgSurfaceRequests[2], "get_popup", "n?oo"},
		{"wp_viewport.set_source", wpViewportRequests[1], "set_source", "ffff"},
		{"wl_pointer.set_cursor", wlPointerRequests[0], "set_cursor", "u?oii"},
		{"wl_data_device.start_drag", wlDataDeviceRequests[0], "start_drag", "?oo?ou"},
		{"wl_data_device.set_selection", wlDataDeviceRequests[1], "set_selection", "?ou"},
	}

	for _, tt := range tests {
		gotName := waylandGoString(tt.msg.Name)
		gotSig := waylandGoString(tt.msg.Signature)
		if gotName != tt.wantName {
			t.Errorf("%s: got name %q, want %q", tt.name, gotName, tt.wantName)
		}
		if gotSig != tt.wantSig {
			t.Errorf("%s: got signature %q, want %q", tt.name, gotSig, tt.wantSig)
		}
	}
}

func TestWaylandViewportDestinationSync(t *testing.T) {
	var destViewport uintptr
	var destW, destH int32

	api := &waylandAPI{
		proxyMarshalFlags: func(proxy uintptr, opcode uint32, interfacePtr uintptr, version uint32, flags uint32, args ...any) uintptr {
			if opcode == 2 && proxy == 0xbeef && len(args) >= 2 { // wp_viewport.set_destination
				destViewport = proxy
				if w, ok := args[0].(int32); ok {
					destW = w
				}
				if h, ok := args[1].(int32); ok {
					destH = h
				}
			}
			return 0
		},
	}
	app := &waylandApplication{api: api}

	// 1. Initial size at 1.25x scaling with margin (custom titlebar)
	// Logical content: 800x600. Scale: 1.25.
	// contentW = round(800 * 1.25) = 1000
	// contentH = round(600 * 1.25) = 750
	// margin = frameMargin(1.25) = round(24 * 1.25) = 30
	// totalW = 1000 + 60 = 1060, totalH = 750 + 60 = 810
	win := &waylandWindow{
		app:      app,
		viewport: 0xbeef,
		width:    1060,
		height:   810,
		margin:   30,
	}
	win.storeScale(1.25)

	win.updateViewportDestination()
	if destViewport != 0xbeef || destW != 848 || destH != 648 {
		t.Fatalf("expected viewport destination (848, 648) at 1.25x with margin, got (%d, %d)", destW, destH)
	}

	// 2. Maximized window at 1.25x (no margin)
	// totalW = 1250, totalH = 1000
	win.margin = 0
	win.width = 1250
	win.height = 1000
	win.updateViewportDestination()
	if destW != 1000 || destH != 800 {
		t.Fatalf("expected viewport destination (1000, 800) at 1.25x maximized, got (%d, %d)", destW, destH)
	}

	// 3. Configure event with new dimensions
	win.configure.width = 1200 // new logical width
	win.configure.height = 900 // new logical height
	win.handleXdgSurfaceConfigure(42)

	expectedLogicalW := int32(math.Round(float64(float32(win.width) / 1.25)))
	expectedLogicalH := int32(math.Round(float64(float32(win.height) / 1.25)))
	if destW != expectedLogicalW || destH != expectedLogicalH {
		t.Fatalf("expected configure to sync viewport destination to (%d, %d), got (%d, %d)", expectedLogicalW, expectedLogicalH, destW, destH)
	}
}

func TestWaylandResizeErrorsStopConfigureCompletion(t *testing.T) {
	callbackErr := errors.New("resize callback failed")
	routerErr := errors.New("resize upload failed")
	tests := []struct {
		name           string
		resizeFailure  bool
		callbackErr    error
		routerErr      error
		presentFailure bool
		wantACK        int
		wantPresents   int
		wantErr        error
	}{
		{name: "renderer resize", resizeFailure: true},
		{name: "resize callback", callbackErr: callbackErr, wantErr: callbackErr},
		{name: "router present", routerErr: routerErr, wantACK: 1, wantErr: routerErr},
		{name: "renderer present", presentFailure: true, wantACK: 1, wantPresents: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			acks := 0
			frameReports := 0
			rendererPresents := 0
			api := &waylandAPI{
				proxyMarshalFlags: func(proxy uintptr, opcode uint32, interfacePtr uintptr, version uint32, flags uint32, args ...any) uintptr {
					if opcode == xdgSurfaceAckConfigureOpcode {
						acks++
					}
					return 0
				},
			}
			app := &waylandApplication{api: api}
			installWaylandFrameTestCallbacks(api)
			win := &waylandWindow{
				app:          app,
				surface:      1,
				xdgSurface:   2,
				surfaceState: waylandSurfaceMapped,
				width:        800,
				height:       600,
				configure:    waylandConfigureTransaction{width: 780, height: 570, serial: 99},
				framePending: true,
			}
			win.storeScale(1)
			renderer := &vulkanRenderer{
				width:  780,
				height: 570,
				api: &vulkanAPI{
					waitForFences: func(uintptr, uint32, *uintptr, uint32, uint64) int32 {
						rendererPresents++
						return vkErrorDeviceLost
					},
				},
			}
			if test.resizeFailure {
				renderer.width = 800
				renderer.height = 600
				renderer.api.deviceWaitIdle = func(uintptr) int32 { return vkErrorDeviceLost }
			}
			state := &waylandWindowState{
				app:      app,
				win:      win,
				renderer: renderer,
				config: platform.Config{
					OnResize: func(platform.Size) (platform.Frame, error) {
						return platform.Frame{}, test.callbackErr
					},
					OnFrameChange: func(ui.Rect) {
						frameReports++
					},
				},
			}
			if test.presentFailure {
				state.config.OnResize = nil
			}
			state.router.present = func(platform.Frame) error { return test.routerErr }
			win.state = state

			win.applyPendingResize()
			if app.firstErr != nil || !win.resizeDeferred || acks != 0 || rendererPresents != 0 {
				t.Fatalf("pending resize advanced early: err=%v deferred=%v acks=%d presents=%d", app.firstErr, win.resizeDeferred, acks, rendererPresents)
			}
			win.onFrameDone(0, 0)
			if app.firstErr == nil {
				t.Fatal("resize failure did not reach the application")
			}
			if test.wantErr != nil && !errors.Is(app.firstErr, test.wantErr) {
				t.Fatalf("application error = %v, want %v", app.firstErr, test.wantErr)
			}
			if acks != test.wantACK {
				t.Fatalf("configure ACKs = %d, want %d", acks, test.wantACK)
			}
			if frameReports != 0 {
				t.Fatalf("failed resize reported %d new frames", frameReports)
			}
			if rendererPresents != test.wantPresents {
				t.Fatalf("renderer presents = %d, want %d", rendererPresents, test.wantPresents)
			}
		})
	}
}

func TestWaylandAtomicConfigureResizeSequence(t *testing.T) {
	type protocolEvent struct {
		name string
		val  any
	}
	var events []protocolEvent

	api := &waylandAPI{
		proxyMarshalFlags: func(proxy uintptr, opcode uint32, interfacePtr uintptr, version uint32, flags uint32, args ...any) uintptr {
			switch opcode {
			case xdgSurfaceAckConfigureOpcode:
				if len(args) > 0 {
					events = append(events, protocolEvent{name: "ack_configure", val: args[0]})
				}
			case xdgSurfaceSetWindowGeometryOpcode:
				if len(args) >= 4 {
					events = append(events, protocolEvent{name: "set_window_geometry", val: [4]int32{args[0].(int32), args[1].(int32), args[2].(int32), args[3].(int32)}})
				}
			case 2: // wp_viewport.set_destination
				if len(args) >= 2 {
					events = append(events, protocolEvent{name: "set_destination", val: [2]int32{args[0].(int32), args[1].(int32)}})
				}
			case wlSurfaceCommitOpcode:
				events = append(events, protocolEvent{name: "surface_commit", val: proxy})
			}
			return 0
		},
		proxyAddListener: func(proxy uintptr, implementation uintptr, data unsafe.Pointer) int32 {
			return 0
		},
		displayFlush: func(display uintptr) int32 {
			events = append(events, protocolEvent{name: "display_flush", val: nil})
			return 0
		},
	}

	app := &waylandApplication{api: api, display: 0x111}
	installWaylandFrameTestCallbacks(api)
	win := &waylandWindow{
		app:          app,
		surface:      0x222,
		xdgSurface:   0x333,
		viewport:     0x444,
		surfaceState: waylandSurfaceMapped,
		width:        800,
		height:       600,
		margin:       0,
	}
	state := &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			OnResize: func(size platform.Size) (platform.Frame, error) {
				events = append(events, protocolEvent{name: "on_resize", val: size})
				return platform.Frame{}, nil
			},
		},
	}
	win.state = state

	// Simulate compositor configuring a top-left resize from 800x600 down to 780x570
	win.configure.width = 780
	win.configure.height = 570
	win.handleXdgSurfaceConfigure(9999)

	// Verify order:
	// 1. ack_configure must occur before surface_commit
	// 2. set_window_geometry must reflect new size (780, 570) before surface_commit
	// 3. Exactly one commit per configure event!
	ackIdx := -1
	geomIdx := -1
	commitIdx := -1
	commitCount := 0

	for i, ev := range events {
		switch ev.name {
		case "ack_configure":
			if ackIdx == -1 {
				ackIdx = i
			}
			if ev.val != uint32(9999) {
				t.Errorf("expected ack serial 9999, got %v", ev.val)
			}
		case "set_window_geometry":
			if geomIdx == -1 {
				geomIdx = i
			}
			geom := ev.val.([4]int32)
			if geom[2] != 780 || geom[3] != 570 {
				t.Errorf("expected set_window_geometry width=780, height=570, got %+v", geom)
			}
		case "surface_commit":
			commitCount++
			if commitIdx == -1 {
				commitIdx = i
			}
		}
	}

	if ackIdx == -1 {
		t.Fatal("expected ack_configure event")
	}
	if geomIdx == -1 {
		t.Fatal("expected set_window_geometry event")
	}
	if commitIdx == -1 {
		t.Fatal("expected surface_commit event")
	}
	if ackIdx > commitIdx {
		t.Errorf("ack_configure (index %d) occurred after surface_commit (index %d)", ackIdx, commitIdx)
	}
	if geomIdx > commitIdx {
		t.Errorf("set_window_geometry (index %d) occurred after surface_commit (index %d)", geomIdx, commitIdx)
	}
	if commitCount != 1 {
		t.Errorf("expected exactly 1 commit for resize configure, got %d", commitCount)
	}
}

func TestWaylandDeferredAtomicResizeAndConfigureAck(t *testing.T) {
	type protocolEvent struct {
		name string
		val  any
	}
	var events []protocolEvent
	var nextCallbackId uintptr = 0x555

	const (
		mockSurface    = 0x222
		mockXdgSurface = 0x333
	)

	api := &waylandAPI{
		proxyMarshalFlags: func(proxy uintptr, opcode uint32, interfacePtr uintptr, version uint32, flags uint32, args ...any) uintptr {
			if proxy == mockXdgSurface {
				switch opcode {
				case xdgSurfaceAckConfigureOpcode:
					if len(args) > 0 {
						events = append(events, protocolEvent{name: "ack_configure", val: args[0]})
					}
				case xdgSurfaceSetWindowGeometryOpcode:
					if len(args) >= 4 {
						events = append(events, protocolEvent{name: "set_window_geometry", val: [4]int32{args[0].(int32), args[1].(int32), args[2].(int32), args[3].(int32)}})
					}
				}
			} else if proxy == mockSurface {
				switch opcode {
				case wlSurfaceFrameOpcode:
					id := nextCallbackId
					nextCallbackId++
					events = append(events, protocolEvent{name: "surface_frame", val: id})
					return id
				case wlSurfaceCommitOpcode:
					events = append(events, protocolEvent{name: "surface_commit", val: proxy})
				}
			}
			return 0
		},
		proxyAddListener: func(proxy uintptr, implementation uintptr, data unsafe.Pointer) int32 {
			return 0
		},
		proxyDestroy: func(proxy uintptr) {
			events = append(events, protocolEvent{name: "proxy_destroy", val: proxy})
		},
		displayFlush: func(display uintptr) int32 {
			events = append(events, protocolEvent{name: "display_flush", val: nil})
			return 0
		},
	}

	app := &waylandApplication{api: api, display: 0x111}
	installWaylandFrameTestCallbacks(api)
	win := &waylandWindow{
		app:          app,
		surface:      mockSurface,
		xdgSurface:   mockXdgSurface,
		viewport:     0x444,
		surfaceState: waylandSurfaceMapped,
		width:        800,
		height:       600,
		margin:       0,
	}
	state := &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			OnResize: func(size platform.Size) (platform.Frame, error) {
				events = append(events, protocolEvent{name: "on_resize", val: size})
				return platform.Frame{}, nil
			},
		},
	}
	win.state = state

	// 1. Initial resize configure (500x400, serial 101)
	win.configure.width = 500
	win.configure.height = 400
	win.handleXdgSurfaceConfigure(101)

	// Since framePending was false, this first configure triggers applyPendingResize immediately
	if !win.framePending {
		t.Fatal("expected framePending to be true after first configure commit")
	}
	if win.width != 500 || win.height != 400 {
		t.Fatalf("expected width=500, height=400, got (%d, %d)", win.width, win.height)
	}

	// Count commits so far
	commitsAfterFirst := 0
	for _, ev := range events {
		if ev.name == "surface_commit" {
			commitsAfterFirst++
		}
	}
	if commitsAfterFirst != 1 {
		t.Fatalf("expected 1 commit after first configure, got %d", commitsAfterFirst)
	}

	// The second configure supersedes the pending geometry without committing.
	win.configure.width = 520
	win.configure.height = 410
	win.handleXdgSurfaceConfigure(102)

	if win.width != 500 || win.height != 400 {
		t.Fatalf("second configure committed before its callback: got (%d, %d)", win.width, win.height)
	}
	if win.configure.serial != 102 {
		t.Fatalf("latest serial = %d, want 102", win.configure.serial)
	}

	// The third configure arrives before the same callback; only the latest
	// geometry is acknowledged and presented after that callback completes.
	win.configure.width = 540
	win.configure.height = 420
	win.handleXdgSurfaceConfigure(103)

	if win.width != 500 || win.height != 400 {
		t.Fatalf("third configure committed before its callback: got (%d, %d)", win.width, win.height)
	}
	win.onFrameDone(0x555, 0)
	if win.width != 540 || win.height != 420 {
		t.Fatalf("expected width=540, height=420, got (%d, %d)", win.width, win.height)
	}

	// Verify the intermediate configure was coalesced, and that both completed
	// frames acknowledged and committed the geometry current at their boundary.
	var acks []uint32
	var geoms [][4]int32
	totalCommits := 0
	for _, ev := range events {
		switch ev.name {
		case "surface_commit":
			totalCommits++
		case "ack_configure":
			acks = append(acks, ev.val.(uint32))
		case "set_window_geometry":
			geoms = append(geoms, ev.val.([4]int32))
		}
	}

	if totalCommits != 2 {
		t.Fatalf("expected 2 commits after coalescing three configures, got %d", totalCommits)
	}
	if len(acks) != 2 || acks[0] != 101 || acks[1] != 103 {
		t.Fatalf("expected acks [101, 103], got %v", acks)
	}
	if len(geoms) != 2 || geoms[1][2] != 540 || geoms[1][3] != 420 {
		t.Fatalf("expected latest geometry (540, 420), got %+v", geoms)
	}
}

func TestWaylandMinimumSizeHints(t *testing.T) {
	calls := 0
	var gotWidth, gotHeight int32
	api := &waylandAPI{
		proxyMarshalFlags: func(proxy uintptr, opcode uint32, interfacePtr uintptr, version uint32, flags uint32, args ...any) uintptr {
			if opcode == xdgToplevelSetMinSizeOpcode {
				calls++
				gotWidth = args[0].(int32)
				gotHeight = args[1].(int32)
			}
			return 0
		},
	}
	app := &waylandApplication{api: api}
	win := &waylandWindow{app: app, xdgToplevel: 30}

	win.setMinimumSize(400, 300)
	if calls != 1 || gotWidth != 400 || gotHeight != 300 {
		t.Fatalf("minimum size calls=%d size=%dx%d, want one 400x300 request", calls, gotWidth, gotHeight)
	}
	win.setMinimumSize(0, 0)
	if calls != 1 {
		t.Fatal("zero minimum size sent an unnecessary protocol request")
	}
}

func TestWaylandStartResizeFlushesDisplay(t *testing.T) {
	flushed := false
	resized := false
	moved := false

	api := &waylandAPI{
		proxyMarshalFlags: func(proxy uintptr, opcode uint32, interfacePtr uintptr, version uint32, flags uint32, args ...any) uintptr {
			switch opcode {
			case xdgToplevelResizeOpcode:
				resized = true
			case xdgToplevelMoveOpcode:
				moved = true
			}
			return 0
		},
		displayFlush: func(display uintptr) int32 {
			flushed = true
			return 0
		},
	}
	app := &waylandApplication{
		api:     api,
		display: 0x111,
		globals: waylandGlobals{seat: 0x222},
	}
	win := &waylandWindow{
		app:         app,
		xdgToplevel: 0x333,
	}

	flushed = false
	win.startResize(42, platform.EdgeBottomRight)
	if !resized || !flushed {
		t.Fatalf("startResize: resized=%v, flushed=%v; want true, true", resized, flushed)
	}

	flushed = false
	win.startMove(43)
	if !moved || !flushed {
		t.Fatalf("startMove: moved=%v, flushed=%v; want true, true", moved, flushed)
	}
}

func TestWaylandRestoreToSavedFloatingSize(t *testing.T) {
	var (
		resizes []platform.Size
		commits int
		acks    []uint32
	)

	api := &waylandAPI{
		proxyMarshalFlags: func(proxy uintptr, opcode uint32, interfacePtr uintptr, version uint32, flags uint32, args ...any) uintptr {
			if proxy == 0x222 { // xdgSurface
				if opcode == xdgSurfaceAckConfigureOpcode && len(args) > 0 {
					acks = append(acks, args[0].(uint32))
				}
			} else if proxy == 0x111 { // surface
				if opcode == wlSurfaceCommitOpcode {
					commits++
				}
			}
			return 0
		},
		proxyAddListener: func(proxy uintptr, implementation uintptr, data unsafe.Pointer) int32 { return 0 },
		proxyDestroy:     func(proxy uintptr) {},
		displayFlush:     func(display uintptr) int32 { return 0 },
	}

	app := &waylandApplication{api: api, display: 0x99}
	installWaylandFrameTestCallbacks(api)
	win := &waylandWindow{
		app:           app,
		surface:       0x111,
		xdgSurface:    0x222,
		xdgToplevel:   0x333,
		viewport:      0x444,
		surfaceState:  waylandSurfaceMapped,
		width:         548,
		height:        448,
		margin:        24,
		savedLogicalW: 500,
		savedLogicalH: 400,
		frameWanted:   true,
	}
	win.storeScale(1.0)
	state := &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			OnResize: func(size platform.Size) (platform.Frame, error) {
				resizes = append(resizes, size)
				return platform.Frame{}, nil
			},
		},
	}
	win.state = state

	// 1. Toplevel configure: Maximized state at 1920x1080
	statesData := []uint32{xdgToplevelStateMaximized}
	statesArray := wlArray{
		Size:  uintptr(len(statesData) * 4),
		Alloc: uintptr(len(statesData) * 4),
		Data:  unsafe.Pointer(&statesData[0]),
	}
	win.handleXdgToplevelConfigure(1920, 1080, &statesArray)
	win.handleXdgSurfaceConfigure(501)
	win.onFrameDone(99, 0)

	if !win.maximized {
		t.Fatal("expected window to be maximized")
	}
	if win.margin != 0 {
		t.Fatalf("expected margin = 0 when maximized, got %d", win.margin)
	}
	if win.width != 1920 || win.height != 1080 {
		t.Fatalf("expected maximized dimensions (1920, 1080), got (%d, %d)", win.width, win.height)
	}
	if win.savedLogicalW != 500 || win.savedLogicalH != 400 {
		t.Fatalf("expected saved floating size (500, 400), got (%g, %g)", win.savedLogicalW, win.savedLogicalH)
	}

	// 2. Unmaximize: compositor sends (0, 0)
	emptyStates := wlArray{}
	win.handleXdgToplevelConfigure(0, 0, &emptyStates)

	if win.maximized {
		t.Fatal("expected window to be unmaximized")
	}
	if win.margin != 24 {
		t.Fatalf("expected margin restored to 24, got %d", win.margin)
	}

	resizesBefore := len(resizes)
	win.handleXdgSurfaceConfigure(502)

	if win.width != 548 || win.height != 448 {
		t.Fatalf("expected restored dimensions (548, 448), got (%d, %d)", win.width, win.height)
	}
	if len(resizes) != resizesBefore+1 {
		t.Fatalf("expected OnResize called on restore, got %d calls", len(resizes)-resizesBefore)
	}
	lastResize := resizes[len(resizes)-1]
	if lastResize.Width != 500 || lastResize.Height != 400 {
		t.Fatalf("expected restored content size (500, 400), got %+v", lastResize)
	}
	if logical := win.logicalSize(); logical.Width != 500 || logical.Height != 400 {
		t.Fatalf("expected win.logicalSize() = (500, 400), got %+v", logical)
	}
}

func TestWaylandFractionalScaleTransitionsAcrossOutputs(t *testing.T) {
	var viewportDests [][2]int32
	api := &waylandAPI{
		proxyMarshalFlags: func(proxy uintptr, opcode uint32, interfacePtr uintptr, version uint32, flags uint32, args ...any) uintptr {
			if proxy == 0x444 && opcode == 2 { // wp_viewport.set_destination
				if len(args) >= 2 {
					viewportDests = append(viewportDests, [2]int32{args[0].(int32), args[1].(int32)})
				}
			}
			return 0
		},
		proxyAddListener: func(proxy uintptr, implementation uintptr, data unsafe.Pointer) int32 { return 0 },
		proxyDestroy:     func(proxy uintptr) {},
		displayFlush:     func(display uintptr) int32 { return 0 },
	}

	app := &waylandApplication{api: api, display: 0x99}
	installWaylandFrameTestCallbacks(api)
	win := &waylandWindow{
		app:           app,
		surface:       0x111,
		xdgSurface:    0x222,
		viewport:      0x444,
		surfaceState:  waylandSurfaceMapped,
		width:         448, // 400 + 2*24
		height:        348, // 300 + 2*24
		margin:        24,
		savedLogicalW: 400,
		savedLogicalH: 300,
		frameWanted:   true,
	}
	win.storeScale(1.0)
	state := &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			OnResize: func(size platform.Size) (platform.Frame, error) {
				return platform.Frame{}, nil
			},
		},
	}
	win.state = state

	transitions := []float32{1.25, 1.75, 2.0, 1.0}
	for _, targetScale := range transitions {
		// Each transition starts from a delivered frame: the previous one
		// installed a callback that stops a resize from presenting again.
		win.framePending = false
		win.frameCallback = 0
		before := len(viewportDests)
		win.applyScale(targetScale)

		if got := win.displayScale(); got != targetScale {
			t.Fatalf("displayScale = %g, want %g", got, targetScale)
		}

		wantMargin := frameMargin(targetScale)
		if win.margin != wantMargin {
			t.Fatalf("scale %g: margin = %d, want %d", targetScale, win.margin, wantMargin)
		}

		expectedContentW := deviceLength(400, targetScale)
		expectedContentH := deviceLength(300, targetScale)
		actualContentW := contentSize(win.width, win.margin)
		actualContentH := contentSize(win.height, win.margin)
		if actualContentW != expectedContentW || actualContentH != expectedContentH {
			t.Fatalf("scale %g: content size = (%d, %d), want (%d, %d)", targetScale, actualContentW, actualContentH, expectedContentW, expectedContentH)
		}

		logical := win.logicalSize()
		if math.Abs(float64(logical.Width-400)) > 1.0 || math.Abs(float64(logical.Height-300)) > 1.0 {
			t.Fatalf("scale %g: logical size = %+v, want ~(400, 300)", targetScale, logical)
		}

		if len(viewportDests) <= before {
			t.Fatalf("scale %g: expected a viewport destination update", targetScale)
		}
		latestDest := viewportDests[len(viewportDests)-1]
		wantTotalLogicalW := int32(math.Round(float64(float32(win.width) / targetScale)))
		wantTotalLogicalH := int32(math.Round(float64(float32(win.height) / targetScale)))
		if latestDest[0] != wantTotalLogicalW || latestDest[1] != wantTotalLogicalH {
			t.Fatalf("scale %g: viewport destination = (%d, %d), want (%d, %d)", targetScale, latestDest[0], latestDest[1], wantTotalLogicalW, wantTotalLogicalH)
		}
	}
}
