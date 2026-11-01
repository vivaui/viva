package platform

import "testing"

// A layout position half a device pixel either way snaps to the nearest pixel, and
// the remainder the painter took off is part of it.
func TestDevicePositionUsesTheLayoutOrigin(t *testing.T) {
	if got := DevicePosition(13, 0, 1.5); got != 20 { // 19.5 device pixels
		t.Errorf("13 at 1.5x: %d, want 20", got)
	}
	if got := DevicePosition(13, -0.4, 1.5); got != 19 { // 18.9
		t.Errorf("13 less 0.4 at 1.5x: %d, want 19", got)
	}
}

// Shrinking a bitmap keeps its average color, including where the ratio is not whole,
// and an even split of black and white comes out gray, not one of the two.
func TestResampleBGRAAveragesTheAreaUnderEachPixel(t *testing.T) {
	uniform := make([]byte, 7*5*4)
	for index := range uniform {
		uniform[index] = 200
	}
	for _, size := range [][2]int{{3, 2}, {4, 3}, {7, 5}, {1, 1}} {
		out := ResampleBGRA(uniform, 7*4, 7, 5, size[0], size[1])
		if len(out) != size[0]*size[1]*4 {
			t.Fatalf("size %v: %d bytes", size, len(out))
		}
		for index, value := range out {
			if value != 200 {
				t.Fatalf("size %v: byte %d is %d, want 200", size, index, value)
			}
		}
	}
	split := make([]byte, 4*1*4) // black, black, white, white
	for pixel := 2; pixel < 4; pixel++ {
		copy(split[pixel*4:], []byte{255, 255, 255, 255})
	}
	out := ResampleBGRA(split, 16, 4, 1, 1, 1)
	if out[0] != 128 && out[0] != 127 {
		t.Errorf("average of black and white is %d, want 127 or 128", out[0])
	}
}

func TestResampleBGRARefusesToEnlarge(t *testing.T) {
	if ResampleBGRA(make([]byte, 16), 8, 2, 2, 3, 2) != nil {
		t.Error("enlarging a bitmap returned pixels")
	}
}

// A clip that cuts the left of a label skips the columns it cuts, so the texture is
// the visible width; one that does not reaches the label keeps the whole texture.
func TestLabelWindowSkipsTheColumnsTheClipCuts(t *testing.T) {
	cases := []struct {
		name                      string
		origin, full, left, right int
		skip, width               int
		ok                        bool
	}{
		{"clip outside", 100, 200, 90, 320, 0, 200, true},
		{"scrolled", 10, 1000, 210, 1010, 200, 800, true},
		{"right edge inside", 10, 100, 0, 60, 0, 100, true},
		{"nothing visible", 100, 50, 200, 160, 0, 0, false},
		{"right clamped", 10, 100, 50, 500, 40, 60, true},
		{"empty label", 10, 0, 0, 100, 0, 0, false},
	}
	for _, c := range cases {
		skip, width, ok := LabelWindow(c.origin, c.full, c.left, c.right)
		if skip != c.skip || width != c.width || ok != c.ok {
			t.Errorf("%s: LabelWindow(%d, %d, %d, %d) = %d, %d, %v; want %d, %d, %v",
				c.name, c.origin, c.full, c.left, c.right, skip, width, ok, c.skip, c.width, c.ok)
		}
	}
}

// An image drawn small and then drawn large again needs a larger texture, or it blurs; one
// drawn a little smaller keeps its texture, and one shrunk past half rebuilds so it does not alias.
func TestBitmapTextureFollowsTheDrawnSize(t *testing.T) {
	label := TextLabel{Width: 16, Height: 16, BitmapWidth: 64, BitmapHeight: 64}
	width, height := BitmapTextureExtent(label, 1)
	if width != 16 || height != 16 {
		t.Fatalf("drawn at 16 the texture is %dx%d", width, height)
	}
	label.Width, label.Height = 64, 64
	if !BitmapTextureStale(label, 1, width, height) {
		t.Error("an image drawn larger kept its smaller texture")
	}
	label.Width, label.Height = 14, 14
	if BitmapTextureStale(label, 1, 16, 16) {
		t.Error("a slightly smaller image rebuilt its texture")
	}
	label.Width, label.Height = 6, 6
	if !BitmapTextureStale(label, 1, 16, 16) {
		t.Error("an image shrunk past half kept its oversized texture")
	}
	label.Width, label.Height = 200, 200
	if width, height := BitmapTextureExtent(label, 1); width != 64 || height != 64 {
		t.Errorf("an image drawn larger than stored is %dx%d, want the stored size", width, height)
	}
}
