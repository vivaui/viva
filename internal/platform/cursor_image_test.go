package platform

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

func cursorImage(t *testing.T, width, height, hotX, hotY int, scale float32, bgra []byte) *ui.CursorImage {
	t.Helper()
	image, err := ui.NewCursorImage(width, height, hotX, hotY, scale, bgra)
	if err != nil {
		t.Fatal(err)
	}
	return image
}

// The native cursor APIs take color multiplied by alpha; a half-transparent white must not stay
// full white, or its edge would glow.
func TestCursorPixelsMultipliesColorByAlpha(t *testing.T) {
	image := cursorImage(t, 1, 1, 0, 0, 1, []byte{255, 255, 255, 128})
	width, height, _, _, bgra := CursorPixels(image, 1, true)
	if width != 1 || height != 1 || bgra[0] != 128 || bgra[1] != 128 || bgra[2] != 128 || bgra[3] != 128 {
		t.Fatalf("%dx%d %v", width, height, bgra)
	}
}

// A picture made for 1x shown on a 2x window covers twice the device pixels, with its hotspot on
// the same part of the picture, and a solid picture stays solid and exact.
func TestCursorPixelsFollowTheWindowScale(t *testing.T) {
	pixels := make([]byte, 4*4*4)
	for index := 0; index < len(pixels); index += 4 {
		copy(pixels[index:], []byte{10, 20, 30, 255})
	}
	image := cursorImage(t, 4, 4, 1, 2, 1, pixels)
	width, height, hotX, hotY, bgra := CursorPixels(image, 2, true)
	if width != 8 || height != 8 {
		t.Fatalf("size %dx%d, want 8x8", width, height)
	}
	if hotX != 3 || hotY != 5 {
		t.Fatalf("hotspot %d,%d, want 3,5", hotX, hotY)
	}
	for index := 0; index < len(bgra); index += 4 {
		if bgra[index] != 10 || bgra[index+1] != 20 || bgra[index+2] != 30 || bgra[index+3] != 255 {
			t.Fatalf("pixel %d = %v, want a solid color", index/4, bgra[index:index+4])
		}
	}
}

// A picture made for the screen's own pixels is not resampled at all.
func TestCursorPixelsKeepADevicePixelPictureExact(t *testing.T) {
	pixels := []byte{1, 2, 3, 255, 4, 5, 6, 255}
	image := cursorImage(t, 2, 1, 1, 0, 2, pixels)
	width, height, hotX, hotY, bgra := CursorPixels(image, 2, true)
	if width != 2 || height != 1 || hotX != 1 || hotY != 0 || string(bgra) != string(pixels) {
		t.Fatalf("%dx%d hot %d,%d %v", width, height, hotX, hotY, bgra)
	}
}

func TestNewCursorImageRefusesWhatNoSystemCouldShow(t *testing.T) {
	good := make([]byte, 16)
	for name, make := range map[string]func() (*ui.CursorImage, error){
		"empty":          func() (*ui.CursorImage, error) { return ui.NewCursorImage(0, 1, 0, 0, 1, nil) },
		"too large":      func() (*ui.CursorImage, error) { return ui.NewCursorImage(ui.MaxCursorImageSize+1, 1, 0, 0, 1, nil) },
		"hotspot out":    func() (*ui.CursorImage, error) { return ui.NewCursorImage(2, 2, 2, 0, 1, good) },
		"short pixels":   func() (*ui.CursorImage, error) { return ui.NewCursorImage(2, 2, 0, 0, 1, good[:12]) },
		"negative scale": func() (*ui.CursorImage, error) { return ui.NewCursorImage(2, 2, 0, 0, -1, good) },
	} {
		if _, err := make(); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
	if _, err := ui.NewCursorImage(2, 2, 1, 1, 0, good); err != nil {
		t.Errorf("a valid image was refused: %v", err)
	}
}

// Plain ARGB keeps the color a picture was made with, resampled or not: a half-transparent
// solid color stays that color at the new size.
func TestCursorPixelsWithoutPremultiplicationKeepTheColor(t *testing.T) {
	pixels := make([]byte, 2*2*4)
	for index := 0; index < len(pixels); index += 4 {
		copy(pixels[index:], []byte{200, 100, 50, 128})
	}
	image := cursorImage(t, 2, 2, 0, 0, 1, pixels)
	for _, windowScale := range []float32{1, 3} {
		_, _, _, _, bgra := CursorPixels(image, windowScale, false)
		for index := 0; index < len(bgra); index += 4 {
			if bgra[index] != 200 || bgra[index+1] != 100 || bgra[index+2] != 50 || bgra[index+3] != 128 {
				t.Fatalf("scale %v: pixel %d = %v, want 200 100 50 128", windowScale, index/4, bgra[index:index+4])
			}
		}
	}
}
