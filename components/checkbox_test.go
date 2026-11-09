package components

import (
	"math"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestCheckboxEnclosingCardHasOneSemanticTarget(t *testing.T) {
	var canvas gfx.Canvas
	card := gfx.Rect{X: 10, Y: 20, Width: 300, Height: 64}
	BeginCheckbox(&canvas, gfx.DarkTheme(), gfx.Item(1), gfx.Rect{X: 270, Y: 40}, true).
		Label("Production server").HitBounds(card).Paint()
	if len(canvas.Hits()) != 1 {
		t.Fatal("card and square must be one keyboard stop")
	}
	hit := canvas.Hits()[0]
	if hit.Bounds != card || !hit.Checked || hit.Role != gfx.RoleCheckbox || hit.Label != "Production server" {
		t.Fatalf("selection card target: %+v", hit)
	}
	if !hit.Focusable || hit.SkipTab {
		t.Fatalf("enclosing target must keep the checkbox in the Tab order: %+v", hit)
	}
	if got := canvas.Rectangles()[0].Bounds; got.X != 270 || got.Y != 40 || got.Width != 16 {
		t.Fatalf("enclosing target changed the visual square: %+v", got)
	}
}

func TestCheckboxCheckIsCenteredAtDisplayScales(t *testing.T) {
	// Display presets must land within half a device pixel: at these scales the
	// box and the mark round to the same lattice. Custom fractional scales still
	// snap the box and the icon independently, so they may end one pixel off;
	// that is measured, not the icon adding an offset of its own.
	for _, test := range []struct {
		name      string
		scales    []float32
		tolerance float64
	}{
		{"presets", []float32{1, 1.25, 1.5, 2, 3}, 0.501},
		{"custom", []float32{1.6, 1.75}, 1.01},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, scale := range test.scales {
				for _, origin := range []gfx.Point{{X: 16, Y: 16}, {X: 458, Y: 602}, {X: 82.4, Y: 94.4}} {
					center, want := checkboxCheckCenter(scale, origin)
					if math.Abs(float64((center.X-want.X)*scale)) > test.tolerance || math.Abs(float64((center.Y-want.Y)*scale)) > test.tolerance {
						t.Errorf("scale %g at %v: visible mark centered at %v, checkbox center %v", scale, origin, center, want)
					}
				}
			}
		})
	}
}

func checkboxCheckCenter(scale float32, origin gfx.Point) (center, want gfx.Point) {
	var canvas gfx.Canvas
	canvas.SetPixelScale(scale)
	BeginCheckbox(&canvas, gfx.LightTheme(), gfx.Item(1), gfx.Rect{X: origin.X, Y: origin.Y}, true).Paint()
	box := canvas.Rectangles()[0].Bounds
	want = gfx.Point{X: box.X + box.Width/2, Y: box.Y + box.Height/2}
	least, greatest := gfx.Point{X: 1e9, Y: 1e9}, gfx.Point{X: -1e9, Y: -1e9}
	for _, triangle := range canvas.Triangles() {
		for _, point := range triangle.Points {
			least.X, least.Y = min(least.X, point.X), min(least.Y, point.Y)
			greatest.X, greatest.Y = max(greatest.X, point.X), max(greatest.Y, point.Y)
		}
	}
	return gfx.Point{X: (least.X + greatest.X) / 2, Y: (least.Y + greatest.Y) / 2}, want
}

func BenchmarkCheckboxPaint(b *testing.B) {
	var canvas gfx.Canvas
	canvas.SetPixelScale(1.25)
	theme := gfx.DarkTheme()
	BeginCheckbox(&canvas, theme, gfx.Item(1), gfx.Rect{X: 16, Y: 16}, true).Paint()
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		BeginCheckbox(&canvas, theme, gfx.Item(1), gfx.Rect{X: 16, Y: 16}, true).Paint()
	}
}

func TestCheckboxCheckedPaintAndTypedState(t *testing.T) {
	theme := gfx.DarkTheme()
	var canvas gfx.Canvas
	BeginCheckbox(&canvas, theme, gfx.Item(1), gfx.Rect{X: 20, Y: 20, Width: 16, Height: 16}, true).Label("Accept terms").Paint()
	rectangles := canvas.Rectangles()
	if len(rectangles) != 2 || rectangles[1].Color != theme.Primary {
		t.Fatalf("checked checkbox rectangles = %+v", rectangles)
	}
	if len(canvas.Texts()) != 0 || len(canvas.Triangles()) == 0 {
		t.Fatalf("the check mark is a path, not a character: %d texts, %d triangles", len(canvas.Texts()), len(canvas.Triangles()))
	}
	hit := canvas.Hits()[0]
	if hit.Role != gfx.RoleCheckbox || !hit.Checked || hit.Label != "Accept terms" || !hit.Focusable {
		t.Fatalf("typed checkbox hit = %+v", hit)
	}
	if hit.Bounds != (gfx.Rect{X: 8, Y: 12, Width: 40, Height: 32}) {
		t.Fatalf("checkbox hit bounds = %+v, want official expanded target", hit.Bounds)
	}
}

func TestCheckboxOfficialDefaultGeometryAndUncheckedSurfaces(t *testing.T) {
	for _, theme := range []gfx.Theme{gfx.LightTheme(), gfx.DarkTheme()} {
		var canvas gfx.Canvas
		BeginCheckbox(&canvas, theme, gfx.ID{}, gfx.Rect{X: 3, Y: 5, Width: 40, Height: 28}, false).Paint()
		box := canvas.Rectangles()[0]
		if box.Bounds != (gfx.Rect{X: 3, Y: 5, Width: 16, Height: 16}) || box.Radius != 4 {
			t.Fatalf("default checkbox geometry = %+v", box)
		}
		// An unchecked box is the page's color inside its border (or Input/30 over
		// it in a dark theme), never the border color showing through.
		want := theme.Background
		if theme.Background.Red < 0.5 {
			want = scaledAlpha(theme.Input, 0.30).Over(theme.Background)
		}
		if fill := canvas.Rectangles()[1].Color; fill != want {
			t.Fatalf("unchecked fill = %+v, want %+v", fill, want)
		}
		if len(canvas.Hits()) != 0 {
			t.Fatalf("preview checkbox declared %d hits", len(canvas.Hits()))
		}
	}
}

func TestCheckboxDisabledRetainsSemanticHitAtHalfOpacity(t *testing.T) {
	theme := gfx.DarkTheme()
	var canvas gfx.Canvas
	BeginCheckbox(&canvas, theme, gfx.Item(1), gfx.Rect{Width: 16, Height: 16}, true).Disabled().Paint()
	if len(canvas.Hits()) != 1 || !canvas.Hits()[0].Disabled || canvas.Hits()[0].Focusable || !canvas.Hits()[0].Checked {
		t.Fatalf("disabled checkbox hit = %+v", canvas.Hits())
	}
	want := withAlpha(theme.Primary, 0.50).Over(theme.Background)
	if fill := canvas.Rectangles()[1].Color; fill != want {
		t.Fatalf("disabled checked fill = %+v, want %+v", fill, want)
	}
}

func TestCheckboxFocusAndInvalidUseOfficialRings(t *testing.T) {
	theme := gfx.DarkTheme()
	var focused, invalid gfx.Canvas
	focused.SetInteraction(gfx.ID{}, gfx.ID{}, gfx.Item(1), true)
	BeginCheckbox(&focused, theme, gfx.Item(1), gfx.Rect{X: 4, Y: 4, Width: 16, Height: 16}, false).Paint()
	BeginCheckbox(&invalid, theme, gfx.Item(2), gfx.Rect{X: 4, Y: 4, Width: 16, Height: 16}, false).Invalid().Paint()

	wantFocus := theme.Ring
	wantFocus.Alpha *= 0.50
	if ring := focused.Rectangles()[0]; ring.Color != wantFocus || ring.Bounds != (gfx.Rect{X: 1, Y: 1, Width: 22, Height: 22}) {
		t.Fatalf("focus ring = %+v", ring)
	}
	wantInvalid := withAlpha(theme.Danger, 0.40)
	wantBorder := invalidBorder(&theme).Over(theme.Background)
	if ring := invalid.Rectangles()[0]; ring.Color != wantInvalid || invalid.Rectangles()[1].Color != wantBorder {
		t.Fatalf("invalid ring/border = %+v / %+v", ring, invalid.Rectangles()[1])
	}
}

func TestSeparatorExplicitOrientationAndThickness(t *testing.T) {
	theme := gfx.DarkTheme()
	var canvas gfx.Canvas
	PaintSeparator(&canvas, theme, gfx.Rect{Width: 120, Height: 9}, SeparatorHorizontal)
	PaintSeparator(&canvas, theme, gfx.Rect{X: 130, Width: 9, Height: 120}, SeparatorVertical)
	lines := canvas.Rectangles()
	if len(lines) != 2 || lines[0].Bounds.Height != 1 || lines[0].Bounds.Width != 120 {
		t.Fatalf("horizontal separator = %+v", lines)
	}
	if lines[1].Bounds.Width != 1 || lines[1].Bounds.Height != 120 {
		t.Fatalf("vertical separator = %+v", lines[1].Bounds)
	}
}

func TestCheckboxRepeatedPaintReusesCanvasStorage(t *testing.T) {
	theme := gfx.DarkTheme()
	var canvas gfx.Canvas
	BeginCheckbox(&canvas, theme, gfx.Item(1), gfx.Rect{Width: 16, Height: 16}, true).Paint()
	allocations := testing.AllocsPerRun(100, func() {
		canvas.Reset()
		BeginCheckbox(&canvas, theme, gfx.Item(1), gfx.Rect{Width: 16, Height: 16}, true).Paint()
	})
	if allocations != 0 {
		t.Fatalf("repeated checkbox paint allocated %v times", allocations)
	}
}

// The check and the cross are paths centered in the box they are given, so they
// look the same on every platform and sit where the box says; no character is
// involved.
func TestMarksAreVectorsCenteredInTheirBounds(t *testing.T) {
	bounds := gfx.Rect{X: 100, Y: 40, Width: 28, Height: 28}
	for name, paint := range map[string]func(*gfx.Canvas, gfx.Rect, float32, gfx.Color){"check": paintCheck, "cross": paintCross} {
		var canvas gfx.Canvas
		paint(&canvas, bounds, 16, gfx.Color{Red: 1, Alpha: 1})
		if len(canvas.Texts()) != 0 || len(canvas.Triangles()) == 0 {
			t.Fatalf("%s: %d texts, %d triangles", name, len(canvas.Texts()), len(canvas.Triangles()))
		}
		least, greatest := gfx.Point{X: 1e9, Y: 1e9}, gfx.Point{X: -1e9, Y: -1e9}
		for _, triangle := range canvas.Triangles() {
			for _, point := range triangle.Points {
				least.X, least.Y = min(least.X, point.X), min(least.Y, point.Y)
				greatest.X, greatest.Y = max(greatest.X, point.X), max(greatest.Y, point.Y)
			}
		}
		if centerX, centerY := (least.X+greatest.X)/2, (least.Y+greatest.Y)/2; centerX < 113 || centerX > 115 || centerY < 53 || centerY > 55 {
			t.Errorf("%s is centered on (%v, %v), not on the box's (114, 54)", name, centerX, centerY)
		}
	}
}

// The indeterminate state is a primary box with a dash and a mixed
// accessibility value, not a checked box with a tick.
func TestCheckboxIndeterminatePaintAndSemantics(t *testing.T) {
	theme := gfx.LightTheme()
	var mixed, checked gfx.Canvas
	mixedHit := BeginCheckbox(&mixed, theme, gfx.Item(9), gfx.Rect{Width: 16, Height: 16}, true).Indeterminate().Paint()
	BeginCheckbox(&checked, theme, gfx.Item(9), gfx.Rect{Width: 16, Height: 16}, true).Paint()
	if hit := mixed.Hits()[0]; !hit.Mixed || hit.Checked || hit.Role != gfx.RoleCheckbox {
		t.Fatalf("mixed hit = %+v", hit)
	}
	if hit := checked.Hits()[0]; hit.Mixed || !hit.Checked {
		t.Fatalf("checked hit = %+v", hit)
	}
	if len(mixed.Rectangles()) == len(checked.Rectangles()) {
		t.Fatal("the dash did not add a rectangle the tick does not have")
	}
	if mixedHit.Clicked() || !mixed.Hits()[0].Focusable {
		t.Fatal("mixed checkbox lost its hit")
	}
}
