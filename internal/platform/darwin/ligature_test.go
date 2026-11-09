//go:build darwin && vivahead

package darwin

import (
	"bytes"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

// TestLigatureRunFlagIsPacked guards the flag word the native renderer reads:
// without it a run can never be shaped as one line.
func TestLigatureRunFlagIsPacked(t *testing.T) {
	if got := textRunFlags(gfx.TextRun{Ligatures: true}); got&runLigatures == 0 {
		t.Fatalf("flags = %d, want the ligature bit", got)
	}
	if got := textRunFlags(gfx.TextRun{}); got != 0 {
		t.Fatalf("plain run flags = %d", got)
	}
}

// TestFixedPitchLigatureRunShapesTheFont is the visual gate: with an installed
// programming font, the same run must produce different pixels when shaping is
// allowed. Machines without such a font skip it; the flag test above still runs.
func TestFixedPitchLigatureRunShapesTheFont(t *testing.T) {
	const family = "Maple Mono"
	installed := false
	for _, name := range MonospaceFamilies() {
		if name == family {
			installed = true
			break
		}
	}
	if !installed {
		t.Skipf("%s is not installed", family)
	}
	face := gfx.MonoFamily(family)
	render := func(ligatures bool) []byte {
		offscreen, err := NewOffscreen(100, 32, 2, Color{Alpha: 1})
		if err != nil {
			t.Skipf("no Metal device: %v", err)
		}
		defer offscreen.Close()
		err = offscreen.Set(nil, []TextLabel{{
			Value: "->", Width: 100, Height: 32, FontSize: 20, CellWidth: 12,
			Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
			Mono:  true, MonoFace: face,
			Runs: []gfx.TextRun{{Bytes: 2, Cells: 2, Color: gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}, Ligatures: ligatures}},
		}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		pixels, _, _, err := offscreen.Render()
		if err != nil {
			t.Fatal(err)
		}
		return pixels
	}
	plain, shaped := render(false), render(true)
	if bytes.Equal(plain, shaped) {
		t.Fatal("the ligature run rendered exactly like the per-cell run")
	}
}
