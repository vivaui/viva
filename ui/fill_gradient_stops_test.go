package ui

import (
	"math"
	"reflect"
	"testing"
)

// meshVertexColors lists every (point, color) pair the canvas painted triangles with.
func meshVertexColors(canvas *Canvas) (points []Point, colors []Color) {
	for _, triangle := range canvas.Triangles() {
		for index, point := range triangle.Points {
			color := triangle.Color
			if triangle.Shaded && index > 0 {
				color = triangle.Shade[index-1]
			}
			points, colors = append(points, point), append(colors, color)
		}
	}
	return points, colors
}

func near32(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-4 }

func nearColor(a, b Color) bool {
	return near32(a.Red, b.Red) && near32(a.Green, b.Green) && near32(a.Blue, b.Blue) && near32(a.Alpha, b.Alpha)
}

var (
	gradientRed   = Color{Red: 1, Alpha: 1}
	gradientGreen = Color{Green: 1, Alpha: 1}
	gradientBlue  = Color{Blue: 1, Alpha: 1}
)

// A gradient at 135 degrees runs from one corner to the opposite one, whatever
// the proportions of the rectangle, and every vertex has the color the line gives
// its position, three stops included.
func TestFillGradientColorsFollowTheLineThroughTheCorners(t *testing.T) {
	for _, radius := range []float32{0, 10} {
		var canvas Canvas
		bounds := Rect{X: 5, Y: 7, Width: 200, Height: 60}
		stops := []GradientStop{Stop(0, gradientRed), Stop(0.5, gradientGreen), Stop(1, gradientBlue)}
		canvas.FillGradient(bounds, radius, 135, stops)
		points, colors := meshVertexColors(&canvas)
		if len(points) == 0 {
			t.Fatal("nothing painted")
		}
		// 135 degrees points down and to the right: t is the projection of the
		// point onto that direction, 0 at the top left corner and 1 at the bottom right.
		dx, dy := float32(math.Sqrt2/2), float32(math.Sqrt2/2)
		half := (bounds.Width*dx + bounds.Height*dy) / 2
		centerX, centerY := bounds.X+bounds.Width/2, bounds.Y+bounds.Height/2
		for index, point := range points {
			t01 := ((point.X-centerX)*dx+(point.Y-centerY)*dy)/(2*half) + 0.5
			var want Color
			switch {
			case t01 <= 0:
				want = gradientRed
			case t01 >= 1:
				want = gradientBlue
			case t01 <= 0.5:
				want = gradientRed.Mix(gradientGreen, t01*2)
			default:
				want = gradientGreen.Mix(gradientBlue, (t01-0.5)*2)
			}
			if !nearColor(colors[index], want) {
				t.Fatalf("radius %v: color at %+v is %+v, want %+v", radius, point, colors[index], want)
			}
		}
	}
}

// With a radius the corners are cut away: no triangle reaches the square corner.
func TestFillGradientRoundsItsCorners(t *testing.T) {
	var canvas Canvas
	canvas.FillGradient(Rect{Width: 100, Height: 100}, 20, 90, []GradientStop{Stop(0, gradientRed), Stop(1, gradientBlue)})
	points, _ := meshVertexColors(&canvas)
	for _, point := range points {
		dx, dy := min(point.X, 100-point.X), min(point.Y, 100-point.Y)
		if dx < 20 && dy < 20 && math.Hypot(float64(20-dx), float64(20-dy)) > 20.01 {
			t.Fatalf("%+v lies outside the rounded corner", point)
		}
	}
}

func TestFillGradientHardEdgeAndHeldEnds(t *testing.T) {
	var canvas Canvas
	canvas.FillGradient(Rect{Width: 100, Height: 20}, 0, 90, []GradientStop{Stop(0.25, gradientRed), Stop(0.5, gradientRed), Stop(0.5, gradientBlue), Stop(0.75, gradientBlue)})
	points, colors := meshVertexColors(&canvas)
	for index, point := range points {
		switch {
		case point.X <= 50-1e-3 && !nearColor(colors[index], gradientRed):
			t.Fatalf("left of the edge, %+v is %+v, want red", point, colors[index])
		case point.X >= 50+1e-3 && !nearColor(colors[index], gradientBlue):
			t.Fatalf("right of the edge, %+v is %+v, want blue", point, colors[index])
		}
	}
}

// A fade to transparent changes opacity alone: the color never passes through
// the black of Color{}.
func TestFillGradientFadingToNothingKeepsTheColor(t *testing.T) {
	var canvas Canvas
	canvas.FillGradient(Rect{Width: 100, Height: 20}, 0, 90, []GradientStop{Stop(0, gradientRed), Stop(1, Color{})})
	_, colors := meshVertexColors(&canvas)
	for _, color := range colors {
		if color.Alpha > 0.001 && (!near32(color.Red, 1) || color.Green != 0 || color.Blue != 0) {
			t.Fatalf("color %+v is not the red it fades from", color)
		}
	}
}

func TestFillGradientOneStopIsSolidAndNoneIsNothing(t *testing.T) {
	var canvas Canvas
	canvas.FillGradient(Rect{Width: 10, Height: 10}, 3, 0, nil)
	canvas.FillGradient(Rect{Width: 0, Height: 10}, 3, 0, []GradientStop{Stop(0, gradientRed), Stop(1, gradientBlue)})
	if len(canvas.Rectangles())+len(canvas.Triangles()) != 0 {
		t.Fatal("painted without stops or without room")
	}
	canvas.FillGradient(Rect{Width: 10, Height: 10}, 3, 0, []GradientStop{Stop(0, gradientRed)})
	if len(canvas.Rectangles()) != 1 || canvas.Rectangles()[0].Color != gradientRed {
		t.Fatalf("one stop painted %+v", canvas.Rectangles())
	}
}

func TestFillGradientKeepsLayerClipAndOrder(t *testing.T) {
	var canvas Canvas
	clip := Rect{X: 1, Y: 1, Width: 50, Height: 50}
	canvas.Clip(clip)
	canvas.PushLayer(2)
	canvas.Fill(Rect{Width: 1, Height: 1}, gradientRed)
	canvas.FillGradient(Rect{Width: 40, Height: 40}, 8, 45, []GradientStop{Stop(0, gradientRed), Stop(1, gradientBlue)})
	if len(canvas.Triangles()) == 0 {
		t.Fatal("nothing painted")
	}
	boundary := false
	for _, triangle := range canvas.Triangles() {
		if !triangle.HasClip || triangle.ClipOf != clip || triangle.Layer != 2 || triangle.RectsBefore != 1 {
			t.Fatalf("lost its clip, layer or place in the order: %+v", triangle)
		}
		boundary = boundary || triangle.EdgeMask != 0
	}
	if !boundary {
		t.Fatal("no triangle has an outer edge to feather")
	}
}

func TestFillGradientAllocatesNothingOnceWarm(t *testing.T) {
	var canvas Canvas
	stops := []GradientStop{Stop(0, gradientRed), Stop(0.5, gradientGreen), Stop(1, gradientBlue)}
	paint := func() { canvas.Reset(); canvas.FillGradient(Rect{Width: 300, Height: 80}, 12, 120, stops) }
	paint()
	if allocs := testing.AllocsPerRun(100, paint); allocs != 0 {
		t.Fatalf("%v allocations a paint", allocs)
	}
}

func BenchmarkFillGradient(b *testing.B) {
	var canvas Canvas
	stops := []GradientStop{Stop(0, gradientRed), Stop(1, gradientBlue)}
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		canvas.FillGradient(Rect{Width: 300, Height: 80}, 12, 135, stops)
	}
}

func TestElevateFadedScalesEveryLayer(t *testing.T) {
	var full, half Canvas
	full.Elevate(Rect{Width: 20, Height: 20}, 4, ShadowMD)
	half.ElevateFaded(Rect{Width: 20, Height: 20}, 4, ShadowMD, 0.5)
	if len(full.Shadows()) != 2 || len(half.Shadows()) != 2 {
		t.Fatalf("%d and %d layers, want 2 each", len(full.Shadows()), len(half.Shadows()))
	}
	for index := range full.Shadows() {
		if !near32(half.Shadows()[index].Color.Alpha, full.Shadows()[index].Color.Alpha/2) {
			t.Fatalf("layer %d has alpha %v, want half of %v", index, half.Shadows()[index].Color.Alpha, full.Shadows()[index].Color.Alpha)
		}
	}
	var none Canvas
	none.ElevateFaded(Rect{Width: 20, Height: 20}, 4, ShadowMD, 0)
	if len(none.Shadows()) != 0 {
		t.Fatal("a shadow of no opacity was painted")
	}
}

// Every color of a theme is faded, so a control painted with the copy cannot
// show one at full strength. The walk finds colors added to Theme later.
func TestThemeWithOpacityFadesEveryColor(t *testing.T) {
	theme := LightTheme().WithOpacity(0.5)
	original := LightTheme()
	colorType := reflect.TypeOf(Color{})
	faded, got, want := 0, reflect.ValueOf(theme), reflect.ValueOf(original)
	for index := 0; index < got.NumField(); index++ {
		switch field := got.Field(index); {
		case field.Type() == colorType:
			a, b := field.Interface().(Color), want.Field(index).Interface().(Color)
			if !near32(a.Alpha, b.Alpha/2) {
				t.Errorf("%s has alpha %v, want %v", got.Type().Field(index).Name, a.Alpha, b.Alpha/2)
			}
			faded++
		case field.Kind() == reflect.Array && field.Type().Elem() == colorType:
			for item := 0; item < field.Len(); item++ {
				a, b := field.Index(item).Interface().(Color), want.Field(index).Index(item).Interface().(Color)
				if !near32(a.Alpha, b.Alpha/2) {
					t.Errorf("%s[%d] has alpha %v, want %v", got.Type().Field(index).Name, item, a.Alpha, b.Alpha/2)
				}
			}
		}
	}
	if faded == 0 {
		t.Fatal("found no color fields")
	}
	if LightTheme().WithOpacity(1) != original {
		t.Error("full opacity changed the theme")
	}
}
