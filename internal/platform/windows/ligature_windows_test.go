package windows

import (
	"bytes"
	"fmt"
	"image"
	"runtime"
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

func BenchmarkFixedPitchPaddedRun(b *testing.B) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rasterizer, err := newDirectRasterizer()
	if err != nil {
		b.Fatal(err)
	}
	defer rasterizer.close()
	value := "2026/10/26    "
	label := platform.TextLabel{Value: value, Width: 240, Height: 24, FontSize: 14, CellWidth: 8, Mono: true,
		Runs: []gfx.TextRun{{Bytes: int32(len(value)), Cells: int32(len(value)), Ligatures: true, Color: gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}}}}
	b.ReportAllocs()
	for b.Loop() {
		if _, _, err := rasterizer.rasterize(label, 1, 240, 24, true, 0); err != nil {
			b.Fatal(err)
		}
	}
}

// Padding occupies cells; it must not stretch the visible prefix into them.
// PowerShell emits styled commands and dates followed by same-style spaces.
func TestFixedPitchTrailingSpacesDoNotStretchText(t *testing.T) {
	for _, ligatures := range []bool{false, true} {
		for _, scale := range []float64{1, 1.5, 2} {
			for _, weight := range []float32{400, 700} {
				for _, italic := range []bool{false, true} {
					t.Run(fmt.Sprintf("ligatures=%v/scale=%g/weight=%g/italic=%v", ligatures, scale, weight, italic), func(t *testing.T) {
						for _, prefix := range []string{"cd", "2026/10/26", "->"} {
							render := func(padding int) *image.RGBA {
								value := prefix + strings.Repeat(" ", padding)
								label := platform.TextLabel{
									Value: value, Width: 260, Height: 36, FontSize: 20, CellWidth: 12,
									Mono: true, Runs: []gfx.TextRun{{Bytes: int32(len(value)), Cells: int32(len(value)), Weight: weight, Italic: italic, Ligatures: ligatures, Color: gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}}},
								}
								return renderScene(t, newOffscreen(t, 260, 36, scale), nil, []platform.TextLabel{label})
							}
							// The final italic glyph may overhang into the padding;
							// compare inside the original cells, away from that clip.
							width := int(float64(len(prefix)*12-3) * scale)
							height := int(36 * scale)
							if difference := largestDifference(crop(render(0), 0, 0, width, height), crop(render(4), 0, 0, width, height)); difference > 2 {
								t.Errorf("%q changed by %d when trailing spaces were added", prefix, difference)
							}
						}
					})
				}
			}
		}
	}
}

// TestFixedPitchLigatureRunShapesTheFont guards the Direct2D shaped-run path:
// the same cells must produce different pixels when the run may ligate. The
// Windows environments use Consolas, whose programming ligatures cover these
// sequences.
func TestFixedPitchLigatureRunShapesTheFont(t *testing.T) {
	render := func(ligatures bool) []byte {
		label := platform.TextLabel{
			Value: "fi -> => != <=", Width: 240, Height: 36, FontSize: 22, CellWidth: 13,
			Mono: true, Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
			Runs: []gfx.TextRun{{Bytes: 14, Cells: 14, Color: gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}, Ligatures: ligatures}},
		}
		pixels := renderScene(t, newOffscreen(t, 250, 40, 1), nil, []platform.TextLabel{label})
		return pixels.Pix
	}
	if bytes.Equal(render(false), render(true)) {
		t.Fatal("the fixed-pitch renderer drew no ligature for Consolas")
	}
}
