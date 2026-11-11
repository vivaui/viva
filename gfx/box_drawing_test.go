package gfx

import (
	"math"
	"testing"
)

// Every Box Drawing character is drawn as geometry, never left to the font, and stays within its
// cell and the quarter cell a sprite may reach past it.
func TestBoxDrawingDrawsEveryCharacter(t *testing.T) {
	const cellWidth, cellHeight float32 = 10, 20
	for r := rune(0x2500); r <= 0x257F; r++ {
		shapes := glyphShapes(t, r, cellWidth, cellHeight, 1, 15, 1)
		rectangles, triangles := shapes.Rectangles, shapes.Triangles
		if len(rectangles) == 0 && len(triangles) == 0 {
			t.Errorf("%U draws nothing", r)
			continue
		}
		box := geometryBox(rectangles, triangles)
		inside := box.X >= -cellWidth/4 && box.Y >= -cellHeight/4 &&
			box.Right() <= cellWidth*5/4 && box.Bottom() <= cellHeight*5/4
		if !inside || box.Width <= 0 || box.Height <= 0 {
			t.Errorf("%U draws %+v outside its %gx%g cell and its quarter", r, box, cellWidth, cellHeight)
		}
	}
}

// Two cells whose shared edge joins the same stroke join it over the same pixels, and a cell
// that closes a line ends its ink at the stroke it turns into, not at the cell's edge. Together
// these stop a row of rules from breaking and a table's corner from poking out.
func TestBoxDrawingJoinsItsNeighbours(t *testing.T) {
	for _, size := range []struct{ width, height float32 }{{10, 20}, {9, 20}, {10, 25}} {
		leftSpans := map[boxStyle][][2]float32{}
		rightSpans := map[boxStyle][][2]float32{}
		for index, lines := range boxLines {
			r := rune(index) + 0x2500
			rectangles := glyphShapes(t, r, size.width, size.height, 1, 15, 1).Rectangles
			if lines[boxLeft] != boxNone {
				leftSpans[lines[boxLeft]] = append(leftSpans[lines[boxLeft]], edgeSpans(rectangles, 0)...)
			}
			if lines[boxRight] != boxNone {
				rightSpans[lines[boxRight]] = append(rightSpans[lines[boxRight]], edgeSpans(rectangles, size.width)...)
			}
			if lines == [4]boxStyle{} {
				continue // a dash, a corner or a diagonal: nothing ends here
			}
			right, left := float32(0), size.width
			for _, rectangle := range rectangles {
				right = max(right, rectangle.Bounds.Right())
				left = min(left, rectangle.Bounds.X)
			}
			switch {
			case lines[boxRight] == boxNone && right >= size.width:
				t.Errorf("%U closes the line but its ink reaches %g of the %g wide cell", r, right, size.width)
			case lines[boxRight] != boxNone && right != size.width:
				t.Errorf("%U continues right but its ink ends at %g, not the cell's edge %g", r, right, size.width)
			}
			switch {
			case lines[boxLeft] == boxNone && left <= 0:
				t.Errorf("%U opens the line but its ink starts at %g of the %g wide cell", r, left, size.width)
			case lines[boxLeft] != boxNone && left != 0:
				t.Errorf("%U continues left but its ink starts at %g, not the cell's edge 0", r, left)
			}
		}
		for style, rights := range rightSpans {
			lefts := leftSpans[style]
			if len(lefts) == 0 {
				t.Fatalf("nothing joins the %v edge from the left", style)
			}
			for _, right := range rights {
				joined := false
				for _, left := range lefts {
					joined = joined || left == right
				}
				if !joined {
					t.Errorf("a %v stroke ends at %v on the right but its neighbour starts at %v: cell %gx%g",
						style, right, lefts, size.width, size.height)
				}
			}
		}
	}
}

// A rounded corner is one stroke that reaches the two cell edges it opens: the stems and the bend
// are one unbroken chain, so no seam can fall where the curve meets the straight.
func TestBoxArcIsOneStroke(t *testing.T) {
	for _, test := range []struct {
		r                        rune
		left, right, top, bottom bool
	}{
		{'╭', false, true, false, true},
		{'╮', true, false, false, true},
		{'╯', true, false, true, false},
		{'╰', false, true, true, false},
	} {
		shapes := glyphShapes(t, test.r, 10, 20, 1, 15, 1)
		if len(shapes.Rectangles) != 0 {
			t.Errorf("%U draws %d rectangles, want the stems in the one stroke", test.r, len(shapes.Rectangles))
		}
		if pieces := trianglePieces(shapes.Triangles); pieces != 1 {
			t.Fatalf("%U draws its corner in %d pieces, want one stroke", test.r, pieces)
		}
		box := geometryBox(nil, shapes.Triangles)
		switch {
		case test.left && box.X > 0:
			t.Errorf("%U stops at %g, short of the cell's left edge", test.r, box.X)
		case test.right && box.X+box.Width < 10:
			t.Errorf("%U stops at %g, short of the cell's right edge", test.r, box.X+box.Width)
		case test.top && box.Y > 0:
			t.Errorf("%U stops at %g, short of the cell's top edge", test.r, box.Y)
		case test.bottom && box.Y+box.Height < 20:
			t.Errorf("%U stops at %g, short of the cell's bottom edge", test.r, box.Y+box.Height)
		}
	}
}

// edgeSpans is the vertical extent of the ink that reaches x, as the pairs it covers.
func edgeSpans(rectangles []Rectangle, x float32) [][2]float32 {
	var spans [][2]float32
	for _, rectangle := range rectangles {
		bounds := rectangle.Bounds
		if math.Abs(float64(bounds.X-x)) < 1e-4 || math.Abs(float64(bounds.Right()-x)) < 1e-4 {
			spans = append(spans, [2]float32{bounds.Y, bounds.Y + bounds.Height})
		}
	}
	return spans
}

// A rule is whole device pixels at every display scale and follows the font size, at least one
// device pixel: the straight strokes are rectangles, and only the curved and slanted ones are
// allowed off the grid.
func TestBoxDrawingRulesAreWholeDevicePixels(t *testing.T) {
	for _, scale := range []float32{1, 2} {
		for r := rune(0x2500); r <= 0x257F; r++ {
			rectangles := glyphShapes(t, r, 10, 20, scale, 15, 1).Rectangles
			for _, rectangle := range rectangles {
				for _, edge := range []float32{rectangle.Bounds.X, rectangle.Bounds.Y, rectangle.Bounds.X + rectangle.Bounds.Width, rectangle.Bounds.Y + rectangle.Bounds.Height} {
					if _, fraction := math.Modf(float64(edge * scale)); math.Abs(fraction) > 1e-3 && math.Abs(fraction-1) > 1e-3 {
						t.Errorf("%U at scale %g draws an edge at %g, not a device pixel", r, scale, edge)
						break
					}
				}
			}
		}
	}
	for _, test := range []struct {
		size, scale float32
		want        float32
	}{{15, 1, 1}, {15, 2, 2}, {40, 1, 2}, {8, 1, 1}} {
		rectangles := glyphShapes(t, '─', 10, 20, test.scale, test.size, 1).Rectangles
		if len(rectangles) == 0 {
			t.Fatalf("size %g: a rule draws nothing", test.size)
		}
		for _, rectangle := range rectangles {
			if got := rectangle.Bounds.Height * test.scale; math.Abs(float64(got-test.want)) > 1e-4 {
				t.Errorf("size %g at scale %g: rule is %g thick, want %g", test.size, test.scale, got, test.want)
			}
		}
	}
}

// A dashed line keeps its gaps equal, and fills the cell: tiled cells read as one line.
func TestBoxDashKeepsEqualGaps(t *testing.T) {
	rectangles := glyphShapes(t, '┄', 20, 20, 1, 15, 1).Rectangles
	if len(rectangles) != 3 {
		t.Fatalf("┄ is %d dashes, want 3", len(rectangles))
	}
	gap := rectangles[1].Bounds.X - (rectangles[0].Bounds.X + rectangles[0].Bounds.Width)
	for index := 1; index < len(rectangles); index++ {
		if other := rectangles[index].Bounds.X - (rectangles[index-1].Bounds.X + rectangles[index-1].Bounds.Width); other != gap {
			t.Fatalf("gap %d is %g, gap 0 is %g", index, other, gap)
		}
	}
	if first := rectangles[0].Bounds.X; first != float32(int32(gap)/2) {
		t.Fatalf("the first dash starts at %g, want half a gap (%g device pixels) from the cell's left", first, gap)
	}
	for _, rectangle := range rectangles {
		if rectangle.Bounds.Y != (20-1)/2 {
			t.Fatalf("a dash sits at y=%g, want the middle of the cell", rectangle.Bounds.Y)
		}
	}
}

// trianglePieces is how many chains the triangles form when two that share a vertex are in the
// same chain: a stroke tessellates into one chain, and a gap reads as two.
func trianglePieces(triangles []Triangle) int {
	unseen := make([]bool, len(triangles))
	for index := range unseen {
		unseen[index] = true
	}
	pieces := 0
	for seed := range triangles {
		if !unseen[seed] {
			continue
		}
		pieces++
		unseen[seed] = false
		for queue := []int{seed}; len(queue) > 0; queue = queue[1:] {
			first := queue[0]
			for second := range triangles {
				if unseen[second] && trianglesShareVertex(triangles[first], triangles[second]) {
					unseen[second] = false
					queue = append(queue, second)
				}
			}
		}
	}
	return pieces
}

func trianglesShareVertex(first, second Triangle) bool {
	for _, one := range first.Points {
		for _, other := range second.Points {
			if math.Abs(float64(one.X-other.X)) < 1e-3 && math.Abs(float64(one.Y-other.Y)) < 1e-3 {
				return true
			}
		}
	}
	return false
}

// geometryBox is the rectangle that holds every shape of a cell glyph.
func geometryBox(rectangles []Rectangle, triangles []Triangle) Rect {
	var box Rect
	grown := false
	add := func(bounds Rect) {
		if !grown {
			box, grown = bounds, true
			return
		}
		right, bottom := max(box.Right(), bounds.Right()), max(box.Bottom(), bounds.Bottom())
		box.X, box.Y = min(box.X, bounds.X), min(box.Y, bounds.Y)
		box.Width, box.Height = right-box.X, bottom-box.Y
	}
	for _, rectangle := range rectangles {
		add(rectangle.Bounds)
	}
	for _, triangle := range triangles {
		for _, point := range triangle.Points {
			add(Rect{X: point.X, Y: point.Y})
		}
	}
	return box
}
