package windows

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestSideButtonsAreBackAndForward(t *testing.T) {
	for _, test := range []struct {
		wParam uintptr
		want   gfx.PointerButton
		known  bool
	}{
		{1 << 16, gfx.PointerButtonBack, true},
		{2<<16 | mkLButton, gfx.PointerButtonForward, true},
		{3 << 16, gfx.PointerButtonNone, false},
		{0, gfx.PointerButtonNone, false},
	} {
		if got, known := sideButton(test.wParam); got != test.want || known != test.known {
			t.Errorf("sideButton(%#x) = %v, %v; want %v, %v", test.wParam, got, known, test.want, test.known)
		}
	}
}
