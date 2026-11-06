//go:build darwin || windows || linux

package app

import (
	"bytes"
	"image"
	"image/png"
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

func onePixelPNG(t testing.TB) []byte {
	t.Helper()
	var out bytes.Buffer
	if err := png.Encode(&out, image.NewNRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

// A paste with a picture on the clipboard and no text changes no editor, but a
// field that takes pictures is told, so that it can read the clipboard's image.
func TestPasteWithNoTextTellsTheFieldAndChangesNothing(t *testing.T) {
	theme := ui.DarkTheme()
	editor := text.NewEditor("kept")
	var events []TextEvent
	config := Config{Title: "paste", Width: 300, Height: 80}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		components.PaintInput(canvas, theme, ui.Rect{X: 10, Y: 10, Width: 200, Height: 30}, components.InputSnapshot{
			ID: ui.Item(5), Label: "Message", Value: editor.String(), PlatformEditing: canvas.Focused() == ui.Item(5),
		})
	}
	config.TextInput = func(TextInputContext) *TextInput {
		return &TextInput{State: TextStateOf(editor), OnEvent: func(event TextEvent) (TextState, error) {
			events = append(events, event)
			event.Apply(editor)
			return TextStateOf(editor), nil
		}}
	}
	driver, err := NewDriver(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Click(50, 25); err != nil {
		t.Fatal(err)
	}
	if err := driver.SetClipboardImage(onePixelPNG(t)); err != nil {
		t.Fatal(err)
	}
	if handled, err := driver.Shortcut('v'); err != nil || !handled {
		t.Fatalf("paste: handled=%v err=%v", handled, err)
	}
	if len(events) != 1 || events[0].Kind != TextPasteNoText {
		t.Fatalf("events = %+v", events)
	}
	if editor.String() != "kept" {
		t.Fatalf("a paste of no text changed the field to %q", editor.String())
	}
	if png, err := ClipboardImagePNG(); err != nil || len(png) == 0 {
		t.Fatalf("the field cannot read the picture it was told about: %v", err)
	}

	// Text on the clipboard is still a plain paste.
	driver.SetClipboard(" more")
	if handled, err := driver.Shortcut('v'); err != nil || !handled {
		t.Fatalf("paste: handled=%v err=%v", handled, err)
	}
	if last := events[len(events)-1]; last.Kind != TextPaste || editor.String() != "kept more" {
		t.Fatalf("last event %+v, text %q", last, editor.String())
	}
}

// A terminal needs a no-text paste event to handle non-text clipboard formats.
func TestTerminalReceivesAPasteWithNoText(t *testing.T) {
	probe := &terminalProbe{}
	driver, err := NewDriver(probe.config(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Click(100, 100); err != nil {
		t.Fatal(err)
	}
	if err := driver.SetClipboardImage(onePixelPNG(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := driver.Shortcut('v'); err != nil {
		t.Fatal(err)
	}
	if len(probe.text) != 1 || probe.text[0].Kind != TextPasteNoText {
		t.Fatalf("terminal paste events = %+v", probe.text)
	}
}

func BenchmarkTerminalPasteWithNoText(b *testing.B) {
	probe := &terminalProbe{}
	driver, err := NewDriver(probe.config(), nil)
	if err != nil {
		b.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Click(100, 100); err != nil {
		b.Fatal(err)
	}
	if err := driver.SetClipboardImage(onePixelPNG(b)); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if handled, err := driver.Shortcut('v'); err != nil || !handled {
			b.Fatalf("paste: handled=%v err=%v", handled, err)
		}
	}
}
