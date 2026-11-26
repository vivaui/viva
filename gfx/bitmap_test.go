package gfx

import "testing"

func TestCanvasRetainsValidBitmapWithoutCopyingPixels(t *testing.T) {
	pixels := []byte{
		0, 0, 255, 255, 0, 255, 0, 255,
		255, 0, 0, 255, 255, 255, 255, 255,
	}
	var canvas Canvas
	canvas.Clip(Rect{X: 5, Y: 5, Width: 10, Height: 10})
	canvas.DrawBitmap(Bitmap{Pix: pixels, Width: 2, Height: 2, Stride: 8, ID: 7}, Rect{Width: 20, Height: 20})
	canvas.Unclip()
	if len(canvas.Images()) != 1 {
		t.Fatalf("images = %d, want 1", len(canvas.Images()))
	}
	got := canvas.Images()[0]
	if got.Bitmap.ID != 7 || &got.Bitmap.Pix[0] != &pixels[0] || !got.HasClip {
		t.Fatalf("retained bitmap = %+v", got)
	}
	canvas.DrawBitmap(Bitmap{Pix: pixels[:4], Width: 2, Height: 2, Stride: 8}, Rect{Width: 10, Height: 10})
	if len(canvas.Images()) != 1 {
		t.Fatal("short pixel storage was accepted")
	}
}

func BenchmarkCanvasDrawBitmap(b *testing.B) {
	pixels := make([]byte, 640*360*4)
	bitmap := Bitmap{Pix: pixels, Width: 640, Height: 360, Stride: 640 * 4, ID: 1}
	var canvas Canvas
	canvas.DrawBitmap(bitmap, Rect{Width: 640, Height: 360})
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		canvas.Reset()
		canvas.DrawBitmap(bitmap, Rect{Width: 640, Height: 360})
	}
}

func TestDrawBackdropIgnoresClipAndLayer(t *testing.T) {
	var canvas Canvas
	pixels := make([]byte, 4*4*4)
	canvas.Clip(Rect{X: 10, Y: 10, Width: 5, Height: 5})
	canvas.PushLayer(9)
	canvas.DrawBackdrop(Bitmap{Pix: pixels, Width: 4, Height: 4, Stride: 16, ID: 1}, Rect{Width: 4, Height: 4}, Rect{Width: 100, Height: 100})
	canvas.PopLayer()
	canvas.DrawBackdrop(Bitmap{Width: 4}, Rect{Width: 4, Height: 4}, Rect{Width: 100, Height: 100}) // invalid: ignored
	images := canvas.Images()
	if len(images) != 1 || !images[0].Backdrop || images[0].Layer != 0 || images[0].HasClip {
		t.Fatalf("images = %+v", images)
	}
}

// A bitmap of the screen's own pixels lands on a device pixel at any display
// scale, and covers as many logical pixels as its device size says: 32 pixels
// at 125% cover 25.6 logical ones, not the 26 the painter would round them to.
func TestDrawBitmapPixelsKeepsTheBitmapOnDevicePixels(t *testing.T) {
	pixels := make([]byte, 3*2*4)
	var canvas Canvas
	canvas.SetPixelScale(1.25)
	covered := canvas.DrawBitmapPixels(Bitmap{Pix: pixels, Width: 3, Height: 2, Stride: 12, ID: 9}, Point{X: 2.4, Y: 4.8})
	images := canvas.Images()
	if len(images) != 1 {
		t.Fatalf("images = %d, want 1", len(images))
	}
	image := images[0]
	if !image.DevicePixels {
		t.Fatal("the bitmap is not marked as device pixels")
	}
	// 2.4 logical pixels is device pixel 3, and 4.8 is device pixel 6.
	if covered.X*1.25 != 3 || covered.Y*1.25 != 6 {
		t.Fatalf("covered at %v, want device pixel 3,6", covered)
	}
	if covered.Width*1.25 != 3 || covered.Height*1.25 != 2 {
		t.Fatalf("covered %v, want 3x2 device pixels", covered)
	}
	if image.Bounds != covered {
		t.Fatalf("bounds %v, covered %v", image.Bounds, covered)
	}
	// Without a scale a device pixel is half a logical one, as Surface.DrawPixels
	// takes it: the reference scale is 2.
	var plain Canvas
	plainCovered := plain.DrawBitmapPixels(Bitmap{Pix: pixels, Width: 3, Height: 2, Stride: 12}, Point{X: 1, Y: 1})
	if plainCovered.X != 1 || plainCovered.Y != 1 || plainCovered.Width != 1.5 || plainCovered.Height != 1 {
		t.Fatalf("without a scale: %v", plainCovered)
	}
}
