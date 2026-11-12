package gfx

import (
	"math"
	"testing"
)

func quadCorners() Quad {
	return Quad{
		Points: [4]Point{{X: 10, Y: 20}, {X: 110, Y: 26}, {X: 14, Y: 220}, {X: 118, Y: 210}},
		Tint: [4]Color{
			{Red: 1, Green: 1, Blue: 1, Alpha: 1},
			{Red: 1, Green: 1, Blue: 1, Alpha: .8},
			{Red: 0, Green: 0, Blue: 0, Alpha: .4},
			{Red: 0, Green: 0, Blue: 0, Alpha: .2},
		},
	}
}

func TestFillQuadKeepsCornerColorsClipAndOrder(t *testing.T) {
	var canvas Canvas
	canvas.Clip(Rect{Width: 200, Height: 300})
	canvas.PushLayer(2)
	canvas.Fill(Rect{Width: 1, Height: 1}, Color{Alpha: 1})
	quad := quadCorners()
	canvas.FillQuad(quad)
	triangles := canvas.Triangles()
	if len(triangles) != 2 {
		t.Fatalf("triangles = %d, want 2", len(triangles))
	}
	for index, triangle := range triangles {
		if !triangle.HasClip || triangle.Layer != 2 || triangle.Order <= 1 {
			t.Fatalf("triangle %d lost clip, layer or order: %+v", index, triangle)
		}
		if !triangle.Shaded {
			t.Fatalf("triangle %d is not shaded from its three corners", index)
		}
		// Each half names three of the quad's corners, in the order its colors
		// arrive so that the face is interpolated between them.
		for corner, point := range triangle.Points {
			if !quadHasPoint(quad, point) {
				t.Fatalf("triangle %d corner %d at %+v is not a corner of the quad", index, corner, point)
			}
		}
		// The diagonal the halves share is not feathered; the outline is.
		wantMask := uint8(1<<0 | 1<<2)
		if index == 1 {
			wantMask = 1<<0 | 1<<1
		}
		if triangle.EdgeMask != wantMask {
			t.Fatalf("triangle %d edge mask = %b, want %b", index, triangle.EdgeMask, wantMask)
		}
	}
	// The first half's colors are its corners', in the same order.
	first := triangles[0]
	colors := []Color{first.Color, first.Shade[0], first.Shade[1]}
	want := []Color{quad.Tint[0], quad.Tint[1], quad.Tint[2]}
	if colors[0] != want[0] || colors[1] != want[1] || colors[2] != want[2] {
		t.Fatalf("first half colors = %+v, want %+v", colors, want)
	}
}

func quadHasPoint(quad Quad, point Point) bool {
	for _, corner := range quad.Points {
		if corner == point {
			return true
		}
	}
	return false
}

func TestFillQuadPaintsNothingWhenEmptyOrClear(t *testing.T) {
	var canvas Canvas
	flat := quadCorners()
	flat.Points = [4]Point{{X: 5, Y: 5}, {X: 5, Y: 5}, {X: 5, Y: 5}, {X: 5, Y: 5}}
	canvas.FillQuad(flat)
	clear := quadCorners()
	clear.Tint = [4]Color{}
	canvas.FillQuad(clear)
	if len(canvas.Triangles()) != 0 {
		t.Fatalf("empty quads painted %d triangles", len(canvas.Triangles()))
	}
}

func TestLabelQuadKeepsItsBoxAndWarp(t *testing.T) {
	var canvas Canvas
	box := Rect{X: 20, Y: 100, Width: 300, Height: 24}
	quad := quadCorners()
	canvas.LabelQuad("In the beginning", box, quad, TextStyle{Size: 16})
	texts := canvas.Texts()
	if len(texts) != 1 {
		t.Fatalf("texts = %d, want 1", len(texts))
	}
	text := texts[0]
	if !text.HasQuad || text.Quad != quad.Points || text.Tint != quad.Tint {
		t.Fatalf("label lost its quad: %+v", text)
	}
	// The box stays the rasterisation size, so the picture the quad places is
	// the whole line.
	if text.Bounds != box {
		t.Fatalf("bounds = %+v, want the line box %+v", text.Bounds, box)
	}
	canvas.LabelQuad("clear", box, clearQuad(), TextStyle{Size: 16})
	if len(canvas.Texts()) != 1 {
		t.Fatal("a clear quad painted a label")
	}
}

func clearQuad() Quad {
	quad := quadCorners()
	quad.Tint = [4]Color{}
	return quad
}

func pointNear(a, b Point) bool {
	return math.Abs(float64(a.X-b.X)) < 1e-4 && math.Abs(float64(a.Y-b.Y)) < 1e-4
}

// TestLabelQuadAlignedMovesTheWholePicture pins that alignment moves the
// rasterisation box and the quad together. The box keeps its width, so both
// corners of each edge move by the same amount; moving only the left edge would
// squeeze the whole texture into a narrower quad.
func TestLabelQuadAlignedMovesTheWholePicture(t *testing.T) {
	const value = "In the beginning"
	style := TextStyle{Size: 16}
	var measure Canvas
	box := Rect{X: 20, Y: 100, Width: measure.TextWidthStyle(value, style) + 80, Height: 24}
	for _, test := range []struct {
		name      string
		alignment Alignment
	}{
		{"center", AlignCenter},
		{"right", AlignRight},
	} {
		t.Run(test.name, func(t *testing.T) {
			var canvas Canvas
			quad := quadCorners()
			canvas.LabelQuadAligned(value, box, quad, style, test.alignment)
			texts := canvas.Texts()
			if len(texts) != 1 {
				t.Fatalf("texts = %d, want 1", len(texts))
			}
			inset := box.Width - measure.TextWidthStyle(value, style)
			if test.alignment == AlignCenter {
				inset /= 2
			}
			if got := texts[0].Bounds.X; math.Abs(float64(got-(box.X+inset))) > 1e-4 {
				t.Fatalf("bounds x = %v, want %v", got, box.X+inset)
			}
			frac := inset / box.Width
			want := quad.Points
			top := Point{X: quad.Points[1].X - quad.Points[0].X, Y: quad.Points[1].Y - quad.Points[0].Y}
			bottom := Point{X: quad.Points[3].X - quad.Points[2].X, Y: quad.Points[3].Y - quad.Points[2].Y}
			for _, corner := range []int{0, 1} {
				want[corner].X += top.X * frac
				want[corner].Y += top.Y * frac
			}
			for _, corner := range []int{2, 3} {
				want[corner].X += bottom.X * frac
				want[corner].Y += bottom.Y * frac
			}
			for corner, point := range texts[0].Quad {
				if !pointNear(point, want[corner]) {
					t.Fatalf("corner %d = %+v, want %+v (the whole picture moves with the aligned line)", corner, point, want[corner])
				}
			}
		})
	}
}

// TestLabelQuadAlignedSetsTheMonoBaseline pins that a quad label on a fixed-pitch
// face stands on the same row as LabelAligned's, not on the generic baseline.
func TestLabelQuadAlignedSetsTheMonoBaseline(t *testing.T) {
	var canvas Canvas
	canvas.SetTextEngine(&monoCellEngine{MonoCellMetrics: MonoCellMetrics{Advance: 0.55, Ascent: 0.80, Descent: 0.25, LineGap: 0.08}})
	style := TextStyle{Size: 20, Mono: true}
	box := Rect{X: 10, Y: 20, Width: 160, Height: 23}
	canvas.LabelQuadAligned("abc", box, quadCorners(), style, AlignLeft)
	texts := canvas.Texts()
	if len(texts) != 1 {
		t.Fatalf("texts = %d, want 1", len(texts))
	}
	want := canvas.MonoBaseline(style.Size, style.MonoFace, box.Height)
	if want != 17 {
		t.Fatalf("measured baseline = %v, want the cell's row 17", want)
	}
	if got := texts[0].Baseline; got != want {
		t.Fatalf("baseline = %v, want %v", got, want)
	}
}
