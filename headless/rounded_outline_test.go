package headless

import (
	"math"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

func TestRoundedOutlineTranslucentCoverage(t *testing.T) {
	if !rendererAvailable {
		t.Skip("requires native offscreen renderer")
	}
	for _, scale := range []float64{1, 1.25, 1.5, 2, 3} {
		session, err := Open(app.Config{Title: "outline", Width: 96, Height: 56, ClearColor: app.Color{Alpha: 1},
			Paint: func(canvas *ui.Canvas, size ui.Size) {
				canvas.RoundedOutline(ui.Rect{X: 8, Y: 8, Width: 80, Height: 40}, 8, 4, ui.Color{Red: 1, Alpha: .5})
			}}, nil, Options{Scale: scale})
		if err != nil {
			t.Fatal(err)
		}
		pixels, err := session.Image()
		session.Close()
		if err != nil {
			t.Fatal(err)
		}
		distance := func(x, y, halfWidth, halfHeight, radius float64) float64 {
			x, y = math.Abs(x-48)-halfWidth+radius, math.Abs(y-28)-halfHeight+radius
			return math.Hypot(math.Max(x, 0), math.Max(y, 0)) + math.Min(math.Max(x, y), 0) - radius
		}
		for y := 0; y < pixels.Bounds().Dy(); y++ {
			for x := 0; x < pixels.Bounds().Dx(); x++ {
				red := pixels.RGBAAt(x, y).R
				px, py := (float64(x)+.5)/scale, (float64(y)+.5)/scale
				outer, inner := distance(px, py, 40, 20, 8), distance(px, py, 36, 16, 4)
				if red > 132 {
					t.Fatalf("scale %g: overlapping alpha at %d,%d: %d", scale, x, y, red)
				}
				if outer < -1/scale && inner > 1/scale && red < 124 {
					t.Fatalf("scale %g: gap inside border at %d,%d: %d", scale, x, y, red)
				}
				if (outer > 1/scale || inner < -1/scale) && red > 4 {
					t.Fatalf("scale %g: paint outside border at %d,%d: %d", scale, x, y, red)
				}
			}
		}
	}
}
