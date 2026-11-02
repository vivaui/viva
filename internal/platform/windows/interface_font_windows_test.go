package windows

import (
	"bytes"
	"testing"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

func interfaceFontLabel(generation uint32) platform.TextLabel {
	return platform.TextLabel{
		Value: "Hamburgefonstiv 0123", X: 4, Y: 4, Width: 220, Height: 28, FontSize: 16,
		Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}, FontGeneration: generation,
	}
}

// The labels of a window are textures: naming another interface family must draw the
// ones whose text, size and box did not change again, or the window keeps the old
// glyphs under the new layout. The generation the painter stamps on them says so.
func TestALabelIsDrawnAgainWhenTheInterfaceFamilyChanges(t *testing.T) {
	engine, err := theTextEngine()
	if err != nil {
		t.Skipf("no DirectWrite: %v", err)
	}
	families := MonospaceFamilies()
	if len(families) == 0 {
		t.Skip("no fixed-pitch family installed")
	}
	engine.SetFontFamily("")
	t.Cleanup(func() { engine.SetFontFamily("") })
	var follower ui.FontFamilyAware = engine

	offscreen := newOffscreen(t, 240, 40, 1)
	before := renderScene(t, offscreen, nil, []platform.TextLabel{interfaceFontLabel(follower.FontGeneration())})

	follower.SetFontFamily(families[0])
	after := renderScene(t, offscreen, nil, []platform.TextLabel{interfaceFontLabel(follower.FontGeneration())})
	if bytes.Equal(before.Pix, after.Pix) {
		t.Fatalf("the image is the same in %q as in Segoe UI: the label kept its old texture", families[0])
	}
}
