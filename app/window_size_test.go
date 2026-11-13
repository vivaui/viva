package app

import (
	"errors"
	"testing"

	"github.com/vivaui/viva/internal/platform"
)

type sizingWindow struct {
	WindowControl
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
	if err := window.SetSize(65535, 65535); err != nil {
		t.Fatalf("maximum size was rejected: %v", err)
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
	window := &Window{control: struct{ WindowControl }{}}
	if err := window.SetSize(800, 600); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("unsupported host error = %v", err)
	}

	window.control = &sizingWindow{err: platform.ErrWindowSizeUnsupported}
	if err := window.SetSize(800, 600); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("unsupported native window error = %v", err)
	}

	failure := errors.New("native resize failed")
	window.control = &sizingWindow{err: failure}
	if err := window.SetSize(800, 600); !errors.Is(err, failure) || errors.Is(err, ErrUnsupported) {
		t.Fatalf("native resize error = %v", err)
	}
}

func TestContentWindowForwardsSetSize(t *testing.T) {
	native := &sizingWindow{}
	content := &ContentWindow{window: &Window{control: native}}
	if err := content.SetSize(800, 600); err != nil {
		t.Fatal(err)
	}
	if native.width != 800 || native.height != 600 {
		t.Fatalf("SetSize = %dx%d", native.width, native.height)
	}
	var nilContent *ContentWindow
	if err := nilContent.SetSize(800, 600); err == nil {
		t.Fatal("nil content window accepted SetSize")
	}
}
