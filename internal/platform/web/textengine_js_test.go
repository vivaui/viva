//go:build js && wasm

package web

import (
	"syscall/js"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/textenginetest"
)

// browserEngine is the engine over the page's real document; the tests skip under Node,
// which has none (the geometry is tested on the host in lineengine_test.go).
func browserEngine(tb testing.TB) *lineEngine {
	tb.Helper()
	document := js.Global().Get("document")
	if !document.Truthy() {
		tb.Skip("no document: run in a browser (see docs/conformance.md)")
	}
	return newLineEngine(newDOMMeasurer(document))
}

func TestBrowserEnginePassesTheTextEngineContract(t *testing.T) {
	textenginetest.Run(t, browserEngine(t))
}

// The advance Canvas 2D draws and the extent the DOM lays out are two measurements of one
// line; a caret at the end of the line has to land where the last glyph ends.
func TestBrowserWidthAndCaretAgreeOnMixedScripts(t *testing.T) {
	engine := browserEngine(t)
	for _, value := range []string{"2213123中文", "office affine ffi", "abc שלום def", "👍🏽 ok"} {
		width := engine.Width(value, 14, gfx.WeightRegular)
		if width <= 0 {
			t.Fatalf("%q: width %v", value, width)
		}
		spans := engine.Selection(nil, value, 14, gfx.WeightRegular, 0, len(value))
		var covered float32
		for _, span := range spans {
			covered += span.Width
		}
		if covered < width-0.5 || covered > width+0.5 {
			t.Errorf("%q: the selection of the whole line is %v wide, the line %v", value, covered, width)
		}
	}
}

func BenchmarkBrowserWidthMiss(b *testing.B) {
	engine := browserEngine(b)
	values := make([]string, 1024)
	for i := range values {
		values[i] = "Host name " + string(rune('A'+i%26)) + string(rune('a'+i/26%26)) + " 主机"
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		engine.lines.Clear()
		engine.Width(values[i%len(values)], 14, gfx.WeightRegular)
	}
}

func BenchmarkBrowserGeometryMiss(b *testing.B) {
	engine := browserEngine(b)
	b.ReportAllocs()
	for b.Loop() {
		engine.lines.Clear()
		engine.Caret("192.168.1.1 主机 office affine", 14, gfx.WeightRegular, 10)
	}
}

func BenchmarkBrowserHit(b *testing.B) {
	engine := browserEngine(b)
	const value = "192.168.1.1 主机 office affine"
	dst := make([]gfx.TextSpan, 0, 4)
	engine.Caret(value, 14, gfx.WeightRegular, 3)
	b.ReportAllocs()
	for b.Loop() {
		engine.Width(value, 14, gfx.WeightRegular)
		engine.Caret(value, 14, gfx.WeightRegular, 10)
		engine.OffsetAt(value, 14, gfx.WeightRegular, 60)
		dst = engine.Selection(dst[:0], value, 14, gfx.WeightRegular, 2, 12)
	}
}
