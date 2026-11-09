package gfx

import "testing"

func TestPathBuilderMatchesNewPath(t *testing.T) {
	var builder PathBuilder
	builder.MoveTo(1, 1)
	builder.LineTo(9, 1)
	builder.CubicTo(9, 5, 5, 9, 1, 9)
	builder.Close()
	want := NewPath(10, 10, MoveTo(1, 1), LineTo(9, 1), CubicTo(9, 5, 5, 9, 1, 9), ClosePath())
	style := PathStyle{Fill: Color{Red: 1, Alpha: 1}, Stroke: Color{Blue: 1, Alpha: 1}, StrokeWidth: 1}
	bounds := Rect{Width: 40, Height: 40}
	var a, b Canvas
	a.DrawPath(builder.Path(10, 10), bounds, style)
	b.DrawPath(want, bounds, style)
	if len(a.Triangles()) == 0 || len(a.Triangles()) != len(b.Triangles()) {
		t.Fatalf("triangles = %d, want %d", len(a.Triangles()), len(b.Triangles()))
	}
	for index := range a.Triangles() {
		if a.Triangles()[index] != b.Triangles()[index] {
			t.Fatalf("triangle %d differs", index)
		}
	}
}

func TestPathBuilderEmptyAndReset(t *testing.T) {
	var builder PathBuilder
	if got := builder.Path(10, 10); got.Size() != (Size{}) {
		t.Fatalf("empty builder path = %+v", got.Size())
	}
	builder.MoveTo(0, 0)
	builder.LineTo(1, 1)
	if builder.Len() != 2 || builder.Path(0, 10).Size() != (Size{}) {
		t.Fatalf("len = %d", builder.Len())
	}
	builder.Reset()
	if builder.Len() != 0 {
		t.Fatalf("len after reset = %d", builder.Len())
	}
}

func TestPathFillGradient(t *testing.T) {
	gradient := LinearGradient{Start: Point{Y: 0}, End: Point{Y: 10}, From: Color{Red: 1, Alpha: 0.8}, To: Color{Red: 1, Alpha: 0.1}}
	var canvas Canvas
	canvas.DrawPath(NewPath(10, 10, MoveTo(0, 0), LineTo(10, 0), LineTo(10, 10), LineTo(0, 10), ClosePath()),
		Rect{Width: 100, Height: 100}, PathStyle{FillGradient: &gradient})
	triangles := canvas.Triangles()
	if len(triangles) == 0 {
		t.Fatal("gradient fill painted nothing")
	}
	var top, bottom float32 = 2, 2
	for _, triangle := range triangles {
		colors := [3]Color{triangle.Color, triangle.Color, triangle.Color}
		if triangle.Shaded {
			colors[1], colors[2] = triangle.Shade[0], triangle.Shade[1]
		}
		for index, point := range triangle.Points {
			switch point.Y {
			case 0:
				top = colors[index].Alpha
			case 100:
				bottom = colors[index].Alpha
			}
		}
	}
	if abs32(top-0.8) > 1e-4 || abs32(bottom-0.1) > 1e-4 {
		t.Fatalf("alpha top=%v bottom=%v, want 0.8 and 0.1", top, bottom)
	}
}

func TestPathFillGradientLeavesStrokeSolid(t *testing.T) {
	gradient := LinearGradient{End: Point{Y: 10}, From: Color{Alpha: 0.5}, To: Color{Alpha: 0.5}}
	stroke := Color{Green: 1, Alpha: 1}
	var canvas Canvas
	canvas.DrawPath(NewPath(10, 10, MoveTo(0, 5), LineTo(10, 5)), Rect{Width: 10, Height: 10},
		PathStyle{FillGradient: &gradient, Stroke: stroke, StrokeWidth: 2})
	for _, triangle := range canvas.Triangles() {
		if triangle.Shaded || triangle.Color != stroke {
			t.Fatalf("stroke triangle = %+v", triangle)
		}
	}
}

func BenchmarkPathBuilderPolyline(b *testing.B) {
	for _, points := range []int{1000, 10000} {
		b.Run(itoa(points), func(b *testing.B) {
			var canvas Canvas
			draw := func() {
				builder := canvas.PathBuilder()
				builder.MoveTo(0, 50)
				for index := 1; index < points; index++ {
					builder.LineTo(float32(index), 50+float32(index%7))
				}
				canvas.DrawPath(builder.Path(float32(points), 100), Rect{Width: 400, Height: 100},
					PathStyle{Stroke: Color{Blue: 1, Alpha: 1}, StrokeWidth: 2, Join: JoinRound})
			}
			draw()
			b.ReportAllocs()
			b.ResetTimer()
			for iteration := 0; iteration < b.N; iteration++ {
				canvas.Reset()
				draw()
			}
		})
	}
}

func BenchmarkPathFillGradient(b *testing.B) {
	gradient := LinearGradient{End: Point{Y: 100}, From: Color{Red: 1, Alpha: 0.8}, To: Color{Red: 1, Alpha: 0.1}}
	var canvas Canvas
	draw := func() {
		builder := canvas.PathBuilder()
		builder.MoveTo(0, 100)
		for index := 0; index <= 200; index++ {
			builder.LineTo(float32(index), 40+float32(index%13)*3)
		}
		builder.LineTo(200, 100)
		builder.Close()
		canvas.DrawPath(builder.Path(200, 100), Rect{Width: 400, Height: 200}, PathStyle{FillGradient: &gradient})
	}
	draw()
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		canvas.Reset()
		draw()
	}
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [20]byte
	index := len(digits)
	for value > 0 {
		index--
		digits[index] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[index:])
}
