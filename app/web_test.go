package app

import (
	"errors"
	"testing"

	"github.com/vivaui/viva/ui"
)

func TestNativeViewsReachThePlatformWhenTheyChange(t *testing.T) {
	var canvas ui.Canvas
	var declare bool
	config := Config{Title: "web", Width: 200, Height: 100, Paint: func(canvas *ui.Canvas, size ui.Size) {
		if declare {
			canvas.PushLayer(1)
			canvas.Clip(ui.Rect{X: 0, Y: 0, Width: 150, Height: 100})
			canvas.NativeView(4, ui.Rect{X: 10, Y: 20, Width: 300, Height: 60})
		}
	}}
	painter := newTestPainter(t, &canvas, config)
	frame, err := painter.repaint()
	if err != nil || frame.NativeViews != nil {
		t.Fatalf("with no view: %+v, %v", frame.NativeViews, err)
	}
	declare = true
	if frame, err = painter.repaint(); err != nil {
		t.Fatal(err)
	}
	if len(frame.NativeViews) != 1 || frame.NativeViews[0].ID != 4 || frame.NativeViews[0].Bounds.Width != 140 {
		t.Fatalf("declared view = %+v, want id 4 cut to the clip", frame.NativeViews)
	}
	if frame, err = painter.repaint(); err != nil || frame.NativeViews != nil {
		t.Fatalf("an unchanged view was sent again: %+v, %v", frame.NativeViews, err)
	}
	declare = false
	if frame, err = painter.repaint(); err != nil {
		t.Fatal(err)
	}
	if frame.NativeViews == nil || len(frame.NativeViews) != 0 {
		t.Fatalf("a view taken away must be told as an empty, non-nil list: %#v", frame.NativeViews)
	}
}

func TestOpeningAPageWithoutAHostIsUnsupported(t *testing.T) {
	var host *WebHost
	if _, err := host.Open(WebViewConfig{}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("err = %v", err)
	}
	if _, err := (&WebHost{}).Open(WebViewConfig{}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("a host no window filled: %v", err)
	}
}
