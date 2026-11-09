package app

import (
	"errors"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

func TestNativeViewsReachThePlatformWhenTheyChange(t *testing.T) {
	var canvas gfx.Canvas
	var declare bool
	config := Config{Title: "web", Width: 200, Height: 100, Paint: func(canvas *gfx.Canvas, size gfx.Size) {
		if declare {
			canvas.PushLayer(1)
			canvas.Clip(gfx.Rect{X: 0, Y: 0, Width: 150, Height: 100})
			canvas.NativeView(4, gfx.Rect{X: 10, Y: 20, Width: 300, Height: 60})
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

func TestAPageProxyMustBeOnThisMachine(t *testing.T) {
	var opened []string
	host := &WebHost{native: &platform.WebHost{Open: func(config platform.WebViewConfig) (platform.WebView, error) {
		opened = append(opened, config.Proxy)
		return nil, platform.ErrWebViewUnsupported
	}}}
	for _, proxy := range []string{"10.0.0.2:1080", "localhost:1080", "127.0.0.1", "127.0.0.1:0", "127.0.0.1:1080 --flag"} {
		if _, err := host.Open(WebViewConfig{Proxy: proxy}); err == nil || errors.Is(err, ErrUnsupported) {
			t.Errorf("proxy %q: err = %v, want it refused before the platform", proxy, err)
		}
	}
	if len(opened) != 0 {
		t.Fatalf("refused proxies reached the platform: %q", opened)
	}
	for _, proxy := range []string{"127.0.0.1:1080", "[::1]:1080"} {
		if _, err := host.Open(WebViewConfig{Proxy: proxy}); !errors.Is(err, ErrUnsupported) {
			t.Errorf("proxy %q: err = %v, want the platform's answer", proxy, err)
		}
	}
	if len(opened) != 2 || opened[0] != "127.0.0.1:1080" || opened[1] != "[::1]:1080" {
		t.Fatalf("the platform got %q", opened)
	}
}
