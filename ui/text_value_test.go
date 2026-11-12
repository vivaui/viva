package ui_test

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

// A TextField bound to a string writes what is typed into it, shows what the
// program puts in it, and reports the user's edit, not the program's, as Changed.
func TestTextFieldBindsAString(t *testing.T) {
	name, changes := "Ada", 0
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Gap(8).Children(func() {
			if ui.TextField(c, &name, "Name", "").Width(240).Changed() {
				changes++
			}
		})
	})
	if d.hit("Name").Value != "Ada" {
		t.Fatalf("the field shows %q, not the string", d.hit("Name").Value)
	}
	d.click("Name")
	d.typeText("!")
	if name != "Ada!" || changes != 1 {
		t.Fatalf("name %q, changes %d", name, changes)
	}
	name = "Grace"
	d.repaint()
	if d.hit("Name").Value != "Grace" || changes != 1 {
		t.Fatalf("the program's value: shown %q, changes %d", d.hit("Name").Value, changes)
	}
	d.typeText("?") // the caret went to the end of the program's text
	if name != "Grace?" {
		t.Fatalf("typed after the program's change: %q", name)
	}
}

// A field bound to a string keeps its editor while a Memo copies it.
func TestTextFieldStringInAMemoKeepsItsEditor(t *testing.T) {
	name, version := "", uint64(1)
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Gap(8).Children(func() {
			ui.Button(c, "Other")
			ui.Memo(c, version, func() { ui.TextField(c, &name, "Name", "").Width(240) })
		})
	})
	d.click("Name")
	d.typeText("ab")
	d.click("Other") // the field is no longer focused: the Memo copies it
	d.repaint()
	d.repaint()
	d.click("Name")
	d.typeText("c")
	if name != "abc" {
		t.Fatalf("the copied field lost its editor: %q", name)
	}
}

// Secure masks a field bound to a string as Field.Secure does.
func TestTextFieldSecure(t *testing.T) {
	password := "hunter2"
	d := openPage(t, func(c ui.C) {
		ui.TextField(c, &password, "Password", "").Width(240).Secure()
	})
	hit := d.hit("Password")
	if !hit.Secure || hit.Value != "" {
		t.Fatalf("secure=%v value=%q", hit.Secure, hit.Value)
	}
}

func BenchmarkTextFieldString(b *testing.B) {
	name := "Ada Lovelace"
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	view := func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() { ui.TextField(c, &name, "Name", "").Width(240) })
	}
	b.ReportAllocs()
	for b.Loop() {
		drawPage(page, canvas, view)
	}
}
