package app

import (
	"errors"
	"testing"
)

type sizingWindow struct {
	fakeWindow
	width, height int
	err           error
}

func (window *sizingWindow) SetSize(width, height int) error {
	window.width, window.height = width, height
	return window.err
}

func TestWindowSetSizeReachesTheNativeWindow(t *testing.T) {
	native := &sizingWindow{}
	window := &Window{control: native}
	if err := window.SetSize(1440, 900); err != nil {
		t.Fatal(err)
	}
	if native.width != 1440 || native.height != 900 {
		t.Fatalf("SetSize = %dx%d", native.width, native.height)
	}
	for _, size := range [][2]int{{0, 1}, {1, 0}, {65536, 1}, {1, 65536}} {
		if err := window.SetSize(size[0], size[1]); err == nil {
			t.Fatalf("invalid size %dx%d was accepted", size[0], size[1])
		}
	}
	window.closed.Store(true)
	if err := window.SetSize(800, 600); err == nil || errors.Is(err, ErrUnsupported) {
		t.Fatalf("closed window error = %v", err)
	}
	var nilWindow *Window
	if err := nilWindow.SetSize(800, 600); err == nil {
		t.Fatal("nil window accepted SetSize")
	}
}

func TestWindowSetSizeReportsUnsupportedHosts(t *testing.T) {
	window := &Window{control: &fakeWindow{}}
	if err := window.SetSize(800, 600); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("unsupported host error = %v", err)
	}
}
