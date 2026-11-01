package platform

import (
	"testing"

	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
	"github.com/vivaui/viva/ui"
)

func TestOnlyWhatLiesOverTheViewIsCutAndItIsJoined(t *testing.T) {
	view := NativeView{ID: 1, Layer: 1, Bounds: ui.Rect{Y: 40, Width: 400, Height: 300}, Covers: []ui.Rect{{X: 300, Y: 300, Width: 500, Height: 500}}}
	rect := func(x, y, w, h float32, layer uint16, kind scene.Kind) render.Rectangle {
		return render.Rectangle{Kind: kind, Layer: layer, Bounds: scene.Rect{X: x, Y: y, Width: w, Height: h}, Color: scene.Color{Alpha: 1}}
	}
	rects := []render.Rectangle{
		rect(0, 0, 400, 340, 1, scene.KindRectangle),    // the page's own layer: under the view
		rect(100, 60, 100, 50, 2, scene.KindRectangle),  // a menu: a cut
		rect(100, 110, 100, 30, 2, scene.KindRectangle), // its next row touches it: one cut
		rect(100, 60, 130, 90, 3, scene.KindShadow),     // a shadow has no edge to cut at
	}
	labels := []TextLabel{{X: 110, Y: 70, Width: 60, Height: 14, Layer: 2}, {X: 0, Y: 0, Width: 50, Height: 20, Layer: 5}} // the second is outside the view
	cuts := NativeCuts(view, rects, labels, nil)
	want := []ui.Rect{{X: 100, Y: 60, Width: 100, Height: 80}}
	if len(cuts) != len(want) || cuts[0] != want[0] {
		t.Fatalf("cuts = %v, want %v", cuts, want)
	}
	covers := NativeCovers(view, nil)
	if len(covers) != 1 || covers[0] != (ui.Rect{X: 300, Y: 300, Width: 100, Height: 40}) {
		t.Fatalf("covers = %v, a cover is cut to the view", covers)
	}
}
