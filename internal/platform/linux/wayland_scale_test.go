//go:build linux

package linux

import (
	"errors"
	"fmt"
	"testing"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

func TestWaylandFractionalScaleCalculations(t *testing.T) {
	cases := []struct {
		numerator uint32
		wantScale float32
	}{
		{120, 1.0},
		{150, 1.25},
		{180, 1.5},
		{210, 1.75},
		{240, 2.0},
		{0, 1.0},
	}

	for _, c := range cases {
		scale := float32(c.numerator) / 120.0
		if scale <= 0 {
			scale = 1.0
		}
		if scale != c.wantScale {
			t.Errorf("numerator %d -> scale %g, want %g", c.numerator, scale, c.wantScale)
		}
	}
}

func TestWaylandOutputScaleFallback(t *testing.T) {
	var boundVersion uint32
	api := &waylandAPI{
		proxyMarshalFlags: func(proxy uintptr, opcode uint32, interfacePtr uintptr, version uint32, flags uint32, args ...any) uintptr {
			if opcode == wlRegistryBindOpcode {
				boundVersion = version
				return 700
			}
			return 0
		},
		proxyAddListener: func(proxy uintptr, implementation uintptr, data unsafe.Pointer) int32 {
			return 0
		},
		proxyDestroy: func(uintptr) {},
	}
	app := &waylandApplication{api: api}
	app.globals.api = api
	app.globals.onOutputScale = app.handleOutputScale
	app.globals.handleGlobal(1, 7, "wl_output", 4)
	output := app.globals.outputs[700]
	if output == nil {
		t.Fatal("wl_output global was not bound and cached")
	}
	defer output.destroy(api)
	if boundVersion != 2 {
		t.Fatalf("wl_output bound version = %d, want 2", boundVersion)
	}

	win := &waylandWindow{app: app, surface: 1, surfaceState: waylandSurfaceMapped, width: 400, height: 300, framePending: true}
	win.storeScale(1)
	state := &waylandWindowState{app: app, win: win}
	win.state = state
	app.windows = []*waylandWindowState{state}
	win.handleSurfaceEnter(700)
	if got := win.displayScale(); got != 1 {
		t.Fatalf("initial output scale = %g, want 1", got)
	}

	puregoInvokeCallback3(rawOutputListener[3], unsafe.Pointer(output), 700, 2)
	if got := win.displayScale(); got != 2 {
		t.Fatalf("wl_output.scale fallback = %g, want 2", got)
	}
	if win.width != 800 || win.height != 600 {
		t.Fatalf("2x fallback buffer size = %dx%d, want 800x600", win.width, win.height)
	}
	if app.firstErr != nil || !win.resizeDeferred {
		t.Fatalf("scale backpressure error=%v resizeDeferred=%v, want nil/true", app.firstErr, win.resizeDeferred)
	}
}

func TestWaylandLateOutputAddRemoveUpdatesWindows(t *testing.T) {
	destroyed := make(map[uintptr]int)
	api := &waylandAPI{
		proxyMarshalFlags: func(_ uintptr, opcode uint32, _ uintptr, _, _ uint32, args ...any) uintptr {
			if opcode == wlRegistryBindOpcode {
				return uintptr(args[0].(uint32))
			}
			return 0
		},
		proxyAddListener: func(uintptr, uintptr, unsafe.Pointer) int32 { return 0 },
		proxyDestroy:     func(proxy uintptr) { destroyed[proxy]++ },
	}
	app := &waylandApplication{api: api}
	app.globals.api = api
	app.globals.registry = 100
	app.globals.onOutputScale = app.handleOutputScale
	app.globals.onOutputRemove = app.handleOutputRemove
	app.globals.handleGlobal(app.globals.registry, 7, "wl_output", 2)
	app.globals.handleGlobal(app.globals.registry, 8, "wl_output", 2)

	win := &waylandWindow{app: app, surface: 1, surfaceState: waylandSurfaceMapped, width: 400, height: 300, framePending: true}
	win.storeScale(1)
	state := &waylandWindowState{app: app, win: win}
	win.state = state
	app.windows = []*waylandWindowState{state}
	win.handleSurfaceEnter(7)
	win.handleSurfaceEnter(8)
	app.globals.outputs[7].setScale(2)
	if got := win.displayScale(); got != 2 {
		t.Fatalf("late output scale = %g, want 2", got)
	}

	app.globals.handleGlobalRemove(7)
	if got := win.displayScale(); got != 1 {
		t.Fatalf("scale after removing the 2x output = %g, want 1", got)
	}
	if len(win.activeOutputs) != 1 || win.activeOutputs[0] != 8 {
		t.Fatalf("active outputs after removal = %v, want [8]", win.activeOutputs)
	}
	if destroyed[7] != 1 || app.globals.outputs[7] != nil {
		t.Fatalf("removed output destroyed=%d retained=%v", destroyed[7], app.globals.outputs[7])
	}

	app.globals.handleGlobalRemove(8)
	if len(win.activeOutputs) != 0 || destroyed[8] != 1 {
		t.Fatalf("final output removal active=%v destroyed=%d", win.activeOutputs, destroyed[8])
	}
	if app.firstErr != nil || !win.resizeDeferred {
		t.Fatalf("late output backpressure error=%v resizeDeferred=%v, want nil/true", app.firstErr, win.resizeDeferred)
	}
}

func TestWaylandOutputScaleDoesNotOverridePreferredScale(t *testing.T) {
	tests := []struct {
		name       string
		fractional uintptr
		preferred  bool
	}{
		{name: "fractional scale", fractional: 1},
		{name: "preferred buffer scale", preferred: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := &waylandApplication{
				globals: waylandGlobals{
					outputs: map[uintptr]*waylandOutput{
						700: {proxy: 700, scale: 2},
					},
				},
			}
			win := &waylandWindow{
				app:                  app,
				width:                400,
				height:               300,
				fractionalScale:      test.fractional,
				preferredBufferScale: test.preferred,
			}
			win.storeScale(1)
			win.handleSurfaceEnter(700)
			if got := win.displayScale(); got != 1 {
				t.Fatalf("output fallback overrode preferred scale: got %g", got)
			}
		})
	}
}

func TestWaylandApplyScaleAndViewport(t *testing.T) {
	var viewportSetWidth, viewportSetHeight int32

	api := &waylandAPI{
		proxyMarshalFlags: func(proxy uintptr, opcode uint32, interfacePtr uintptr, version uint32, flags uint32, args ...any) uintptr {
			if opcode == 2 && len(args) >= 2 { // wp_viewport.set_destination
				if w, ok := args[0].(int32); ok {
					viewportSetWidth = w
				}
				if h, ok := args[1].(int32); ok {
					viewportSetHeight = h
				}
			}
			return 0
		},
	}
	app := &waylandApplication{api: api}
	installWaylandFrameTestCallbacks(api)

	// 800x600 logical at 1.0x with margin 24
	win := &waylandWindow{
		app:          app,
		surface:      10,
		xdgSurface:   20,
		viewport:     30,
		surfaceState: waylandSurfaceMapped,
		width:        848, // 800 + 2*24
		height:       648, // 600 + 2*24
		margin:       24,
		frameWanted:  true,
	}
	win.storeScale(1.0)

	var reportedSize platform.Size
	state := &waylandWindowState{
		app: app,
		win: win,
		config: platform.Config{
			OnResize: func(size platform.Size) (platform.Frame, error) {
				reportedSize = size
				return platform.Frame{}, nil
			},
		},
	}
	win.state = state

	// Apply 1.5x scale
	win.applyScale(1.5)

	if win.displayScale() != 1.5 {
		t.Fatalf("expected displayScale 1.5, got %g", win.displayScale())
	}
	if win.margin != 36 {
		t.Fatalf("expected margin 36 at 1.5x, got %d", win.margin)
	}

	// 800 * 1.5 = 1200 + 2*36 = 1272
	if win.width != 1272 || win.height != 972 {
		t.Fatalf("expected total size 1272x972, got %dx%d", win.width, win.height)
	}

	// Viewport destination must equal total surface logical dimensions (848x648 at 1.5x with margin 24)
	if viewportSetWidth != 848 || viewportSetHeight != 648 {
		t.Fatalf("expected viewport destination (848, 648), got (%d, %d)", viewportSetWidth, viewportSetHeight)
	}

	// Logical size reported to application must remain 800x600 with scale 1.5
	if reportedSize.Width != 800 || reportedSize.Height != 600 || reportedSize.Scale != 1.5 {
		t.Fatalf("expected logical size (800x600 @ 1.5x), got %v", reportedSize)
	}
}

func TestWaylandScaleCapabilityPairing(t *testing.T) {
	tests := []struct {
		name               string
		scaleMgr           uintptr
		viewporter         uintptr
		viewportFails      bool
		wantFractional     uintptr
		wantViewport       uintptr
		wantFractionalCall int
		wantViewportCall   int
		wantDestroyed      bool
	}{
		{name: "matched capabilities", scaleMgr: 100, viewporter: 200, wantFractional: 501, wantViewport: 502, wantFractionalCall: 1, wantViewportCall: 1},
		{name: "missing viewporter", scaleMgr: 100},
		{name: "missing scale manager", viewporter: 200},
		{name: "viewport allocation failure", scaleMgr: 100, viewporter: 200, viewportFails: true, wantFractionalCall: 1, wantViewportCall: 1, wantDestroyed: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fractionalCalls := 0
			viewportCalls := 0
			var destroyed []uintptr
			api := &waylandAPI{
				proxyMarshalFlags: func(_ uintptr, _ uint32, interfacePtr uintptr, _, _ uint32, _ ...any) uintptr {
					switch interfacePtr {
					case uintptr(unsafe.Pointer(&wpFractionalScaleV1Interface)):
						fractionalCalls++
						return 501
					case uintptr(unsafe.Pointer(&wpViewportInterface)):
						viewportCalls++
						if test.viewportFails {
							return 0
						}
						return 502
					}
					return 0
				},
				proxyAddListener: func(uintptr, uintptr, unsafe.Pointer) int32 { return 0 },
				proxyDestroy:     func(proxy uintptr) { destroyed = append(destroyed, proxy) },
			}
			app := &waylandApplication{api: api}
			app.globals.scaleMgr = test.scaleMgr
			app.globals.viewporter = test.viewporter
			win := &waylandWindow{app: app, surface: 10, width: 400, height: 300}
			win.storeScale(1)

			app.setupWindowScale(win)
			if win.fractionalScale != test.wantFractional || win.viewport != test.wantViewport {
				t.Fatalf("capabilities fractional=%d viewport=%d, want %d/%d", win.fractionalScale, win.viewport, test.wantFractional, test.wantViewport)
			}
			if fractionalCalls != test.wantFractionalCall || viewportCalls != test.wantViewportCall {
				t.Fatalf("requests fractional=%d viewport=%d, want %d/%d", fractionalCalls, viewportCalls, test.wantFractionalCall, test.wantViewportCall)
			}
			if test.wantDestroyed && (len(destroyed) != 1 || destroyed[0] != 501) {
				t.Fatalf("destroyed fractional proxies = %v, want [501]", destroyed)
			}
		})
	}
}

func TestWaylandScaleRoundTripWithoutViewport(t *testing.T) {
	for _, size := range []struct{ width, height uint32 }{{401, 301}, {400, 300}, {403, 297}} {
		t.Run(fmt.Sprintf("%dx%d", size.width, size.height), func(t *testing.T) {
			fractionalCalls := 0
			var bufferScales []int32
			api := &waylandAPI{
				proxyMarshalFlags: func(_ uintptr, opcode uint32, interfacePtr uintptr, _, _ uint32, args ...any) uintptr {
					if interfacePtr == uintptr(unsafe.Pointer(&wpFractionalScaleV1Interface)) {
						fractionalCalls++
						return 501
					}
					if opcode == wlSurfaceSetBufferScaleOpcode {
						bufferScales = append(bufferScales, args[0].(int32))
					}
					return 0
				},
				proxyAddListener: func(uintptr, uintptr, unsafe.Pointer) int32 { return 0 },
				proxyDestroy:     func(uintptr) {},
			}
			app := &waylandApplication{api: api}
			app.globals.scaleMgr = 100
			win := &waylandWindow{
				app: app, surface: 10, width: size.width, height: size.height,
				savedLogicalW: float32(size.width), savedLogicalH: float32(size.height),
			}
			win.storeScale(1)
			app.setupWindowScale(win)
			if win.fractionalScale != 0 || win.viewport != 0 || fractionalCalls != 0 {
				t.Fatal("scale manager without viewporter enabled fractional scaling")
			}

			win.applyScale(1.25)
			win.applyScale(1)
			if win.width != size.width || win.height != size.height {
				t.Fatalf("round-trip size = %dx%d, want %dx%d", win.width, win.height, size.width, size.height)
			}
			if win.savedLogicalW != float32(size.width) || win.savedLogicalH != float32(size.height) || win.configure.width != int32(size.width) || win.configure.height != int32(size.height) {
				t.Fatalf("round-trip logical state saved=%gx%g pending=%dx%d", win.savedLogicalW, win.savedLogicalH, win.configure.width, win.configure.height)
			}
			if len(bufferScales) != 2 || bufferScales[0] != 2 || bufferScales[1] != 1 {
				t.Fatalf("buffer scale fallback = %v, want [2 1]", bufferScales)
			}
		})
	}
}

func TestWaylandScaleErrorsStopTransition(t *testing.T) {
	callbackErr := errors.New("scale callback failed")
	routerErr := errors.New("scale present failed")
	tests := []struct {
		name            string
		margin          uint32
		device          uintptr
		commandPool     uintptr
		noRenderer      bool
		callbackErr     error
		routerErr       error
		wantErr         error
		wantResizeCalls int
	}{
		{name: "frame margin", margin: 24, device: 1, commandPool: 1},
		{name: "renderer scale", device: 1, commandPool: 1},
		{name: "renderer resize"},
		{name: "resize callback", noRenderer: true, callbackErr: callbackErr, wantErr: callbackErr, wantResizeCalls: 1},
		{name: "router present", noRenderer: true, routerErr: routerErr, wantErr: routerErr, wantResizeCalls: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := &waylandApplication{api: &waylandAPI{}}
			installWaylandFrameTestCallbacks(app.api)
			win := &waylandWindow{
				app:          app,
				surface:      1,
				surfaceState: waylandSurfaceMapped,
				width:        800,
				height:       600,
				margin:       test.margin,
			}
			win.storeScale(1)
			resizeCalls := 0
			state := &waylandWindowState{
				app: app,
				win: win,
				config: platform.Config{
					OnResize: func(platform.Size) (platform.Frame, error) {
						resizeCalls++
						return platform.Frame{}, test.callbackErr
					},
				},
			}
			state.router.present = func(platform.Frame) error { return test.routerErr }
			if !test.noRenderer {
				state.renderer = &vulkanRenderer{
					api: &vulkanAPI{
						deviceWaitIdle: func(uintptr) int32 { return vkErrorDeviceLost },
					},
					device:      test.device,
					commandPool: test.commandPool,
					width:       800,
					height:      600,
					scale:       1,
				}
				state.renderer.frame.margin = test.margin
			}
			win.state = state

			win.applyScale(1.5)
			if app.firstErr == nil {
				t.Fatal("scale failure did not reach the application")
			}
			if test.wantErr != nil && !errors.Is(app.firstErr, test.wantErr) {
				t.Fatalf("application error = %v, want %v", app.firstErr, test.wantErr)
			}
			if resizeCalls != test.wantResizeCalls {
				t.Fatalf("OnResize calls = %d, want %d", resizeCalls, test.wantResizeCalls)
			}
		})
	}
}

func TestWaylandSurfaceEnterLeaveOutput(t *testing.T) {
	win := &waylandWindow{}

	// Enter output 1
	win.handleSurfaceEnter(1)
	if len(win.activeOutputs) != 1 || win.activeOutputs[0] != 1 {
		t.Fatalf("expected activeOutputs [1], got %v", win.activeOutputs)
	}

	// Enter output 2 (dual monitor spanning)
	win.handleSurfaceEnter(2)
	if len(win.activeOutputs) != 2 || win.activeOutputs[1] != 2 {
		t.Fatalf("expected activeOutputs [1, 2], got %v", win.activeOutputs)
	}

	// Leave output 1
	win.handleSurfaceLeave(1)
	if len(win.activeOutputs) != 1 || win.activeOutputs[0] != 2 {
		t.Fatalf("expected activeOutputs [2], got %v", win.activeOutputs)
	}

	// Leave output 2
	win.handleSurfaceLeave(2)
	if len(win.activeOutputs) != 0 {
		t.Fatalf("expected activeOutputs empty, got %v", win.activeOutputs)
	}
}

func TestWaylandScaledWindowGeometryAndInputRegion(t *testing.T) {
	var (
		setGeomX, setGeomY, setGeomW, setGeomH                 int32
		addedRegionX, addedRegionY, addedRegionW, addedRegionH int32
	)

	api := &waylandAPI{
		proxyMarshalFlags: func(proxy uintptr, opcode uint32, interfacePtr uintptr, version uint32, flags uint32, args ...any) uintptr {
			if opcode == xdgSurfaceSetWindowGeometryOpcode && len(args) >= 4 {
				setGeomX = args[0].(int32)
				setGeomY = args[1].(int32)
				setGeomW = args[2].(int32)
				setGeomH = args[3].(int32)
			}
			if opcode == wlRegionAddOpcode && len(args) >= 4 {
				addedRegionX = args[0].(int32)
				addedRegionY = args[1].(int32)
				addedRegionW = args[2].(int32)
				addedRegionH = args[3].(int32)
			}
			if opcode == wlCompositorCreateRegionOpcode {
				return 999
			}
			return 0
		},
		proxyDestroy: func(proxy uintptr) {},
	}

	app := &waylandApplication{
		api: api,
		globals: waylandGlobals{
			compositor: 888,
		},
	}

	win := &waylandWindow{
		app:        app,
		surface:    10,
		xdgSurface: 20,
		width:      1272, // 1200 + 2*36
		height:     972,  // 900 + 2*36
		margin:     36,   // 24 * 1.5
	}
	win.storeScale(1.5)

	win.updateWindowGeometry()
	if setGeomX != 24 || setGeomY != 24 || setGeomW != 800 || setGeomH != 600 {
		t.Fatalf("expected logical geometry (24, 24, 800, 600), got (%d, %d, %d, %d)", setGeomX, setGeomY, setGeomW, setGeomH)
	}

	win.updateInputRegion()
	// grip at 1.5x = 8*1.5 = 12. reach = min(12, 36) = 12.
	// margin - reach = 36 - 12 = 24 physical -> 24 / 1.5 = 16 logical
	// cW + 2*reach = 1200 + 24 = 1224 physical -> 1224 / 1.5 = 816 logical
	if addedRegionX != 16 || addedRegionY != 16 || addedRegionW != 816 || addedRegionH != 616 {
		t.Fatalf("expected logical input region (16, 16, 816, 616), got (%d, %d, %d, %d)", addedRegionX, addedRegionY, addedRegionW, addedRegionH)
	}
}
