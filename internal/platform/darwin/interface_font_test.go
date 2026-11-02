//go:build darwin && vivahead

package darwin

import (
	"bytes"
	"testing"

	"github.com/vivaui/viva/internal/platform"
)

// The renderer keeps the texture of a label whose content did not change: naming another
// interface family must draw the ones whose text, size and box did not change again.
// The generation the painter stamps on them says so.
func TestALabelIsDrawnAgainWhenTheInterfaceFamilyChanges(t *testing.T) {
	families := MonospaceFamilies()
	if len(families) == 0 {
		t.Skip("no fixed-pitch family installed")
	}
	engine := SharedTextEngine()
	engine.SetFontFamily("")
	t.Cleanup(func() { engine.SetFontFamily("") })
	offscreen, err := NewOffscreen(240, 40, 1, Color{Alpha: 1})
	if err != nil {
		t.Skipf("no Metal device: %v", err)
	}
	defer offscreen.Close()
	labels := []platform.TextLabel{{
		Value: "Hamburgefonstiv 0123", X: 4, Y: 4, Width: 220, Height: 28, FontSize: 16,
		Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}, FontGeneration: engine.FontGeneration(),
	}}
	render := func() []byte {
		if err := offscreen.Set(nil, labels, nil); err != nil {
			t.Fatal(err)
		}
		pixels, _, _, err := offscreen.Render()
		if err != nil {
			t.Fatal(err)
		}
		return bytes.Clone(pixels)
	}
	before := render()
	engine.SetFontFamily(families[0])
	labels[0].FontGeneration = engine.FontGeneration()
	if bytes.Equal(before, render()) {
		t.Fatalf("the image is the same in %q as in the system font: the label kept its old texture", families[0])
	}
}
