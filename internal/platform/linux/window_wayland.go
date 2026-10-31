//go:build linux

package linux

import (
	"fmt"
	"math"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

var (
	windowListenersOnce    sync.Once
	rawXdgSurfaceListener  [1]uintptr
	rawXdgToplevelListener [4]uintptr
	rawDecorationListener  [1]uintptr
	pinnerWindowListeners  runtime.Pinner
)

func initWindowListeners() {
	windowListenersOnce.Do(func() {
		// xdg_surface.configure(serial)
		rawXdgSurfaceListener[0] = purego.NewCallback(func(data unsafe.Pointer, xdgSurface uintptr, serial uint32) {
			if data == nil {
				return
			}
			win := (*waylandWindow)(data)
			win.handleXdgSurfaceConfigure(serial)
		})

		// xdg_toplevel.configure(width, height, states)
		rawXdgToplevelListener[0] = purego.NewCallback(func(data unsafe.Pointer, xdgToplevel uintptr, width int32, height int32, states unsafe.Pointer) {
			if data == nil {
				return
			}
			win := (*waylandWindow)(data)
			win.handleXdgToplevelConfigure(width, height, (*wlArray)(states))
		})

		// xdg_toplevel.close()
		rawXdgToplevelListener[1] = purego.NewCallback(func(data unsafe.Pointer, xdgToplevel uintptr) {
			if data == nil {
				return
			}
			win := (*waylandWindow)(data)
			win.handleClose()
		})

		// xdg_toplevel.configure_bounds(width, height)
		rawXdgToplevelListener[2] = purego.NewCallback(func(data unsafe.Pointer, xdgToplevel uintptr, width int32, height int32) {
			// Optional bounds configuration from compositor
		})

		// xdg_toplevel.wm_capabilities(capabilities)
		rawXdgToplevelListener[3] = purego.NewCallback(func(data unsafe.Pointer, xdgToplevel uintptr, capabilities unsafe.Pointer) {
			// Optional compositor capabilities
		})

		// zxdg_toplevel_decoration_v1.configure(mode)
		rawDecorationListener[0] = purego.NewCallback(func(data unsafe.Pointer, decoration uintptr, mode uint32) {
			if data == nil {
				return
			}
			win := (*waylandWindow)(data)
			win.handleDecorationConfigure(mode)
		})

		pinnerWindowListeners.Pin(&rawXdgSurfaceListener)
		pinnerWindowListeners.Pin(&rawXdgToplevelListener)
		pinnerWindowListeners.Pin(&rawDecorationListener)
	})
}

// waylandFrameGeometry is one immutable geometry generation published to Frame.
type waylandFrameGeometry struct {
	width, height uint32
	margin        uint32
	scale         float32
}

// waylandSurfaceState is the protocol lifecycle owned by the event-loop thread.
type waylandSurfaceState uint8

const (
	waylandSurfaceCreated waylandSurfaceState = iota
	waylandSurfaceAwaitInitialConfigure
	waylandSurfaceMapped
	waylandSurfaceUnmapped
	waylandSurfaceClosed
)

// waylandConfigureTransaction owns one configure sequence from its toplevel
// geometry event through the serial ACK paired with a buffer commit.
type waylandConfigureTransaction struct {
	serial          uint32
	width, height   int32
	toplevelPending bool
}

// waylandWindow coordinates an XDG-Shell toplevel window, surface state, and frame rendering.
type waylandWindow struct {
	mu sync.Mutex

	app            *waylandApplication
	surface        uintptr
	xdgSurface     uintptr
	xdgToplevel    uintptr
	decoration     uintptr
	decorationMode uint32

	fractionalScale      uintptr
	viewport             uintptr
	activeOutputs        []uintptr
	preferredBufferScale bool

	width, height uint32 // device pixels
	scaleBits     atomic.Uint32
	frameGeometry atomic.Pointer[waylandFrameGeometry]
	margin        uint32
	savedLogicalW float32
	savedLogicalH float32
	frameWanted   bool
	geometryDirty bool
	lastFrame     ui.Rect

	surfaceState   waylandSurfaceState
	configure      waylandConfigureTransaction
	resizeDeferred bool
	framePending   bool
	frameCallback  uintptr
	// mappedOnce records that the surface has presented at least once. Weston
	// sends no configure for a remap and accepts a buffer without a new serial;
	// Show uses this to scope its bounded fallback to such a compositor.
	mappedOnce    bool
	remapDeadline time.Time
	needsRedraw   bool
	forcePresent  bool
	maximized     bool
	fullscreen    bool
	resizing      bool
	activated     bool
	// focusLostSent says the window's IME, AT-SPI object and application have
	// heard that the keyboard left for the current focus span. A blur reaches
	// us as a toplevel deactivation, a wl_keyboard.leave or a seat capability
	// removal; whichever arrives first tells them, the rest stay quiet.
	focusLostSent bool
	closed        atomic.Bool

	pointerX, pointerY float32
	lastClickTime      uint32
	// pendingMove records a press on a move region under a backdrop
	// (PressMoveOnDrag). The application receives the press; the move starts
	// only once the pointer drags past the slop, using the press serial.
	pendingMove       bool
	pendingMoveSerial uint32
	pendingMoveX      float32
	pendingMoveY      float32

	pinner runtime.Pinner
	pinned bool
	state  *waylandWindowState
}

// waylandWindowState maintains platform window state and render loop pipeline.
type waylandWindowState struct {
	app        *waylandApplication
	win        *waylandWindow
	config     platform.Config
	renderer   *vulkanRenderer
	router     eventRouter
	applyFrame func(platform.Frame) (bool, error)
	text       *linuxTextInput
	profile    bool
	started    time.Time
}

// displayScale returns the device pixels per logical pixel for this window.
func (win *waylandWindow) displayScale() float32 {
	if bits := win.scaleBits.Load(); bits != 0 {
		return math.Float32frombits(bits)
	}
	return 1
}

func (win *waylandWindow) storeScale(scale float32) {
	win.scaleBits.Store(math.Float32bits(scale))
	win.publishFrameGeometry()
}

// publishFrameGeometry releases one complete geometry generation to Frame.
// The event-loop thread owns the fields; readers never touch them directly.
func (win *waylandWindow) publishFrameGeometry() {
	win.frameGeometry.Store(&waylandFrameGeometry{
		width: win.width, height: win.height, margin: win.margin,
		scale: win.displayScale(),
	})
}

// logicalSize computes the content area size in logical pixels.
func (win *waylandWindow) logicalSize() platform.Size {
	scale := win.displayScale()
	return platform.Size{
		Width:  float32(contentSize(win.width, win.margin)) / scale,
		Height: float32(contentSize(win.height, win.margin)) / scale,
		Scale:  scale,
	}
}

// pendingLogicalSize returns the effective geometry of the latest configure.
// A zero-size floating configure restores the saved logical window size.
func (win *waylandWindow) pendingLogicalSize() (int32, int32) {
	w, h := win.configure.width, win.configure.height
	if (w <= 0 || h <= 0) && !win.maximized && !win.fullscreen && win.savedLogicalW > 0 && win.savedLogicalH > 0 {
		w = int32(math.Round(float64(win.savedLogicalW)))
		h = int32(math.Round(float64(win.savedLogicalH)))
	}
	return w, h
}

func (win *waylandWindow) canPresent() bool {
	if win.closed.Load() || win.surface == 0 {
		return false
	}
	switch win.surfaceState {
	case waylandSurfaceMapped:
		return true
	case waylandSurfaceAwaitInitialConfigure:
		return win.configure.serial != 0
	default:
		return false
	}
}

// beginRemap performs the empty commit that requests a fresh configure. The
// latest hidden geometry remains pending, but its old serial cannot be reused.
func (win *waylandWindow) beginRemap() {
	win.surfaceState = waylandSurfaceAwaitInitialConfigure
	win.configure.serial = 0
	win.configure.toplevelPending = false
	win.remapDeadline = time.Time{}
	win.needsRedraw = true
	if win.surface != 0 && win.app != nil && win.app.api != nil {
		win.app.api.surfaceCommit(win.surface)
		if win.app.api.displayFlush != nil && win.app.display != 0 {
			win.app.api.displayFlush(win.app.display)
		}
	}
}

func (win *waylandWindow) ackConfigure() {
	if win.app != nil && win.app.api != nil && win.xdgSurface != 0 && win.configure.serial != 0 {
		win.app.api.xdgSurfaceAckConfigure(win.xdgSurface, win.configure.serial)
		win.configure.serial = 0
	}
}

func (win *waylandWindow) reportFrame() {
	if win.state == nil || win.state.config.OnFrameChange == nil || win.maximized || win.fullscreen {
		return
	}
	scale := win.displayScale()
	frame := ui.Rect{
		Width:  float32(contentSize(win.width, win.margin)) / scale,
		Height: float32(contentSize(win.height, win.margin)) / scale,
	}
	if frame == win.lastFrame {
		return
	}
	win.lastFrame = frame
	win.state.config.OnFrameChange(frame)
}

func (win *waylandWindow) updateWindowGeometry() {
	if win.xdgSurface == 0 || win.app == nil || win.app.api == nil {
		return
	}
	scale := win.displayScale()
	if scale <= 0 {
		scale = 1.0
	}
	if win.margin > 0 {
		geomX := int32(math.Round(float64(float32(win.margin) / scale)))
		geomY := int32(math.Round(float64(float32(win.margin) / scale)))
		geomW := int32(math.Round(float64(float32(contentSize(win.width, win.margin)) / scale)))
		geomH := int32(math.Round(float64(float32(contentSize(win.height, win.margin)) / scale)))
		win.app.api.xdgSurfaceSetWindowGeometry(win.xdgSurface, geomX, geomY, geomW, geomH)
	} else {
		geomW := int32(math.Round(float64(float32(win.width) / scale)))
		geomH := int32(math.Round(float64(float32(win.height) / scale)))
		win.app.api.xdgSurfaceSetWindowGeometry(win.xdgSurface, 0, 0, geomW, geomH)
	}
}

func (win *waylandWindow) updateInputRegion() {
	if win.surface == 0 || win.app == nil || win.app.api == nil || win.app.globals.compositor == 0 {
		return
	}
	if win.margin > 0 {
		scale := win.displayScale()
		if scale <= 0 {
			scale = 1.0
		}
		grip := frameGrip(scale)
		cW := contentSize(win.width, win.margin)
		cH := contentSize(win.height, win.margin)
		reach := min(grip, win.margin)
		rx := int32(math.Round(float64(float32(win.margin-reach) / scale)))
		ry := int32(math.Round(float64(float32(win.margin-reach) / scale)))
		rw := int32(math.Round(float64(float32(cW+2*reach) / scale)))
		rh := int32(math.Round(float64(float32(cH+2*reach) / scale)))

		region := win.app.api.createRegion(win.app.globals.compositor)
		if region != 0 {
			win.app.api.regionAdd(region, rx, ry, rw, rh)
			win.app.api.surfaceSetInputRegion(win.surface, region)
			win.app.api.destroyRegion(region)
		}
	} else {
		// Reset input region to default (whole surface)
		win.app.api.surfaceSetInputRegion(win.surface, 0)
	}
}

func (win *waylandWindow) updateViewportDestination() {
	if win.viewport == 0 || win.app == nil || win.app.api == nil {
		return
	}
	scale := win.displayScale()
	if scale <= 0 {
		scale = 1.0
	}
	totalLogicalW := int32(math.Round(float64(float32(win.width) / scale)))
	totalLogicalH := int32(math.Round(float64(float32(win.height) / scale)))
	if totalLogicalW <= 0 {
		totalLogicalW = 1
	}
	if totalLogicalH <= 0 {
		totalLogicalH = 1
	}
	win.app.api.viewportSetDestination(win.viewport, totalLogicalW, totalLogicalH)
}

// presentInputFrame applies an event-driven scene update, then routes any
// actual presentation through the same lifecycle gates as a requested redraw.
func (win *waylandWindow) presentInputFrame(frame platform.Frame) error {
	if win.closed.Load() || win.surface == 0 || win.state == nil || win.state.applyFrame == nil {
		return nil
	}
	changed, err := win.state.applyFrame(frame)
	if err != nil || !changed {
		return err
	}
	win.needsRedraw = true
	if win.configure.toplevelPending {
		win.resizeDeferred = true
		return nil
	}
	if !win.canPresent() || win.framePending {
		return nil
	}
	if win.configure.serial != 0 || win.forcePresent || win.geometryDirty {
		win.renderFrame()
		return nil
	}
	if win.state.renderer == nil {
		return nil
	}
	if err := win.presentWithFrameCallback(win.state.renderer.present, nil); err != nil {
		return err
	}
	win.needsRedraw = false
	return nil
}

// renderFrame applies new content, then atomically acknowledges and presents
// only when either the scene changed or a protocol transition requires a frame.
func (win *waylandWindow) renderFrame() {
	if win.closed.Load() || win.surface == 0 || win.state == nil {
		return
	}
	if win.configure.toplevelPending {
		win.resizeDeferred = true
		win.needsRedraw = true
		return
	}
	if !win.canPresent() {
		win.needsRedraw = true
		return
	}
	if win.framePending {
		win.needsRedraw = true
		return
	}
	redrawRequested := win.needsRedraw
	win.needsRedraw = false

	ackConfigure := win.ackConfigure
	var err error
	if win.state.renderer != nil && win.state.config.OnRedraw != nil && win.state.applyFrame != nil {
		var frame platform.Frame
		frame, err = win.state.config.OnRedraw()
		if err == nil {
			var changed bool
			changed, err = win.state.applyFrame(frame)
			if err == nil {
				if !changed && !win.forcePresent && !win.geometryDirty {
					return
				}
				if win.geometryDirty {
					win.updateWindowGeometry()
					win.updateInputRegion()
					win.updateViewportDestination()
				}
				err = win.presentWithFrameCallback(win.state.renderer.present, ackConfigure)
			}
		}
	} else {
		// States without a separate scene applier use the legacy callback path;
		// openWindow always installs applyFrame for real windows.
		if win.geometryDirty {
			win.updateWindowGeometry()
			win.updateInputRegion()
			win.updateViewportDestination()
		}
		if win.state.renderer != nil {
			if win.state.config.OnRedraw != nil {
				var frame platform.Frame
				frame, err = win.state.config.OnRedraw()
				if err == nil && win.state.router.present != nil {
					err = win.presentWithFrameCallback(func() error {
						return win.state.router.present(frame)
					}, ackConfigure)
				}
			} else {
				err = win.presentWithFrameCallback(win.state.renderer.present, ackConfigure)
			}
		} else if win.app != nil && win.app.api != nil {
			err = win.presentWithFrameCallback(func() error {
				win.app.api.surfaceCommit(win.surface)
				return nil
			}, ackConfigure)
		}
	}
	if err != nil {
		win.needsRedraw = redrawRequested
		if win.app != nil {
			win.app.fail(err)
		}
		return
	}
	win.geometryDirty = false
	win.forcePresent = false
}

func (win *waylandWindow) applyFrameMargin(margin uint32) bool {
	if win.state == nil || win.state.renderer == nil {
		return true
	}
	if err := win.state.renderer.setFrameMargin(margin); err != nil {
		if win.app != nil {
			win.app.fail(err)
		}
		return false
	}
	return true
}

func (win *waylandWindow) handleXdgToplevelConfigure(width, height int32, states *wlArray) {
	if win.closed.Load() || win.surfaceState == waylandSurfaceClosed {
		return
	}
	maximized := false
	fullscreen := false
	resizing := false
	activated := false

	if states != nil && states.Data != nil && states.Size >= 4 {
		count := int(states.Size / 4)
		stateSlice := unsafe.Slice((*uint32)(states.Data), count)
		for _, s := range stateSlice {
			switch s {
			case xdgToplevelStateMaximized:
				maximized = true
			case xdgToplevelStateFullscreen:
				fullscreen = true
			case xdgToplevelStateResizing:
				resizing = true
			case xdgToplevelStateActivated:
				activated = true
			}
		}
	}

	win.resizing = resizing
	if win.activated != activated {
		win.activated = activated
		if activated {
			// A new focus span: the next blur is news again.
			win.focusLostSent = false
		} else {
			if win.state != nil && win.state.config.WindowMode == platform.WindowPanel {
				win.Hide()
			}
			focusError := notifyFocusLost(win)
			if win.app != nil {
				win.app.fail(focusError)
			}
			// A failed callback tears the window down.
			if win.closed.Load() {
				return
			}
		}
	}

	if win.maximized != maximized || win.fullscreen != fullscreen {
		oldMax := win.maximized || win.fullscreen
		newMax := maximized || fullscreen
		win.maximized = maximized
		win.fullscreen = fullscreen
		if oldMax != newMax {
			if newMax {
				// Strip margin on maximized/fullscreen
				win.margin = 0
				if !win.applyFrameMargin(0) {
					return
				}
			} else if win.frameWanted {
				// Restore margin on unmaximize
				win.margin = frameMargin(win.displayScale())
				if !win.applyFrameMargin(win.margin) {
					return
				}
			}
			win.publishFrameGeometry()
			win.geometryDirty = true
			if win.state != nil && win.state.config.OnMaximize != nil {
				win.state.config.OnMaximize(maximized)
			}
		}
	}

	win.configure.width = width
	win.configure.height = height
	win.configure.toplevelPending = true
}

func (win *waylandWindow) applyPendingResize() {
	if win.closed.Load() || win.surface == 0 {
		return
	}
	if win.configure.toplevelPending {
		// A toplevel configure is incomplete until xdg_surface.configure supplies
		// its serial. Scale changes must not present the new geometry before then.
		win.resizeDeferred = true
		return
	}
	if win.state != nil && !win.canPresent() {
		win.resizeDeferred = true
		win.needsRedraw = true
		return
	}
	if win.framePending {
		win.resizeDeferred = true
		return
	}
	win.resizeDeferred = false

	scale := win.displayScale()
	w, h := win.pendingLogicalSize()
	if w > 0 && h > 0 {
		if !win.maximized && !win.fullscreen {
			win.savedLogicalW = float32(w)
			win.savedLogicalH = float32(h)
		}
		newWidth := deviceLength(int(w), scale)
		newHeight := deviceLength(int(h), scale)
		if win.margin > 0 {
			newWidth += 2 * win.margin
			newHeight += 2 * win.margin
		}
		if newWidth != win.width || newHeight != win.height {
			win.width = newWidth
			win.height = newHeight
			win.publishFrameGeometry()
			win.geometryDirty = true
		}
	}

	// Before a renderer exists, retain the compositor's geometry. A later
	// buffer transaction acknowledges the serial with the first present.
	if win.state == nil {
		return
	}

	ackConfigure := win.ackConfigure
	var err error
	if win.state.renderer != nil {
		err = win.state.renderer.resize(win.width, win.height)
		if err != nil {
			if win.app != nil {
				win.app.fail(err)
			}
			return
		}
	}

	var frame platform.Frame
	hasResizeFrame := false
	if win.state.config.OnResize != nil {
		frame, err = win.state.config.OnResize(win.logicalSize())
		if err != nil {
			if win.app != nil {
				win.app.fail(err)
			}
			return
		}
		hasResizeFrame = true
	}
	hasRedrawFrame := false
	if !hasResizeFrame && win.needsRedraw && win.state.config.OnRedraw != nil && (win.state.applyFrame != nil || win.state.router.present != nil) {
		frame, err = win.state.config.OnRedraw()
		if err != nil {
			if win.app != nil {
				win.app.fail(err)
			}
			return
		}
		hasRedrawFrame = true
	}
	hasSceneFrame := hasResizeFrame || hasRedrawFrame

	if win.state.renderer != nil && hasSceneFrame && win.state.applyFrame != nil {
		// A resize is a mandatory present even when the new scene compares clean.
		_, err = win.state.applyFrame(frame)
		if err == nil {
			if win.geometryDirty {
				win.updateWindowGeometry()
				win.updateInputRegion()
				win.updateViewportDestination()
			}
			err = win.presentWithFrameCallback(win.state.renderer.present, ackConfigure)
		}
	} else {
		if win.geometryDirty {
			win.updateWindowGeometry()
			win.updateInputRegion()
			win.updateViewportDestination()
		}
		if win.state.renderer != nil {
			if hasSceneFrame && win.state.router.present != nil {
				err = win.presentWithFrameCallback(func() error {
					return win.state.router.present(frame)
				}, ackConfigure)
			} else {
				err = win.presentWithFrameCallback(win.state.renderer.present, ackConfigure)
			}
		} else {
			err = win.presentWithFrameCallback(func() error {
				if hasSceneFrame && win.state.router.present != nil {
					if err := win.state.router.present(frame); err != nil {
						return err
					}
				}
				win.app.api.surfaceCommit(win.surface)
				return nil
			}, ackConfigure)
		}
	}
	if err != nil {
		if win.app != nil {
			win.app.fail(err)
		}
		return
	}
	win.geometryDirty = false
	if hasSceneFrame {
		win.needsRedraw = false
	}
	win.forcePresent = false
	if win.state.renderer != nil {
		win.reportFrame()
	}
}

func (win *waylandWindow) handleXdgSurfaceConfigure(serial uint32) {
	if win.closed.Load() || win.surfaceState == waylandSurfaceClosed || serial == 0 {
		return
	}
	win.configure.toplevelPending = false
	win.configure.serial = serial
	win.remapDeadline = time.Time{}
	if win.state != nil {
		win.forcePresent = true
	}

	scale := win.displayScale()
	w, h := win.pendingLogicalSize()
	resized := false

	if w > 0 && h > 0 {
		newWidth := deviceLength(int(w), scale)
		newHeight := deviceLength(int(h), scale)
		if win.margin > 0 {
			newWidth += 2 * win.margin
			newHeight += 2 * win.margin
		}
		if newWidth != win.width || newHeight != win.height {
			resized = true
		}
	}

	if resized || win.resizeDeferred {
		win.applyPendingResize()
	} else if win.state != nil {
		win.renderFrame()
	}
}

func (win *waylandWindow) handleDecorationConfigure(mode uint32) {
	win.decorationMode = mode
	if mode == zxdgToplevelDecorationModeServerSide {
		if win.frameWanted {
			win.margin = 0
			win.frameWanted = false
			if !win.applyFrameMargin(0) {
				return
			}
			win.publishFrameGeometry()
			win.geometryDirty = true
		}
	} else if mode == zxdgToplevelDecorationModeClientSide {
		if win.state != nil && win.state.config.WindowMode == platform.WindowCustomTitlebar {
			win.frameWanted = true
			win.margin = frameMargin(win.displayScale())
			if !win.applyFrameMargin(win.margin) {
				return
			}
			win.publishFrameGeometry()
			win.geometryDirty = true
		}
	}
}

func (win *waylandWindow) handleClose() {
	if win.state != nil && win.state.config.OnCloseRequest != nil && win.state.config.WindowMode != platform.WindowPanel {
		win.state.config.OnCloseRequest(func() {
			win.Close()
		})
	} else {
		win.Close()
	}
}

func (win *waylandWindow) requestClose() {
	win.app.post(func() {
		win.handleClose()
	})
}

func (win *waylandWindow) setMaximized(maximized bool) {
	if win.xdgToplevel == 0 || win.app == nil || win.app.api == nil {
		return
	}
	if maximized {
		win.app.api.xdgToplevelSetMaximized(win.xdgToplevel)
	} else {
		win.app.api.xdgToplevelUnsetMaximized(win.xdgToplevel)
	}
}

func (win *waylandWindow) toggleMaximize() {
	win.setMaximized(!win.maximized)
}

func (win *waylandWindow) setMinimumSize(width, height int) {
	if (width <= 0 && height <= 0) || win.xdgToplevel == 0 || win.app == nil || win.app.api == nil {
		return
	}
	win.app.api.xdgToplevelSetMinSize(win.xdgToplevel, int32(width), int32(height))
}

func (win *waylandWindow) minimize() {
	if win.xdgToplevel != 0 && win.app != nil && win.app.api != nil {
		win.app.api.xdgToplevelSetMinimized(win.xdgToplevel)
	}
}

func (win *waylandWindow) startMove(serial uint32) {
	if win.xdgToplevel != 0 && win.app != nil && win.app.api != nil && win.app.globals.seat != 0 {
		win.app.api.xdgToplevelMove(win.xdgToplevel, win.app.globals.seat, serial)
		if win.app.api.displayFlush != nil && win.app.display != 0 {
			win.app.api.displayFlush(win.app.display)
		}
	}
}

func (win *waylandWindow) startResize(serial uint32, edge platform.WindowEdge) {
	if win.xdgToplevel != 0 && win.app != nil && win.app.api != nil && win.app.globals.seat != 0 {
		waylandEdge := xdgResizeEdge(edge)
		win.app.api.xdgToplevelResize(win.xdgToplevel, win.app.globals.seat, serial, waylandEdge)
		if win.app.api.displayFlush != nil && win.app.display != 0 {
			win.app.api.displayFlush(win.app.display)
		}
	}
}

const (
	xdgToplevelResizeEdgeNone        uint32 = 0
	xdgToplevelResizeEdgeTop         uint32 = 1
	xdgToplevelResizeEdgeBottom      uint32 = 2
	xdgToplevelResizeEdgeLeft        uint32 = 4
	xdgToplevelResizeEdgeTopLeft     uint32 = 5
	xdgToplevelResizeEdgeBottomLeft  uint32 = 6
	xdgToplevelResizeEdgeRight       uint32 = 8
	xdgToplevelResizeEdgeTopRight    uint32 = 9
	xdgToplevelResizeEdgeBottomRight uint32 = 10
)

func xdgResizeEdge(edge platform.WindowEdge) uint32 {
	switch edge {
	case platform.EdgeTop:
		return xdgToplevelResizeEdgeTop
	case platform.EdgeBottom:
		return xdgToplevelResizeEdgeBottom
	case platform.EdgeLeft:
		return xdgToplevelResizeEdgeLeft
	case platform.EdgeRight:
		return xdgToplevelResizeEdgeRight
	case platform.EdgeTopLeft:
		return xdgToplevelResizeEdgeTopLeft
	case platform.EdgeTopRight:
		return xdgToplevelResizeEdgeTopRight
	case platform.EdgeBottomLeft:
		return xdgToplevelResizeEdgeBottomLeft
	case platform.EdgeBottomRight:
		return xdgToplevelResizeEdgeBottomRight
	default:
		return xdgToplevelResizeEdgeNone
	}
}

func (win *waylandWindow) destroy() {
	if win == nil {
		return
	}
	win.closed.Store(true)
	win.surfaceState = waylandSurfaceClosed
	win.configure.serial = 0
	win.configure.toplevelPending = false
	var api *waylandAPI
	if win.app != nil {
		api = win.app.api
	}

	if api != nil {
		if win.decoration != 0 {
			api.decorationDestroy(win.decoration)
			win.decoration = 0
		}
		if win.xdgToplevel != 0 {
			api.xdgToplevelDestroy(win.xdgToplevel)
			win.xdgToplevel = 0
		}
		if win.xdgSurface != 0 {
			api.xdgSurfaceDestroy(win.xdgSurface)
			win.xdgSurface = 0
		}
		if win.frameCallback != 0 {
			api.proxyDestroy(win.frameCallback)
			win.frameCallback = 0
		}
		win.framePending = false
		win.needsRedraw = false
		if win.fractionalScale != 0 {
			api.fractionalScaleDestroy(win.fractionalScale)
			win.fractionalScale = 0
		}
		if win.viewport != 0 {
			api.viewportDestroy(win.viewport)
			win.viewport = 0
		}
	}

	// Vulkan swapchain and VkSurfaceKHR MUST be destroyed before wl_surface is destroyed!
	if win.state != nil {
		if win.state.config.Waker != nil && win.state.config.OnRedraw != nil {
			win.state.config.Waker.Unbind()
		}
		// The engine keeps this window's context focused, and its handler closures
		// point at the field and router that are about to go away. Close it first,
		// as the X11 close path does, so the daemon stops sending to a dead window.
		if win.state.router.ime != nil {
			win.state.router.ime.close()
			win.state.router.ime = nil
		}
		if win.state.renderer != nil {
			win.state.renderer.close()
			win.state.renderer = nil
		}
		if win.state.text != nil {
			win.state.text.close()
			win.state.text = nil
		}
		if win.app != nil {
			win.app.atspiClosed(win.state)
			if win.app.pointerFocus == win {
				win.app.pointerFocus = nil
			}
			if win.app.keyboardFocus == win {
				win.app.stopKeyRepeat()
				win.app.keyboardFocus = nil
			}
			if win.app.dragState != nil && win.app.dragState.win == win {
				win.app.dragState = nil
			}
			win.app.removeWindow(win.state)
		}
		if win.state.config.OnClosed != nil {
			win.state.config.OnClosed()
		}
		win.state = nil
	}

	if api != nil && win.surface != 0 {
		api.surfaceDestroy(win.surface)
		win.surface = 0
	}

	if win.pinned {
		win.pinner.Unpin()
		win.pinned = false
	}
}

// Close closes the window and releases all resources.
func (win *waylandWindow) Close() {
	if win != nil && !win.closed.Swap(true) {
		win.app.post(func() {
			win.destroy()
		})
	}
}

// Hide hides the window without destroying its state or renderer.
func (win *waylandWindow) Hide() {
	if win == nil || win.closed.Load() {
		return
	}
	win.app.post(func() {
		if win.closed.Load() || win.surfaceState == waylandSurfaceClosed {
			return
		}
		if win.surfaceState == waylandSurfaceUnmapped {
			return
		}
		win.surfaceState = waylandSurfaceUnmapped
		win.remapDeadline = time.Time{}
		if win.frameCallback != 0 && win.app != nil && win.app.api != nil {
			win.app.api.proxyDestroy(win.frameCallback)
			win.frameCallback = 0
		}
		win.framePending = false
		win.needsRedraw = false
		if win.surface != 0 && win.app != nil && win.app.api != nil {
			win.app.api.surfaceAttach(win.surface, 0, 0, 0)
			win.app.api.surfaceCommit(win.surface)
			win.app.api.displayFlush(win.app.display)
		}
	})
}

// Show starts remapping a hidden window. A newly-unmapped xdg_toplevel repeats
// the initial empty commit and waits for a new configure; for Weston, which
// sends none, a previously presented window also arms a bounded fallback.
func (win *waylandWindow) Show() {
	if win == nil || win.closed.Load() {
		return
	}
	win.app.post(func() {
		if win.closed.Load() || win.surfaceState == waylandSurfaceClosed {
			return
		}
		if win.surfaceState != waylandSurfaceUnmapped {
			return
		}
		win.beginRemap()
		if win.mappedOnce && win.app != nil && win.app.needsRemapFallback() {
			// Weston does not send the configure xdg-shell requires for a
			// remap. Do not hold the window hidden forever waiting for one:
			// fall back to the last configure after the grace period. Other
			// compositors must get their configure first.
			win.remapDeadline = win.app.timeNow().Add(waylandRemapConfigureGrace)
		}
	})
}

// remapWithoutConfigure finishes a Show on Weston, which does not answer a
// remap with a configure. The last configure still describes the surface and
// Weston keeps its state across the unmap, so committing a buffer maps it
// without a new serial.
func (win *waylandWindow) remapWithoutConfigure() {
	if win.closed.Load() || win.app == nil || !win.app.needsRemapFallback() ||
		win.surfaceState != waylandSurfaceAwaitInitialConfigure || !win.mappedOnce {
		return
	}
	win.remapDeadline = time.Time{}
	win.surfaceState = waylandSurfaceMapped
	win.needsRedraw = true
	win.forcePresent = true
	win.applyPendingResize()
}

// Frame returns the content area rectangle in logical pixels.
func (win *waylandWindow) Frame() (ui.Rect, bool) {
	if win == nil {
		return ui.Rect{}, false
	}
	geometry := win.frameGeometry.Load()
	if geometry == nil {
		return ui.Rect{}, false
	}
	scale := geometry.scale
	if scale <= 0 {
		scale = 1
	}
	w := float32(contentSize(geometry.width, geometry.margin)) / scale
	h := float32(contentSize(geometry.height, geometry.margin)) / scale
	return ui.Rect{Width: w, Height: h}, false
}

func (win *waylandWindow) contentOrigin() (x, y int32) {
	scale := win.displayScale()
	margin := int32(float32(win.margin) / scale)
	return margin, margin
}

var newWaylandWindowRenderer = newVulkanRenderer

// openWindow instantiates a new Wayland XDG-Shell window.
func (app *waylandApplication) openWindow(config platform.Config) (*waylandWindow, error) {
	if err := validateConfig(config); err != nil {
		return nil, err
	}
	if app.globals.compositor == 0 {
		return nil, fmt.Errorf("linux: compositor global not available")
	}
	if app.globals.xdgWmBase == 0 {
		return nil, fmt.Errorf("linux: xdg_wm_base global not available")
	}

	scale := float32(1)
	deviceW := deviceLength(config.Width, scale)
	deviceH := deviceLength(config.Height, scale)
	margin := uint32(0)
	frameWanted := false

	if config.WindowMode == platform.WindowCustomTitlebar {
		margin = frameMargin(scale)
		frameWanted = true
	}

	totalW := deviceW + 2*margin
	totalH := deviceH + 2*margin

	surface := app.api.createSurface(app.globals.compositor)
	if surface == 0 {
		return nil, fmt.Errorf("linux: failed to create wl_surface")
	}

	xdgSurface := app.api.getXdgSurface(app.globals.xdgWmBase, surface)
	if xdgSurface == 0 {
		app.api.surfaceDestroy(surface)
		return nil, fmt.Errorf("linux: failed to create xdg_surface")
	}

	xdgToplevel := app.api.xdgSurfaceGetToplevel(xdgSurface)
	if xdgToplevel == 0 {
		app.api.proxyDestroy(xdgSurface)
		app.api.surfaceDestroy(surface)
		return nil, fmt.Errorf("linux: failed to create xdg_toplevel")
	}

	if config.Title != "" {
		cTitle := []byte(config.Title + "\x00")
		app.api.xdgToplevelSetTitle(xdgToplevel, &cTitle[0])
	}
	appId := config.Class
	if appId == "" {
		appId = "viva"
	}
	cAppId := []byte(appId + "\x00")
	app.api.xdgToplevelSetAppId(xdgToplevel, &cAppId[0])

	var decoration uintptr
	var decorationMode uint32
	if config.WindowMode == platform.WindowTitled {
		if app.globals.decorationMgr != 0 {
			decoration = app.api.getToplevelDecoration(app.globals.decorationMgr, xdgToplevel)
			app.api.setDecorationMode(decoration, zxdgToplevelDecorationModeServerSide)
		}
	} else {
		if app.globals.decorationMgr != 0 {
			decoration = app.api.getToplevelDecoration(app.globals.decorationMgr, xdgToplevel)
			app.api.setDecorationMode(decoration, zxdgToplevelDecorationModeClientSide)
		}
	}

	win := &waylandWindow{
		app:            app,
		surface:        surface,
		xdgSurface:     xdgSurface,
		xdgToplevel:    xdgToplevel,
		decoration:     decoration,
		decorationMode: decorationMode,
		width:          totalW,
		height:         totalH,
		margin:         margin,
		savedLogicalW:  float32(config.Width),
		savedLogicalH:  float32(config.Height),
		frameWanted:    frameWanted,
		geometryDirty:  true,
	}
	win.storeScale(scale)
	win.pinner.Pin(win)
	win.pinned = true

	initWindowListeners()
	app.api.proxyAddListener(xdgSurface, uintptr(unsafe.Pointer(&rawXdgSurfaceListener[0])), unsafe.Pointer(win))
	app.api.proxyAddListener(xdgToplevel, uintptr(unsafe.Pointer(&rawXdgToplevelListener[0])), unsafe.Pointer(win))
	if decoration != 0 {
		app.api.proxyAddListener(decoration, uintptr(unsafe.Pointer(&rawDecorationListener[0])), unsafe.Pointer(win))
	}
	app.setupWindowScale(win)

	if config.WindowMode == platform.WindowCustomTitlebar && config.Actions != nil {
		*config.Actions = platform.WindowActions{
			Minimize:       win.minimize,
			ToggleMaximize: win.toggleMaximize,
			RequestClose:   win.requestClose,
			Maximized:      func() bool { return win.maximized },
		}
	}

	win.setMinimumSize(config.MinWidth, config.MinHeight)

	textInput, err := newLinuxTextInput(&config)
	if err != nil {
		win.destroy()
		return nil, err
	}
	initialRectangles, initialLabels := textInput.initial()

	surfaceProvider := newWaylandSurfaceProvider(app.display, surface)
	renderer, err := newWaylandWindowRenderer(surfaceProvider, win.width, win.height, win.displayScale(), config.ClearColor, initialRectangles, initialLabels, win.margin)
	if err != nil {
		textInput.close()
		win.destroy()
		return nil, err
	}

	if win.margin > 0 && !renderer.frame.alpha {
		win.margin = 0
		win.frameWanted = false
		if err := renderer.setFrameMargin(0); err != nil {
			renderer.close()
			textInput.close()
			win.destroy()
			return nil, err
		}
		win.publishFrameGeometry()
	}

	state := &waylandWindowState{
		app:      app,
		win:      win,
		config:   config,
		renderer: renderer,
		text:     textInput,
		profile:  os.Getenv("VIVA_PROFILE_STARTUP") != "",
	}
	if state.profile {
		state.started = time.Now()
	}
	win.state = state

	var ime *linuxIME
	applyFrame := func(frame platform.Frame) (bool, error) {
		if ime != nil {
			defer ime.sync()
		}
		if frame.Accessibility != nil && app != nil {
			app.atspiFrame(state, frame.Accessibility)
		}
		merged, dirty, err := textInput.prepare(frame)
		if err != nil || !dirty {
			return false, err
		}
		changed, err := renderer.apply(merged)
		return changed, err
	}
	state.applyFrame = applyFrame
	state.router = eventRouter{
		config:       config,
		text:         textInput,
		scale:        win.displayScale,
		modifierKeys: newModifierTracker(),
		clicks:       &input.ClickCounter{Interval: 500 * time.Millisecond, Distance: 4},
		present:      win.presentInputFrame,
	}
	if (config.DynamicTextInput || config.TextInput != nil) && app != nil {
		if ime = app.newIME(win, textInput, state.router); ime != nil {
			state.router.ime = ime
		}
	}

	if config.Waker != nil && config.OnRedraw != nil {
		config.Waker.Bind(func() {
			app.post(func() {
				win.needsRedraw = true
				if !win.framePending {
					win.renderFrame()
				}
			})
		})
	}

	app.mu.Lock()
	app.windows = append(app.windows, state)
	app.mu.Unlock()
	app.atspiOpened(state)
	win.beginRemap()

	return win, nil
}
