//go:build darwin || windows || linux

package app

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

// The display scale a platform reports with a resize is what the application paints
// for: icons and strokes are rounded to whole pixels of it. A platform that does not
// report one leaves the canvas as it was.
func TestResizeTellsTheCanvasTheDisplayScale(t *testing.T) {
	var painted float32
	driver, err := NewDriver(Config{Width: 200, Height: 100, Paint: func(canvas *gfx.Canvas, _ gfx.Size) { painted = canvas.PixelScale() }}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	if _, err := driver.painter.resize(platform.Size{Width: 200, Height: 100, Scale: 1.5}); err != nil {
		t.Fatal(err)
	}
	if painted != 1.5 {
		t.Fatalf("painted for scale %v after a resize at 1.5x", painted)
	}
	if _, err := driver.painter.resize(platform.Size{Width: 220, Height: 100}); err != nil {
		t.Fatal(err)
	}
	if painted != 1.5 {
		t.Fatalf("painted for scale %v after a resize that reported none", painted)
	}
}
