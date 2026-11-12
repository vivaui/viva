package devreload

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/reloadwire"
)

func TestSupervisorPaintPreservesRealAndVisibleFocus(t *testing.T) {
	var canvas gfx.Canvas
	var supervisor supervisor
	for _, testCase := range []struct {
		name         string
		focused      uint32
		focusVisible bool
	}{
		{name: "pointer focus", focused: 7},
		{name: "keyboard focus", focused: 7, focusVisible: true},
		{name: "pointer refocus", focused: 7},
		{name: "blur"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			canvas.SetInteraction(gfx.Item(3), gfx.Item(5), gfx.Item(int(testCase.focused)), testCase.focusVisible)
			canvas.SetScroll(12.5, -8.25)
			supervisor.paint(&canvas, gfx.Size{Width: 640, Height: 480})
			want := reloadwire.PaintRequest{
				Width: 640, Height: 480, Hovered: gfx.Item(3), Pressed: gfx.Item(5), Focused: gfx.Item(int(testCase.focused)),
				FocusVisible: testCase.focusVisible, ScrollX: 12.5, ScrollY: -8.25,
			}
			if supervisor.lastRequest != want {
				t.Fatalf("paint request = %+v, want %+v", supervisor.lastRequest, want)
			}
		})
	}
}
