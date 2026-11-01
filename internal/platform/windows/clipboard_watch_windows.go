//go:build windows

package windows

import (
	"sync"
	"sync/atomic"
)

// The clipboard format listener: Windows posts WM_CLIPBOARDUPDATE to every window added to the
// clipboard's listener list whenever its content changes, and the window procedure turns that
// into the callback registered here.
//
// The registration belongs to the process, not to a window. It may be made before the first window
// exists (a window listens from its creation) and every window of the process listens, so the
// watch survives any one of them closing. Several listening windows are each told of the same
// change, so the clipboard's sequence number, which moves once per change, decides whether the
// news is new.

var (
	addClipboardFormatListener    = user.NewProc("AddClipboardFormatListener")
	removeClipboardFormatListener = user.NewProc("RemoveClipboardFormatListener")
	getClipboardSequenceNumber    = user.NewProc("GetClipboardSequenceNumber")

	clipboardWatch atomic.Pointer[func()]
	// clipboardSeen is the sequence number of the change last reported.
	clipboardSeen atomic.Uint32
	// clipboardListeners is the windows (HWND) on the listener list.
	clipboardListeners sync.Map
	// clipboardSequence reads the clipboard's sequence number; tests replace it.
	clipboardSequence = func() uint32 {
		sequence, _, _ := getClipboardSequenceNumber.Call()
		return uint32(sequence)
	}
)

const wmClipboardUpdate = 0x031D

// OnClipboardChange registers the function the window procedure calls when the system
// clipboard's content changes, and nil stops the calls. The callback runs on the thread of the
// window that heard the change; it must only hand the news on. A window that cannot listen
// leaves the others listening, and the first failure is returned.
func OnClipboardChange(notify func()) error {
	if notify == nil {
		clipboardWatch.Store(nil)
		clipboardListeners.Range(func(key, _ any) bool {
			removeClipboardFormatListener.Call(key.(uintptr))
			clipboardListeners.Delete(key)
			return true
		})
		return nil
	}
	clipboardSeen.Store(clipboardSequence())
	clipboardWatch.Store(&notify)
	var failure error
	windowStates.Range(func(key, _ any) bool {
		if err := listenToClipboard(key.(uintptr)); err != nil && failure == nil {
			failure = err
		}
		return true
	})
	return failure
}

// listenToClipboard adds a window to the listener list when a callback is registered and the
// window is not on it yet.
func listenToClipboard(window uintptr) error {
	if clipboardWatch.Load() == nil || window == 0 {
		return nil
	}
	if _, listening := clipboardListeners.LoadOrStore(window, struct{}{}); listening {
		return nil
	}
	added, _, callErr := addClipboardFormatListener.Call(window)
	if added == 0 {
		clipboardListeners.Delete(window)
		return windowsCallError("listen for clipboard changes", callErr)
	}
	return nil
}

// forgetClipboardListener drops a window that is being destroyed: Windows removes it from the
// listener list itself.
func forgetClipboardListener(window uintptr) { clipboardListeners.Delete(window) }

// notifyClipboardChange runs in the window procedure.
func notifyClipboardChange() {
	notify := clipboardWatch.Load()
	if notify == nil {
		return
	}
	// A sequence number of 0 means this process may not read it: every message is then taken for
	// a change.
	if sequence := clipboardSequence(); sequence != 0 && clipboardSeen.Swap(sequence) == sequence {
		return // another window of the process already reported this change
	}
	(*notify)()
}
