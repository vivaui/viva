//go:build linux

package linux

import (
	"errors"
	"image"
	"math"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

// newTestOffscreen skips the test on a machine with no Vulkan driver, as the
// other graphics tests do, and fails on any other error.
func newTestOffscreen(t *testing.T, width, height int) *Offscreen {
	t.Helper()
	return newScaledOffscreen(t, width, height, 1)
}

// newScaledOffscreen is newTestOffscreen at a display scale.
func newScaledOffscreen(t *testing.T, width, height int, scale float64) *Offscreen {
	t.Helper()
	offscreen, err := NewOffscreen(width, height, scale, platform.Color{Alpha: 1})
	if errors.Is(err, platform.ErrNoGraphics) {
		t.Skipf("no Vulkan device: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(offscreen.Close)
	return offscreen
}

func renderScene(t *testing.T, offscreen *Offscreen, rectangles []render.Rectangle, labels []platform.TextLabel) *image.RGBA {
	t.Helper()
	if err := offscreen.Set(rectangles, labels); err != nil {
		t.Fatal(err)
	}
	pixels, err := offscreen.Image()
	if err != nil {
		t.Fatal(err)
	}
	return pixels
}

func rectanglesScene(t *testing.T, size int, rectangles []render.Rectangle) *image.RGBA {
	t.Helper()
	return renderScene(t, newTestOffscreen(t, size, size), rectangles, nil)
}

func rgbaAt(pixels *image.RGBA, x, y int) (red, green, blue, alpha byte) {
	offset := pixels.PixOffset(x, y)
	return pixels.Pix[offset], pixels.Pix[offset+1], pixels.Pix[offset+2], pixels.Pix[offset+3]
}

func isPainted(pixels *image.RGBA, x, y int) bool {
	red, green, blue, _ := rgbaAt(pixels, x, y)
	return int(red)+int(green)+int(blue) > 60
}

func TestOffscreenReadsBackRGBAWithExactFlatFills(t *testing.T) {
	// Pure red must come back as red, not blue: the target is BGRA in memory.
	pixels := rectanglesScene(t, 40, []render.Rectangle{
		{Bounds: scene.Rect{X: 5, Y: 5, Width: 10, Height: 10}, Color: scene.Color{Red: 1, Alpha: 1}},
		{Bounds: scene.Rect{X: 20, Y: 5, Width: 10, Height: 10}, Color: scene.Color{Green: 0.5, Blue: 1, Alpha: 1}},
	})
	if pixels.Bounds() != image.Rect(0, 0, 40, 40) {
		t.Fatalf("bounds = %v, want 40x40", pixels.Bounds())
	}
	if r, g, b, a := rgbaAt(pixels, 10, 10); r != 255 || g != 0 || b != 0 || a != 255 {
		t.Errorf("red rectangle = rgba(%d,%d,%d,%d)", r, g, b, a)
	}
	// Half green is the unorm value, not an sRGB-encoded one.
	if r, g, b, a := rgbaAt(pixels, 25, 10); r != 0 || g < 127 || g > 128 || b != 255 || a != 255 {
		t.Errorf("blue-green rectangle = rgba(%d,%d,%d,%d)", r, g, b, a)
	}
	if r, g, b, a := rgbaAt(pixels, 35, 35); r != 0 || g != 0 || b != 0 || a != 255 {
		t.Errorf("background = rgba(%d,%d,%d,%d), want the opaque clear color", r, g, b, a)
	}
}

func TestOffscreenLayersPaintOverEarlierInstances(t *testing.T) {
	pixels := rectanglesScene(t, 100, []render.Rectangle{
		{Bounds: scene.Rect{X: 10, Y: 10, Width: 40, Height: 20}, Color: scene.Color{Red: 1, Alpha: 1}},
		// A later layer paints over an earlier one regardless of instance order.
		{Bounds: scene.Rect{X: 30, Y: 10, Width: 40, Height: 20}, Color: scene.Color{Green: 1, Alpha: 1}, Layer: 1},
		{Bounds: scene.Rect{X: 30, Y: 10, Width: 40, Height: 20}, Color: scene.Color{Blue: 1, Alpha: 1}},
	})
	if r, g, b, _ := rgbaAt(pixels, 20, 20); r != 255 || g != 0 || b != 0 {
		t.Errorf("red rectangle = %d,%d,%d", r, g, b)
	}
	if r, g, b, _ := rgbaAt(pixels, 40, 20); r != 0 || g != 255 || b != 0 {
		t.Errorf("layered overlap = %d,%d,%d, want the layer-1 green", r, g, b)
	}
	if r, g, b, _ := rgbaAt(pixels, 60, 20); r != 0 || g != 255 || b != 0 {
		t.Errorf("layer-1 only region = %d,%d,%d, want green", r, g, b)
	}
}

func TestOffscreenRoundedRectangleHasRoundCornersAndSolidInterior(t *testing.T) {
	white := scene.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	pixels := rectanglesScene(t, 100, []render.Rectangle{{
		Bounds: scene.Rect{X: 10, Y: 10, Width: 80, Height: 80}, Radius: 20, Color: white,
	}})
	if isPainted(pixels, 11, 11) || isPainted(pixels, 88, 88) {
		t.Error("a corner of a radius-20 rectangle is square")
	}
	if r, g, b, a := rgbaAt(pixels, 50, 50); r != 255 || g != 255 || b != 255 || a != 255 {
		t.Errorf("interior = rgba(%d,%d,%d,%d)", r, g, b, a)
	}
	if !isPainted(pixels, 50, 11) || !isPainted(pixels, 11, 50) {
		t.Error("the straight edges are missing")
	}
	if isPainted(pixels, 5, 50) || isPainted(pixels, 50, 95) {
		t.Error("the rectangle painted outside its bounds")
	}
}

func TestOffscreenTriangleMeshHasNoInternalFeathering(t *testing.T) {
	white := scene.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	// Two triangles share the diagonal; only their outer edges are feathered.
	pixels := rectanglesScene(t, 100, []render.Rectangle{
		{
			Kind: scene.KindTriangle, Color: white, EdgeMask: 1<<0 | 1<<1,
			Points: [3]scene.Point{{X: 10, Y: 10}, {X: 90, Y: 10}, {X: 90, Y: 90}},
		},
		{
			Kind: scene.KindTriangle, Color: white, EdgeMask: 1<<1 | 1<<2,
			Points: [3]scene.Point{{X: 10, Y: 10}, {X: 90, Y: 90}, {X: 10, Y: 90}},
		},
	})
	for coordinate := 20; coordinate < 80; coordinate += 7 {
		if r, g, b, a := rgbaAt(pixels, coordinate, coordinate); r < 250 || g < 250 || b < 250 || a != 255 {
			t.Fatalf("shared diagonal pixel %d = rgba(%d,%d,%d,%d), want opaque white", coordinate, r, g, b, a)
		}
	}
	if isPainted(pixels, 5, 5) || isPainted(pixels, 95, 95) {
		t.Error("a triangle painted outside its points")
	}
}

func TestOffscreenShadedTriangleInterpolatesItsCornerColors(t *testing.T) {
	pixels := rectanglesScene(t, 120, []render.Rectangle{{
		Kind: scene.KindTriangle, Color: scene.Color{Red: 1, Alpha: 1},
		Shade:  scene.Shade{On: true, Colors: [2]scene.Color{{Green: 1, Alpha: 1}, {Blue: 1, Alpha: 1}}},
		Points: [3]scene.Point{{X: 10, Y: 10}, {X: 110, Y: 10}, {X: 10, Y: 110}},
	}})
	for _, at := range [][2]int{{20, 20}, {60, 20}, {20, 60}, {44, 44}, {30, 70}} {
		w1, w2 := (float64(at[0])+0.5-10)/100, (float64(at[1])+0.5-10)/100
		want := [3]float64{(1 - w1 - w2) * 255, w1 * 255, w2 * 255}
		red, green, blue, alpha := rgbaAt(pixels, at[0], at[1])
		got := [3]float64{float64(red), float64(green), float64(blue)}
		for channel := range got {
			if diff := got[channel] - want[channel]; diff > 8 || diff < -8 {
				t.Fatalf("pixel %v = rgb(%v), want about rgb(%.0f, %.0f, %.0f)", at, got, want[0], want[1], want[2])
			}
		}
		if alpha != 255 {
			t.Fatalf("pixel %v alpha = %d", at, alpha)
		}
	}
}

func TestOffscreenClipCutsARoundedRectangleOnlyWhereTheClipIs(t *testing.T) {
	white := scene.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	// The rectangle spans 10..90; the clip cuts its right and bottom parts away
	// at 60 but leaves its top-left corner (radius 20) in place.
	pixels := rectanglesScene(t, 100, []render.Rectangle{{
		Bounds: scene.Rect{X: 10, Y: 10, Width: 80, Height: 80}, Radius: 20, Color: white,
		Clipped: true, Clip: scene.Rect{X: 0, Y: 0, Width: 60, Height: 60},
	}})
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			if (x >= 60 || y >= 60) && isPainted(pixels, x, y) {
				t.Fatalf("pixel (%d,%d) outside the clip is painted", x, y)
			}
		}
	}
	if !isPainted(pixels, 50, 50) || !isPainted(pixels, 59, 59) {
		t.Fatal("the part inside the clip is missing")
	}
	if isPainted(pixels, 11, 11) {
		t.Fatal("the uncut top-left corner is not rounded")
	}
	if !isPainted(pixels, 30, 30) {
		t.Fatal("the rectangle's interior is missing")
	}
}

func TestOffscreenClipCutsATriangle(t *testing.T) {
	white := scene.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	pixels := rectanglesScene(t, 100, []render.Rectangle{{
		Kind: scene.KindTriangle, Color: white, EdgeMask: 7,
		Points:  [3]scene.Point{{X: 10, Y: 10}, {X: 90, Y: 10}, {X: 10, Y: 90}},
		Clipped: true, Clip: scene.Rect{X: 0, Y: 0, Width: 50, Height: 50},
	}})
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			if (x >= 50 || y >= 50) && isPainted(pixels, x, y) {
				t.Fatalf("pixel (%d,%d) outside the clip is painted", x, y)
			}
		}
	}
	if !isPainted(pixels, 25, 25) {
		t.Fatal("the part of the triangle inside the clip is missing")
	}
}

func TestOffscreenClipsOnOneLayerAreIndependentAndKeepPaintOrder(t *testing.T) {
	red := scene.Color{Red: 1, Alpha: 1}
	green := scene.Color{Green: 1, Alpha: 1}
	whole := scene.Rect{X: 0, Y: 0, Width: 100, Height: 100}
	pixels := rectanglesScene(t, 100, []render.Rectangle{
		{Bounds: whole, Color: red, Clipped: true, Clip: scene.Rect{X: 0, Y: 0, Width: 40, Height: 100}},
		{Bounds: whole, Color: red, Clipped: true, Clip: scene.Rect{X: 60, Y: 0, Width: 40, Height: 100}},
		// A later, unclipped rectangle paints over the earlier clipped ones.
		{Bounds: scene.Rect{X: 20, Y: 20, Width: 20, Height: 20}, Color: green},
	})
	if isPainted(pixels, 50, 50) {
		t.Fatal("the gap between the two clips is painted")
	}
	if r, _, _, _ := rgbaAt(pixels, 10, 80); r < 250 {
		t.Fatal("the first clip's rectangle is missing")
	}
	if r, _, _, _ := rgbaAt(pixels, 90, 80); r < 250 {
		t.Fatal("the second clip's rectangle is missing")
	}
	if r, g, b, _ := rgbaAt(pixels, 30, 30); g < 250 || r > 5 || b > 5 {
		t.Fatalf("the later rectangle does not paint over the earlier one: RGB %d,%d,%d", r, g, b)
	}
}

func TestOffscreenClipLandsOnTheNearestPixel(t *testing.T) {
	white := scene.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	// The clip edges 10.6 and 20.4 round to pixels 11 and 20, so pixels 11..19
	// are painted and 10 and 20 are not: neither the floor nor the ceiling.
	pixels := rectanglesScene(t, 50, []render.Rectangle{{
		Bounds: scene.Rect{X: 0, Y: 0, Width: 50, Height: 50}, Color: white,
		Clipped: true, Clip: scene.Rect{X: 10.6, Y: 10.6, Width: 9.8, Height: 9.8},
	}})
	for _, test := range []struct {
		x, y int
		want bool
	}{
		{10, 15, false}, {11, 15, true}, {19, 15, true}, {20, 15, false},
		{15, 10, false}, {15, 11, true}, {15, 19, true}, {15, 20, false},
	} {
		if got := isPainted(pixels, test.x, test.y); got != test.want {
			t.Errorf("pixel (%d,%d) painted = %v, want %v", test.x, test.y, got, test.want)
		}
	}
}

// A bitmap label is two premultiplied BGRA pixels stretched over its quad.
func bitmapLabel() platform.TextLabel {
	return platform.TextLabel{
		X: 2, Y: 2, Width: 16, Height: 6,
		Bitmap: []byte{0, 0, 255, 255, 0, 255, 0, 255}, BitmapWidth: 2, BitmapHeight: 1, BitmapStride: 8, BitmapID: 1, BitmapUpload: true,
	}
}

func TestOffscreenDrawsPremultipliedBGRABitmapPixels(t *testing.T) {
	offscreen := newTestOffscreen(t, 20, 10)
	pixels := renderScene(t, offscreen, nil, []platform.TextLabel{bitmapLabel()})
	if r, g, b, a := rgbaAt(pixels, 5, 5); r < 240 || g > 5 || b > 5 || a < 240 {
		t.Fatalf("left pixel = RGBA %d,%d,%d,%d, want red", r, g, b, a)
	}
	if r, g, b, a := rgbaAt(pixels, 15, 5); g < 240 || r > 5 || b > 5 || a < 240 {
		t.Fatalf("right pixel = RGBA %d,%d,%d,%d, want green", r, g, b, a)
	}
}

// A label a canvas clip cuts partway is painted only inside the clip; the rest
// of the same quad stays untouched and the scissor does not leak to the next
// frame.
func TestOffscreenScissorsAClippedLabelToItsClip(t *testing.T) {
	offscreen := newTestOffscreen(t, 20, 10)
	label := bitmapLabel()
	lit := func(pixels *image.RGBA, x, y int) bool {
		red, green, blue, alpha := rgbaAt(pixels, x, y)
		return alpha > 200 && (red > 200 || green > 200 || blue > 200)
	}

	pixels := renderScene(t, offscreen, nil, []platform.TextLabel{label})
	if !lit(pixels, 5, 5) || !lit(pixels, 15, 5) || !lit(pixels, 5, 7) {
		t.Fatal("the unclipped label does not cover its bounds")
	}

	label.Clip, label.Clipped = gfx.Rect{X: 0, Y: 0, Width: 10, Height: 6}, true
	pixels = renderScene(t, offscreen, nil, []platform.TextLabel{label})
	if !lit(pixels, 5, 5) {
		t.Fatal("the clipped label lost the part inside its clip")
	}
	if lit(pixels, 15, 5) {
		t.Fatal("the clipped label painted right of its clip")
	}
	if lit(pixels, 5, 7) {
		t.Fatal("the clipped label painted below its clip")
	}

	label.Clip = gfx.Rect{X: 30, Y: 0, Width: 5, Height: 5}
	pixels = renderScene(t, offscreen, nil, []platform.TextLabel{label})
	if lit(pixels, 5, 5) || lit(pixels, 15, 5) {
		t.Fatal("a clip outside the target still painted")
	}
	label.Clipped = false
	pixels = renderScene(t, offscreen, nil, []platform.TextLabel{label})
	if !lit(pixels, 15, 5) {
		t.Fatal("the scissor stayed set for the next frame")
	}
}

func TestOffscreenRendersShapedTextAndClipsIt(t *testing.T) {
	offscreen := newTestOffscreen(t, 200, 40)
	label := platform.TextLabel{
		Value: "Hello Hello Hello", X: 4, Y: 4, Width: 190, Height: 24, FontSize: 16,
		Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
	}
	count := func(pixels *image.RGBA, x0, x1 int) int {
		total := 0
		for y := 0; y < 40; y++ {
			for x := x0; x < x1; x++ {
				if isPainted(pixels, x, y) {
					total++
				}
			}
		}
		return total
	}
	pixels, err := func() (*image.RGBA, error) {
		if err := offscreen.Set(nil, []platform.TextLabel{label}); err != nil {
			return nil, err
		}
		return offscreen.Image()
	}()
	if err != nil {
		t.Skipf("text is not available here: %v", err)
	}
	if count(pixels, 0, 100) < 30 || count(pixels, 100, 200) < 30 {
		t.Fatal("the label drew no glyph pixels across its width")
	}
	label.Clip, label.Clipped = gfx.Rect{X: 0, Y: 0, Width: 100, Height: 40}, true
	pixels = renderScene(t, offscreen, nil, []platform.TextLabel{label})
	if count(pixels, 0, 100) < 30 {
		t.Fatal("the clipped label lost the part inside its clip")
	}
	if count(pixels, 100, 200) != 0 {
		t.Fatal("the clipped label painted right of its clip")
	}
}

func TestOffscreenIsDeterministicAndRerenderable(t *testing.T) {
	offscreen := newTestOffscreen(t, 30, 30)
	rectangles := []render.Rectangle{
		{Bounds: scene.Rect{X: 3, Y: 3, Width: 20, Height: 20}, Radius: 6, Color: scene.Color{Red: 0.2, Green: 0.6, Blue: 0.9, Alpha: 1}},
	}
	first := renderScene(t, offscreen, rectangles, nil)
	again, err := offscreen.Image()
	if err != nil {
		t.Fatal(err)
	}
	if string(first.Pix) != string(again.Pix) {
		t.Fatal("two renders of one scene differ")
	}
	// A new scene replaces the old one entirely.
	empty := renderScene(t, offscreen, nil, nil)
	if isPainted(empty, 10, 10) {
		t.Fatal("the previous scene is still drawn after Set")
	}
}

func TestOffscreenImageIsLogicalSizeTimesScaleRoundedUp(t *testing.T) {
	for _, test := range []struct {
		scale         float64
		width, height int
	}{{1, 21, 11}, {1.5, 32, 17}, {2, 42, 22}} {
		offscreen := newScaledOffscreen(t, 21, 11, test.scale)
		pixels := renderScene(t, offscreen, nil, nil)
		if got := pixels.Bounds(); got != image.Rect(0, 0, test.width, test.height) {
			t.Errorf("21x11 at %gx = %v, want %dx%d", test.scale, got, test.width, test.height)
		}
	}
	for _, scale := range []float64{0.5, 0, 9} {
		if _, err := NewOffscreen(10, 10, scale, platform.Color{}); err == nil {
			t.Errorf("scale %g was accepted", scale)
		}
	}
}

// A shape is positioned in logical pixels and covers its logical size times the
// scale in device pixels: the same layout, more pixels.
func TestOffscreenShapesCoverTheirLogicalSizeTimesTheScale(t *testing.T) {
	white := scene.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	for _, scale := range []float64{1.5, 2} {
		offscreen := newScaledOffscreen(t, 40, 40, scale)
		pixels := renderScene(t, offscreen, []render.Rectangle{{Bounds: scene.Rect{X: 10, Y: 10, Width: 10, Height: 10}, Color: white}}, nil)
		left, right := int(10*scale), int(20*scale)
		for _, test := range []struct {
			x, y int
			want bool
		}{
			{left - 1, left + 1, false}, {left, left, true}, {right - 1, right - 1, true}, {right, left, false}, {left, right, false},
		} {
			if got := isPainted(pixels, test.x, test.y); got != test.want {
				t.Errorf("%gx: pixel (%d,%d) painted = %v, want %v", scale, test.x, test.y, got, test.want)
			}
		}
		// The edge is one device pixel of anti-aliasing at most, at any scale.
		if r, _, _, _ := rgbaAt(pixels, left, left+3); r != 255 {
			t.Errorf("%gx: the first covered column is not solid: red %d", scale, r)
		}
	}
}

// A rounded edge feathers over one device pixel at any scale, so it does not
// get softer on a denser screen.
func TestOffscreenRoundedEdgeIsOneDevicePixelWideAtEveryScale(t *testing.T) {
	white := scene.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	for _, scale := range []float64{1, 1.5, 2} {
		offscreen := newScaledOffscreen(t, 60, 60, scale)
		pixels := renderScene(t, offscreen, []render.Rectangle{{Bounds: scene.Rect{X: 10, Y: 10, Width: 40, Height: 40}, Radius: 8, Color: white}}, nil)
		partial := 0
		y := int(30 * scale)
		for x := 0; x < pixels.Bounds().Dx(); x++ {
			if r, _, _, _ := rgbaAt(pixels, x, y); r > 8 && r < 247 {
				partial++
			}
		}
		if partial > 2 {
			t.Errorf("%gx: %d partly covered pixels along a straight edge row, want at most one at each side", scale, partial)
		}
	}
}

// A clip edge in logical pixels lands on the nearest device pixel, as on the
// other backends.
func TestOffscreenClipSnapsToDevicePixelsAtAFractionalScale(t *testing.T) {
	white := scene.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	offscreen := newScaledOffscreen(t, 40, 20, 1.5)
	// 10.6 * 1.5 = 15.9 and (10.6 + 9.8) * 1.5 = 30.6 round to 16 and 31.
	pixels := renderScene(t, offscreen, []render.Rectangle{{
		Bounds: scene.Rect{X: 0, Y: 0, Width: 40, Height: 20}, Color: white,
		Clipped: true, Clip: scene.Rect{X: 10.6, Y: 2, Width: 9.8, Height: 10},
	}}, nil)
	for _, test := range []struct {
		x    int
		want bool
	}{{14, false}, {15, false}, {16, true}, {30, true}, {31, false}, {32, false}} {
		if got := isPainted(pixels, test.x, 10); got != test.want {
			t.Errorf("pixel column %d painted = %v, want %v", test.x, got, test.want)
		}
	}
}

// inkBounds is the bounding box of the pixels a label painted.
func inkBounds(pixels *image.RGBA) image.Rectangle {
	box := image.Rectangle{Min: image.Pt(1<<20, 1<<20)}
	for y := 0; y < pixels.Bounds().Dy(); y++ {
		for x := 0; x < pixels.Bounds().Dx(); x++ {
			if isPainted(pixels, x, y) {
				box.Min.X, box.Min.Y = min(box.Min.X, x), min(box.Min.Y, y)
				box.Max.X, box.Max.Y = max(box.Max.X, x+1), max(box.Max.Y, y+1)
			}
		}
	}
	return box
}

// Text is rasterized at the device scale: the same line is as many times larger
// as the scale says, and not blurred up from a 1x bitmap.
func TestOffscreenTextIsRasterizedAtTheDeviceScale(t *testing.T) {
	label := platform.TextLabel{
		Value: "Hamburgefonstiv", X: 4, Y: 4, Width: 150, Height: 24, FontSize: 16,
		Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
	}
	boxes := map[float64]image.Rectangle{}
	for _, scale := range []float64{1, 1.5, 2} {
		offscreen := newScaledOffscreen(t, 160, 32, scale)
		if err := offscreen.Set(nil, []platform.TextLabel{label}); err != nil {
			t.Skipf("text is not available here: %v", err)
		}
		pixels, err := offscreen.Image()
		if err != nil {
			t.Fatal(err)
		}
		boxes[scale] = inkBounds(pixels)
		if boxes[scale].Empty() {
			t.Fatalf("%gx: the label drew nothing", scale)
		}
	}
	for _, scale := range []float64{1.5, 2} {
		for _, axis := range []struct {
			name     string
			one, got int
		}{
			{"width", boxes[1].Dx(), boxes[scale].Dx()},
			{"height", boxes[1].Dy(), boxes[scale].Dy()},
		} {
			// The 1x ink is a whole number of pixels, so scaling it up carries that
			// rounding: up to one 1x pixel at this scale on top of the 7 percent.
			want := float64(axis.one) * scale
			if math.Abs(float64(axis.got)-want) > 0.07*want+scale {
				t.Errorf("%gx: ink %s = %d, want about %.0f (1x was %d)", scale, axis.name, axis.got, want, axis.one)
			}
		}
	}
}

// The canvas clip of a label is cut at device pixels at a fractional scale and
// does not move the label's glyphs.
func TestOffscreenScissorsAClippedLabelAtAFractionalScale(t *testing.T) {
	offscreen := newScaledOffscreen(t, 20, 10, 1.5)
	label := bitmapLabel()
	label.Clip, label.Clipped = gfx.Rect{X: 0, Y: 0, Width: 10, Height: 6}, true
	pixels := renderScene(t, offscreen, nil, []platform.TextLabel{label})
	lit := func(x, y int) bool {
		_, _, _, alpha := rgbaAt(pixels, x, y)
		return alpha > 200 && isPainted(pixels, x, y)
	}
	// 10 logical pixels are 15 device pixels, 6 are 9.
	if !lit(14, 8) {
		t.Error("the part inside the clip is missing")
	}
	if lit(15, 8) || lit(14, 9) {
		t.Error("the label painted outside its clip")
	}
}

// A shadow darkens only what lies outside its caster, fades to nothing within
// the reach it declares, and is deeper on the side its offset pushes it to.
func TestOffscreenShadowFadesAwayOutsideItsCaster(t *testing.T) {
	pixels := rectanglesScene(t, 200, []render.Rectangle{
		{Kind: scene.KindRectangle, Bounds: scene.Rect{Width: 200, Height: 200}, Color: scene.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}},
		{
			Kind: scene.KindShadow, Bounds: scene.Rect{X: 60, Y: 60, Width: 80, Height: 80}, Radius: 8,
			Color: scene.Color{Alpha: 0.5}, Shadow: scene.Shadow{OffsetY: 4, Blur: 12},
		},
	})
	level := func(x, y int) int { return int(func() byte { _, g, _, _ := rgbaAt(pixels, x, y); return g }()) }
	if level(100, 100) != 255 {
		t.Errorf("the caster's inside = %d, want it left untouched", level(100, 100))
	}
	if level(100, 143) > 200 {
		t.Errorf("just under the caster = %d, want a clearly darkened pixel", level(100, 143))
	}
	if level(100, 170) != 255 || level(30, 100) != 255 || level(100, 30) != 255 {
		t.Errorf("beyond the reach: %d %d %d, want untouched", level(100, 170), level(30, 100), level(100, 30))
	}
	for y := 141; y < 160; y++ {
		if level(100, y) > level(100, y+1) {
			t.Errorf("shadow below the caster is not fading away at y=%d: %d then %d", y, level(100, y), level(100, y+1))
		}
	}
	if level(100, 143) >= level(100, 57) {
		t.Errorf("shadow below (%d) is not deeper than above (%d) although it is offset downwards", level(100, 143), level(100, 57))
	}
}
