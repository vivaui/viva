package gfx

import (
	"math"
	"testing"
)

// pointCovered reports whether triangles paint the point, ignoring their
// feathering: a solid border covers every point of its band, a dashed one
// leaves gaps behind.
func pointCovered(triangles []Triangle, point Point) bool {
	for _, triangle := range triangles {
		a, b, c := triangle.Points[0], triangle.Points[1], triangle.Points[2]
		d1 := (point.X-b.X)*(a.Y-b.Y) - (a.X-b.X)*(point.Y-b.Y)
		d2 := (point.X-c.X)*(b.Y-c.Y) - (b.X-c.X)*(point.Y-c.Y)
		d3 := (point.X-a.X)*(c.Y-a.Y) - (c.X-a.X)*(point.Y-a.Y)
		negative := d1 < 0 || d2 < 0 || d3 < 0
		positive := d1 > 0 || d2 > 0 || d3 > 0
		if !(negative && positive) {
			return true
		}
	}
	return false
}

// The dashed outline paints a band with holes in it: painted and unpainted
// stretches alternate along the band, which distinguishes it from a solid
// RoundedOutline (all painted) and from nothing (none painted).
func TestDashedRoundedOutlineHasGapsAndPaint(t *testing.T) {
	var canvas Canvas
	ink := Color{Red: 1, Alpha: .5}
	rect := Rect{X: 10, Y: 10, Width: 80, Height: 40}
	canvas.DashedRoundedOutline(rect, 8, 2, ink)
	triangles := canvas.Triangles()
	if len(triangles) == 0 {
		t.Fatal("the dashed border painted no geometry")
	}
	for _, triangle := range triangles {
		if triangle.Color != ink {
			t.Errorf("a translucent dash changed color to %+v", triangle.Color)
		}
	}
	// Sample the band a pixel above the bottom outline: paint and gaps must
	// alternate along it.
	bottom := rect.Y + rect.Height - 1
	painted, gaps := 0, 0
	covered := pointCovered(triangles, Point{X: rect.X + 4, Y: bottom})
	for x := rect.X + 4.5; x < rect.X+rect.Width-4; x += 0.5 {
		next := pointCovered(triangles, Point{X: x, Y: bottom})
		if next {
			painted++
		}
		if next != covered {
			gaps++
			covered = next
		}
	}
	if painted == 0 || gaps < 2 {
		t.Fatalf("%d covered samples and %d coverage changes along the bottom band, want paint and gaps to alternate", painted, gaps)
	}
}

// A rounded outline dashes through its corners: the four corner bands each
// carry paint somewhere in them, not a blank where the dash phase happened to
// end at a corner.
func TestDashedRoundedOutlineCornersCarryPaint(t *testing.T) {
	var canvas Canvas
	canvas.DashedRoundedOutline(Rect{Width: 60, Height: 60}, 12, 2, Color{Alpha: 1})
	triangles := canvas.Triangles()
	// Each corner band is the quarter arc of the outline in that corner: a
	// 12x12 box just inside it.
	corners := [4][2]float32{{0, 0}, {48, 0}, {0, 48}, {48, 48}}
	for _, corner := range corners {
		var covered bool
		for dx := float32(0); dx <= 12 && !covered; dx += 0.25 {
			for dy := float32(0); dy <= 12 && !covered; dy += 0.25 {
				covered = covered || pointCovered(triangles, Point{X: corner[0] + dx, Y: corner[1] + dy})
			}
		}
		if !covered {
			t.Fatalf("corner band %v has no paint", corner)
		}
	}
}

func TestDashedPixelsStayInsideThePaddedOutline(t *testing.T) {
	for _, test := range []struct {
		name   string
		bounds Rect
		radius float32
	}{
		{"square", Rect{Width: 40, Height: 20}, 0},
		{"rounded", Rect{Width: 40, Height: 20}, 8},
		{"pill", Rect{Width: 60, Height: 16}, 16},
	} {
		var canvas Canvas
		canvas.DashedRoundedOutline(test.bounds, test.radius, 2, Color{Alpha: 1})
		if len(canvas.Triangles()) == 0 {
			t.Fatalf("%s: nothing painted", test.name)
		}
		for _, triangle := range canvas.Triangles() {
			for _, point := range triangle.Points {
				if point.X < test.bounds.X-0.5 || point.Y < test.bounds.Y-0.5 ||
					point.X > test.bounds.X+test.bounds.Width+0.5 || point.Y > test.bounds.Y+test.bounds.Height+0.5 {
					t.Fatalf("%s: stroke geometry at %+v is outside the element %+v", test.name, point, test.bounds)
				}
			}
		}
	}
}

// PathStyle.Dashed drives the same walk: a stroked closed line breaks into
// dashes along its arc length, each painted through the stroke pipeline (so a
// translucent stroke stays seam-free and keeps its color).
func TestDrawPathDashedStrokeIsShorterThanSolid(t *testing.T) {
	path := NewPath(40, 20, MoveTo(1, 1), LineTo(39, 1), LineTo(39, 19), LineTo(1, 19), ClosePath())
	var solid, dashed Canvas
	solid.DrawPath(path, Rect{Width: 40, Height: 20}, PathStyle{Stroke: Color{Alpha: .5}, StrokeWidth: 2, Join: JoinMiter})
	dashed.DrawPath(path, Rect{Width: 40, Height: 20}, PathStyle{Stroke: Color{Alpha: .5}, StrokeWidth: 2, Join: JoinMiter, Dashed: true})
	solidArea, dashedArea := triangleCoverArea(solid.Triangles()), triangleCoverArea(dashed.Triangles())
	if len(dashed.Triangles()) == 0 || dashedArea <= 0 {
		t.Fatal("the dashed stroke painted nothing")
	}
	if dashedArea >= solidArea {
		t.Fatalf("dashed stroke area %v, want less than the solid stroke's %v", dashedArea, solidArea)
	}
	for _, triangle := range dashed.Triangles() {
		if triangle.Color != (Color{Alpha: .5}) {
			t.Fatalf("a translucent dash changed color to %+v", triangle.Color)
		}
	}
	// The fitted pattern paints about the 6:4 share of the solid band; the
	// tolerance covers the corner joins of the closed square.
	if math.Abs(float64(dashedArea)-float64(solidArea)*0.6) > float64(solidArea)*0.25 {
		t.Fatalf("dashed stroke area %v, solid %v: not the 6:4 rhythm", dashedArea, solidArea)
	}
}

// A closed outline fits whole pattern periods to its perimeter: no stub dash is
// left at the starting corner and no gap is painted over. Whatever the radius,
// sampling the whole top edge band finds some painted samples and some gaps,
// and the outline is never empty or solid.
func TestDashedOutlineSplitsIntoAlternatingPaintAndGaps(t *testing.T) {
	rect := Rect{X: 5, Y: 5, Width: 100, Height: 58}
	for _, radius := range []float32{0, 6, 14} {
		var canvas Canvas
		canvas.DashedRoundedOutline(rect, radius, 2, Color{Alpha: 1})
		triangles := canvas.Triangles()
		if len(triangles) == 0 {
			t.Fatalf("radius %v: nothing painted", radius)
		}
		covered := 0
		total := 0
		for x := rect.X + 0.5; x < rect.X+rect.Width; x += 0.5 {
			total++
			if pointCovered(triangles, Point{X: x, Y: rect.Y + 1}) {
				covered++
			}
		}
		if covered == 0 || covered == total {
			t.Fatalf("radius %v: %d of %d samples covered, want an alternation", radius, covered, total)
		}
	}
}

func TestDashedOutlineIsAllocationFreeOnAWarmCanvas(t *testing.T) {
	var canvas Canvas
	paint := func() {
		canvas.DashedRoundedOutline(Rect{X: 10, Y: 10, Width: 80, Height: 40}, 8, 2, Color{Alpha: 1})
	}
	paint()
	if allocs := testing.AllocsPerRun(200, paint); allocs != 0 {
		t.Fatalf("warm dashed outline allocates %v per paint, want 0", allocs)
	}
}
