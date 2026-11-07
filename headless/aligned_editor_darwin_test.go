//go:build darwin && vivahead

package headless

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

func TestHeadlessScreenshotPositionsNativeAlignedEditor(t *testing.T) {
	firstInk := func(alignment ui.Alignment) int {
		t.Helper()
		editor := text.NewEditor("abc")
		field := ui.Rect{X: 20, Y: 20, Width: 220, Height: 40}
		session, err := Open(app.Config{
			Title: "aligned editor", Width: 280, Height: 80,
			ClearColor: app.Color{Alpha: 1},
			Paint: func(canvas *ui.Canvas, _ ui.Size) {
				canvas.HitWithOptions(ui.Item(5), field, "Text", ui.HitOptions{
					Role: ui.RoleTextField, Value: editor.String(), TextBounds: field, TextSize: 18, TextColor: ui.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
				})
			},
			TextInput: func(app.TextInputContext) *app.TextInput {
				return &app.TextInput{Editor: editor, Alignment: alignment}
			},
		}, nil, Options{Scale: 1})
		if err != nil {
			t.Fatal(err)
		}
		defer session.Close()
		if err := session.Click(field.X+10, field.Y+10); err != nil {
			t.Fatal(err)
		}
		image, err := session.Image()
		if err != nil {
			t.Fatal(err)
		}
		for x := 20; x < 240; x++ {
			for y := 20; y < 60; y++ {
				red, green, blue, _ := image.At(x, y).RGBA()
				if red > 0x8000 && green > 0x8000 && blue > 0x8000 {
					return x
				}
			}
		}
		t.Fatal("aligned editor drew no glyphs")
		return 0
	}
	left, center, right := firstInk(ui.AlignLeft), firstInk(ui.AlignCenter), firstInk(ui.AlignRight)
	if center-left < 45 || right-center < 45 || right >= 240 {
		t.Fatalf("headless editor origins: left=%d center=%d right=%d", left, center, right)
	}
}
