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

func TestAWebSchemeIsCopiedForThePlatform(t *testing.T) {
	var opened platform.WebViewConfig
	host := &WebHost{native: &platform.WebHost{Open: func(config platform.WebViewConfig) (platform.WebView, error) {
		opened = config
		return nil, platform.ErrWebViewUnsupported
	}}}
	var asyncPath string
	scheme := &WebScheme{
		Name: "viva-app",
		Serve: func(path string) (string, []byte, bool) {
			return "text/html; charset=utf-8", []byte(path), true
		},
		Respond: func(request WebRequest) (WebResponse, bool) {
			return WebResponse{
				Status: 201, MIME: "text/css", Headers: map[string]string{"Cache-Control": "no-store"}, Data: []byte(request.Path),
			}, true
		},
		ServeAsync: func(request WebRequest, respond func(WebResponse)) {
			asyncPath = request.Path
			respond(WebResponse{Status: 202, Data: []byte(request.Path)})
		},
	}
	if _, err := host.Open(WebViewConfig{URL: "viva-app://host/index.html", Scheme: scheme}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("err = %v", err)
	}
	if opened.Scheme == nil {
		t.Fatal("the platform got no scheme")
	}
	// The page holds its own copy: what the caller does to its value afterwards does not reach it.
	scheme.Name = "changed"
	scheme.Serve, scheme.Respond, scheme.ServeAsync = nil, nil, nil
	native := opened.Scheme
	if native.Name != "viva-app" || native.Serve == nil || native.Respond == nil || native.ServeAsync == nil {
		t.Fatalf("the platform got a changed scheme: %+v", native)
	}
	if mime, data, ok := native.Serve("index.html"); !ok || mime != "text/html; charset=utf-8" || string(data) != "index.html" {
		t.Fatalf("Serve = %q, %q, %v", mime, data, ok)
	}
	response, ok := native.Respond(platform.WebRequest{Path: "theme.css"})
	if !ok || response.Status != 201 || response.MIME != "text/css" || string(response.Data) != "theme.css" || response.Headers["Cache-Control"] != "no-store" {
		t.Fatalf("Respond = %+v, %v", response, ok)
	}
	var answered platform.WebResponse
	native.ServeAsync(platform.WebRequest{Path: "late"}, func(response platform.WebResponse) { answered = response })
	if asyncPath != "late" || answered.Status != 202 || string(answered.Data) != "late" {
		t.Fatalf("ServeAsync got %q and answered %+v", asyncPath, answered)
	}
}

func TestAPageWithoutASchemeGivesThePlatformNone(t *testing.T) {
	var opened platform.WebViewConfig
	host := &WebHost{native: &platform.WebHost{Open: func(config platform.WebViewConfig) (platform.WebView, error) {
		opened = config
		return nil, platform.ErrWebViewUnsupported
	}}}
	if _, err := host.Open(WebViewConfig{URL: "https://example.com/"}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("err = %v", err)
	}
	if opened.Scheme != nil {
		t.Fatalf("a page with no scheme got %+v", opened.Scheme)
	}
}
