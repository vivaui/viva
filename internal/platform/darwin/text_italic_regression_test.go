//go:build darwin && vivahead

package darwin

import (
	"bytes"
	"testing"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

var _ ui.StyledTextEngine = (*TextEngine)(nil)

func TestItalicAndRegularLinesHaveSeparateCoreTextCacheEntries(t *testing.T) {
	engine := SharedTextEngine()
	engine.mutex.Lock()
	defer engine.mutex.Unlock()
	regular := engine.lineStyle("墨栈 Italic", 20, ui.WeightRegular, false)
	italic := engine.lineStyle("墨栈 Italic", 20, ui.WeightRegular, true)
	if regular == nil || italic == nil || regular == italic {
		t.Fatal("italic and regular shared a shaped CoreText line")
	}
}

func TestItalicProportionalLabelDrawsDifferentGlyphPixels(t *testing.T) {
	color := platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	render := func(italic bool) []byte {
		offscreen, err := NewOffscreen(280, 50, 2, Color{Alpha: 1})
		if err != nil {
			t.Skipf("no Metal device: %v", err)
		}
		defer offscreen.Close()
		err = offscreen.Set(nil, []TextLabel{{
			Value: "Italic 本文", X: 0, Y: 0, Width: 280, Height: 50,
			FontSize: 24, Weight: ui.WeightRegular, Italic: italic, Color: color,
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
	if bytes.Equal(render(false), render(true)) {
		t.Fatal("proportional italic label has regular glyph pixels")
	}
}
