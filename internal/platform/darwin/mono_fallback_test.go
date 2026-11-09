//go:build darwin && vivahead

package darwin

import (
	"bytes"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

// TestAFallbackFamilyDrawsTheCharactersThePrimaryLacks is the visual gate: Menlo has no
// CJK glyphs, so the same line drawn with two different CJK fallbacks must differ, and
// the same line with the same fallback must not. Characters Menlo has must not move.
func TestAFallbackFamilyDrawsTheCharactersThePrimaryLacks(t *testing.T) {
	menlo := gfx.MonoFamily("Menlo")
	render := func(face gfx.MonoFace, value string, runs []gfx.TextRun) []byte {
		offscreen, err := NewOffscreen(160, 32, 2, Color{Alpha: 1})
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
	white := gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	ascii := []gfx.TextRun{{Bytes: 5, Cells: 5, Color: white}}

	songti := gfx.MonoStack(menlo, "Songti SC")
	heiti := gfx.MonoStack(menlo, "STHeiti")
	hello := "你好"
	runs := []gfx.TextRun{{Bytes: 3, Cells: 2, Color: white}, {Column: 2, Bytes: 3, Cells: 2, Color: white}}
	withSongti, withSongtiAgain, withHeiti, plain := render(songti, hello, runs), render(songti, hello, runs), render(heiti, hello, runs), render(menlo, hello, runs)
	if !bytes.Equal(withSongti, withSongtiAgain) {
		t.Fatal("the same face rendered two different images")
	}
	if bytes.Equal(withSongti, withHeiti) {
		t.Fatal("two different fallback families drew the same pixels: the fallback list was not used")
	}
	if bytes.Equal(withSongti, plain) {
		t.Fatal("a stack with a fallback drew like its bare primary")
	}
	if !bytes.Equal(render(songti, "abcde", ascii), render(menlo, "abcde", ascii)) {
		t.Fatal("a fallback changed how the primary draws characters it has")
	}
}
