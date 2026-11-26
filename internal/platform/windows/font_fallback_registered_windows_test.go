package windows

import (
	"bytes"
	"runtime"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

// A font registered at run time is not in DirectWrite's system collection. A face
// that names it as a fallback therefore used to draw .notdef, as if the family
// were not installed, while the same family as the primary drew its glyphs. The
// fallback has to search the collection the text format is made from.
func TestRegisteredFontDrawsFallbackCharacters(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	engine, err := theTextEngine()
	if err != nil {
		t.Skipf("no DirectWrite: %v", err)
	}
	family, unregister, err := engine.RegisterFont(testIconsFont)
	if err != nil {
		t.Fatalf("register the test font: %v", err)
	}
	defer unregister()
	rasterizer, err := newDirectRasterizer()
	if err != nil {
		t.Skipf("no DirectWrite: %v", err)
	}
	defer rasterizer.close()

	white := gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	draw := func(face gfx.MonoFace, value string, cells int32) []byte {
		runs := []gfx.TextRun{{Bytes: int32(len(value)), Cells: cells, Color: white}}
		label := platform.TextLabel{Value: value, Width: 48, Height: 32, FontSize: 20, CellWidth: 12, Mono: true, MonoFace: face, Runs: runs}
		pixels, _, err := rasterizer.rasterize(label, 1, 48, 32, false, 0)
		if err != nil {
			t.Fatal(err)
		}
		return append([]byte(nil), pixels...)
	}
	base := gfx.MonoFamily("Consolas")
	// A code point in a private-use plane the test font does not map: every
	// family in the test draws it as the last-resort box.
	boxed := draw(base, "\U0010FFFE", 1)
	if bytes.Equal(draw(gfx.MonoFamily(family), testIcon, 1), boxed) {
		t.Fatal("the registered family as the primary drew the box")
	}
	stacked := draw(gfx.MonoStack(base, family), testIcon, 1)
	if bytes.Equal(stacked, boxed) {
		t.Fatal("the registered family as a fallback drew the box")
	}
	if bytes.Equal(stacked, draw(base, testIcon, 1)) {
		t.Fatal("the registered fallback changed no pixels")
	}
	// What the primary draws must not change because of the fallback.
	if !bytes.Equal(draw(gfx.MonoStack(base, family), "abc", 3), draw(base, "abc", 3)) {
		t.Fatal("the fallback changed how the primary draws the characters it has")
	}
}
