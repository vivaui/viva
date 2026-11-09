//go:build darwin || windows || linux

package app

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func TestEditableComboboxUsesNativeQueryGeometry(t *testing.T) {
	var editor text.Editor
	id := gfx.Item(1)
	query := gfx.Rect{X: 120, Y: 10, Width: 150, Height: 30}
	driver, err := NewDriver(Config{Width: 300, Height: 100, Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
		canvas.HitWithOptions(id, query, "Tags", gfx.HitOptions{Role: gfx.RoleCombobox, Value: editor.String(), Expanded: true, TextBounds: query, TextSize: 14})
	}, TextInput: func(c TextInputContext) *TextInput {
		if c.ID != id || c.TextBounds != query {
			t.Fatal("combobox lost its query editor geometry", c)
		}
		return &TextInput{Editor: &editor}
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Click(140, 20); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("开发"); err != nil {
		t.Fatal(err)
	}
	if editor.String() != "开发" {
		t.Fatal("combobox did not receive native text", editor.String())
	}
}

// A combobox that paints its own query declares no text geometry: the native
// editor must not start over it and draw the query a second time.
func TestComboboxWithoutTextBoundsKeepsItsOwnQuery(t *testing.T) {
	var editor text.Editor
	id := gfx.Item(1)
	calls := 0
	driver, err := NewDriver(Config{Width: 300, Height: 100, Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
		canvas.HitWithOptions(id, gfx.Rect{X: 10, Y: 10, Width: 150, Height: 30}, "Search", gfx.HitOptions{Role: gfx.RoleCombobox, Value: editor.String()})
	}, TextInput: func(c TextInputContext) *TextInput {
		calls++
		return &TextInput{Editor: &editor}
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Click(20, 20); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("a combobox that draws its own query started the native editor %d times", calls)
	}
}
