package platform

import (
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
	"github.com/vivaui/viva/ui"
)

// NativeCuts returns the parts of a native view that are not the view's, in logical window pixels:
// where Viva draws over it on a higher layer (rectangles and labels, not shadows, which have no
// edge to cut at). Adapters whose native view lies above Viva's drawing cut these out of it, so
// what Viva draws shows and takes the pointer. Touching parts are joined, so a menu is one cut and
// not a hundred. The result is appended to into.
func NativeCuts(view NativeView, rectangles []render.Rectangle, labels []TextLabel, into []ui.Rect) []ui.Rect {
	start := len(into)
	add := func(b ui.Rect) {
		b = intersectRect(b, view.Bounds)
		if b.Width <= 0 || b.Height <= 0 {
			return
		}
		into = joinRect(into, start, b)
	}
	for i := range rectangles {
		r := &rectangles[i]
		if r.Layer <= view.Layer || r.Kind == scene.KindShadow || r.Color.Alpha <= 0 {
			continue
		}
		b := ui.Rect{X: r.Bounds.X, Y: r.Bounds.Y, Width: r.Bounds.Width, Height: r.Bounds.Height}
		if r.Clipped {
			b = intersectRect(b, ui.Rect{X: r.Clip.X, Y: r.Clip.Y, Width: r.Clip.Width, Height: r.Clip.Height})
		}
		add(b)
	}
	for i := range labels {
		l := &labels[i]
		if l.Layer <= view.Layer {
			continue
		}
		b := ui.Rect{X: l.X, Y: l.Y, Width: l.Width, Height: l.Height}
		if l.Clipped {
			b = intersectRect(b, l.Clip)
		}
		add(b)
	}
	return into
}

// NativeCovers appends the parts of the view where Viva handles the pointer without drawing
// anything (view.Covers, cut to the view). An adapter whose native view lies above Viva's drawing
// gives the pointer to Viva in these as well as in the cuts, but keeps the picture of the view.
// Touching parts are joined.
func NativeCovers(view NativeView, into []ui.Rect) []ui.Rect {
	start := len(into)
	for _, cover := range view.Covers {
		if b := intersectRect(cover, view.Bounds); b.Width > 0 && b.Height > 0 {
			into = joinRect(into, start, b)
		}
	}
	return into
}

func intersectRect(a, b ui.Rect) ui.Rect {
	left, top := max(a.X, b.X), max(a.Y, b.Y)
	right, bottom := min(a.X+a.Width, b.X+b.Width), min(a.Y+a.Height, b.Y+b.Height)
	if right <= left || bottom <= top {
		return ui.Rect{}
	}
	return ui.Rect{X: left, Y: top, Width: right - left, Height: bottom - top}
}

// joinRect adds b to the rectangles from start on, joining it with any it touches.
func joinRect(list []ui.Rect, start int, b ui.Rect) []ui.Rect {
	for merged := true; merged; {
		merged = false
		for i := start; i < len(list); i++ {
			o := list[i]
			if o.X <= b.X+b.Width && b.X <= o.X+o.Width && o.Y <= b.Y+b.Height && b.Y <= o.Y+o.Height {
				left, top := min(o.X, b.X), min(o.Y, b.Y)
				b = ui.Rect{X: left, Y: top, Width: max(o.X+o.Width, b.X+b.Width) - left, Height: max(o.Y+o.Height, b.Y+b.Height) - top}
				list = append(list[:i], list[i+1:]...)
				merged = true
				break
			}
		}
	}
	return append(list, b)
}

// WebBridgeScript is what every page is given before its own scripts run: window.viva, with
// postMessage(text) to the application and onmessage for what the application posts. The engine's
// message channel is window.webkit.messageHandlers.viva (WebKit) or window.chrome.webview
// (WebView2), which the adapter wires to WebViewConfig.OnMessage.
const WebBridgeScript = `(function () {
  if (window.viva) return;
  var send = function (text) {
    if (window.webkit && window.webkit.messageHandlers && window.webkit.messageHandlers.viva) {
      window.webkit.messageHandlers.viva.postMessage(String(text));
    } else if (window.chrome && window.chrome.webview) {
      window.chrome.webview.postMessage(String(text));
    }
  };
  var viva = { onmessage: null, postMessage: send,
    deliver: function (text) { if (typeof viva.onmessage === 'function') viva.onmessage({ data: text }); } };
  Object.defineProperty(window, 'viva', { value: viva });
})();`
