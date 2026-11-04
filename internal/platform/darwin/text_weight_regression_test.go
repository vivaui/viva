//go:build darwin && vivahead

package darwin

import (
	"bytes"
	"testing"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

func TestWeightedNativeEditorMatchesStaticLabelPixels(t *testing.T) {
	const value = "墨栈 Heading"
	color := platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	render := func(editor bool) []byte {
		offscreen, err := NewOffscreen(280, 48, 2, Color{Alpha: 1})
		if err != nil {
			t.Skipf("no Metal device: %v", err)
		}
		defer offscreen.Close()
		if editor {
			err = offscreen.Set(nil, nil, &TextInput{
				X: 0, Y: 0, Width: 280, Height: 48, FontSize: 26, Weight: ui.WeightSemibold,
				Color: color, HideNativeOverlays: true, State: TextState{Value: value},
				OnEvent: func(TextEvent) (TextState, error) { return TextState{}, nil },
			})
		} else {
			err = offscreen.Set(nil, []TextLabel{{
				Value: value, X: 0, Y: 0, Width: 280, Height: 48,
				FontSize: 26, Weight: ui.WeightSemibold, Color: color,
			}}, nil)
		}
		if err != nil {
			t.Fatal(err)
		}
		pixels, _, _, err := offscreen.Render()
		if err != nil {
			t.Fatal(err)
		}
		return pixels
	}
	if label, editor := render(false), render(true); !bytes.Equal(label, editor) {
		t.Fatal("weighted native editor differs from the same static text")
	}
}
