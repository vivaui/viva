package linux

import (
	"bytes"
	"testing"

	"github.com/vivaui/viva/internal/platform"
)

func interfaceFontLabel(generation uint32) platform.TextLabel {
	return platform.TextLabel{
		Value: "Hamburgefonstiv 0123", X: 4, Y: 4, Width: 220, Height: 28, FontSize: 16,
		Color: platform.Color{Red: 0.5, Green: 0.5, Blue: 0.5, Alpha: 1}, FontGeneration: generation,
	}
}

// anotherInterfaceFamily names a fixed-pitch family, which shapes unlike the desktop's
// interface font, and restores the interface family when the test ends.
func anotherInterfaceFamily(t *testing.T) (*textEngine, string) {
	t.Helper()
	engine := sharedTextEngine()
	if engine == nil {
		t.Skip("no Pango on this machine")
	}
	families := MonospaceFamilies()
	if len(families) == 0 {
		t.Skip("no fixed-pitch family installed")
	}
	engine.SetFontFamily("")
	t.Cleanup(func() { engine.SetFontFamily("") })
	return engine, families[0]
}

// The labels of a window are pictures: naming another interface family must draw the
// ones whose text, size and box did not change again, or the window keeps the old
// glyphs under the new layout. The generation the painter stamps on them says so.
func TestASoftSceneDrawsALabelAgainWhenTheInterfaceFamilyChanges(t *testing.T) {
	engine, family := anotherInterfaceFamily(t)
	services, err := newTextServices()
	if err != nil {
		t.Skipf("no text services: %v", err)
	}
	clear := platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	live, err := newSoftScene(services, 240, 40, 1, clear, nil, []platform.TextLabel{interfaceFontLabel(engine.FontGeneration())})
	if err != nil {
		t.Fatal(err)
	}
	live.paint()
	before := bytes.Clone(live.canvas.Pix)

	engine.SetFontFamily(family)
	changed, err := live.apply(platform.Frame{TextLabels: []platform.TextLabel{interfaceFontLabel(engine.FontGeneration())}})
	if err != nil || !changed {
		t.Fatalf("a frame with the same text and a new font generation: changed %v, err %v", changed, err)
	}
	live.paint()
	if bytes.Equal(before, live.canvas.Pix) {
		t.Fatalf("the canvas is the same in %q as in the desktop's font: the label kept its old picture", family)
	}
}

func TestAVulkanSceneDrawsALabelAgainWhenTheInterfaceFamilyChanges(t *testing.T) {
	engine, family := anotherInterfaceFamily(t)
	offscreen := newTestOffscreen(t, 240, 40)
	before := renderScene(t, offscreen, nil, []platform.TextLabel{interfaceFontLabel(engine.FontGeneration())})

	engine.SetFontFamily(family)
	after := renderScene(t, offscreen, nil, []platform.TextLabel{interfaceFontLabel(engine.FontGeneration())})
	if bytes.Equal(before.Pix, after.Pix) {
		t.Fatalf("the image is the same in %q as in the desktop's font: the label kept its old texture", family)
	}
}
