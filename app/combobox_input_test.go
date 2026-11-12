//go:build darwin || windows || linux

package app

import (
	"testing"

	"github.com/vivaui/viva/components"
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

func TestComboboxNativeQueryAndCompositionHaveOnePainter(t *testing.T) {
	for _, query := range []string{"host-with-a-long-address.example", "שלום עולם בדיקת ממשק בחירת שרת"} {
		t.Run(query, func(t *testing.T) {
			var editor text.Editor
			inputID := gfx.Item(1)
			theme := gfx.DarkTheme()
			driver, err := NewDriver(Config{
				Width: 320, Height: 180,
				Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
					components.BeginCombobox(canvas, theme, gfx.Rect{X: 20, Y: 20, Width: 240}, components.ComboboxOptions{
						InputID: inputID, Label: "Host", Value: "server", Detail: "192.0.2.1", Query: editor.String(),
					}).End()
				},
				TextInput: func(context TextInputContext) *TextInput {
					if context.ID != inputID {
						return nil
					}
					return &TextInput{Editor: &editor}
				},
			}, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer driver.Close()
			if err := driver.Click(30, 30); err != nil {
				t.Fatal(err)
			}
			if err := driver.Type(query); err != nil {
				t.Fatal(err)
			}
			assertNativeQuery := func(composing bool) {
				t.Helper()
				snapshot := driver.Snapshot()
				if snapshot.Editor == nil || snapshot.Editor.Value != editor.String() || snapshot.Editor.Composing != composing {
					t.Fatalf("native query state lost: %+v", snapshot.Editor)
				}
				if len(snapshot.Texts) != 0 {
					t.Fatalf("component paints over native query: %+v", snapshot.Texts)
				}
			}
			assertNativeQuery(false)
			if err := driver.SelectText(0, 0); err != nil {
				t.Fatal(err)
			}
			if err := driver.Compose("开发"); err != nil {
				t.Fatal(err)
			}
			assertNativeQuery(true)
		})
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
