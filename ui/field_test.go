//go:build darwin || windows || linux

package ui

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

// A Field works end to end: a click focuses it, typed text reaches its editor, a
// secure one stays in the editor but not on the screen, and none of it needs wiring
// beyond Fields.
func TestFieldsTakeTypedTextThroughTheDriver(t *testing.T) {
	var name, password Field
	password.Secure = true
	var view View
	nameID, passwordID := tree.ID(page, "name"), tree.ID(page, "password")
	config := app.Config{
		Tree: &tree, Title: "form", Width: 300, Height: 200,
		Paint: func(canvas *gfx.Canvas, size gfx.Size) {
			view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: size.Width, Height: size.Height}, func(f *Frame) {
				f.Column().Fill().Pad(12).Gap(8).Children(func() {
					f.Field(&name, nameID, "Name", "Your name")
					f.Field(&password, passwordID, "Password", "")
				})
			})
		},
		TextInput: Fields(&name, &password),
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
	click(nameID)
	if err := driver.Type("Ada"); err != nil {
		t.Fatal(err)
	}
	click(passwordID)
	if err := driver.Type("s3cret"); err != nil {
		t.Fatal(err)
	}
	if name.Text() != "Ada" || password.Text() != "s3cret" {
		t.Fatalf("typed %q and %q, want Ada and s3cret", name.Text(), password.Text())
	}
	for _, label := range driver.Snapshot().Labels {
		if label.Value == "s3cret" {
			t.Error("the secure field's text is on the screen")
		}
	}
	name.SetText("Grace")
	if name.Text() != "Grace" {
		t.Errorf("SetText gave %q", name.Text())
	}
}

// Enter pressed in a field reports Submitted once, in the paint that delivers it; a
// click on the field, typing, and Enter in a field that is not focused do not.
func TestEnterInAFieldIsSubmittedOnceAndAClickIsNot(t *testing.T) {
	var draft, other Field
	var view View
	draftID, otherID := tree.ID(page, "draft"), tree.ID(page, "other")
	var sent []string
	config := app.Config{
		Tree: &tree, Title: "chat", Width: 300, Height: 200,
		Paint: func(canvas *gfx.Canvas, size gfx.Size) {
			view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: size.Width, Height: size.Height}, func(f *Frame) {
				f.Column().Fill().Pad(12).Gap(8).Children(func() {
					if f.Field(&draft, draftID, "Message", "Type").Submitted() {
						sent = append(sent, draft.Text())
						draft.SetText("")
					}
					f.Field(&other, otherID, "Other", "")
				})
			})
		},
		TextInput: Fields(&draft, &other),
	}
	var canvas gfx.Canvas
	driver, err := app.NewDriver(config, &canvas)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	click := func(id gfx.ID) {
		t.Helper()
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
	click(draftID)
	if len(sent) != 0 {
		t.Fatalf("a click submitted %q", sent)
	}
	if err := driver.Type("hello"); err != nil {
		t.Fatal(err)
	}
	click(draftID) // clicking the focused field again is still not Enter
	if len(sent) != 0 {
		t.Fatalf("typing or a second click submitted %q", sent)
	}
	if err := driver.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if len(sent) != 1 || sent[0] != "hello" || draft.Text() != "" {
		t.Fatalf("Enter sent %q and left %q, want [hello] and an empty field", sent, draft.Text())
	}
	if err := driver.Type("again"); err != nil {
		t.Fatal(err)
	}
	if len(sent) != 1 {
		t.Fatalf("typing after Enter sent %q", sent)
	}
	click(otherID)
	if err := driver.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if len(sent) != 1 {
		t.Fatalf("Enter in the other field submitted the first: %q", sent)
	}
}

func TestSubmittedAllocatesNothing(t *testing.T) {
	var field Field
	var view View
	canvas := testCanvas()
	id := tree.ID(page, "draft")
	draw := func() {
		view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 300, Height: 100}, func(f *Frame) {
			f.Column().Pad(8).Children(func() {
				if f.Field(&field, id, "Message", "").Submitted() {
					t.Error("submitted without a key")
				}
			})
		})
	}
	draw()
	if got := testing.AllocsPerRun(50, draw); got != 0 {
		t.Errorf("a page with a field and Submitted allocates %v per frame, want 0", got)
	}
}
