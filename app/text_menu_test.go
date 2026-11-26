package app

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

// textMenuDriver is a window with one text field (and a button apart from it)
// that asks for the system's text menu.
func textMenuDriver(t *testing.T, editor *text.Editor, secure, handled bool, labels *TextMenuLabels) *Driver {
	t.Helper()
	config := Config{Title: "text menu", Width: 300, Height: 120, TextMenu: labels}
	config.Paint = func(canvas *gfx.Canvas, _ gfx.Size) {
		canvas.HitWithOptions(gfx.Item(1), gfx.Rect{X: 10, Y: 10, Width: 220, Height: 30}, "field", gfx.HitOptions{
			Role: gfx.RoleTextField, Secure: secure,
		})
		canvas.Hit(gfx.Item(2), gfx.Rect{X: 10, Y: 60, Width: 80, Height: 30})
	}
	config.TextInput = func(TextInputContext) *TextInput { return &TextInput{Editor: editor} }
	if handled {
		config.OnPointer = func(event gfx.PointerEvent) gfx.EventResult {
			return gfx.EventResult{Handled: event.Button == gfx.PointerButtonSecondary}
		}
	}
	driver, err := NewDriver(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	return driver
}

func textMenuItem(items []TextMenuItem, command TextMenuCommand) (TextMenuItem, bool) {
	for _, item := range items {
		if item.Command == command {
			return item, true
		}
	}
	return TextMenuItem{}, false
}

// A secondary press on a text field opens the system's menu for it, and the field
// takes the keyboard as it does under a primary press. Each command does what the
// chord of the same name does.
func TestTextMenuCommandsActOnTheField(t *testing.T) {
	editor := text.NewEditor("hello world")
	driver := textMenuDriver(t, editor, false, false, &TextMenuLabels{Paste: "粘贴"})
	if err := driver.Press(20, 20, gfx.PointerButtonSecondary); err != nil {
		t.Fatal(err)
	}
	items := driver.TextMenu()
	if len(items) == 0 {
		t.Fatal("a secondary press on a text field opened no menu")
	}
	if paste, _ := textMenuItem(items, TextMenuPaste); paste.Title != "粘贴" {
		t.Fatalf("Paste is %q, want the words the window gave", paste.Title)
	}
	if cut, _ := textMenuItem(items, TextMenuCut); cut.Title != "Cut" || !cut.Disabled {
		t.Fatalf("Cut = %+v, want English and off with nothing selected", cut)
	}
	if err := driver.ChooseTextMenu(TextMenuCut); err == nil {
		t.Fatal("Cut was chosen while it is off")
	}
	if err := driver.ChooseTextMenu(TextMenuSelectAll); err != nil {
		t.Fatal(err)
	}
	if driver.TextMenu() != nil {
		t.Fatal("the menu stayed open after a choice")
	}
	if selection := editor.Selection(); selection != (text.Range{Start: 0, End: 11}) {
		t.Fatalf("selection after Select All = %v", selection)
	}
	if err := driver.Press(20, 20, gfx.PointerButtonSecondary); err != nil {
		t.Fatal(err)
	}
	if err := driver.ChooseTextMenu(TextMenuCut); err != nil {
		t.Fatal(err)
	}
	if editor.String() != "" || driver.Clipboard() != "hello world" {
		t.Fatalf("after Cut: %q, clipboard %q", editor.String(), driver.Clipboard())
	}
	driver.SetClipboard("a\r\nb")
	if err := driver.Press(20, 20, gfx.PointerButtonSecondary); err != nil {
		t.Fatal(err)
	}
	if err := driver.ChooseTextMenu(TextMenuPaste); err != nil {
		t.Fatal(err)
	}
	if editor.String() != "a b" {
		t.Fatalf("Paste into a single line = %q", editor.String())
	}
	if driver.painter.focus.Focused() != gfx.Item(1) {
		t.Fatal("the field does not have the keyboard after its menu")
	}
}

// A secret never leaves through the menu, as it does not through the keyboard.
func TestTextMenuOfASecretOffersNoWayOut(t *testing.T) {
	editor := text.NewEditor("hunter2")
	driver := textMenuDriver(t, editor, true, false, &TextMenuLabels{})
	if err := driver.Press(20, 20, gfx.PointerButtonSecondary); err != nil {
		t.Fatal(err)
	}
	if handled, err := driver.Shortcut('a'); !handled || err != nil {
		t.Fatalf("select all: handled=%v err=%v", handled, err)
	}
	if err := driver.Press(20, 20, gfx.PointerButtonSecondary); err != nil {
		t.Fatal(err)
	}
	for _, command := range []TextMenuCommand{TextMenuCut, TextMenuCopy} {
		if item, _ := textMenuItem(driver.TextMenu(), command); !item.Disabled {
			t.Fatalf("command %d of a secret is on: %+v", command, item)
		}
	}
	if item, _ := textMenuItem(driver.TextMenu(), TextMenuPaste); item.Disabled {
		t.Fatal("Paste of a secret is off")
	}
}

// No menu opens off a text field, when the application took the press (its own
// menu), or when the window asked for none.
func TestTextMenuOpensOnlyWhereNobodyElseTookThePress(t *testing.T) {
	for _, test := range []struct {
		name    string
		x, y    float32
		handled bool
		labels  *TextMenuLabels
	}{
		{"away from the field", 20, 70, false, &TextMenuLabels{}},
		{"the application took the press", 20, 20, true, &TextMenuLabels{}},
		{"the window asked for none", 20, 20, false, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			driver := textMenuDriver(t, text.NewEditor("x"), false, test.handled, test.labels)
			if err := driver.Press(test.x, test.y, gfx.PointerButtonSecondary); err != nil {
				t.Fatal(err)
			}
			if driver.TextMenu() != nil {
				t.Fatal("a text menu opened")
			}
		})
	}
}
