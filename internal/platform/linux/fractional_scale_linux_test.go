package linux

import (
	"image"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

// crop returns the device pixels of a width by height rectangle at (x, y).
func crop(pixels *image.RGBA, x, y, width, height int) *image.RGBA {
	out := image.NewRGBA(image.Rect(0, 0, width, height))
	for row := range height {
		for column := range width {
			out.SetRGBA(column, row, pixels.RGBAAt(x+column, y+row))
		}
	}
	return out
}

// largestDifference is the greatest per-channel distance between two equal-sized images.
func largestDifference(a, b *image.RGBA) int {
	largest := 0
	for index := range a.Pix {
		difference := int(a.Pix[index]) - int(b.Pix[index])
		largest = max(largest, difference, -difference)
	}
	return largest
}

// fringed counts the pixels in a column range whose channels differ, which only LCD text
// has: grayscale antialiasing leaves red, green and blue equal.
func fringed(pixels *image.RGBA, x0, x1 int) int {
	count := 0
	for y := 0; y < pixels.Bounds().Dy(); y++ {
		for x := x0; x < x1; x++ {
			color := pixels.RGBAAt(x, y)
			if max(color.R, color.G, color.B)-min(color.R, color.G, color.B) > 24 {
				count++
			}
		}
	}
	return count
}

// At 150% a label placed at an odd logical x starts half a device pixel in. Drawn there,
// the sampler blended every pair of texels and the text went soft; the label now starts on
// a whole device pixel, so the same text at any logical position is the very same pixels,
// only moved.
func TestTextAtAFractionalScaleIsNotResampled(t *testing.T) {
	const scale = 1.5
	label := platform.TextLabel{Value: "Hello Hello", Width: 90, Height: 20, FontSize: 14, Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}}
	width, height := 135, 30 // the label's device size
	render := func(x, y float32) *image.RGBA {
		offscreen := newScaledOffscreen(t, 160, 60, scale)
		label.X, label.Y = x, y
		pixels := renderScene(t, offscreen, nil, []platform.TextLabel{label})
		return crop(pixels, platform.DevicePosition(x, 0, scale), platform.DevicePosition(y, 0, scale), width, height)
	}
	reference := render(10, 10) // on whole device pixels already: 15, 15
	lit := 0
	for index := 0; index < len(reference.Pix); index += 4 {
		if int(reference.Pix[index])+int(reference.Pix[index+1])+int(reference.Pix[index+2]) > 60 {
			lit++
		}
	}
	if lit < 100 {
		t.Fatalf("the reference label drew %d lit pixels", lit)
	}
	for _, at := range [][2]float32{{11, 10}, {13, 11}, {10, 13}} {
		if difference := largestDifference(reference, render(at[0], at[1])); difference > 2 {
			t.Errorf("label at %v differs from the one on whole pixels by %d: it was resampled", at, difference)
		}
	}
}

// A bitmap shown smaller than it is stored is averaged over the pixels it covers, not
// sampled at a few points, so a one-pixel dot in it does not vanish.
func TestBitmapShownSmallerKeepsItsDetail(t *testing.T) {
	const size = 12
	bitmap := make([]byte, size*size*4)
	for index := 3; index < len(bitmap); index += 4 {
		bitmap[index] = 255
	}
	copy(bitmap[0:4], []byte{255, 255, 255, 255}) // a white dot in the top left corner
	offscreen := newTestOffscreen(t, 8, 8)
	pixels := renderScene(t, offscreen, nil, []platform.TextLabel{{
		X: 2, Y: 2, Width: 4, Height: 4,
		Bitmap: bitmap, BitmapWidth: size, BitmapHeight: size, BitmapStride: size * 4, BitmapID: 1,
	}})
	if r, _, _, _ := rgbaAt(pixels, 2, 2); r < 15 {
		t.Fatalf("the corner pixel's red is %d: the dot was lost", r)
	}
}

// A terminal line is LCD text like any other, and each run of it keeps its own color: red
// text has no green in it however its edges are filtered.
func TestTerminalLineIsLCDInTheColorOfEachRun(t *testing.T) {
	useSystemFonts(t, fontSettings{Antialias: cairoAntialiasSubpixel, HintStyle: cairoHintSlight, SubpixelOrder: cairoSubpixelRGB})
	offscreen := newTestOffscreen(t, 130, 24)
	plain := renderScene(t, offscreen, nil, []platform.TextLabel{{Value: "AAAAAAAA", X: 4, Y: 2, Width: 100, Height: 20, FontSize: 14, Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}}})
	if fringed(plain, 0, 130) == 0 {
		t.Skip("no dual-source blending on this device: labels are never LCD")
	}
	white, red := gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}, gfx.Color{Red: 1, Alpha: 1}
	pixels := renderScene(t, offscreen, nil, []platform.TextLabel{{
		Value: "AAAAAAAABBBBBBBB", Mono: true, CellWidth: 8, X: 4, Y: 2, Width: 128, Height: 20, FontSize: 14, Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
		Runs: []gfx.TextRun{{Column: 0, Bytes: 8, Cells: 8, Color: white}, {Column: 8, Bytes: 8, Cells: 8, Color: red}},
	}})
	if fringed(pixels, 4, 68) < 10 {
		t.Error("the white run of the terminal line has no LCD fringes: it was drawn grayscale")
	}
	lit := 0
	for y := 0; y < 24; y++ {
		for x := 70; x < 130; x++ {
			r, g, b, _ := rgbaAt(pixels, x, y)
			if r > 60 {
				lit++
			}
			if g > 8 || b > 8 {
				t.Fatalf("pixel (%d,%d) of the red run is (%d,%d,%d): it has green or blue", x, y, r, g, b)
			}
		}
	}
	if lit < 20 {
		t.Errorf("the red run drew %d red pixels", lit)
	}
}
