package gfx

import (
	"math"
	"testing"
)

var testStrokeIcon = Icon{
	ViewBox: 24, Stroke: 2, Cap: CapRound, Join: JoinRound,
	Commands: []PathCommand{MoveTo(4, 12), LineTo(20, 12), MoveTo(14, 6), QuadTo(20, 12, 14, 18), MoveTo(12, 3), CubicTo(17, 3, 21, 7, 21, 12), ClosePath()},
}

var testLayeredIcon = Icon{
	ViewBox: 24,
	Layers: []IconLayer{
		{Fill: Color{Red: 0.9, Green: 0.2, Blue: 0.2, Alpha: 1}, Commands: []PathCommand{MoveTo(3, 3), LineTo(21, 3), LineTo(21, 21), LineTo(3, 21), ClosePath()}},
		{Fill: Color{Red: 1, Green: 1, Blue: 1, Alpha: 0.8}, FillRule: PathFillEvenOdd, Commands: []PathCommand{MoveTo(7, 7), LineTo(17, 7), LineTo(12, 17), ClosePath()}},
	},
}

// testWideIcon is a 4:1 horizontal mark: a filled bar and a stroked line.
var testWideIcon = Icon{
	ViewBox: 40, ViewHeight: 10, Stroke: 2, Cap: CapRound, Join: JoinRound,
	Commands: []PathCommand{MoveTo(24, 5), LineTo(36, 5)},
	Layers: []IconLayer{
		{Fill: Color{Red: 0.2, Green: 0.8, Blue: 0.4, Alpha: 1}, Commands: []PathCommand{MoveTo(0, 0), LineTo(20, 0), LineTo(20, 10), LineTo(0, 10), ClosePath()}},
	},
}

// A wide icon is scaled to fit and centered, not stretched to the bounds or
// reduced to the shorter side.
func TestWideIconKeepsItsAspectRatio(t *testing.T) {
	var canvas Canvas
	canvas.DrawIcon(testWideIcon, Rect{X: 100, Y: 200, Width: 80, Height: 80}, Color{Alpha: 1})
	minX, minY, maxX, maxY := float32(math.Inf(1)), float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.Inf(-1))
	for _, triangle := range canvas.Triangles() {
		for _, point := range triangle.Points {
			minX, minY = min(minX, point.X), min(minY, point.Y)
			maxX, maxY = max(maxX, point.X), max(maxY, point.Y)
		}
	}
	// 80 wide fits 40 units at 2 px each, so the 10 units are 20 px high, centered
	// in the 80 px slot: the shapes span y 230..250, from x 100 to the line's round
	// cap one unit past its end at 36.
	if math.Abs(float64(minX-100)) > 0.01 || math.Abs(float64(maxX-174)) > 0.5 || math.Abs(float64(minY-230)) > 0.01 || math.Abs(float64(maxY-250)) > 0.01 {
		t.Fatalf("the wide icon covers x %.2f..%.2f, y %.2f..%.2f; want x 100..174 and y 230..250", minX, maxX, minY, maxY)
	}
	canvas.Reset()
	canvas.DrawIcon(testWideIcon, Rect{X: 0, Y: 0, Width: 400, Height: 5}, Color{Alpha: 1})
	minX, minY, maxX, maxY = float32(math.Inf(1)), float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.Inf(-1))
	for _, triangle := range canvas.Triangles() {
		for _, point := range triangle.Points {
			minX, minY = min(minX, point.X), min(minY, point.Y)
			maxX, maxY = max(maxX, point.X), max(maxY, point.Y)
		}
	}
	// The 5 px height limits the scale to half a pixel per unit: the shapes fit a
	// 20 x 5 box, centered in the 400 px slot, not stretched across it.
	if maxY-minY > 5.01 || maxX-minX > 18.6 || minX < 189.9 {
		t.Fatalf("a 5 px high slot must shrink the wide icon, got x %.2f..%.2f, y %.2f..%.2f", minX, maxX, minY, maxY)
	}
}

// sameTriangles checks that two tessellations of one icon paint the same
// shape. The cache tessellates at the origin and the direct path at the icon's
// position, so the ear clipper sees polygons whose vertices differ in the last
// bits; where the outline has near-collinear vertices (the join arcs of a
// translucent stroke) it may then pick another of the equally valid
// triangulations, and which one depends on the CPU's float32 rounding (fused
// multiply-add on arm64, none on amd64). Both are the same polygon: the test
// accepts identical triangles, and otherwise requires identical covered area and
// identical coverage at a grid of sample points.
func sameTriangles(t *testing.T, got, want []Triangle) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("cached icon made %d triangles, direct made %d", len(got), len(want))
	}
	const tolerance = 2e-3
	identical := true
	for i := range got {
		if !colorsClose(got[i].Color, want[i].Color) || got[i].Shaded != want[i].Shaded ||
			got[i].Shaded && (!colorsClose(got[i].Shade[0], want[i].Shade[0]) || !colorsClose(got[i].Shade[1], want[i].Shade[1])) || got[i].Layer != want[i].Layer ||
			got[i].HasClip != want[i].HasClip || got[i].ClipOf != want[i].ClipOf {
			t.Fatalf("triangle %d differs apart from position: %+v vs %+v", i, got[i], want[i])
		}
		for p := range got[i].Points {
			if math.Abs(float64(got[i].Points[p].X-want[i].Points[p].X)) > tolerance || math.Abs(float64(got[i].Points[p].Y-want[i].Points[p].Y)) > tolerance {
				identical = false
			}
		}
		if identical && got[i].EdgeMask != want[i].EdgeMask {
			identical = false
		}
	}
	if identical {
		return
	}
	areaGot, areaWant := triangleArea(got), triangleArea(want)
	if math.Abs(areaGot-areaWant) > 1e-3*math.Max(areaGot, 1) {
		t.Fatalf("cached icon covers area %.4f, direct %.4f", areaGot, areaWant)
	}
	var minX, minY, maxX, maxY = math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, triangle := range want {
		for _, point := range triangle.Points {
			minX, maxX = math.Min(minX, float64(point.X)), math.Max(maxX, float64(point.X))
			minY, maxY = math.Min(minY, float64(point.Y)), math.Max(maxY, float64(point.Y))
		}
	}
	for x := minX; x < maxX; x += 0.0371 {
		for y := minY; y < maxY; y += 0.0371 {
			if a, b := triangleCoverage(got, x, y), triangleCoverage(want, x, y); a != b {
				t.Fatalf("at (%.3f, %.3f) the cached icon is covered %d times, the direct one %d", x, y, a, b)
			}
		}
	}
}

func colorsClose(a, b Color) bool {
	return math.Abs(float64(a.Red-b.Red)) < 1e-5 && math.Abs(float64(a.Green-b.Green)) < 1e-5 &&
		math.Abs(float64(a.Blue-b.Blue)) < 1e-5 && math.Abs(float64(a.Alpha-b.Alpha)) < 1e-5
}

func triangleArea(triangles []Triangle) float64 {
	total := 0.0
	for _, triangle := range triangles {
		p := triangle.Points
		total += math.Abs(float64(p[1].X-p[0].X)*float64(p[2].Y-p[0].Y)-float64(p[2].X-p[0].X)*float64(p[1].Y-p[0].Y)) / 2
	}
	return total
}

// triangleCoverage counts the triangles that contain the point.
func triangleCoverage(triangles []Triangle, x, y float64) int {
	count := 0
	for _, triangle := range triangles {
		p := triangle.Points
		side := func(a, b Point) float64 {
			return (float64(b.X)-float64(a.X))*(y-float64(a.Y)) - (float64(b.Y)-float64(a.Y))*(x-float64(a.X))
		}
		d1, d2, d3 := side(p[0], p[1]), side(p[1], p[2]), side(p[2], p[0])
		if d1 >= 0 && d2 >= 0 && d3 >= 0 || d1 <= 0 && d2 <= 0 && d3 <= 0 {
			count++
		}
	}
	return count
}

// DrawIcon replays a cached tessellation; it must produce what tessellating from
// scratch would, wherever the icon is, however large, in any tint.
func TestCachedIconsPaintLikeTheDirectPath(t *testing.T) {
	var cached, direct Canvas
	tints := []Color{{Red: 0.1, Green: 0.2, Blue: 0.3, Alpha: 1}, {Red: 0.1, Green: 0.2, Blue: 0.3, Alpha: 0.4}}
	for _, icon := range []Icon{testStrokeIcon, testLayeredIcon, testWideIcon} {
		for _, bounds := range []Rect{
			{X: 10, Y: 20, Width: 16, Height: 16}, {X: 333.5, Y: 701, Width: 16, Height: 16},
			{X: 5, Y: 5, Width: 24, Height: 40}, {X: 0.3, Y: 0.7, Width: 13, Height: 13},
		} {
			for _, tint := range tints {
				for pass := 0; pass < 2; pass++ { // the second is a cache hit
					cached.Reset()
					direct.Reset()
					cached.DrawIcon(icon, bounds, tint)
					direct.drawIconDirect(icon, bounds, tint)
					sameTriangles(t, cached.Triangles(), direct.Triangles())
				}
			}
		}
	}
}

func TestIconCacheIsBounded(t *testing.T) {
	var canvas Canvas
	for size := 8; size < 8+3*iconCacheLimit; size++ {
		canvas.DrawIcon(testStrokeIcon, Rect{Width: float32(size), Height: float32(size)}, Color{Alpha: 1})
	}
	if got := len(canvas.icons().entries); got > iconCacheLimit {
		t.Fatalf("the icon cache holds %d entries, limit %d", got, iconCacheLimit)
	}
}

func BenchmarkDrawIconCached(b *testing.B) {
	var canvas Canvas
	bounds := Rect{X: 40, Y: 40, Width: 16, Height: 16}
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		canvas.DrawIcon(testStrokeIcon, bounds, Color{Alpha: 1})
	}
}

func BenchmarkDrawIconDirect(b *testing.B) {
	var canvas Canvas
	bounds := Rect{X: 40, Y: 40, Width: 16, Height: 16}
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		canvas.drawIconDirect(testStrokeIcon, bounds, Color{Alpha: 1})
	}
}

// The transforming path is the one a motion runs every frame: it must not pile
// up allocations, and it must stay close to the cached still path.
func BenchmarkDrawIconTransformed(b *testing.B) {
	bounds := Rect{X: 40, Y: 40, Width: 16, Height: 16}
	tint := Color{Alpha: 1}
	tests := []struct {
		name      string
		transform IconTransform
	}{
		{"rotate", IconTransform{Rotate: 0.4}},
		{"reveal", IconTransform{Reveal: 0.5}},
		{"parts", IconTransform{Parts: []IconPart{{Contour: 1, Offset: Point{Y: 2}}}}},
		{"trim", IconTransform{Parts: []IconPart{{Contour: 2, Trim: IconTrim{Active: true, From: 0.2, To: 0.7}}}}},
		{"clip", IconTransform{Clip: &IconClip{Bounds: Rect{X: 4, Y: 4, Width: 16, Height: 16}}}},
		{"clip-rounded", IconTransform{Clip: &IconClip{Bounds: Rect{X: 4, Y: 4, Width: 16, Height: 16}, Radius: 6}}},
	}
	for _, test := range tests {
		b.Run(test.name, func(b *testing.B) {
			var canvas Canvas
			b.ReportAllocs()
			for b.Loop() {
				canvas.Reset()
				canvas.DrawIconTransformed(testStrokeIcon, bounds, tint, test.transform)
			}
		})
	}
}

// The materialized cache is keyed by the canvas state its triangles carry, so a
// paint after a rectangle, or inside a clip, must not reuse the plain one.
func TestIconPaintCacheKeepsCanvasState(t *testing.T) {
	var canvas Canvas
	bounds := Rect{X: 20, Y: 20, Width: 16, Height: 16}
	tint := Color{Red: 0.2, Green: 0.4, Blue: 0.6, Alpha: 1}

	canvas.Fill(Rect{Width: 40, Height: 40}, Color{Alpha: 1})
	canvas.DrawIcon(testStrokeIcon, bounds, tint)
	withRect := append([]Triangle(nil), canvas.Triangles()...)
	if len(withRect) == 0 || withRect[0].Order <= canvas.Rectangles()[0].Order {
		t.Fatalf("the icon after a rectangle carries order %d", withRect[0].Order)
	}
	canvas.Reset()
	canvas.DrawIcon(testStrokeIcon, bounds, tint)
	plain := canvas.Triangles()
	if len(plain) != len(withRect) || plain[0].Order != 1 {
		t.Fatalf("the plain icon reused order %d", plain[0].Order)
	}
	canvas.Reset()
	canvas.Clip(Rect{Width: 30, Height: 30})
	canvas.DrawIcon(testStrokeIcon, bounds, tint)
	if !canvas.Triangles()[0].HasClip {
		t.Fatal("the clipped icon reused an unclipped cache entry")
	}
}

func TestIconPaintCacheIsBounded(t *testing.T) {
	var canvas Canvas
	for size := 8; size < 8+3*iconPaintCacheLimit; size++ {
		canvas.Reset()
		canvas.DrawIcon(testStrokeIcon, Rect{Width: float32(size), Height: float32(size)}, Color{Alpha: 1})
	}
	if got := len(canvas.iconPaints().entries); got > iconPaintCacheLimit {
		t.Fatalf("the paint cache holds %d entries, limit %d", got, iconPaintCacheLimit)
	}
}

// Moving an icon must not materialize it again: scrolling paints the same icon
// at a new origin every frame, and a position-keyed cache would copy it, per
// frame, until the entry rotated out.
func TestIconPaintCacheReusesEntryAcrossOrigins(t *testing.T) {
	var canvas Canvas
	tint := Color{Red: 0.2, Green: 0.4, Blue: 0.6, Alpha: 1}
	for offset := 0; offset < 64; offset++ {
		canvas.Reset()
		canvas.DrawIcon(testStrokeIcon, Rect{X: float32(offset), Y: float32(offset * 3), Width: 16, Height: 16}, tint)
	}
	if got := len(canvas.iconPaints().entries); got != 1 {
		t.Fatalf("the same icon at 64 origins made %d paint entries; want 1", got)
	}
}

// The moving path must stay allocation-free once the canvas and caches have
// warmed: an icon that scrolls is not a new icon.
func BenchmarkDrawIconCachedMoving(b *testing.B) {
	var canvas Canvas
	tint := Color{Alpha: 1}
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		canvas.Reset()
		canvas.DrawIcon(testStrokeIcon, Rect{X: 40, Y: float32(i % 1024), Width: 16, Height: 16}, tint)
	}
}

// triangleBounds is the box the canvas's triangles cover.
func triangleBounds(canvas *Canvas) (minimum, maximum Point) {
	minimum = Point{X: float32(math.Inf(1)), Y: float32(math.Inf(1))}
	maximum = Point{X: float32(math.Inf(-1)), Y: float32(math.Inf(-1))}
	for _, triangle := range canvas.Triangles() {
		for _, point := range triangle.Points {
			minimum.X, minimum.Y = min(minimum.X, point.X), min(minimum.Y, point.Y)
			maximum.X, maximum.Y = max(maximum.X, point.X), max(maximum.Y, point.Y)
		}
	}
	return minimum, maximum
}

// A transform at rest is the still icon, vertex for vertex: an animation starts
// and ends on that frame, so anything else would make the icon jump.
func TestIconTransformAtRestIsTheStillIcon(t *testing.T) {
	bounds := Rect{X: 8, Y: 8, Width: 16, Height: 16}
	tint := Color{Alpha: 1}
	var still, zero, unit, anchored Canvas
	still.DrawIcon(testStrokeIcon, bounds, tint)
	zero.DrawIconTransformed(testStrokeIcon, bounds, tint, IconTransform{})
	unit.DrawIconTransformed(testStrokeIcon, bounds, tint, IconTransform{Scale: 1})
	center := Point{X: 12, Y: 12}
	anchored.DrawIconTransformed(testStrokeIcon, bounds, tint, IconTransform{Scale: 1, ScaleY: 1, Anchor: &center})
	want, got, scaled := still.Triangles(), zero.Triangles(), unit.Triangles()
	if len(want) != len(got) || len(want) != len(scaled) || len(want) != len(anchored.Triangles()) {
		t.Fatalf("still %d triangles, zero %d, scale 1 %d, anchored %d", len(want), len(got), len(scaled), len(anchored.Triangles()))
	}
	for index := range want {
		if want[index].Points != got[index].Points {
			t.Fatalf("triangle %d moved with a zero transform: %v then %v", index, want[index].Points, got[index].Points)
		}
		if want[index].Points != scaled[index].Points {
			t.Fatalf("triangle %d moved with scale 1: %v then %v", index, want[index].Points, scaled[index].Points)
		}
		if want[index].Points != anchored.Triangles()[index].Points {
			t.Fatalf("triangle %d moved with an identity anchor: %v then %v", index, want[index].Points, anchored.Triangles()[index].Points)
		}
	}
}

// A view-box translate is the same fraction of the icon at every size, while
// the logical-pixel offset remains for a displacement that is deliberately the
// same on screen.
func TestIconTranslateScalesWithTheIcon(t *testing.T) {
	tint := Color{Alpha: 1}
	for _, size := range []float32{16, 64} {
		var still, moved Canvas
		bounds := Rect{Width: size, Height: size}
		still.DrawIcon(testStrokeIcon, bounds, tint)
		moved.DrawIconTransformed(testStrokeIcon, bounds, tint, IconTransform{Translate: Point{X: 2}})
		restMinimum, restMaximum := triangleBounds(&still)
		minimum, maximum := triangleBounds(&moved)
		want := size / 24 * 2
		if math.Abs(float64(minimum.X-restMinimum.X-want)) > 0.02 ||
			math.Abs(float64(maximum.X-restMaximum.X-want)) > 0.02 {
			t.Fatalf("a two-unit translate at %gpx moved by %v, want %v", size, minimum.X-restMinimum.X, want)
		}
	}
}

// A non-square icon keeps one source unit on both axes, so a whole-icon
// translate moves every vertex by the same number of view-box units.
func TestWideIconTransformUsesSourceUnits(t *testing.T) {
	bounds := Rect{X: 10, Y: 20, Width: 80, Height: 80}
	tint := Color{Alpha: 1}
	var still, moved Canvas
	still.DrawIcon(testWideIcon, bounds, tint)
	moved.DrawIconTransformed(testWideIcon, bounds, tint, IconTransform{
		Translate: Point{X: 20, Y: 5},
	})
	restMinimum, restMaximum := triangleBounds(&still)
	minimum, maximum := triangleBounds(&moved)
	if math.Abs(float64(minimum.X-restMinimum.X-40)) > 0.02 || math.Abs(float64(maximum.X-restMaximum.X-40)) > 0.02 {
		t.Fatalf("translate X moved %v..%v, want %v..%v", minimum.X, maximum.X, restMinimum.X+40, restMaximum.X+40)
	}
	if math.Abs(float64(minimum.Y-restMinimum.Y-10)) > 0.02 || math.Abs(float64(maximum.Y-restMaximum.Y-10)) > 0.02 {
		t.Fatalf("translate Y moved %v..%v, want %v..%v", minimum.Y, maximum.Y, restMinimum.Y+10, restMaximum.Y+10)
	}
}

// A part of a non-square icon converts both axes with the fitted box, not with
// its width for y as well: a two-unit lift moves the 4:1 glyph two source units.
func TestWideIconPartUsesSourceUnitsOnBothAxes(t *testing.T) {
	bounds := Rect{X: 10, Y: 20, Width: 80, Height: 80}
	tint := Color{Alpha: 1}
	rest := partsTriangles(t, testWideIcon, bounds, tint, nil)
	moved := partsTriangles(t, testWideIcon, bounds, tint, []IconPart{{Contour: 0, Translate: Point{Y: 2}}})
	restMinimum, restMaximum := triangleBounds(&Canvas{triangles: rest[0]})
	minimum, maximum := triangleBounds(&Canvas{triangles: moved[0]})
	if math.Abs(float64(minimum.Y-restMinimum.Y-4)) > 0.01 || math.Abs(float64(maximum.Y-restMaximum.Y-4)) > 0.01 {
		t.Fatalf("the part moved y %.2f..%.2f, at rest %.2f..%.2f; want it four points lower",
			minimum.Y, maximum.Y, restMinimum.Y, restMaximum.Y)
	}
}

// A part anchor is a point in the source view box on a non-square icon too: a
// horizontal bar mirrored about its own line stays where it was.
func TestWideIconPartAnchorUsesSourceUnitsOnBothAxes(t *testing.T) {
	bounds := Rect{X: 10, Y: 20, Width: 80, Height: 80}
	tint := Color{Alpha: 1}
	rest := partsTriangles(t, testWideIcon, bounds, tint, nil)
	anchor := Point{X: 30, Y: 5}
	mirrored := partsTriangles(t, testWideIcon, bounds, tint, []IconPart{{
		Contour: 0, ScaleY: -1, Anchor: &anchor,
	}})
	restMinimum, restMaximum := triangleBounds(&Canvas{triangles: rest[0]})
	minimum, maximum := triangleBounds(&Canvas{triangles: mirrored[0]})
	if math.Abs(float64(minimum.Y-restMinimum.Y)) > 0.01 || math.Abs(float64(maximum.Y-restMaximum.Y)) > 0.01 ||
		math.Abs(float64(minimum.X-restMinimum.X)) > 0.01 || math.Abs(float64(maximum.X-restMaximum.X)) > 0.01 {
		t.Fatalf("mirroring the bar moved it from %v..%v to %v..%v", restMinimum, restMaximum, minimum, maximum)
	}
}

// A clip on a non-square icon stays in source units on both axes. The wide test
// glyph's visible ink is all in the clip; a narrower clip must cut it on both
// axes the way an X-only conversion would not.
func TestWideIconClipUsesSourceUnitsOnBothAxes(t *testing.T) {
	bounds := Rect{X: 10, Y: 20, Width: 80, Height: 80}
	tint := Color{Alpha: 1}
	icon := testWideIcon
	icon.Layers = nil
	var still, clipped Canvas
	still.DrawIcon(icon, bounds, tint)
	clipped.DrawIconTransformed(icon, bounds, tint, IconTransform{
		Clip: &IconClip{Bounds: Rect{X: 20, Y: 4, Width: 20, Height: 1}},
	})
	restMinimum, restMaximum := triangleBounds(&still)
	minimum, maximum := triangleBounds(&clipped)
	// The line's round caps reach past its endpoints, so its ink spans x
	// 56.06..84 inside the clip. The one-unit-high source window is y 4..5,
	// which is 58..60 in the target.
	if math.Abs(float64(minimum.X-56.058)) > 0.02 || math.Abs(float64(maximum.X-84)) > 0.01 ||
		math.Abs(float64(minimum.Y-58)) > 0.01 || math.Abs(float64(maximum.Y-60)) > 0.01 {
		t.Fatalf("the one-unit clip paints %v..%v, want x 56.06..84 and y 58..60", minimum, maximum)
	}
	if minimum.Y < restMinimum.Y || maximum.Y > restMaximum.Y {
		t.Fatalf("the clip reaches outside the glyph %v..%v", restMinimum, restMaximum)
	}
}

// ScaleY stretches on top of Scale, so a transformed icon is the original
// stretched about its chosen point on each axis independently.
func TestIconScaleYStretchesOnTopOfScale(t *testing.T) {
	bounds := Rect{X: 8, Y: 8, Width: 24, Height: 24}
	tint := Color{Alpha: 1}
	var still, stretched Canvas
	still.DrawIcon(testStrokeIcon, bounds, tint)
	stretched.DrawIconTransformed(testStrokeIcon, bounds, tint, IconTransform{Scale: 2, ScaleY: 0.5})
	restMinimum, restMaximum := triangleBounds(&still)
	minimum, maximum := triangleBounds(&stretched)
	center := Point{X: 20, Y: 20}
	if math.Abs(float64(minimum.X-(center.X+(restMinimum.X-center.X)*2))) > 0.02 ||
		math.Abs(float64(maximum.X-(center.X+(restMaximum.X-center.X)*2))) > 0.02 ||
		math.Abs(float64(minimum.Y-(center.Y+(restMinimum.Y-center.Y)*1))) > 0.02 ||
		math.Abs(float64(maximum.Y-(center.Y+(restMaximum.Y-center.Y)*1))) > 0.02 {
		t.Fatalf("stretched bounds = %v..%v, want x scaled twice and y unscaled", minimum, maximum)
	}
}

// A whole-glyph rotation about a corner is the same as rotating about the box
// centre and then translating the centre onto that corner.
func TestIconTransformUsesTheGlyphAnchor(t *testing.T) {
	bounds := Rect{X: 8, Y: 8, Width: 16, Height: 16}
	tint := Color{Alpha: 1}
	corner := Point{}
	var anchored, translated Canvas
	anchored.DrawIconTransformed(testStrokeIcon, bounds, tint, IconTransform{Rotate: math.Pi / 2, Anchor: &corner})
	// A quarter turn takes the vector from corner to centre, (8, 8), to (-8, 8),
	// so moving the centre's rotation back onto the corner shifts it by (-16, 0).
	translated.DrawIconTransformed(testStrokeIcon, bounds, tint, IconTransform{
		Rotate: math.Pi / 2,
		Offset: Point{X: -16, Y: 0},
	})
	want, got := translated.Triangles(), anchored.Triangles()
	if len(want) != len(got) {
		t.Fatalf("anchored turn made %d triangles, translated %d", len(got), len(want))
	}
	for index := range want {
		for corner := range want[index].Points {
			if math.Abs(float64(want[index].Points[corner].X-got[index].Points[corner].X)) > 0.02 ||
				math.Abs(float64(want[index].Points[corner].Y-got[index].Points[corner].Y)) > 0.02 {
				t.Fatalf("triangle %d corner %d = %v, want %v", index, corner, got[index].Points[corner], want[index].Points[corner])
			}
		}
	}
}

// A local clip cuts the icon's own mesh: a triangle outside the clip paints
// nothing, one inside paints unchanged, and the cut's new edge is not feathered
// as though it were the glyph's own outline.
func TestIconClipCutsOnlyItsOwnTriangles(t *testing.T) {
	bounds := Rect{X: 10, Y: 20, Width: 24, Height: 24}
	tint := Color{Alpha: 1}
	var whole, clipped Canvas
	whole.DrawIcon(testStrokeIcon, bounds, tint)
	clipped.DrawIconTransformed(testStrokeIcon, bounds, tint, IconTransform{
		Clip: &IconClip{Bounds: Rect{X: 4, Y: 4, Width: 8, Height: 8}},
	})
	if len(clipped.Triangles()) == 0 || len(clipped.Triangles()) >= len(whole.Triangles()) {
		t.Fatalf("clip of the top-left eighth painted %d of %d triangles", len(clipped.Triangles()), len(whole.Triangles()))
	}
	for _, triangle := range clipped.Triangles() {
		for _, point := range triangle.Points {
			// The 8-unit square is a third of the 24px box, from view-box 4, so
			// it spans 14..22 across and 24..32 down.
			if point.X < 14-1e-3 || point.X > 22+1e-3 || point.Y < 24-1e-3 || point.Y > 32+1e-3 {
				t.Fatalf("a clipped triangle reaches %v", point)
			}
		}
	}
}

// The icon clip moves with the glyph: rotating the icon also rotates the part of
// its view box the clip leaves behind.
func TestIconClipFollowsTheWholeTransform(t *testing.T) {
	bounds := Rect{X: 0, Y: 0, Width: 24, Height: 24}
	tint := Color{Alpha: 1}
	clip := IconClip{Bounds: Rect{X: 0, Y: 0, Width: 24, Height: 12}}
	var upright, turned Canvas
	upright.DrawIconTransformed(testStrokeIcon, bounds, tint, IconTransform{Clip: &clip})
	turned.DrawIconTransformed(testStrokeIcon, bounds, tint, IconTransform{Rotate: math.Pi / 2, Clip: &clip})
	if len(upright.Triangles()) == 0 || len(turned.Triangles()) == 0 {
		t.Fatal("the clip painted nothing")
	}
	upMinimum, upMaximum := triangleBounds(&upright)
	minimum, maximum := triangleBounds(&turned)
	if upMinimum.X < 0-1e-3 || upMaximum.Y > 12+1e-3 {
		t.Fatalf("the upright clip paints %v..%v", upMinimum, upMaximum)
	}
	if minimum.X < 12-1e-3 || maximum.Y > 24+1e-3 {
		t.Fatalf("the quarter-turned clip paints %v..%v", minimum, maximum)
	}
}

// Fading one contour leaves every other contour exactly where it was, and a
// fully hidden part paints nothing.
func TestIconPartFadeHidesOneContourWithoutMovingTheOthers(t *testing.T) {
	bounds := Rect{X: 10, Y: 20, Width: 16, Height: 16}
	tint := Color{Alpha: 1}
	var got Canvas
	got.DrawIconTransformed(testStrokeIcon, bounds, tint, IconTransform{
		Parts: []IconPart{{Contour: 1, Fade: 1}},
	})
	var want Canvas
	for index, contour := range testStrokeIcon.Contours() {
		if index != 1 {
			want.DrawIcon(contour, bounds, tint)
		}
	}
	if len(got.Triangles()) != len(want.Triangles()) {
		t.Fatalf("hiding one contour painted %d triangles, the other two %d", len(got.Triangles()), len(want.Triangles()))
	}
	sameTriangles(t, got.Triangles(), want.Triangles())
}

// A part's trims are measured along its contour: half the contour paints half
// of its stroke, not half of each segment stopped at the same x.
func TestIconPartTrimWritesTheContourFromItsStart(t *testing.T) {
	bounds := Rect{X: 10, Y: 20, Width: 24, Height: 24}
	tint := Color{Alpha: 1}
	line := Icon{
		ViewBox: 24, Stroke: 2, Cap: CapRound, Join: JoinRound,
		Commands: []PathCommand{MoveTo(2, 12), LineTo(22, 12)},
	}
	var whole Canvas
	whole.DrawIcon(line, bounds, tint)
	var trimmed Canvas
	trimmed.DrawIconTransformed(line, bounds, tint, IconTransform{
		Parts: []IconPart{{Contour: 0, Trim: IconTrim{Active: true, From: 0, To: 0.5}}},
	})
	minimum, maximum := triangleBounds(&trimmed)
	wholeMinimum, _ := triangleBounds(&whole)
	if math.Abs(float64(minimum.X-wholeMinimum.X)) > 0.02 || math.Abs(float64(maximum.X-(wholeMinimum.X+12))) > 0.15 {
		t.Fatalf("half the line spans %v..%v, want about %v..%v", minimum, maximum, wholeMinimum, wholeMinimum.X+12)
	}

	var hidden Canvas
	hidden.DrawIconTransformed(line, bounds, tint, IconTransform{
		Parts: []IconPart{{Contour: 0, Trim: IconTrim{Active: true, From: 0.5, To: 0.5}}},
	})
	if len(hidden.Triangles()) != 0 {
		t.Fatalf("a zero-length trim painted %d triangles", len(hidden.Triangles()))
	}
}

// A full trim is the source glyph, including its curves and cap, rather than a
// polyline sampled from them.
func TestIconFullTrimIsTheSourceGlyph(t *testing.T) {
	bounds := Rect{X: 10, Y: 20, Width: 16, Height: 16}
	tint := Color{Alpha: 1}
	var whole, trimmed Canvas
	whole.DrawIcon(testStrokeIcon, bounds, tint)
	trimmed.DrawIconTransformed(testStrokeIcon, bounds, tint, IconTransform{
		Parts: []IconPart{{Contour: 2, Trim: IconTrim{Active: true, From: 0, To: 1}}},
	})
	sameTriangles(t, trimmed.Triangles(), whole.Triangles())
}

// A trim on a filled glyph has no stroke to write, so the whole fill remains
// rather than becoming the outline of the trimmed contour. This holds when the
// part also moves, which is the path that tessellates a fresh contour.
func TestIconTrimLeavesAMovingFilledGlyphWhole(t *testing.T) {
	bounds := Rect{X: 10, Y: 20, Width: 24, Height: 24}
	tint := Color{Alpha: 1}
	filled := Icon{
		ViewBox: 24,
		Commands: []PathCommand{
			MoveTo(4, 4), LineTo(20, 4), LineTo(20, 20), LineTo(4, 20), ClosePath(),
		},
	}
	var whole, trimmed Canvas
	whole.DrawIcon(filled, bounds, tint)
	trimmed.DrawIconTransformed(filled, bounds, tint, IconTransform{
		Parts: []IconPart{{Contour: 0, Translate: Point{X: 1}, Trim: IconTrim{Active: true, From: 0, To: 0.25}}},
	})
	if len(trimmed.Triangles()) != len(whole.Triangles()) {
		t.Fatalf("a trimmed filled glyph painted %d triangles, the whole fill %d", len(trimmed.Triangles()), len(whole.Triangles()))
	}
}

// A trim that changes from frame to frame paints the geometry that frame asks
// for, not the cached geometry of the frame before it.
func TestIconTrimChangesGeometryAcrossFrames(t *testing.T) {
	bounds := Rect{X: 10, Y: 20, Width: 24, Height: 24}
	tint := Color{Alpha: 1}
	line := Icon{
		ViewBox: 24, Stroke: 2, Cap: CapRound, Join: JoinRound,
		Commands: []PathCommand{MoveTo(2, 12), LineTo(22, 12)},
	}
	var shared Canvas
	for _, to := range []float32{0.25, 0.75, 0.5} {
		transform := IconTransform{
			Parts: []IconPart{{Contour: 0, Trim: IconTrim{Active: true, From: 0, To: to}}},
		}
		shared.Reset()
		shared.DrawIconTransformed(line, bounds, tint, transform)
		var fresh Canvas
		fresh.DrawIconTransformed(line, bounds, tint, transform)
		sameTriangles(t, shared.Triangles(), fresh.Triangles())
	}
}

func TestWarmTrimmedIconPaintDoesNotAllocate(t *testing.T) {
	bounds := Rect{X: 10, Y: 20, Width: 24, Height: 24}
	tint := Color{Alpha: 1}
	line := Icon{
		ViewBox: 24, Stroke: 2, Cap: CapRound, Join: JoinRound,
		Commands: []PathCommand{MoveTo(2, 12), LineTo(22, 12)},
	}
	var canvas Canvas
	ends := [...]float32{0.25, 0.5, 0.75}
	paint := func(to float32) {
		canvas.Reset()
		canvas.DrawIconTransformed(line, bounds, tint, IconTransform{
			Parts: []IconPart{{Contour: 0, Trim: IconTrim{Active: true, From: 0, To: to}}},
		})
	}
	for index := range ends {
		paint(ends[index])
	}
	next := 0
	allocations := testing.AllocsPerRun(100, func() {
		paint(ends[next])
		next = (next + 1) % len(ends)
	})
	if allocations != 0 {
		t.Fatalf("a warm trimmed icon allocates %v times", allocations)
	}
}

// Scale shrinks the glyph about the center of the box it was laid out in, so the
// strokes and the space between them keep their proportion.
func TestIconTransformScalesAboutTheCenter(t *testing.T) {
	bounds := Rect{X: 8, Y: 8, Width: 16, Height: 16}
	tint := Color{Alpha: 1}
	var still, half Canvas
	still.DrawIcon(testStrokeIcon, bounds, tint)
	half.DrawIconTransformed(testStrokeIcon, bounds, tint, IconTransform{Scale: 0.5})
	minimum, maximum := triangleBounds(&still)
	// The glyph is not symmetric about its box: the center the transform uses is
	// the box's, exactly as CSS turns an element about its center.
	center := Point{X: bounds.X + bounds.Width/2, Y: bounds.Y + bounds.Height/2}
	halved, halvedMaximum := triangleBounds(&half)
	pairs := [4][2]float32{
		{halved.X, center.X + (minimum.X-center.X)*0.5},
		{halved.Y, center.Y + (minimum.Y-center.Y)*0.5},
		{halvedMaximum.X, center.X + (maximum.X-center.X)*0.5},
		{halvedMaximum.Y, center.Y + (maximum.Y-center.Y)*0.5},
	}
	for index, pair := range pairs {
		if math.Abs(float64(pair[0]-pair[1])) > 1e-4 {
			t.Fatalf("corner %d of half the icon is at %v, want %v", index, pair[0], pair[1])
		}
	}
}

// A half turn about the center leaves a box that is the original mirrored
// through that center, whatever shape the glyph is.
func TestIconTransformTurnsAboutTheCenter(t *testing.T) {
	bounds := Rect{X: 8, Y: 8, Width: 16, Height: 16}
	tint := Color{Alpha: 1}
	var still, turned Canvas
	still.DrawIcon(testStrokeIcon, bounds, tint)
	turned.DrawIconTransformed(testStrokeIcon, bounds, tint, IconTransform{Rotate: float32(math.Pi)})
	minimum, maximum := triangleBounds(&still)
	center := Point{X: bounds.X + bounds.Width/2, Y: bounds.Y + bounds.Height/2}
	turnedMinimum, turnedMaximum := triangleBounds(&turned)
	pairs := [4][2]float32{
		{turnedMinimum.X, 2*center.X - maximum.X},
		{turnedMaximum.X, 2*center.X - minimum.X},
		{turnedMinimum.Y, 2*center.Y - maximum.Y},
		{turnedMaximum.Y, 2*center.Y - minimum.Y},
	}
	for index, pair := range pairs {
		if math.Abs(float64(pair[0]-pair[1])) > 1e-3 {
			t.Fatalf("edge %d of the turned icon is at %v, want %v", index, pair[0], pair[1])
		}
	}
}

// A stroked glyph splits into its contours, in drawing order, without losing or
// reordering a command: the split is what lets a part of an icon move alone.
func TestIconContoursCoverEveryCommand(t *testing.T) {
	contours := testStrokeIcon.Contours()
	if len(contours) != 3 {
		t.Fatalf("the test icon made %d contours, want 3", len(contours))
	}
	total := 0
	for index, contour := range contours {
		if len(contour.Commands) == 0 || contour.Commands[0].Kind != PathCommandMove {
			t.Fatalf("contour %d does not start at a move", index)
		}
		if contour.ViewBox != testStrokeIcon.ViewBox || contour.Stroke != testStrokeIcon.Stroke || contour.Cap != testStrokeIcon.Cap {
			t.Fatalf("contour %d lost the glyph's view box, stroke or caps", index)
		}
		total += len(contour.Commands)
	}
	if total != len(testStrokeIcon.Commands) {
		t.Fatalf("the contours hold %d commands, the glyph %d", total, len(testStrokeIcon.Commands))
	}
}

// contourTriangles tessellates each contour on a canvas of its own, so a test
// can compare one contour of a motion against the same contour at rest.
func contourTriangles(icon Icon, bounds Rect, tint Color) [][]Triangle {
	contours := icon.Contours()
	out := make([][]Triangle, len(contours))
	for index, contour := range contours {
		var canvas Canvas
		canvas.DrawIcon(contour, bounds, tint)
		out[index] = canvas.Triangles()
	}
	return out
}

// partsTriangles paints the glyph contour by contour through the given parts
// and returns the triangles of each contour, in drawing order.
func partsTriangles(t *testing.T, icon Icon, bounds Rect, tint Color, parts []IconPart) [][]Triangle {
	t.Helper()
	var canvas Canvas
	target, ok := canvas.iconTarget(icon, bounds)
	if !ok {
		t.Fatal("the icon fits nowhere")
	}
	contours := icon.Contours()
	continuous := make([][]Triangle, len(contours))
	for index, contour := range contours {
		var rest Canvas
		rest.DrawIcon(contour, bounds, tint)
		continuous[index] = rest.Triangles()
	}
	counts := make([]int, len(continuous))
	for index := range continuous {
		counts[index] = len(continuous[index])
	}
	place := stillPlacement(Point{X: target.X, Y: target.Y})
	canvas.drawIconParts(icon, target, tint, place, parts)
	triangles := canvas.Triangles()
	out := make([][]Triangle, len(counts))
	offset := 0
	for index, count := range counts {
		if offset+count > len(triangles) {
			t.Fatalf("contour %d ends past the %d painted triangles", index, len(triangles))
		}
		out[index] = triangles[offset : offset+count]
		offset += count
	}
	// A glyph's Layers are not contours: they paint once after the contours, so
	// the triangles past the contours are exactly them. Accounting for that here
	// keeps the split exact instead of tolerating slack the contours did not paint.
	layers := icon
	layers.Commands = nil
	var layerCanvas Canvas
	layerCanvas.DrawIcon(layers, bounds, tint)
	if painted, want := len(triangles)-offset, len(layerCanvas.Triangles()); painted != want {
		t.Fatalf("the contours painted %d triangles and the layers %d, of %d painted", offset, want, len(triangles))
	}
	return out
}

// Painting a glyph contour by contour paints the same picture as painting it
// whole while its parts stay put.
func TestIconPartsAtRestDrawTheWholeGlyph(t *testing.T) {
	bounds := Rect{X: 10, Y: 20, Width: 16, Height: 16}
	tint := Color{Alpha: 1}
	want := contourTriangles(testStrokeIcon, bounds, tint)
	got := partsTriangles(t, testStrokeIcon, bounds, tint, nil)
	for index := range want {
		sameTriangles(t, got[index], want[index])
	}
}

// Only the contour named by a moving part moves; its neighbours paint where
// they always did.
func TestOneMovingPartLeavesTheOthersAlone(t *testing.T) {
	bounds := Rect{X: 10, Y: 20, Width: 16, Height: 16}
	tint := Color{Alpha: 1}
	// The test icon's first contour is a horizontal line at y 12; the others are
	// the arrow head and the arc.
	want := contourTriangles(testStrokeIcon, bounds, tint)
	got := partsTriangles(t, testStrokeIcon, bounds, tint, []IconPart{{Contour: 0, Offset: Point{Y: 4}}})
	for index := 1; index < len(want); index++ {
		sameTriangles(t, got[index], want[index])
	}
	minimum, maximum := triangleBounds(&Canvas{triangles: got[0]})
	restMinimum, restMaximum := triangleBounds(&Canvas{triangles: want[0]})
	if math.Abs(float64(minimum.Y-restMinimum.Y-4)) > 0.01 || math.Abs(float64(maximum.Y-restMaximum.Y-4)) > 0.01 {
		t.Fatalf("the moving contour spans y %.2f..%.2f, at rest %.2f..%.2f; want it four points lower",
			minimum.Y, maximum.Y, restMinimum.Y, restMaximum.Y)
	}
}

// A glyph's first contour starts at the same command the glyph does, so the two
// icons share a start pointer. The caches must still tell them apart: a contour
// painted after its glyph, or the other way round, is a different picture.
func TestAContourAndItsGlyphCacheApart(t *testing.T) {
	bounds := Rect{X: 10, Y: 20, Width: 16, Height: 16}
	tint := Color{Alpha: 1}

	var alone Canvas
	alone.DrawIcon(testStrokeIcon, bounds, tint)
	want := append([]Triangle(nil), alone.Triangles()...)

	// The glyph is cached first, then its first contour is painted on the same
	// canvas, then the glyph again: neither draw may serve the other's triangles.
	var shared Canvas
	shared.DrawIcon(testStrokeIcon, bounds, tint)
	contour := testStrokeIcon.Contours()[0]
	shared.DrawIcon(contour, bounds, tint)
	shared.Reset()
	shared.DrawIcon(testStrokeIcon, bounds, tint)
	sameTriangles(t, shared.Triangles(), want)

	// And the other way round: the contour cached first, then the glyph.
	var reversed Canvas
	reversed.DrawIcon(contour, bounds, tint)
	reversed.Reset()
	reversed.DrawIcon(testStrokeIcon, bounds, tint)
	sameTriangles(t, reversed.Triangles(), want)
}

// A glyph's filled layers are not contours, so a part that moves must not drop
// them: they paint once, through the glyph's placement, where the whole glyph
// paints them.
func TestAMovingPartKeepsTheGlyphLayers(t *testing.T) {
	bounds := Rect{X: 10, Y: 20, Width: 16, Height: 16}
	tint := Color{Alpha: 1}
	layersOnly := testWideIcon
	layersOnly.Commands = nil
	var only Canvas
	only.DrawIcon(layersOnly, bounds, tint)
	layers := only.Triangles()
	if len(layers) == 0 {
		t.Fatal("the test icon has no layers to compare against")
	}

	var moved Canvas
	target, ok := moved.iconTarget(testWideIcon, bounds)
	if !ok {
		t.Fatal("the icon fits nowhere")
	}
	moved.drawIconParts(testWideIcon, target, tint, stillPlacement(Point{X: target.X, Y: target.Y}),
		[]IconPart{{Contour: 0, Offset: Point{Y: 2}}})
	got := moved.Triangles()
	if len(got) < len(layers) {
		t.Fatalf("the moving glyph painted %d triangles, fewer than its %d layer triangles", len(got), len(layers))
	}
	sameTriangles(t, got[len(got)-len(layers):], layers)
}

// A reveal writes the glyph on from its right edge: the triangles are the still
// glyph's, clipped to the part already written, and a full reveal is the still
// icon again.
func TestIconRevealWritesTheGlyphFromItsRightEdge(t *testing.T) {
	bounds := Rect{X: 10, Y: 20, Width: 16, Height: 16}
	tint := Color{Alpha: 1}
	var whole Canvas
	whole.DrawIcon(testStrokeIcon, bounds, tint)
	if whole.Triangles()[0].HasClip {
		t.Fatal("the still icon paints with a clip")
	}

	var half Canvas
	half.DrawIconTransformed(testStrokeIcon, bounds, tint, IconTransform{Reveal: 0.5})
	triangles := half.Triangles()
	if len(triangles) != len(whole.Triangles()) {
		t.Fatalf("a half reveal painted %d triangles, the still icon %d", len(triangles), len(whole.Triangles()))
	}
	// Half of a 16 point box starting at 10 is 18, the middle of the icon.
	if clip := triangles[0].ClipOf; !triangles[0].HasClip || clip.X != 18 || clip.Width != 8 || clip.Y != 20 || clip.Height != 16 {
		t.Fatalf("a half reveal clips to %+v, want x 18 width 8 of the box", clip)
	}
	for index, triangle := range triangles {
		if triangle.ClipOf != triangles[0].ClipOf || triangle.HasClip != triangles[0].HasClip {
			t.Fatalf("triangle %d clips to %+v, triangle 0 to %+v", index, triangle.ClipOf, triangles[0].ClipOf)
		}
		if triangle.Points != whole.Triangles()[index].Points {
			t.Fatalf("triangle %d moved: a reveal clips the glyph, it does not redraw it", index)
		}
	}

	// A reveal that has written all of it, and one that has written none of it
	// asked for, are both the still icon.
	for _, reveal := range []float32{0, 1} {
		var settled Canvas
		settled.DrawIconTransformed(testStrokeIcon, bounds, tint, IconTransform{Reveal: reveal})
		sameTriangles(t, settled.Triangles(), whole.Triangles())
	}
}

// Reveal and ClipToBox bound a glyph that is also moving: changing its points
// must not bypass either clip.
func TestIconBoxClipsApplyToATransformedGlyph(t *testing.T) {
	bounds := Rect{Width: 24, Height: 24}
	tint := Color{Alpha: 1}
	var boxed, revealed Canvas
	boxed.DrawIconTransformed(testStrokeIcon, bounds, tint, IconTransform{Translate: Point{X: 12}, ClipToBox: true})
	if len(boxed.Triangles()) == 0 {
		t.Fatal("the boxed glyph painted nothing")
	}
	if clip := boxed.Triangles()[0].ClipOf; !boxed.Triangles()[0].HasClip || clip != bounds {
		t.Fatalf("the moved glyph clips to %+v, want its box %+v", clip, bounds)
	}
	revealed.DrawIconTransformed(testStrokeIcon, bounds, tint, IconTransform{Translate: Point{Y: 4}, Reveal: 0.5})
	if len(revealed.Triangles()) == 0 {
		t.Fatal("the revealed glyph painted nothing")
	}
	if clip := revealed.Triangles()[0].ClipOf; !revealed.Triangles()[0].HasClip || clip != (Rect{X: 12, Y: 0, Width: 12, Height: 24}) {
		t.Fatalf("the moved glyph's reveal clips to %+v", clip)
	}
}

// A part with an anchor turns and stretches about that point of the glyph rather
// than about itself: a page stretched to minus one over the spine lands mirrored
// on the other side of the book.
func TestAPartStretchesAboutItsAnchor(t *testing.T) {
	bounds := Rect{X: 10, Y: 20, Width: 16, Height: 16}
	tint := Color{Alpha: 1}
	spine := Point{X: 12, Y: 12} // the glyph's centre, which is the box's centre
	rest := contourTriangles(testStrokeIcon, bounds, tint)
	got := partsTriangles(t, testStrokeIcon, bounds, tint, []IconPart{{Contour: 2, ScaleX: -1, Anchor: &spine}})
	if len(got[2]) != len(rest[2]) {
		t.Fatalf("the turned page painted %d triangles, at rest %d", len(got[2]), len(rest[2]))
	}
	// The box is 16 points wide from 10, so the spine sits at 18.
	for index := range rest[2] {
		for corner := range rest[2][index].Points {
			want := rest[2][index].Points[corner]
			mirrored := 2*18 - want.X
			if math.Abs(float64(got[2][index].Points[corner].X-mirrored)) > 0.01 ||
				math.Abs(float64(got[2][index].Points[corner].Y-want.Y)) > 0.01 {
				t.Fatalf("vertex %d of triangle %d is at %+v, want the rest vertex %+v mirrored over the spine",
					corner, index, got[2][index].Points[corner], want)
			}
		}
	}
	for index := range rest {
		if index == 2 {
			continue
		}
		sameTriangles(t, got[index], rest[index])
	}
}
