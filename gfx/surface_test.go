package gfx

import "testing"

// patch is a width x height block of one BGRX color with padding in the
// fourth byte, as a remote-desktop protocol delivers it.
func patch(width, height int, blue, green, red byte) []byte {
	pix := make([]byte, width*height*4)
	for offset := 0; offset < len(pix); offset += 4 {
		pix[offset], pix[offset+1], pix[offset+2], pix[offset+3] = blue, green, red, 0
	}
	return pix
}

// pixelAt reads the pixel of the picture at (x, y) from what Draw handed the
// canvas, the way a renderer would see it.
func pixelAt(t *testing.T, surface *Surface, x, y int) [4]byte {
	t.Helper()
	var canvas Canvas
	width, height := surface.Size()
	surface.Draw(&canvas, Rect{Width: float32(width), Height: float32(height)})
	for _, image := range canvas.Images() {
		bounds := image.Bounds
		if float32(x) >= bounds.X && float32(x) < bounds.X+bounds.Width && float32(y) >= bounds.Y && float32(y) < bounds.Y+bounds.Height {
			offset := (y-int(bounds.Y))*image.Bitmap.Stride + (x-int(bounds.X))*4
			return [4]byte(image.Bitmap.Pix[offset : offset+4])
		}
	}
	t.Fatalf("no tile covers (%d, %d)", x, y)
	return [4]byte{}
}

func versions(surface *Surface) map[uint64]uint32 {
	var canvas Canvas
	width, height := surface.Size()
	surface.Draw(&canvas, Rect{Width: float32(width), Height: float32(height)})
	result := map[uint64]uint32{}
	for _, image := range canvas.Images() {
		result[image.Bitmap.ID] = image.Bitmap.Version
	}
	return result
}

func TestSurfaceUpdateChangesOnlyTheTilesItTouches(t *testing.T) {
	var surface Surface
	surface.Resize(1000, 600) // 4 x 3 tiles of 256
	before := versions(&surface)
	if len(before) != 12 {
		t.Fatalf("tiles = %d, want 12", len(before))
	}
	// A rectangle across the corner shared by four tiles.
	surface.Update(250, 250, 12, 12, patch(12, 12, 10, 20, 30), 12*4)
	after := versions(&surface)
	changed := 0
	for id, version := range after {
		if version != before[id] {
			changed++
		}
	}
	if changed != 4 {
		t.Fatalf("changed tiles = %d, want 4", changed)
	}
	for _, point := range [][2]int{{250, 250}, {255, 255}, {256, 256}, {261, 261}} {
		if got := pixelAt(t, &surface, point[0], point[1]); got != [4]byte{10, 20, 30, 255} {
			t.Fatalf("pixel %v = %v", point, got)
		}
	}
	if got := pixelAt(t, &surface, 249, 250); got != [4]byte{0, 0, 0, 255} {
		t.Fatalf("pixel outside the rectangle = %v", got)
	}
}

func TestSurfaceUpdateReadsASubrectangleOfALargerSource(t *testing.T) {
	var surface Surface
	surface.Resize(300, 300)
	// A 20 x 10 source whose right half is the patch.
	source := make([]byte, 20*10*4)
	for row := 0; row < 10; row++ {
		for column := 10; column < 20; column++ {
			copy(source[(row*20+column)*4:], []byte{1, 2, 3, 0})
		}
	}
	surface.Update(100, 100, 10, 10, source[10*4:], 20*4)
	if got := pixelAt(t, &surface, 105, 105); got != [4]byte{1, 2, 3, 255} {
		t.Fatalf("pixel = %v", got)
	}
}

func TestSurfaceUpdateIgnoresWhatIsOutsideThePicture(t *testing.T) {
	var surface Surface
	surface.Resize(300, 300)
	surface.Update(290, 290, 40, 40, patch(40, 40, 9, 9, 9), 40*4)
	surface.Update(-5, -5, 10, 10, patch(10, 10, 7, 7, 7), 10*4)
	surface.Update(400, 400, 10, 10, patch(10, 10, 5, 5, 5), 10*4)
	surface.Update(0, 0, 10, 10, patch(10, 10, 5, 5, 5)[:20], 10*4) // short source
	if got := pixelAt(t, &surface, 299, 299); got != [4]byte{9, 9, 9, 255} {
		t.Fatalf("clipped corner = %v", got)
	}
	if got := pixelAt(t, &surface, 4, 4); got != [4]byte{7, 7, 7, 255} {
		t.Fatalf("clipped origin = %v", got)
	}
	if got := pixelAt(t, &surface, 5, 5); got != [4]byte{0, 0, 0, 255} {
		t.Fatalf("pixel past the clipped patch = %v", got)
	}
}

func TestSurfaceTilesMeetExactlyAtAnyScale(t *testing.T) {
	var surface Surface
	surface.Resize(1920, 1080)
	for _, bounds := range []Rect{
		{X: 0, Y: 0, Width: 1920, Height: 1080},
		{X: 3.4, Y: 17.6, Width: 777.3, Height: 437.1},
		{X: 0, Y: 0, Width: 2561, Height: 1441},
	} {
		var canvas Canvas
		surface.Draw(&canvas, bounds)
		images := canvas.Images()
		if len(images) != 40 {
			t.Fatalf("tiles = %d", len(images))
		}
		// Neighbours share an edge: the tile to the right starts where this ends.
		for index, image := range images {
			column := index % 8
			if column < 7 && image.Bounds.X+image.Bounds.Width != images[index+1].Bounds.X {
				t.Fatalf("scale %v: horizontal gap or overlap at tile %d", bounds, index)
			}
			if index+8 < len(images) && image.Bounds.Y+image.Bounds.Height != images[index+8].Bounds.Y {
				t.Fatalf("scale %v: vertical gap or overlap at tile %d", bounds, index)
			}
		}
		last := images[len(images)-1].Bounds
		if last.X+last.Width != float32(int(bounds.X+bounds.Width+0.5)) || last.Y+last.Height != float32(int(bounds.Y+bounds.Height+0.5)) {
			t.Fatalf("the picture ends at %v, %v for bounds %v", last.X+last.Width, last.Y+last.Height, bounds)
		}
	}
}

func TestSurfaceKeepsTheTileCountBoundedForHugePictures(t *testing.T) {
	var surface Surface
	surface.Resize(7680, 4320)
	var canvas Canvas
	surface.Draw(&canvas, Rect{Width: 1000, Height: 500})
	if got := len(canvas.Images()); got > surfaceMaxTiles {
		t.Fatalf("tiles = %d, want at most %d", got, surfaceMaxTiles)
	}
}

func TestSurfaceResizeDoesNotReuseBitmapIdentities(t *testing.T) {
	var surface Surface
	surface.Resize(512, 512)
	first := versions(&surface)
	surface.Resize(512, 512)
	for id := range versions(&surface) {
		if _, reused := first[id]; reused {
			t.Fatal("a resized surface reused a bitmap ID, so a renderer could keep a stale texture")
		}
	}
	surface.Resize(0, 0)
	var canvas Canvas
	surface.Draw(&canvas, Rect{Width: 10, Height: 10})
	if len(canvas.Images()) != 0 {
		t.Fatal("an empty surface drew")
	}
}

func BenchmarkSurfaceUpdate(b *testing.B) {
	for _, size := range []struct {
		name          string
		width, height int
	}{{"64x64", 64, 64}, {"640x480", 640, 480}, {"1920x1080", 1920, 1080}} {
		b.Run(size.name, func(b *testing.B) {
			var surface Surface
			surface.Resize(1920, 1080)
			pix := patch(size.width, size.height, 1, 2, 3)
			b.ReportAllocs()
			b.SetBytes(int64(len(pix)))
			for b.Loop() {
				surface.Update(100, 100, size.width, size.height, pix, size.width*4)
			}
		})
	}
}

func BenchmarkSurfaceDraw(b *testing.B) {
	var surface Surface
	surface.Resize(1920, 1080)
	var canvas Canvas
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		surface.Draw(&canvas, Rect{Width: 1280, Height: 720})
	}
}

func TestSurfaceDrawPixelsPutsEveryTilePixelOnADevicePixel(t *testing.T) {
	var surface Surface
	surface.Resize(1750, 903)
	for _, scale := range []float32{1, 1.25, 1.5, 1.75, 2, 3} {
		var canvas Canvas
		canvas.SetPixelScale(scale)
		covered := surface.DrawPixels(&canvas, Point{X: 3.3, Y: 17.9})
		if wantX := float32(int(3.3*scale+0.5)) / scale; covered.X != wantX {
			t.Fatalf("scale %g: the picture starts at %v, want the nearest device pixel %v", scale, covered.X, wantX)
		}
		if covered.Width*scale < 1749.99 || covered.Width*scale > 1750.01 || covered.Height*scale < 902.99 || covered.Height*scale > 903.01 {
			t.Fatalf("scale %g: the picture covers %vx%v logical pixels, not 1750x903 device pixels", scale, covered.Width, covered.Height)
		}
		whole := func(value float32) bool {
			device := float64(value * scale)
			return device-float64(int(device+0.5)) < 1e-3 && float64(int(device+0.5))-device < 1e-3
		}
		for _, image := range canvas.Images() {
			bounds := image.Bounds
			if !image.DevicePixels || !whole(bounds.X) || !whole(bounds.Y) {
				t.Fatalf("scale %g: tile %v is not on device pixels", scale, bounds)
			}
			if got := int(bounds.Width*scale + 0.5); got != image.Bitmap.Width {
				t.Fatalf("scale %g: a %d pixel wide tile is drawn %d device pixels wide", scale, image.Bitmap.Width, got)
			}
			if got := int(bounds.Height*scale + 0.5); got != image.Bitmap.Height {
				t.Fatalf("scale %g: a %d pixel high tile is drawn %d device pixels high", scale, image.Bitmap.Height, got)
			}
		}
	}
}

func BenchmarkSurfaceDrawPixels(b *testing.B) {
	var surface Surface
	surface.Resize(1920, 1080)
	var canvas Canvas
	canvas.SetPixelScale(1.25)
	surface.DrawPixels(&canvas, Point{})
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		canvas.SetPixelScale(1.25)
		surface.DrawPixels(&canvas, Point{X: 10, Y: 20})
	}
}
