package app

import (
	"math"
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

func TestBitmapPaintIsRetainedAndClippedWithoutPixelCopies(t *testing.T) {
	pixels := make([]byte, 4*4*4)
	caption := "one"
	for index := 3; index < len(pixels); index += 4 {
		pixels[index] = 255
	}
	config := Config{Width: 40, Height: 40, Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
		canvas.Clip(gfx.Rect{X: 10, Y: 10, Width: 20, Height: 20})
		canvas.DrawBitmap(gfx.Bitmap{Pix: pixels, Width: 4, Height: 4, Stride: 16, ID: 9}, gfx.Rect{Width: 40, Height: 40})
		canvas.Label(caption, gfx.Rect{X: 12, Y: 12, Width: 16, Height: 10}, gfx.TextStyle{Size: 10})
		canvas.Unclip()
	}}
	painter, err := newPainter(&gfx.Canvas{}, &config)
	if err != nil {
		t.Fatal(err)
	}
	if len(painter.labels) != 2 || !painter.labels[0].BitmapUpload {
		t.Fatalf("labels = %+v", painter.labels)
	}
	label := painter.labels[0]
	if label.X != 10 || label.Y != 10 || label.Width != 20 || label.Height != 20 || label.BitmapWidth != 2 || label.BitmapHeight != 2 {
		t.Fatalf("clipped bitmap label = %+v", label)
	}
	frame, err := painter.repaint()
	if err != nil {
		t.Fatal(err)
	}
	if frame.TextLabels != nil {
		t.Fatal("unchanged bitmap was uploaded again")
	}
	caption = "two"
	frame, err = painter.repaint()
	if err != nil {
		t.Fatal(err)
	}
	if len(frame.TextLabels) != 2 || frame.TextLabels[0].BitmapUpload {
		t.Fatalf("text-only frame restaged bitmap pixels: %+v", frame.TextLabels)
	}
}

func TestInitialBitmapUploadAfterPreparationPaint(test *testing.T) {
	bitmap := gfx.Bitmap{Pix: []byte{0, 0, 255, 255}, Width: 1, Height: 1, Stride: 4, ID: 9}
	for _, preparation := range []string{"", "ready", "estimated"} {
		test.Run(preparation, func(test *testing.T) {
			caption := "ready"
			config := Config{Width: 80, Height: 40, Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
				canvas.DrawBitmap(bitmap, gfx.Rect{Width: 20, Height: 20})
				canvas.Label(caption, gfx.Rect{X: 20, Width: 60, Height: 20}, gfx.TextStyle{Size: 12})
			}}
			var canvas gfx.Canvas
			if preparation != "" {
				caption = preparation
				config.Paint(&canvas, gfx.Size{Width: 80, Height: 40})
				caption = "ready"
			}
			painter, err := newPainter(&canvas, &config)
			if err != nil {
				test.Fatal(err)
			}
			found := false
			for _, label := range painter.labels {
				if label.BitmapID == bitmap.ID {
					found = true
					if !label.BitmapUpload {
						test.Fatal("initial frame omitted pixels before the native renderer had any bitmap")
					}
				}
			}
			if !found {
				test.Fatal("initial frame omitted the bitmap")
			}
			caption = "updated"
			frame, err := painter.repaint()
			if err != nil {
				test.Fatal(err)
			}
			if len(frame.TextLabels) == 0 {
				test.Fatal("caption change produced no label update")
			}
			for _, label := range frame.TextLabels {
				if len(label.Bitmap) > 0 && label.BitmapUpload {
					test.Fatal("a later text-only update uploaded retained pixels again")
				}
			}
		})
	}
}

func BenchmarkBitmapStartup(bench *testing.B) {
	bitmap := gfx.Bitmap{Pix: make([]byte, 64*64*4), Width: 64, Height: 64, Stride: 256, ID: 9}
	caption := "estimated"
	config := Config{Width: 200, Height: 100, Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
		canvas.DrawBitmap(bitmap, gfx.Rect{Width: 64, Height: 64})
		canvas.Label(caption, gfx.Rect{X: 64, Width: 120, Height: 20}, gfx.TextStyle{Size: 12})
	}}
	bench.Run("initial", func(bench *testing.B) {
		bench.ReportAllocs()
		for bench.Loop() {
			var canvas gfx.Canvas
			caption = "estimated"
			config.Paint(&canvas, gfx.Size{Width: 200, Height: 100})
			caption = "ready"
			if _, err := newPainter(&canvas, &config); err != nil {
				bench.Fatal(err)
			}
		}
	})
	bench.Run("unchanged", func(bench *testing.B) {
		painter, err := newPainter(&gfx.Canvas{}, &config)
		if err != nil {
			bench.Fatal(err)
		}
		bench.ReportAllocs()
		for bench.Loop() {
			if _, err := painter.repaint(); err != nil {
				bench.Fatal(err)
			}
		}
	})
}

func TestBackdropIsBeneathEveryLayerAndShiftsThemOnlyWhilePresent(t *testing.T) {
	pixels := make([]byte, 4*4*4)
	for index := 3; index < len(pixels); index += 4 {
		pixels[index] = 255
	}
	backdrop := true
	config := Config{Width: 40, Height: 40, Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
		if backdrop {
			// Painted after the content, and clipped content around it: the
			// backdrop is still first and unclipped.
			defer canvas.DrawBackdrop(gfx.Bitmap{Pix: pixels, Width: 4, Height: 4, Stride: 16, ID: 7}, gfx.Rect{Width: 4, Height: 4}, gfx.Rect{Width: 40, Height: 40})
		}
		canvas.Fill(gfx.Rect{Width: 10, Height: 10}, gfx.Color{Alpha: 1})
		canvas.Label("base", gfx.Rect{Width: 20, Height: 10}, gfx.TextStyle{Size: 10})
		canvas.PushLayer(600)
		canvas.Fill(gfx.Rect{Width: 5, Height: 5}, gfx.Color{Alpha: 1})
		canvas.PopLayer()
		canvas.PushLayer(math.MaxUint16)
		canvas.Fill(gfx.Rect{Width: 2, Height: 2}, gfx.Color{Alpha: 1})
		canvas.PopLayer()
	}}
	painter, err := newPainter(&gfx.Canvas{}, &config)
	if err != nil {
		t.Fatal(err)
	}
	layers := func() (rectangles, labels []uint16) {
		for _, rectangle := range painter.renderer.Rectangles() {
			rectangles = append(rectangles, rectangle.Layer)
		}
		for _, label := range painter.labels {
			labels = append(labels, label.Layer)
		}
		return
	}
	rectangles, labels := layers()
	if !slices.Equal(rectangles, []uint16{1, 601, math.MaxUint16}) || !slices.Equal(labels, []uint16{0, 1}) {
		t.Fatalf("with a backdrop: rectangles %v, labels %v", rectangles, labels)
	}
	if painter.labels[0].BitmapID != 7 || painter.labels[0].BitmapWidth != 4 {
		t.Fatalf("backdrop label = %+v", painter.labels[0])
	}

	backdrop = false
	frame, err := painter.repaint()
	if err != nil {
		t.Fatal(err)
	}
	rectangles, labels = layers()
	if !slices.Equal(rectangles, []uint16{0, 600, math.MaxUint16}) || !slices.Equal(labels, []uint16{0}) {
		t.Fatalf("without: rectangles %v, labels %v", rectangles, labels)
	}
	if !frame.Update.Rebuild {
		t.Fatalf("losing the backdrop moved every layer but the update is %+v", frame.Update)
	}
	if frame.TextLabels == nil {
		t.Fatal("the labels were not sent again")
	}

	backdrop = true
	frame, err = painter.repaint()
	if err != nil {
		t.Fatal(err)
	}
	rectangles, _ = layers()
	if !frame.Update.Rebuild || !slices.Equal(rectangles, []uint16{1, 601, math.MaxUint16}) {
		t.Fatalf("gaining it again: %+v %v", frame.Update, rectangles)
	}
}

func BenchmarkRepaintWithBackdrop(b *testing.B) {
	pixels := make([]byte, 64*64*4)
	config := Config{Width: 400, Height: 300, Paint: func(canvas *gfx.Canvas, size gfx.Size) {
		canvas.DrawBackdrop(gfx.Bitmap{Pix: pixels, Width: 64, Height: 64, Stride: 256, ID: 3}, gfx.Rect{Width: 64, Height: 64}, gfx.Rect{Width: size.Width, Height: size.Height})
		canvas.Fill(gfx.Rect{Width: 100, Height: 40}, gfx.Color{Alpha: 0.5})
		canvas.Label("steady", gfx.Rect{Y: 50, Width: 100, Height: 20}, gfx.TextStyle{Size: 12})
	}}
	painter, err := newPainter(&gfx.Canvas{}, &config)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := painter.repaint(); err != nil {
			b.Fatal(err)
		}
	}
}

func TestOnlyChangedBitmapsCrossTheNativeBoundary(t *testing.T) {
	var surface gfx.Surface
	surface.Resize(600, 300) // 3 x 2 tiles
	caption := "one"
	config := Config{Width: 600, Height: 320, Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
		surface.Draw(canvas, gfx.Rect{Width: 600, Height: 300})
		canvas.Label(caption, gfx.Rect{Y: 300, Width: 100, Height: 16}, gfx.TextStyle{Size: 10})
	}}
	painter, err := newPainter(&gfx.Canvas{}, &config)
	if err != nil {
		t.Fatal(err)
	}
	uploads := func(labels []platform.TextLabel) (count int) {
		for _, label := range labels {
			if len(label.Bitmap) > 0 && label.BitmapUpload {
				count++
			}
		}
		return count
	}
	if got := uploads(painter.labels); got != 6 {
		t.Fatalf("first frame uploads %d bitmaps, want all 6", got)
	}
	// A patch inside the first tile: one upload, the other five stay on the GPU.
	surface.Update(10, 10, 8, 8, make([]byte, 8*8*4), 8*4)
	frame, err := painter.repaint()
	if err != nil {
		t.Fatal(err)
	}
	if got := uploads(frame.TextLabels); got != 1 {
		t.Fatalf("a patch in one tile uploads %d bitmaps, want 1", got)
	}
	// A frame that changes only text uploads none.
	caption = "two"
	frame, err = painter.repaint()
	if err != nil {
		t.Fatal(err)
	}
	if got := uploads(frame.TextLabels); got != 0 {
		t.Fatalf("a text-only frame uploads %d bitmaps", got)
	}
}

// A picture laid out in device pixels reaches the platform unrounded: at a
// fractional display scale a tile that logical-pixel rounding moved by a
// device pixel would be resampled and blurred.
func TestDevicePixelBitmapsKeepTheirDevicePixelsAtFractionalScales(t *testing.T) {
	var surface gfx.Surface
	surface.Resize(1000, 300)
	for _, scale := range []float32{1.25, 1.5, 1.75} {
		clipped := false
		config := Config{Width: 900, Height: 300, Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
			if clipped {
				canvas.Clip(gfx.Rect{X: 0, Y: 0, Width: 333.3, Height: 111.1})
			}
			surface.DrawPixels(canvas, gfx.Point{X: 7.7, Y: 5.5})
			if clipped {
				canvas.Unclip()
			}
		}}
		painter, err := newPainter(&gfx.Canvas{}, &config)
		if err != nil {
			t.Fatal(err)
		}
		for _, clipped = range []bool{false, true} {
			if _, err := painter.resize(platform.Size{Width: 900, Height: 300, Scale: scale}); err != nil {
				t.Fatal(err)
			}
			covered := 0
			for _, label := range painter.labels {
				if len(label.Bitmap) == 0 {
					continue
				}
				x, y := float64(label.X*scale), float64(label.Y*scale)
				width, height := float64(label.Width*scale), float64(label.Height*scale)
				for _, device := range []float64{x, y, width, height} {
					if math.Abs(device-math.Round(device)) > 1e-3 {
						t.Fatalf("scale %g clipped %v: label %+v is not on device pixels", scale, clipped, label)
					}
				}
				if int(math.Round(width)) != label.BitmapWidth || int(math.Round(height)) != label.BitmapHeight {
					t.Fatalf("scale %g clipped %v: a %dx%d bitmap region is drawn %vx%v device pixels", scale, clipped, label.BitmapWidth, label.BitmapHeight, width, height)
				}
				covered += label.BitmapWidth * label.BitmapHeight
			}
			if !clipped && covered != 1000*300 {
				t.Fatalf("scale %g: %d of %d picture pixels are drawn", scale, covered, 1000*300)
			}
			if clipped && (covered == 0 || covered >= 1000*300) {
				t.Fatalf("scale %g: the clip leaves %d picture pixels", scale, covered)
			}
		}
	}
}
