package components

import (
	"errors"
	"math"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/internal/platform"
)

// Reproduce the real form's position and 125% Windows display scale. Geometry
// near the origin can pass while fractional-size snapping shifts this glyph.
func TestCheckboxCheckIsCenteredInWindowsPixels(t *testing.T) {
	theme := gfx.LightTheme()
	session, err := headless.Open(app.Config{
		Title: "Checkbox centering", Width: 512, Height: 640,
		ClearColor: app.Color{Red: 1, Alpha: 1},
		Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
			BeginCheckbox(canvas, theme, gfx.Item(1), gfx.Rect{X: 458, Y: 602}, true).Paint()
		},
	}, nil, headless.Options{Scale: 1.25})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	pixels, err := session.Image()
	if errors.Is(err, headless.ErrNoRenderer) || errors.Is(err, platform.ErrNoGraphics) {
		t.Skipf("no offscreen renderer: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	// The 16-point box occupies device pixels x=573..592, y=753..772.
	// The clear color is red, so only the white check has green/blue coverage.
	least, greatest := 800, -1
	for y := 753; y < 773; y++ {
		for x := 573; x < 593; x++ {
			color := pixels.RGBAAt(x, y)
			if color.G > 180 && color.B > 180 {
				least, greatest = min(least, y), max(greatest, y)
			}
		}
	}
	if greatest < 0 {
		t.Fatal("no visible check pixels")
	}
	if center := float64(least+greatest+1) / 2; math.Abs(center-763) > 0.5 {
		t.Fatalf("visible check spans rows %d..%d, center %g; checkbox center is 763", least, greatest, center)
	}
}
