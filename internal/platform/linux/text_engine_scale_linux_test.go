//go:build !android

package linux

import (
	"math"
	"testing"
)

// Layout is measured at the logical size and drawn at the device size, so a line must be
// exactly as much wider at a scale as the scale: with hinted metrics or rounded glyph
// positions it changed by up to 17 percent between 1x and 1.5x, and a window moved to
// another monitor would have laid out differently.
func TestALineIsAsWideAsTheScaleTimesItsLogicalWidth(t *testing.T) {
	engine := newTestEngine(t)
	engine.mu.Lock()
	defer engine.mu.Unlock()
	for _, value := range []string{"Connect", "192.168.1.1", "iiiiiiiiii llllllllll", "Terminal 终端 2213123", "The quick brown fox jumps over the lazy dog"} {
		for _, size := range []float32{12, 13, 14, 16} {
			logical, err := engine.lineLocked(value, size, 0)
			if err != nil {
				t.Fatal(err)
			}
			for _, scale := range []float32{1.25, 1.5, 1.75, 2, 3} {
				device, err := engine.lineLocked(value, size*scale, 0)
				if err != nil {
					t.Fatal(err)
				}
				if drift := math.Abs(float64(device.width/scale - logical.width)); drift > 0.05 {
					t.Errorf("%q %gpx at %gx is %v logical pixels wide, %v at 1x", value, size, scale, device.width/scale, logical.width)
				}
			}
		}
	}
}
