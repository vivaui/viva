//go:build js && wasm

// Package web is the browser deployment backend: it renders the shared
// canvas vocabulary onto a Canvas2D surface and bridges DOM input events.
// It is a leaf in the native dependency graph and never imports native
// platform packages.
package web

import (
	"fmt"
	"strings"
	"syscall/js"
	"time"
	"unicode/utf8"
	"unsafe"

	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

// Config describes the browser deployment.
type Config struct {
	Title  string
	Width  int
	Height int
	// ClearColor is the color the canvas is cleared to before Paint runs. The
	// zero color keeps the backend's dark default.
	ClearColor ui.Color
	Paint      func(canvas *ui.Canvas, size ui.Size)
	OnPointer  func(event ui.PointerEvent) ui.EventResult
	OnKey      func(event ui.KeyEvent) ui.EventResult
	OnShortcut func(event ui.KeyEvent) ui.EventResult
	OnScroll   func(event ui.ScrollEvent) ui.EventResult
	// OnFileDrop receives files dragged from the page's outside over the
	// canvas, and the drop. The browser reports no paths: the event's Files carry
	// the chosen File objects.
	OnFileDrop func(event ui.FileDropEvent) ui.EventResult
	// OnModifierTap receives a modifier key pressed and released on its own
	// (see ui.ModifierTapEvent).
	OnModifierTap func(event ui.ModifierTapEvent) ui.EventResult
	// OnResize runs after the viewport size changes and before the repaint it
	// causes. It is not called for the initial viewport size.
	OnResize func(size ui.Size)
	// OnMinimize reports the page becoming hidden or visible, the browser
	// counterpart of a window being minimized and restored.
	OnMinimize func(hidden bool)
	// OnFullscreen reports a transition into or out of the browser's
	// fullscreen mode.
	OnFullscreen func(fullscreen bool)
	// OnCloseRequest runs before the page unloads. allow must be called during
	// the callback to let the unload proceed; a browser cannot wait for an
	// asynchronous answer.
	OnCloseRequest func(allow func())
	// OnClosed runs once when the page is being torn down, before the browser
	// releases the WASM instance.
	OnClosed func()
	// ForwardWheel, when set, receives a wheel gesture the application did not
	// use, so an embedding page can scroll instead.
	ForwardWheel func(deltaX, deltaY float32)
	TextInput    TextInputProvider
	// Canvas is the retained canvas the runtime paints into; nil makes a fresh one.
	Canvas *ui.Canvas
	// Waker, when set, is bound for the life of the page: waking it runs OnWake
	// and then repaints, so background work can ask for a frame from any goroutine.
	Waker  *platform.Waker
	OnWake func()
}

type runtime struct {
	config     Config
	canvas     *ui.Canvas
	painter    *painter
	document   js.Value
	window     js.Value
	focusRoot  js.Value
	surface    js.Value
	context    js.Value
	a11y       *accessibilityMirror
	dpr        float32
	pending    bool
	nextPaint  bool
	frameTimer int
	timerFunc  js.Func
	metrics    *textMetrics
	measurer   *domMeasurer
	clock      frameClock
	paths      pathCache
	sized      bool
	hidden     bool
	missed     bool
	fullscreen bool
	closed     bool
}

// Run mounts the application into the document and returns when the page
// unloads. It must be called from the main goroutine of a WASM binary.
func Run(config Config) error {
	window := js.Global().Get("window")
	runtime := &runtime{
		config:   config,
		window:   window,
		document: window.Get("document"),
		dpr:      float32(window.Get("devicePixelRatio").Float()),
	}
	if runtime.dpr <= 0 {
		runtime.dpr = 1
	}
	runtime.clock = newFrameClock()
	runtime.mount()
	runtime.bindEvents()
	defer runtime.bindWaker()()
	runtime.requestFrame()
	// Yield once through the microtask queue so the initial rAF callback can
	// run before the blocking select parks the main goroutine.
	awaitReady(make(chan struct{}))
	select {}
}

// awaitReady pumps the JS event loop once so pending callbacks (the initial
// rAF) execute before the caller blocks.
func awaitReady(done chan struct{}) {
	js.Global().Get("window").Call("setTimeout", js.FuncOf(func(js.Value, []js.Value) any {
		close(done)
		return nil
	}), 0)
	<-done
}

// mount creates the canvas element and sizes it to the viewport.
func (runtime *runtime) mount() {
	if runtime.config.Title != "" {
		runtime.document.Set("title", runtime.config.Title)
	}
	body := runtime.document.Get("body")
	bodyStyle := body.Get("style")
	bodyStyle.Set("margin", "0")
	bodyStyle.Set("overflow", "hidden")
	bodyStyle.Set("overscrollBehavior", "none")
	focusRoot := runtime.document.Call("createElement", "div")
	rootStyle := focusRoot.Get("style")
	rootStyle.Set("position", "fixed")
	rootStyle.Set("inset", "0")
	rootStyle.Set("overflow", "hidden")
	rootStyle.Set("outline", "none")
	rootStyle.Set("touchAction", "none")

	surface := runtime.document.Call("createElement", "canvas")
	style := surface.Get("style")
	style.Set("position", "absolute")
	style.Set("inset", "0")
	style.Set("width", "100%")
	style.Set("height", "100%")
	style.Set("cursor", "default")
	style.Set("userSelect", "none")
	style.Set("touchAction", "none")
	style.Set("webkitTapHighlightColor", "transparent")

	accessibilityLayer := runtime.document.Call("createElement", "div")
	accessibilityStyle := accessibilityLayer.Get("style")
	accessibilityStyle.Set("position", "absolute")
	accessibilityStyle.Set("inset", "0")
	accessibilityStyle.Set("pointerEvents", "none")

	focusRoot.Call("appendChild", surface)
	focusRoot.Call("appendChild", accessibilityLayer)
	body.Call("appendChild", focusRoot)
	runtime.focusRoot = focusRoot
	runtime.surface = surface
	runtime.context = surface.Call("getContext", "2d")
	runtime.a11y = newAccessibilityMirror(runtime.document, focusRoot, accessibilityLayer, runtime.config.Title)

	runtime.canvas = runtime.config.Canvas
	if runtime.canvas == nil {
		runtime.canvas = &ui.Canvas{}
	}
	runtime.measurer = newDOMMeasurer(runtime.document)
	runtime.canvas.SetTextEngine(newLineEngine(runtime.measurer))
	started := time.Now()
	runtime.canvas.SetClock(func() time.Duration { return time.Since(started) })
	runtime.canvas.SetReduceMotion(runtime.window.Call("matchMedia", "(prefers-reduced-motion: reduce)").Get("matches").Bool())
	runtime.hidden = runtime.document.Get("hidden").Bool()
	runtime.fullscreen = fullscreenActive(runtime.document)
	// A font that loads late changes the baselines and widths as it does every line.
	clearLines := runtime.measurer.invalidate
	runtime.measurer.setInvalidate(func() {
		clearLines()
		runtime.metrics.clear()
	})
	runtime.metrics = newTextMetrics(runtime.measurer)
	runtime.painter = newPainter(runtime.canvas, &runtime.config)
	runtime.a11y.setActions(
		func(id ui.ID) {
			// The painter repaints its own canvas; the runtime still has to
			// publish that frame and the ARIA mirror to the document.
			if runtime.painter.accessibilityPress(id) {
				runtime.requestFrame()
			}
		},
		func(id ui.ID) {
			if runtime.painter.accessibilityFocus(id) {
				runtime.requestFrame()
			}
		},
		func(id ui.ID, value float64) {
			if runtime.painter.accessibilitySetNumber(id, value) {
				runtime.requestFrame()
			}
		},
	)
	runtime.resize()
}

// resizeSync reads the viewport size and reflows backing store and layout.
func (runtime *runtime) resize() {
	previous := runtime.dpr
	runtime.dpr = float32(runtime.window.Get("devicePixelRatio").Float())
	if runtime.dpr <= 0 {
		runtime.dpr = 1
	}
	if runtime.dpr != previous && runtime.measurer != nil {
		// Browser zoom changes the pixel size text is shaped at, and so its advances.
		runtime.measurer.forget()
	}
	width := runtime.window.Get("innerWidth").Int()
	if width <= 0 && runtime.config.Width > 0 {
		width = runtime.config.Width
	}
	height := runtime.window.Get("innerHeight").Int()
	if height <= 0 && runtime.config.Height > 0 {
		height = runtime.config.Height
	}
	changed := runtime.painter != nil && (runtime.painter.width != float32(width) || runtime.painter.height != float32(height))
	runtime.surface.Set("width", int(float32(width)*runtime.dpr))
	runtime.surface.Set("height", int(float32(height)*runtime.dpr))
	if changed && runtime.sized && runtime.config.OnResize != nil {
		runtime.config.OnResize(ui.Size{Width: float32(width), Height: float32(height)})
	}
	if runtime.hidden {
		runtime.painter.width, runtime.painter.height = float32(width), float32(height)
		runtime.missed = true
		runtime.nextPaint = true
	} else {
		runtime.painter.resize(width, height)
	}
	runtime.sized = true
}

// requestFrame schedules one render on the browser's display pipeline. The
// runtime is event-driven: frames happen only when something changed.
func (runtime *runtime) requestFrame() {
	runtime.cancelFrameTimer()
	if runtime.hidden {
		// A hidden page never gets a display frame. Remember that one was
		// wanted so becoming visible paints it, without waking the browser's
		// rendering pipeline while nobody can see the result.
		runtime.missed = true
		return
	}
	if runtime.pending {
		return
	}
	runtime.pending = true
	js.Global().Get("window").Call("requestAnimationFrame", js.FuncOf(func(js.Value, []js.Value) any {
		runtime.pending = false
		if runtime.nextPaint {
			runtime.nextPaint = false
			runtime.painter.repaint()
		}
		runtime.render()
		if runtime.painter.takeAgain() {
			runtime.scheduleFrame(0)
		}
		return nil
	}))
}

func (runtime *runtime) cancelFrameTimer() {
	if runtime.frameTimer != 0 {
		runtime.window.Call("clearTimeout", runtime.frameTimer)
		runtime.frameTimer = 0
		runtime.timerFunc.Release()
	}
}

func (runtime *runtime) scheduleFrame(delay time.Duration) {
	if delay <= 0 {
		runtime.nextPaint = true
		runtime.requestFrame()
		return
	}
	runtime.cancelFrameTimer()
	runtime.timerFunc = js.FuncOf(func(js.Value, []js.Value) any {
		runtime.frameTimer = 0
		runtime.timerFunc.Release()
		runtime.painter.repaint()
		runtime.requestFrame()
		return nil
	})
	milliseconds := int((delay + time.Millisecond - 1) / time.Millisecond)
	runtime.frameTimer = runtime.window.Call("setTimeout", runtime.timerFunc, milliseconds).Int()
}

// drawRuns draws a fixed-pitch grid line: each run at its cell, in its own
// color and weight, wide characters centered in their cells.
func (runtime *runtime) drawRuns(text ui.Text) {
	context := runtime.context
	context.Call("save")
	if text.HasClip {
		clipRect(context, text.ClipOf)
	}
	clipRect(context, text.Bounds)
	metrics := monoBaseline(context, text.Style.Size, text.Style.MonoFace)
	baseline := text.Bounds.Y + (text.Bounds.Height-metrics.Ascent-metrics.Descent)*0.5 + metrics.Ascent
	context.Set("textBaseline", "alphabetic")
	offset := 0
	for _, run := range runtime.canvas.TextRuns(text) {
		end := offset + int(run.Bytes)
		if run.Bytes < 0 || end > len(text.Value) {
			break
		}
		piece := text.Value[offset:end]
		offset = end
		left := text.Bounds.X + float32(run.Column)*text.CellWidth
		span := float32(run.Cells) * text.CellWidth
		context.Set("fillStyle", cssColor(run.Color))
		context.Set("font", cssMonoFont(text.Style.Size, run.Weight, run.Italic, text.Style.MonoFace))
		x := left
		if run.Cells > 0 && utf8.RuneCountInString(piece) == 1 {
			x = left + (span-float32(context.Call("measureText", piece).Get("width").Float()))/2
		}
		if printableASCII(piece) && len(piece) > 1 {
			// Plain ASCII: each character stands on its cell, whatever the font's own
			// advance, and keeps its own shape.
			for index := range len(piece) {
				context.Call("fillText", piece[index:index+1], left+float32(index)*text.CellWidth, baseline)
			}
		} else {
			context.Call("fillText", piece, x, baseline)
		}
		if span > 0 {
			thickness := max(1, text.Style.Size/14)
			if run.Underline {
				context.Call("fillRect", left, baseline+thickness, span, thickness)
			}
			if run.Strike {
				context.Call("fillRect", left, baseline-metrics.Ascent*0.35, span, thickness)
			}
		}
	}
	context.Call("restore")
}

func (runtime *runtime) textInputChanged() {
	runtime.painter.repaint()
	runtime.requestFrame()
}

func clipRect(context js.Value, bounds ui.Rect) {
	context.Call("beginPath")
	context.Call("rect", bounds.X, bounds.Y, bounds.Width, bounds.Height)
	context.Call("clip")
}

// fullscreenActive reports whether the document is in fullscreen mode,
// including the WebKit-prefixed property older Safari exposes.
func fullscreenActive(document js.Value) bool {
	return document.Get("fullscreenElement").Truthy() || document.Get("webkitFullscreenElement").Truthy()
}

// bindEvents wires pointer and keyboard input to the interaction router.
func (runtime *runtime) bindEvents() {
	surface := runtime.surface
	runtime.document.Call("addEventListener", "visibilitychange", js.FuncOf(func(js.Value, []js.Value) any {
		hidden := runtime.document.Get("hidden").Bool()
		if hidden == runtime.hidden {
			return nil
		}
		runtime.hidden = hidden
		if runtime.config.OnMinimize != nil {
			runtime.config.OnMinimize(hidden)
		}
		if hidden {
			// Releases go to whatever has the keyboard now; the page will not
			// receive their keyup events.
			runtime.painter.tap.Cancel()
			if runtime.painter.releaseKeys() {
				runtime.missed = true
				runtime.nextPaint = true
			}
			return nil
		}
		if runtime.missed {
			runtime.missed = false
			runtime.requestFrame()
		}
		return nil
	}))
	if runtime.config.OnFullscreen != nil {
		changed := func(js.Value, []js.Value) any {
			fullscreen := fullscreenActive(runtime.document)
			if fullscreen == runtime.fullscreen {
				return nil
			}
			runtime.fullscreen = fullscreen
			runtime.config.OnFullscreen(fullscreen)
			return nil
		}
		runtime.document.Call("addEventListener", "fullscreenchange", js.FuncOf(changed))
		runtime.document.Call("addEventListener", "webkitfullscreenchange", js.FuncOf(changed))
	}
	if runtime.config.OnCloseRequest != nil {
		runtime.window.Call("addEventListener", "beforeunload", js.FuncOf(func(_ js.Value, args []js.Value) any {
			allowed := false
			runtime.config.OnCloseRequest(func() { allowed = true })
			if !allowed && len(args) > 0 {
				args[0].Call("preventDefault")
				args[0].Set("returnValue", "")
			}
			return nil
		}))
	}
	if runtime.config.OnClosed != nil {
		closed := func(js.Value, []js.Value) any {
			if runtime.closed {
				return nil
			}
			runtime.closed = true
			runtime.config.OnClosed()
			return nil
		}
		runtime.window.Call("addEventListener", "pagehide", js.FuncOf(func(_ js.Value, args []js.Value) any {
			if len(args) > 0 && args[0].Get("persisted").Bool() {
				return nil
			}
			return closed(js.Value{}, args)
		}))
		runtime.window.Call("addEventListener", "unload", js.FuncOf(closed))
	}
	if runtime.config.OnFileDrop != nil {
		runtime.bindFileDrop()
	}
	motion := runtime.window.Call("matchMedia", "(prefers-reduced-motion: reduce)")
	motion.Call("addEventListener", "change", js.FuncOf(func(_ js.Value, args []js.Value) any {
		runtime.canvas.SetReduceMotion(args[0].Get("matches").Bool())
		runtime.painter.repaint()
		runtime.requestFrame()
		return nil
	}))

	surface.Call("addEventListener", "pointermove", js.FuncOf(func(_ js.Value, args []js.Value) any {
		event := args[0]
		if runtime.painter.pointerEvent(pointerEvent(ui.PointerMove, event)) {
			runtime.requestFrame()
		}
		return nil
	}))
	surface.Call("addEventListener", "pointerdown", js.FuncOf(func(_ js.Value, args []js.Value) any {
		event := args[0]
		pointerID := event.Get("pointerId").Int()
		surface.Call("setPointerCapture", pointerID)
		if runtime.painter.pointerEvent(pointerEvent(ui.PointerDown, event)) {
			runtime.requestFrame()
		}
		runtime.focusRoot.Call("focus", map[string]any{"preventScroll": true})
		return nil
	}))
	surface.Call("addEventListener", "pointerup", js.FuncOf(func(_ js.Value, args []js.Value) any {
		event := args[0]
		if runtime.painter.pointerEvent(pointerEvent(ui.PointerUp, event)) {
			runtime.requestFrame()
		}
		pointerID := event.Get("pointerId").Int()
		if surface.Call("hasPointerCapture", pointerID).Bool() {
			surface.Call("releasePointerCapture", pointerID)
		}
		return nil
	}))
	surface.Call("addEventListener", "pointerleave", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if runtime.painter.pointerEvent(pointerEvent(ui.PointerExit, args[0])) {
			runtime.requestFrame()
		}
		return nil
	}))
	surface.Call("addEventListener", "pointercancel", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if runtime.painter.pointerCancel(pointerEvent(ui.PointerExit, args[0])) {
			runtime.requestFrame()
		}
		pointerID := args[0].Get("pointerId").Int()
		if surface.Call("hasPointerCapture", pointerID).Bool() {
			surface.Call("releasePointerCapture", pointerID)
		}
		return nil
	}))
	surface.Call("addEventListener", "contextmenu", js.FuncOf(func(_ js.Value, args []js.Value) any {
		args[0].Call("preventDefault")
		return nil
	}))
	runtime.window.Call("addEventListener", "resize", js.FuncOf(func(js.Value, []js.Value) any {
		runtime.resize()
		runtime.requestFrame()
		return nil
	}))
	surface.Call("addEventListener", "wheel", js.FuncOf(func(_ js.Value, args []js.Value) any {
		event := args[0]
		event.Call("preventDefault")
		deltaX, deltaY, precise := wheelDeltas(
			float32(event.Get("deltaX").Float()),
			float32(event.Get("deltaY").Float()),
			event.Get("deltaMode").Int(),
			runtime.painter.width,
			runtime.painter.height,
		)
		handled, changed := runtime.painter.wheel(ui.ScrollEvent{
			Modifiers: webModifiers(event.Get("shiftKey").Bool(), event.Get("ctrlKey").Bool(), event.Get("altKey").Bool(), event.Get("metaKey").Bool()),
			X:         float32(event.Get("clientX").Float()),
			Y:         float32(event.Get("clientY").Float()),
			DeltaX:    deltaX,
			DeltaY:    deltaY,
			Precise:   precise,
		})
		if changed {
			runtime.requestFrame()
		}
		if !handled && runtime.config.ForwardWheel != nil {
			runtime.config.ForwardWheel(deltaX, deltaY)
		}
		return nil
	}), map[string]any{"passive": false})
	runtime.focusRoot.Call("addEventListener", "keyup", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		event := args[0]
		code := webKeyCode(event.Get("code").String())
		changed := false
		composing := event.Get("isComposing").Bool() || event.Get("keyCode").Int() == 229
		if !composing && !runtime.painter.textInputID.IsZero() && runtime.painter.textInput.Terminal && runtime.painter.keyWanted() {
			if key, _, ok := webTerminalKey(event); ok {
				changed = runtime.painter.keyUp(key, code)
			}
		}
		if _, ok := webModifierKey(event.Get("code").String()); ok && runtime.painter.modifierUp(code) {
			changed = true
		}
		if changed {
			runtime.requestFrame()
		}
		return nil
	}))
	runtime.focusRoot.Call("addEventListener", "blur", js.FuncOf(func(_ js.Value, _ []js.Value) any {
		runtime.painter.tap.Cancel()
		return nil
	}), true)
	runtime.window.Call("addEventListener", "blur", js.FuncOf(func(js.Value, []js.Value) any {
		// The page stopped being the one the keyboard types into. Releases for
		// held keys will go to whatever has the keyboard now.
		if runtime.painter.focusLost() {
			runtime.requestFrame()
		}
		return nil
	}))
	runtime.focusRoot.Call("addEventListener", "keydown", js.FuncOf(func(_ js.Value, args []js.Value) any {
		event := args[0]
		modifiers := webModifiers(event.Get("shiftKey").Bool(), event.Get("ctrlKey").Bool(), event.Get("altKey").Bool(), event.Get("metaKey").Bool())
		composing := event.Get("isComposing").Bool() || event.Get("keyCode").Int() == 229
		if key, ok := webModifierKey(event.Get("code").String()); ok {
			runtime.painter.modifierDown(key, webKeyCode(event.Get("code").String()), modifiers, event.Get("repeat").Bool())
			if !composing && runtime.painter.keyWanted() {
				if keyEvent, code, ok := webTerminalKey(event); ok {
					event.Call("preventDefault")
					if runtime.painter.keyDownCode(keyEvent, code) {
						runtime.requestFrame()
					}
				}
			}
			return nil
		}
		runtime.painter.tap.Cancel()
		shortcut := strings.ToLower(event.Get("key").String())
		enter := event.Get("key").String() == "Enter"
		if runtime.config.OnShortcut != nil && !event.Get("isComposing").Bool() &&
			(!enter || modifiers&(ui.ModifierControl|ui.ModifierMeta) != 0) && (enter || utf8.RuneCountInString(shortcut) == 1) {
			chord := ui.KeyEvent{Modifiers: modifiers, Repeat: event.Get("repeat").Bool(), Text: shortcut}
			if enter {
				chord.Key, chord.Text = input.KeyEnter, ""
			}
			handled, changed := runtime.painter.shortcut(chord)
			if handled {
				event.Call("preventDefault")
			}
			if changed {
				runtime.requestFrame()
			}
			if handled {
				return nil
			}
		}
		if composing {
			return nil
		}
		terminal := !runtime.painter.textInputID.IsZero() && runtime.painter.textInput.Terminal
		if terminal {
			keyEvent, code, ok := webTerminalKey(event)
			if !ok {
				return nil
			}
			altRaw := modifiers&ui.ModifierAlt != 0 && runtime.painter.textInput.AltSendsEscape
			if keyEvent.Key == ui.KeySpace && modifiers&(ui.ModifierControl|ui.ModifierMeta) == 0 && !altRaw {
				// A terminal receives a plain space as committed text, not as a
				// raw key: that is what lets it reach a remote pty as " ".
				return nil
			}
			if keyEvent.Key == ui.KeyUnknown && modifiers&ui.ModifierAlt != 0 &&
				modifiers&(ui.ModifierControl|ui.ModifierMeta) == 0 && !altRaw {
				// Without Alt-as-Meta the text system owns the chord, as it does
				// natively even for a terminal that asked for releases.
				return nil
			}
			if runtime.painter.keyWanted() {
				// A terminal that asked for releases gets text, modifiers and
				// named keys as raw presses; the browser must not also insert
				// them into its hidden editor.
				event.Call("preventDefault")
				if runtime.painter.keyDownCode(keyEvent, code) {
					runtime.requestFrame()
				}
				return nil
			}
			if keyEvent.Text != "" && modifiers&(ui.ModifierControl|ui.ModifierAlt|ui.ModifierMeta) == 0 {
				// Plain typed text stays with the browser text service: it is
				// what delivers committed text and input-method composition.
				return nil
			}
			event.Call("preventDefault")
			if runtime.painter.keyDownCode(keyEvent, code) {
				runtime.requestFrame()
			}
			return nil
		}
		if runtime.a11y.text.activeElement(event.Get("target")) {
			if key, ok := webKey(event.Get("key").String()); ok && !event.Get("isComposing").Bool() &&
				(key == input.KeyTab || placedRowKey(runtime.painter.textInput, key, modifiers)) {
				event.Call("preventDefault")
				if runtime.painter.keyDown(ui.KeyEvent{
					Key:       key,
					Modifiers: modifiers,
					Repeat:    event.Get("repeat").Bool(),
				}) {
					runtime.requestFrame()
				}
			}
			return nil
		}
		if key, ok := webKey(event.Get("key").String()); ok {
			event.Call("preventDefault")
			if runtime.painter.keyDown(ui.KeyEvent{
				Key:       key,
				Modifiers: modifiers,
				Repeat:    event.Get("repeat").Bool(),
			}) {
				runtime.requestFrame()
			}
		}
		return nil
	}))
}

// bindFileDrop forwards drags of files from outside the page to the canvas
// hit under the pointer. A drag between descendants of the page is not a leave.
func (runtime *runtime) bindFileDrop() {
	root := runtime.focusRoot
	depth := 0
	position := func(event js.Value) (float32, float32) {
		return float32(event.Get("clientX").Float()), float32(event.Get("clientY").Float())
	}
	effect := func(event js.Value, handled bool) {
		if transfer := event.Get("dataTransfer"); transfer.Truthy() {
			value := "none"
			if handled {
				value = "copy"
			}
			transfer.Set("dropEffect", value)
		}
	}
	root.Call("addEventListener", "dragenter", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		event := args[0]
		event.Call("preventDefault")
		depth++
		if depth != 1 {
			return nil
		}
		x, y := position(event)
		handled, changed := runtime.painter.fileDrop(ui.FileDropOver, x, y, nil)
		effect(event, handled)
		if changed {
			runtime.requestFrame()
		}
		return nil
	}))
	root.Call("addEventListener", "dragover", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		event := args[0]
		event.Call("preventDefault")
		x, y := position(event)
		handled, changed := runtime.painter.fileDrop(ui.FileDropOver, x, y, nil)
		effect(event, handled)
		if changed {
			runtime.requestFrame()
		}
		return nil
	}))
	root.Call("addEventListener", "dragleave", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		event := args[0]
		event.Call("preventDefault")
		if related := event.Get("relatedTarget"); related.Truthy() && root.Call("contains", related).Bool() {
			return nil
		}
		depth = 0
		_, changed := runtime.painter.fileDrop(ui.FileDropLeave, 0, 0, nil)
		if changed {
			runtime.requestFrame()
		}
		return nil
	}))
	root.Call("addEventListener", "drop", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		event := args[0]
		event.Call("preventDefault")
		depth = 0
		x, y := position(event)
		var files []platform.FileRef
		if transfer := event.Get("dataTransfer"); transfer.Truthy() {
			list := transfer.Get("files")
			for index := 0; index < list.Length(); index++ {
				files = append(files, FileRefFromDOM(list.Index(index)))
			}
		}
		_, changed := runtime.painter.fileDrop(ui.FileDropDrop, x, y, files)
		if changed {
			runtime.requestFrame()
		}
		return nil
	}))
}

func placedRowKey(editor TextInput, key input.Key, modifiers ui.Modifiers) bool {
	if editor.NavigationKeys {
		switch key {
		case input.KeyUp, input.KeyDown, input.KeyPageUp, input.KeyPageDown:
			return true
		}
	}
	if !editor.Placed {
		return false
	}
	switch key {
	case input.KeyUp, input.KeyDown, input.KeyHome, input.KeyEnd, input.KeyPageUp, input.KeyPageDown:
		return true
	case input.KeyLeft, input.KeyRight:
		return modifiers&ui.ModifierMeta != 0
	}
	return false
}

func pointerEvent(kind ui.PointerEventKind, event js.Value) ui.PointerEvent {
	button := ui.PointerButtonNone
	if kind == ui.PointerDown || kind == ui.PointerUp {
		button = webPointerButton(event.Get("button").Int())
	}
	// UI Events defines detail on mousedown and mouseup as the click count.
	clicks := 0
	if kind == ui.PointerDown || kind == ui.PointerUp {
		clicks = max(event.Get("detail").Int(), 1)
	}
	return ui.PointerEvent{
		Kind:       kind,
		Button:     button,
		Modifiers:  webModifiers(event.Get("shiftKey").Bool(), event.Get("ctrlKey").Bool(), event.Get("altKey").Bool(), event.Get("metaKey").Bool()),
		X:          float32(event.Get("clientX").Float()),
		Y:          float32(event.Get("clientY").Float()),
		ClickCount: clicks,
	}
}

func webPointerButton(button int) ui.PointerButton {
	switch button {
	case 0:
		return ui.PointerButtonPrimary
	case 1:
		return ui.PointerButtonMiddle
	case 2:
		return ui.PointerButtonSecondary
	case 3:
		return ui.PointerButtonBack
	case 4:
		return ui.PointerButtonForward
	default:
		return ui.PointerButtonNone
	}
}

func webModifiers(shift, control, alt, meta bool) ui.Modifiers {
	var modifiers ui.Modifiers
	if shift {
		modifiers |= ui.ModifierShift
	}
	if control {
		modifiers |= ui.ModifierControl
	}
	if alt {
		modifiers |= ui.ModifierAlt
	}
	if meta {
		modifiers |= ui.ModifierMeta
	}
	return modifiers
}

func wheelDeltas(deltaX, deltaY float32, mode int, width, height float32) (float32, float32, bool) {
	switch mode {
	case 1:
		return deltaX * 16, deltaY * 16, false
	case 2:
		return deltaX * width, deltaY * height, false
	default:
		return deltaX, deltaY, true
	}
}

// webKey maps browser KeyboardEvent.key values to the shared input.Key set.
// webModifierKey names the modifier key a KeyboardEvent.code is for.
func webModifierKey(code string) (input.Key, bool) {
	switch code {
	case "ShiftLeft":
		return input.KeyShiftLeft, true
	case "ShiftRight":
		return input.KeyShiftRight, true
	case "ControlLeft":
		return input.KeyControlLeft, true
	case "ControlRight":
		return input.KeyControlRight, true
	case "AltLeft":
		return input.KeyAltLeft, true
	case "AltRight":
		return input.KeyAltRight, true
	case "MetaLeft":
		return input.KeyMetaLeft, true
	case "MetaRight":
		return input.KeyMetaRight, true
	}
	return input.KeyUnknown, false
}

// webKeyCode is the stable physical-key code that pairs a press with its
// release. KeyboardEvent.code names the physical key, not the character its
// layout produces, so hashing it keeps the pair together across modifier and
// layout changes without a fixed table of every keyboard.
func webKeyCode(code string) uint32 {
	if code == "" {
		return 0
	}
	hash := uint32(2166136261)
	for index := range len(code) {
		hash ^= uint32(code[index])
		hash *= 16777619
	}
	if hash == 0 {
		hash = 1
	}
	return hash
}

func webKey(key string) (input.Key, bool) {
	switch key {
	case "Tab":
		return input.KeyTab, true
	case "Enter":
		return input.KeyEnter, true
	case " ":
		return input.KeySpace, true
	case "Escape":
		return input.KeyEscape, true
	case "ArrowLeft":
		return input.KeyLeft, true
	case "ArrowRight":
		return input.KeyRight, true
	case "ArrowUp":
		return input.KeyUp, true
	case "ArrowDown":
		return input.KeyDown, true
	case "Home":
		return input.KeyHome, true
	case "End":
		return input.KeyEnd, true
	case "PageUp":
		return input.KeyPageUp, true
	case "PageDown":
		return input.KeyPageDown, true
	case "Backspace":
		return input.KeyBackspace, true
	case "Delete":
		return input.KeyDelete, true
	case "Insert":
		return input.KeyInsert, true
	case "F1":
		return input.KeyF1, true
	case "F2":
		return input.KeyF2, true
	case "F3":
		return input.KeyF3, true
	case "F4":
		return input.KeyF4, true
	case "F5":
		return input.KeyF5, true
	case "F6":
		return input.KeyF6, true
	case "F7":
		return input.KeyF7, true
	case "F8":
		return input.KeyF8, true
	case "F9":
		return input.KeyF9, true
	case "F10":
		return input.KeyF10, true
	case "F11":
		return input.KeyF11, true
	case "F12":
		return input.KeyF12, true
	default:
		return 0, false
	}
}

// webTerminalKey normalizes one browser keyboard event the way a native
// terminal receives it: named keys keep their identity, printable keys carry
// the character without modifiers in Text and the typed character in Typed.
func webTerminalKey(event js.Value) (ui.KeyEvent, uint32, bool) {
	keyName := event.Get("key").String()
	codeName := event.Get("code").String()
	modifiers := webModifiers(event.Get("shiftKey").Bool(), event.Get("ctrlKey").Bool(), event.Get("altKey").Bool(), event.Get("metaKey").Bool())
	repeat := event.Get("repeat").Bool()
	if key, ok := webModifierKey(codeName); ok {
		return ui.KeyEvent{Key: key, Modifiers: modifiers, Repeat: repeat}, webKeyCode(codeName), true
	}
	if key, ok := webKey(keyName); ok {
		return ui.KeyEvent{Key: key, Modifiers: modifiers, Repeat: repeat}, webKeyCode(codeName), true
	}
	text := printableKeyText(keyName)
	if text == "" {
		return ui.KeyEvent{}, 0, false
	}
	base := unmodifiedKeyText(keyName, codeName)
	typed := ""
	if modifiers&(ui.ModifierControl|ui.ModifierAlt|ui.ModifierMeta) == 0 {
		typed = keyName
	}
	return ui.KeyEvent{Key: ui.KeyUnknown, Modifiers: modifiers, Repeat: repeat, Text: base, Typed: typed}, webKeyCode(codeName), true
}

func printableKeyText(key string) string {
	if utf8.RuneCountInString(key) != 1 {
		return ""
	}
	r := []rune(key)[0]
	if r < 0x20 || r == 0x7f {
		return ""
	}
	return key
}

// unmodifiedKeyText is the character a printable key types with no modifiers.
// Letters and the digit row come from the physical code so Shift+A and Shift+1
// still identify A and 1; other punctuation uses the common US pairs, which is
// the best a browser exposes without a layout API.
func unmodifiedKeyText(key, code string) string {
	if key >= "A" && key <= "Z" {
		return strings.ToLower(key)
	}
	if len(code) == 4 && strings.HasPrefix(code, "Key") {
		return strings.ToLower(code[3:])
	}
	if len(code) == 6 && strings.HasPrefix(code, "Digit") {
		return code[5:]
	}
	switch key {
	case "!":
		return "1"
	case "@":
		return "2"
	case "#":
		return "3"
	case "$":
		return "4"
	case "%":
		return "5"
	case "^":
		return "6"
	case "&":
		return "7"
	case "*":
		return "8"
	case "(":
		return "9"
	case ")":
		return "0"
	case "_":
		return "-"
	case "+":
		return "="
	case "{":
		return "["
	case "}":
		return "]"
	case "|":
		return "\\"
	case ":":
		return ";"
	case "\"":
		return "'"
	case "<":
		return ","
	case ">":
		return "."
	case "?":
		return "/"
	case "~":
		return "`"
	}
	return key
}

// cssColor formats a linear RGBA color as a CSS rgb() string.
// triangleFill is what a triangle is filled with: its color, or, for a shaded
// one, the linear gradient Canvas 2D can paint in place of per-corner colors
// (see shadeAxis).
func triangleFill(context js.Value, triangle ui.Triangle) any {
	if triangle.Shaded {
		colors := [3]ui.Color{triangle.Color, triangle.Shade[0], triangle.Shade[1]}
		if start, end, from, to, ok := shadeAxis(triangle.Points, colors); ok {
			gradient := context.Call("createLinearGradient", start.X, start.Y, end.X, end.Y)
			gradient.Call("addColorStop", 0, cssColor(from))
			gradient.Call("addColorStop", 1, cssColor(to))
			return gradient
		}
	}
	return cssColor(triangle.Color)
}

func cssColor(color ui.Color) string {
	red := int(color.Red*255 + 0.5)
	green := int(color.Green*255 + 0.5)
	blue := int(color.Blue*255 + 0.5)
	alpha := color.Alpha
	if alpha > 1 {
		alpha = 1
	}
	if alpha >= 0.999 {
		return fmt.Sprintf("rgb(%d,%d,%d)", clamp8(red), clamp8(green), clamp8(blue))
	}
	return fmt.Sprintf("rgba(%d,%d,%d,%.3f)", clamp8(red), clamp8(green), clamp8(blue), alpha)
}

func clamp8(value int) int {
	if value < 0 {
		return 0
	}
	if value > 255 {
		return 255
	}
	return value
}

// shadowFar is how far, in logical pixels, a shadow's source shape is drawn from
// where its shadow falls, so that only the shadow lands on the canvas.
const shadowFar = 100000

// drawShadow paints a rectangle's soft shadow with the canvas's own shadow
// blur, which has the CSS box-shadow's meaning: the source shape is drawn far off
// to one side and its shadow offset back, because a canvas shadow is cast by
// whatever is drawn. The casting rectangle is cut out of the result with an
// even-odd clip. Shadow offsets and blur are in device pixels, outside the
// canvas transform, so they are scaled by scale.
func drawShadow(context js.Value, shadow ui.Shadow, scale float32) {
	style := shadow.Style
	extent := style.Extent()
	context.Call("save")
	if shadow.HasClip {
		clipRect(context, shadow.ClipOf)
	}
	context.Call("beginPath")
	context.Call("rect", shadow.Bounds.X-extent-1, shadow.Bounds.Y-extent-1, shadow.Bounds.Width+2*extent+2, shadow.Bounds.Height+2*extent+2)
	appendRoundedRect(context, shadow.Bounds, shadow.Radius)
	context.Call("clip", "evenodd")
	source := ui.Rect{
		X:      shadow.Bounds.X - style.Spread - shadowFar,
		Y:      shadow.Bounds.Y - style.Spread,
		Width:  shadow.Bounds.Width + 2*style.Spread,
		Height: shadow.Bounds.Height + 2*style.Spread,
	}
	radius := float32(0)
	if shadow.Radius > 0 {
		radius = max(shadow.Radius+style.Spread, 0)
	}
	context.Set("shadowColor", cssColor(shadow.Color))
	context.Set("shadowBlur", style.Blur*scale)
	context.Set("shadowOffsetX", (style.OffsetX+shadowFar)*scale)
	context.Set("shadowOffsetY", style.OffsetY*scale)
	context.Set("fillStyle", "#000")
	context.Call("beginPath")
	appendRoundedRect(context, source, radius)
	context.Call("fill")
	context.Call("restore")
}

// traceRoundedRect appends a rounded rectangle path to the context.
func traceRoundedRect(context js.Value, bounds ui.Rect, radius float32) {
	context.Call("beginPath")
	appendRoundedRect(context, bounds, radius)
}

// appendRoundedRect adds a rounded rectangle to the context's current path as a
// subpath of its own.
func appendRoundedRect(context js.Value, bounds ui.Rect, radius float32) {
	x, y := bounds.X, bounds.Y
	w, h := bounds.Width, bounds.Height
	r := radius
	if r > w/2 {
		r = w / 2
	}
	if r > h/2 {
		r = h / 2
	}
	context.Call("moveTo", x+r, y)
	context.Call("arcTo", x+w, y, x+w, y+h, r)
	context.Call("arcTo", x+w, y+h, x, y+h, r)
	context.Call("arcTo", x, y+h, x, y, r)
	context.Call("arcTo", x, y, x+w, y, r)
	context.Call("closePath")
}

// keep the unsafe import referenced for future zero-copy buffers.
var _ = unsafe.Sizeof(0)

// printableASCII reports whether piece holds only printable ASCII, which has no shaping.
func printableASCII(piece string) bool {
	for index := range len(piece) {
		if piece[index] < 0x20 || piece[index] >= 0x7f {
			return false
		}
	}
	return piece != ""
}
