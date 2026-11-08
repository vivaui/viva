//go:build linux && vivagtk

package linux

import (
	"errors"
	"runtime"
	"syscall"
	"testing"
)

// File-list writes are an optional GTK capability. Their absence must reach
// callers as the Linux unsupported sentinel, without disabling plain text or
// falling through to an X11 clipboard on a GTK-owned display connection.
func TestGTKClipboardFileWriteReportsUnsupportedWithoutASetter(t *testing.T) {
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
		setText: func(board uintptr, value *byte, length int32) {
			if board != 17 || value == nil || length != 5 {
				t.Fatalf("unexpected text clipboard call: board=%d length=%d", board, length)
			}
			textWrites++
		},
	}
	application := &gtkApplication{mainThread: syscall.Gettid()}
	application.installHooks()
	paths := []string{"/tmp/copied file.txt"}
	if err := SetClipboardFiles(paths); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("GTK without file-list writes returned %v, want ErrUnsupported", err)
	}
	if err := SetClipboardText("hello"); err != nil || textWrites != 1 {
		t.Fatalf("optional file support broke text writes: error=%v writes=%d", err, textWrites)
	}
	fileWrites := 0
	clipboards.setURIs = func(board, uris uintptr) {
		if board != 17 || uris == 0 {
			t.Fatalf("unexpected file clipboard call: board=%d uris=%d", board, uris)
		}
		fileWrites++
	}
	if err := SetClipboardFiles(paths); err != nil || fileWrites != 1 {
		t.Fatalf("GTK with file-list writes returned error=%v writes=%d", err, fileWrites)
	}
}
