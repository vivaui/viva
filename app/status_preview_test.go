//go:build darwin || windows || linux

package app

import (
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
)

type previewTestHost struct {
	fakeHost
	hover     func(StatusAnchor, bool)
	posts     chan func()
	refreshes int
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

func (i previewTestIcon) RefreshMenu() { i.host.refreshes++ }

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
	if !item.SetHoverPreview(nil) || !item.SetHoverPreview(nil) {
		t.Fatal("disabling a preview must succeed on a host without hover support")
	}
	host.items[0].config.OnClick(StatusClick{})
	if !clicked {
		t.Fatal("normal click was lost")
	}
}

func TestStatusPreviewClosesBeforeClickAndMenuCallbacks(t *testing.T) {
	for _, event := range []string{"click", "menu"} {
		t.Run(event, func(t *testing.T) {
			host := &previewTestHost{posts: make(chan func(), 4)}
			application := &Application{host: host}
			called, closed := 0, 0
			check := func() {
				t.Helper()
				called++
				if len(host.windows) != 1 || !host.windows[0].done || closed != 1 {
					t.Fatal("native callback ran before the preview closed")
				}
			}
			item, err := application.AddStatusItem(StatusItemConfig{
				Icon: StatusIcon{Size: 1, RGBA: make([]byte, 4)},
				OnClick: func(click StatusClick) {
					check()
					if click.Button != gfx.PointerButtonPrimary || !click.PanelDismissed {
						t.Fatal("click wrapping changed the native event")
					}
				},
				Menu: func() []MenuItem { check(); return []MenuItem{{Title: "current menu"}} },
			})
			if err != nil {
				t.Fatal(err)
			}
			item.SetHoverPreview(func() StatusPreview {
				return StatusPreview{Config: Config{Paint: func(*gfx.Canvas, gfx.Size) {}, OnClosed: func() { closed++ }}}
			})
			host.hover(StatusAnchor{}, true)
			if event == "click" {
				host.items[0].config.OnClick(StatusClick{Button: gfx.PointerButtonPrimary, PanelDismissed: true})
			} else if menu := host.items[0].config.Menu(); len(menu) != 1 || menu[0].Title != "current menu" {
				t.Fatal("menu wrapping lost the supplied entries")
			}
			if called != 1 || len(application.windows) != 0 {
				t.Fatalf("callback count = %d, live windows = %d", called, len(application.windows))
			}
		})
	}
}

func TestStatusPreviewReportsOpenFailureAndCanRetry(t *testing.T) {
	host := &previewTestHost{posts: make(chan func(), 4)}
	host.reject = "preview"
	application := &Application{host: host}
	item, err := application.AddStatusItem(StatusItemConfig{Icon: StatusIcon{Size: 1, RGBA: make([]byte, 4)}})
	if err != nil {
		t.Fatal(err)
	}
	var opened *Window
	var reported error
	calls := 0
	item.SetHoverPreview(func() StatusPreview {
		return StatusPreview{
			Config: Config{Title: "preview", Paint: func(*gfx.Canvas, gfx.Size) {}},
			Opened: func(window *Window, err error) { calls++; opened, reported = window, err },
		}
	})
	host.hover(StatusAnchor{}, true)
	if calls != 1 || opened != nil || reported == nil || reported.Error() != "rejected" || len(application.windows) != 0 {
		t.Fatalf("open failure = (%p, %v), calls=%d, live windows=%d", opened, reported, calls, len(application.windows))
	}
	host.reject = ""
	host.hover(StatusAnchor{}, false)
	host.hover(StatusAnchor{}, true)
	if calls != 2 || opened == nil || reported != nil {
		t.Fatalf("retry = (%p, %v), calls=%d", opened, reported, calls)
	}
	opened.Close()
}

func TestStatusPreviewRefreshesTheCachedMenu(t *testing.T) {
	host := &previewTestHost{}
	application := &Application{host: host}
	item, err := application.AddStatusItem(StatusItemConfig{Icon: StatusIcon{Size: 1, RGBA: make([]byte, 4)}})
	if err != nil {
		t.Fatal(err)
	}
	item.RefreshMenu()
	if host.refreshes != 1 {
		t.Fatalf("menu refreshes = %d, want 1", host.refreshes)
	}
	item.Remove()
	item.RefreshMenu()
	if host.refreshes != 1 {
		t.Fatal("removed icon refreshed its cached menu")
	}
}
