package gfx

import (
	"math"
	"testing"
)

var testGradientIcon = Icon{
	ViewBox: 24,
	Layers: []IconLayer{{
		Fill: Color{Red: 0.9, Green: 0.1, Blue: 0.5, Alpha: 1},
		Gradient: &LinearGradient{
			Start: Point{X: 3, Y: 0}, End: Point{X: 21, Y: 0},
			From: Color{Red: 0.9, Green: 0.1, Blue: 0.5, Alpha: 1}, To: Color{Red: 1, Green: 0.4, Blue: 0.2, Alpha: 1},
		},
		// A circle-ish blob: curved boundary, so cuts meet boundary edges at
		// every angle.
		Commands: []PathCommand{
			MoveTo(12, 2), CubicTo(18, 2, 22, 6, 22, 12), CubicTo(22, 18, 18, 22, 12, 22),
			CubicTo(6, 22, 2, 18, 2, 12), CubicTo(2, 6, 6, 2, 12, 2), ClosePath(),
		},
	}},
}

// solidTwin is the gradient icon with a plain fill: the shape the pieces must tile.
func solidTwin() Icon {
	twin := testGradientIcon
	twin.Layers = []IconLayer{{Fill: testGradientIcon.Layers[0].Fill, Commands: testGradientIcon.Layers[0].Commands}}
	return twin
}

// boundaryLength is the total length of the edges triangles mark as the shape's
// outline, which renderers feather.
func boundaryLength(triangles []Triangle) float64 {
	total := 0.0
	for _, triangle := range triangles {
		for edge := 0; edge < 3; edge++ {
			if triangle.EdgeMask>>edge&1 == 0 {
				continue
			}
			a, b := triangle.Points[edge], triangle.Points[(edge+1)%3]
			total += math.Hypot(float64(b.X-a.X), float64(b.Y-a.Y))
		}
	}
	return total
}

func TestGradientLayerTilesTheShapeAndKeepsItsOutline(t *testing.T) {
	var solid, shaded Canvas
	bounds := Rect{Width: 32, Height: 32}
	solid.DrawIcon(solidTwin(), bounds, Color{Alpha: 1})
	shaded.DrawIcon(testGradientIcon, bounds, Color{Alpha: 1})

	if got, want := triangleArea(shaded.Triangles()), triangleArea(solid.Triangles()); math.Abs(got-want) > 1e-3*want {
		t.Fatalf("pieces cover area %.4f, the solid shape %.4f", got, want)
	}
	// Cuts are interior: if they were feathered the bands would show seams.
	if got, want := boundaryLength(shaded.Triangles()), boundaryLength(solid.Triangles()); math.Abs(got-want) > 1e-3*want {
		t.Fatalf("feathered outline is %.4f long, the solid shape's %.4f", got, want)
	}
	// Every point is covered once, as in the solid shape.
	for x := 1.0; x < 31; x += 0.173 {
		for y := 1.0; y < 31; y += 0.173 {
			if a, b := triangleCoverage(shaded.Triangles(), x, y), triangleCoverage(solid.Triangles(), x, y); a != b {
				t.Fatalf("at (%.2f, %.2f) the pieces cover %d times, the solid shape %d", x, y, a, b)
			}
		}
	}
}

// vertexColors lists the color at each corner of a triangle.
func vertexColors(triangle Triangle) [3]Color {
	if !triangle.Shaded {
		return [3]Color{triangle.Color, triangle.Color, triangle.Color}
	}
	return [3]Color{triangle.Color, triangle.Shade[0], triangle.Shade[1]}
}

func TestGradientLayerColorsEveryVertexFromTheGradient(t *testing.T) {
	var canvas Canvas
	canvas.DrawIcon(testGradientIcon, Rect{Width: 32, Height: 32}, Color{Alpha: 1})
	gradient := testGradientIcon.Layers[0].Gradient

	shaded, atFrom, atTo := 0, false, false
	for _, triangle := range canvas.Triangles() {
		if triangle.Shaded {
			shaded++
		}
		for index, color := range vertexColors(triangle) {
			point := triangle.Points[index]
			want := gradient.At((point.X/32*24 - 3) / 18)
			if !colorsClose(color, want) {
				t.Fatalf("vertex (%.2f, %.2f) is %+v, the gradient is %+v there", point.X, point.Y, color, want)
			}
			atFrom = atFrom || color == gradient.From
			atTo = atTo || color == gradient.To
		}
	}
	if shaded == 0 || !atFrom || !atTo {
		t.Fatalf("%d shaded triangles; reaches From %v, To %v", shaded, atFrom, atTo)
	}
	// Beyond the ends the gradient is flat, so those triangles are not shaded: the
	// renderers' interpolation is only asked for where the color changes.
	if shaded == len(canvas.Triangles()) {
		t.Fatal("every triangle is shaded, none lies beyond an end of the gradient")
	}
}

func TestGradientWithoutLengthPaintsItsLastColor(t *testing.T) {
	icon := testGradientIcon
	layer := *icon.Layers[0].Gradient
	layer.End = layer.Start
	icon.Layers = []IconLayer{{Fill: icon.Layers[0].Fill, Gradient: &layer, Commands: testGradientIcon.Layers[0].Commands}}
	var canvas Canvas
	canvas.DrawIcon(icon, Rect{Width: 16, Height: 16}, Color{Alpha: 1})
	for _, triangle := range canvas.Triangles() {
		if triangle.Shaded || triangle.Color != layer.To {
			t.Fatalf("zero-length gradient painted %+v", triangle)
		}
	}
}

func TestGradientIconCachedMatchesDirectAndTintFadesIt(t *testing.T) {
	for _, alpha := range []float32{1, 0.5} {
		var cached, direct Canvas
		bounds := Rect{X: 10, Y: 6, Width: 32, Height: 32}
		cached.DrawIcon(testGradientIcon, bounds, Color{Alpha: alpha})
		cached.Reset()
		cached.DrawIcon(testGradientIcon, bounds, Color{Alpha: alpha})
		direct.drawIconDirect(testGradientIcon, bounds, Color{Alpha: alpha})
		sameTriangles(t, cached.Triangles(), direct.Triangles())
		for _, triangle := range cached.Triangles() {
			for _, color := range vertexColors(triangle) {
				if color.Alpha > alpha+1e-6 {
					t.Fatalf("tint alpha %v, vertex alpha %v", alpha, color.Alpha)
				}
			}
		}
	}
}

func TestGradientIconReplayAllocatesNothing(t *testing.T) {
	var canvas Canvas
	bounds := Rect{Width: 16, Height: 16}
	paint := func() {
		canvas.Reset()
		canvas.DrawIcon(testGradientIcon, bounds, Color{Alpha: 1})
	}
	paint()
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("replaying a gradient icon allocated %v times", allocations)
	}
}

func BenchmarkGradientIconReplay(b *testing.B) {
	var canvas Canvas
	bounds := Rect{Width: 16, Height: 16}
	canvas.DrawIcon(testGradientIcon, bounds, Color{Alpha: 1})
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		canvas.DrawIcon(testGradientIcon, bounds, Color{Alpha: 1})
	}
}

func BenchmarkGradientIconTessellate(b *testing.B) {
	bounds := Rect{Width: 16, Height: 16}
	b.ReportAllocs()
	for b.Loop() {
		var canvas Canvas
		canvas.DrawIcon(testGradientIcon, bounds, Color{Alpha: 1})
	}
}
