package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestSliderPaintsClampedSnappedValueAndMapsCoordinates(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	slider := BeginSlider(&canvas, theme, gfx.Item(71), gfx.Rect{X: 10, Y: 20, Width: 210}, 46).Range(0, 100).Step(5).ValueText("45")
	geometry := slider.Paint("Volume")
	if geometry.Value != 45 {
		t.Fatalf("painted slider value = %v, want 45", geometry.Value)
	}
	if len(canvas.Rectangles()) != 4 || len(canvas.Hits()) != 1 {
		t.Fatalf("slider commands = %d rectangles, %d hits", len(canvas.Rectangles()), len(canvas.Hits()))
	}
	if center := geometry.Thumb.X + geometry.Thumb.Width/2; geometry.Active.X != geometry.Track.X || geometry.Active.X+geometry.Active.Width != center {
		t.Fatalf("active range %+v does not run from the groove's start to the thumb's center %v", geometry.Active, center)
	}
	if got := slider.ValueAt(geometry.Thumb.X + geometry.Thumb.Width/2); got != 45 {
		t.Fatalf("ValueAt(thumb center) = %v, want the painted value 45", got)
	}
	if got := slider.ValueAt(geometry.Thumb.X + geometry.Thumb.Width/2 + (geometry.Track.Width-16)*0.17); got != 60 {
		t.Fatalf("ValueAt(62%%) = %v, want 60", got)
	}
	if canvas.Hits()[0].Role != gfx.RoleSlider || canvas.Hits()[0].Value != "45" {
		t.Fatalf("slider semantics = %+v", canvas.Hits()[0])
	}
}

func TestSliderDisabledStableAndAllocationFree(t *testing.T) {
	theme := gfx.DarkTheme()
	var canvas gfx.Canvas
	paint := func() {
		canvas.Reset()
		BeginSlider(&canvas, theme, gfx.Item(72), gfx.Rect{Width: 240}, 0.35).ValueText("35%").Disabled().Paint("Opacity")
	}
	paint()
	rectangles := slices.Clone(canvas.Rectangles())
	paint()
	if len(canvas.Hits()) != 1 || !canvas.Hits()[0].Disabled || canvas.Hits()[0].Focusable {
		t.Fatalf("disabled slider semantics = %+v", canvas.Hits())
	}
	if !slices.Equal(rectangles, canvas.Rectangles()) {
		t.Fatal("identical sliders produced different command streams")
	}
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("slider paint allocations = %v", allocations)
	}
}

func TestRangeSliderPaintsMultipleStableThumbs(t *testing.T) {
	thumbs := [...]SliderThumbValue{
		{ID: gfx.Item(73), Label: "Minimum", ValueText: "20", Value: 20},
		{ID: gfx.Item(74), Label: "Maximum", ValueText: "80", Value: 80},
	}
	var canvas gfx.Canvas
	slider := BeginRangeSlider(&canvas, gfx.LightTheme(), gfx.Rect{Width: 216}, thumbs[:])
	geometry := slider.Paint("Range")
	if geometry.ThumbCount != 2 || len(canvas.Rectangles()) != 6 || len(canvas.Hits()) != 2 {
		t.Fatalf("range slider = %+v, %d rectangles, %d hits", geometry, len(canvas.Rectangles()), len(canvas.Hits()))
	}
	low, high := slider.ThumbBounds(0), slider.ThumbBounds(1)
	if geometry.Active.X != low.X+low.Width/2 || geometry.Active.X+geometry.Active.Width != high.X+high.Width/2 {
		t.Fatalf("range active geometry = %+v between thumbs %+v and %+v", geometry.Active, low, high)
	}
	if canvas.Hits()[0].Value != "20" || canvas.Hits()[1].Value != "80" {
		t.Fatalf("range slider values = %+v", canvas.Hits())
	}
	paint := func() {
		canvas.Reset()
		BeginRangeSlider(&canvas, gfx.LightTheme(), gfx.Rect{Width: 216}, thumbs[:]).Paint("Range")
	}
	paint()
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("range slider paint allocations = %v", allocations)
	}
}

func TestSliderVerticalMapsMaximumAtTop(t *testing.T) {
	var canvas gfx.Canvas
	slider := BeginSlider(&canvas, gfx.LightTheme(), gfx.Item(75), gfx.Rect{X: 10, Y: 20, Width: 32, Height: 216}, 25).Vertical()
	geometry := slider.Paint("Brightness")
	if geometry.Track.Width != 6 || geometry.Active.Width != 6 {
		t.Fatalf("vertical geometry = %+v", geometry)
	}
	if got := slider.ValueAt(geometry.Track.Y); got != 100 {
		t.Fatalf("vertical top value = %v", got)
	}
}

// The groove is the slider's whole length and the thumb never leaves it: at the
// minimum and the maximum it sits flush with the groove's ends instead of hanging
// out past them.
func TestSliderThumbStaysInsideTheGroove(t *testing.T) {
	for _, vertical := range []bool{false, true} {
		for _, value := range []float32{0, 100} {
			slider := BeginSlider(nil, gfx.LightTheme(), gfx.Item(76), gfx.Rect{X: 10, Y: 20, Width: 200, Height: 200}, value)
			if vertical {
				slider = slider.Vertical()
			}
			track, thumb := slider.trackBounds(), slider.ThumbBounds(0)
			if vertical && (thumb.Y < track.Y || thumb.Y+thumb.Height > track.Y+track.Height) ||
				!vertical && (thumb.X < track.X || thumb.X+thumb.Width > track.X+track.Width) {
				t.Fatalf("vertical=%v value=%v: thumb %+v outside groove %+v", vertical, value, thumb, track)
			}
		}
	}
}

func TestSliderKeyDown(t *testing.T) {
	tests := []struct {
		name      string
		value     float32
		step      float32
		direction Direction
		key       gfx.Key
		modifiers gfx.Modifiers
		want      float32
		handled   bool
	}{
		{name: "right adds a step", value: 40, step: 5, key: gfx.KeyRight, want: 45, handled: true},
		{name: "up adds a step", value: 40, step: 5, key: gfx.KeyUp, want: 45, handled: true},
		{name: "left subtracts a step", value: 40, step: 5, key: gfx.KeyLeft, want: 35, handled: true},
		{name: "down subtracts a step", value: 40, step: 5, key: gfx.KeyDown, want: 35, handled: true},
		{name: "rtl swaps left and right", value: 40, step: 5, direction: RTL, key: gfx.KeyLeft, want: 45, handled: true},
		{name: "rtl leaves up alone", value: 40, step: 5, direction: RTL, key: gfx.KeyUp, want: 45, handled: true},
		{name: "shift jumps ten units", value: 20, step: 1, key: gfx.KeyRight, modifiers: gfx.ModifierShift, want: 30, handled: true},
		{name: "page up jumps ten units", value: 20, step: 1, key: gfx.KeyPageUp, want: 30, handled: true},
		{name: "page down jumps ten units", value: 20, step: 1, key: gfx.KeyPageDown, want: 10, handled: true},
		// Base UI's large step is ten value units, not ten steps: a slider that
		// moves by five still jumps two steps, not fifty.
		{name: "shift jumps ten units at step five", value: 40, step: 5, key: gfx.KeyRight, modifiers: gfx.ModifierShift, want: 50, handled: true},
		{name: "page up jumps ten units at step five", value: 40, step: 5, key: gfx.KeyPageUp, want: 50, handled: true},
		{name: "page down jumps ten units at step five", value: 40, step: 5, key: gfx.KeyPageDown, want: 30, handled: true},
		{name: "shift at step seven snaps to the grid", value: 40, step: 7, key: gfx.KeyRight, modifiers: gfx.ModifierShift, want: 49, handled: true},
		{name: "clamps at the maximum", value: 98, step: 5, key: gfx.KeyRight, want: 100, handled: true},
		{name: "clamps at the minimum", value: 2, step: 5, key: gfx.KeyLeft, want: 0, handled: true},
		{name: "home", value: 40, step: 5, key: gfx.KeyHome, want: 0, handled: true},
		{name: "end", value: 40, step: 5, key: gfx.KeyEnd, want: 100, handled: true},
		{name: "no step moves by one", value: 40, key: gfx.KeyRight, want: 41, handled: true},
		{name: "other keys are not the slider's", value: 40, step: 5, key: gfx.KeyEnter, want: 40},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, handled := SliderKeyDown(test.value, 0, 100, test.step, test.direction, test.key, test.modifiers)
			if got != test.want || handled != test.handled {
				t.Fatalf("SliderKeyDown() = (%v, %v), want (%v, %v)", got, handled, test.want, test.handled)
			}
		})
	}
}

// A caller that sizes its own coarse movement passes it as value units, as
// Base UI's largeStep does, and a non-positive one falls back to ten.
func TestSliderKeyDownLargeUsesTheCallersCoarseStep(t *testing.T) {
	for _, test := range []struct {
		name      string
		largeStep float32
		want      float32
	}{
		{"twenty-five units", 25, 65},
		{"zero falls back to ten", 0, 50},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, handled := SliderKeyDownLarge(40, 0, 100, 5, test.largeStep, LTR, gfx.KeyUp, gfx.ModifierShift)
			if !handled || got != test.want {
				t.Fatalf("SliderKeyDownLarge() = (%v, %v), want (%v, true)", got, handled, test.want)
			}
		})
	}
}

// A range thumb keeps minSteps*step from its neighbours: Home/End stop at the
// neighbour, while arrows push it away.
func TestRangeSliderKeyDownKeepsAndPushesNeighbours(t *testing.T) {
	values := []float32{20, 50, 80}
	if !RangeSliderKeyDown(values, 1, 0, 100, 5, 10, 1, LTR, gfx.KeyEnd, 0) || values[1] != 75 {
		t.Fatalf("End = %v, want 75 (next 80 minus one gap)", values)
	}
	values = []float32{20, 50, 80}
	if !RangeSliderKeyDown(values, 1, 0, 100, 5, 10, 1, LTR, gfx.KeyRight, gfx.ModifierShift) {
		t.Fatal("Shift+Right was not handled")
	}
	if values[1] != 60 {
		t.Fatalf("Shift+Right = %v, want 60", values)
	}
	values = []float32{20, 50, 80}
	for range 10 {
		RangeSliderKeyDown(values, 1, 0, 100, 5, 10, 1, LTR, gfx.KeyRight, 0)
	}
	if values[1] != 95 || values[2] != 100 {
		t.Fatalf("pushed up = %v, want [20 95 100]", values)
	}
	values = []float32{20, 48, 50}
	if !RangeSliderKeyDown(values, 2, 0, 100, 5, 10, 1, LTR, gfx.KeyLeft, 0) {
		t.Fatal("Left was not handled")
	}
	if values[0] != 20 || values[1] != 40 || values[2] != 45 {
		t.Fatalf("pushed down = %v, want [20 40 45]", values)
	}
}
