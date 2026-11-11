package app

import (
	"runtime"
	"testing"
)

func TestConfigRejectsAWindowIconWhosePixelsDoNotMatchItsSize(t *testing.T) {
	good := WindowIcon{Size: 2, RGBA: make([]byte, 16)}
	if _, err := (Config{Icon: good}).normalize(); err != nil {
		t.Fatalf("a well-formed icon was rejected: %v", err)
	}
	if _, err := (Config{}).normalize(); err != nil {
		t.Fatalf("a window without an icon was rejected: %v", err)
	}
	for name, icon := range map[string]WindowIcon{
		"short pixels": {Size: 2, RGBA: make([]byte, 15)},
		"no size":      {RGBA: make([]byte, 16)},
		"too large":    {Size: 1025, RGBA: make([]byte, 1025*1025*4)},
	} {
		if _, err := (Config{Icon: icon}).normalize(); err == nil {
			t.Errorf("%s: the icon was accepted", name)
		}
	}
}

func TestStatusIconFitsWhatThePlatformsStatusAreaExpects(t *testing.T) {
	// A dark mark on white.
	pixels := make([]byte, 64*64*4)
	for index := 0; index < len(pixels); index += 4 {
		pixels[index], pixels[index+1], pixels[index+2], pixels[index+3] = 255, 255, 255, 255
	}
	for y := 20; y < 44; y++ {
		for x := 20; x < 44; x++ {
			copy(pixels[(y*64+x)*4:], []byte{10, 10, 10, 255})
		}
	}
	status := WindowIcon{Size: 64, RGBA: pixels}.StatusIcon(16)
	if status.Size != 16 || len(status.RGBA) != 16*16*4 {
		t.Fatalf("got size %d with %d bytes", status.Size, len(status.RGBA))
	}
	middle := status.RGBA[(8*16+8)*4:]
	if runtime.GOOS == "darwin" {
		if !status.Template || middle[3] < 240 || middle[0] != 0 {
			t.Errorf("on macOS the icon must be a black template of the mark: template %v, middle %v", status.Template, middle[:4])
		}
		return
	}
	if status.Template || status.RGBA[3] != 0 || middle[3] != 255 {
		t.Errorf("elsewhere it is the colored tile: template %v, corner alpha %d, middle %v", status.Template, status.RGBA[3], middle[:4])
	}
	// The status area draws the icon tiny, so the mark fills it: the same canvas
	// margin that suits a window's tile would leave the mark a size smaller.
	tile := WindowIcon{Size: 64, RGBA: pixels}.platform().Tile(16, 1)
	if got, canvas := darkPixels(status.RGBA), darkPixels(tile.RGBA); got <= canvas {
		t.Errorf("the status icon holds %d pixels of the mark, the window tile %d: the margin was not spent", got, canvas)
	}
}

// darkPixels counts the opaque pixels of a picture's dark mark.
func darkPixels(pixels []byte) int {
	count := 0
	for index := 0; index < len(pixels); index += 4 {
		if pixels[index+3] > 128 && pixels[index] < 64 {
			count++
		}
	}
	return count
}
