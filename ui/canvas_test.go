package ui

import (
	"math"
	"testing"
)

func TestCanvasSeparatesFocusFromFocusRing(t *testing.T) {
	var canvas Canvas
	canvas.SetInteraction(Item(7), Item(7), Item(7), false)
	if canvas.Focused() != Item(7) || !canvas.FocusVisible().IsZero() {
		t.Fatal("pointer focus was hidden from editing or exposed a keyboard ring")
	}
	canvas.SetInteraction(ID{}, ID{}, Item(8), true)
	if canvas.Focused() != Item(8) || canvas.FocusVisible() != Item(8) {
		t.Fatal("keyboard focus did not expose both focus and its ring")
	}
	canvas.SetInteraction(ID{}, ID{}, ID{}, true)
	if !canvas.Focused().IsZero() || !canvas.FocusVisible().IsZero() {
		t.Fatal("blur retained focus")
	}
}

func TestCanvasPreservesPaintOrder(t *testing.T) {
	var canvas Canvas
	canvas.Fill(Rect{Width: 10, Height: 10}, Color{Red: 1, Alpha: 1})
	canvas.Border(Rect{X: 2, Y: 2, Width: 8, Height: 8}, 1, Color{Blue: 1}, Color{Green: 1})
	canvas.Label("Viva", Rect{X: 4, Y: 4, Width: 20, Height: 10}, TextStyle{Size: 12})

	if len(canvas.Rectangles()) != 3 || canvas.Rectangles()[2].Color.Green != 1 {
		t.Fatalf("rectangles = %+v", canvas.Rectangles())
	}
	if len(canvas.Texts()) != 1 || canvas.Texts()[0].Value != "Viva" {
		t.Fatalf("texts = %+v", canvas.Texts())
	}
}

func TestCanvasResetClearsContentAndReusesBacking(t *testing.T) {
	var canvas Canvas
	canvas.Fill(Rect{Width: 10, Height: 10}, Color{Red: 1, Alpha: 1})
	canvas.Label("Viva", Rect{Width: 20, Height: 10}, TextStyle{Size: 12})
	rectangleCapacity, textCapacity := cap(canvas.rectangles), cap(canvas.texts)

	canvas.Reset()
	if len(canvas.Rectangles()) != 0 || len(canvas.Triangles()) != 0 || len(canvas.Texts()) != 0 {
		t.Fatalf("reset left %d rectangles, %d triangles, and %d texts", len(canvas.Rectangles()), len(canvas.Triangles()), len(canvas.Texts()))
	}

	canvas.Fill(Rect{Width: 5, Height: 5}, Color{Blue: 1, Alpha: 1})
	if cap(canvas.rectangles) != rectangleCapacity || cap(canvas.texts) != textCapacity {
		t.Fatal("repaint after reset reallocated the backing arrays")
	}
}

func TestCanvasCapturesNestedLayersAndResetRestoresBaseLayer(t *testing.T) {
	var canvas Canvas
	canvas.Fill(Rect{Width: 10, Height: 10}, Color{Alpha: 1})
	canvas.PushLayer(3)
	canvas.Fill(Rect{Width: 8, Height: 8}, Color{Red: 1, Alpha: 1})
	canvas.Label("overlay", Rect{Width: 40, Height: 12}, TextStyle{Size: 12})
	canvas.HitWithLabel(Item(1), Rect{Width: 8, Height: 8}, "overlay")
	canvas.PushLayer(7)
	canvas.Rounded(Rect{Width: 6, Height: 6}, 2, Color{Blue: 1, Alpha: 1})
	canvas.HitWithLabel(Item(2), Rect{Width: 6, Height: 6}, "nested")
	canvas.PopLayer()
	canvas.PopLayer()
	canvas.Label("base", Rect{Width: 40, Height: 12}, TextStyle{Size: 12})

	rectangles := canvas.Rectangles()
	if len(rectangles) != 3 || rectangles[0].Layer != 0 || rectangles[1].Layer != 3 || rectangles[2].Layer != 7 {
		t.Fatalf("rectangle layers = %+v", rectangles)
	}
	texts := canvas.Texts()
	if len(texts) != 2 || texts[0].Layer != 3 || texts[1].Layer != 0 {
		t.Fatalf("text layers = %+v", texts)
	}
	hits := canvas.Hits()
	if len(hits) != 2 || hits[0].Layer != 3 || hits[1].Layer != 7 {
		t.Fatalf("hit layers = %+v", hits)
	}

	canvas.PushLayer(9)
	canvas.Reset()
	canvas.Fill(Rect{Width: 4, Height: 4}, Color{Alpha: 1})
	if len(canvas.layers) != 0 || canvas.Rectangles()[0].Layer != 0 {
		t.Fatal("reset retained the previous compositing layer")
	}
}

func TestHitRegionsRespectNestedClipping(t *testing.T) {
	var canvas Canvas
	canvas.Clip(Rect{X: 10, Y: 20, Width: 100, Height: 80})
	canvas.Clip(Rect{X: 20, Y: 30, Width: 50, Height: 40})
	canvas.HitWithLabel(Item(1), Rect{X: 0, Y: 0, Width: 40, Height: 50}, "Partly visible")
	canvas.HitWithLabel(Item(2), Rect{X: 0, Y: 0, Width: 5, Height: 5}, "Hidden")
	if len(canvas.Hits()) != 1 || canvas.Hits()[0].Bounds != (Rect{X: 20, Y: 30, Width: 20, Height: 20}) {
		t.Fatalf("clipped hits = %+v", canvas.Hits())
	}
	canvas.Unclip()
	canvas.Unclip()
	canvas.Hit(Item(3), Rect{Width: 5, Height: 5})
	if len(canvas.Hits()) != 2 {
		t.Fatal("unclipping must restore normal hit declaration")
	}
}

func TestTypedHitCapturesSemanticStateAndFocusScope(t *testing.T) {
	var canvas Canvas
	canvas.PushFocusScope(Item(42))
	canvas.HitWithOptions(Item(7), Rect{Width: 80, Height: 24}, "General", HitOptions{
		Role: RoleTab, Value: "general", Hint: "Settings section",
		Selected: true, Expanded: true, HideNativeOverlays: true,
	})
	canvas.PopFocusScope()
	canvas.TrapFocus(Item(42))
	canvas.SetEscapeTarget(Item(9))
	canvas.SetInitialFocus(Item(7))

	if len(canvas.Hits()) != 1 {
		t.Fatalf("hits = %+v", canvas.Hits())
	}
	hit := canvas.Hits()[0]
	if hit.Scope != Item(42) || hit.Role != RoleTab || hit.Value != "general" || !hit.Selected || !hit.Expanded || !hit.Focusable || !hit.HideNativeOverlays {
		t.Fatalf("typed hit = %+v", hit)
	}
	if canvas.TrappedFocusScope() != Item(42) || canvas.EscapeTarget() != Item(9) || canvas.InitialFocus() != Item(7) {
		t.Fatalf("scope = %d, escape = %d, initial = %d", canvas.TrappedFocusScope(), canvas.EscapeTarget(), canvas.InitialFocus())
	}

	canvas.Reset()
	if !canvas.TrappedFocusScope().IsZero() || !canvas.EscapeTarget().IsZero() || !canvas.InitialFocus().IsZero() || len(canvas.focusScopes) != 0 {
		t.Fatal("reset retained overlay interaction state")
	}
}

func TestTypedHitsAreFocusableByDefaultWithExplicitOptOut(t *testing.T) {
	var canvas Canvas
	canvas.HitWithOptions(Item(1), Rect{Width: 20, Height: 20}, "Link", HitOptions{Role: RoleLink})
	canvas.HitWithOptions(Item(2), Rect{X: 30, Width: 20, Height: 20}, "Disabled", HitOptions{Role: RoleButton, Disabled: true})
	canvas.HitWithOptions(Item(3), Rect{X: 60, Width: 20, Height: 20}, "Backdrop", HitOptions{Role: RoleButton, SkipFocus: true})

	hits := canvas.Hits()
	if !hits[0].Focusable || hits[1].Focusable || hits[2].Focusable {
		t.Fatalf("focus defaults = %+v", hits)
	}
}

func TestSecureHitNeverCarriesItsValue(t *testing.T) {
	var canvas Canvas
	canvas.HitWithOptions(Item(1), Rect{Width: 100, Height: 20}, "Password", HitOptions{
		Role: RoleTextField, Value: "hunter2", Secure: true,
	})
	canvas.HitWithOptions(Item(2), Rect{Y: 30, Width: 100, Height: 20}, "Name", HitOptions{
		Role: RoleTextField, Value: "alice",
	})
	secure, plain := canvas.Hits()[0], canvas.Hits()[1]
	if !secure.Secure || secure.Value != "" {
		t.Fatalf("secure hit = %+v, want Secure with no value", secure)
	}
	if plain.Secure || plain.Value != "alice" {
		t.Fatalf("plain hit = %+v", plain)
	}
}

// An icon keeps the weight it was drawn with at every display scale. A 16 point
// Lucide icon draws a 1.33 point stroke; rounding that to whole device pixels gave
// 1 point at 100% and 1.5 at 200%, so the same toolbar read a third fainter on a 1x
// display. A stroke rounds up to half a device pixel: never thinner than drawn, and
// no more than a fifth bolder.
func TestIconStrokeKeepsItsDrawnWeightAtEveryScale(t *testing.T) {
	icon := Icon{ViewBox: 24, Stroke: 2, Cap: CapRound, Join: JoinRound, Commands: []PathCommand{MoveTo(4, 12), LineTo(20, 12)}}
	const drawn = 2 * 16.0 / 24
	for _, scale := range []float32{1, 1.25, 1.5, 1.75, 2, 3} {
		var canvas Canvas
		canvas.SetPixelScale(scale)
		canvas.DrawIcon(icon, Rect{X: 3.2, Y: 7.1, Width: 16, Height: 16}, Color{Alpha: 1})
		triangles := canvas.Triangles()
		if len(triangles) == 0 {
			t.Fatalf("scale %v drew nothing", scale)
		}
		low, high := float32(1e9), float32(-1e9)
		for _, triangle := range triangles {
			for _, point := range triangle.Points {
				low, high = min(low, point.Y), max(high, point.Y)
			}
		}
		if weight := high - low; weight < drawn*0.99 || weight > drawn*1.21 {
			t.Errorf("scale %v: a %v point stroke renders %v point", scale, drawn, weight)
		}
	}
}

// The tessellation of an icon is cached per scale: one painted for 2x does not serve
// 1.5x with strokes rounded for the other.
func TestIconCacheDoesNotCrossPixelScales(t *testing.T) {
	icon := Icon{ViewBox: 24, Stroke: 2, Cap: CapRound, Join: JoinRound, Commands: []PathCommand{MoveTo(4, 12), LineTo(20, 12)}}
	var canvas Canvas
	span := func(scale float32) float32 {
		canvas.Reset()
		canvas.SetPixelScale(scale)
		canvas.DrawIcon(icon, Rect{Width: 16, Height: 16}, Color{Alpha: 1})
		low, high := float32(1e9), float32(-1e9)
		for _, triangle := range canvas.Triangles() {
			for _, point := range triangle.Points {
				low, high = min(low, point.Y), max(high, point.Y)
			}
		}
		return (high - low) * scale
	}
	if got := span(2); got < 2.99 || got > 3.01 {
		t.Fatalf("2x stroke spans %v device pixels, want 3", got)
	}
	if got := span(1.5); got < 1.99 || got > 2.01 {
		t.Fatalf("1.5x stroke spans %v device pixels after a 2x paint, want 2", got)
	}
}

func near(a, b float32) bool { return a-b < 1e-4 && b-a < 1e-4 }

// onGrid reports whether a logical coordinate is a whole number of device pixels at scale.
func onGrid(value, scale float32) bool {
	device := value * scale
	return near(device, float32(math.Round(float64(device))))
}

// A border is a whole number of device pixels, the same on all four sides, and its outer edge is on
// the pixel grid: 1 point at 150% is one pixel, not a pixel and a half blurred over two.
func TestBorderIsWholeDevicePixelsOnAllSides(t *testing.T) {
	for _, test := range []struct {
		scale, width float32
		pixels       float32 // the border's thickness in device pixels
	}{
		{1.5, 1, 1}, {1.25, 1, 1}, {1.75, 1, 1}, {2, 1, 2}, {1.5, 2, 3}, {1, 1, 1},
	} {
		var canvas Canvas
		canvas.SetPixelScale(test.scale)
		canvas.RoundedBorder(Rect{X: 10.4, Y: 20.2, Width: 100.3, Height: 40.6}, 6, test.width, Color{Alpha: 1}, Color{Red: 1, Alpha: 1})
		rectangles := canvas.Rectangles()
		if len(rectangles) != 2 {
			t.Fatalf("scale %v: %d rectangles", test.scale, len(rectangles))
		}
		outer, inner := rectangles[0].Bounds, rectangles[1].Bounds
		for _, edge := range []float32{outer.X, outer.Y, outer.X + outer.Width, outer.Y + outer.Height} {
			if !onGrid(edge, test.scale) {
				t.Errorf("scale %v: outer edge %v is between device pixels", test.scale, edge)
			}
		}
		sides := [4]float32{inner.X - outer.X, inner.Y - outer.Y, outer.X + outer.Width - inner.X - inner.Width, outer.Y + outer.Height - inner.Y - inner.Height}
		for _, side := range sides {
			if !near(side*test.scale, test.pixels) {
				t.Errorf("scale %v width %v: border sides %v device pixels, want %v on all four", test.scale, test.width, sides, test.pixels)
				break
			}
		}
	}
}

// A line a point thick is as thick at one position as at the next: rounding both of its edges made it
// one pixel here and two there.
func TestHairlineKeepsItsThicknessAtAnyPosition(t *testing.T) {
	const scale = 1.5
	var canvas Canvas
	canvas.SetPixelScale(scale)
	for step := range 20 {
		canvas.Fill(Rect{X: 5, Y: 10 + float32(step)*0.1, Width: 100, Height: 1}, Color{Alpha: 1})
	}
	for index, rectangle := range canvas.Rectangles() {
		if !near(rectangle.Bounds.Height*scale, 1) || !onGrid(rectangle.Bounds.Y, scale) {
			t.Fatalf("line %d is %v device pixels thick at y %v", index, rectangle.Bounds.Height*scale, rectangle.Bounds.Y)
		}
	}
}

// Rectangles that meet at a logical edge meet at a device edge, so a row of panels has no
// seam of the background between them and no pixel painted twice.
func TestNeighbouringRectanglesShareTheirSnappedEdge(t *testing.T) {
	for _, scale := range []float32{1.25, 1.5, 1.75} {
		var canvas Canvas
		canvas.SetPixelScale(scale)
		x := float32(3.3)
		for _, width := range []float32{33.3, 7.7, 101.1, 14.9, 5.5} {
			canvas.Fill(Rect{X: x, Y: 0, Width: width, Height: 20}, Color{Alpha: 1})
			x += width
		}
		rectangles := canvas.Rectangles()
		for index := 1; index < len(rectangles); index++ {
			previous, current := rectangles[index-1].Bounds, rectangles[index].Bounds
			if !near(previous.X+previous.Width, current.X) {
				t.Errorf("scale %v: rectangle %d ends at %v but the next starts at %v", scale, index-1, previous.X+previous.Width, current.X)
			}
		}
	}
}

// A canvas nobody told the scale keeps the exact geometry it was given.
func TestRectanglesAreExactWithoutAPixelScale(t *testing.T) {
	var canvas Canvas
	bounds := Rect{X: 10.4, Y: 20.2, Width: 100.3, Height: 40.6}
	canvas.Fill(bounds, Color{Alpha: 1})
	canvas.Border(bounds, 1, Color{Alpha: 1}, Color{Alpha: 1})
	rectangles := canvas.Rectangles()
	if rectangles[0].Bounds != bounds || rectangles[1].Bounds != bounds || rectangles[2].Bounds != (Rect{X: 11.4, Y: 21.2, Width: 98.3, Height: 38.6}) {
		t.Fatalf("rectangles = %+v", rectangles)
	}
}

// A circle stays a circle: its width and height are rounded to the same number of pixels, not
// each by the rounding of its own two edges.
func TestSnappedCircleStaysSquare(t *testing.T) {
	for _, scale := range []float32{1.25, 1.5, 1.75} {
		for step := range 12 {
			var canvas Canvas
			canvas.SetPixelScale(scale)
			canvas.Rounded(Rect{X: 3.3 + float32(step)*0.37, Y: 2.9 + float32(step)*0.21, Width: 14, Height: 14}, 7, Color{Alpha: 1})
			bounds := canvas.Rectangles()[0].Bounds
			if !near(bounds.Width, bounds.Height) {
				t.Fatalf("scale %v step %d: a 14 point circle is %v by %v", scale, step, bounds.Width, bounds.Height)
			}
		}
	}
}

func TestElevateCastsOneOrTwoShadowLayersInTheCurrentLayerAndClip(t *testing.T) {
	var canvas Canvas
	canvas.PushLayer(3)
	canvas.Clip(Rect{Width: 50, Height: 50})
	canvas.Elevate(Rect{X: 10, Y: 10, Width: 20, Height: 20}, 4, ShadowXS)
	canvas.Elevate(Rect{X: 10, Y: 10, Width: 20, Height: 20}, 4, ShadowMD)
	shadows := canvas.Shadows()
	if len(shadows) != 3 {
		t.Fatalf("got %d shadows, want 1 layer of ShadowXS and 2 of ShadowMD", len(shadows))
	}
	for _, shadow := range shadows {
		if shadow.Style.Blur <= 0 || shadow.Layer != 3 || !shadow.HasClip || shadow.Radius != 4 {
			t.Fatalf("shadow layer = %+v", shadow)
		}
		if shadow.Color.Red != 0 || shadow.Color.Alpha <= 0 || shadow.Color.Alpha > 0.2 {
			t.Fatalf("shadow color = %+v, want faint black", shadow.Color)
		}
	}
	if extent := shadows[2].Style.Extent(); extent < shadows[2].Style.Blur {
		t.Fatalf("extent %v is smaller than the blur %v", extent, shadows[2].Style.Blur)
	}
	if len(canvas.Rectangles()) != 0 {
		t.Fatal("a shadow is not a rectangle")
	}
}

func TestShadowsThatPaintNothingAreNotRecorded(t *testing.T) {
	var canvas Canvas
	canvas.Elevate(Rect{Width: 20, Height: 20}, 4, ShadowNone)
	canvas.Elevate(Rect{Width: 20, Height: 20}, 4, ShadowLevel(200))
	canvas.Shadow(Rect{Width: 20, Height: 20}, 4, ShadowStyle{OffsetY: 2}, Color{Alpha: 1})
	canvas.Shadow(Rect{Width: 20, Height: 20}, 4, ShadowStyle{Blur: 4}, Color{})
	canvas.Shadow(Rect{Width: 0, Height: 20}, 4, ShadowStyle{Blur: 4}, Color{Alpha: 1})
	if got := len(canvas.Shadows()); got != 0 {
		t.Fatalf("recorded %d shadows that paint nothing", got)
	}
}

// A shadow remembers how many rectangles and triangles came before it, which is
// what puts it back between them in paint order.
func TestShadowsRecordWhereTheyWerePainted(t *testing.T) {
	var canvas Canvas
	canvas.Fill(Rect{Width: 10, Height: 10}, Color{Alpha: 1})
	canvas.Shadow(Rect{Width: 10, Height: 10}, 0, ShadowStyle{Blur: 3}, Color{Alpha: 0.1})
	canvas.Fill(Rect{Width: 10, Height: 10}, Color{Alpha: 1})
	canvas.Fill(Rect{Width: 10, Height: 10}, Color{Alpha: 1})
	canvas.Shadow(Rect{Width: 10, Height: 10}, 0, ShadowStyle{Blur: 3}, Color{Alpha: 0.1})
	got := canvas.Shadows()
	if len(got) != 2 || got[0].RectsBefore != 1 || got[1].RectsBefore != 3 {
		t.Fatalf("shadows = %+v, want them after 1 and 3 rectangles", got)
	}
	canvas.Reset()
	if len(canvas.Shadows()) != 0 {
		t.Fatal("Reset kept the shadows")
	}
}

func TestMoveRegionIsInertAndAllocatesNothingOnceWarm(t *testing.T) {
	var canvas Canvas
	canvas.MoveRegion(Rect{Width: 100, Height: 30})
	hits := canvas.Hits()
	if len(hits) != 1 || !hits[0].MovesWindow || !hits[0].ID.IsZero() || hits[0].Focusable || hits[0].Bounds != (Rect{Width: 100, Height: 30}) {
		t.Fatalf("hits = %+v, want one inert move region", hits)
	}
	canvas.MoveRegion(Rect{})
	if len(canvas.Hits()) != 1 {
		t.Fatal("an empty region was declared")
	}
	allocations := testing.AllocsPerRun(100, func() {
		canvas.Reset()
		canvas.MoveRegion(Rect{Width: 100, Height: 30})
	})
	if allocations != 0 {
		t.Fatalf("MoveRegion allocates %v times per frame", allocations)
	}
}

func TestTrimReleasesSpikeCapacityAndKeepsContent(t *testing.T) {
	var canvas Canvas
	for i := 0; i < 20000; i++ {
		canvas.Hit(Item(i+1), Rect{X: float32(i % 100), Y: float32(i / 100), Width: 1, Height: 1})
	}
	canvas.Reset()
	canvas.Fill(Rect{Width: 10, Height: 10}, Color{Red: 1, Alpha: 1})
	canvas.Hit(Item(1), Rect{Width: 10, Height: 10})
	canvas.Trim()
	if cap(canvas.hits) > 4096 {
		t.Fatalf("hits capacity after Trim = %d, want the spike released", cap(canvas.hits))
	}
	if len(canvas.Rectangles()) != 1 || len(canvas.Hits()) != 1 {
		t.Fatalf("Trim changed the content: %d rectangles, %d hits", len(canvas.Rectangles()), len(canvas.Hits()))
	}
}
