package ui_test

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

// A Secure TextArea is a password field of several rows. The hit is a password's;
// a press, the arrow keys and typing reach the same characters of the secret as
// they do while the text shows, though only bullets are seen. Secure off, it is
// the text's field again.
func TestPageTextAreaSecure(t *testing.T) {
	const secret = "alpha\nbravo"
	type reached struct {
		left, right, down, end, up int
		typed, value               string
	}
	var results [2]reached
	for index, secure := range []bool{true, false} {
		editor := text.NewEditor(secret)
		var key ui.Field
		key.Bind(editor)
		key.Secure = secure
		d := openPage(t, func(c ui.C) {
			ui.Column(c).Pad(16).Children(func() {
				ui.TextArea(c, &key, "Key", "").Width(240).Height(104)
			})
		})
		hit := d.hit("Key")
		if hit.Secure != secure {
			t.Fatalf("secure=%v: the hit says secure=%v", secure, hit.Secure)
		}
		got := &results[index]
		got.value = hit.Value
		y := hit.Bounds.Y + hit.Bounds.Height/2
		for _, edge := range []struct {
			name string
			x    float32
			to   *int
		}{{"left", hit.Bounds.X + 1, &got.left}, {"right", hit.Bounds.X + hit.Bounds.Width - 1, &got.right}} {
			if err := d.driver.Click(edge.x, y); err != nil {
				t.Fatal(err)
			}
			*edge.to = editor.Caret()
		}
		editor.MoveTo(0, false)
		d.repaint()
		d.key(gfx.KeyDown)
		got.down = editor.Caret()
		d.key(gfx.KeyEnd)
		got.end = editor.Caret()
		d.key(gfx.KeyUp)
		got.up = editor.Caret()
		d.typeText("x")
		got.typed = editor.String()
	}
	hidden, shown := results[0], results[1]
	// The field that shows its text is what the caret indexes mean.
	if shown.value != secret || shown.left >= shown.right || shown.down <= 0 || shown.end < shown.down || shown.up > shown.down || shown.typed == secret {
		t.Fatalf("the field of the text itself: %+v", shown)
	}
	if hidden.value != "" {
		t.Fatalf("the hit of a secure textarea carries %q", hidden.value)
	}
	hidden.value, shown.value = "", ""
	if hidden != shown {
		t.Fatalf("the secure textarea reached %+v where the field of its text reaches %+v", hidden, shown)
	}
}

// A secret offers no way out through the edit menu, from a TextArea as from a
// TextField, whether Secure is the field's or the option chained on the box;
// revealed, the menu is the text's again.
func TestEditMenuOfASecretTextArea(t *testing.T) {
	var key ui.Field
	key.Bind(text.NewEditor("-----BEGIN-----\nsecret\n"))
	key.Secure = true
	password := "hunter2"
	pasted := "-----BEGIN-----\npasted secret\n"
	d := openCanvasMenus(t, func(c ui.C) {
		ui.Column(c).Pad(16).Gap(8).Children(func() {
			ui.TextField(c, &password, "Password", "").Secure()
			ui.TextArea(c, &key, "Key", "").Height(104)
			ui.TextArea(c, &pasted, "Pasted", "").Height(104).Secure()
		})
	})
	for _, label := range []string{"Password", "Key", "Pasted"} {
		d.secondaryClick(label)
		d.click("Select All")
		d.secondaryClick(label)
		for _, item := range []string{"Cut", "Copy"} {
			if _, ok := d.menuItem(item); ok {
				t.Fatalf("the menu of the secure %s offers %s", label, item)
			}
		}
	}
	if got := d.driver.Clipboard(); got != "" {
		t.Fatalf("the clipboard holds %q after the menus of secrets", got)
	}
	key.Secure = false
	d.repaint()
	d.secondaryClick("Key")
	for _, item := range []string{"Cut", "Copy"} {
		if _, ok := d.menuItem(item); !ok {
			t.Fatalf("the menu of the revealed textarea has no %s", item)
		}
	}
	d.click("Select All")
	d.secondaryClick("Key")
	d.click("Copy")
	if got := d.driver.Clipboard(); !strings.Contains(got, "secret") {
		t.Fatalf("Copy of the revealed textarea put %q on the clipboard", got)
	}
}

// A page paints a secret the size of a key without allocating once its bullets
// are made, as it paints a text.
func TestPageTextAreaSecureSteadyPaintAllocatesNothing(t *testing.T) {
	var key ui.Field
	key.Secure = true
	editor := text.NewEditor(strings.Repeat("bG9uZyBsaW5lIG9mIGEgc2VjcmV0IHRoYXQgd3JhcHMgaW4gdGhlIGJveA\n", 70))
	editor.MoveTo(editor.RuneCount(), false)
	key.Bind(editor)
	view := func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() { ui.TextArea(c, &key, "Key", "").Width(440).Height(150) })
	}
	page := ui.NewPage(ui.PageConfig{View: view})
	canvas := benchCanvas()
	drawPage(page, canvas, view)
	if allocs := testing.AllocsPerRun(50, func() { drawPage(page, canvas, view) }); allocs != 0 {
		t.Fatalf("a steady paint of a secure textarea allocates %v objects", allocs)
	}
}

// BenchmarkTextAreaSecure is a steady paint of a page with a textarea the size of
// a key in it, showing its text and showing its bullets.
func BenchmarkTextAreaSecure(b *testing.B) {
	value := strings.Repeat("bG9uZyBsaW5lIG9mIGEgc2VjcmV0IHRoYXQgd3JhcHMgaW4gdGhlIGJveA\n", 70)
	for _, secure := range []bool{false, true} {
		name := "text"
		if secure {
			name = "secure"
		}
		b.Run(name, func(b *testing.B) {
			var key ui.Field
			key.Secure = secure
			editor := text.NewEditor(value)
			editor.MoveTo(editor.RuneCount(), false)
			key.Bind(editor)
			page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
			view := func(c ui.C) {
				ui.Column(c).Pad(16).Children(func() { ui.TextArea(c, &key, "Key", "").Width(440).Height(150) })
			}
			b.ReportAllocs()
			for b.Loop() {
				drawPage(page, canvas, view)
			}
		})
	}
}
