//go:build darwin || linux || windows

package app

import (
	"testing"

	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

func TestDriverTextInputAlignmentAppearsInSnapshot(t *testing.T) {
	editor := text.NewEditor("centered")
	config := Config{Title: "aligned input", Width: 280, Height: 100}
	config.Paint = func(canvas *ui.Canvas, _ ui.Size) {
		canvas.HitWithOptions(ui.Item(5), ui.Rect{X: 20, Y: 20, Width: 220, Height: 40}, "Text", ui.HitOptions{
			Role: ui.RoleTextField, Value: editor.String(), TextBounds: ui.Rect{X: 24, Y: 23, Width: 210, Height: 34}, TextSize: 16,
		})
	}
	config.TextInput = func(TextInputContext) *TextInput {
		return &TextInput{Editor: editor, Alignment: ui.AlignCenter}
	}
	driver, err := NewDriver(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Click(30, 30); err != nil {
		t.Fatal(err)
	}
	if snapshot := driver.Snapshot(); snapshot.Editor == nil || snapshot.Editor.Alignment != ui.AlignCenter {
		t.Fatalf("driver editor alignment = %+v, want centered", snapshot.Editor)
	}
}
