package linux

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

// BenchmarkRasterizeSentence is one proportional sentence drawn word by word, grayscale and
// with LCD antialiasing (an RGB surface and the per-channel coverage).
func BenchmarkRasterizeSentence(benchmark *testing.B) {
	label := platform.TextLabel{Value: "Connect to host-01 as ada, port 22", Width: 400, Height: 28, FontSize: 13, Color: platform.Color{Alpha: 1}}
	for _, test := range []struct {
		name     string
		settings fontSettings
		lcd      bool
	}{
		{"grayscale", fontSettings{Antialias: cairoAntialiasGray, HintStyle: cairoHintSlight}, false},
		{"lcd", fontSettings{Antialias: cairoAntialiasSubpixel, HintStyle: cairoHintSlight, SubpixelOrder: cairoSubpixelRGB}, true},
	} {
		benchmark.Run(test.name, func(benchmark *testing.B) {
			useSystemFonts(benchmark, test.settings)
			services, err := newTextServices()
			if err != nil {
				benchmark.Fatal(err)
			}
			defer services.close()
			benchmark.ReportAllocs()
			benchmark.ResetTimer()
			for range benchmark.N {
				if _, lcd, err := services.rasterize(label, 1, 400, 28, test.lcd); err != nil || lcd != test.lcd {
					benchmark.Fatal(err, lcd)
				}
			}
		})
	}
}

// BenchmarkGridRasterize is one terminal line of 100 cells in four colors, as grayscale and as
// LCD text (an RGB surface, the coverage of each channel, a span per color).
func BenchmarkGridRasterize(benchmark *testing.B) {
	colors := []gfx.Color{{Red: 1, Green: 1, Blue: 1, Alpha: 1}, {Green: 1, Alpha: 1}, {Red: 1, Alpha: 1}, {Red: 0.2, Green: 0.4, Blue: 1, Alpha: 1}}
	value := make([]byte, 100)
	for index := range value {
		value[index] = byte('a' + index%26)
	}
	label := platform.TextLabel{Value: string(value), Mono: true, CellWidth: 8, Width: 800, Height: 20, FontSize: 14, Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}}
	for index, color := range colors {
		label.Runs = append(label.Runs, gfx.TextRun{Column: int32(index * 25), Bytes: 25, Cells: 25, Color: color})
	}
	for _, test := range []struct {
		name     string
		settings fontSettings
		lcd      bool
	}{
		{"grayscale", fontSettings{Antialias: cairoAntialiasGray, HintStyle: cairoHintSlight}, false},
		{"lcd", fontSettings{Antialias: cairoAntialiasSubpixel, HintStyle: cairoHintSlight, SubpixelOrder: cairoSubpixelRGB}, true},
	} {
		benchmark.Run(test.name, func(benchmark *testing.B) {
			useSystemFonts(benchmark, test.settings)
			services, err := newTextServices()
			if err != nil {
				benchmark.Fatal(err)
			}
			defer services.close()
			benchmark.ReportAllocs()
			benchmark.ResetTimer()
			for range benchmark.N {
				if _, _, err := services.rasterize(label, 1, 800, 20, test.lcd); err != nil {
					benchmark.Fatal(err)
				}
			}
		})
	}
}
