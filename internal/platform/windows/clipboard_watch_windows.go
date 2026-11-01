//go:build windows

package windows

import (
	"fmt"
	"sync/atomic"
)

// The clipboard format listener: Windows posts WM_CLIPBOARDUPDATE to the
// window whenever the clipboard's content changes, which the window procedure
// turns into the callback registered here. One listener serves the process.

var (
	addClipboardFormatListener    = user.NewProc("AddClipboardFormatListener")
	removeClipboardFormatListener = user.NewProc("RemoveClipboardFormatListener")

	clipboardWatch atomic.Pointer[func()]
)

const wmClipboardUpdate = 0x031D

// OnClipboardChange registers the function the window procedure calls when the
// system clipboard's content changes. The callback runs on the window's
// thread; it must only hand the news on. An error means no window exists to
// listen with: the clipboard keeps working, only the news is missing.
func OnClipboardChange(notify func()) error {
	clipboardWatch.Store(&notify)
	state := active.Load()
	if state == nil || state.window == 0 {
		return fmt.Errorf("windows: clipboard change needs a running window")
	}
	added, _, callErr := addClipboardFormatListener.Call(state.window)
	if added == 0 {
		return windowsCallError("listen for clipboard changes", callErr)
	}
	return nil
}

// notifyClipboardChange runs in the window procedure.
func notifyClipboardChange() {
	if notify := clipboardWatch.Load(); notify != nil {
		(*notify)()
	}
}
