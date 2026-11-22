//go:build linux && vivagtk

package linux

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"runtime"
	"syscall"
	"testing"
)

// The picture is an optional GTK capability, like the rich pair: without
// gtk_clipboard_set_with_data its absence must reach callers as the Linux
// unsupported sentinel, and with it the copy goes through one call advertising
// the single image/png target.
func TestGTKClipboardImageWriteReportsUnsupportedWithoutSetWithData(t *testing.T) {
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)
	previousClipboards, previousHooks := clipboards, shellHooks.Load()
	t.Cleanup(func() {
		clipboards = previousClipboards
		shellHooks.Store(previousHooks)
	})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewNRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	picture := encoded.Bytes()
	clipboards = gtkClipboards{
		ok: true, clipboard: 17,
		atomIntern: func(*byte, int32) uintptr { return 1 },
	}
	application := &gtkApplication{mainThread: syscall.Gettid()}
	application.installHooks()
	if err := SetClipboardImage(picture); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("GTK without set_with_data returned %v, want ErrUnsupported", err)
	}
	imageWrites := 0
	var gets []uintptr
	clipboards.setWithData = func(clipboard, targets uintptr, count int32, get, clear, data uintptr) int32 {
		if clipboard != 17 || targets == 0 || count != 1 || get == 0 || clear == 0 {
			t.Fatalf("unexpected set_with_data call: clipboard=%d targets=%d count=%d", clipboard, targets, count)
		}
		gets = append(gets, get)
		imageWrites++
		return 1
	}
	if err := SetClipboardImage(picture); err != nil || imageWrites != 1 {
		t.Fatalf("GTK picture write returned error=%v writes=%d", err, imageWrites)
	}
	if err := SetClipboardImage(picture); err != nil || imageWrites != 2 {
		t.Fatalf("second GTK picture write returned error=%v writes=%d", err, imageWrites)
	}
	if gets[0] != gets[1] {
		t.Fatal("a second copy made a new callback, which purego never releases")
	}
	if err := SetClipboardImage([]byte("not a PNG")); err == nil || imageWrites != 2 {
		t.Fatalf("a non-PNG reached GTK: error=%v writes=%d", err, imageWrites)
	}
}
