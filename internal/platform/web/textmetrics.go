//go:build js && wasm

package web

import (
	"syscall/js"
)

// maxCachedWidths bounds the width cache; past it the cache starts over.
const maxCachedWidths = 8192

type widthKey struct {
	value        string
	size, weight float32
}

// textMetrics remembers what the browser told us about the proportional UI font,
// so a frame that draws the same lines again asks the browser nothing.
//
// Every frame needs each line's baseline offsets (what centers it in its box) and
// width (whether it fits, and so whether it needs clipping); measuring either
// costs browser calls. A font arriving late changes both: clear drops what was
// measured.
type textMetrics struct {
	context   js.Value
	measurer  *domMeasurer
	baselines map[float32]baselineMetrics
	widths    map[widthKey]float32
}

func newTextMetrics(measurer *domMeasurer) *textMetrics {
	context := js.Global().Get("document").Call("createElement", "canvas").Call("getContext", "2d")
	return &textMetrics{
		context: context, measurer: measurer,
		baselines: make(map[float32]baselineMetrics), widths: make(map[widthKey]float32),
	}
}

// baseline reports the font's ascent and descent at size, in the formula the native
// CoreText labels use. Widths, carets and hit-testing come from the text engine
// (textengine_js.go).
func (metrics *textMetrics) baseline(fontSize float32) baselineMetrics {
	if cached, ok := metrics.baselines[fontSize]; ok {
		return cached
	}
	metrics.context.Set("font", cssFont(fontSize, 0))
	measured := metrics.context.Call("measureText", "Ag")
	ascent := float32(measured.Get("fontBoundingBoxAscent").Float())
	descent := float32(measured.Get("fontBoundingBoxDescent").Float())
	if ascent <= 0 {
		ascent = float32(measured.Get("actualBoundingBoxAscent").Float())
		descent = float32(measured.Get("actualBoundingBoxDescent").Float())
	}
	cached := baselineMetrics{Ascent: ascent, Descent: descent}
	metrics.baselines[fontSize] = cached
	return cached
}

// width reports the advance of a left-to-right line of text.
func (metrics *textMetrics) width(value string, size, weight float32) float32 {
	key := widthKey{value, size, weight}
	if cached, ok := metrics.widths[key]; ok {
		return cached
	}
	if len(metrics.widths) >= maxCachedWidths {
		clear(metrics.widths)
	}
	width := metrics.measurer.width(value, size, weight, false)
	metrics.widths[key] = width
	return width
}

func (metrics *textMetrics) clear() {
	clear(metrics.baselines)
	clear(metrics.widths)
}
