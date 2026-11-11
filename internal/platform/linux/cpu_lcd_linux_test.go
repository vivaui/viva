package linux

import (
	"errors"
	"math"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

// With LCD antialiasing on the desktop, the CPU renderer draws text as
// per-channel coverage and blends it per channel, exactly as the Vulkan LCD
// shader does: dark text on white has dark stems with colored edges. Grayscale
// settings keep the edges gray.
func TestCPUOffscreenLCDTextIsBlendedPerChannel(t *testing.T) {
	draw := func(settings fontSettings) int {
		useSystemFonts(t, settings)
		offscreen, err := NewCPUOffscreen(160, 28, 1, platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(offscreen.Close)
		label := platform.TextLabel{Value: "Hamburgefonstiv", Width: 150, Height: 28, FontSize: 14, Color: platform.Color{Red: 0.1, Green: 0.1, Blue: 0.1, Alpha: 1}}
		if err := offscreen.Set(nil, []platform.TextLabel{label}); err != nil {
			t.Fatal(err)
		}
		pixels, err := offscreen.Image()
		if err != nil {
			t.Fatal(err)
		}
		colored, darkest := coloredInk(pixels)
		if darkest > 80 {
			t.Fatalf("darkest pixel %d, want a dark stem", darkest)
		}
		return colored
	}

	if colored := draw(fontSettings{Antialias: cairoAntialiasSubpixel, HintStyle: cairoHintSlight, SubpixelOrder: cairoSubpixelRGB}); colored < 20 {
		t.Errorf("LCD text on white has %d colored pixels, want colored edges", colored)
	}
	if colored := draw(fontSettings{Antialias: cairoAntialiasGray, HintStyle: cairoHintSlight}); colored != 0 {
		t.Errorf("grayscale text on white has %d colored pixels, want gray edges", colored)
	}
}

// A fixed-pitch line with runs of different colors keeps the color of each
// run: the LCD picture is one coverage, and the renderer carries the columns
// each run owns with its color, as the Vulkan path does with spans.
func TestCPUGridLCDLabelKeepsItsRunColors(t *testing.T) {
	useSystemFonts(t, fontSettings{Antialias: cairoAntialiasSubpixel, HintStyle: cairoHintSlight, SubpixelOrder: cairoSubpixelRGB})
	services, err := newTextServices()
	if err != nil {
		t.Fatal(err)
	}
	defer services.close()
	label := platform.TextLabel{
		Value: "AB", Width: 32, Height: 20, FontSize: 14, Mono: true, CellWidth: 8,
		Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
		Runs: []gfx.TextRun{
			{Column: 0, Bytes: 1, Cells: 1, Color: gfx.RGB(255, 0, 0)},
			{Column: 1, Bytes: 1, Cells: 1, Color: gfx.RGB(0, 0, 255)},
		},
	}
	image, ok, err := rasterizeLabel(services, label, 1, true)
	if err != nil || !ok {
		t.Fatalf("rasterize: ok=%v err=%v", ok, err)
	}
	if !image.lcd {
		t.Fatal("grid text was not rasterized as LCD coverage")
	}
	if len(image.spans) != 2 {
		t.Fatalf("%d spans, want one per run", len(image.spans))
	}
	first, second := image.spans[0], image.spans[1]
	if first.Color.Red != 1 || first.Color.Green != 0 || first.Color.Blue != 0 {
		t.Fatalf("first span color %+v, want red", first.Color)
	}
	if second.Color.Red != 0 || second.Color.Green != 0 || second.Color.Blue != 1 {
		t.Fatalf("second span color %+v, want blue", second.Color)
	}
	if first.From >= first.To || second.From >= second.To || first.To > second.From {
		t.Fatalf("span columns %d..%d and %d..%d are not the runs' own", first.From, first.To, second.From, second.To)
	}
}

// The CPU path draws the same LCD text as the Vulkan path: both rasterize the
// same coverage and blend it per channel, so their pixels are within the same
// few levels the shape parity test allows. It skips where the GPU has no
// dual-source blending or no Vulkan driver, which is exactly where the Vulkan
// path falls back to grayscale.
func TestCPULCDTextMatchesTheGPUPath(t *testing.T) {
	useSystemFonts(t, fontSettings{Antialias: cairoAntialiasSubpixel, HintStyle: cairoHintSlight, SubpixelOrder: cairoSubpixelRGB})
	const width, height = 180, 40
	clear := platform.Color{Red: 0.93, Green: 0.95, Blue: 0.98, Alpha: 1}
	labels := []platform.TextLabel{{
		Value: "Hamburgefonstiv 0123", Width: 170, Height: 30, FontSize: 14,
		Color: platform.Color{Red: 0.1, Green: 0.1, Blue: 0.1, Alpha: 1},
	}}

	gpu, err := NewOffscreen(width, height, 1, clear)
	if errors.Is(err, platform.ErrNoGraphics) {
		t.Skipf("no Vulkan driver: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	dualSource := gpu.renderer.dualSource
	if err := gpu.Set(nil, labels); err != nil {
		gpu.Close()
		t.Fatal(err)
	}
	gpuPixels, err := gpu.Image()
	gpu.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !dualSource {
		t.Skip("the GPU has no dual-source blending, so its LCD text is grayscale")
	}

	cpu, err := NewCPUOffscreen(width, height, 1, clear)
	if err != nil {
		t.Fatal(err)
	}
	if err := cpu.Set(nil, labels); err != nil {
		cpu.Close()
		t.Fatal(err)
	}
	cpuPixels, err := cpu.Image()
	cpu.Close()
	if err != nil {
		t.Fatal(err)
	}

	worst, over2, over8, total := 0, 0, 0, 0
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			g := gpuPixels.RGBAAt(x, y)
			c := cpuPixels.RGBAAt(x, y)
			for _, pair := range [][2]uint8{{c.R, g.R}, {c.G, g.G}, {c.B, g.B}, {c.A, g.A}} {
				diff := int(math.Abs(float64(int(pair[0]) - int(pair[1]))))
				total++
				if diff > 2 {
					over2++
				}
				if diff > 8 {
					over8++
				}
				worst = max(worst, diff)
			}
		}
	}
	t.Logf("LCD text parity: max channel difference %d; >2 levels %.3f%%; >8 levels %.3f%%",
		worst, 100*float64(over2)/float64(total), 100*float64(over8)/float64(total))
	if worst > 8 || float64(over2)/float64(total) > 0.001 || over8 > 0 {
		t.Fatalf("LCD text differs from the GPU path: worst %d, %.4f%% over 2 levels", worst, 100*float64(over2)/float64(total))
	}
}
