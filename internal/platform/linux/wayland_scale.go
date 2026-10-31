//go:build linux

package linux

import (
	"math"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

var (
	scaleListenerOnce          sync.Once
	rawFractionalScaleListener [1]uintptr
	pinnerScale                runtime.Pinner

	surfaceListenerOnce sync.Once
	rawSurfaceListener  [4]uintptr
	pinnerSurface       runtime.Pinner

	outputListenerOnce sync.Once
	rawOutputListener  [4]uintptr
	pinnerOutput       runtime.Pinner
)

type waylandOutput struct {
	globals *waylandGlobals
	proxy   uintptr
	name    uint32
	scale   int32
	pinner  runtime.Pinner
	pinned  bool
}

func (output *waylandOutput) setScale(scale int32) {
	if scale <= 0 {
		scale = 1
	}
	if output.scale == scale {
		return
	}
	output.scale = scale
	if output.globals != nil && output.globals.onOutputScale != nil {
		output.globals.onOutputScale(output.proxy, scale)
	}
}

func (output *waylandOutput) destroy(api *waylandAPI) {
	if output == nil {
		return
	}
	if output.proxy != 0 && api != nil {
		api.proxyDestroy(output.proxy)
		output.proxy = 0
	}
	if output.pinned {
		output.pinner.Unpin()
		output.pinned = false
	}
}

func initScaleListeners() {
	scaleListenerOnce.Do(func() {
		// 0: preferred_scale(scaleObj, numerator)
		rawFractionalScaleListener[0] = purego.NewCallback(func(data unsafe.Pointer, scaleObj uintptr, numerator uint32) {
			if data == nil {
				return
			}
			win := (*waylandWindow)(data)
			scale := float32(numerator) / 120.0
			win.applyScale(scale)
		})

		pinnerScale.Pin(&rawFractionalScaleListener)
	})
}

func initOutputListeners() {
	outputListenerOnce.Do(func() {
		// wl_output geometry, mode and done do not affect the integer scale cache.
		rawOutputListener[0] = purego.NewCallback(func(data unsafe.Pointer, output uintptr, x, y, physicalWidth, physicalHeight, subpixel int32, make, model *byte, transform int32) {
		})
		rawOutputListener[1] = purego.NewCallback(func(data unsafe.Pointer, output uintptr, flags uint32, width, height, refresh int32) {
		})
		rawOutputListener[2] = purego.NewCallback(func(data unsafe.Pointer, output uintptr) {
		})
		rawOutputListener[3] = purego.NewCallback(func(data unsafe.Pointer, proxy uintptr, factor int32) {
			if data == nil {
				return
			}
			(*waylandOutput)(data).setScale(factor)
		})
		pinnerOutput.Pin(&rawOutputListener)
	})
}

func initSurfaceListeners() {
	surfaceListenerOnce.Do(func() {
		// 0: enter(surface, output)
		rawSurfaceListener[0] = purego.NewCallback(func(data unsafe.Pointer, surface uintptr, output uintptr) {
			if data == nil {
				return
			}
			win := (*waylandWindow)(data)
			win.handleSurfaceEnter(output)
		})

		// 1: leave(surface, output)
		rawSurfaceListener[1] = purego.NewCallback(func(data unsafe.Pointer, surface uintptr, output uintptr) {
			if data == nil {
				return
			}
			win := (*waylandWindow)(data)
			win.handleSurfaceLeave(output)
		})

		// 2: preferred_buffer_scale(surface, factor)
		rawSurfaceListener[2] = purego.NewCallback(func(data unsafe.Pointer, surface uintptr, factor int32) {
			if data == nil {
				return
			}
			win := (*waylandWindow)(data)
			if win.fractionalScale == 0 && factor > 0 {
				win.preferredBufferScale = true
				win.applyScale(float32(factor))
			}
		})

		// 3: preferred_buffer_transform(surface, transform)
		rawSurfaceListener[3] = purego.NewCallback(func(data unsafe.Pointer, surface uintptr, transform uint32) {
			// No-op for now
		})

		pinnerSurface.Pin(&rawSurfaceListener)
	})
}

func (app *waylandApplication) setupWindowScale(win *waylandWindow) {
	if win == nil || win.surface == 0 || app == nil || app.api == nil {
		return
	}

	// Fractional scale requires a viewport to map the scaled buffer back to
	// logical surface coordinates. Without either capability, use the integer
	// preferred-buffer-scale/wl_output fallback instead.
	if app.globals.scaleMgr != 0 && app.globals.viewporter != 0 {
		win.fractionalScale = app.api.getFractionalScale(app.globals.scaleMgr, win.surface)
		if win.fractionalScale != 0 {
			win.viewport = app.api.getViewport(app.globals.viewporter, win.surface)
			if win.viewport != 0 {
				initScaleListeners()
				app.api.proxyAddListener(win.fractionalScale, uintptr(unsafe.Pointer(&rawFractionalScaleListener[0])), unsafe.Pointer(win))
				win.updateViewportDestination()
			} else {
				app.api.fractionalScaleDestroy(win.fractionalScale)
				win.fractionalScale = 0
			}
		}
	}

	// Surface enter/leave for output tracking
	initSurfaceListeners()
	app.api.proxyAddListener(win.surface, uintptr(unsafe.Pointer(&rawSurfaceListener[0])), unsafe.Pointer(win))
}

func (win *waylandWindow) handleSurfaceEnter(output uintptr) {
	if output == 0 {
		return
	}
	win.mu.Lock()
	found := false
	for _, o := range win.activeOutputs {
		if o == output {
			found = true
			break
		}
	}
	if !found {
		win.activeOutputs = append(win.activeOutputs, output)
	}
	win.mu.Unlock()

	// Fractional and preferred-buffer scale take precedence over wl_output.scale.
	win.refreshOutputScale()
}

func (win *waylandWindow) handleSurfaceLeave(output uintptr) {
	if output == 0 {
		return
	}
	win.mu.Lock()
	for i, o := range win.activeOutputs {
		if o == output {
			win.activeOutputs = append(win.activeOutputs[:i], win.activeOutputs[i+1:]...)
			break
		}
	}
	win.mu.Unlock()
	win.refreshOutputScale()
}

func (win *waylandWindow) refreshOutputScale() {
	if win.fractionalScale != 0 || win.preferredBufferScale || win.app == nil {
		return
	}
	win.mu.Lock()
	outputs := append([]uintptr(nil), win.activeOutputs...)
	win.mu.Unlock()
	if len(outputs) == 0 {
		return
	}
	scale := int32(1)
	for _, output := range outputs {
		scale = max(scale, win.app.getOutputScale(output))
	}
	win.applyScale(float32(scale))
}

func (win *waylandWindow) applyScale(scale float32) {
	if scale <= 0 {
		scale = 1.0
	}
	oldScale := win.displayScale()
	if oldScale == scale {
		return
	}
	fail := func(err error) bool {
		if err == nil {
			return false
		}
		if win.app != nil {
			win.app.fail(err)
		}
		return true
	}

	logicalW := float32(contentSize(win.width, win.margin)) / oldScale
	logicalH := float32(contentSize(win.height, win.margin)) / oldScale
	if win.configure.toplevelPending || win.configure.serial != 0 || win.resizeDeferred {
		// An uncommitted configure owns the latest logical geometry. The current
		// device size still describes the previous commit and must not replace it.
		w, h := win.pendingLogicalSize()
		if w > 0 && h > 0 {
			logicalW = float32(w)
			logicalH = float32(h)
		}
	}

	logicalW = float32(math.Round(float64(logicalW)))
	logicalH = float32(math.Round(float64(logicalH)))

	if !win.maximized && !win.fullscreen {
		win.savedLogicalW = logicalW
		win.savedLogicalH = logicalH
	}

	// Keep the previous Frame snapshot until all geometry fields are ready.
	win.scaleBits.Store(math.Float32bits(scale))

	// Recompute physical dimensions
	physContentW := deviceLength(int(logicalW), scale)
	physContentH := deviceLength(int(logicalH), scale)

	if win.margin > 0 {
		win.margin = frameMargin(scale)
		win.width = physContentW + 2*win.margin
		win.height = physContentH + 2*win.margin
		if win.state != nil && win.state.renderer != nil {
			if fail(win.state.renderer.setFrameMargin(win.margin)) {
				return
			}
		}
	} else {
		win.width = physContentW
		win.height = physContentH
	}
	win.publishFrameGeometry()

	// Update Vulkan renderer scale & resize swapchain
	if win.state != nil && win.state.renderer != nil {
		if fail(win.state.renderer.setScale(scale)) {
			return
		}
		if fail(win.state.renderer.resize(win.width, win.height)) {
			return
		}
	}

	if win.viewport == 0 && win.app != nil && win.app.api != nil && win.surface != 0 {
		win.app.api.surfaceSetBufferScale(win.surface, int32(math.Ceil(float64(scale))))
	}

	// Scale changes use the same mandatory-present transaction as configure.
	win.geometryDirty = true
	win.configure.width = int32(logicalW)
	win.configure.height = int32(logicalH)
	if win.state != nil {
		win.forcePresent = true
	}
	win.applyPendingResize()
}

func (app *waylandApplication) handleOutputScale(output uintptr, scale int32) {
	app.mu.Lock()
	windows := append([]*waylandWindowState(nil), app.windows...)
	app.mu.Unlock()
	for _, state := range windows {
		if state != nil && state.win != nil {
			state.win.refreshOutputScale()
		}
	}
}

func (app *waylandApplication) handleOutputRemove(output uintptr) {
	app.mu.Lock()
	windows := append([]*waylandWindowState(nil), app.windows...)
	app.mu.Unlock()
	for _, state := range windows {
		if state != nil && state.win != nil {
			state.win.handleSurfaceLeave(output)
		}
	}
}

func (app *waylandApplication) getOutputScale(output uintptr) int32 {
	if bound := app.globals.outputs[output]; bound != nil && bound.scale > 0 {
		return bound.scale
	}
	return 1
}
