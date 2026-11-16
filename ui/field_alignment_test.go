//go:build darwin || windows || linux

package ui

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

// TextEnd reaches the platform through the whole chain: the hit the field declares
// tells the focused editor its alignment, so a number field stays right-aligned
// while it is being edited in, and a field that did not ask stays start-aligned.
func TestFieldTextEndToldToThePlatformEditor(t *testing.T) {
	var amount, note Field
	var view View
	amountID, noteID := tree.ID(page, "amount"), tree.ID(page, "note")
	amount.SetText("42")
	config := app.Config{
		Tree: &tree, Title: "aligned", Width: 300, Height: 200,
		Paint: func(canvas *gfx.Canvas, size gfx.Size) {
			view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: size.Width, Height: size.Height}, func(f *Frame) {
				f.Column().Fill().Pad(12).Gap(8).Children(func() {
					f.Field(&amount, amountID, "Amount", "").TextEnd()
					f.Field(&note, noteID, "Note", "")
				})
			})
		},
		TextInput: Fields(&amount, &note),
	}
	var canvas gfx.Canvas
	driver, err := app.NewDriver(config, &canvas)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	click := func(id gfx.ID) {
		for _, hit := range driver.Snapshot().Hits {
			if hit.ID == id {
				if err := driver.Click(hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2); err != nil {
					t.Fatal(err)
				}
				return
			}
		}
		t.Fatalf("no field %v on screen", id)
	}
	editorAlignment := func() gfx.Alignment {
		snapshot := driver.Snapshot()
		if snapshot.Editor == nil {
			t.Fatal("no focused editor on screen")
		}
		return snapshot.Editor.Alignment
	}
	click(amountID)
	if got := editorAlignment(); got != gfx.AlignRight {
		t.Errorf("the right-aligned field's editor = %v, want AlignRight (resolved from AlignEnd)", got)
	}
	// The line the editor draws sits at the same right edge the paint put it at, and
	// the field's own painted line is right-aligned too.
	snapshot := driver.Snapshot()
	var line gfx.Text
	for _, text := range snapshot.Texts {
		if text.Value == "42" && text.Bounds.Width == snapshot.Editor.Bounds.Width {
			line = text
			break
		}
	}
	if line.Value == "" {
		t.Fatalf("the value was not painted: %+v", snapshot.Texts)
	}
	if line.Bounds.X < snapshot.Editor.Bounds.X+snapshot.Editor.Bounds.Width/2 {
		t.Errorf("the painted value starts at %v, left of center: it was not right-aligned", line.Bounds.X)
	}
	click(noteID)
	if got := editorAlignment(); got != gfx.AlignLeft {
		t.Errorf("an ordinary field's editor = %v, want AlignLeft (start for LTR)", got)
	}
}
