//go:build darwin

package darwin

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

// AppKit takes the picture as a cursor of the size it was made for, at the screen's density:
// a picture at 2x is half as many points, and neither it nor a replacement is refused.
func TestAnApplicationPictureBecomesACursor(t *testing.T) {
	for _, test := range []struct {
		name          string
		width, height int
		hotX, hotY    int
		scale         float32
	}{
		{"1x", 24, 32, 3, 4, 1},
		{"2x", 48, 64, 6, 8, 2},
		{"a one pixel picture", 1, 1, 0, 0, 1},
	} {
		pixels := make([]byte, test.width*test.height*4)
		for index := 0; index < len(pixels); index += 4 {
			copy(pixels[index:], []byte{40, 120, 220, 200})
		}
		image, err := ui.NewCursorImage(test.width, test.height, test.hotX, test.hotY, test.scale, pixels)
		if err != nil {
			t.Fatal(err)
		}
		if err := SetCursorImage(image); err != nil {
			t.Errorf("%s: %v", test.name, err)
		}
	}
	t.Cleanup(func() { _ = SetCursorShape(0) })
}
