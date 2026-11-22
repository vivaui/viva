package windowframe

import "testing"

func TestFrameMarginKeepsWholePixelsAtEveryScale(t *testing.T) {
	for _, test := range []struct {
		scale float32
		want  uint32
	}{{1, 24}, {1.25, 30}, {1.5, 36}, {2, 48}, {0.5, 24}} {
		if got := Margin(test.scale); got != test.want {
			t.Fatalf("margin at %gx = %d, want %d", test.scale, got, test.want)
		}
	}
}

func TestContentSizeLeavesAtLeastAPixel(t *testing.T) {
	if got := ContentSize(448, 24); got != 400 {
		t.Fatalf("content = %d", got)
	}
	if got := ContentSize(40, 24); got != 1 {
		t.Fatalf("a window smaller than its margins has content %d", got)
	}
}

func TestFrameBandsTileTheMarginExactly(t *testing.T) {
	const width, height, margin = 448, 348, 24
	covered := make(map[[2]int32]int)
	for _, band := range Bands(width, height, margin) {
		for y := band.Y; y < band.Y+band.Height; y++ {
			for x := band.X; x < band.X+band.Width; x++ {
				covered[[2]int32{x, y}]++
			}
		}
	}
	content := Content(width, height, margin)
	for y := int32(0); y < height; y++ {
		for x := int32(0); x < width; x++ {
			inside := x >= content.X && x < content.X+content.Width && y >= content.Y && y < content.Y+content.Height
			count := covered[[2]int32{x, y}]
			if inside && count != 0 || !inside && count != 1 {
				t.Fatalf("pixel %d,%d is in %d bands, inside the content: %v", x, y, count, inside)
			}
		}
	}
}

func TestFrameCornersSitInsideTheContent(t *testing.T) {
	const width, height, margin = 448, 348, 24
	content := Content(width, height, margin)
	for index, corner := range Corners(width, height, margin, 10) {
		if corner.Width != 11 || corner.Height != 11 {
			t.Fatalf("corner %d is %dx%d, want the radius and a pixel", index, corner.Width, corner.Height)
		}
		if corner.X < content.X || corner.Y < content.Y || corner.X+corner.Width > content.X+content.Width || corner.Y+corner.Height > content.Y+content.Height {
			t.Fatalf("corner %d %+v is outside the content %+v", index, corner, content)
		}
	}
}

func TestFrameInputReachesOnlyAGripIntoTheMargin(t *testing.T) {
	got := Input(448, 348, 24, 8)
	if got != (Box{16, 16, 416, 316}) {
		t.Fatalf("input = %+v", got)
	}
	if got := Input(448, 348, 4, 8); got != (Box{0, 0, 448, 348}) {
		t.Fatalf("a margin smaller than the grip: %+v", got)
	}
}
