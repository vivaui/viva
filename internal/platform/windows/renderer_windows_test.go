package windows

import (
	"testing"
	"unsafe"

	"github.com/vivaui/viva/gfx"
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
	frame := []int{0, 1, 2, 3, 4}
	got := groupTextLayers(labels, frame, nil)
	want := []int{1, 4, 3, 0, 2}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("order %d = %d, want %d", index, got[index], want[index])
		}
	}
}

func TestGroupCompositingLayersInterleavesByLayer(t *testing.T) {
	rectangles := render.GroupBatches(nil, layered(2, 0, 1, 0))
	// Labels arrive in paint order, saying how many shapes came before each:
	// one after the first shape, two after the second, two after the third.
	labels := []labelTexture{
		{label: platform.TextLabel{Value: "layer one", Layer: 1, ShapesBefore: 1}},
		{label: platform.TextLabel{Value: "first layer zero", Layer: 0, ShapesBefore: 2}},
		{label: platform.TextLabel{Value: "layer two", Layer: 2, ShapesBefore: 3}},
		{label: platform.TextLabel{Value: "second layer zero", Layer: 0, ShapesBefore: 3}},
	}
	order := groupTextLayers(labels, []int{0, 1, 2, 3}, nil)
	got := groupCompositingLayers(rectangles, labels, order, nil)
	want := []compositingBatch{
		{Kind: compositeRectangles, Layer: 0, First: 1, Count: 1},
		{Kind: compositeText, Layer: 0, First: 0, Count: 2},
		{Kind: compositeRectangles, Layer: 0, First: 3, Count: 1},
		{Kind: compositeText, Layer: 1, First: 2, Count: 1},
		{Kind: compositeRectangles, Layer: 1, First: 2, Count: 1},
		{Kind: compositeText, Layer: 2, First: 3, Count: 1},
		{Kind: compositeRectangles, Layer: 2, First: 0, Count: 1},
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

func TestRendererClosesLabelsRemovedFromTheFrame(t *testing.T) {
	renderer := renderer{labels: []labelTexture{{label: platform.TextLabel{Value: "retained"}}, {label: platform.TextLabel{Value: "removed"}}}}
	if err := renderer.apply(platform.Frame{TextLabels: []platform.TextLabel{}}); err != nil {
		t.Fatal(err)
	}
	for _, label := range renderer.labels {
		if label.label.Value != "" {
			t.Fatal("removed text retained by backing array")
		}
	}
	if len(renderer.labelFree) != len(renderer.labels) {
		t.Fatalf("free slots = %d, want all %d", len(renderer.labelFree), len(renderer.labels))
	}
}

func TestRendererReusesAMovedLabelByIdentity(t *testing.T) {
	first := platform.TextLabel{Value: "first", Width: 30, Height: 10, Identity: gfx.TextIdentity{Owner: 1, Item: 11}}
	second := platform.TextLabel{Value: "second", Y: 10, Width: 30, Height: 10, Identity: gfx.TextIdentity{Owner: 1, Item: 12}}
	var renderer renderer
	if err := renderer.apply(platform.Frame{TextLabels: []platform.TextLabel{first, second}}); err != nil {
		t.Fatal(err)
	}
	firstSlot, secondSlot := renderer.labelFrame[0], renderer.labelFrame[1]
	renderer.labels[firstSlot].stale = false
	renderer.labels[secondSlot].stale = false

	// The screen scrolled: the second row is now at the top.
	movedSecond, movedFirst := second, first
	movedSecond.Y, movedFirst.Y = 0, 10
	if err := renderer.apply(platform.Frame{TextLabels: []platform.TextLabel{movedSecond, movedFirst}}); err != nil {
		t.Fatal(err)
	}
	if renderer.labelFrame[0] != secondSlot || renderer.labelFrame[1] != firstSlot {
		t.Fatalf("slots after the move = %v, want %d then %d", renderer.labelFrame, secondSlot, firstSlot)
	}
	if renderer.labels[secondSlot].stale || renderer.labels[firstSlot].stale {
		t.Fatal("a moved label was marked for rasterization")
	}
}

func TestRendererDoesNotShareSlotsAcrossOwners(t *testing.T) {
	first := platform.TextLabel{Value: "same", Width: 30, Height: 10, Identity: gfx.TextIdentity{Owner: 1, Item: 7}}
	other := platform.TextLabel{Value: "same", Width: 30, Height: 10, Identity: gfx.TextIdentity{Owner: 2, Item: 7}}
	var renderer renderer
	if err := renderer.apply(platform.Frame{TextLabels: []platform.TextLabel{first}}); err != nil {
		t.Fatal(err)
	}
	firstSlot := renderer.labelFrame[0]
	renderer.labels[firstSlot].stale = false
	if err := renderer.apply(platform.Frame{TextLabels: []platform.TextLabel{other}}); err != nil {
		t.Fatal(err)
	}
	if renderer.labelFrame[0] == firstSlot {
		t.Fatal("the same item number under another owner reused the retained slot")
	}
	otherSlot := renderer.labelFrame[0]
	renderer.labels[otherSlot].stale = false
	if err := renderer.apply(platform.Frame{TextLabels: []platform.TextLabel{other}}); err != nil {
		t.Fatal(err)
	}
	if renderer.labelFrame[0] != otherSlot || renderer.labels[otherSlot].stale {
		t.Fatal("an unchanged identity did not keep its texture")
	}
}

func TestRendererKeepsThePositionalMatchWithoutIdentity(t *testing.T) {
	first := platform.TextLabel{Value: "first", Width: 30, Height: 10}
	second := platform.TextLabel{Value: "second", Y: 10, Width: 30, Height: 10}
	var renderer renderer
	if err := renderer.apply(platform.Frame{TextLabels: []platform.TextLabel{first, second}}); err != nil {
		t.Fatal(err)
	}
	slots := append([]int(nil), renderer.labelFrame...)
	renderer.labels[slots[0]].stale = false
	renderer.labels[slots[1]].stale = false

	// No identity means the frame index, as before: the slots stay put and the
	// content comparison alone decides whether the textures are still valid.
	first.Value, second.Value = "first changed", "second changed"
	if err := renderer.apply(platform.Frame{TextLabels: []platform.TextLabel{first, second}}); err != nil {
		t.Fatal(err)
	}
	if len(renderer.labelFrame) != len(slots) {
		t.Fatalf("positional slots = %v, want %v", renderer.labelFrame, slots)
	}
	for index := range slots {
		if renderer.labelFrame[index] != slots[index] {
			t.Fatalf("positional slots = %v, want %v", renderer.labelFrame, slots)
		}
	}
	if !renderer.labels[slots[0]].stale || !renderer.labels[slots[1]].stale {
		t.Fatal("changed content at the same position was not marked stale")
	}
}

func TestRendererRedrawsChangedContentUnderOneIdentity(t *testing.T) {
	label := platform.TextLabel{Value: "before", Width: 30, Height: 10, Identity: gfx.TextIdentity{Owner: 1, Item: 7}}
	var renderer renderer
	if err := renderer.apply(platform.Frame{TextLabels: []platform.TextLabel{label}}); err != nil {
		t.Fatal(err)
	}
	slot := renderer.labelFrame[0]
	renderer.labels[slot].stale = false
	label.Value = "after"
	if err := renderer.apply(platform.Frame{TextLabels: []platform.TextLabel{label}}); err != nil {
		t.Fatal(err)
	}
	if renderer.labelFrame[0] != slot || !renderer.labels[slot].stale {
		t.Fatal("changed content under one identity kept the old texture")
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

// A label whose text changes keeps its texture: the next draw rewrites the
// pixels in place when the size is unchanged. Destroying and making a texture
// for every changed row leaves the graphics driver holding the destroyed ones.
func TestChangedLabelKeepsItsTextureAndIsMarkedStale(t *testing.T) {
	label := labelTexture{label: platform.TextLabel{Value: "one", Width: 50, Height: 20, FontSize: 12}, texture: 1, view: 2, scale: 2, width: 100, height: 40}
	moved := label.label
	moved.X = 9
	label.update(moved)
	if label.stale {
		t.Fatal("moving a label marked its texture stale")
	}
	changed := moved
	changed.Value = "two"
	label.update(changed)
	if !label.stale || label.texture != 1 || label.view != 2 || label.width != 100 {
		t.Fatalf("a changed label has stale=%v texture=%d view=%d width=%d, want it stale with its texture kept", label.stale, label.texture, label.view, label.width)
	}
	// The handles above are not real objects: forget them before dropping.
	label.texture, label.view = 0, 0
	label.dropTexture()
	if label.width != 0 || label.stale || label.label.Value != "two" {
		t.Fatal("dropTexture did not reset the texture state, or lost the label")
	}
}

// A grid line's texture covers its runs and a little more, never the whole
// width of a mostly empty row, and never more than the label itself.
func TestGridInkWidthStopsNearTheLastRun(t *testing.T) {
	runs := []gfx.TextRun{{Column: 0, Cells: 5}, {Column: 5, Cells: 7}}
	const cell, scale = 8, 2
	ink := gridInkWidth(runs, cell, scale)
	if ink < 12*cell*scale {
		t.Fatalf("ink width %d does not hold the 12 cells of the runs", ink)
	}
	if ink >= 200*cell*scale {
		t.Fatalf("ink width %d is as wide as an empty row", ink)
	}
	typed := append(append([]gfx.TextRun(nil), runs...), gfx.TextRun{Column: 12, Cells: 1})
	if gridInkWidth(typed, cell, scale) != ink {
		t.Fatal("one more typed character changed the texture width, so every key would make a new texture")
	}
	label := labelTexture{label: platform.TextLabel{Value: "x", Mono: true, Runs: runs, CellWidth: cell, Width: 100 * cell, Height: 20, FontSize: 12}}
	skip, width, ok := label.window(scale)
	if !ok || skip != 0 || width != ink {
		t.Fatalf("window = %d, %d, %v, want 0, %d, true", skip, width, ok, ink)
	}
	label.label.Width = 4
	if _, width, _ = label.window(scale); width > 4*scale {
		t.Fatalf("a narrow label got a texture %d wide, wider than itself", width)
	}
}
