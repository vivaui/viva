//go:build js && wasm

package web

import (
	"errors"
	"testing"
)

func TestCursorCSS(t *testing.T) {
	want := [...]string{"default", "text", "pointer", "crosshair", "wait", "ew-resize", "ns-resize", "nwse-resize", "nesw-resize", "not-allowed", "none"}
	for shape, expected := range want {
		actual, err := cursorCSS(uint8(shape))
		if err != nil || actual != expected {
			t.Fatalf("cursorCSS(%d) = %q, %v; want %q", shape, actual, err, expected)
		}
	}
	if _, err := cursorCSS(255); err == nil {
		t.Fatal("cursorCSS accepted an invalid shape")
	}
}

func TestPromiseOnlySystemAPIsFailSynchronously(t *testing.T) {
	if _, err := ClipboardText(); !errors.Is(err, ErrAsyncRequired) {
		t.Fatalf("ClipboardText error = %v", err)
	}
	if err := SetClipboardText("viva"); !errors.Is(err, ErrAsyncRequired) {
		t.Fatalf("SetClipboardText error = %v", err)
	}
	if _, err := OpenFiles(); !errors.Is(err, ErrAsyncRequired) {
		t.Fatalf("OpenFiles error = %v", err)
	}
	if _, err := SaveFile(); !errors.Is(err, ErrAsyncRequired) {
		t.Fatalf("SaveFile error = %v", err)
	}
	if err := PlaySystemSound(0); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("PlaySystemSound error = %v", err)
	}
}
