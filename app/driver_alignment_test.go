//go:build darwin || linux || windows

package app

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func TestDriverTextInputAlignmentAppearsInSnapshot(t *testing.T) {
	editor := text.NewEditor("centered")
	config := Config{Title: "aligned input", Width: 280, Height: 100}
	config.Paint = func(canvas *gfx.Canvas, _ gfx.Size) {
		canvas.HitWithOptions(gfx.Item(5), gfx.Rect{X: 20, Y: 20, Width: 220, Height: 40}, "Text", gfx.HitOptions{
			Role: gfx.RoleTextField, Value: editor.String(), TextBounds: gfx.Rect{X: 24, Y: 23, Width: 210, Height: 34}, TextSize: 16,
		})
	}
	config.TextInput = func(TextInputContext) *TextInput {
		return &TextInput{Editor: editor, Alignment: gfx.AlignCenter}
	}
	driver, err := NewDriver(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Click(30, 30); err != nil {
		t.Fatal(err)
	}
	if snapshot := driver.Snapshot(); snapshot.Editor == nil || snapshot.Editor.Alignment != gfx.AlignCenter {
		t.Fatalf("driver editor alignment = %+v, want centered", snapshot.Editor)
	}
}
