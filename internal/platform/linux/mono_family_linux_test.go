package linux

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestAFixedPitchFaceNamesItsFallbacksToPangoInOrderBeforeTheGenericFamily(t *testing.T) {
	maple := gfx.MonoFamily("Test Maple Mono NF")
	stack := gfx.MonoStack(maple, "Test LXGW WenKai Mono Lite", "Test Symbols")
	for _, test := range []struct {
		name string
		face gfx.MonoFace
		want string
	}{
		{"system", gfx.MonoSystem, "monospace"},
		{"classic", gfx.MonoClassic, "DejaVu Sans Mono"},
		{"family", maple, "Test Maple Mono NF, monospace"},
		{"stack", stack, "Test Maple Mono NF, Test LXGW WenKai Mono Lite, Test Symbols, monospace"},
		{"system stack", gfx.MonoStack(gfx.MonoSystem, "Test CJK"), "Test CJK, monospace"},
		{"classic stack", gfx.MonoStack(gfx.MonoClassic, "Test CJK"), "DejaVu Sans Mono, Test CJK, monospace"},
	} {
		if got := monoFamily(test.face); got != test.want {
			t.Errorf("%s: family list = %q, want %q", test.name, got, test.want)
		}
	}
}
