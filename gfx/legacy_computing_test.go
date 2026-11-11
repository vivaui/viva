package gfx

import (
	"math"
	"strings"
	"testing"
)

// Every family drawn for the Symbols for Legacy Computing and the Powerline block draws
// something, and everything it draws is inside the quarter cell a sprite may use.
func TestCellGlyphFamiliesDrawInsideTheirCell(t *testing.T) {
	families := []rune{
		0x1FB00, 0x1FB3C, 0x1FB68, 0x1FB70, 0x1FB76, 0x1FB7C, 0x1FB87,
		0x1FB90, 0x1FB95, 0x1FB98, 0x1FB9A, 0x1FBA0, 0x1FBAF, 0x1FBCE,
		0x1FBD0, 0x1FBE0, 0x1FBE8, 0x1CC1B, 0x1CC21, 0x1CC30, 0x1CD00,
		0x1CE00, 0x1CE0B, 0x1CE16, 0x1CE51, 0x1CE90, 0x1CEA0, 0x1CEAF,
		0xE0B0, 0xE0B1, 0xE0B4, 0xE0B5, 0xE0D2,
	}
	const width, height = 10, 20
	padX, padY := float32(width/4+1), float32(height/4+1)
	for _, r := range families {
		shapes := glyphShapes(t, r, width, height, 1, 15, 1)
		if len(shapes.Rectangles) == 0 && len(shapes.Triangles) == 0 {
			t.Fatalf("%U draws nothing", r)
		}
		box := geometryBox(shapes.Rectangles, shapes.Triangles)
		if box.X < -padX || box.Y < -padY || box.X+box.Width > width+padX || box.Y+box.Height > height+padY {
			t.Fatalf("%U draws beyond the quarter cell a sprite may use: %+v", r, box)
		}
	}
}

// The eighth blocks continue each other: the next block starts where the previous one ends or on
// the pixel they share, from the second eighth to the seventh.
func TestLegacyComputingEighthBlocksContinue(t *testing.T) {
	covered := float32(1)
	for r := rune(0x1FB70); r <= 0x1FB75; r++ {
		rectangles := glyphCell(t, r, 8, 16, 1, 15, 1)
		if len(rectangles) != 1 {
			t.Fatalf("%U draws %d rectangles", r, len(rectangles))
		}
		bounds := rectangles[0].Bounds
		if bounds.X != covered || bounds.Y != 0 || bounds.Height != 16 {
			t.Fatalf("%U at %+v leaves a gap after %g of its cell", r, bounds, covered)
		}
		covered = bounds.X + bounds.Width
	}
	if covered != 7 {
		t.Fatalf("the sixth vertical eighth ends at %g of 8", covered)
	}
	covered = 2
	for r := rune(0x1FB76); r <= 0x1FB7B; r++ {
		rectangles := glyphCell(t, r, 8, 16, 1, 15, 1)
		if len(rectangles) != 1 {
			t.Fatalf("%U draws %d rectangles", r, len(rectangles))
		}
		bounds := rectangles[0].Bounds
		if bounds.Y != covered || bounds.X != 0 || bounds.Width != 8 {
			t.Fatalf("%U at %+v leaves a gap after %g of its cell", r, bounds, covered)
		}
		covered = bounds.Y + bounds.Height
	}
	if covered != 14 {
		t.Fatalf("the sixth horizontal eighth ends at %g of 16", covered)
	}
}

// The sixteen sixteenth blocks partition the cell's top three quarters exactly.
func TestLegacyComputingSixteenthsPartitionTheirCell(t *testing.T) {
	area := 0.0
	for r := rune(0x1CE90); r <= 0x1CE9F; r++ {
		for _, rectangle := range glyphCell(t, r, 8, 16, 1, 15, 1) {
			area += float64(rectangle.Bounds.Width * rectangle.Bounds.Height)
		}
	}
	if area != 8*16 {
		t.Fatalf("the sixteenths cover %g of a 128 pixel cell", area)
	}
}

// The Powerline separators are the cell's own triangles, and the rounded bar and its mirror are
// the same geometry turned around the middle of the cell.
func TestPowerlineDrawsItsSeparators(t *testing.T) {
	width, height := float32(10), float32(20)
	shapes := glyphShapes(t, 0xE0B0, width, height, 1, 15, 1)
	if len(shapes.Triangles) != 1 {
		t.Fatalf("the separator draws %d triangles", len(shapes.Triangles))
	}
	points := shapes.Triangles[0].Points
	want := [3]Point{{X: 0, Y: 0}, {X: width, Y: height / 2}, {X: 0, Y: height}}
	for index, point := range points {
		if point != want[index] {
			t.Fatalf("point %d is %+v, want %+v", index, point, want[index])
		}
	}
	box := func(r rune) Rect {
		shapes := glyphShapes(t, r, width, height, 1, 15, 1)
		return geometryBox(shapes.Rectangles, shapes.Triangles)
	}
	bar, mirror := box(0xE0B4), box(0xE0B6)
	if math.Abs(float64(mirror.X+mirror.Width-(width-bar.X))) > 1e-4 ||
		mirror.Width != bar.Width || mirror.Height != bar.Height {
		t.Fatalf("the bar is %+v but its mirror is %+v", bar, mirror)
	}
}

// A filled shape covers the share of its cell the character names: the mosaic's own squares, an
// edge triangle, its negative (the rest of the cell), and the two edge triangles that halve it.
func TestLegacyFilledShapesCoverTheirShare(t *testing.T) {
	for _, test := range []struct {
		r             rune
		width, height float32
		want          float64
	}{
		{0x1FB3C, 12, 24, 24},
		{0x1FB6C, 10, 20, 50},
		{0x1FB68, 10, 20, 150},
		{0x1FB9A, 10, 20, 100},
	} {
		shapes := glyphShapes(t, test.r, test.width, test.height, 1, 15, 1)
		total := 0.0
		for _, triangle := range shapes.Triangles {
			total += filledArea(triangle)
		}
		if math.Abs(total-test.want) > 1e-3 {
			t.Fatalf("%U covers %g of its cell, want %g", test.r, total, test.want)
		}
	}
}

// The tables are the specification: every smooth mosaic is four rows of three cells, and the
// octant spec has one entry for each octant character.
func TestLegacyTablesAreWhole(t *testing.T) {
	for index, pattern := range legacyMosaics {
		if len(pattern) != 12 {
			t.Fatalf("mosaic U+%04X is %q, want four rows of three", 0x1FB3C+index, pattern)
		}
	}
	if entries := len(strings.Fields(legacyOctantSpec)); entries != len(legacyOctants) {
		t.Fatalf("the octant spec has %d entries, want %d", entries, len(legacyOctants))
	}
}
