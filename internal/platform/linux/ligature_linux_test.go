//go:build linux

package linux

import (
	"bytes"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

// TestFixedPitchLigatureRunShapesTheFont guards the Pango/Cairo shaped-run
// path: the same cells must produce different pixels when the run may ligate.
// DejaVu Sans Mono, the Linux environment's default mono face, covers these.
func TestFixedPitchLigatureRunShapesTheFont(t *testing.T) {
	render := func(ligatures bool) []byte {
		label := platform.TextLabel{
			Value: "fi -> => != <=", Width: 240, Height: 36, FontSize: 22, CellWidth: 13,
			Mono: true, Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
			Runs: []gfx.TextRun{{Bytes: 14, Cells: 14, Color: gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}, Ligatures: ligatures}},
		}
		pixels := renderScene(t, newTestOffscreen(t, 250, 40), nil, []platform.TextLabel{label})
		return pixels.Pix
	}
	if bytes.Equal(render(false), render(true)) {
		t.Fatal("the fixed-pitch renderer drew no ligature for DejaVu Sans Mono")
	}
}
