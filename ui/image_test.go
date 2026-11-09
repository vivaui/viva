package ui

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func imageBitmap(id uint64) gfx.Bitmap {
	return gfx.Bitmap{Pix: make([]byte, 32*32*4), Width: 32, Height: 32, Stride: 32 * 4, ID: id}
}

// An Image is drawn into the rectangle the layout gives it, 16 pixels square
// unless sized, and keeps the bitmap's identity so the renderer can reuse the
// texture it made for it.
func TestImageIsDrawnIntoItsLayoutRectangle(t *testing.T) {
	var view View
	canvas := testCanvas()
	var small, large Box
	view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 200, Height: 100}, func(f *Frame) {
		f.Row().Gap(10).Items(Center).Children(func() {
			small = f.Image(imageBitmap(7))
			large = f.Image(imageBitmap(8)).Size(40, 40)
		})
	})
	images := canvas.Images()
	if len(images) != 2 {
		t.Fatalf("images drawn = %d, want 2", len(images))
	}
	for index, box := range []Box{small, large} {
		if images[index].Bounds != box.Rect() {
			t.Fatalf("image %d bounds = %+v, want its rectangle %+v", index, images[index].Bounds, box.Rect())
		}
	}
	if got := small.Rect(); got.Width != 16 || got.Height != 16 {
		t.Fatalf("default size = %vx%v, want 16x16", got.Width, got.Height)
	}
	if images[0].Bitmap.ID != 7 || images[1].Bitmap.ID != 8 {
		t.Fatalf("bitmap IDs = %d, %d, want 7, 8", images[0].Bitmap.ID, images[1].Bitmap.ID)
	}
}

func TestAPageOfImagesAllocatesNothingOnceWarm(t *testing.T) {
	var view View
	canvas := testCanvas()
	theme := gfx.LightTheme()
	bitmap := imageBitmap(1)
	run := func() {
		canvas.Reset()
		view.Draw(canvas, theme, gfx.Rect{Width: 400, Height: 400}, func(f *Frame) {
			f.Column().Children(func() {
				for range 12 {
					f.Row().Items(Center).Children(func() {
						f.Image(bitmap)
						f.Text("Safari.app").Grow(1)
					})
				}
			})
		})
	}
	run()
	if allocs := testing.AllocsPerRun(200, run); allocs != 0 {
		t.Fatalf("a warm page of images allocates %v times a frame, want 0", allocs)
	}
}

func BenchmarkPageOfImages(b *testing.B) {
	var view View
	canvas := testCanvas()
	theme := gfx.LightTheme()
	bitmap := imageBitmap(1)
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		view.Draw(canvas, theme, gfx.Rect{Width: 400, Height: 400}, func(f *Frame) {
			f.Column().Children(func() {
				for range 12 {
					f.Row().Items(Center).Children(func() {
						f.Image(bitmap)
						f.Text("Safari.app").Grow(1)
					})
				}
			})
		})
	}
}
