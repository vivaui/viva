package linux

import "fmt"

// SetTitle posts the title to the X11 event loop, which writes it to the window. The
// title is kept until the loop applies it, so a later call replaces an earlier one that
// has not been applied yet.
func (window *nativeWindow) SetTitle(title string) error {
	if window == nil || window.closed.Load() {
		return fmt.Errorf("linux: window is closed")
	}
	win := window.state.win
	win.titleMu.Lock()
	win.pendingTitle = title
	win.titleMu.Unlock()
	win.postClientMessage(win.commandAtom, commandSetTitle)
	return nil
}

// applyPendingTitle runs on the event loop: it writes the latest title SetTitle stored.
func (win *window) applyPendingTitle() {
	win.titleMu.Lock()
	title := win.pendingTitle
	win.titleMu.Unlock()
	win.setTitle(title)
}
