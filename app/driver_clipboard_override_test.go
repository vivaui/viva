//go:build darwin || windows || linux

package app

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

func TestDriverTextInputCanOverrideCopyAndCutText(t *testing.T) {
	editor := text.NewEditor("abc\ndef")
	copyText := "bc\n  "
	copyCalls := 0
	cutEvents := 0
	config := Config{Title: "clipboard", Width: 300, Height: 120}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		components.PaintInput(canvas, ui.LightTheme(), ui.Rect{X: 10, Y: 10, Width: 220, Height: 30}, components.InputSnapshot{
			ID: ui.Item(1), Label: "Editor", Value: editor.String(), PlatformEditing: canvas.Focused() == ui.Item(1),
		})
	}
	config.TextInput = func(TextInputContext) *TextInput {
		return &TextInput{State: TextStateOf(editor), CopyOverride: true, CopyText: func() string {
			copyCalls++
			return copyText
		}, OnEvent: func(event TextEvent) (TextState, error) {
			if event.Kind == TextPaste && event.Text == "" {
				cutEvents++
			}
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
	if copyCalls != 0 {
		t.Fatalf("paint prepared copy text %d times", copyCalls)
	}
	driver.SetClipboard("sentinel")
	if handled, err := driver.ShortcutChord(ui.ModifierControl, 'c'); !handled || err != nil || driver.Clipboard() != copyText || copyCalls != 1 {
		t.Fatalf("custom copy handled=%v err=%v clipboard=%q", handled, err, driver.Clipboard())
	}
	if handled, err := driver.ShortcutChord(ui.ModifierControl, 'x'); !handled || err != nil || cutEvents != 1 || driver.Clipboard() != copyText || copyCalls != 2 {
		t.Fatalf("custom cut handled=%v err=%v events=%d clipboard=%q", handled, err, cutEvents, driver.Clipboard())
	}
	copyText = ""
	if handled, err := driver.ShortcutChord(ui.ModifierControl, 'c'); !handled || err != nil || driver.Clipboard() != "bc\n  " {
		t.Fatalf("empty custom selection overwrote clipboard: %q, %v", driver.Clipboard(), err)
	}
}

func BenchmarkDriverCopyOverrideLargeSelectionRepaint(b *testing.B) {
	editor := text.NewEditor("value")
	calls := 0
	config := Config{Title: "large copy", Width: 300, Height: 120}
	config.Paint = func(canvas *ui.Canvas, _ ui.Size) {
		canvas.HitWithOptions(ui.Item(1), ui.Rect{X: 10, Y: 10, Width: 220, Height: 30}, "editor", ui.HitOptions{Role: ui.RoleTextField})
	}
	config.TextInput = func(TextInputContext) *TextInput {
		return &TextInput{Editor: editor, CopyOverride: true, CopyText: func() string {
			calls++
			return strings.Repeat("wide selection\n", 1<<16)
		}}
	}
	driver, err := NewDriver(config, nil)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(driver.Close)
	if err := driver.Click(20, 20); err != nil {
		b.Fatal(err)
	}
	if !driver.painter.nativeText.CopyOverride {
		b.Fatal("copy override is not active")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		if err := driver.Resize(300+index%2, 120); err != nil {
			b.Fatal(err)
		}
	}
	if calls != 0 {
		b.Fatalf("large copy text materialized %d times without a copy action", calls)
	}
}

func BenchmarkDriverCopyOverrideLargeSelectionCopy(b *testing.B) {
	editor := text.NewEditor("value")
	config := Config{Title: "large copy", Width: 300, Height: 120}
	config.Paint = func(canvas *ui.Canvas, _ ui.Size) {
		canvas.HitWithOptions(ui.Item(1), ui.Rect{X: 10, Y: 10, Width: 220, Height: 30}, "editor", ui.HitOptions{Role: ui.RoleTextField})
	}
	config.TextInput = func(TextInputContext) *TextInput {
		return &TextInput{Editor: editor, CopyOverride: true, CopyText: func() string {
			return strings.Repeat("wide selection\n", 1<<16)
		}}
	}
	driver, err := NewDriver(config, nil)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(driver.Close)
	if err := driver.Click(20, 20); err != nil {
		b.Fatal(err)
	}
	if !driver.painter.nativeText.CopyOverride {
		b.Fatal("copy override is not active")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if handled, err := driver.ShortcutChord(ui.ModifierControl, 'c'); err != nil || !handled {
			b.Fatalf("copy handled=%v err=%v", handled, err)
		}
	}
}
