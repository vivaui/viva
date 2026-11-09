package gfx

import "testing"

// BenchmarkDrawIcon measures one Lucide-shaped icon stroke per paint pass
// after warm-up. Steady state must be zero-allocation.
func BenchmarkDrawIcon(b *testing.B) {
	icon := Icon{
		ViewBox: 24,
		Stroke:  2,
		Cap:     CapRound,
		Join:    JoinRound,
		Commands: []PathCommand{
			MoveTo(21, 15), LineTo(21, 19),
			CubicTo(21, 20.1, 20.1, 21, 19, 21),
			LineTo(5, 21),
			CubicTo(3.9, 21, 3, 20.1, 3, 19),
			LineTo(3, 15),
			MoveTo(7, 10), LineTo(12, 15), LineTo(17, 10),
			MoveTo(12, 15), LineTo(12, 3),
		},
	}
	var canvas Canvas
	bounds := Rect{X: 100, Y: 100, Width: 16, Height: 16}
	canvas.DrawIcon(icon, bounds, Color{Alpha: 1})
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		canvas.Reset()
		canvas.DrawIcon(icon, bounds, Color{Alpha: 1})
	}
}

// BenchmarkDrawPathCurveFill stresses adaptive flattening plus ear clipping
// on a curved closed contour.
func BenchmarkDrawPathCurveFill(b *testing.B) {
	path := NewPath(24, 24,
		MoveTo(2, 12), CubicTo(8, 2, 16, 22, 22, 12),
		QuadTo(12, 20, 2, 18), ClosePath(),
	)
	var canvas Canvas
	bounds := Rect{X: 0, Y: 0, Width: 24, Height: 24}
	canvas.DrawPath(path, bounds, PathStyle{Fill: Color{Alpha: 1}})
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		canvas.Reset()
		canvas.DrawPath(path, bounds, PathStyle{Fill: Color{Alpha: 1}})
	}
}

func BenchmarkDrawColoredIcon(b *testing.B) {
	// The folder glyph of a file-type icon set: a body and a lighter tab.
	icon := Icon{ViewBox: 16, Layers: []IconLayer{
		{Fill: Color{Red: 0.55, Green: 0.43, Blue: 0.39, Alpha: 1}, Commands: []PathCommand{
			MoveTo(6.922, 3.768), LineTo(6.278, 3.232), CubicTo(6.1, 3.08, 5.87, 3, 5.638, 3), LineTo(2, 3),
			CubicTo(1.45, 3, 1, 3.45, 1, 4), LineTo(1, 12), CubicTo(1, 12.55, 1.45, 13, 2, 13), LineTo(14, 13),
			CubicTo(14.55, 13, 15, 12.55, 15, 12), LineTo(15, 5), CubicTo(15, 4.45, 14.55, 4, 14, 4), LineTo(7.562, 4),
			CubicTo(7.3, 4, 7.1, 3.9, 6.922, 3.768), ClosePath(),
		}},
		{Fill: Color{Red: 0.84, Green: 0.8, Blue: 0.78, Alpha: 1}, Commands: []PathCommand{
			MoveTo(7.5, 11), LineTo(7, 11.5), LineTo(7, 13.5), LineTo(7.5, 14), LineTo(15.5, 14), LineTo(16, 13.5),
			LineTo(16, 11.5), LineTo(15.5, 11), ClosePath(),
		}},
	}}
	var canvas Canvas
	bounds := Rect{X: 4, Y: 4, Width: 16, Height: 16}
	tint := Color{Alpha: 1}
	canvas.DrawIcon(icon, bounds, tint)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		canvas.Reset()
		canvas.DrawIcon(icon, bounds, tint)
	}
}
