package windows

import (
	"bytes"
	"runtime"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

// TestFallbackFamiliesDrawTheCharactersThePrimaryLacks: Consolas has no CJK glyphs, so
// a line drawn with two different CJK fallback families must differ, the same face must
// draw the same pixels twice, and the characters Consolas has must not move. The test
// requires the two CJK families to be installed.
func TestFallbackFamiliesDrawTheCharactersThePrimaryLacks(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rasterizer, err := newDirectRasterizer()
	if err != nil {
		t.Skipf("no DirectWrite: %v", err)
	}
	defer rasterizer.close()
	consolas := gfx.MonoFamily("Consolas")
	white := gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	draw := func(face gfx.MonoFace, value string, runs []gfx.TextRun) []byte {
		label := platform.TextLabel{Value: value, Width: 160, Height: 32, FontSize: 20, CellWidth: 12, Mono: true, MonoFace: face, Runs: runs}
		pixels, _, err := rasterizer.rasterize(label, 1, 160, 32, false, 0)
		if err != nil {
			t.Fatal(err)
		}
		return append([]byte(nil), pixels...)
	}
	runs := []gfx.TextRun{{Bytes: 3, Cells: 2, Color: white}, {Column: 2, Bytes: 3, Cells: 2, Color: white}}
	first := gfx.MonoStack(consolas, "Microsoft YaHei UI")
	second := gfx.MonoStack(consolas, "SimSun")
	a, again, b := draw(first, "你好", runs), draw(first, "你好", runs), draw(second, "你好", runs)
	if !bytes.Equal(a, again) {
		t.Fatal("the same face drew two different images")
	}
	if bytes.Equal(a, b) {
		t.Fatal("two different fallback families drew the same pixels: the fallback list was not used")
	}
	// The bare primary also uses DirectWrite's system fallback, which may
	// already be YaHei. Equality with it is valid; the two explicit families
	// above must differ, so ignoring the fallback list still fails this test.
	ascii := []gfx.TextRun{{Bytes: 5, Cells: 5, Color: white}}
	if !bytes.Equal(draw(first, "abcde", ascii), draw(consolas, "abcde", ascii)) {
		t.Fatal("a fallback changed how the primary draws characters it has")
	}
}

func TestFallbackFamiliesNeverChangeTheCell(t *testing.T) {
	plainWidth, plainHeight, err := MeasureMono(13, gfx.MonoFamily("Consolas"))
	if err != nil {
		t.Skipf("no DirectWrite: %v", err)
	}
	width, height, err := MeasureMono(13, gfx.MonoStack(gfx.MonoFamily("Consolas"), "No Such Family Installed", "SimSun"))
	if err != nil || width != plainWidth || height != plainHeight {
		t.Fatalf("Consolas with fallbacks measures %v x %v (%v), want its own %v x %v", width, height, err, plainWidth, plainHeight)
	}
}
