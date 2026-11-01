package raster

import (
	"testing"

	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

func rect(x, y, w, h float32, c scene.Color) render.Rectangle {
	return render.Rectangle{Kind: scene.KindRectangle, Bounds: scene.Rect{X: x, Y: y, Width: w, Height: h}, Color: c}
}

func at(c *Canvas, x, y int) [4]byte {
	i := y*c.Stride + x*4
	return [4]byte{c.Pix[i], c.Pix[i+1], c.Pix[i+2], c.Pix[i+3]}
}

var (
	red   = scene.Color{Red: 1, Alpha: 1}
	blue  = scene.Color{Blue: 1, Alpha: 1}
	white = scene.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
)

func TestRectangleCoversTheDevicePixelsWhoseCentersItContains(t *testing.T) {
	c := NewCanvas(20, 10)
	var r Rasterizer
	r.Draw(c, Params{Scale: 1.5, Clear: white}, []render.Rectangle{rect(2, 1, 4, 3, red)}, nil)
	// 2..6 logical is 3..9 device pixels, and 1..4 is 1.5..6: centers 1.5 .. 5.5 start at y=2 (center 2.5)
	// because the center of row 1 (1.5) is on the top edge and an edge belongs to the pixel below it.
	for y := 0; y < 10; y++ {
		for x := 0; x < 20; x++ {
			want := white
			if x >= 3 && x < 9 && y >= 1 && y < 6 {
				want = red
			}
			got := at(c, x, y)
			if got != [4]byte{unorm8(want.Blue), unorm8(want.Green), unorm8(want.Red), 255} {
				t.Fatalf("pixel %d,%d = %v, want %+v", x, y, got, want)
			}
		}
	}
}

func TestTranslucentColorBlendsOverWhatIsThere(t *testing.T) {
	c := NewCanvas(4, 4)
	var r Rasterizer
	half := scene.Color{Blue: 1, Alpha: 0.5}
	r.Draw(c, Params{Scale: 1, Clear: white}, []render.Rectangle{rect(0, 0, 4, 4, half)}, nil)
	// premultiplied (0, 0, 0.5, 0.5) over white: blue stays 255, red and green fall to 127.5 -> 128
	if got := at(c, 1, 1); got != [4]byte{255, 128, 128, 255} {
		t.Fatalf("blended = %v", got)
	}
}

func TestHigherLayerIsOnTopWhateverTheOrderOfTheList(t *testing.T) {
	top := rect(0, 0, 4, 4, red)
	top.Layer = 2
	under := rect(0, 0, 4, 4, blue)
	under.Layer = 1
	c := NewCanvas(4, 4)
	var r Rasterizer
	r.Draw(c, Params{Scale: 1}, []render.Rectangle{top, under}, nil)
	if got := at(c, 2, 2); got != [4]byte{0, 0, 255, 255} {
		t.Fatalf("pixel = %v, want red on top", got)
	}
}

func TestClippedInstanceStaysInsideItsClip(t *testing.T) {
	in := rect(0, 0, 10, 10, red)
	in.Clipped, in.Clip = true, scene.Rect{X: 2, Y: 2, Width: 3, Height: 3}
	c := NewCanvas(10, 10)
	var r Rasterizer
	r.Draw(c, Params{Scale: 1}, []render.Rectangle{in}, nil)
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			inside := x >= 2 && x < 5 && y >= 2 && y < 5
			if (at(c, x, y)[3] != 0) != inside {
				t.Fatalf("pixel %d,%d painted=%v, want %v", x, y, at(c, x, y)[3] != 0, inside)
			}
		}
	}
}

func TestOnlyTheDamagedPartIsTouched(t *testing.T) {
	c := NewCanvas(8, 8)
	var r Rasterizer
	r.Draw(c, Params{Scale: 1, Clear: white}, []render.Rectangle{rect(0, 0, 8, 8, blue)}, nil)
	before := append([]byte(nil), c.Pix...)
	r.Draw(c, Params{Scale: 1, Clear: white, Damage: render.Scissor{Left: 2, Top: 2, Right: 4, Bottom: 4}},
		[]render.Rectangle{rect(0, 0, 8, 8, red)}, nil)
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			damaged := x >= 2 && x < 4 && y >= 2 && y < 4
			want := at(&Canvas{Pix: before, Stride: c.Stride}, x, y)
			if damaged {
				want = [4]byte{0, 0, 255, 255}
			}
			if got := at(c, x, y); got != want {
				t.Fatalf("pixel %d,%d = %v, want %v", x, y, got, want)
			}
		}
	}
}

func TestRoundedRectangleIsSymmetricAndFeathered(t *testing.T) {
	c := NewCanvas(40, 40)
	var r Rasterizer
	in := rect(4, 4, 32, 32, red)
	in.Radius = 10
	r.Draw(c, Params{Scale: 1, Clear: white}, []render.Rectangle{in}, nil)
	if got := at(c, 20, 20); got != [4]byte{0, 0, 255, 255} {
		t.Fatalf("center = %v, want solid red", got)
	}
	if got := at(c, 4, 4); got != [4]byte{255, 255, 255, 255} {
		t.Fatalf("corner pixel = %v, want the background (outside the arc)", got)
	}
	for _, p := range [][2]int{{5, 12}, {6, 7}, {8, 5}} {
		a, b := at(c, p[0], p[1]), at(c, 39-p[0], 39-p[1])
		if a != b {
			t.Fatalf("pixel %v = %v but its mirror = %v", p, a, b)
		}
	}
	partial := false
	for y := 4; y < 14; y++ {
		for x := 4; x < 14; x++ {
			if g := at(c, x, y)[1]; g != 0 && g != 255 {
				partial = true
			}
		}
	}
	if !partial {
		t.Fatal("no feathered pixel on the corner arc")
	}
}

// Two triangles that share an edge must own every pixel exactly once: no crack, no double paint.
func TestTrianglesSharingAnEdgeMeetWithoutGapOrOverlap(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		c := NewCanvas(60, 60)
		translucent := scene.Color{Red: 1, Alpha: 0.5}
		tri := func(a, b, d scene.Point, mask uint8) render.Rectangle {
			return render.Rectangle{Kind: scene.KindTriangle, Points: [3]scene.Point{a, b, d}, Color: translucent, EdgeMask: mask}
		}
		p00, p10, p01, p11 := scene.Point{X: 3.3, Y: 4.1}, scene.Point{X: 31.7, Y: 4.1}, scene.Point{X: 3.3, Y: 29.9}, scene.Point{X: 31.7, Y: 29.9}
		// The diagonal p10-p01 is shared: edge 1 of the first triangle (b to c) and edge 2 of the second
		// (c to a) in these windings are left out of the masks.
		first := tri(p00, p10, p01, 0b101)
		second := tri(p10, p11, p01, 0b011)
		var r Rasterizer
		r.Draw(c, Params{Scale: scale}, []render.Rectangle{first, second}, nil)
		for y := 0; y < c.Height; y++ {
			for x := 0; x < c.Width; x++ {
				a := at(c, x, y)[3]
				// translucent 0.5 once is 128; twice would be 192
				if a > 128 {
					t.Fatalf("scale %v: pixel %d,%d painted twice (alpha %d)", scale, x, y, a)
				}
			}
		}
		// Every pixel strictly inside the square is painted once.
		for y := int(6 * scale); y < int(28*scale); y++ {
			for x := int(6 * scale); x < int(30*scale); x++ {
				if at(c, x, y)[3] != 128 {
					t.Fatalf("scale %v: pixel %d,%d inside the quad has alpha %d, want 128 (a gap)", scale, x, y, at(c, x, y)[3])
				}
			}
		}
	}
}

func TestShadowIsDarkestNearItsEdgeAndHasTheCasterCutOut(t *testing.T) {
	c := NewCanvas(80, 60)
	var r Rasterizer
	sh := render.Rectangle{Kind: scene.KindShadow, Bounds: scene.Rect{X: 20, Y: 15, Width: 40, Height: 20}, Radius: 4,
		Color: scene.Color{Alpha: 0.6}, Shadow: scene.Shadow{OffsetY: 4, Blur: 8, Spread: 0}}
	r.Draw(c, Params{Scale: 1, Clear: white}, []render.Rectangle{sh}, nil)
	if got := at(c, 40, 25); got != [4]byte{255, 255, 255, 255} {
		t.Fatalf("inside the casting rectangle = %v, want the background (cut out)", got)
	}
	below, far := at(c, 40, 38), at(c, 40, 54)
	if !(below[0] < 250 && far[0] > below[0]) {
		t.Fatalf("shadow below the caster %v is not darker than far away %v", below, far)
	}
	if at(c, 2, 2) != [4]byte{255, 255, 255, 255} {
		t.Fatal("shadow reached past its extent")
	}
}

func TestLabelIsBlendedOverAndCutToItsClip(t *testing.T) {
	c := NewCanvas(6, 4)
	var r Rasterizer
	img := make([]byte, 4*2*4)
	for i := 0; i < len(img); i += 4 {
		img[i], img[i+1], img[i+2], img[i+3] = 0, 0, 128, 128 // premultiplied half red
	}
	label := Label{Pix: img, Stride: 16, Width: 4, Height: 2, X: 1, Y: 1, Clipped: true, Clip: render.Scissor{Left: 2, Top: 0, Right: 4, Bottom: 4}}
	r.Draw(c, Params{Scale: 1, Clear: white}, nil, []Label{label})
	if got := at(c, 1, 1); got != [4]byte{255, 255, 255, 255} {
		t.Fatalf("pixel left of the clip = %v, want the background", got)
	}
	if got := at(c, 2, 1); got != [4]byte{127, 127, 255, 255} {
		t.Fatalf("blended label pixel = %v", got)
	}
}

// The diagonal below passes exactly through pixel centers, where only the top-left rule decides which
// triangle owns the pixel.
func TestSharedEdgeThroughPixelCentersIsOwnedByOneTriangle(t *testing.T) {
	c := NewCanvas(30, 30)
	translucent := scene.Color{Green: 1, Alpha: 0.5}
	tri := func(a, b, d scene.Point, mask uint8) render.Rectangle {
		return render.Rectangle{Kind: scene.KindTriangle, Points: [3]scene.Point{a, b, d}, Color: translucent, EdgeMask: mask}
	}
	p00, p10, p01, p11 := scene.Point{X: 0.5, Y: 0.5}, scene.Point{X: 20.5, Y: 0.5}, scene.Point{X: 0.5, Y: 20.5}, scene.Point{X: 20.5, Y: 20.5}
	var r Rasterizer
	r.Draw(c, Params{Scale: 1}, []render.Rectangle{tri(p00, p10, p01, 0b101), tri(p10, p11, p01, 0b011)}, nil)
	for i := 1; i < 20; i++ { // the pixels on the diagonal x+y=20
		if a := at(c, i, 20-i)[3]; a != 128 {
			t.Fatalf("diagonal pixel %d,%d has alpha %d, want one coat (128)", i, 20-i, a)
		}
	}
}

func TestAHoleErasesItsLayerAndBelowButNotWhatIsAbove(t *testing.T) {
	c := NewCanvas(12, 6)
	var r Rasterizer
	page := rect(0, 0, 12, 6, white)
	page.Layer = 1
	popover := rect(4, 1, 4, 3, red)
	popover.Layer = 2
	hole := Hole{Box: render.Scissor{Left: 2, Top: 1, Right: 10, Bottom: 5}, Layer: 1}
	r.Draw(c, Params{Scale: 1, Clear: white, Holes: []Hole{hole}}, []render.Rectangle{page, popover}, nil)
	if got := at(c, 1, 1); got != [4]byte{255, 255, 255, 255} {
		t.Fatalf("outside the hole = %v", got)
	}
	if got := at(c, 3, 1); got != [4]byte{} {
		t.Fatalf("inside the hole = %v, want transparent", got)
	}
	if got := at(c, 5, 2); got != [4]byte{0, 0, 255, 255} {
		t.Fatalf("a layer above the hole = %v, want red", got)
	}
}

func TestAHoleInALayerWithNothingElseStillErases(t *testing.T) {
	c := NewCanvas(4, 4)
	var r Rasterizer
	hole := Hole{Box: render.Scissor{Right: 2, Bottom: 2}, Layer: 7}
	r.Draw(c, Params{Scale: 1, Clear: white, Holes: []Hole{hole}}, []render.Rectangle{rect(0, 0, 4, 4, red)}, nil)
	if at(c, 1, 1) != ([4]byte{}) || at(c, 3, 3) == ([4]byte{}) {
		t.Fatalf("hole pixels %v and %v", at(c, 1, 1), at(c, 3, 3))
	}
}
