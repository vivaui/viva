//go:build darwin && vivahead && !ios

package darwin

import (
	"math"
	"testing"

	"github.com/vivaui/viva/internal/platform"
)

// Digits do not change when another script follows them. Laid out with a
// reference table and squeezed to fit it, "2213123" before "中文" was drawn about 5 percent
// narrower than "2213123" alone. Now the line is shaped once, so the digits are the same
// pixels whatever comes after them.
func TestDigitsKeepTheirGlyphsWhenCJKFollows(t *testing.T) {
	engine := SharedTextEngine()
	const size = 14.0
	const scale = 2
	white := platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}

	render := func(value string, editor bool) ([]byte, int, int) {
		offscreen, err := NewOffscreen(160, 36, scale, Color{Alpha: 1})
		if err != nil {
			t.Skipf("no Metal device: %v", err)
		}
		defer offscreen.Close()
		if editor {
			err = offscreen.Set(nil, nil, &TextInput{
				X: 0, Y: 0, Width: 160, Height: 36, FontSize: size, Color: white,
				State:   TextState{Value: value},
				OnEvent: func(TextEvent) (TextState, error) { return TextState{}, nil },
			})
		} else {
			err = offscreen.Set(nil, []TextLabel{{Value: value, Width: 160, Height: 36, FontSize: size, Color: white}}, nil)
		}
		if err != nil {
			t.Fatal(err)
		}
		pixels, w, h, err := offscreen.Render()
		if err != nil {
			t.Fatal(err)
		}
		return pixels, w, h
	}

	digitsWidth := engine.Width("2213123", size, 0)
	for _, editor := range []bool{false, true} {
		alone, w, h := render("2213123", editor)
		followed, _, _ := render("2213123中文", editor)
		// The editor's caret sits in the first device columns; the digits end before the
		// next script starts, less a pixel for the neighbour's side bearing.
		from, to := 0, int(digitsWidth*scale)-2
		if editor {
			from = 6
		}
		worst := 0
		for y := 0; y < h; y++ {
			for x := from; x < to; x++ {
				_, a, _, _ := pixelAt(alone, w, x, y)
				_, b, _, _ := pixelAt(followed, w, x, y)
				worst = max(worst, int(math.Abs(float64(int(a)-int(b)))))
			}
		}
		if worst > 3 {
			t.Errorf("editor=%v: the digits differ by up to %d/255 when 中文 follows them", editor, worst)
		}
	}
}
