//go:build darwin

package headless

import (
	"math"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

// inkCentroid renders a label laid out at (x, y) and returns where the centre of its ink is,
// in device pixels, at 2x.
func inkCentroid(t *testing.T, x, y float32) (cx, cy float64) {
	t.Helper()
	config := app.Config{
		Title: "origin", Width: 160, Height: 40, ClearColor: app.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
		Paint: func(canvas *gfx.Canvas, size gfx.Size) {
			canvas.Label("Hamburgefonstiv", gfx.Rect{X: x, Y: y, Width: 120, Height: 20}, gfx.TextStyle{Size: 13, Color: gfx.Color{Alpha: 1}})
		},
	}
	session, err := Open(config, nil, Options{Scale: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	picture, err := session.Image()
	if err != nil {
		t.Skipf("no renderer: %v", err)
	}
	var sum, sx, sy float64
	bounds := picture.Bounds()
	for py := bounds.Min.Y; py < bounds.Max.Y; py++ {
		for px := bounds.Min.X; px < bounds.Max.X; px++ {
			r, _, _, _ := picture.At(px, py).RGBA()
			ink := 1 - float64(r)/0xFFFF
			sum += ink
			sx += ink * float64(px)
			sy += ink * float64(py)
		}
	}
	return sx / sum, sy / sum
}

// The macOS screenshot of a label laid out at a fraction of a logical pixel has its text there
// (platform.TextLabel.OriginX), not on the whole pixel the label's frame is rounded to: half a
// point is one device pixel at 2x, from the painter to the Metal pipeline.
func TestLabelAtHalfAPointIsDrawnHalfAPointOver(t *testing.T) {
	x0, y0 := inkCentroid(t, 10, 10)
	x1, y1 := inkCentroid(t, 10.5, 10.5)
	if dx := x1 - x0; math.Abs(dx-1) > 0.1 {
		t.Errorf("the text moved %.2f device pixels for half a point across, want 1", dx)
	}
	if dy := y1 - y0; math.Abs(dy-1) > 0.1 {
		t.Errorf("the text moved %.2f device pixels for half a point down, want 1", dy)
	}
}
