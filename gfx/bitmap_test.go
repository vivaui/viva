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
