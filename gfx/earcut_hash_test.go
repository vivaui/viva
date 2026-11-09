package gfx

import (
	"math"
	"testing"
)

func trianglesArea(triangles []Triangle) float64 {
	total := 0.0
	for _, t := range triangles {
		total += math.Abs(polygonArea(t.Points[:]))
	}
	return total
}

// A ring above the hash threshold must cover exactly the area it encloses,
// whether it is a thin curve-bounded band, a star or a ring with a hole.
func TestEarcutHashedRingsCoverTheirArea(t *testing.T) {
	band := func(n int) []PathCommand {
		commands := []PathCommand{MoveTo(0, 100)}
		for i := 0; i <= n; i++ {
			x := float32(i) * 400 / float32(n)
			commands = append(commands, LineTo(x, 50+30*float32(math.Sin(float64(i)/7))))
		}
		return append(commands, LineTo(400, 100), ClosePath())
	}
	star := func(n int) []PathCommand {
		commands := []PathCommand{}
		for i := 0; i < n; i++ {
			radius := 50.0
			if i%2 == 1 {
				radius = 20
			}
			angle := float64(i) * 2 * math.Pi / float64(n)
			x, y := float32(60+radius*math.Cos(angle)), float32(60+radius*math.Sin(angle))
			if i == 0 {
				commands = append(commands, MoveTo(x, y))
			} else {
				commands = append(commands, LineTo(x, y))
			}
		}
		return append(commands, ClosePath())
	}
	ringWithHole := func(n int) []PathCommand {
		commands := star(n)
		for i := 0; i < 40; i++ {
			angle := float64(i) * 2 * math.Pi / 40
			x, y := float32(60+10*math.Cos(-angle)), float32(60+10*math.Sin(-angle))
			if i == 0 {
				commands = append(commands, MoveTo(x, y))
			} else {
				commands = append(commands, LineTo(x, y))
			}
		}
		return append(commands, ClosePath())
	}
	cases := map[string][]PathCommand{"band": band(2000), "star": star(400), "hole": ringWithHole(400)}
	for name, commands := range cases {
		var points []Point
		for _, c := range commands {
			if c.Kind == PathCommandMove || c.Kind == PathCommandLine {
				points = append(points, c.Point)
			}
		}
		var want float64
		if name == "hole" {
			want = math.Abs(polygonArea(points[:400])) - math.Abs(polygonArea(points[400:]))
		} else {
			want = math.Abs(polygonArea(points))
		}
		var canvas Canvas
		canvas.DrawPath(NewPath(400, 100, commands...), Rect{Width: 400, Height: 100}, PathStyle{Fill: Color{Alpha: 1}})
		got := trianglesArea(canvas.Triangles())
		if math.Abs(got-want) > want*1e-3 {
			t.Fatalf("%s: triangles cover %v, polygon encloses %v", name, got, want)
		}
	}
}

func BenchmarkEarcutThinBand(b *testing.B) {
	for _, n := range []int{1000, 10000} {
		b.Run(itoa(n), func(b *testing.B) {
			var builder PathBuilder
			builder.MoveTo(0, 100)
			for i := 0; i <= n; i++ {
				builder.LineTo(float32(i)*400/float32(n), 50+30*float32(math.Sin(float64(i)/7)))
			}
			builder.LineTo(400, 100)
			builder.Close()
			path := builder.Path(400, 100)
			var canvas Canvas
			canvas.DrawPath(path, Rect{Width: 400, Height: 100}, PathStyle{Fill: Color{Alpha: 1}})
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				canvas.Reset()
				canvas.DrawPath(path, Rect{Width: 400, Height: 100}, PathStyle{Fill: Color{Alpha: 1}})
			}
		})
	}
}
