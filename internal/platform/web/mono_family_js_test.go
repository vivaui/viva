//go:build js && wasm

package web

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestAFixedPitchFaceNamesItsFallbacksToTheBrowserInOrderBeforeTheGenericFamily(t *testing.T) {
	stack := gfx.MonoStack(gfx.MonoFamily("Test Maple Mono NF"), "Test LXGW WenKai Mono Lite", "Test Symbols")
	want := "'Test Maple Mono NF', 'Test LXGW WenKai Mono Lite', 'Test Symbols', monospace"
	if got := cssMonoFamilies(stack); got != want {
		t.Errorf("font-family = %q, want %q", got, want)
	}
	if got := cssMonoFamilies(gfx.MonoClassic); got != "'Courier New', Courier, monospace" {
		t.Errorf("classic = %q", got)
	}
	if got := cssMonoFamilies(gfx.MonoSystem); got != "ui-monospace, SFMono-Regular, Menlo, Consolas, monospace" {
		t.Errorf("system = %q", got)
	}
}
