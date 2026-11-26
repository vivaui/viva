package app

import (
	"time"

	"github.com/vivaui/viva/gfx"
)

const statusPreviewLeaveDelay = 220 * time.Millisecond

// StatusPreview describes the content of a non-activating status item preview.
// Viva sets the window mode and anchor. Opened receives the window or the error
// from opening it, so the application can release content it prepared on failure.
type StatusPreview struct {
	Config Config
	Opened func(*Window, error)
}

// statusHoverControl is the optional part of StatusItemControl used by hosts
// that report native pointer entry and exit. Transitions run on the event loop.
type statusHoverControl interface {
	SetStatusHover(func(anchor StatusAnchor, entered bool)) bool
}

// statusMenuControl is the optional part of StatusItemControl used by hosts
// that cache the native menu outside the application.
type statusMenuControl interface{ RefreshMenu() }

// SetHoverPreview shows build's window when the pointer rests on the icon.
// Call it on the event loop. Passing nil disables the preview and closes it.
// False means the host cannot provide a usable hover event for this icon.
// The normal click and menu remain available. A preview closes after the
// pointer leaves both surfaces, or when the application closes the window.
// While a preview is enabled the system tooltip is not shown, so SetTooltip has
// no visible effect. Windows reports false when the icon is hidden in the
// overflow area at that moment and does not retry: call it again once the icon
// is visible.
func (item *StatusItem) SetHoverPreview(build func() StatusPreview) bool {
	if item == nil || item.removed.Load() {
		return false
	}
	if build == nil {
		if preview := item.preview.Swap(nil); preview != nil {
			preview.close()
		}
		if host, ok := item.control.(statusHoverControl); ok {
			return host.SetStatusHover(nil)
		}
		return true
	}
	host, ok := item.control.(statusHoverControl)
	if !ok {
		return false
	}
	if preview := item.preview.Load(); preview != nil {
		preview.build = build
	} else {
		preview = &statusPreview{item: item, build: build}
		item.preview.Store(preview)
		if !host.SetStatusHover(preview.hover) {
			item.preview.CompareAndSwap(preview, nil)
			preview.close()
			return false
		}
		if item.removed.Load() {
			item.preview.CompareAndSwap(preview, nil)
			return false
		}
	}
	return true
}

// RefreshMenu tells the current host to reload the menu after its contents
// change. Call it on the event loop. Hosts that build on each open do nothing.
func (item *StatusItem) RefreshMenu() {
	if item != nil && !item.removed.Load() {
		if menu, ok := item.control.(statusMenuControl); ok {
			menu.RefreshMenu()
		}
	}
}

type statusPreview struct {
	item             *StatusItem
	build            func() StatusPreview
	window           *Window
	stop             func()
	onIcon, onWindow bool
}

func (p *statusPreview) hover(anchor StatusAnchor, entered bool) {
	if p.item.removed.Load() || entered == p.onIcon {
		return
	}
	p.onIcon = entered
	if !entered {
		p.leave()
		return
	}
	p.cancelLeave()
	if p.window != nil {
		return
	}
	spec := p.build()
	cfg := spec.Config
	if cfg.Paint == nil || p.item.removed.Load() || p.item.preview.Load() != p {
		return
	}
	cfg.WindowMode, cfg.Anchor, cfg.Origin = WindowNotification, anchor, nil
	pointer, closed := cfg.OnPointer, cfg.OnClosed
	cfg.OnPointer = func(event gfx.PointerEvent) gfx.EventResult {
		switch event.Kind {
		case gfx.PointerExit:
			p.onWindow = false
			p.leave()
		case gfx.PointerMove, gfx.PointerDown:
			p.onWindow = true
			p.cancelLeave()
		}
		if pointer != nil {
			return pointer(event)
		}
		return gfx.EventResult{}
	}
	cfg.OnClosed = func() {
		p.window, p.onWindow = nil, false
		p.cancelLeave()
		if closed != nil {
			closed()
		}
	}
	window, err := p.item.application.OpenWindow(cfg, nil)
	p.window = window
	if spec.Opened != nil {
		spec.Opened(window, err)
	}
}

func (p *statusPreview) leave() {
	if p.window == nil || p.onIcon || p.onWindow || p.stop != nil {
		return
	}
	p.stop = p.item.application.PostAfter(statusPreviewLeaveDelay, func() {
		p.stop = nil
		p.close()
	})
}

func (p *statusPreview) cancelLeave() {
	if p.stop != nil {
		p.stop()
		p.stop = nil
	}
}

func (p *statusPreview) close() {
	p.cancelLeave()
	if p.window != nil {
		p.window.Close()
	}
}
