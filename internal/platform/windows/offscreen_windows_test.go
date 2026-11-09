package windows

import (
	"image"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

var black = platform.Color{Alpha: 1}

func newOffscreen(t testing.TB, width, height int, scale float64) *Offscreen {
	t.Helper()
	offscreen, err := NewOffscreen(width, height, scale, black)
	if err != nil {
		t.Skipf("no Direct3D 11 device: %v", err)
	}
	t.Cleanup(offscreen.Close)
	return offscreen
}

func renderScene(t testing.TB, offscreen *Offscreen, rectangles []render.Rectangle, labels []platform.TextLabel) *image.RGBA {
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

// clipScene renders the instances at the given scale on a size x size scene
// cleared to black.
func clipScene(t *testing.T, size int, scale float64, rectangles []render.Rectangle) *image.RGBA {
	t.Helper()
	return renderScene(t, newOffscreen(t, size, size, scale), rectangles, nil)
}

func isPainted(pixels *image.RGBA, x, y int) bool {
	color := pixels.RGBAAt(x, y)
	return int(color.R)+int(color.G)+int(color.B) > 60
}

func TestOffscreenLayersPaintOverEarlierInstancesAtAnyOrder(t *testing.T) {
	offscreen := newOffscreen(t, 100, 50, 2)
	pixels := renderScene(t, offscreen, []render.Rectangle{
		{Kind: scene.KindRectangle, Bounds: scene.Rect{X: 10, Y: 10, Width: 40, Height: 20}, Color: scene.Color{Red: 1, Alpha: 1}},
		// A later layer paints over an earlier one regardless of instance order.
		{Kind: scene.KindRectangle, Bounds: scene.Rect{X: 30, Y: 10, Width: 40, Height: 20}, Color: scene.Color{Green: 1, Alpha: 1}, Layer: 1},
		{Kind: scene.KindRectangle, Bounds: scene.Rect{X: 30, Y: 10, Width: 40, Height: 20}, Color: scene.Color{Blue: 1, Alpha: 1}},
	}, nil)
	if got := pixels.Bounds().Size(); got != (image.Point{200, 100}) {
		t.Fatalf("size = %v device pixels, want 200x100 at scale 2", got)
	}
	for _, test := range []struct {
		name string
		x, y int
		want [3]uint8
	}{
		{"red rectangle", 40, 40, [3]uint8{255, 0, 0}},
		{"layer 1 over layer 0", 120, 40, [3]uint8{0, 255, 0}},
		{"blue beyond the layer", 130, 40, [3]uint8{0, 255, 0}},
		{"clear color", 180, 10, [3]uint8{0, 0, 0}},
	} {
		if got := pixels.RGBAAt(test.x, test.y); got.R != test.want[0] || got.G != test.want[1] || got.B != test.want[2] || got.A != 255 {
			t.Errorf("%s: pixel (%d,%d) = %v, want %v", test.name, test.x, test.y, got, test.want)
		}
	}
}

func TestOffscreenTriangleMeshHasNoInternalFeathering(t *testing.T) {
	white := scene.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	pixels := clipScene(t, 100, 2, []render.Rectangle{
		{
			Kind: scene.KindTriangle, Color: white, EdgeMask: 1<<0 | 1<<1,
			Points: [3]scene.Point{{X: 10, Y: 10}, {X: 90, Y: 10}, {X: 90, Y: 90}},
		},
		{
			Kind: scene.KindTriangle, Color: white, EdgeMask: 1<<1 | 1<<2,
			Points: [3]scene.Point{{X: 10, Y: 10}, {X: 90, Y: 90}, {X: 10, Y: 90}},
		},
	})
	for coordinate := 40; coordinate < 160; coordinate += 7 {
		if got := pixels.RGBAAt(coordinate, coordinate); got.R < 250 || got.G < 250 || got.B < 250 || got.A != 255 {
			t.Fatalf("shared diagonal pixel %d = %v, want opaque white", coordinate, got)
		}
	}
}

func TestOffscreenShadedTriangleInterpolatesItsCornerColors(t *testing.T) {
	pixels := clipScene(t, 120, 1, []render.Rectangle{{
		Kind: scene.KindTriangle, Color: scene.Color{Red: 1, Alpha: 1},
		Shade:  scene.Shade{On: true, Colors: [2]scene.Color{{Green: 1, Alpha: 1}, {Blue: 1, Alpha: 1}}},
		Points: [3]scene.Point{{X: 10, Y: 10}, {X: 110, Y: 10}, {X: 10, Y: 110}},
	}})
	for _, at := range [][2]int{{20, 20}, {60, 20}, {20, 60}, {44, 44}, {30, 70}} {
		w1, w2 := (float64(at[0])+0.5-10)/100, (float64(at[1])+0.5-10)/100
		want := [3]float64{(1 - w1 - w2) * 255, w1 * 255, w2 * 255}
		got := pixels.RGBAAt(at[0], at[1])
		have := [3]float64{float64(got.R), float64(got.G), float64(got.B)}
		for channel := range have {
			if diff := have[channel] - want[channel]; diff > 8 || diff < -8 {
				t.Fatalf("pixel %v = rgb(%v), want about rgb(%.0f, %.0f, %.0f)", at, have, want[0], want[1], want[2])
			}
		}
		if got.A != 255 {
			t.Fatalf("pixel %v alpha = %d", at, got.A)
		}
	}
}

func TestOffscreenRoundedRectangleCutsItsCorners(t *testing.T) {
	pixels := clipScene(t, 100, 1, []render.Rectangle{{
		Bounds: scene.Rect{X: 10, Y: 10, Width: 80, Height: 80}, Radius: 20, Color: scene.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
	}})
	if isPainted(pixels, 11, 11) || isPainted(pixels, 88, 88) {
		t.Fatal("the corners are not rounded")
	}
	if !isPainted(pixels, 50, 11) || !isPainted(pixels, 11, 50) || !isPainted(pixels, 30, 30) {
		t.Fatal("the edges or the interior are missing")
	}
}

func TestOffscreenClipCutsARoundedRectangleOnlyWhereTheClipIs(t *testing.T) {
	white := scene.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	// The rectangle spans 10..90; the clip cuts its right and bottom parts away
	// at 60 but leaves its top-left corner (radius 20) in place.
	pixels := clipScene(t, 100, 1, []render.Rectangle{{
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
	pixels := clipScene(t, 100, 1, []render.Rectangle{{
		Kind: scene.KindTriangle, Color: scene.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}, EdgeMask: 7,
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
	whole := scene.Rect{X: 0, Y: 0, Width: 100, Height: 100}
	pixels := clipScene(t, 100, 1, []render.Rectangle{
		{Bounds: whole, Color: red, Clipped: true, Clip: scene.Rect{X: 0, Y: 0, Width: 40, Height: 100}},
		{Bounds: whole, Color: red, Clipped: true, Clip: scene.Rect{X: 60, Y: 0, Width: 40, Height: 100}},
		// A later, unclipped rectangle paints over the earlier clipped ones.
		{Bounds: scene.Rect{X: 20, Y: 20, Width: 20, Height: 20}, Color: scene.Color{Green: 1, Alpha: 1}},
	})
	if isPainted(pixels, 50, 50) {
		t.Fatal("the gap between the two clips is painted")
	}
	if pixels.RGBAAt(10, 80).R < 250 {
		t.Fatal("the first clip's rectangle is missing")
	}
	if pixels.RGBAAt(90, 80).R < 250 {
		t.Fatal("the second clip's rectangle is missing")
	}
	if got := pixels.RGBAAt(30, 30); got.G < 250 || got.R > 5 || got.B > 5 {
		t.Fatalf("the later rectangle does not paint over the earlier one: %v", got)
	}
}

func TestOffscreenClipLandsOnTheNearestDevicePixelAtTwoTimes(t *testing.T) {
	// At 2x the clip edges 10.3 and 20.6 are device pixels 20.6 and 41.2, which
	// round to 21 and 41: pixels 21..40 are painted and 20 and 41 are not.
	pixels := clipScene(t, 50, 2, []render.Rectangle{{
		Bounds: scene.Rect{X: 0, Y: 0, Width: 50, Height: 50}, Color: scene.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
		Clipped: true, Clip: scene.Rect{X: 10.3, Y: 10.3, Width: 10.3, Height: 10.3},
	}})
	for _, test := range []struct {
		x, y int
		want bool
	}{
		{20, 30, false}, {21, 30, true}, {40, 30, true}, {41, 30, false},
		{30, 20, false}, {30, 21, true}, {30, 40, true}, {30, 41, false},
	} {
		if got := isPainted(pixels, test.x, test.y); got != test.want {
			t.Errorf("pixel (%d,%d) painted = %v, want %v", test.x, test.y, got, test.want)
		}
	}
}

func TestOffscreenDrawsPremultipliedBGRABitmapPixels(t *testing.T) {
	offscreen := newOffscreen(t, 20, 10, 1)
	pixels := renderScene(t, offscreen, nil, []platform.TextLabel{{
		X: 2, Y: 2, Width: 16, Height: 6,
		Bitmap: []byte{0, 0, 255, 255, 0, 255, 0, 255}, BitmapWidth: 2, BitmapHeight: 1, BitmapStride: 8, BitmapID: 1,
	}})
	if got := pixels.RGBAAt(5, 5); got.R < 240 || got.G > 5 || got.B > 5 || got.A < 240 {
		t.Fatalf("left pixel = %v, want red", got)
	}
	if got := pixels.RGBAAt(15, 5); got.G < 240 || got.R > 5 || got.B > 5 || got.A < 240 {
		t.Fatalf("right pixel = %v, want green", got)
	}
}

// A label a canvas clip cuts partway is painted only inside the clip, at the
// target's own scale; the rest of the same quad stays untouched.
func TestOffscreenScissorsAClippedLabelToItsClip(t *testing.T) {
	offscreen := newOffscreen(t, 20, 10, 2)
	label := platform.TextLabel{
		X: 2, Y: 2, Width: 16, Height: 6,
		Bitmap: []byte{0, 0, 255, 255, 0, 255, 0, 255}, BitmapWidth: 2, BitmapHeight: 1, BitmapStride: 8, BitmapID: 1,
	}
	lit := func(pixels *image.RGBA, x, y int) bool {
		color := pixels.RGBAAt(x*2, y*2) // logical to device
		return color.A > 200 && (color.R > 200 || color.G > 200 || color.B > 200)
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

	// A clip with no area paints nothing, and the next frame without a clip
	// paints the whole label again: the scissor does not leak.
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

func TestOffscreenRasterizesTextLabelsAndClipsThem(t *testing.T) {
	offscreen := newOffscreen(t, 200, 40, 1)
	label := platform.TextLabel{Value: "Hello Hello", X: 10, Y: 10, Width: 180, Height: 20, FontSize: 14, Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}}
	lit := func(pixels *image.RGBA, x0, x1 int) int {
		count := 0
		for y := 0; y < 40; y++ {
			for x := x0; x < x1; x++ {
				if isPainted(pixels, x, y) {
					count++
				}
			}
		}
		return count
	}
	pixels := renderScene(t, offscreen, nil, []platform.TextLabel{label})
	if lit(pixels, 10, 40) < 30 || lit(pixels, 40, 190) < 30 {
		t.Fatal("the label drew no glyph pixels")
	}
	label.Clip, label.Clipped = gfx.Rect{X: 0, Y: 0, Width: 40, Height: 40}, true
	pixels = renderScene(t, offscreen, nil, []platform.TextLabel{label})
	if lit(pixels, 10, 40) < 30 {
		t.Fatal("the clipped label lost the part inside its clip")
	}
	if lit(pixels, 40, 200) != 0 {
		t.Fatal("the clipped label painted right of its clip")
	}
}

// The antialiased band of a rounded edge is one device pixel wide at any scale,
// as on the other platforms: a pixel whose center is half a device pixel outside
// an edge is untouched and one half a device pixel inside is fully covered. A band
// fixed in logical pixels smeared every edge over three device pixels at 2x.
func TestRoundedEdgesAreFeatheredOverOneDevicePixel(t *testing.T) {
	for _, scale := range []float64{1, 2} {
		offscreen := newOffscreen(t, 40, 40, scale)
		pixels := renderScene(t, offscreen, []render.Rectangle{
			{Kind: scene.KindRectangle, Bounds: scene.Rect{X: 10, Y: 10, Width: 20, Height: 20}, Color: scene.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}, Radius: 4},
		}, nil)
		left := int(10 * scale)
		middle := int(20 * scale)
		for _, test := range []struct {
			name string
			x    int
			full bool
		}{{"just outside", left - 1, false}, {"just inside", left, true}} {
			got := pixels.RGBAAt(test.x, middle)
			if test.full && got.R != 255 || !test.full && got.R != 0 {
				t.Errorf("scale %g, %s the left edge: red = %d, want %v", scale, test.name, got.R, map[bool]int{true: 255, false: 0}[test.full])
			}
		}
	}
}

// A shadow darkens only what lies outside its caster, fades to nothing within
// the reach it declares, and is deeper on the side its offset pushes it to.
func TestOffscreenShadowFadesAwayOutsideItsCaster(t *testing.T) {
	pixels := clipScene(t, 200, 1, []render.Rectangle{
		{Kind: scene.KindRectangle, Bounds: scene.Rect{Width: 200, Height: 200}, Color: scene.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}},
		{
			Kind: scene.KindShadow, Bounds: scene.Rect{X: 60, Y: 60, Width: 80, Height: 80}, Radius: 8,
			Color: scene.Color{Alpha: 0.5}, Shadow: scene.Shadow{OffsetY: 4, Blur: 12},
		},
	})
	level := func(x, y int) int { return int(pixels.RGBAAt(x, y).G) }
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
