package windows

import (
	"bytes"
	"image"
	"image/color"
	"math"
	"runtime"
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

// wideValue is a log line of 4096 characters: at 8 logical pixels a cell and 150% it is
// 49152 device pixels wide, three times what a Direct3D 11 texture holds.
var wideValue = strings.Repeat("0123456789abcdef", 256)

func wideMonoLine(value string, x float32, runs []gfx.TextRun) platform.TextLabel {
	return platform.TextLabel{
		Value: value, X: x, Y: 4, Width: float32(len(value))*8 + 8, Height: 18, FontSize: 13, CellWidth: 8,
		Mono: true, Runs: runs, Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
		Clipped: true, Clip: gfx.Rect{Width: 400, Height: 40},
	}
}

func wholeRun(value string) []gfx.TextRun {
	return []gfx.TextRun{{Bytes: int32(len(value)), Cells: int32(len(value)), Weight: 400, Color: gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}}}
}

// A line wider than a texture holds gets a window of it around what the viewport and its
// clip show, of any kind, within the limit; one that cannot be seen, or that is too tall
// for any texture, gets none and is not drawn.
func TestWideLabelWindowFollowsTheViewport(t *testing.T) {
	const scale, viewport = 1.5, 600
	for _, runs := range [][]gfx.TextRun{nil, wholeRun(wideValue)} {
		label := labelTexture{label: wideMonoLine(wideValue, 0, runs)}
		skip, width, ok := label.window(scale, viewport)
		if !ok || skip != 0 || width != 1800 {
			t.Errorf("runs %d, start of the line: window = %d, %d, %v; want 0, 1800, true", len(runs), skip, width, ok)
		}
		label.label.X = -20000 // 30000 device pixels scrolled off the left
		skip, width, ok = label.window(scale, viewport)
		if !ok || skip != 30000-600 || width != 1800 {
			t.Errorf("runs %d, scrolled: window = %d, %d, %v; want 29400, 1800, true", len(runs), skip, width, ok)
		}
		label.label.X = -40000
		if _, _, ok = label.window(scale, viewport); ok {
			t.Errorf("runs %d: a line scrolled out of the viewport has a window", len(runs))
		}
		label.label.X, label.label.Clipped = 0, false
		if skip, width, ok = label.window(scale, 0); !ok || width > platform.LabelTextureLimit || skip != 0 {
			t.Errorf("runs %d, no viewport or clip: window = %d, %d, %v; want one texture from the start", len(runs), skip, width, ok)
		}
		label.label.Height = 12000 // 18000 device pixels
		if _, _, ok = label.window(scale, viewport); ok {
			t.Errorf("runs %d: a line too tall for any texture has a window", len(runs))
		}
	}
}

// A monospaced line wider than any texture the device makes, such as a long log line
// shown unwrapped, is rasterized from a window of it at its start and scrolled, as ClearType
// and in grayscale, without and with runs. The window's texture holds the pixels the texture
// of the start of the same line, which one texture holds, has for the columns both show: the
// window moves no glyph.
func TestWideMonoLineWindowHoldsThePixelsOfTheLine(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rasterizer, err := newDirectRasterizer()
	if err != nil {
		t.Skipf("no DirectWrite: %v", err)
	}
	defer rasterizer.close()
	const scale, viewport = 1.5, 600
	raster := func(label platform.TextLabel, lcd bool) (pixels []byte, skip, width, height int) {
		t.Helper()
		texture := labelTexture{label: label}
		skip, width, ok := texture.window(scale, viewport)
		if !ok {
			t.Fatalf("a line of %d characters at %v has no window", len(label.Value), label.X)
		}
		height = int(math.Ceil(float64(label.Height * scale)))
		pixels, _, err := rasterizer.rasterize(label, scale, width, height, lcd, skip)
		if err != nil {
			t.Fatal(err)
		}
		return append([]byte(nil), pixels...), skip, width, height
	}
	for _, lcd := range []bool{true, false} {
		for _, withRuns := range []bool{false, true} {
			for _, scrolled := range []int{0, 1000} { // characters scrolled off the left
				start := wideValue[:scrolled+300]
				wideRuns, startRuns := []gfx.TextRun(nil), []gfx.TextRun(nil)
				if withRuns {
					wideRuns, startRuns = wholeRun(wideValue), wholeRun(start)
				}
				x := -8 * float32(scrolled)
				name := map[bool]string{false: "grayscale", true: "ClearType"}[lcd] + map[bool]string{false: "", true: " with runs"}[withRuns]
				wide, wideSkip, wideWidth, height := raster(wideMonoLine(wideValue, x, wideRuns), lcd)
				want, wantSkip, wantWidth, _ := raster(wideMonoLine(start, x, startRuns), lcd)
				if wideWidth <= 0 || wideWidth > platform.LabelTextureLimit {
					t.Fatalf("%s, %d characters scrolled: the line's texture is %d wide", name, scrolled, wideWidth)
				}
				origin := platform.DevicePosition(x, 0, scale)
				painted := 0
				for column := 0; column < viewport; column++ { // device columns of the target
					got, at := column-origin-wideSkip, column-origin-wantSkip
					if got < 0 || got >= wideWidth || at < 0 || at >= wantWidth {
						t.Fatalf("%s, %d characters scrolled: column %d is outside a texture (%d of %d, %d of %d)",
							name, scrolled, column, got, wideWidth, at, wantWidth)
					}
					for row := 0; row < height; row++ {
						a, b := wide[(row*wideWidth+got)*4:][:4], want[(row*wantWidth+at)*4:][:4]
						if !bytes.Equal(a, b) {
							t.Fatalf("%s, %d characters scrolled: pixel %d,%d is %v, the start of the line's is %v",
								name, scrolled, column, row, a, b)
						}
						if a[3] != 0 {
							painted++
						}
					}
				}
				if painted < 1000 {
					t.Errorf("%s, %d characters scrolled: %d pixels of text", name, scrolled, painted)
				}
			}
		}
	}
}

// Drawn on a target, as ClearType (an opaque one) and in grayscale (a transparent one), the
// wide line is where the start of the same line is, at its start and scrolled. Drawing it
// whole asked Direct3D 11 for a texture wider than it makes, and the error stopped the window
// drawing anything again. A channel may differ by one level: the window's texture starts off
// the target where the start's does not, so the sampler reaches the same texel centres along
// a different quad, and Direct3D's linear filter weighs texels to 8 bits, which can give a
// texel 1/256 of its neighbour. A glyph moved by a device pixel differs by far more.
func TestWideMonoLineIsDrawnFromAWindowOfIt(t *testing.T) {
	for _, clearColor := range []platform.Color{black, {}} {
		for _, withRuns := range []bool{false, true} {
			for _, scrolled := range []int{0, 1000} { // characters scrolled off the left
				start := wideValue[:scrolled+300]
				wideRuns, startRuns := []gfx.TextRun(nil), []gfx.TextRun(nil)
				if withRuns {
					wideRuns, startRuns = wholeRun(wideValue), wholeRun(start)
				}
				x := -8 * float32(scrolled)
				wide := renderOffscreen(t, clearColor, wideMonoLine(wideValue, x, wideRuns))
				want := renderOffscreen(t, clearColor, wideMonoLine(start, x, startRuns))
				name := map[bool]string{false: "grayscale", true: "ClearType"}[clearColor.Alpha >= 1] +
					map[bool]string{false: "", true: " with runs"}[withRuns]
				painted := 0
				for y := 0; y < 60; y++ {
					for x := 0; x < 600; x++ {
						if a, b := wide.RGBAAt(x, y), want.RGBAAt(x, y); !withinALevel(a, b) {
							t.Fatalf("%s, %d characters scrolled: pixel %d,%d is %v, the start of the line's is %v",
								name, scrolled, x, y, a, b)
						}
						if isPainted(wide, x, y) {
							painted++
						}
					}
				}
				if painted < 1000 {
					t.Errorf("%s, %d characters scrolled: %d pixels of text painted", name, scrolled, painted)
				}
			}
		}
	}
}

// withinALevel reports whether no channel of a and b differs by more than one level.
func withinALevel(a, b color.RGBA) bool {
	near := func(p, q uint8) bool { return int(p)-int(q) <= 1 && int(q)-int(p) <= 1 }
	return near(a.R, b.R) && near(a.G, b.G) && near(a.B, b.B) && near(a.A, b.A)
}

// A label no texture holds, an image larger than the device allows or a line taller than
// a texture, is not drawn, and the labels beside it still are: one label the device cannot
// hold does not fail the frame.
func TestLabelNoTextureHoldsDoesNotFailTheFrame(t *testing.T) {
	const imageWidth = platform.LabelTextureLimit + 16
	tall := platform.TextLabel{Value: "tall", X: 200, Y: 0, Width: 40, Height: platform.LabelTextureLimit + 1, FontSize: 13,
		Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}}
	wide := platform.TextLabel{X: 0, Y: 30, Width: imageWidth, Height: 2,
		Bitmap: make([]byte, imageWidth*2*4), BitmapWidth: imageWidth, BitmapHeight: 2, BitmapStride: imageWidth * 4, BitmapID: 1, BitmapVersion: 1}
	beside := platform.TextLabel{Value: "beside", X: 10, Y: 4, Width: 80, Height: 18, FontSize: 13,
		Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}}
	pixels := renderOffscreen(t, black, tall, wide, beside)
	painted := 0
	for y := 0; y < 40; y++ { // the label beside, at 150%
		for x := 0; x < 150; x++ {
			if isPainted(pixels, x, y) {
				painted++
			}
		}
	}
	if painted == 0 {
		t.Fatal("the label beside one no texture holds was not drawn")
	}
}

// renderOffscreen draws the labels on a 400 by 40 scene at 150%, cleared to clearColor.
func renderOffscreen(t *testing.T, clearColor platform.Color, labels ...platform.TextLabel) *image.RGBA {
	t.Helper()
	offscreen, err := NewOffscreen(400, 40, 1.5, clearColor)
	if err != nil {
		t.Skipf("no Direct3D 11 device: %v", err)
	}
	defer offscreen.Close()
	return renderScene(t, offscreen, nil, labels)
}
