//go:build darwin || windows || linux

package app

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
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

// movingFontEngine is a text engine whose font generation a test moves, as a change
// of the interface font or of the text's weight is.
type movingFontEngine struct {
	gfx.TextEngine
	generation uint32
}

func (e *movingFontEngine) SetFontFamily(string)   {}
func (e *movingFontEngine) FontGeneration() uint32 { return e.generation }

// The platform keeps a picture of the editor, so a field that is already on screen has
// to be handed over again when the fonts change under it; otherwise it keeps drawing
// the weight it was made with until the text or the focus changes.
func TestFontChangeRedrawsTheTextInput(t *testing.T) {
	engine := &movingFontEngine{TextEngine: gfx.ReferenceTextEngine(), generation: 1}
	var canvas gfx.Canvas
	canvas.SetTextEngine(engine)
	input := &TextInput{Editor: text.NewEditor("same"), Bounds: gfx.Rect{Width: 100, Height: 24}, FontSize: 13}
	painter := newTestPainter(t, &canvas, Config{
		Title: "fonts", Width: 200, Height: 100,
		Paint: func(c *gfx.Canvas, _ gfx.Size) {
			c.HitWithOptions(gfx.Item(1), gfx.Rect{Width: 100, Height: 24}, "field", gfx.HitOptions{Role: gfx.RoleTextField})
		},
		TextInput: func(TextInputContext) *TextInput { return input },
	})
	presented := false
	for _, kind := range []platform.PointerEventKind{platform.PointerDown, platform.PointerUp} {
		frame, err := painter.pointerEvent(platform.PointerEvent{Kind: kind, X: 10, Y: 10})
		if err != nil {
			t.Fatal(err)
		}
		presented = presented || frame.TextInput.Changed && frame.TextInput.Input != nil
	}
	if !presented {
		t.Fatal("the focused field was not sent to the platform")
	}
	if update := painter.syncTextInput(); update.Changed {
		t.Fatal("nothing changed, yet the field was sent again")
	}
	engine.generation++
	if update := painter.syncTextInput(); !update.Changed || update.Input == nil {
		t.Fatalf("the fonts changed, yet the field was not drawn again: %+v", update)
	}
}
