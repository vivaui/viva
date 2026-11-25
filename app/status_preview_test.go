//go:build darwin || windows || linux

package app

import (
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
)

type previewTestHost struct {
	fakeHost
	hover func(StatusAnchor, bool)
	posts chan func()
}

func (h *previewTestHost) AddStatusItem(cfg StatusItemConfig) (StatusItemControl, error) {
	item, err := h.fakeHost.AddStatusItem(cfg)
	return previewTestIcon{item, h}, err
}

func (h *previewTestHost) OpenWindow(cfg Config, canvas *gfx.Canvas, closed func()) (WindowControl, error) {
	return h.fakeHost.OpenWindow(cfg, canvas, func() {
		if cfg.OnClosed != nil {
			cfg.OnClosed()
		}
		closed()
	})
}

func (h *previewTestHost) Post(fn func()) { h.posts <- fn }

type previewTestIcon struct {
	StatusItemControl
	host *previewTestHost
}

func (i previewTestIcon) SetStatusHover(fn func(StatusAnchor, bool)) bool {
	i.host.hover = fn
	return true
}

func TestStatusPreviewCrossesGapAndClosesOnce(t *testing.T) {
	host := &previewTestHost{posts: make(chan func(), 4)}
	app := &Application{host: host}
	t.Cleanup(func() {
		for _, window := range host.windows {
			window.Close()
		}
	})
	item, err := app.AddStatusItem(StatusItemConfig{Icon: StatusIcon{Size: 1, RGBA: make([]byte, 4)}})
	if err != nil {
		t.Fatal(err)
	}
	closed := 0
	if !item.SetHoverPreview(func() StatusPreview {
		return StatusPreview{Config: Config{
			Paint:    func(*gfx.Canvas, gfx.Size) {},
			OnClosed: func() { closed++ },
		}}
	}) {
		t.Fatal("preview registration failed")
	}
	anchor := StatusAnchor{set: true}
	host.hover(anchor, true)
	host.hover(anchor, true)
	if len(host.windows) != 1 || host.windows[0].config.WindowMode != WindowNotification || host.windows[0].config.Anchor != anchor {
		t.Fatal("expected one non-activating window beside the icon")
	}
	window := host.windows[0]
	host.hover(anchor, false)
	window.config.OnPointer(gfx.PointerEvent{Kind: gfx.PointerMove})
	if window.done {
		t.Fatal("moving from the icon to the window closed the preview")
	}
	window.config.OnPointer(gfx.PointerEvent{Kind: gfx.PointerExit})
	select {
	case task := <-host.posts:
		task()
	case <-time.After(2 * time.Second):
		t.Fatal("leaving both surfaces did not post a close")
	}
	if !window.done || closed != 1 {
		t.Fatalf("window done=%v, OnClosed called %d times", window.done, closed)
	}
	host.hover(anchor, true)
	host.windows[1].Close()
	host.hover(anchor, true)
	if len(host.windows) != 2 {
		t.Fatal("closing while still hovering reopened the preview")
	}
	host.hover(anchor, false)
	host.hover(anchor, true)
	item.Remove()
	(<-host.posts)()
	if len(host.windows) != 3 || closed != 3 || !host.windows[2].done {
		t.Fatal("removing the item did not close its preview")
	}
}

func TestStatusPreviewWithoutHoverControlLeavesIconUsable(t *testing.T) {
	host := &fakeHost{}
	app := &Application{host: host}
	clicked := false
	item, err := app.AddStatusItem(StatusItemConfig{
		Icon:    StatusIcon{Size: 1, RGBA: make([]byte, 4)},
		OnClick: func(StatusClick) { clicked = true },
	})
	if err != nil {
		t.Fatal(err)
	}
	if item.SetHoverPreview(func() StatusPreview { t.Fatal("built on unsupported host"); return StatusPreview{} }) {
		t.Fatal("ordinary status item unexpectedly supports hover")
	}
	host.items[0].config.OnClick(StatusClick{})
	if !clicked {
		t.Fatal("normal click was lost")
	}
}
