//go:build js && wasm

package web

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestPainterUsesTheConfiguredClearColor(t *testing.T) {
	clear := gfx.Color{Red: 0.1, Green: 0.2, Blue: 0.3, Alpha: 0.4}
	painter := newPainter(&gfx.Canvas{}, &Config{ClearColor: clear})
	if painter.clear != clear {
		t.Fatalf("clear = %+v, want %+v", painter.clear, clear)
	}
}

func TestPainterKeepsTheDefaultClearColorWhenNoneIsSet(t *testing.T) {
	painter := newPainter(&gfx.Canvas{}, &Config{})
	if painter.clear == (gfx.Color{}) {
		t.Fatal("the default clear color is transparent")
	}
}
