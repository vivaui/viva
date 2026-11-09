package componentdemo

import (
	"testing"
	"unicode"

	"github.com/vivaui/viva/gfx"
)

// TestEnglishExamplesPaintNoChinese catches a demo that still hands Chinese
// copy to the canvas in English mode. It paints the initial state only; menus
// that open on a click are covered by the source translation and snippet tests.
func TestEnglishExamplesPaintNoChinese(t *testing.T) {
	for _, component := range Catalog() {
		for _, example := range component.Examples {
			state, err := NewEnglish(component.ID, example.ID)
			if err != nil {
				t.Fatalf("%s/%s: %v", component.ID, example.ID, err)
			}
			var canvas gfx.Canvas
			canvas.BeginFrame()
			canvas.Reset()
			state.Paint(&canvas, gfx.Size{Width: 760, Height: 600})
			canvas.EndFrame()
			for _, text := range canvas.Texts() {
				if containsHan(text.Value) {
					t.Errorf("%s/%s paints Chinese text in English mode: %q", component.ID, example.ID, text.Value)
				}
			}
			for _, hit := range canvas.Hits() {
				for _, value := range [...]string{hit.Label, hit.Hint, hit.Value} {
					if containsHan(value) {
						t.Errorf("%s/%s declares Chinese semantics in English mode: %q", component.ID, example.ID, value)
					}
				}
			}
		}
	}
}

func containsHan(value string) bool {
	for _, character := range value {
		if unicode.Is(unicode.Han, character) {
			return true
		}
	}
	return false
}
