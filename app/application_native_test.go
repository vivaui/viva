//go:build darwin || windows || linux

package app

import (
	"errors"
	"testing"
)

func TestResultWindowForwardsSetSize(t *testing.T) {
	native := &sizingWindow{}
	window := &Window{control: resultWindow{WindowControl: native}}
	if err := window.SetSize(1280, 720); err != nil {
		t.Fatal(err)
	}
	if native.width != 1280 || native.height != 720 {
		t.Fatalf("SetSize = %dx%d", native.width, native.height)
	}

	unsupported := &Window{control: resultWindow{WindowControl: struct{ WindowControl }{}}}
	if err := unsupported.SetSize(800, 600); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("wrapped unsupported window error = %v", err)
	}
}

func TestResultWindowForwardsAlwaysOnTop(t *testing.T) {
	native := &topmostTestControl{}
	window := &Window{control: resultWindow{WindowControl: native}}
	if err := window.SetAlwaysOnTop(true); err != nil {
		t.Fatal(err)
	}
	if !window.AlwaysOnTop() || len(native.requests) != 1 || !native.requests[0] {
		t.Fatalf("always-on-top request did not reach the native window: requests=%v", native.requests)
	}

	unsupported := &Window{control: resultWindow{WindowControl: &fakeWindow{}}}
	if err := unsupported.SetAlwaysOnTop(true); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("wrapped unsupported window error = %v", err)
	}
}
