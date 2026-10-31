package windows

import (
	"testing"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

func TestTriangleEdgeMaskUsesTheExistingInstanceSlot(t *testing.T) {
	instance := makeRectangleInstance(render.Rectangle{Kind: scene.KindTriangle, EdgeMask: 5})
	if instance.Style[3] != 6 {
		t.Fatalf("packed triangle marker = %v, want edge mask + 1 = 6", instance.Style[3])
	}
	instance = makeRectangleInstance(render.Rectangle{Kind: scene.KindRectangle})
	if instance.Style[3] != 0 {
		t.Fatalf("packed rectangle marker = %v, want 0", instance.Style[3])
	}
}

func TestRendererUnchangedFrame(t *testing.T) {
	renderer := renderer{instances: []rectangleInstance{{}}}
	if err := renderer.apply(platform.Frame{}); err != nil {
		t.Fatal(err)
	}
	if renderer.rebuilds != 0 || renderer.partials != 0 || renderer.submissions != 0 || renderer.labelRebuilds != 0 {
		t.Fatal("unchanged frame performed renderer work")
	}
}

func TestRendererRejectsInvalidUpdates(t *testing.T) {
	for _, update := range []render.Update{
		{First: -1, Count: 1},
		{First: 1, Count: 1},
		{First: 0, Count: -1},
		{First: 0, Count: 1},
	} {
		renderer := renderer{}
		if err := renderer.apply(platform.Frame{Update: update}); err == nil {
			t.Errorf("invalid update accepted: %+v", update)
		}
	}
	renderer := renderer{}
	if err := renderer.apply(platform.Frame{Update: render.Update{Count: 1}, Rectangles: make([]render.Rectangle, 2)}); err == nil {
		t.Fatal("growth outside a suffix update accepted")
	}
}

func TestLabelCacheMovesWithoutRasterization(t *testing.T) {
	label := labelTexture{label: platform.TextLabel{Value: "Viva", X: 1, Y: 2, Width: 50, Height: 20, FontSize: 12}, texture: 1, view: 2, scale: 2}
	next := label.label
	next.X, next.Y = 20, 30
	next.Layer = 7
	label.update(next)
	if label.texture != 1 || label.view != 2 || label.scale != 2 || label.label.X != next.X || label.label.Layer != next.Layer || label.label.Value != next.Value {
		t.Fatal("moving label discarded unchanged glyph texture")
	}
	label.texture, label.view = 0, 0
}

func TestBitmapCacheResizesWithoutRasterization(t *testing.T) {
	pixels := []byte{0, 0, 255, 255}
	label := platform.TextLabel{
		X: 1, Y: 2, Width: 10, Height: 10, Bitmap: pixels, BitmapWidth: 1, BitmapHeight: 1, BitmapStride: 4, BitmapID: 7, BitmapVersion: 1,
	}
	next := label
	next.X, next.Y, next.Width, next.Height, next.Layer = 20, 30, 40, 24, 3
	if labelContentChanged(label, next) {
		t.Fatal("resizing bitmap quad would discard its source texture")
	}
	next.BitmapVersion++
	if !labelContentChanged(label, next) {
		t.Fatal("bitmap content change did not invalidate its source texture")
	}
}

func layered(layers ...uint16) []render.Rectangle {
	rectangles := make([]render.Rectangle, len(layers))
	for index, layer := range layers {
		rectangles[index].Layer = layer
	}
	return rectangles
}

func TestGroupRectangleLayersStable(t *testing.T) {
	got := render.GroupBatches(nil, layered(2, 2, 0, 1, 0, 0, 2))
	want := []render.Batch{
		{Layer: 0, First: 2, Count: 1},
		{Layer: 0, First: 4, Count: 2},
		{Layer: 1, First: 3, Count: 1},
		{Layer: 2, First: 0, Count: 2},
		{Layer: 2, First: 6, Count: 1},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d batches, want %d", len(got), len(want))
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("batch %d = %+v, want %+v", index, got[index], want[index])
		}
	}
}

func TestClippedBatchGetsItsOwnScissor(t *testing.T) {
	rectangles := layered(0, 0, 0)
	rectangles[1].Clipped = true
	rectangles[1].Clip = scene.Rect{X: 10.25, Y: 5, Width: 50, Height: 30}
	batches := groupCompositingLayers(render.GroupBatches(nil, rectangles), nil, nil, nil)
	if len(batches) != 3 {
		t.Fatalf("got %d batches, want unclipped, clipped, unclipped: %+v", len(batches), batches)
	}
	whole := scissorRect{Right: 400, Bottom: 200}
	for _, index := range []int{0, 2} {
		if got, visible := rectangleScissor(batches[index], 2, 400, 200); !visible || got != whole {
			t.Fatalf("unclipped batch %d scissor = %+v, %v; want the whole target", index, got, visible)
		}
	}
	got, visible := rectangleScissor(batches[1], 2, 400, 200)
	if want := (scissorRect{Left: 21, Top: 10, Right: 121, Bottom: 70}); !visible || got != want {
		t.Fatalf("clipped batch scissor = %+v, %v; want %+v", got, visible, want)
	}
	batches[1].Clip.X = 300
	if _, visible = rectangleScissor(batches[1], 2, 400, 200); visible {
		t.Fatal("a batch clipped outside the target is drawn")
	}
}

func TestClipOnlyChangeRegroupsBatches(t *testing.T) {
	renderer := renderer{}
	rectangles := layered(0, 0, 0)
	if _, _, err := renderer.storeRectangles(platform.Frame{Update: render.Update{Rebuild: true}, Rectangles: rectangles}); err != nil {
		t.Fatal(err)
	}
	if len(renderer.rectangleBatches) != 1 {
		t.Fatalf("equal clips made %d batches, want 1", len(renderer.rectangleBatches))
	}
	rectangles[1].Clipped = true
	rectangles[1].Clip = scene.Rect{Width: 5, Height: 5}
	if _, _, err := renderer.storeRectangles(platform.Frame{Update: render.Update{First: 1, Count: 1}, Rectangles: rectangles}); err != nil {
		t.Fatal(err)
	}
	if len(renderer.rectangleBatches) != 3 || !renderer.rectangleBatches[1].Clipped {
		t.Fatalf("clip-only update gave %+v, want three batches with the middle one clipped", renderer.rectangleBatches)
	}
	compositing := groupCompositingLayers(renderer.rectangleBatches, nil, nil, nil)
	if len(compositing) != 3 || !compositing[1].Clipped {
		t.Fatalf("compositing batches %+v did not follow the clip", compositing)
	}
}

func TestGroupTextLayersStable(t *testing.T) {
	labels := []labelTexture{
		{label: platform.TextLabel{Value: "first layer two", Layer: 2}},
		{label: platform.TextLabel{Value: "first layer zero", Layer: 0}},
		{label: platform.TextLabel{Value: "second layer two", Layer: 2}},
		{label: platform.TextLabel{Value: "layer one", Layer: 1}},
		{label: platform.TextLabel{Value: "second layer zero", Layer: 0}},
	}
	got := groupTextLayers(labels, nil)
	want := []int{1, 4, 3, 0, 2}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("order %d = %d, want %d", index, got[index], want[index])
		}
	}
}

func TestGroupCompositingLayersInterleavesByLayer(t *testing.T) {
	rectangles := render.GroupBatches(nil, layered(2, 0, 1, 0))
	labels := []labelTexture{
		{label: platform.TextLabel{Value: "layer one", Layer: 1}},
		{label: platform.TextLabel{Value: "first layer zero", Layer: 0}},
		{label: platform.TextLabel{Value: "layer two", Layer: 2}},
		{label: platform.TextLabel{Value: "second layer zero", Layer: 0}},
	}
	order := groupTextLayers(labels, nil)
	got := groupCompositingLayers(rectangles, labels, order, nil)
	want := []compositingBatch{
		{Kind: compositeRectangles, Layer: 0, First: 1, Count: 1},
		{Kind: compositeRectangles, Layer: 0, First: 3, Count: 1},
		{Kind: compositeText, Layer: 0, First: 0, Count: 2},
		{Kind: compositeRectangles, Layer: 1, First: 2, Count: 1},
		{Kind: compositeText, Layer: 1, First: 2, Count: 1},
		{Kind: compositeRectangles, Layer: 2, First: 0, Count: 1},
		{Kind: compositeText, Layer: 2, First: 3, Count: 1},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d batches, want %d: %+v", len(got), len(want), got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("batch %d = %+v, want %+v", index, got[index], want[index])
		}
	}
}

func TestRendererTruncatesLabelState(t *testing.T) {
	renderer := renderer{labels: []labelTexture{{label: platform.TextLabel{Value: "retained"}}, {label: platform.TextLabel{Value: "removed"}}}}
	if err := renderer.apply(platform.Frame{TextLabels: []platform.TextLabel{}}); err != nil {
		t.Fatal(err)
	}
	if len(renderer.labels) != 0 {
		t.Fatal("empty label update did not clear labels")
	}
	for _, label := range renderer.labels[:cap(renderer.labels)] {
		if label.label.Value != "" {
			t.Fatal("removed text retained by backing array")
		}
	}
}

func TestRendererTruncatesRectangleState(t *testing.T) {
	renderer := renderer{
		instances:      []rectangleInstance{{Bounds: [4]float32{1}}, {Bounds: [4]float32{2}}},
		rectangleClips: layered(3, 4),
	}
	if err := renderer.apply(platform.Frame{Update: render.Update{Rebuild: true}, Rectangles: []render.Rectangle{}}); err != nil {
		t.Fatal(err)
	}
	if len(renderer.instances) != 0 || len(renderer.rectangleClips) != 0 {
		t.Fatal("empty rebuild did not truncate rectangle state")
	}
	for _, instance := range renderer.instances[:cap(renderer.instances)] {
		if instance != (rectangleInstance{}) {
			t.Fatal("removed rectangle instance remained in backing storage")
		}
	}
	for _, clip := range renderer.rectangleClips[:cap(renderer.rectangleClips)] {
		if clip.Layer != 0 {
			t.Fatal("removed rectangle layer remained in backing storage")
		}
	}
}

func BenchmarkRendererUnchangedFrame(benchmark *testing.B) {
	renderer := renderer{}
	benchmark.ReportAllocs()
	for range benchmark.N {
		if err := renderer.apply(platform.Frame{}); err != nil {
			benchmark.Fatal(err)
		}
	}
}

func TestLabelScissorFollowsTheCanvasClipAtTheTargetScale(t *testing.T) {
	if size := unsafe.Sizeof(scissorRect{}); size != 16 {
		t.Fatalf("D3D11_RECT is %d bytes, want 16", size)
	}
	whole, visible := labelScissor(platform.TextLabel{}, 2, 400, 200)
	if !visible || whole != (scissorRect{Right: 400, Bottom: 200}) {
		t.Fatalf("an unclipped label gets %+v, %v; want the whole target", whole, visible)
	}

	clipped := platform.TextLabel{Clipped: true}
	clipped.Clip.X, clipped.Clip.Y, clipped.Clip.Width, clipped.Clip.Height = 10.25, -5, 50, 30
	got, visible := labelScissor(clipped, 2, 400, 200)
	if want := (scissorRect{Left: 20, Top: 0, Right: 121, Bottom: 50}); !visible || got != want {
		t.Fatalf("scissor = %+v, %v; want %+v", got, visible, want)
	}

	clipped.Clip.X, clipped.Clip.Y, clipped.Clip.Width, clipped.Clip.Height = 190, 90, 100, 100
	got, visible = labelScissor(clipped, 2, 400, 200)
	if want := (scissorRect{Left: 380, Top: 180, Right: 400, Bottom: 200}); !visible || got != want {
		t.Fatalf("a clip past the target gets %+v, %v; want %+v", got, visible, want)
	}

	clipped.Clip.X = 300
	if _, visible = labelScissor(clipped, 2, 400, 200); visible {
		t.Fatal("a clip outside the target is visible")
	}
}
