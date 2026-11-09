package components

import (
	"math"
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
)

// spinnerSpokeRectangles returns the rectangles painted for one spoke:
// straight spokes are one pill, diagonal spokes are a staircase of strips.
func spinnerSpokeRectangles(canvas *gfx.Canvas, spoke int) []gfx.Rectangle {
	start := spoke/2*(1+spinnerDiagonalRows) + spoke%2
	count := 1
	if spoke%2 == 1 {
		count = spinnerDiagonalRows
	}
	return canvas.Rectangles()[start : start+count]
}

func TestSpinnerPaintsExplicitOfficialSizedFrame(t *testing.T) {
	var canvas gfx.Canvas
	layout := PaintSpinner(&canvas, gfx.LightTheme(), gfx.Rect{X: 10, Y: 20, Width: 20, Height: 24}, 10)
	if layout != (SpinnerLayout{Bounds: gfx.Rect{X: 12, Y: 24, Width: 16, Height: 16}, Phase: 2, Label: "Loading"}) {
		t.Fatalf("layout = %+v", layout)
	}
	if want := 4 + 4*spinnerDiagonalRows; len(canvas.Rectangles()) != want || len(canvas.Texts()) != 0 || len(canvas.Hits()) != 0 {
		t.Fatalf("spinner primitives: rectangles=%d (want %d) texts=%d hits=%d", len(canvas.Rectangles()), want, len(canvas.Texts()), len(canvas.Hits()))
	}
	for index, rectangle := range canvas.Rectangles() {
		if rectangle.Color.Alpha <= 0 || rectangle.Color.Alpha > 1 {
			t.Fatalf("rectangle %d alpha = %v", index, rectangle.Color.Alpha)
		}
	}
}

func TestSpinnerFadeTrailsBehindTheClockwiseHead(t *testing.T) {
	var canvas gfx.Canvas
	PaintSpinner(&canvas, gfx.LightTheme(), gfx.Rect{Width: 16, Height: 16}, 2)
	alpha := func(spoke int) float32 {
		rectangles := spinnerSpokeRectangles(&canvas, spoke)
		for _, rectangle := range rectangles {
			if rectangle.Color != rectangles[0].Color {
				t.Fatalf("spoke %d is not one uniform color: %+v", spoke, rectangles)
			}
		}
		return rectangles[0].Color.Alpha
	}
	if head := alpha(2); head != 1 {
		t.Fatalf("head alpha = %v", head)
	}
	// Spokes the head has already passed fade in order, counterclockwise from
	// it; the spoke it is about to reach is the faintest.
	for distance := 1; distance < 8; distance++ {
		want := 1 - float32(distance)*0.10
		if got := alpha((2 - distance + 8) % 8); math.Abs(float64(got-want)) > 1e-6 {
			t.Fatalf("spoke %d behind the head has alpha %v, want %v", distance, got, want)
		}
	}
	if next, behind := alpha(3), alpha(1); next >= behind {
		t.Fatalf("the spoke ahead of the head (%v) must be fainter than the one behind it (%v)", next, behind)
	}
}

func TestSpinnerDiagonalSpokesAreSlantedStaircases(t *testing.T) {
	var canvas gfx.Canvas
	PaintSpinner(&canvas, gfx.LightTheme(), gfx.Rect{Width: 16, Height: 16}, 0)
	// Outward direction of each diagonal spoke: top-right, bottom-right,
	// bottom-left, top-left.
	directions := [4][2]float32{{1, -1}, {1, 1}, {-1, 1}, {-1, -1}}
	for diagonal, direction := range directions {
		rows := spinnerSpokeRectangles(&canvas, diagonal*2+1)
		for index := 1; index < len(rows); index++ {
			previous, row := rows[index-1], rows[index]
			if math.Abs(float64(row.Bounds.Y-(previous.Bounds.Y+previous.Bounds.Height))) > 1e-4 {
				t.Fatalf("diagonal %d strips must abut vertically: %+v then %+v", diagonal, previous.Bounds, row.Bounds)
			}
			shift := (row.Bounds.X + row.Bounds.Width/2) - (previous.Bounds.X + previous.Bounds.Width/2)
			if want := spinnerDiagonalStep * direction[0] * direction[1]; math.Abs(float64(shift-want)) > 1e-4 {
				t.Fatalf("diagonal %d strip %d moved %v sideways, want %v for a 45 degree edge", diagonal, index, shift, want)
			}
		}
		first, last := rows[0], rows[len(rows)-1]
		if first.Bounds.Width >= rows[1].Bounds.Width || last.Bounds.Width >= rows[len(rows)-2].Bounds.Width {
			t.Fatalf("diagonal %d tips must be narrower than the body: %+v", diagonal, rows)
		}
		middle := rows[len(rows)/2]
		x, y := middle.Bounds.X+middle.Bounds.Width/2-8, middle.Bounds.Y-8
		if x*direction[0] <= 0 || y*direction[1] <= 0 {
			t.Fatalf("diagonal %d points away from its quadrant: offset (%v, %v)", diagonal, x, y)
		}
	}
}

func TestSpinnerSpokesAreSymmetricAndStayInsideTheIcon(t *testing.T) {
	var canvas gfx.Canvas
	for _, bounds := range []gfx.Rect{{Width: 16, Height: 16}, {X: 3, Y: 5, Width: 8, Height: 8}, {X: 1, Y: 2, Width: 24, Height: 24}} {
		canvas.Reset()
		layout := PaintSpinner(&canvas, gfx.DarkTheme(), bounds, 0)
		center := layout.Bounds.X + layout.Bounds.Width/2
		middle := layout.Bounds.Y + layout.Bounds.Height/2
		for index, rectangle := range canvas.Rectangles() {
			b := rectangle.Bounds
			if b.X < layout.Bounds.X || b.Y < layout.Bounds.Y ||
				b.X+b.Width > layout.Bounds.X+layout.Bounds.Width || b.Y+b.Height > layout.Bounds.Y+layout.Bounds.Height {
				t.Fatalf("bounds %+v: rectangle %d %+v leaves the icon %+v", bounds, index, b, layout.Bounds)
			}
		}
		const tolerance = 1e-4
		near := func(a, b float32) bool { return math.Abs(float64(a-b)) < tolerance }
		topRight, topLeft := spinnerSpokeRectangles(&canvas, 1), spinnerSpokeRectangles(&canvas, 7)
		bottomRight := spinnerSpokeRectangles(&canvas, 3)
		for row := range topRight {
			right, left := topRight[row].Bounds, topLeft[row].Bounds
			if !near(left.X, 2*center-(right.X+right.Width)) || !near(left.Y, right.Y) || !near(left.Width, right.Width) {
				t.Fatalf("bounds %+v: top-left row %d %+v does not mirror top-right %+v", bounds, row, left, right)
			}
			below := bottomRight[len(bottomRight)-1-row].Bounds
			if !near(below.X, right.X) || !near(below.Y, 2*middle-(right.Y+right.Height)) || !near(below.Width, right.Width) {
				t.Fatalf("bounds %+v: bottom-right row %d %+v does not mirror top-right %+v", bounds, row, below, right)
			}
		}
	}
}

func TestSpinnerFitsSmallBoundsAndPaintsDeterministically(t *testing.T) {
	var canvas gfx.Canvas
	paint := func() {
		canvas.Reset()
		PaintSpinner(&canvas, gfx.DarkTheme(), gfx.Rect{X: 1, Y: 2, Width: 8, Height: 12}, 7)
	}
	paint()
	if got := canvas.Rectangles()[0].Bounds; got != (gfx.Rect{X: 4.5, Y: 4, Width: 1, Height: 2}) {
		t.Fatalf("first segment = %+v", got)
	}
	rectangles := slices.Clone(canvas.Rectangles())
	paint()
	if !slices.Equal(rectangles, canvas.Rectangles()) {
		t.Fatal("spinner frame changed")
	}
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("spinner allocations = %v", allocations)
	}
}

func TestSpinnerRejectsEmptyPaint(t *testing.T) {
	var canvas gfx.Canvas
	if layout := PaintSpinner(&canvas, gfx.LightTheme(), gfx.Rect{Width: 0, Height: 16}, 0); layout != (SpinnerLayout{}) {
		t.Fatalf("empty layout = %+v", layout)
	}
	if len(canvas.Rectangles()) != 0 {
		t.Fatalf("empty spinner painted %+v", canvas.Rectangles())
	}
}

func BenchmarkSpinner(b *testing.B) {
	var canvas gfx.Canvas
	theme := gfx.LightTheme()
	bounds := gfx.Rect{Width: 24, Height: 24}
	PaintSpinner(&canvas, theme, bounds, 0)
	b.ReportAllocs()
	b.ResetTimer()
	for index := range b.N {
		canvas.Reset()
		PaintSpinner(&canvas, theme, bounds, SpinnerPhase(index))
	}
}
