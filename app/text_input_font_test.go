//go:build darwin || windows || linux

package app

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// Every platform refuses a text input without a font size, so the runtime never
// gives one: what neither the application nor the hit's declaration sets is the
// body text's. (Linux failed the window over it; the driver did not notice.)
func TestTextInputWithoutAFontSizeGetsTheBodySize(t *testing.T) {
	field := gfx.Item(3)
	typed := ""
	config := Config{Width: 300, Height: 200}
	config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
		canvas.HitWithOptions(field, gfx.Rect{X: 10, Y: 10, Width: 200, Height: 30}, "Console", gfx.HitOptions{Role: gfx.RoleTerminal})
	}
	input := TextInput{OnEvent: func(event TextEvent) (TextState, error) {
		typed += event.Text
		return TextState{}, nil
	}}
	config.TextInput = func(TextInputContext) *TextInput { return &input }
	driver, err := NewDriver(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Click(100, 25); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("x"); err != nil || typed != "x" {
		t.Fatalf("err %v, typed %q", err, typed)
	}
	if size := driver.painter.nativeText.FontSize; size != gfx.DefaultMetrics().FontSizeM {
		t.Fatalf("the platform is given font size %v", size)
	}
}
