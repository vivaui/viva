//go:build linux

package linux

import (
	"testing"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

func TestClipOnlyChangeRegroupsBatches(t *testing.T) {
	rectangles := []render.Rectangle{{Layer: 1}, {Layer: 1}, {Layer: 1}}
	batches := render.GroupBatches(nil, rectangles)
	if len(batches) != 1 {
		t.Fatalf("batches = %+v", batches)
	}
	rectangles[1].Clipped, rectangles[1].Clip = true, scene.Rect{Width: 4, Height: 4}
	batches = render.GroupBatches(batches, rectangles)
	if len(batches) != 3 || !batches[1].Clipped || batches[1].First != 1 || batches[1].Count != 1 {
		t.Fatalf("batches = %+v", batches)
	}
	composite := groupCompositingLayers(batches, nil, nil, nil)
	if len(composite) != 3 || !composite[1].Clipped || composite[1].Clip != rectangles[1].Clip {
		t.Fatalf("composite = %+v", composite)
	}
}

func TestBitmapCacheResizesWithoutRecreatingImage(t *testing.T) {
	pixels := []byte{0, 0, 255, 255}
	label := platform.TextLabel{
		X: 1, Y: 2, Width: 10, Height: 10, Bitmap: pixels, BitmapWidth: 1, BitmapHeight: 1, BitmapStride: 4, BitmapID: 7, BitmapVersion: 1,
	}
	next := label
	next.X, next.Y, next.Width, next.Height, next.Layer = 20, 30, 40, 24, 3
	if labelContentChanged(label, next) {
		t.Fatal("resizing bitmap quad would discard its source image")
	}
	next.BitmapVersion++
	if !labelContentChanged(label, next) {
		t.Fatal("bitmap content change did not invalidate its source image")
	}
}

func TestCompositingLayersInterleaveRectanglesAndText(t *testing.T) {
	rectangles := []render.Rectangle{{Layer: 0}, {Layer: 2}, {Layer: 2}, {Layer: 5}}
	labels := []vulkanLabelTexture{
		{label: platform.TextLabel{Value: "top", Layer: 5}},
		{label: platform.TextLabel{Value: "middle", Layer: 2}},
		{label: platform.TextLabel{Value: "back", Layer: 0}},
	}
	rectangleBatches := render.GroupBatches(nil, rectangles)
	order := groupTextLayers(labels, nil)
	batches := groupCompositingLayers(rectangleBatches, labels, order, nil)
	wantKinds := []compositingBatchKind{compositeRectangles, compositeText, compositeRectangles, compositeText, compositeRectangles, compositeText}
	wantLayers := []uint16{0, 0, 2, 2, 5, 5}
	if len(batches) != len(wantKinds) {
		t.Fatalf("batches = %+v", batches)
	}
	for index := range batches {
		if batches[index].Kind != wantKinds[index] || batches[index].Layer != wantLayers[index] {
			t.Fatalf("batch %d = %+v", index, batches[index])
		}
	}
	if order[0] != 2 || order[1] != 1 || order[2] != 0 {
		t.Fatalf("text order = %v", order)
	}
}
