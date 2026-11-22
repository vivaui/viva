//go:build darwin && vivahead && !ios

package darwin

import (
	"fmt"
	"math"
	"testing"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/textsharp"
)

// A label is as sharp as the same string drawn by CoreText in one call, which is what any
// macOS application does: the rasterizer does not resample the glyphs. An earlier design
// scaled every word to its layout width and squeezed some; this test measures the line the
// platform draws now.
func TestLabelIsAsSharpAsTheWholeLineDrawnNatively(t *testing.T) {
	engine := SharedTextEngine()
	for _, scale := range []float64{1, 2} {
		for _, specimen := range textsharp.Specimens {
			for _, size := range textsharp.Sizes {
				for _, weight := range textsharp.Weights {
					width := int(math.Ceil(float64(engine.Width(specimen.Text, size, weight)))) + 40
					height := 28
					offscreen, err := NewOffscreen(width, height, scale, platform.Color{Alpha: 1})
					if err != nil {
						t.Skipf("no Metal device: %v", err)
					}
					label := TextLabel{Value: specimen.Text, Width: float32(width), Height: float32(height), FontSize: size, Weight: weight,
						Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}}
					if err := offscreen.Set(nil, []TextLabel{label}, nil); err != nil {
						t.Fatal(err)
					}
					pixels, w, h, err := offscreen.Render()
					offscreen.Close()
					if err != nil {
						t.Fatal(err)
					}
					reference, err := ReferenceText(specimen.Text, float64(size), float64(weight), scale, w, h)
					if err != nil {
						t.Fatal(err)
					}
					// The scene is white text on opaque black: its green channel is the coverage.
					produced := textsharp.Measure(pixels[1:], w, h, 4, w*4)
					native := textsharp.Measure(reference[3:], w, h, 4, w*4)
					name := specimen.Name
					t.Log(textsharp.Report(name+" "+sizeName(size, weight, scale), produced, native))
					if produced.Edge < 0.97*native.Edge || produced.Solid < 0.85*native.Solid {
						t.Errorf("%s: a label is blurrier than the line drawn whole: %s", name, textsharp.Report("", produced, native))
					}
				}
			}
		}
	}
}

func sizeName(size, weight float32, scale float64) string {
	return fmt.Sprintf("%gpx w%g @%gx", size, weight, scale)
}
