//go:build js && wasm

package web

import (
	"fmt"
	"syscall/js"

	"github.com/vivaui/viva/internal/bidi"
	"github.com/vivaui/viva/ui"
)

// domMeasurer is the browser as a lineMeasurer. Widths come from Canvas 2D
// measureText, which is what fillText draws. The extent of each grapheme cluster comes
// from a DOM Range over a hidden element in the same font and direction: the browser's
// own shaping, ligature carets and bidirectional order, which measureText cannot give.
// Both set their font only when it changes, and the Range is only used the first time a
// caret, selection or click asks about a line.
type domMeasurer struct {
	document      js.Value
	context       js.Value // Canvas 2D context of an off-screen canvas
	rasterContext js.Value // separate off-screen context: resizing it must not reset measurement state
	font          string
	rtl           bool

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
	fontInvalidators = append(fontInvalidators, measurer.forget)
	return measurer
}

func (measurer *domMeasurer) setInvalidate(invalidate func()) { measurer.invalidate = invalidate }

func (measurer *domMeasurer) rasterizeText(value string, style ui.TextStyle, scale float32, width, height int) (ui.Bitmap, error) {
	if width <= 0 || height <= 0 || width > 16384 || height > 16384 || width*height > 16*1024*1024 || scale <= 0 {
		return ui.Bitmap{}, fmt.Errorf("web: invalid text bitmap size or scale")
	}
	if !measurer.rasterContext.Truthy() {
		measurer.rasterContext = measurer.document.Call("createElement", "canvas").Call("getContext", "2d")
	}
	context := measurer.rasterContext
	canvas := context.Get("canvas")
	canvas.Set("width", width)
	canvas.Set("height", height)
	context.Set("textBaseline", "alphabetic")
	context.Set("textAlign", "left")
	context.Set("direction", "ltr")
	context.Set("fillStyle", cssColor(style.Color))
	if bidi.BaseDirection(value) == bidi.RTL {
		context.Set("direction", "rtl")
	}

	context.Set("font", cssFont(style.Size*scale, 0))
	metrics := context.Call("measureText", "Ag")
	ascent := metrics.Get("fontBoundingBoxAscent").Float()
	descent := metrics.Get("fontBoundingBoxDescent").Float()
	if ascent <= 0 {
		ascent = metrics.Get("actualBoundingBoxAscent").Float()
		descent = metrics.Get("actualBoundingBoxDescent").Float()
	}
	baseline := (float64(height)-ascent-descent)*0.5 + ascent
	context.Set("font", cssFontStyle(style.Size*scale, style.Weight, style.Italic))
	context.Call("fillText", value, 0, baseline)

	data := context.Call("getImageData", 0, 0, width, height).Get("data")
	rgba := make([]byte, width*height*4)
	if copied := js.CopyBytesToGo(rgba, data); copied != len(rgba) {
		return ui.Bitmap{}, fmt.Errorf("web: copy text bitmap: copied %d of %d bytes", copied, len(rgba))
	}
	pixels := make([]byte, len(rgba))
	for index := 0; index < len(rgba); index += 4 {
		alpha := int(rgba[index+3])
		pixels[index+0] = byte(int(rgba[index+2]) * alpha / 255)
		pixels[index+1] = byte(int(rgba[index+1]) * alpha / 255)
		pixels[index+2] = byte(int(rgba[index+0]) * alpha / 255)
		pixels[index+3] = rgba[index+3]
	}
	return ui.Bitmap{Pix: pixels, Width: width, Height: height, Stride: width * 4}, nil
}

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
