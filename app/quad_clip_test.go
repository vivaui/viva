package app

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestQuadClippingPreservesBothBackendPlacements(t *testing.T) {
	inside := gfx.Rect{X: 10, Y: 20, Width: 60, Height: 20}
	outside := gfx.Rect{X: 120, Y: 20, Width: 60, Height: 20}
	clip := gfx.Rect{Width: 100, Height: 100}
	white := gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	for _, test := range []struct {
		name             string
		box, destination gfx.Rect
		visible, cut     bool
	}{
		{"fallback outside", outside, inside, true, true},
		{"fallback inside", inside, outside, true, true},
		{"both inside", inside, inside, true, false},
		{"both outside", outside, outside, false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			var canvas gfx.Canvas
			canvas.Clip(clip)
			bounds := test.destination
			canvas.LabelQuad("visible", test.box, gfx.Quad{
				Points: [4]gfx.Point{{X: bounds.X, Y: bounds.Y}, {X: bounds.X + bounds.Width, Y: bounds.Y},
					{X: bounds.X, Y: bounds.Y + bounds.Height}, {X: bounds.X + bounds.Width, Y: bounds.Y + bounds.Height}},
				Tint: [4]gfx.Color{white, white, white, white},
			}, gfx.TextStyle{Size: 14, Color: white})
			painter := newTestPainter(t, &canvas, Config{Title: "quad clip", Width: 200, Height: 100})
			if (len(painter.labels) != 0) != test.visible {
				t.Fatalf("visible=%t, labels=%+v", test.visible, painter.labels)
			}
			if test.visible && (painter.labels[0].Clipped != test.cut || test.cut && painter.labels[0].Clip != clip) {
				t.Fatalf("clip cut=%t, label=%+v", test.cut, painter.labels[0])
			}
		})
	}
}
