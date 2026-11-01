package raster

import (
	"math/rand"
	"testing"

	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

// uiScene is the kind of frame a desktop application draws: panels, rounded buttons and cards with
// shadows, thin dividers, and a few translucent overlays, over a window of the given logical size.
func uiScene(width, height float32) []render.Rectangle {
	random := rand.New(rand.NewSource(1))
	out := []render.Rectangle{{Kind: scene.KindRectangle, Bounds: scene.Rect{Width: width, Height: height}, Color: scene.Color{Red: 0.97, Green: 0.97, Blue: 0.98, Alpha: 1}}}
	out = append(out, render.Rectangle{Kind: scene.KindRectangle, Bounds: scene.Rect{Width: 220, Height: height}, Color: scene.Color{Red: 0.93, Green: 0.94, Blue: 0.96, Alpha: 1}})
	for i := 0; i < 24; i++ {
		y := float32(40 + i*28)
		out = append(out, render.Rectangle{Kind: scene.KindRectangle, Bounds: scene.Rect{X: 12, Y: y, Width: 196, Height: 24}, Color: scene.Color{Red: 0.1, Green: 0.25, Blue: 0.9, Alpha: 0.12}, Radius: 6})
	}
	for i := 0; i < 60; i++ {
		x, y := 240+float32(i%4)*190, 20+float32(i/4)*60
		card := scene.Rect{X: x, Y: y, Width: 176, Height: 48}
		out = append(out,
			render.Rectangle{Kind: scene.KindShadow, Bounds: card, Radius: 8, Color: scene.Color{Alpha: 0.18}, Shadow: scene.Shadow{OffsetY: 2, Blur: 6}},
			render.Rectangle{Kind: scene.KindRectangle, Bounds: card, Radius: 8, Color: scene.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}},
			render.Rectangle{Kind: scene.KindRectangle, Bounds: scene.Rect{X: x + 12, Y: y + 14, Width: 60 + random.Float32()*80, Height: 8}, Color: scene.Color{Red: 0.6, Green: 0.62, Blue: 0.68, Alpha: 1}, Radius: 4})
	}
	return out
}

func benchmarkDraw(b *testing.B, scale float32, damaged bool) {
	const width, height = 1280, 800
	canvas := NewCanvas(int(width*scale), int(height*scale))
	rects := uiScene(width, height)
	params := Params{Scale: scale, Clear: scene.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}}
	if damaged {
		// The line a text field redraws as the user types.
		params.Damage = render.Scissor{Left: int(260 * scale), Top: int(300 * scale), Right: int(460 * scale), Bottom: int(332 * scale)}
	}
	var r Rasterizer
	r.Draw(canvas, params, rects, nil) // warm the scratch lists
	b.ReportAllocs()
	b.SetBytes(int64(canvas.Width * canvas.Height * 4))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Draw(canvas, params, rects, nil)
	}
}

func BenchmarkDrawWindow(b *testing.B)           { benchmarkDraw(b, 1, false) }
func BenchmarkDrawWindow150Percent(b *testing.B) { benchmarkDraw(b, 1.5, false) }
func BenchmarkDrawTypingLine(b *testing.B)       { benchmarkDraw(b, 1, true) }

func TestSteadyFramesAllocateNothing(t *testing.T) {
	canvas := NewCanvas(640, 400)
	rects := uiScene(640, 400)
	labels := []Label{{Pix: make([]byte, 64*16*4), Stride: 64 * 4, Width: 64, Height: 16, X: 10, Y: 10, Layer: 1}}
	var r Rasterizer
	params := Params{Scale: 1.5, Clear: scene.Color{Alpha: 1}}
	r.Draw(canvas, params, rects, labels)
	if allocs := testing.AllocsPerRun(20, func() { r.Draw(canvas, params, rects, labels) }); allocs != 0 {
		t.Fatalf("a steady frame allocates %v times, want 0", allocs)
	}
}
