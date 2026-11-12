package gfx

import (
	"math"
	"testing"
	"time"
)

var testPresence = Presence{
	Motion: Motion{Duration: 100 * time.Millisecond, Easing: EaseLinear},
	Fade:   true, Scale: 0.9, SlideIn: Point{Y: -10}, SlideOut: Point{Y: -10},
}

// paintPanel paints a two-part overlay the way a component does: a panel with a
// label on it, and a hit region that must not outlive the panel.
func paintPanel(canvas *Canvas, id ID) {
	canvas.BeginPresence(id, 0, testPresence, Point{X: 50, Y: 50})
	canvas.PushLayer(3)
	canvas.Rounded(Rect{X: 0, Y: 0, Width: 100, Height: 100}, 10, Color{Red: 1, Alpha: 1})
	canvas.Label("hello", Rect{X: 0, Y: 0, Width: 100, Height: 20}, TextStyle{Size: 13, Color: Color{Alpha: 1}})
	canvas.Hit(id, Rect{X: 0, Y: 0, Width: 100, Height: 100})
	canvas.PopLayer()
	canvas.EndPresence()
}

func panelAlpha(canvas *Canvas) (float32, bool) {
	if len(canvas.Rectangles()) == 0 {
		return 0, false
	}
	return canvas.Rectangles()[0].Color.Alpha, true
}

func TestPresenceEntersAndLeavesWithoutAppState(t *testing.T) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	id := Item(1)
	step := func(at time.Duration, open bool) (alpha float32, painted bool) {
		canvas.Reset()
		frames.at(at, func() {
			if open {
				paintPanel(&canvas, id)
			}
		})
		alpha, painted = panelAlpha(&canvas)
		return alpha, painted
	}

	// Closed and idle: nothing painted, nothing asked for.
	if _, painted := step(0, false); painted || canvas.Animating() {
		t.Fatalf("closed overlay painted=%v animating=%v", painted, canvas.Animating())
	}
	// Entering rises monotonically from nothing to fully shown.
	previous := float32(-1)
	for at := 0; at <= 100; at += 10 {
		alpha, _ := step(time.Duration(at)*time.Millisecond, true)
		if alpha < previous {
			t.Fatalf("entering alpha fell from %v to %v at %dms", previous, alpha, at)
		}
		previous = alpha
		if want := float32(at) / 100; math.Abs(float64(alpha-want)) > 1e-5 {
			t.Fatalf("alpha at %dms = %v, want %v", at, alpha, want)
		}
		if at < 100 && !canvas.Animating() {
			t.Fatalf("not animating at %dms", at)
		}
	}
	if canvas.Animating() {
		t.Fatal("still animating after the entrance ended")
	}
	if got := canvas.Rectangles()[0]; got.Bounds != (Rect{Width: 100, Height: 100}) || got.Radius != 10 {
		t.Fatalf("settled overlay was moved: %+v", got)
	}
	// The app closes it: the last frame keeps being drawn, fading out, and takes no input.
	previous = 2
	for at := 110; at <= 200; at += 10 {
		alpha, painted := step(time.Duration(at)*time.Millisecond, false)
		if at < 200 && (!painted || !canvas.Animating()) {
			t.Fatalf("leaving overlay missing at %dms (painted=%v animating=%v)", at, painted, canvas.Animating())
		}
		if len(canvas.Hits()) != 0 || !canvas.TrappedFocusScope().IsZero() || !canvas.EscapeTarget().IsZero() {
			t.Fatalf("a leaving overlay must not take input: hits=%d", len(canvas.Hits()))
		}
		if painted {
			if alpha >= previous {
				t.Fatalf("leaving alpha rose from %v to %v at %dms", previous, alpha, at)
			}
			previous = alpha
		}
		if painted && canvas.Rectangles()[0].Layer != 3 {
			t.Fatalf("a leaving overlay lost its layer: %d", canvas.Rectangles()[0].Layer)
		}
	}
	if _, painted := step(210*time.Millisecond, false); painted || canvas.Animating() {
		t.Fatalf("overlay still drawn after its exit: painted=%v animating=%v", painted, canvas.Animating())
	}
	if len(canvas.motion.presences) != 0 {
		t.Fatalf("a finished exit kept %d entries", len(canvas.motion.presences))
	}
}

func TestPresenceReversesFromWhereItIs(t *testing.T) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	id := Item(2)
	alphaAt := func(at time.Duration, open bool) float32 {
		canvas.Reset()
		frames.at(at, func() {
			if open {
				paintPanel(&canvas, id)
			}
		})
		alpha, _ := panelAlpha(&canvas)
		return alpha
	}
	alphaAt(0, true)
	rising := alphaAt(40*time.Millisecond, true)
	// Closing in the middle of the entrance continues from the same opacity.
	falling := alphaAt(40*time.Millisecond, false)
	if math.Abs(float64(rising-falling)) > 1e-5 || rising < 0.3 || rising > 0.5 {
		t.Fatalf("closing mid-entrance jumped: %v then %v", rising, falling)
	}
	later := alphaAt(60*time.Millisecond, false)
	if later >= falling {
		t.Fatalf("exit did not fall: %v then %v", falling, later)
	}
	// Opening again in the middle of the exit turns around without a jump.
	again := alphaAt(60*time.Millisecond, true)
	if math.Abs(float64(later-again)) > 1e-5 {
		t.Fatalf("reopening jumped: %v then %v", later, again)
	}
	if next := alphaAt(80*time.Millisecond, true); next <= again {
		t.Fatalf("reopened overlay did not rise: %v then %v", again, next)
	}
}

func TestPresenceIsFinalWithoutClockOrUnderReducedMotion(t *testing.T) {
	// No clock: the overlay is painted as it is and is gone the frame it is not painted.
	var plain Canvas
	plain.BeginFrame()
	paintPanel(&plain, Item(1))
	plain.EndFrame()
	if alpha, _ := panelAlpha(&plain); alpha != 1 || plain.Animating() || len(plain.motion.presences) != 0 {
		t.Fatalf("no clock: alpha=%v animating=%v entries=%d", alpha, plain.Animating(), len(plain.motion.presences))
	}
	plain.Reset()
	plain.BeginFrame()
	plain.EndFrame()
	if len(plain.Rectangles()) != 0 {
		t.Fatal("no clock: a closed overlay left a picture behind")
	}

	var reduced Canvas
	frames := newTestFrames(&reduced)
	reduced.SetReduceMotion(true)
	for _, open := range []bool{true, false} {
		reduced.Reset()
		frames.at(0, func() {
			if open {
				paintPanel(&reduced, Item(1))
			}
		})
		alpha, painted := panelAlpha(&reduced)
		if reduced.Animating() || painted != open || (open && alpha != 1) {
			t.Fatalf("reduced motion open=%v: painted=%v alpha=%v animating=%v", open, painted, alpha, reduced.Animating())
		}
	}

	// An overlay without an identity (a static preview) does not animate.
	var preview Canvas
	frames = newTestFrames(&preview)
	frames.at(0, func() { paintPanel(&preview, ID{}) })
	if alpha, _ := panelAlpha(&preview); alpha != 1 || preview.Animating() {
		t.Fatalf("zero ID: alpha=%v animating=%v", alpha, preview.Animating())
	}
}

func TestPresenceMovesTheWholeGroupAboutItsPivot(t *testing.T) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	canvas.Reset()
	frames.at(0, func() { paintPanel(&canvas, Item(1)) })
	// At progress zero the panel is its smallest, shifted by the full slide.
	rectangle := canvas.Rectangles()[0]
	want := Rect{X: 5, Y: 5 - 10, Width: 90, Height: 90}
	near := func(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-4 }
	if got := rectangle.Bounds; !near(got.X, want.X) || !near(got.Y, want.Y) || !near(got.Width, want.Width) || !near(got.Height, want.Height) || !near(rectangle.Radius, 9) {
		t.Fatalf("panel at the start of entering = %+v radius %v, want %+v", rectangle.Bounds, rectangle.Radius, want)
	}
	text := canvas.Texts()[0]
	if text.Style.Color.Alpha != 0 || text.Bounds.Width != 100 {
		t.Fatalf("label at the start: alpha %v, width %v (size must not scale)", text.Style.Color.Alpha, text.Bounds.Width)
	}
	if hit := canvas.Hits()[0]; hit.Bounds != (Rect{Width: 100, Height: 100}) {
		t.Fatalf("hit region moved with the picture: %+v", hit.Bounds)
	}
}

// A native overlay anchored to a control must take the same placement as the
// control's paint. Otherwise a text caret stays at the final place while its
// drawer is still outside the window.
func TestPresenceMovesHitsAlongWithTheirPaint(t *testing.T) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	canvas.Reset()
	frames.at(0, func() { paintPanel(&canvas, Item(1)) })

	hit := canvas.Hits()[0]
	moved, moving := canvas.HitPresenceRect(0, hit.Bounds)
	if !moving {
		t.Fatal("a hit in an entering overlay did not move")
	}
	if want := canvas.Rectangles()[0].Bounds; moved != want {
		t.Fatalf("hit overlay rect = %+v, painted panel = %+v", moved, want)
	}
	if allocations := testing.AllocsPerRun(100, func() { canvas.HitPresenceRect(0, hit.Bounds) }); allocations != 0 {
		t.Fatalf("moving a native overlay allocated %.2f objects", allocations)
	}

	canvas.Reset()
	frames.at(100*time.Millisecond, func() { paintPanel(&canvas, Item(1)) })
	if _, moving := canvas.HitPresenceRect(0, canvas.Hits()[0].Bounds); moving {
		t.Fatal("a settled overlay still moved its hit")
	}
}

func TestPresenceComposesNestedHitMotions(t *testing.T) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	canvas.Reset()
	frames.at(0, func() {
		canvas.BeginPresence(Item(1), 0, Presence{
			Motion: Motion{Duration: 100 * time.Millisecond, Easing: EaseLinear}, SlideIn: Point{X: 100},
		}, Point{})
		canvas.BeginPresence(Item(2), 0, Presence{
			Motion: Motion{Duration: 100 * time.Millisecond, Easing: EaseLinear}, SlideIn: Point{Y: 10},
		}, Point{})
		canvas.Hit(Item(3), Rect{X: 5, Y: 7, Width: 20, Height: 20})
		canvas.EndPresence()
		canvas.EndPresence()
	})
	moved, moving := canvas.HitPresenceRect(0, canvas.Hits()[0].Bounds)
	if !moving || moved != (Rect{X: 105, Y: 17, Width: 20, Height: 20}) {
		t.Fatalf("nested motion = %+v/%v, want the inner and outer slides composed", moved, moving)
	}
}

func TestPresenceKeepsPaintOrderOfAMixedGroup(t *testing.T) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	icon := Icon{ViewBox: 24, Stroke: 2, Commands: []PathCommand{MoveTo(2, 2), LineTo(20, 2), LineTo(20, 20), LineTo(2, 20), ClosePath()}}
	paint := func(open bool) {
		canvas.Reset()
		canvas.Fill(Rect{Width: 5, Height: 5}, Color{Alpha: 1})
		if open {
			canvas.BeginPresence(Item(1), 0, testPresence, Point{})
			canvas.Fill(Rect{Width: 10, Height: 10}, Color{Alpha: 1})
			canvas.DrawIcon(icon, Rect{Width: 24, Height: 24}, Color{Alpha: 1})
			canvas.Fill(Rect{Width: 20, Height: 20}, Color{Alpha: 1})
			canvas.EndPresence()
		}
	}
	frames.now = 0
	canvas.BeginFrame()
	paint(true)
	canvas.EndFrame()
	if len(canvas.Triangles()) == 0 {
		t.Fatal("icon produced no triangles")
	}
	frames.now = 20 * time.Millisecond
	canvas.BeginFrame()
	paint(false)
	canvas.EndFrame()
	// The leaving copy is appended after the rest of the frame, with the icon
	// still between its two rectangles.
	rectangles := canvas.Rectangles()
	if len(rectangles) != 3 {
		t.Fatalf("rectangles after leaving = %d", len(rectangles))
	}
	last := canvas.Triangles()[len(canvas.Triangles())-1]
	if last.Order <= rectangles[1].Order || last.Order >= rectangles[2].Order {
		t.Fatalf("triangle order %d, want it between the group's two rectangles at %d and %d",
			last.Order, rectangles[1].Order, rectangles[2].Order)
	}
}

func TestPresenceIdleCostsNothingAndKeepsNoState(t *testing.T) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	for step := 0; step < 3; step++ {
		canvas.Reset()
		frames.at(time.Duration(step)*time.Second, func() { canvas.Fill(Rect{Width: 1, Height: 1}, Color{Alpha: 1}) })
		if canvas.Animating() {
			t.Fatal("a frame without overlays asked for another")
		}
		if delay, ok := canvas.NextFrame(); ok {
			t.Fatalf("idle canvas asked for a frame after %v", delay)
		}
	}
	if len(canvas.motion.presences) != 0 || len(canvas.motion.open) != 0 {
		t.Fatal("idle canvas holds presence state")
	}
}

func BenchmarkPresenceOpen(b *testing.B) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	b.ReportAllocs()
	frames.now = 0
	for range 3 {
		canvas.Reset()
		frames.at(frames.now, func() { paintPanel(&canvas, Item(1)) })
		frames.now += time.Second
	}
	b.ResetTimer()
	for range b.N {
		canvas.Reset()
		frames.at(frames.now, func() { paintPanel(&canvas, Item(1)) })
	}
}

func BenchmarkPresenceNone(b *testing.B) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	b.ReportAllocs()
	for range b.N {
		canvas.Reset()
		frames.at(frames.now, func() {
			canvas.Rounded(Rect{Width: 100, Height: 100}, 10, Color{Red: 1, Alpha: 1})
			canvas.Label("hello", Rect{Width: 100, Height: 20}, TextStyle{Size: 13, Color: Color{Alpha: 1}})
			canvas.Hit(Item(1), Rect{Width: 100, Height: 100})
		})
	}
}

// An overlay's shadow is part of the overlay: it fades and shrinks with it, and a
// leaving overlay brings it back in the place it had among the group's rectangles.
func TestPresenceTakesTheOverlaysShadowAlong(t *testing.T) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	paint := func(open bool) {
		canvas.Reset()
		canvas.Fill(Rect{Width: 5, Height: 5}, Color{Alpha: 1})
		if open {
			canvas.BeginPresence(Item(1), 0, testPresence, Point{X: 50, Y: 50})
			canvas.Elevate(Rect{X: 0, Y: 0, Width: 100, Height: 100}, 10, ShadowLG)
			canvas.Rounded(Rect{X: 0, Y: 0, Width: 100, Height: 100}, 10, Color{Alpha: 1})
			canvas.EndPresence()
		}
	}
	frames.now = 0
	canvas.BeginFrame()
	paint(true)
	canvas.EndFrame()
	shadows := canvas.Shadows()
	if len(shadows) != 2 || shadows[0].Color.Alpha != 0 || shadows[0].Bounds.Width >= 100 {
		t.Fatalf("entering shadows = %+v, want two transparent ones, shrunk with the overlay", shadows)
	}
	frames.now = 50 * time.Millisecond
	canvas.BeginFrame()
	paint(true)
	canvas.EndFrame()
	full := shadowLayers[ShadowLG][0]
	if got := canvas.Shadows()[0]; got.Color.Alpha <= 0 || got.Color.Alpha >= full.alpha || got.Style.Blur >= full.style.Blur {
		t.Fatalf("halfway shadow = %+v, want it fainter and tighter than shadow-lg", got)
	}

	frames.now = 60 * time.Millisecond
	canvas.BeginFrame()
	paint(false)
	canvas.EndFrame()
	if len(canvas.Shadows()) != 2 || len(canvas.Rectangles()) != 2 {
		t.Fatalf("leaving overlay has %d shadows and %d rectangles, want 2 and 2", len(canvas.Shadows()), len(canvas.Rectangles()))
	}
	rectangles := canvas.Rectangles()
	for _, shadow := range canvas.Shadows() {
		if shadow.Order <= rectangles[0].Order || shadow.Order >= rectangles[1].Order {
			t.Fatalf("leaving shadow = %+v, want it between the page's rectangle and the overlay's", shadow)
		}
	}
}

// A fade multiplies each shape's opacity, so a border painted as a backing
// under its fill would show through the fill while the overlay fades. Inside a
// fading overlay the border becomes a vector ring: its rounded corners remain,
// and none of it lies under the interior.
func TestABorderInAFadingOverlayKeepsItsCornersAndLeavesItsInteriorToTheFill(t *testing.T) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	paint := func() {
		canvas.BeginPresence(Item(1), 0, Presence{Motion: Motion{Duration: 100 * time.Millisecond}, Fade: true}, Point{})
		canvas.RoundedBorder(Rect{Width: 100, Height: 40}, 6, 1, Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}, Color{Alpha: 1})
		canvas.EndPresence()
	}
	frames.at(0, paint)
	canvas.Reset()
	frames.at(50*time.Millisecond, paint)
	centre := Point{X: 50, Y: 20}
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Color.Red != 1 {
			continue
		}
		t.Fatalf("a border backing remains while the field fades: %+v", rectangle)
	}
	corner := Point{X: 2.1, Y: 2.1}
	cornerCovered := false
	for _, triangle := range canvas.Triangles() {
		if triangle.Color.Red != 1 {
			continue
		}
		if triangle.Color.Alpha <= 0 || triangle.Color.Alpha >= 1 {
			t.Fatalf("border triangle did not fade: %+v", triangle)
		}
		if pointInTriangle(centre, triangle.Points[0], triangle.Points[1], triangle.Points[2]) {
			t.Fatalf("a border triangle %+v covers the interior of a fading field", triangle)
		}
		cornerCovered = cornerCovered || pointInTriangle(corner, triangle.Points[0], triangle.Points[1], triangle.Points[2])
	}
	if !cornerCovered {
		t.Fatal("the rounded corner of a fading border is missing")
	}
}

func TestAFadingBorderRingKeepsItsClipAndPaintOrder(t *testing.T) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	clip := Rect{X: 1, Y: 2, Width: 90, Height: 30}
	frames.at(0, func() {
		canvas.BeginPresence(Item(1), 0, Presence{Motion: Motion{Duration: 100 * time.Millisecond}, Fade: true}, Point{})
		canvas.Clip(clip)
		canvas.RoundedBorder(Rect{Width: 100, Height: 40}, 6, 1, Color{Red: 1, Alpha: 1}, Color{Alpha: 1})
		canvas.Unclip()
		canvas.appendTriangle(Point{X: 10, Y: 10}, Point{X: 20, Y: 10}, Point{X: 10, Y: 20}, Color{Blue: 1, Alpha: 1})
		canvas.EndPresence()
	})
	triangles := canvas.Triangles()
	if len(triangles) < 2 || triangles[len(triangles)-1].Color.Blue != 1 {
		t.Fatal("the border ring moved above a primitive painted after the field")
	}
	for _, triangle := range triangles[:len(triangles)-1] {
		if triangle.Color.Red == 1 && (!triangle.HasClip || triangle.ClipOf != clip) {
			t.Fatalf("border triangle lost its clip: %+v", triangle)
		}
	}
}

func BenchmarkVectorizeRoundedBorderDuringFade(b *testing.B) {
	var canvas Canvas
	paint := func() {
		canvas.RoundedBorder(Rect{Width: 320, Height: 36}, 6, 1, Color{Red: 1, Alpha: 1}, Color{Alpha: 1})
		vectorizeBorderBackings(&canvas, 0)
	}
	paint()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		canvas.Reset()
		paint()
	}
}

// TestPresenceMovesAQuadLabel pins that an overlay's label moves its quad with
// its rasterisation box, without scaling or fading the quad's own tints: the
// fade is already in the label's colour.
func TestPresenceMovesAQuadLabel(t *testing.T) {
	var canvas Canvas
	frames := newTestFrames(&canvas)
	canvas.Reset()
	quad := quadCorners()
	presence := Presence{
		Motion:  Motion{Duration: 100 * time.Millisecond, Easing: EaseLinear},
		Fade:    true,
		SlideIn: Point{X: 100},
	}
	frames.at(0, func() {
		canvas.BeginPresence(Item(1), 0, presence, Point{})
		canvas.Fill(Rect{X: 5, Y: 5, Width: 20, Height: 20}, Color{Alpha: 1})
		canvas.LabelQuad("hello", Rect{X: 5, Y: 5, Width: 90, Height: 20}, quad, TextStyle{Size: 12, Color: Color{Alpha: 1}})
		canvas.EndPresence()
	})

	rectangle := canvas.Rectangles()[0]
	dx, dy := rectangle.Bounds.X-5, rectangle.Bounds.Y-5
	texts := canvas.Texts()
	if len(texts) != 1 {
		t.Fatalf("texts = %d, want 1", len(texts))
	}
	for corner, point := range quad.Points {
		want := Point{X: point.X + dx, Y: point.Y + dy}
		if !pointNear(texts[0].Quad[corner], want) {
			t.Fatalf("corner %d = %+v, want %+v (the quad moves with the label)", corner, texts[0].Quad[corner], want)
		}
	}
	if texts[0].Tint != quad.Tint {
		t.Fatalf("tint = %+v, want the caller's %+v (the fade is in the label colour)", texts[0].Tint, quad.Tint)
	}
}
