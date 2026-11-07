//go:build linux && vivagtk

package linux

import (
	"errors"
	"testing"

	"github.com/vivaui/viva/internal/platform"
)

func TestGTKWaylandAlwaysOnTopReportsUnsupported(t *testing.T) {
	window := &gtkWindow{app: &gtkApplication{wayland: true}}
	if err := window.SetAlwaysOnTop(true); !errors.Is(err, platform.ErrAlwaysOnTopUnsupported) {
		t.Fatalf("Wayland topmost request = %v", err)
	}
}
