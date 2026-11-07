//go:build js && wasm

package web

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

func TestPainterUsesTheConfiguredClearColor(t *testing.T) {
	clear := ui.Color{Red: 0.1, Green: 0.2, Blue: 0.3, Alpha: 0.4}
	painter := newPainter(&ui.Canvas{}, &Config{ClearColor: clear})
	if painter.clear != clear {
		t.Fatalf("clear = %+v, want %+v", painter.clear, clear)
	}
}

func TestPainterKeepsTheDefaultClearColorWhenNoneIsSet(t *testing.T) {
	painter := newPainter(&ui.Canvas{}, &Config{})
	if painter.clear == (ui.Color{}) {
		t.Fatal("the default clear color is transparent")
	}
}
