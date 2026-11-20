package platform

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

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

// A label texture is held to the device limit across and to the pixel budget overall, so a
// tall label gets a narrower texture, and one taller than the limit gets none.
func TestLabelTextureWidthKeepsTheTextureBounded(t *testing.T) {
	cases := []struct{ height, width int }{
		{0, 0},
		{-3, 0},
		{24, LabelTextureLimit},
		{1024, LabelTextureLimit},
		{2048, LabelTexturePixels / 2048},
		{LabelTextureLimit, LabelTexturePixels / LabelTextureLimit},
		{LabelTextureLimit + 1, 0},
	}
	for _, c := range cases {
		if got := LabelTextureWidth(c.height); got != c.width {
			t.Errorf("LabelTextureWidth(%d) = %d, want %d", c.height, got, c.width)
		}
	}
	if !LabelTextureFits(LabelTextureLimit, 24) || LabelTextureFits(LabelTextureLimit+1, 24) || LabelTextureFits(0, 24) {
		t.Error("a short label's texture is not bounded by the device limit")
	}
	if LabelTextureFits(LabelTexturePixels/2048+1, 2048) {
		t.Error("a tall label's texture is not bounded by the pixel budget")
	}
	// LabelTextureFits tests without a division what LabelTextureWidth bounds, at each edge.
	for _, height := range []int{-1, 0, 1, 24, 1023, 1024, 1025, 2047, 2048, 3000, LabelTextureLimit - 1, LabelTextureLimit, LabelTextureLimit + 1} {
		limit := LabelTextureWidth(height)
		for _, width := range []int{-1, 0, 1, limit - 1, limit, limit + 1, LabelTextureLimit, LabelTextureLimit + 1} {
			if got, want := LabelTextureFits(width, height), width > 0 && width <= limit; got != want {
				t.Errorf("LabelTextureFits(%d, %d) = %v; LabelTextureWidth is %d", width, height, got, limit)
			}
		}
	}
}

// The columns a clip shows are counted from the label's left edge and cut to the label.
func TestLabelVisibleColumnsCutsTheClipToTheLabel(t *testing.T) {
	cases := []struct {
		name                      string
		origin, full, left, right int
		first, last               int
		ok                        bool
	}{
		{"label inside", 100, 200, 0, 1000, 0, 200, true},
		{"label starts left of the clip", -5000, 40000, 0, 1920, 5000, 6920, true},
		{"label ends inside the clip", -39000, 40000, 0, 1920, 39000, 40000, true},
		{"clip left of the label", 2000, 100, 0, 1920, 100, 100, false},
		{"clip right of the label", -500, 100, 0, 1920, 100, 100, false},
	}
	for _, c := range cases {
		first, last, ok := LabelVisibleColumns(c.origin, c.full, c.left, c.right)
		if ok != c.ok || ok && (first != c.first || last != c.last) {
			t.Errorf("%s: LabelVisibleColumns(%d, %d, %d, %d) = %d, %d, %v; want %d, %d, %v",
				c.name, c.origin, c.full, c.left, c.right, first, last, ok, c.first, c.last, c.ok)
		}
	}
}

// A line wider than a texture holds is rasterized around what can be seen, with a margin of
// the visible width on each side, shifted inward at the ends of the line; a texture that
// still holds what can be seen is kept, and one that does not is replaced.
func TestWideLabelWindowKeepsTheVisibleColumnsWithAMargin(t *testing.T) {
	const full, limit = 60000, LabelTextureLimit
	cases := []struct {
		name                string
		left, right         int
		haveSkip, haveWidth int
		skip, width         int
	}{
		{"start of the line", 0, 2000, 0, 0, 0, 6000},
		{"middle of the line", 20000, 22000, 0, 0, 18000, 6000},
		{"end of the line", 58000, 60000, 0, 0, 54000, 6000},
		{"wide span", 10000, 20000, 0, 0, 10000 - 3192, 10000 + 2*3192},
		{"span wider than a texture", 1000, 1000 + limit + 10, 0, 0, 1000, limit},
		{"scroll inside the texture", 19000, 21000, 18000, 6000, 18000, 6000},
		{"scroll past the texture", 23000, 25000, 18000, 6000, 21000, 6000},
		{"texture past the end of the line", 0, 2000, 59000, 6000, 0, 6000},
		{"texture wider than the limit", 0, 2000, 0, limit + 1, 0, 6000},
	}
	for _, c := range cases {
		skip, width := WideLabelWindow(full, limit, c.left, c.right, c.haveSkip, c.haveWidth)
		if skip != c.skip || width != c.width {
			t.Errorf("%s: WideLabelWindow(%d, %d, %d, %d, %d, %d) = %d, %d; want %d, %d",
				c.name, full, limit, c.left, c.right, c.haveSkip, c.haveWidth, skip, width, c.skip, c.width)
			continue
		}
		if width > limit || skip < 0 || skip+width > full || skip > c.left || skip+width < min(c.right, skip+limit) {
			t.Errorf("%s: window %d+%d does not hold the visible columns %d..%d within the limit", c.name, skip, width, c.left, c.right)
		}
	}
}

// A text label wider than a texture is windowed to the columns the viewport shows, cut to
// its clip; one the viewport does not show, or too tall for any texture, is not drawn, and
// one a texture holds whole is left to its backend.
func TestWideTextLabelWindowFollowsTheViewportAndTheClip(t *testing.T) {
	const scale, full, height, viewport = 1.5, 49164, 27, 600
	label := TextLabel{X: -20000} // 30000 device columns off the left of the viewport
	cases := []struct {
		name                string
		label               TextLabel
		full, height        int
		haveSkip, haveWidth int
		skip, width         int
		wide, ok            bool
	}{
		{"viewport", label, full, height, 0, 0, 29400, 1800, true, true},
		{"clip", TextLabel{X: -20000, Clipped: true, Clip: gfx.Rect{X: 100, Width: 200}}, full, height, 0, 0, 29850, 900, true, true},
		{"kept texture", label, full, height, 29000, 2000, 29000, 2000, true, true},
		{"scrolled out", TextLabel{X: -40000}, full, height, 0, 0, 0, 0, true, false},
		{"too tall", label, full, LabelTextureLimit + 1, 0, 0, 0, 0, true, false},
		{"one texture holds it", label, 2000, height, 0, 0, 0, 2000, false, true},
	}
	for _, c := range cases {
		skip, width, wide, ok := WideTextLabelWindow(c.label, scale, c.full, c.height, viewport, c.haveSkip, c.haveWidth)
		if skip != c.skip || width != c.width || wide != c.wide || ok != c.ok {
			t.Errorf("%s: window = %d, %d, %v, %v; want %d, %d, %v, %v", c.name, skip, width, wide, ok, c.skip, c.width, c.wide, c.ok)
		}
	}
}
