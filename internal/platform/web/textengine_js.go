//go:build js && wasm

package web

import (
	"syscall/js"
)

// domMeasurer is the browser as a lineMeasurer. Widths come from Canvas 2D
// measureText, which is what fillText draws. The extent of each grapheme cluster comes
// from a DOM Range over a hidden element in the same font and direction: the browser's
// own shaping, ligature carets and bidirectional order, which measureText cannot give.
// Both set their font only when it changes, and the Range is only used the first time a
// caret, selection or click asks about a line.
type domMeasurer struct {
	document js.Value
	context  js.Value // Canvas 2D context of an off-screen canvas
	font     string
	rtl      bool

	ruler      js.Value // hidden element, created on first use
	node       js.Value // its text node
	span       js.Value // reusable Range
	rulerFont  string
	rulerRTL   bool
	rulerValue string

	invalidate func()
	listener   js.Func
}

func newDOMMeasurer(document js.Value) *domMeasurer {
	measurer := &domMeasurer{document: document}
	measurer.context = document.Call("createElement", "canvas").Call("getContext", "2d")
	measurer.setDirection(false)
	// Fonts the page loads after the first paint change every width (the engine only
	// asks for system-ui, but the page may name another family in its own CSS).
	if fonts := document.Get("fonts"); fonts.Truthy() {
		measurer.listener = js.FuncOf(func(js.Value, []js.Value) any {
			measurer.forget()
			return nil
		})
		fonts.Call("addEventListener", "loadingdone", measurer.listener)
	}
	return measurer
}

func (measurer *domMeasurer) setInvalidate(invalidate func()) { measurer.invalidate = invalidate }

// forget drops everything measured so far: a font arrived, or the zoom changed.
func (measurer *domMeasurer) forget() {
	if measurer.invalidate != nil {
		measurer.invalidate()
	}
}

func (measurer *domMeasurer) setDirection(rtl bool) {
	direction := "ltr"
	if rtl {
		direction = "rtl"
	}
	measurer.context.Set("direction", direction)
	measurer.rtl = rtl
}

func (measurer *domMeasurer) width(value string, size, weight float32, rtl bool) float32 {
	return measurer.widthStyle(value, size, weight, false, rtl)
}

func (measurer *domMeasurer) widthStyle(value string, size, weight float32, italic, rtl bool) float32 {
	if font := cssFontStyle(size, weight, italic); font != measurer.font {
		measurer.context.Set("font", font)
		measurer.font = font
	}
	if rtl != measurer.rtl {
		measurer.setDirection(rtl)
	}
	return float32(measurer.context.Call("measureText", value).Get("width").Float())
}

func (measurer *domMeasurer) mountRuler() {
	ruler := measurer.document.Call("createElement", "div")
	style := ruler.Get("style")
	style.Set("position", "absolute")
	style.Set("left", "0")
	style.Set("top", "0")
	style.Set("visibility", "hidden")
	style.Set("pointerEvents", "none")
	style.Set("whiteSpace", "pre")
	style.Set("margin", "0")
	style.Set("padding", "0")
	style.Set("border", "0")
	ruler.Call("setAttribute", "aria-hidden", "true")
	node := measurer.document.Call("createTextNode", "")
	ruler.Call("appendChild", node)
	measurer.document.Get("body").Call("appendChild", ruler)
	measurer.ruler, measurer.node = ruler, node
	measurer.span = measurer.document.Call("createRange")
}

func (measurer *domMeasurer) rects(value string, size, weight float32, rtl bool, bounds []int32, lefts, rights []float32) {
	measurer.rectsStyle(value, size, weight, false, rtl, bounds, lefts, rights)
}

func (measurer *domMeasurer) rectsStyle(value string, size, weight float32, italic, rtl bool, bounds []int32, lefts, rights []float32) {
	if !measurer.ruler.Truthy() {
		measurer.mountRuler()
	}
	if font := cssFontStyle(size, weight, italic); font != measurer.rulerFont {
		measurer.ruler.Get("style").Set("font", font)
		measurer.rulerFont = font
	}
	if rtl != measurer.rulerRTL || measurer.rulerValue == "" {
		direction := "ltr"
		if rtl {
			direction = "rtl"
		}
		measurer.ruler.Get("style").Set("direction", direction)
		measurer.rulerRTL = rtl
	}
	if value != measurer.rulerValue {
		measurer.node.Set("data", value)
		measurer.rulerValue = value
	}
	origin := measurer.ruler.Call("getBoundingClientRect").Get("left").Float()
	for i := range lefts {
		measurer.span.Call("setStart", measurer.node, bounds[i])
		measurer.span.Call("setEnd", measurer.node, bounds[i+1])
		rect := measurer.span.Call("getBoundingClientRect")
		lefts[i] = float32(rect.Get("left").Float() - origin)
		rights[i] = float32(rect.Get("right").Float() - origin)
	}
}
