//go:build dev || vivadev

package base

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
)

// A look on an element with no identity changes nothing in release builds; the
// dev build says so when the page is declared, since no state can ever report.
func TestAnElementWithoutIdentityIgnoresItsLooks(t *testing.T) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("a look on an element without an identity did not report in the dev build")
		}
		if message, _ := recovered.(string); !strings.Contains(message, "Hover/Pressed/Focus") || !strings.Contains(message, "Row") {
			t.Fatalf("unhelpful diagnostic: %v", recovered)
		}
	}()
	var view View
	view.Draw(styleCanvas(gfx.ID{}, gfx.ID{}, gfx.ID{}, false), gfx.LightTheme(), gfx.Rect{Width: 100, Height: 100}, func(f *Frame) {
		f.Row().Size(50, 50).Hover(Look{}.Background(colorHover))
	})
}
