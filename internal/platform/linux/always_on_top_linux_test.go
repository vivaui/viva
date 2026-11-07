//go:build linux

package linux

import (
	"errors"
	"testing"

	"github.com/vivaui/viva/internal/platform"
)

func TestX11AlwaysOnTopRejectsPanelsAndClosedWindows(t *testing.T) {
	window := &nativeWindow{state: &linuxWindowState{config: platform.Config{WindowMode: platform.WindowPanel}}}
	if err := window.SetAlwaysOnTop(true); !errors.Is(err, platform.ErrAlwaysOnTopUnsupported) {
		t.Fatalf("panel topmost = %v", err)
	}
	window.closed.Store(true)
	if err := window.SetAlwaysOnTop(true); err == nil {
		t.Fatal("closed window accepted topmost")
	}
}
