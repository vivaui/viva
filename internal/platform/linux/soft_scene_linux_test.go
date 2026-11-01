package linux

import (
	"bytes"
	"testing"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
	"github.com/vivaui/viva/ui"
)

func softTestRect(x, y, w, h float32, c scene.Color, layer uint16) render.Rectangle {
	return render.Rectangle{Bounds: scene.Rect{X: x, Y: y, Width: w, Height: h}, Color: c, Layer: layer, Radius: 3}
}

func softTestLabel(value string, x, y float32, layer uint16) platform.TextLabel {
	return platform.TextLabel{Value: value, X: x, Y: y, Width: 70, Height: 20, FontSize: 14, Color: platform.Color{Alpha: 1}, Layer: layer}
}

// Whatever sequence of frames a window receives, painting only the damage leaves the canvas as a
// scene built from the final content and painted whole.
func TestSoftSceneFramesPaintWhatAFullPaintWould(t *testing.T) {
	services, err := newTextServices()
	if err != nil {
		t.Skipf("no text services: %v", err)
	}
	clear := platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	red, blue := scene.Color{Red: 1, Alpha: 1}, scene.Color{Blue: 1, Alpha: 0.6}
	rects := []render.Rectangle{softTestRect(4, 4, 60, 30, red, 0), softTestRect(40, 20, 70, 40, blue, 1), softTestRect(10, 70, 100, 10, red, 0)}
	labels := []platform.TextLabel{softTestLabel("Hello", 8, 6, 0), softTestLabel("World", 30, 40, 1)}
	for _, scale := range []float32{1, 1.5} {
		live, err := newSoftScene(services, int(130*scale), int(90*scale), scale, clear, rects, labels)
		if err != nil {
			t.Fatal(err)
		}
		live.paint()
		wantRects, wantLabels := append([]render.Rectangle(nil), rects...), append([]platform.TextLabel(nil), labels...)
		step := func(name string, frame platform.Frame) {
			t.Helper()
			if frame.Update.Rebuild || frame.Update.Count > 0 {
				wantRects = append(wantRects[:0], frame.Rectangles...)
			}
			if frame.TextLabels != nil {
				wantLabels = append(wantLabels[:0], frame.TextLabels...)
			}
			if _, err := live.apply(frame); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			live.paint()
			whole, err := newSoftScene(services, live.canvas.Width, live.canvas.Height, scale, clear, wantRects, wantLabels)
			if err != nil {
				t.Fatal(err)
			}
			whole.paint()
			if !bytes.Equal(live.canvas.Pix, whole.canvas.Pix) {
				t.Fatalf("scale %g, after %s: canvas differs from a full paint", scale, name)
			}
		}
		moved := append([]render.Rectangle(nil), rects...)
		moved[1] = softTestRect(50, 24, 70, 40, blue, 1)
		step("one rectangle moved", platform.Frame{Update: render.Update{First: 1, Count: 1}, Rectangles: moved})
		grown := append(append([]render.Rectangle(nil), moved...), softTestRect(90, 5, 30, 30, red, 2))
		step("a rectangle appended", platform.Frame{Update: render.Update{First: 3, Count: 1}, Rectangles: grown})
		edited := []platform.TextLabel{softTestLabel("Hullo", 8, 6, 0), softTestLabel("World", 30, 40, 1)}
		step("a label edited", platform.Frame{TextLabels: edited})
		scrolled := []platform.TextLabel{softTestLabel("Hullo", 8, 6, 0), softTestLabel("World", 30, 47, 1)}
		step("a label scrolled", platform.Frame{TextLabels: scrolled})
		clipped := []platform.TextLabel{softTestLabel("Hullo", 8, 6, 0), softTestLabel("World", 30, 47, 1)}
		clipped[1].Clipped, clipped[1].Clip = true, ui.Rect{X: 30, Y: 47, Width: 20, Height: 8}
		step("a label clipped", platform.Frame{TextLabels: clipped})
		step("a label removed", platform.Frame{TextLabels: clipped[:1]})
		step("the scene rebuilt smaller", platform.Frame{Update: render.Update{Rebuild: true}, Rectangles: grown[:2]})
	}
}

func TestSoftSceneTypingPaintsOnlyTheLine(t *testing.T) {
	services, err := newTextServices()
	if err != nil {
		t.Skipf("no text services: %v", err)
	}
	clear := platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	labels := []platform.TextLabel{softTestLabel("Hello", 8, 6, 0), softTestLabel("Static", 8, 60, 0)}
	live, err := newSoftScene(services, 200, 100, 1, clear, nil, labels)
	if err != nil {
		t.Fatal(err)
	}
	if got := live.paint(); got != live.canvas.Bounds() {
		t.Fatalf("first paint covers %+v, want the whole canvas", got)
	}
	if got := live.paint(); got.Right > got.Left {
		t.Fatalf("a scene with nothing new painted %+v", got)
	}
	labels[0].Value = "Hello!"
	changed, err := live.apply(platform.Frame{TextLabels: labels})
	if err != nil || !changed {
		t.Fatalf("apply = %v, %v", changed, err)
	}
	got := live.paint()
	if got.Top > 6 || got.Bottom < 26 || got.Bottom > 40 || got.Left > 8 {
		t.Fatalf("typing repainted %+v, want about the first label's 8,6 70x20", got)
	}
}
