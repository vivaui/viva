//go:build !android

package linux

import (
	"fmt"
	"math"
	"testing"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/textmetrics"
	"github.com/vivaui/viva/internal/textsharp"
)

// referenceText draws value the way a Pango application does, as one layout for the whole
// string, in the font and on the baseline a label has, and returns its BGRA bitmap.
func (services *textServices) referenceText(value string, size, weight, scale float32, width, height int) ([]byte, error) {
	pixels := make([]byte, width*height*4)
	surface := services.imageSurfaceCreateForData(&pixels[0], 0, int32(width), int32(height), int32(width*4))
	if surface == 0 || services.surfaceStatus(surface) != 0 {
		return nil, fmt.Errorf("no Cairo surface")
	}
	defer services.surfaceDestroy(surface)
	context := services.cairoCreate(surface)
	if context == 0 || services.cairoStatus(context) != 0 {
		return nil, fmt.Errorf("no Cairo context")
	}
	defer services.cairoDestroy(context)
	services.cairoSetSourceRGBA(context, 1, 1, 1, 1)
	metrics, err := services.configureLayout(value, "sans", size*scale, weight, false)
	if err != nil {
		return nil, err
	}
	services.cairoMoveTo(context, 0, float64(platform.LabelBaseline(height, float64(size*scale)))-float64(metrics.Ascent))
	services.updateLayout(context, services.layout)
	services.showLayout(context, services.layout)
	services.surfaceFlush(surface)
	return pixels, nil
}

// A label is the engine's layout drawn in one call, as any GTK or Qt application
// draws a string: its glyphs are not resampled, so it is as sharp as the string drawn by
// Pango directly.
func TestLabelsAreAsSharpAsTheStringDrawnByPangoInOneCall(t *testing.T) {
	services, err := newTextServices()
	if err != nil {
		t.Fatal(err)
	}
	defer services.close()
	for _, scale := range []float32{1, 2} {
		for _, specimen := range textsharp.Specimens {
			for _, size := range textsharp.Sizes {
				for _, weight := range textsharp.Weights {
					width := int(math.Ceil(float64(textmetrics.Width(specimen.Text, size, weight)*scale))) + 40
					height := int(28 * scale)
					label := platform.TextLabel{Value: specimen.Text, Width: float32(width) / scale, Height: float32(height) / scale, FontSize: size, Weight: weight,
						Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}}
					drawn, _, err := services.rasterize(label, scale, width, height, false)
					if err != nil {
						t.Fatal(err)
					}
					reference, err := services.referenceText(specimen.Text, size, weight, scale, width, height)
					if err != nil {
						t.Fatal(err)
					}
					produced := textsharp.Measure(drawn[3:], width, height, 4, width*4)
					native := textsharp.Measure(reference[3:], width, height, 4, width*4)
					name := fmt.Sprintf("%s %gpx w%g @%gx", specimen.Name, size, weight, scale)
					t.Log(textsharp.Report(name, produced, native))
					if produced.Edge < 0.97*native.Edge || produced.Solid < 0.85*native.Solid {
						t.Errorf("%s: the label is blurrier than the string drawn directly: %s", name, textsharp.Report("", produced, native))
					}
				}
			}
		}
	}
}
