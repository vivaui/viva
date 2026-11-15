//go:build linux && vivagtk

package linux

import (
	"errors"
	"runtime"
	"syscall"
	"testing"
)

// The rich pair is an optional GTK capability, like the file list: without
// gtk_clipboard_set_with_data its absence must reach callers as the Linux
// unsupported sentinel, and with it the pair goes through one call, without
// disabling plain text.
func TestGTKClipboardHTMLWriteReportsUnsupportedWithoutSetWithData(t *testing.T) {
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)
	previousClipboards, previousHooks := clipboards, shellHooks.Load()
	t.Cleanup(func() {
		clipboards = previousClipboards
		shellHooks.Store(previousHooks)
	})
	textWrites := 0
	clipboards = gtkClipboards{
		ok: true, clipboard: 17,
		setText:    func(board uintptr, value *byte, length int32) { textWrites++ },
		atomIntern: func(*byte, int32) uintptr { return 1 },
	}
	application := &gtkApplication{mainThread: syscall.Gettid()}
	application.installHooks()
	if err := SetClipboardHTML("<b>bold</b>", "bold"); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("GTK without rich writes returned %v, want ErrUnsupported", err)
	}
	if err := SetClipboardText("hello"); err != nil || textWrites != 1 {
		t.Fatalf("optional rich support broke text writes: error=%v writes=%d", err, textWrites)
	}
	htmlWrites := 0
	clipboards.setWithData = func(clipboard, targets uintptr, count int32, get, clear, data uintptr) int32 {
		if clipboard != 17 || targets == 0 || count != 2 || get == 0 || clear == 0 {
			t.Fatalf("unexpected set_with_data call: clipboard=%d targets=%d count=%d", clipboard, targets, count)
		}
		htmlWrites++
		return 1
	}
	clipboards.selectionGet = func(uintptr) uintptr { return 1 }
	clipboards.selectionSet = func(data, kind uintptr, format int32, bytes *byte, length int32) int32 { return 1 }
	if err := SetClipboardHTML("<b>bold</b>", "bold"); err != nil || htmlWrites != 1 {
		t.Fatalf("GTK with rich writes returned error=%v writes=%d", err, htmlWrites)
	}
	if err := SetClipboardText("hello again"); err != nil || textWrites != 2 {
		t.Fatalf("rich writes broke text writes: error=%v writes=%d", err, textWrites)
	}
}
