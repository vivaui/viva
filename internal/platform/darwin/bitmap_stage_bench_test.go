//go:build darwin && !ios

package darwin

import "testing"

// BenchmarkStageBitmapUpload is the cost on the Go side of the native boundary
// of a remote-desktop frame: staging the pixels of the bitmaps the frame
// uploads. "whole" is one 1920x1080 bitmap whose version changed, the only
// choice before a picture could be drawn in tiles; "tile" is the same picture as
// 40 tiles of which one changed (the other 39 reference their retained texture).
func BenchmarkStageBitmapUpload(b *testing.B) {
	whole := []TextLabel{{
		Width: 1920, Height: 1080, Bitmap: make([]byte, 1920*1080*4), BitmapWidth: 1920, BitmapHeight: 1080,
		BitmapStride: 1920 * 4, BitmapID: 1, BitmapVersion: 2, BitmapUpload: true,
	}}
	tiles := make([]TextLabel, 40)
	for index := range tiles {
		tiles[index] = TextLabel{
			Width: 256, Height: 256, Bitmap: make([]byte, 256*256*4), BitmapWidth: 256, BitmapHeight: 256,
			BitmapStride: 256 * 4, BitmapID: uint64(10 + index), BitmapVersion: 1,
		}
	}
	tiles[3].BitmapUpload = true
	for _, test := range []struct {
		name   string
		labels []TextLabel
	}{{"whole", whole}, {"tile", tiles}} {
		b.Run(test.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				memory, strings, _, err := makeNativeTextLabels(test.labels)
				if err != nil {
					b.Fatal(err)
				}
				freeAccessibility(memory, strings)
			}
		})
	}
}
