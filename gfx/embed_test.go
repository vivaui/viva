package gfx

import (
	"testing"
	"time"
)

// testMount returns a place for a view to be mounted at.
func testMount(page *Canvas, n int) Scope {
	tree := page.Tree()
	return tree.Sub(tree.Feature("page"), Index(n))
}

func TestEmbedTranslatesPrimitivesAndMountsIDs(t *testing.T) {
	var child, page Canvas
	child.Fill(Rect{X: 1, Y: 2, Width: 30, Height: 20}, Color{Red: 1, Alpha: 1})
	child.Label("hello", Rect{X: 4, Y: 5, Width: 60, Height: 18}, TextStyle{Size: 12})
	child.HitWithOptions(Item(7), Rect{X: 1, Y: 2, Width: 30, Height: 20}, "seven", HitOptions{Role: RoleButton, TextBounds: Rect{X: 3, Y: 4, Width: 10, Height: 8}})
	child.Block(Rect{X: 0, Y: 0, Width: 10, Height: 10})
	child.DrawPath(Path{width: 1, height: 1, commands: []PathCommand{MoveTo(0, 0), LineTo(1, 0), LineTo(1, 1), LineTo(0, 1)}},
		Rect{X: 0, Y: 0, Width: 10, Height: 10}, PathStyle{Fill: Color{Alpha: 1}})

	mount := testMount(&page, 1)
	page.Embed(&child, Point{X: 100, Y: 200}, mount)

	if got := page.Rectangles(); len(got) != 1 || got[0].Bounds != (Rect{X: 101, Y: 202, Width: 30, Height: 20}) {
		t.Fatalf("rectangles = %+v", got)
	}
	if got := page.Texts(); len(got) != 1 || got[0].Bounds != (Rect{X: 104, Y: 205, Width: 60, Height: 18}) || got[0].Value != "hello" {
		t.Fatalf("texts = %+v", got)
	}
	hits := page.Hits()
	if len(hits) != 2 {
		t.Fatalf("hits = %+v", hits)
	}
	if hits[0].ID != page.Tree().MountID(mount, Item(7)) || hits[0].Bounds != (Rect{X: 101, Y: 202, Width: 30, Height: 20}) || hits[0].Label != "seven" ||
		hits[0].TextBounds != (Rect{X: 103, Y: 204, Width: 10, Height: 8}) {
		t.Fatalf("hit = %+v", hits[0])
	}
	if !hits[1].Block || !hits[1].ID.IsZero() || hits[1].Bounds.X != 100 {
		t.Fatalf("a block region must stay ID-less and move with the view: %+v", hits[1])
	}
	if triangles := page.Triangles(); len(triangles) == 0 || triangles[0].Points[0].X < 100 || triangles[0].Points[0].Y < 200 {
		t.Fatalf("path triangles were not translated: %+v", triangles)
	}
}

func TestEmbedStacksLayersAndClips(t *testing.T) {
	var child, page Canvas
	child.Clip(Rect{X: 0, Y: 0, Width: 50, Height: 50})
	child.Fill(Rect{X: 0, Y: 0, Width: 80, Height: 80}, Color{Alpha: 1})
	child.Unclip()
	child.PushLayer(3)
	child.Fill(Rect{X: 0, Y: 0, Width: 10, Height: 10}, Color{Alpha: 1})
	child.PopLayer()
	child.HitWithLabel(Item(1), Rect{X: 0, Y: 0, Width: 80, Height: 80}, "wide")
	child.PushLayer(4)
	child.HitWithLabel(Item(2), Rect{X: 0, Y: 0, Width: 20, Height: 20}, "corner")
	child.PopLayer()

	page.PushLayer(10)
	page.Clip(Rect{X: 0, Y: 0, Width: 130, Height: 1000})
	page.Embed(&child, Point{X: 100, Y: 100}, 0)
	page.Unclip()
	page.PopLayer()

	rectangles := page.Rectangles()
	if len(rectangles) != 2 {
		t.Fatalf("rectangles = %+v", rectangles)
	}
	// The child's own clip is translated and cut by the page clip's right edge at 130.
	if clip := rectangles[0].ClipOf; !rectangles[0].HasClip || clip != (Rect{X: 100, Y: 100, Width: 30, Height: 50}) {
		t.Fatalf("combined clip = %+v (has %v)", clip, rectangles[0].HasClip)
	}
	// A primitive with no clip of its own still inherits the page's clip.
	if clip := rectangles[1].ClipOf; !rectangles[1].HasClip || clip != (Rect{X: 0, Y: 0, Width: 130, Height: 1000}) {
		t.Fatalf("inherited clip = %+v (has %v)", clip, rectangles[1].HasClip)
	}
	if rectangles[0].Layer != 10 || rectangles[1].Layer != 13 {
		t.Fatalf("layers = %d, %d; want the child's added to the page's 10", rectangles[0].Layer, rectangles[1].Layer)
	}
	hits := page.Hits()
	if len(hits) != 2 || hits[0].Bounds != (Rect{X: 100, Y: 100, Width: 30, Height: 80}) || hits[0].Layer != 10 || hits[1].Layer != 14 {
		t.Fatalf("hit regions were not clipped to the page: %+v", hits)
	}
}

func TestEmbedDropsHitsOutsideThePageClip(t *testing.T) {
	var child, page Canvas
	child.HitWithLabel(Item(1), Rect{X: 0, Y: 0, Width: 40, Height: 40}, "gone")
	page.Clip(Rect{X: 0, Y: 0, Width: 50, Height: 50})
	page.Embed(&child, Point{X: 60, Y: 60}, 0)
	page.Unclip()
	if len(page.Hits()) != 0 {
		t.Fatalf("a view scrolled out of sight must not keep invisible hit regions: %+v", page.Hits())
	}
}

func TestEmbedMountsFocusScopesButLeavesModalStateWithTheView(t *testing.T) {
	var child, page Canvas
	child.PushFocusScope(Item(23))
	child.HitWithLabel(Item(1), Rect{Width: 10, Height: 10}, "inside")
	child.PopFocusScope()
	child.HitWithLabel(Item(2), Rect{X: 20, Width: 10, Height: 10}, "outside")
	child.TrapFocus(Item(23))
	child.SetEscapeTarget(Item(9))
	child.SetInitialFocus(Item(1))

	page.PushFocusScope(Item(4))
	mount := testMount(&page, 2)
	page.Embed(&child, Point{}, mount)
	page.PopFocusScope()

	hits := page.Hits()
	if hits[0].Scope != page.Tree().MountID(mount, Item(23)) || hits[1].Scope != Item(4) {
		t.Fatalf("scopes = %d, %d; want the child's shifted and the page's inherited", hits[0].Scope, hits[1].Scope)
	}
	// One open dialog among many previews must not lock the rest of the page.
	if !page.TrappedFocusScope().IsZero() || !page.EscapeTarget().IsZero() || !page.InitialFocus().IsZero() {
		t.Fatalf("the page adopted the view's modal state: trap %d escape %d focus %d",
			page.TrappedFocusScope(), page.EscapeTarget(), page.InitialFocus())
	}
	// The host can still read it from the child and adopt it deliberately.
	if child.TrappedFocusScope() != Item(23) || child.EscapeTarget() != Item(9) || child.InitialFocus() != Item(1) {
		t.Fatal("the child lost its modal state")
	}
}

func TestBeginEmbeddedHandsTheViewItsOwnInteraction(t *testing.T) {
	var page, view Canvas
	mount := testMount(&page, 3)
	tree := page.Tree()
	page.SetInteraction(tree.MountID(mount, Item(12)), tree.MountID(mount, Item(12)), tree.MountID(mount, Item(30)), true)
	view.BeginEmbedded(&page, mount)
	if view.Hovered() != Item(12) || view.Pressed() != Item(12) || view.Focused() != Item(30) || view.FocusVisible() != Item(30) {
		t.Fatalf("interaction = %v %v %v %v", view.Hovered(), view.Pressed(), view.Focused(), view.FocusVisible())
	}

	// The page's own controls, and another pane's, are not the view's.
	other := testMount(&page, 4)
	page.SetInteraction(Item(5), tree.MountID(other, Item(12)), tree.MountID(other, Item(30)), true)
	view.BeginEmbedded(&page, mount)
	if !view.Hovered().IsZero() || !view.Pressed().IsZero() || !view.Focused().IsZero() || !view.FocusVisible().IsZero() {
		t.Fatalf("a control outside the view leaked in: %v %v %v", view.Hovered(), view.Pressed(), view.Focused())
	}
}

func TestEmbeddedViewSharesTheClockAndRequestsFrames(t *testing.T) {
	now := time.Duration(0)
	var page, view Canvas
	page.SetClock(func() time.Duration { return now })

	mount := testMount(&page, 5)
	paint := func(target float32) float32 {
		view.BeginEmbedded(&page, mount)
		value := view.Animate(Item(1), 0, target, Motion{Duration: 100 * time.Millisecond})
		view.EndFrame()
		page.Embed(&view, Point{}, mount)
		return value
	}
	paint(0)
	now = 10 * time.Millisecond
	paint(1) // the target changes here; the transition runs from this frame
	now = 50 * time.Millisecond
	page.Reset()
	if got := paint(1); got <= 0 || got >= 1 {
		t.Fatalf("the embedded view did not animate on the page clock: %v", got)
	}
	if !page.Animating() {
		t.Fatal("the page was not asked for another frame while the view animates")
	}
	now = time.Second
	page.Reset()
	if got := paint(1); got != 1 {
		t.Fatalf("settled value = %v", got)
	}
}

func TestEmbeddedViewReusesThePagesMeasurerAndReducedMotion(t *testing.T) {
	var page, view Canvas
	page.SetMeasurer(func(string, float32, float32) float32 { return 123 })
	page.SetReduceMotion(true)
	view.BeginEmbedded(&page, testMount(&page, 6))
	if view.TextWidth("anything", 12) != 123 || !view.ReduceMotion() {
		t.Fatal("the embedded view must measure text and honor reduced motion like its page")
	}
}

func TestEmbedCopiesTextRunsAndDelayedFrameRequests(t *testing.T) {
	var child, page Canvas
	child.BeginFrame()
	child.LabelRuns("ab", Rect{Width: 20, Height: 10}, TextStyle{Size: 10}, 10, []TextRun{{Bytes: 1}, {Column: 1, Bytes: 1}})
	child.RequestFrameIn(125 * time.Millisecond)
	child.EndFrame()
	page.BeginFrame()
	page.Embed(&child, Point{X: 5, Y: 7}, testMount(&page, 7))
	page.EndFrame()
	texts := page.Texts()
	if len(texts) != 1 || texts[0].Bounds.X != 5 || texts[0].Bounds.Y != 7 {
		t.Fatalf("embedded texts = %+v", texts)
	}
	runs := page.TextRuns(texts[0])
	if len(runs) != 2 || runs[1].Column != 1 {
		t.Fatalf("embedded runs = %+v", runs)
	}
	if delay, ok := page.NextFrame(); !ok || delay != 125*time.Millisecond {
		t.Fatalf("embedded next frame = %v, %v", delay, ok)
	}
}

func TestEmbedCopiesBitmapGeometry(t *testing.T) {
	bitmap := Bitmap{Pix: make([]byte, 4), Width: 1, Height: 1, Stride: 4, ID: 9}
	var child, page Canvas
	child.DrawBitmap(bitmap, Rect{X: 2, Y: 3, Width: 20, Height: 10})
	page.Embed(&child, Point{X: 5, Y: 7}, 0)
	images := page.Images()
	if len(images) != 1 || images[0].Bounds != (Rect{X: 7, Y: 10, Width: 20, Height: 10}) || images[0].Bitmap.ID != 9 {
		t.Fatalf("embedded images = %+v", images)
	}
}

func TestEmbedZeroAllocationWhenWarm(t *testing.T) {
	var page, view Canvas
	for index := 0; index < 50; index++ {
		x := float32(index)
		view.Fill(Rect{X: x, Width: 4, Height: 4}, Color{Alpha: 1})
		view.Label("label", Rect{X: x, Width: 40, Height: 14}, TextStyle{Size: 12})
		view.HitWithLabel(Item(index+1), Rect{X: x, Width: 4, Height: 4}, "hit")
	}
	mount := testMount(&page, 8)
	page.Embed(&view, Point{X: 3, Y: 4}, mount)
	allocs := testing.AllocsPerRun(100, func() {
		page.Reset()
		page.Clip(Rect{Width: 500, Height: 500})
		page.Embed(&view, Point{X: 3, Y: 4}, mount)
		page.Unclip()
	})
	if allocs != 0 {
		t.Fatalf("embedding allocated %v times per frame", allocs)
	}
}

func TestEmbedIgnoresNilAndSelf(t *testing.T) {
	var page Canvas
	page.Fill(Rect{Width: 1, Height: 1}, Color{Alpha: 1})
	page.Embed(nil, Point{}, 1)
	page.Embed(&page, Point{}, 1)
	if len(page.Rectangles()) != 1 {
		t.Fatal("embedding nil or a canvas into itself must do nothing")
	}
}

func BenchmarkEmbed(b *testing.B) {
	var page, view Canvas
	for index := 0; index < 60; index++ {
		x := float32(index * 3)
		view.Fill(Rect{X: x, Width: 4, Height: 4}, Color{Alpha: 1})
		view.Label("label", Rect{X: x, Width: 40, Height: 14}, TextStyle{Size: 12})
		view.HitWithLabel(Item(index+1), Rect{X: x, Width: 4, Height: 4}, "hit")
	}
	mount := testMount(&page, 9)
	page.Embed(&view, Point{}, mount)
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		page.Reset()
		page.Clip(Rect{Width: 1000, Height: 1000})
		page.Embed(&view, Point{X: 12, Y: 40}, mount)
		page.Unclip()
	}
}

func TestEmbeddedViewPaintsForTheDisplayScaleOfItsPage(t *testing.T) {
	var page, view Canvas
	page.SetPixelScale(1.25)
	view.BeginEmbedded(&page, 0)
	if got := view.PixelScale(); got != 1.25 {
		t.Fatalf("an embedded view paints for scale %v, its page for 1.25", got)
	}
}

func TestEmbeddedDevicePixelImagesStayOnDevicePixelsWhereverTheViewSits(t *testing.T) {
	var surface Surface
	surface.Resize(300, 100)
	for _, scale := range []float32{1.25, 1.5, 1.75} {
		var page, view Canvas
		page.SetPixelScale(scale)
		view.BeginEmbedded(&page, 0)
		surface.DrawPixels(&view, Point{X: 4, Y: 8})
		view.EndFrame()
		page.Embed(&view, Point{X: 265.7, Y: 31.3}, 0)
		for _, image := range page.Images() {
			for _, edge := range []float32{image.Bounds.X, image.Bounds.Y, image.Bounds.Width, image.Bounds.Height} {
				device := float64(edge * scale)
				if d := device - float64(int(device+0.5)); d > 1e-3 || d < -1e-3 {
					t.Fatalf("scale %g: embedded tile %+v is off the device pixels", scale, image.Bounds)
				}
			}
		}
	}
}
