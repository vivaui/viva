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
	"unicode/utf8"
	"unsafe"

	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

// Config describes the browser deployment.
type Config struct {
	Title      string
	Width      int
	Height     int
	Paint      func(canvas *ui.Canvas, size ui.Size)
	OnPointer  func(event ui.PointerEvent) ui.EventResult
	OnKey      func(event ui.KeyEvent) ui.EventResult
	OnShortcut func(event ui.KeyEvent) ui.EventResult
	OnScroll   func(event ui.ScrollEvent) ui.EventResult
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
	config    Config
	canvas    *ui.Canvas
	painter   *painter
	document  js.Value
	window    js.Value
	focusRoot js.Value
	surface   js.Value
	context   js.Value
	a11y      *accessibilityMirror
	dpr       float32
	pending   bool
	metrics   *textMetrics
	measurer  *domMeasurer
	clock     frameClock
	paths     pathCache
}

// Run mounts the application into the document and returns when the page
// unloads. It must be called from the main goroutine of a WASM binary.
func Run(config Config) error {
	if config.Paint == nil {
		return fmt.Errorf("web: Paint is required")
	}
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
	focusRoot := runtime.document.Call("createElement", "div")
	rootStyle := focusRoot.Get("style")
	rootStyle.Set("position", "fixed")
	rootStyle.Set("inset", "0")
	rootStyle.Set("overflow", "hidden")
	rootStyle.Set("outline", "none")

	surface := runtime.document.Call("createElement", "canvas")
	style := surface.Get("style")
	style.Set("position", "absolute")
	style.Set("inset", "0")
	style.Set("width", "100%")
	style.Set("height", "100%")
	style.Set("cursor", "default")
	style.Set("userSelect", "none")

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
	// A font that loads late changes the baselines and widths as it does every line.
	clearLines := runtime.measurer.invalidate
	runtime.measurer.setInvalidate(func() {
		clearLines()
		runtime.metrics.clear()
	})
	runtime.metrics = newTextMetrics(runtime.measurer)
	runtime.painter = newPainter(runtime.canvas, &runtime.config)
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
	height := runtime.window.Get("innerHeight").Int()
	runtime.surface.Set("width", int(float32(width)*runtime.dpr))
	runtime.surface.Set("height", int(float32(height)*runtime.dpr))
	runtime.painter.resize(width, height)
}

// requestFrame schedules one render on the browser's display pipeline. The
// runtime is event-driven: frames happen only when something changed.
func (runtime *runtime) requestFrame() {
	if runtime.pending {
		return
	}
	runtime.pending = true
	js.Global().Get("window").Call("requestAnimationFrame", js.FuncOf(func(js.Value, []js.Value) any {
		runtime.pending = false
		runtime.render()
		return nil
	}))
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

// bindEvents wires pointer and keyboard input to the interaction router.
func (runtime *runtime) bindEvents() {
	surface := runtime.surface

	surface.Call("addEventListener", "pointermove", js.FuncOf(func(_ js.Value, args []js.Value) any {
		event := args[0]
		if runtime.painter.pointer(pointerEvent(ui.PointerMove, event)) {
			runtime.requestFrame()
		}
		return nil
	}))
	surface.Call("addEventListener", "pointerdown", js.FuncOf(func(_ js.Value, args []js.Value) any {
		event := args[0]
		pointerID := event.Get("pointerId").Int()
		surface.Call("setPointerCapture", pointerID)
		if runtime.painter.pointer(pointerEvent(ui.PointerDown, event)) {
			runtime.requestFrame()
		}
		runtime.focusRoot.Call("focus", map[string]any{"preventScroll": true})
		return nil
	}))
	surface.Call("addEventListener", "pointerup", js.FuncOf(func(_ js.Value, args []js.Value) any {
		event := args[0]
		if runtime.painter.pointer(pointerEvent(ui.PointerUp, event)) {
			runtime.requestFrame()
		}
		pointerID := event.Get("pointerId").Int()
		if surface.Call("hasPointerCapture", pointerID).Bool() {
			surface.Call("releasePointerCapture", pointerID)
		}
		return nil
	}))
	surface.Call("addEventListener", "pointerleave", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if runtime.painter.pointer(pointerEvent(ui.PointerExit, args[0])) {
			runtime.requestFrame()
		}
		return nil
	}))
	surface.Call("addEventListener", "pointercancel", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if runtime.painter.pointer(pointerEvent(ui.PointerExit, args[0])) {
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
	runtime.focusRoot.Call("addEventListener", "keydown", js.FuncOf(func(_ js.Value, args []js.Value) any {
		event := args[0]
		modifiers := webModifiers(event.Get("shiftKey").Bool(), event.Get("ctrlKey").Bool(), event.Get("altKey").Bool(), event.Get("metaKey").Bool())
		shortcut := strings.ToLower(event.Get("key").String())
		if runtime.config.OnShortcut != nil && modifiers&(ui.ModifierControl|ui.ModifierMeta) != 0 && utf8.RuneCountInString(shortcut) == 1 {
			handled, changed := runtime.painter.shortcut(ui.KeyEvent{Modifiers: modifiers, Repeat: event.Get("repeat").Bool(), Text: shortcut})
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
		if runtime.a11y.text.activeElement(event.Get("target")) {
			if event.Get("key").String() == "Tab" {
				event.Call("preventDefault")
				if runtime.painter.keyDown(ui.KeyEvent{
					Key:       input.KeyTab,
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
	case "Backspace":
		return input.KeyBackspace, true
	case "Delete":
		return input.KeyDelete, true
	default:
		return 0, false
	}
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
