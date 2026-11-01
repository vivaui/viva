package ui

import "testing"

func TestNativeViewKeepsItsLayerAndClip(t *testing.T) {
	var canvas Canvas
	canvas.PushLayer(2)
	canvas.Clip(Rect{X: 0, Y: 0, Width: 50, Height: 50})
	canvas.NativeView(7, Rect{X: 10, Y: 10, Width: 100, Height: 100})
	canvas.NativeView(0, Rect{Width: 5, Height: 5}) // no such view
	canvas.NativeView(8, Rect{Width: 0, Height: 5}) // nothing to show
	views := canvas.NativeViews()
	if len(views) != 1 || views[0].ID != 7 || views[0].Layer != 2 || !views[0].HasClip || views[0].ClipOf.Width != 50 {
		t.Fatalf("views = %+v", views)
	}
	canvas.Reset()
	if len(canvas.NativeViews()) != 0 {
		t.Fatal("a reset canvas still has views")
	}
}

func TestEmbeddedNativeViewMovesWithTheView(t *testing.T) {
	var page, view Canvas
	view.NativeView(3, Rect{X: 5, Y: 5, Width: 20, Height: 20})
	page.PushLayer(1)
	page.Embed(&view, Point{X: 100, Y: 40}, 0)
	views := page.NativeViews()
	if len(views) != 1 || views[0].Bounds.X != 105 || views[0].Bounds.Y != 45 || views[0].Layer != 1 {
		t.Fatalf("views = %+v", views)
	}
}
