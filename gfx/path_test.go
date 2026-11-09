package gfx

import (
	"math"
	"math/rand"
	"testing"
)

func TestDrawPathFillsConcavePolygon(t *testing.T) {
	// A chevron: concave at (5, 6).
	path := NewPath(10, 10,
		MoveTo(1, 1), LineTo(9, 1), LineTo(9, 4), LineTo(5, 6), LineTo(9, 8),
		LineTo(9, 9), LineTo(1, 9), ClosePath(),
	)
	var canvas Canvas
	canvas.DrawPath(path, Rect{Width: 10, Height: 10}, PathStyle{Fill: Color{Alpha: 1}})

	triangles := canvas.Triangles()
	if len(triangles) < 3 {
		t.Fatalf("triangles = %d, want at least 3 for a concave fill", len(triangles))
	}
	if area := triangleCoverArea(triangles); math.Abs(area-56) > 0.5 {
		t.Fatalf("concave fill area = %v, want 56", area)
	}
	assertBoundaryMasks(t, triangles, 7)
}

func TestDrawPathFeathersOnlyTheOuterMeshBoundary(t *testing.T) {
	path := NewPath(10, 10,
		MoveTo(1, 1), LineTo(9, 1), LineTo(9, 9), LineTo(1, 9), ClosePath(),
	)
	var canvas Canvas
	canvas.DrawPath(path, Rect{Width: 10, Height: 10}, PathStyle{Fill: Color{Alpha: 1}})

	triangles := canvas.Triangles()
	if len(triangles) != 2 {
		t.Fatalf("square triangles = %d, want 2", len(triangles))
	}
	assertBoundaryMasks(t, triangles, 4)
}

func TestDrawPathFillsRingWithHole(t *testing.T) {
	path := NewPath(10, 10,
		MoveTo(1, 1), LineTo(9, 1), LineTo(9, 9), LineTo(1, 9), ClosePath(),
		MoveTo(3, 3), LineTo(3, 7), LineTo(7, 7), LineTo(7, 3), ClosePath(),
	)
	var canvas Canvas
	canvas.DrawPath(path, Rect{Width: 10, Height: 10}, PathStyle{Fill: Color{Alpha: 1}})

	triangles := canvas.Triangles()
	if area := triangleCoverArea(triangles); math.Abs(area-48) > 0.5 {
		t.Fatalf("ring fill area = %v, want 48 (64 outer - 16 hole)", area)
	}
	assertBoundaryMasks(t, triangles, 8)
}

func TestDrawPathEvenOddRule(t *testing.T) {
	// Nested same-winding squares: nonzero keeps the inner square covered
	// (island inside filled region stays part of it), even-odd turns it
	// into a hole (144 - 16 = 128).
	commands := []PathCommand{
		MoveTo(1, 1), LineTo(13, 1), LineTo(13, 13), LineTo(1, 13), ClosePath(),
		MoveTo(5, 5), LineTo(9, 5), LineTo(9, 9), LineTo(5, 9), ClosePath(),
	}

	var nonzero, evenOdd Canvas
	nonzero.DrawPath(NewPath(14, 14, commands...), Rect{Width: 14, Height: 14}, PathStyle{Fill: Color{Alpha: 1}})
	evenOdd.DrawPath(NewPath(14, 14, commands...), Rect{Width: 14, Height: 14}, PathStyle{Fill: Color{Alpha: 1}, FillRule: PathFillEvenOdd})

	if area := triangleCoverArea(nonzero.Triangles()); math.Abs(area-144) > 0.5 {
		t.Fatalf("nonzero area = %v, want 144", area)
	}
	if area := triangleCoverArea(evenOdd.Triangles()); math.Abs(area-128) > 0.5 {
		t.Fatalf("even-odd area = %v, want 128", area)
	}
}

func TestDrawPathNonZeroIslandInsideHole(t *testing.T) {
	// Outer square, opposite-winding hole, same-winding island inside the
	// hole: nonzero fills outer + island = 144 - 16 + 4... the hole is 16
	// and the island 4, so visible area is 144 - 16 + 4 = 132.
	commands := []PathCommand{
		MoveTo(1, 1), LineTo(13, 1), LineTo(13, 13), LineTo(1, 13), ClosePath(),
		MoveTo(5, 5), LineTo(5, 9), LineTo(9, 9), LineTo(9, 5), ClosePath(),
		MoveTo(6, 6), LineTo(8, 6), LineTo(8, 8), LineTo(6, 8), ClosePath(),
	}
	var canvas Canvas
	canvas.DrawPath(NewPath(14, 14, commands...), Rect{Width: 14, Height: 14}, PathStyle{Fill: Color{Alpha: 1}})
	if area := triangleCoverArea(canvas.Triangles()); math.Abs(area-132) > 0.5 {
		t.Fatalf("island-in-hole area = %v, want 132", area)
	}
}

func TestDrawPathStrokeMatchesRectangleArea(t *testing.T) {
	// An open square missing its left edge, stroked with width 2 around a
	// 8x8 path: three 8x2 bands = 48 area units, plus the 1x1 square a miter
	// join fills at each of the two outer right-angle corners.
	path := NewPath(10, 10, MoveTo(1, 1), LineTo(9, 1), LineTo(9, 9), LineTo(1, 9))
	var canvas Canvas
	canvas.DrawPath(path, Rect{Width: 10, Height: 10}, PathStyle{
		Stroke: Color{Alpha: 1}, StrokeWidth: 2, Cap: CapButt, Join: JoinMiter,
	})

	if area := triangleCoverArea(canvas.Triangles()); math.Abs(area-50) > 0.1 {
		t.Fatalf("stroke area = %v, want 50", area)
	}
}

// Every join style closes the outer side of a corner: before, only round joins
// did, and a miter or bevel corner was left with a notch.
func TestDrawPathOpaqueStrokeJoinsFillTheOuterCorner(t *testing.T) {
	path := NewPath(10, 10, MoveTo(1, 1), LineTo(9, 1), LineTo(9, 9), LineTo(1, 9))
	for _, test := range []struct {
		name string
		join StrokeJoin
		area float64 // three 8x2 bands, plus what the two outer corners add
	}{
		{"miter", JoinMiter, 48 + 2*1},    // a 1x1 square at each corner
		{"bevel", JoinBevel, 48 + 2*0.5},  // half of it
		{"round", JoinRound, 48 + 2*0.78}, // a quarter disc of radius 1, approximated by chords
	} {
		var canvas Canvas
		canvas.DrawPath(path, Rect{Width: 10, Height: 10}, PathStyle{
			Stroke: Color{Alpha: 1}, StrokeWidth: 2, Cap: CapButt, Join: test.join,
		})
		if area := triangleCoverArea(canvas.Triangles()); math.Abs(area-test.area) > 0.08 {
			t.Errorf("%s: stroke area = %v, want about %v", test.name, area, test.area)
		}
	}
}

// A smooth curve keeps its accuracy: the stroke follows the circle it was cut
// from to within a fraction of a pixel instead of being simplified to a coarse
// polygon whose error grows with the stroke width.
func TestDrawPathStrokedCircleStaysRound(t *testing.T) {
	const radius, width = 8, 1.5
	path := NewPath(24, 24, MoveTo(20, 12), CubicTo(20, 16.4183, 16.4183, 20, 12, 20), CubicTo(7.5817, 20, 4, 16.4183, 4, 12),
		CubicTo(4, 7.5817, 7.5817, 4, 12, 4), CubicTo(16.4183, 4, 20, 7.5817, 20, 12), ClosePath())
	var canvas Canvas
	canvas.DrawPath(path, Rect{Width: 24, Height: 24}, PathStyle{
		Stroke: Color{Alpha: 1}, StrokeWidth: width, Cap: CapRound, Join: JoinRound,
	})
	triangles := canvas.Triangles()
	if len(triangles) == 0 {
		t.Fatal("nothing drawn")
	}
	inner, outer := float64(radius-width/2), float64(radius+width/2)
	for _, triangle := range triangles {
		for _, point := range triangle.Points {
			distance := math.Hypot(float64(point.X)-12, float64(point.Y)-12)
			if distance < inner-0.05 || distance > outer+0.05 {
				t.Fatalf("stroke vertex %+v is %.3f from the center, outside the band [%.2f, %.2f]", point, distance, inner, outer)
			}
		}
	}
}

func TestDrawPathOpaqueStrokeDoesNotFeatherItsTriangleDiagonal(t *testing.T) {
	path := NewPath(10, 10, MoveTo(1, 5), LineTo(9, 5))
	var canvas Canvas
	canvas.DrawPath(path, Rect{Width: 10, Height: 10}, PathStyle{
		Stroke: Color{Alpha: 1}, StrokeWidth: 2, Cap: CapButt,
	})

	triangles := canvas.Triangles()
	if len(triangles) != 2 {
		t.Fatalf("stroke triangles = %d, want 2", len(triangles))
	}
	if triangles[0].EdgeMask != 1<<0|1<<2 || triangles[1].EdgeMask != 1<<1|1<<2 {
		t.Fatalf("stroke edge masks = %03b, %03b; shared diagonal must stay opaque", triangles[0].EdgeMask, triangles[1].EdgeMask)
	}
}

func TestDrawPathStrokeOverlapPreservesAlpha(t *testing.T) {
	// Translucent strokes must composite as if painted once: any triangle
	// overlap would darken seams. Sample the diagonal join corner band.
	path := NewPath(10, 10, MoveTo(2, 8), LineTo(8, 8), LineTo(8, 2))
	var canvas Canvas
	canvas.DrawPath(path, Rect{Width: 20, Height: 20}, PathStyle{
		Stroke: Color{Red: 1, Alpha: 0.5}, StrokeWidth: 3, Cap: CapRound, Join: JoinRound,
	})

	overlaps := 0
	triangles := canvas.Triangles()
	for i := 0; i < len(triangles); i++ {
		for j := i + 1; j < len(triangles); j++ {
			if trianglesOverlap(triangles[i], triangles[j]) {
				overlaps++
			}
		}
	}
	if overlaps > 0 {
		t.Fatalf("stroke triangles overlap %d times; translucent strokes would show seams", overlaps)
	}
}

func TestDrawPathFlattensCurvesWithinTolerance(t *testing.T) {
	// A quadratic that bows to y=5 at the middle of a 8-wide span.
	path := NewPath(10, 10, MoveTo(1, 9), QuadTo(5, -11, 9, 9), ClosePath())
	var canvas Canvas
	canvas.DrawPath(path, Rect{Width: 10, Height: 10}, PathStyle{Fill: Color{Alpha: 1}})

	if len(canvas.Triangles()) < 3 {
		t.Fatalf("triangles = %d; curve did not flatten into a polygon", len(canvas.Triangles()))
	}
	// The curve peak (5, 5) must fall inside the flattened fill.
	inside := false
	for _, triangle := range canvas.Triangles() {
		if pointInTriangle(Point{X: 5, Y: 5}, triangle.Points[0], triangle.Points[1], triangle.Points[2]) {
			inside = true
			break
		}
	}
	if !inside {
		t.Fatal("flattened curve does not cover its apex")
	}
}

func TestDrawPathPreservesClip(t *testing.T) {
	path := NewPath(4, 4, MoveTo(0, 0), LineTo(4, 0), LineTo(4, 4), ClosePath())
	var canvas Canvas
	clip := Rect{X: 2, Y: 3, Width: 4, Height: 5}
	canvas.Clip(clip)
	canvas.DrawPath(path, Rect{Width: 8, Height: 8}, PathStyle{Fill: Color{Alpha: 1}})
	canvas.Unclip()

	if len(canvas.Triangles()) != 1 || !canvas.Triangles()[0].HasClip || canvas.Triangles()[0].ClipOf != clip {
		t.Fatalf("clipped triangle = %+v", canvas.Triangles())
	}
}

func TestSteadyStatePathPaintDoesNotAllocate(t *testing.T) {
	path := NewPath(24, 24,
		MoveTo(2, 12), CubicTo(8, 2, 16, 22, 22, 12),
		LineTo(22, 18), QuadTo(12, 20, 2, 18), ClosePath(),
	)
	var canvas Canvas
	paint := func() {
		canvas.Reset()
		canvas.DrawPath(path, Rect{Width: 24, Height: 24}, PathStyle{
			Fill:   Color{Alpha: 0.4},
			Stroke: Color{Alpha: 1}, StrokeWidth: 2, Cap: CapRound, Join: JoinRound,
		})
	}
	paint()
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("steady-state path allocations = %v", allocations)
	}
}

// triangleCoverArea sums triangle areas. Overlaps are counted twice, so
// exact expected areas also prove non-overlap.
func triangleCoverArea(triangles []Triangle) float64 {
	total := 0.0
	for _, triangle := range triangles {
		ax, ay := float64(triangle.Points[0].X), float64(triangle.Points[0].Y)
		bx, by := float64(triangle.Points[1].X), float64(triangle.Points[1].Y)
		cx, cy := float64(triangle.Points[2].X), float64(triangle.Points[2].Y)
		total += math.Abs(ax*(by-cy)+bx*(cy-ay)+cx*(ay-by)) * 0.5
	}
	return total
}

func assertBoundaryMasks(t *testing.T, triangles []Triangle, wantBoundaryEdges int) {
	t.Helper()
	type edgeRef struct {
		triangle int
		edge     uint8
	}
	edges := make(map[[4]uint32][]edgeRef, len(triangles)*3)
	for triangleIndex, triangle := range triangles {
		if triangle.EdgeMask&^triangleAllEdges != 0 {
			t.Fatalf("triangle %d has invalid edge mask %03b", triangleIndex, triangle.EdgeMask)
		}
		for edge := uint8(0); edge < 3; edge++ {
			key := canonicalEdge(triangle.Points[edge], triangle.Points[(edge+1)%3])
			edges[key] = append(edges[key], edgeRef{triangle: triangleIndex, edge: edge})
		}
	}
	boundaryEdges := 0
	for key, references := range edges {
		boundary := len(references) == 1
		if boundary {
			boundaryEdges++
		}
		for _, reference := range references {
			marked := triangles[reference.triangle].EdgeMask&(1<<reference.edge) != 0
			if marked != boundary {
				t.Fatalf("edge %v occurs %d times but triangle %d edge %d marked=%v", key, len(references), reference.triangle, reference.edge, marked)
			}
		}
	}
	if boundaryEdges != wantBoundaryEdges {
		t.Fatalf("boundary edges = %d, want %d", boundaryEdges, wantBoundaryEdges)
	}
}

// trianglesOverlap reports whether two triangles share area beyond a hair
// of numerical tolerance. Triangles meeting at a shared edge or vertex do
// not overlap: the shared-boundary vertices are excluded first.
func trianglesOverlap(a, b Triangle) bool {
	shared := func(p Point) bool {
		for _, q := range b.Points {
			if distance(p, q) < 1e-3 {
				return true
			}
		}
		return false
	}
	nonSharedA := 0
	for _, p := range a.Points {
		if !shared(p) {
			nonSharedA++
		}
	}
	if nonSharedA == 0 {
		return false // identical or fully shared-boundary
	}
	sharedA := func(p Point) bool {
		for _, q := range b.Points {
			if distance(p, q) < 1e-3 {
				return true
			}
		}
		return false
	}
	nonSharedB := 0
	for _, p := range b.Points {
		if !sharedA(p) {
			nonSharedB++
		}
	}
	if nonSharedB == 0 {
		return false
	}
	aPoints := [][2]float64{
		{float64(a.Points[0].X), float64(a.Points[0].Y)},
		{float64(a.Points[1].X), float64(a.Points[1].Y)},
		{float64(a.Points[2].X), float64(a.Points[2].Y)},
	}
	bPoints := [][2]float64{
		{float64(b.Points[0].X), float64(b.Points[0].Y)},
		{float64(b.Points[1].X), float64(b.Points[1].Y)},
		{float64(b.Points[2].X), float64(b.Points[2].Y)},
	}
	for _, p := range aPoints {
		if !sharedA(Point{X: float32(p[0]), Y: float32(p[1])}) &&
			pointInTriangleF(p, bPoints[0], bPoints[1], bPoints[2]) {
			return true
		}
	}
	for _, p := range bPoints {
		if !shared(Point{X: float32(p[0]), Y: float32(p[1])}) &&
			pointInTriangleF(p, aPoints[0], aPoints[1], aPoints[2]) {
			return true
		}
	}
	for i := range aPoints {
		for j := range bPoints {
			if segmentsCrossF(
				aPoints[i], aPoints[(i+1)%3],
				bPoints[j], bPoints[(j+1)%3],
			) {
				return true
			}
		}
	}
	return false
}

func pointInTriangleF(p, a, b, c [2]float64) bool {
	const eps = 1e-6
	d1 := crossF(a, b, p)
	d2 := crossF(b, c, p)
	d3 := crossF(c, a, p)
	hasNegative := d1 < -eps || d2 < -eps || d3 < -eps
	hasPositive := d1 > eps || d2 > eps || d3 > eps
	return !(hasNegative && hasPositive)
}

func crossF(a, b, p [2]float64) float64 {
	return (b[0]-a[0])*(p[1]-a[1]) - (b[1]-a[1])*(p[0]-a[0])
}

func segmentsCrossF(a, b, c, d [2]float64) bool {
	d1 := crossF(c, d, a)
	d2 := crossF(c, d, b)
	d3 := crossF(a, b, c)
	d4 := crossF(a, b, d)
	const eps = 1e-6
	return ((d1 > eps && d2 < -eps) || (d1 < -eps && d2 > eps)) &&
		((d3 > eps && d4 < -eps) || (d3 < -eps && d4 > eps))
}

// A multi-color icon paints every layer in its own color, scaled by the tint's
// alpha; the tint's color does not leak into it.
func TestDrawIconPaintsColoredLayersInTheirOwnColors(t *testing.T) {
	red, green := Color{Red: 1, Alpha: 1}, Color{Green: 1, Alpha: 1}
	icon := Icon{ViewBox: 16, Layers: []IconLayer{
		{Fill: red, Commands: []PathCommand{MoveTo(2, 2), LineTo(8, 2), LineTo(8, 8), LineTo(2, 8), ClosePath()}},
		{Fill: green, Commands: []PathCommand{MoveTo(9, 9), LineTo(14, 9), LineTo(14, 14), LineTo(9, 14), ClosePath()}},
	}}
	var canvas Canvas
	canvas.DrawIcon(icon, Rect{Width: 16, Height: 16}, Color{Blue: 1, Alpha: 0.5})
	var sawRed, sawGreen bool
	for _, triangle := range canvas.Triangles() {
		switch {
		case triangle.Color == (Color{Red: 1, Alpha: 0.5}):
			sawRed = true
		case triangle.Color == (Color{Green: 1, Alpha: 0.5}):
			sawGreen = true
		default:
			t.Fatalf("a triangle was painted %+v, want a layer color at half alpha", triangle.Color)
		}
	}
	if !sawRed || !sawGreen {
		t.Fatalf("layers painted: red %v green %v", sawRed, sawGreen)
	}
	canvas.Reset()
	canvas.DrawIcon(icon, Rect{Width: 16, Height: 16}, Color{})
	if len(canvas.Triangles()) != 0 {
		t.Fatal("an invisible tint still painted the layers")
	}
}

// coverage counts how many of a canvas's triangles contain a point.
func coverage(canvas *Canvas, x, y float32) int {
	count := 0
	for _, triangle := range canvas.Triangles() {
		a, b, c := triangle.Points[0], triangle.Points[1], triangle.Points[2]
		d1 := (x-b.X)*(a.Y-b.Y) - (a.X-b.X)*(y-b.Y)
		d2 := (x-c.X)*(b.Y-c.Y) - (b.X-c.X)*(y-c.Y)
		d3 := (x-a.X)*(c.Y-a.Y) - (c.X-a.X)*(y-a.Y)
		if !((d1 < 0 || d2 < 0 || d3 < 0) && (d1 > 0 || d2 > 0 || d3 > 0)) {
			count++
		}
	}
	return count
}

// A fill's triangles must cover exactly the region its rule selects, each point
// once: no triangle spills over a hole, none overlaps another.
func TestFillTrianglesCoverExactlyTheFilledRegion(t *testing.T) {
	rect := func(x0, y0, x1, y1 float32, clockwise bool) []PathCommand {
		if clockwise {
			return []PathCommand{MoveTo(x0, y0), LineTo(x1, y0), LineTo(x1, y1), LineTo(x0, y1), ClosePath()}
		}
		return []PathCommand{MoveTo(x0, y0), LineTo(x0, y1), LineTo(x1, y1), LineTo(x1, y0), ClosePath()}
	}
	join := func(parts ...[]PathCommand) []PathCommand {
		var all []PathCommand
		for _, part := range parts {
			all = append(all, part...)
		}
		return all
	}
	octagon := []PathCommand{MoveTo(2, 0), LineTo(0, 2), LineTo(0, 14), LineTo(2, 16), LineTo(14, 16), LineTo(16, 14), LineTo(16, 2), LineTo(14, 0), ClosePath()}
	chevron := []PathCommand{MoveTo(3, 7), LineTo(5, 9), LineTo(3, 11), LineTo(4, 12), LineTo(7, 9), LineTo(4, 6), ClosePath()}
	star := func() []PathCommand {
		var commands []PathCommand
		for index := 0; index < 10; index++ {
			angle := float64(index) * math.Pi / 5
			radius := float64(7)
			if index%2 == 1 {
				radius = 3
			}
			x, y := float32(8+radius*math.Cos(angle)), float32(8+radius*math.Sin(angle))
			if index == 0 {
				commands = append(commands, MoveTo(x, y))
			} else {
				commands = append(commands, LineTo(x, y))
			}
		}
		return append(commands, ClosePath())
	}()
	for _, test := range []struct {
		name     string
		commands []PathCommand
		rule     PathFillRule
	}{
		{"square with a rectangular hole", join(rect(0, 0, 16, 16, false), rect(2, 5, 14, 13, true)), PathFillNonZero},
		{"octagon with a rectangular hole", join(octagon, rect(2, 5, 14, 13, true)), PathFillNonZero},
		{"terminal glyph", join(octagon, rect(2, 5, 14, 13, true), chevron, rect(8, 10.5, 13, 12, true)), PathFillNonZero},
		{"two holes", join(rect(0, 0, 16, 16, false), rect(2, 2, 7, 14, true), rect(9, 2, 14, 14, true)), PathFillNonZero},
		{"hole with an island", join(rect(0, 0, 16, 16, false), rect(2, 2, 14, 14, true), rect(6, 6, 10, 10, false)), PathFillNonZero},
		{"same winding even-odd", join(rect(0, 0, 16, 16, true), rect(4, 4, 12, 12, true)), PathFillEvenOdd},
		{"star with a hole", join(star, rect(6, 6, 10, 10, false)), PathFillNonZero},
	} {
		t.Run(test.name, func(t *testing.T) {
			var canvas Canvas
			canvas.DrawPath(NewPath(16, 16, test.commands...), Rect{Width: 16, Height: 16}, PathStyle{Fill: Color{Alpha: 1}, FillRule: test.rule})
			// Reference: the rule evaluated directly on the contours.
			var path Canvas
			path.DrawPath(NewPath(16, 16, test.commands...), Rect{Width: 16, Height: 16}, PathStyle{})
			contours := make([][]Point, 0, 4)
			start := int32(0)
			for _, end := range canvas.contourEnds {
				contours = append(contours, append([]Point(nil), canvas.pathPoints[start:end]...))
				start = end
			}
			for x := float32(0.13); x < 16; x += 0.5 {
				for y := float32(0.29); y < 16; y += 0.5 {
					winding := 0
					for _, contour := range contours {
						winding += windingNumber(Point{X: x, Y: y}, contour)
					}
					want := 0
					if test.rule == PathFillEvenOdd {
						want = abs(winding) % 2
					} else if winding != 0 {
						want = 1
					}
					if got := coverage(&canvas, x, y); got != want {
						t.Fatalf("(%.2f,%.2f) is covered %d times, want %d", x, y, got, want)
					}
				}
			}
		})
	}
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

// Random star-shaped fills with a hole stay exact: the triangulator must never
// spill over the hole, overlap itself, or stall.
func TestRandomFillsWithHolesStayExact(t *testing.T) {
	random := rand.New(rand.NewSource(7))
	for round := 0; round < 200; round++ {
		points := 5 + random.Intn(30)
		inner := 3 + random.Float64()*2 // every radius is at least this
		commands := make([]PathCommand, 0, points+8)
		for index := 0; index < points; index++ {
			angle := 2 * math.Pi * float64(index) / float64(points)
			radius := inner + random.Float64()*(7.5-inner)
			x, y := float32(8+radius*math.Cos(angle)), float32(8+radius*math.Sin(angle))
			if index == 0 {
				commands = append(commands, MoveTo(x, y))
			} else {
				commands = append(commands, LineTo(x, y))
			}
		}
		commands = append(commands, ClosePath())
		// A square hole inside the inscribed circle, wound against the outline.
		half := float32(inner/1.5) * random.Float32()
		half = max(half, 0.4)
		centerX, centerY := float32(8)+(random.Float32()-0.5)*0.4, float32(8)+(random.Float32()-0.5)*0.4
		commands = append(commands,
			MoveTo(centerX-half, centerY-half), LineTo(centerX-half, centerY+half), LineTo(centerX+half, centerY+half), LineTo(centerX+half, centerY-half), ClosePath())

		var canvas Canvas
		canvas.DrawPath(NewPath(16, 16, commands...), Rect{Width: 16, Height: 16}, PathStyle{Fill: Color{Alpha: 1}})
		contours := make([][]Point, 0, 2)
		start := int32(0)
		for _, end := range canvas.contourEnds {
			contours = append(contours, append([]Point(nil), canvas.pathPoints[start:end]...))
			start = end
		}
		for x := float32(0.11); x < 16; x += 0.37 {
			for y := float32(0.23); y < 16; y += 0.37 {
				winding := 0
				for _, contour := range contours {
					winding += windingNumber(Point{X: x, Y: y}, contour)
				}
				want := 0
				if winding != 0 {
					want = 1
				}
				if got := coverage(&canvas, x, y); got != want {
					t.Fatalf("round %d: (%.2f,%.2f) is covered %d times, want %d", round, x, y, got, want)
				}
			}
		}
	}
}
