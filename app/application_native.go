//go:build darwin || windows || linux

package app

import (
	"runtime"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

// RunApplication starts the native application loop. start runs once on the
// locked native event-loop thread and must open at least one window. The call
// blocks until the final window closes or a platform error terminates the loop.
func RunApplication(start func(*Application) error) error {
	attendFaults()
	if applicationRunner != nil {
		return applicationRunner(func(host Host) error {
			application := &Application{host: host}
			if err := start(application); err != nil {
				return err
			}
			if !application.hasOpenedWindow() {
				return ErrNoWindows
			}
			return nil
		})
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	return runNativeApplication(func(native platform.Application) error {
		application := &Application{host: nativeHost{native}}
		if err := start(application); err != nil {
			return err
		}
		if !application.hasOpenedWindow() {
			return ErrNoWindows
		}
		return nil
	})
}

// nativeHost is the Host over a platform adapter.
type nativeHost struct{ native platform.Application }

func (host nativeHost) OpenWindow(config Config, canvas *gfx.Canvas, closed func()) (WindowControl, error) {
	windowConfig, cleanup, err := prepareNativeWindow(config, canvas)
	if err != nil {
		return nil, err
	}
	applicationClosed := windowConfig.OnClosed
	windowConfig.OnClosed = func() {
		cleanup()
		if applicationClosed != nil {
			applicationClosed()
		}
		closed()
	}
	window, err := host.native.OpenWindow(windowConfig)
	if err != nil {
		cleanup()
		return nil, err
	}
	return window, nil
}

// CanPreview says whether the adapter's windows can be a DragPreview.
func (host nativeHost) CanPreview() bool {
	_, ok := host.native.(interface{ SupportsDragPreview() })
	return ok
}

// SetOpenURLHandler passes the handler to adapters whose system delivers links
// to the running program.
func (host nativeHost) SetOpenURLHandler(handler func(string)) {
	if native, ok := host.native.(interface{ SetOpenURLHandler(func(string)) }); ok {
		native.SetOpenURLHandler(handler)
	}
}

// SetOpenFileHandler passes the handler to adapters whose system delivers
// documents to the running program.
func (host nativeHost) SetOpenFileHandler(handler func(string)) {
	if native, ok := host.native.(interface{ SetOpenFileHandler(func(string)) }); ok {
		native.SetOpenFileHandler(handler)
	}
}

func (host nativeHost) SetQuitRequestHandler(handler func()) {
	if native, ok := host.native.(interface{ SetQuitRequestHandler(func()) }); ok {
		native.SetQuitRequestHandler(handler)
	}
}

func (host nativeHost) SetReopenHandler(handler func()) {
	if native, ok := host.native.(interface{ SetReopenHandler(func()) }); ok {
		native.SetReopenHandler(handler)
	}
}

func (host nativeHost) AddStatusItem(config StatusItemConfig) (StatusItemControl, error) {
	var menu func() []platform.MenuItem
	if config.Menu != nil {
		menu = func() []platform.MenuItem { return platformMenu(config.Menu()) }
	}
	item, err := host.native.AddStatusItem(platform.StatusItemConfig{
		Tooltip: config.Tooltip,
		Icon:    config.Icon.platform(),
		Menu:    menu,
		OnClick: func(click platform.StatusClick) {
			if config.OnClick != nil {
				config.OnClick(StatusClick{
					Button:         click.Button,
					Anchor:         StatusAnchor{rect: click.Anchor, set: true},
					PanelDismissed: click.PanelDismissed,
				})
			}
		},
	})
	if err != nil {
		return nil, err
	}
	return nativeStatusItem{item}, nil
}

func (host nativeHost) SetDockBadge(text string) { host.native.SetDockBadge(text) }

func (host nativeHost) SetProgress(fraction float32) { host.native.SetProgress(fraction) }

func (host nativeHost) Quit() { host.native.Quit() }

type nativeStatusItem struct{ item platform.StatusItem }

func (item nativeStatusItem) SetIcon(icon StatusIcon) error {
	return item.item.SetIcon(icon.platform())
}
func (item nativeStatusItem) SetTooltip(tooltip string) { item.item.SetTooltip(tooltip) }
func (item nativeStatusItem) Remove()                   { item.item.Remove() }
