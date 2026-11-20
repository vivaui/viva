//go:build darwin || windows || linux

package app

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/internal/platform"
)

type iconTestControl struct {
	fakeWindow
	icon WindowIcon
}

func (control *iconTestControl) SetIcon(icon WindowIcon) error {
	control.icon = icon
	return nil
}

func TestWindowIconUpdateValidatesAndCopiesPixels(t *testing.T) {
	control := &iconTestControl{}
	window := &Window{control: control}
	icon := WindowIcon{Size: 1, RGBA: []byte{30, 100, 220, 255}}
	if err := window.SetIcon(icon); err != nil {
		t.Fatal(err)
	}
	icon.RGBA[0] = 200
	if control.icon.RGBA[0] != 30 {
		t.Fatal("the host retained caller-owned pixels")
	}
	for _, invalid := range []WindowIcon{{}, {Size: 1}, {Size: 1025}} {
		if err := window.SetIcon(invalid); err == nil || control.icon.RGBA[0] != 30 {
			t.Fatal("an invalid update changed the icon")
		}
	}
	window.closed.Store(true)
	if err := window.SetIcon(icon); err == nil {
		t.Fatal("a closed window accepted an icon")
	}
	var missing *Window
	if err := missing.SetIcon(icon); err == nil {
		t.Fatal("a nil window accepted an icon")
	}
	if err := (&Window{control: &fakeWindow{}}).SetIcon(icon); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("unsupported window: %v", err)
	}
}

type nativeIconTestControl struct {
	fakeWindow
	icon platform.WindowIcon
}

func (control *nativeIconTestControl) SetIcon(icon platform.WindowIcon) error {
	control.icon = icon
	return nil
}

func TestResultWindowForwardsIconUpdates(t *testing.T) {
	native := &nativeIconTestControl{}
	window := &Window{control: resultWindow{WindowControl: native}}
	resource := filepath.Join(t.TempDir(), "logo.ico")
	if err := window.SetIcon(WindowIcon{Size: 1, RGBA: []byte{1, 2, 3, 255}, ResourcePath: resource}); err != nil {
		t.Fatal(err)
	}
	if native.icon.Size != 1 || native.icon.RGBA[2] != 3 || native.icon.ResourcePath != resource {
		t.Fatalf("native icon = %+v", native.icon)
	}
	window = &Window{control: resultWindow{WindowControl: &fakeWindow{}}}
	if err := window.SetIcon(WindowIcon{Size: 1, RGBA: make([]byte, 4)}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("wrapped unsupported window: %v", err)
	}
}
