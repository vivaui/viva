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
