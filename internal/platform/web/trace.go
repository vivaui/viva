//go:build js && wasm

package web

import "syscall/js"

// frameClock records where a frame's time goes, for the page that asks: it is
// on only when the page defined window.vivaWebStats before the program started
// (a benchmark harness does; an application never needs to). Off, every method
// returns at once and nothing calls into the browser.
//
// The object it fills holds running totals since the page started, so a harness
// reads it before and after a scenario and subtracts:
//
//	frames, totalMs, a11yMs, rectanglesMs, trianglesMs, textsMs
//	rectangles, triangles, texts, hits, pathsBuilt, pathsReused   (counts in the last frame)
type frameClock struct {
	stats js.Value
	perf  js.Value
	on    bool

	start, last float64
	a11y        float64 // this frame's phases, in ms
	rectangles  float64
	triangles   float64
	texts       float64
	sum         [6]float64 // running totals: frames, total, then the four phases
}

func newFrameClock() frameClock {
	stats := js.Global().Get("vivaWebStats")
	if stats.Type() != js.TypeObject {
		return frameClock{}
	}
	return frameClock{stats: stats, perf: js.Global().Get("performance"), on: true}
}

func (clock *frameClock) now() float64 { return clock.perf.Call("now").Float() }

func (clock *frameClock) begin() {
	if !clock.on {
		return
	}
	clock.a11y, clock.rectangles, clock.triangles, clock.texts = 0, 0, 0, 0
	clock.start = clock.now()
	clock.last = clock.start
}

// lap adds the time since the previous lap to phase.
func (clock *frameClock) lap(phase *float64) {
	if !clock.on {
		return
	}
	now := clock.now()
	*phase += now - clock.last
	clock.last = now
}

func (clock *frameClock) end(painter *painter, pathsBuilt, pathsReused int) {
	if !clock.on {
		return
	}
	total := clock.now() - clock.start
	for index, value := range [...]float64{1, total, clock.a11y, clock.rectangles, clock.triangles, clock.texts} {
		clock.sum[index] += value
	}
	for index, key := range [...]string{"frames", "totalMs", "a11yMs", "rectanglesMs", "trianglesMs", "textsMs"} {
		clock.stats.Set(key, clock.sum[index])
	}
	clock.stats.Set("rectangles", len(painter.rectangles))
	clock.stats.Set("triangles", len(painter.triangles))
	clock.stats.Set("texts", len(painter.texts))
	clock.stats.Set("hits", len(painter.accessibility))
	clock.stats.Set("pathsBuilt", pathsBuilt)
	clock.stats.Set("pathsReused", pathsReused)
}
