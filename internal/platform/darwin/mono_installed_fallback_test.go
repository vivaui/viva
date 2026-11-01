//go:build darwin && vivahead

package darwin

import (
	"bytes"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

// The visual gate for the fallback macOS's own font machinery does not make: the
// last-resort box CoreText draws for a private-use code point (a Powerline separator, a
// Nerd Font icon) is replaced with the glyph an installed family has. The test carries
// its own family, because no macOS install draws those code points.

// testIconFamily is the family of testdata/VivaTestIcons-Regular.ttf: a fixed-pitch font
// that maps the whole private-use area to two glyphs (U+E0B0 is a triangle, U+E0B1 a
// wave), so it covers more private-use than any font a test machine is likely to have
// and the ranking picks it wherever the glyph comes from.
const testIconFamily = "Viva Test Icons"

// registerTestIcons makes the test's icon font available to this process until the test
// ends.
func registerTestIcons(t testing.TB) {
	t.Helper()
	unregister, err := RegisterFontFile(filepath.Join("testdata", "VivaTestIcons-Regular.ttf"))
	if err != nil {
		t.Fatalf("register the test font: %v", err)
	}
	t.Cleanup(unregister)
}

// renderMonoLine draws one fixed-pitch label and returns its pixels. With runs it takes
// the terminal row's path (a shaped run per cell span); without them the mono label's.
func renderMonoLine(t testing.TB, face ui.MonoFace, value string, runs []ui.TextRun) []byte {
	t.Helper()
	offscreen, err := NewOffscreen(160, 32, 2, platform.Color{Alpha: 1})
	if err != nil {
		t.Skipf("no Metal device: %v", err)
	}
	defer offscreen.Close()
	err = offscreen.Set(nil, []TextLabel{{
		Value: value, Width: 160, Height: 32, FontSize: 20, CellWidth: 12,
		Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
		Mono:  true, MonoFace: face, Runs: runs,
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

// TestAnInstalledFamilyDrawsWhatCoreTextOnlyBoxes: a private-use code point CoreText
// only boxes must draw the glyph an installed family has instead -- a different glyph for
// a different code point, the same way every time. The box of a code point no installed
// family has is the reference.
func TestAnInstalledFamilyDrawsWhatCoreTextOnlyBoxes(t *testing.T) {
	registerTestIcons(t)
	boxed := renderMonoLine(t, ui.MonoSystem, "\U0010FFFE", nil)
	first := renderMonoLine(t, ui.MonoSystem, "\ue0b0", nil)
	second := renderMonoLine(t, ui.MonoSystem, "\ue0b1", nil)
	if bytes.Equal(first, boxed) {
		t.Fatal("the default face boxed U+E0B0: the installed family that has it was not used")
	}
	if bytes.Equal(first, second) {
		t.Fatal("two code points with different glyphs drew alike: the fallback did not follow the family")
	}
	if again := renderMonoLine(t, ui.MonoSystem, "\ue0b0", nil); !bytes.Equal(first, again) {
		t.Fatal("the same face drew the same line two different ways")
	}
	// The application's own fallback family comes first: CoreText ignores a cascade list
	// for private-use code points too, so the lookup has to try it itself.
	if stacked := renderMonoLine(t, ui.MonoStack(ui.MonoSystem, testIconFamily), "\ue0b0", nil); !bytes.Equal(first, stacked) {
		t.Fatal("a face's own fallback family did not draw the code point it has")
	}
}

// TestAnInstalledFamilyDrawsAGridRun: a terminal row is runs on a cell grid, and a
// Powerline separator is a one-cell run that must come from the installed family too.
func TestAnInstalledFamilyDrawsAGridRun(t *testing.T) {
	registerTestIcons(t)
	white := ui.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	runs := func(value string) []ui.TextRun {
		bytes := int32(len(value))
		return []ui.TextRun{{Bytes: bytes, Cells: 1, Color: white}}
	}
	glyph := renderMonoLine(t, ui.MonoSystem, "\ue0b0", runs("\ue0b0"))
	if !bytes.Equal(glyph, renderMonoLine(t, ui.MonoStack(ui.MonoSystem, testIconFamily), "\ue0b0", runs("\ue0b0"))) {
		t.Fatal("a grid run did not draw its character from the installed family that has it")
	}
	if bytes.Equal(glyph, renderMonoLine(t, ui.MonoSystem, "\U0010FFFE", runs("\U0010FFFE"))) {
		t.Fatal("a grid run boxed a code point the installed family draws")
	}
}

// TestTheInstalledFallbackLeavesTheSystemChoiceAlone: the fallback takes only what CoreText
// boxes. Latin, Han, a symbol and an emoji already have a font, and they keep it whether or
// not a family that draws none of them sits behind the face.
func TestTheInstalledFallbackLeavesTheSystemChoiceAlone(t *testing.T) {
	registerTestIcons(t)
	value := "A\u4e2d\u276f\U0001F600"
	if !bytes.Equal(renderMonoLine(t, ui.MonoSystem, value, nil),
		renderMonoLine(t, ui.MonoStack(ui.MonoSystem, testIconFamily), value, nil)) {
		t.Fatal("a character the system draws changed because of the installed fallback")
	}
}

// BenchmarkRasterizeMonoLine is what a fixed-pitch line costs: one the face draws (shaped
// once) and one with a private-use cluster (the installed fallback and a second shaping,
// remembered after its first line). Both redraw every round, so the texture work is the
// same and only the shaping differs.
func BenchmarkRasterizeMonoLine(b *testing.B) {
	registerTestIcons(b)
	cases := []struct {
		name  string
		value string
	}{
		{"drawn", "abcdef"},
		{"installed-fallback", "abc\ue0b0def"},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			offscreen, err := NewOffscreen(160, 32, 2, platform.Color{Alpha: 1})
			if err != nil {
				b.Skipf("no Metal device: %v", err)
			}
			defer offscreen.Close()
			b.ReportAllocs()
			round := 0
			for b.Loop() {
				round++
				err := offscreen.Set(nil, []TextLabel{{
					Value: fmt.Sprintf("%s%d", c.value, round), Width: 160, Height: 32, FontSize: 20, CellWidth: 12,
					Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
					Mono:  true, MonoFace: ui.MonoSystem,
				}}, nil)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
