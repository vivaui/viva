//go:build linux && vivagtk

package linux

import (
	"sync"

	"github.com/ebitengine/purego"
)

type gtkClipboardWatch struct {
	object uintptr
	signal uint64
	notify func()
}

var gtkClipboardWatchCallback = sync.OnceValue(func() uintptr {
	return purego.NewCallback(func(object, _ uintptr, _ uintptr) {
		if a := gtkRunning.Load(); a != nil && a.clipboardWatch.object == object {
			if notify := a.clipboardWatch.notify; notify != nil {
				notify()
			}
		}
	})
})

func OnClipboardChange(notify func()) error {
	a := gtkRunning.Load()
	if a == nil {
		if notify == nil {
			return nil
		}
		return unsupported("clipboard change events need an initialized GTK display")
	}
	var err error
	if e := a.onMain(func() { err = a.setClipboardWatch(notify) }); e != nil {
		return e
	}
	return err
}

func (a *gtkApplication) setClipboardWatch(notify func()) error {
	w := &a.clipboardWatch
	if notify == nil {
		w.stop(a.g)
		return nil
	}
	if !a.started || a.stopping {
		return unsupported("clipboard change events need an initialized GTK display")
	}
	if a.wayland {
		// GTK's Wayland selection offers are focus-scoped; owner-change
		// cannot report copies that happen while another client is focused.
		return unsupported("clipboard change events are not available on GTK Wayland")
	}
	if w.signal != 0 {
		w.notify = notify
		return nil
	}
	var supportsSelectionNotification func(uintptr) int32
	if !a.g.lookup(&supportsSelectionNotification, "gdk_display_supports_selection_notification") ||
		supportsSelectionNotification(a.g.gdkDisplayDefault()) == 0 {
		return unsupported("the GTK display does not support clipboard change notifications")
	}
	clipboard, err := a.clipboardAPI()
	if err != nil {
		return err
	}
	if clipboard.clipboard == 0 {
		return unsupported("the GTK display does not have a clipboard")
	}
	signal := a.g.connect(clipboard.clipboard, "owner-change", gtkClipboardWatchCallback(), 0)
	if signal == 0 {
		return unsupported("could not register the GTK clipboard owner-change signal")
	}
	*w = gtkClipboardWatch{object: clipboard.clipboard, signal: signal, notify: notify}
	return nil
}

func (w *gtkClipboardWatch) stop(g *gtkLibrary) {
	if w.signal != 0 {
		g.signalDisc(w.object, w.signal)
	}
	*w = gtkClipboardWatch{}
}

// The direct XCB application has no watch to stop in a GTK build.
func stopClipboardWatch() {}
