package ui_test

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

func (d *pageDriver) secondaryClick(Label string) {
	d.t.Helper()
	x, y := center(d.hit(Label).Bounds)
	if err := d.driver.Press(x, y, gfx.PointerButtonSecondary); err != nil {
		d.t.Fatal(err)
	}
	if err := d.driver.Release(x, y, gfx.PointerButtonSecondary); err != nil {
		d.t.Fatal(err)
	}
}

// menuItem finds a row of the open menu by its name.
func (d *pageDriver) menuItem(Label string) (gfx.Hit, bool) {
	for _, hit := range d.driver.Snapshot().Hits {
		if hit.Role == gfx.RoleMenuItem && hit.Label == Label {
			return hit, true
		}
	}
	return gfx.Hit{}, false
}

// openCanvasMenus is openPage for a page that keeps its text menus its own, drawn
// in the window: the platform's is shown otherwise.
func openCanvasMenus(t *testing.T, View func(ui.C)) *pageDriver {
	t.Helper()
	return openPageWith(t, ui.PageConfig{View: View, CanvasMenus: true}, app.Config{})
}

// A TextField opens the standard edit menu on a secondary press: it takes the
// keyboard, Copy and Cut act on the selection, Paste folds the clipboard into one
// line at the caret, and Select All selects the value.
func TestTextFieldEditMenu(t *testing.T) {
	var name, other ui.Field
	editor := text.NewEditor("hello world")
	name.Bind(editor)
	d := openCanvasMenus(t, func(c ui.C) {
		ui.Column(c).Pad(16).Gap(8).Children(func() {
			ui.TextField(c, &name, "Name", "")
			ui.TextField(c, &other, "Other", "")
		})
	})
	d.click("Other")
	d.secondaryClick("Name")
	for _, label := range []string{"Cut", "Copy", "Paste", "Select All"} {
		if _, ok := d.menuItem(label); !ok {
			t.Fatalf("the edit menu has no %s", label)
		}
	}
	if hit, _ := d.menuItem("Copy"); !hit.Disabled {
		t.Fatal("Copy is enabled with nothing selected")
	}
	d.click("Select All")
	if sel := editor.Selection(); sel != (text.Range{Start: 0, End: 11}) {
		t.Fatalf("selection after Select All = %v", sel)
	}
	if got := d.Focused(); got != "Name" {
		t.Fatalf("focus after the menu closed = %q, want the field", got)
	}

	d.secondaryClick("Name")
	d.click("Copy")
	if got := d.driver.Clipboard(); got != "hello world" {
		t.Fatalf("clipboard after Copy = %q", got)
	}
	d.secondaryClick("Name")
	d.click("Cut")
	if name.Text() != "" || d.driver.Clipboard() != "hello world" {
		t.Fatalf("after Cut: text %q, clipboard %q", name.Text(), d.driver.Clipboard())
	}

	d.driver.SetClipboard("a\r\nb\tc")
	d.secondaryClick("Name")
	d.click("Paste")
	if got := name.Text(); got != "a b c" {
		t.Fatalf("Paste into a single line = %q", got)
	}
	if !editor.CanUndo() {
		t.Fatal("Paste is not an edit the field can undo")
	}
}

// A TextArea keeps the lines it pastes.
func TestTextAreaEditMenuKeepsLines(t *testing.T) {
	var notes ui.Field
	d := openCanvasMenus(t, func(c ui.C) {
		ui.TextArea(c, &notes, "Notes", "").Height(120)
	})
	d.driver.SetClipboard("one\ntwo")
	d.secondaryClick("Notes")
	d.click("Paste")
	if got := notes.Text(); got != "one\ntwo" {
		t.Fatalf("Paste into a text area = %q", got)
	}
}

// A secret field offers no way out for its text, and a read-only one none in.
func TestEditMenuHonoursSecretAndReadOnly(t *testing.T) {
	var password, token ui.Field
	password.Secure = true
	password.SetText("hunter2")
	token.SetText("abc")
	d := openCanvasMenus(t, func(c ui.C) {
		ui.Column(c).Pad(16).Gap(8).Children(func() {
			ui.TextField(c, &password, "Password", "")
			ui.TextField(c, &token, "Token", "").ReadOnly(true)
		})
	})
	d.secondaryClick("Password")
	for _, label := range []string{"Cut", "Copy"} {
		if _, ok := d.menuItem(label); ok {
			t.Fatalf("a secret field offers %s", label)
		}
	}
	if _, ok := d.menuItem("Paste"); !ok {
		t.Fatal("a secret field cannot be pasted into")
	}
	d.key(gfx.KeyEscape)

	d.driver.SetClipboard("zzz")
	d.secondaryClick("Token")
	for _, label := range []string{"Cut", "Paste"} {
		if hit, ok := d.menuItem(label); !ok || !hit.Disabled {
			t.Fatalf("%s of a read-only field = %+v, %v; want disabled", label, hit, ok)
		}
	}
	if hit, _ := d.menuItem("Select All"); hit.Disabled {
		t.Fatal("Select All of a read-only field is disabled")
	}
	d.click("Paste")
	if token.Text() != "abc" {
		t.Fatalf("a read-only field took a paste: %q", token.Text())
	}
}

// A page names the words in its own language, and a field with a menu of its
// own opts out.
func TestEditMenuLabelsAndOptOut(t *testing.T) {
	var plain, own ui.Field
	d := openPageWith(t, ui.PageConfig{
		EditMenu: &ui.EditMenuLabels{Paste: "粘贴"}, CanvasMenus: true,
		View: func(c ui.C) {
			ui.Column(c).Pad(16).Gap(8).Children(func() {
				ui.TextField(c, &plain, "Plain", "")
				box := ui.TextField(c, &own, "Own", "", ui.WithoutEditMenu())
				ui.ContextMenu(c, box, func(menu *ui.MenuItems) { menu.Item("Mine") })
			})
		},
	}, app.Config{})
	d.secondaryClick("Plain")
	if _, ok := d.menuItem("粘贴"); !ok {
		t.Fatal("the page's Paste label is not used")
	}
	if _, ok := d.menuItem("Cut"); !ok {
		t.Fatal("a label the page leaves empty is not English")
	}
	d.key(gfx.KeyEscape)
	d.secondaryClick("Own")
	if _, ok := d.menuItem("Mine"); !ok {
		t.Fatal("the field's own menu is not shown")
	}
	if _, ok := d.menuItem("Cut"); ok {
		t.Fatal("an opted-out field has the edit menu")
	}
}

// A field in a Dialog has its menu over the dialog, where it can be reached.
func TestEditMenuOverADialog(t *testing.T) {
	var name ui.Field
	d := openCanvasMenus(t, func(c ui.C) {
		dialog := ui.Dialog(c, ui.DialogSpec{Title: "Host", Scope: gfx.Item(900), DismissID: gfx.Item(901), CloseID: gfx.Item(902)})
		dialog.Body(func() { ui.TextField(c, &name, "Name", "") })
	})
	d.driver.SetClipboard("db1")
	d.secondaryClick("Name")
	d.click("Paste")
	if name.Text() != "db1" {
		t.Fatalf("Paste in a dialog's field = %q", name.Text())
	}
	if d.Focused() != "Name" {
		t.Fatalf("the keyboard is on %q after the menu closed", d.Focused())
	}
}

// denseFields is a form of forty text fields, with or without the edit menu.
type denseFields struct {
	fields [40]ui.Field
	plain  bool
}

func (m *denseFields) page(c ui.C) {
	ui.Column(c).Fill().Pad(12).Gap(6).Children(func() {
		for i := range m.fields {
			if m.plain {
				ui.TextField(c, &m.fields[i], "Field", "", ui.WithID(gfx.Item(100+i)), ui.WithoutEditMenu())
			} else {
				ui.TextField(c, &m.fields[i], "Field", "", ui.WithID(gfx.Item(100+i)))
			}
		}
	})
}

func BenchmarkPageTextFieldsEditMenu(b *testing.B) {
	for _, plain := range []bool{false, true} {
		name := "menu"
		if plain {
			name = "none"
		}
		b.Run(name, func(b *testing.B) {
			m := &denseFields{plain: plain}
			page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
			b.ReportAllocs()
			for b.Loop() {
				drawPage(page, canvas, m.page)
			}
		})
	}
}

// The keyboard goes to the field the menu was opened on, also in a Dialog that
// had it on another control.
func TestEditMenuTakesTheKeyboardInADialog(t *testing.T) {
	var name ui.Field
	name.SetText("abc")
	d := openCanvasMenus(t, func(c ui.C) {
		dialog := ui.Dialog(c, ui.DialogSpec{Title: "Host", Scope: gfx.Item(900), DismissID: gfx.Item(901), CloseID: gfx.Item(902)})
		dialog.Body(func() {
			ui.Button(c, "First")
			ui.TextField(c, &name, "Name", "")
		})
	})
	if d.Focused() == "Name" {
		t.Skip("the dialog starts on the field")
	}
	d.secondaryClick("Name")
	d.click("Select All")
	if d.Focused() != "Name" {
		t.Fatalf("the keyboard is on %q after the menu closed, want the field", d.Focused())
	}
}

// By default a text field's menu is the system's, not the page's: the window is
// asked for it with the page's words, a field with a ContextMenu of its own keeps
// that menu, and the choice acts on the field.
func TestTextFieldMenuIsTheSystemsByDefault(t *testing.T) {
	var name, own ui.Field
	words := &ui.EditMenuLabels{Paste: "粘贴"}
	d := openPageWith(t, ui.PageConfig{EditMenu: words, View: func(c ui.C) {
		ui.Column(c).Pad(16).Gap(8).Children(func() {
			ui.TextField(c, &name, "Name", "")
			box := ui.TextField(c, &own, "Own", "", ui.WithoutEditMenu())
			ui.ContextMenu(c, box, func(menu *ui.MenuItems) { menu.Item("Mine") })
		})
	}}, app.Config{})
	d.secondaryClick("Name")
	items := d.driver.TextMenu()
	if len(items) == 0 {
		t.Fatal("a secondary press opened no system menu")
	}
	for _, item := range items {
		if item.Command == app.TextMenuPaste && item.Title != "粘贴" {
			t.Fatalf("Paste is %q, want the page's words", item.Title)
		}
	}
	if d.has("Cut") {
		t.Fatal("the page drew a menu of its own beside the system's")
	}
	words.Paste = "貼上"
	d.driver.DismissTextMenu()
	d.secondaryClick("Name")
	for _, item := range d.driver.TextMenu() {
		if item.Command == app.TextMenuPaste && item.Title != "貼上" {
			t.Fatalf("Paste is %q after the page changed its words", item.Title)
		}
	}
	d.driver.SetClipboard("db1")
	if err := d.driver.ChooseTextMenu(app.TextMenuPaste); err != nil {
		t.Fatal(err)
	}
	if name.Text() != "db1" || d.Focused() != "Name" {
		t.Fatalf("after Paste: text %q, keyboard on %q", name.Text(), d.Focused())
	}
	d.secondaryClick("Own")
	if d.driver.TextMenu() != nil || !d.has("Mine") {
		t.Fatalf("a field with its own menu: system menu %v, own menu shown %v", d.driver.TextMenu() != nil, d.has("Mine"))
	}
}

// A read-only field has no editor for the system's menu to act on, so it keeps
// the page's, with the copy it still offers.
func TestReadOnlyFieldKeepsThePagesMenuBesideTheSystems(t *testing.T) {
	var token ui.Field
	token.SetText("abc")
	d := openPage(t, func(c ui.C) { ui.TextField(c, &token, "Token", "").ReadOnly(true) })
	d.secondaryClick("Token")
	if d.driver.TextMenu() != nil {
		t.Fatal("the system's menu opened for a field it has no editor for")
	}
	if _, ok := d.menuItem("Copy"); !ok {
		t.Fatal("a read-only field has no menu to copy from")
	}
}
