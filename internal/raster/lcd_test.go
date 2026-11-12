package raster

import (
	"testing"

	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

// An LCD label's picture holds per-channel coverage of white text; each channel
// is tinted and blended with its own coverage, the way the GPU's LCD shader
// reads it, rather than with one alpha for the whole pixel.
func TestLCDLabelTintsEachChannelWithItsOwnCoverage(t *testing.T) {
	c := NewCanvas(2, 1)
	clear := scene.Color{Red: 0.2, Green: 0.4, Blue: 0.6, Alpha: 1}
	pixels := []byte{
		// blue, green, red, alpha: red channel only
		0, 0, 255, 255,
		// blue channel only
		255, 0, 0, 255,
	}
	label := Label{
		Pix: pixels, Stride: 8, Width: 2, Height: 1, X: 0, Y: 0,
		LCD: true, Color: scene.Color{Red: 1, Alpha: 1},
	}
	var drawer Rasterizer
	drawer.Draw(c, Params{Scale: 1, Clear: clear}, nil, []Label{label})

	// Pixel 0: red fully covered by the label's red, green and blue untouched.
	if got, want := at(c, 0, 0), [4]byte{unorm8(clear.Blue), unorm8(clear.Green), 255, 255}; got != want {
		t.Fatalf("red-covered pixel = %v, want %v", got, want)
	}
	// Pixel 1: blue fully covered by the label's blue (0), red and green untouched.
	if got, want := at(c, 1, 0), [4]byte{0, unorm8(clear.Green), unorm8(clear.Red), 255}; got != want {
		t.Fatalf("blue-covered pixel = %v, want %v", got, want)
	}
}

// A fixed-pitch line has one picture of coverage and one color per run; the
// columns of each span take that span's color and the columns outside draw
// nothing.
func TestLCDLabelTintsTheColumnsOfEachRun(t *testing.T) {
	c := NewCanvas(4, 1)
	pixels := make([]byte, 16)
	for index := 0; index < 16; index += 4 {
		pixels[index], pixels[index+1], pixels[index+2], pixels[index+3] = 255, 255, 255, 255
	}
	label := Label{
		Pix: pixels, Stride: 16, Width: 4, Height: 1,
		LCD: true, Color: scene.Color{Blue: 1, Alpha: 1},
		Spans: []Span{
			{From: 0, To: 2, Color: scene.Color{Red: 1, Alpha: 1}},
			{From: 2, To: 4, Color: scene.Color{Green: 1, Alpha: 1}},
		},
	}
	var drawer Rasterizer
	drawer.Draw(c, Params{Scale: 1, Clear: scene.Color{}}, nil, []Label{label})

	for x, want := range [][4]byte{{0, 0, 255, 255}, {0, 0, 255, 255}, {0, 255, 0, 255}, {0, 255, 0, 255}} {
		if got := at(c, x, 0); got != want {
			t.Errorf("column %d = %v, want %v", x, got, want)
		}
	}
}

// The color's alpha scales the coverage, as the shader's tint does: a
// half-transparent label tints half as strongly and keeps the target's alpha.
func TestLCDLabelAppliesTheColorsAlpha(t *testing.T) {
	c := NewCanvas(1, 1)
	label := Label{
		Pix: []byte{0, 0, 255, 76}, Stride: 4, Width: 1, Height: 1,
		LCD: true, Color: scene.Color{Red: 1, Alpha: 0.5},
	}
	var drawer Rasterizer
	drawer.Draw(c, Params{Scale: 1, Clear: scene.Color{Alpha: 1}}, nil, []Label{label})

	got := at(c, 0, 0)
	// Red coverage 255 * 0.5 = 127; over opaque black that is 127, and alpha
	// remains 255 over an opaque target.
	if got[2] < 126 || got[2] > 128 || got[3] != 255 {
		t.Fatalf("half-alpha LCD text = %v, want about {0, 0, 127, 255}", got)
	}
}

// BenchmarkDrawLCDText measures tinting a line of subpixel coverage: the CPU
// renderer does this for every visible text label on every damaged paint.
func BenchmarkDrawLCDText(b *testing.B) {
	const width, height = 1280, 20
	canvas := NewCanvas(width, height)
	pixels := make([]byte, width*height*4)
	for index := range pixels {
		pixels[index] = byte(index * 31)
	}
	label := Label{
		Pix: pixels, Stride: width * 4, Width: width, Height: height,
		LCD: true, Color: scene.Color{Red: 0.2, Green: 0.6, Blue: 0.9, Alpha: 1},
	}
	params := Params{Scale: 1, Clear: scene.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}}
	var drawer Rasterizer
	drawer.Draw(canvas, params, nil, []Label{label})
	b.ReportAllocs()
	b.SetBytes(int64(len(pixels)))
	b.ResetTimer()
	for range b.N {
		drawer.Draw(canvas, params, nil, []Label{label})
	}
}

// Drawing an LCD label allocates nothing: the spans are the caller's slice and
// the blend uses no scratch.
func TestLCDLabelDrawAllocatesNothing(t *testing.T) {
	c := NewCanvas(64, 20)
	pixels := make([]byte, 64*20*4)
	for index := range pixels {
		pixels[index] = byte(index * 7)
	}
	label := Label{
		Pix: pixels, Stride: 64 * 4, Width: 64, Height: 20,
		LCD: true, Color: scene.Color{Red: 0.2, Green: 0.6, Blue: 0.9, Alpha: 1},
		Spans: []Span{
			{From: 0, To: 32, Color: scene.Color{Red: 0.2, Green: 0.6, Blue: 0.9, Alpha: 1}},
			{From: 32, To: 64, Color: scene.Color{Red: 0.9, Green: 0.2, Blue: 0.2, Alpha: 1}},
		},
	}
	var drawer Rasterizer
	params := Params{Scale: 1, Clear: scene.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}, Damage: render.Scissor{Right: 64, Bottom: 20}}
	drawer.Draw(c, params, nil, []Label{label})
	if allocs := testing.AllocsPerRun(50, func() {
		drawer.Draw(c, params, nil, []Label{label})
	}); allocs != 0 {
		t.Fatalf("%g allocations per LCD label draw, want 0", allocs)
	}
}
