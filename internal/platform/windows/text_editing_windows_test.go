package windows

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/text"
)

func TestNativeTextDragEndsWhenCaptureIsLost(t *testing.T) {
	const window = ^uintptr(0)
	state := &windowState{ready: true, textDragging: true}
	previous := active.Load()
	windowStates.Store(window, state)
	defer func() { windowStates.Delete(window); active.Store(previous) }()
	windowProc(window, wmCaptureChanged, 0, 0)
	if state.textDragging {
		t.Fatal("capture loss left the editor dragging")
	}
}

func BenchmarkNativeTextVisuals(b *testing.B) {
	engine, err := theTextEngine()
	if err != nil {
		b.Fatal(err)
	}
	for _, secure := range []bool{false, true} {
		for _, length := range []int{32, 256, 4096} {
			b.Run(fmt.Sprintf("secure=%v/runes=%d", secure, length), func(b *testing.B) {
				state := windowState{textEnabled: true, caretVisible: true, textInput: platform.TextInput{Width: 240, Height: 30, FontSize: 16, Secure: secure}, textEngine: engine}
				state.textLabels = make([]platform.TextLabel, 1)
				state.textRectangles = make([]render.Rectangle, textRectangleCount)
				state.textState = platform.EditorState(text.NewEditor(strings.Repeat("a", length)))
				state.updateTextVisuals()
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					state.updateTextVisuals()
				}
			})
		}
	}
}

func BenchmarkNativeEditingKeyMap(b *testing.B) {
	event := platform.KeyEvent{Key: input.KeyLeft, Modifiers: input.ModifierControl | input.ModifierShift}
	b.ReportAllocs()
	for b.Loop() {
		platform.TextEditingCommand(event)
	}
}

func TestNativeEditorOwnsClipboardAndEditingShortcuts(t *testing.T) {
	state := windowState{textEnabled: true}
	calls := 0
	state.config.OnShortcut = func(platform.KeyEvent) (platform.Frame, bool, error) { calls++; return platform.Frame{}, true, nil }
	for _, key := range []uintptr{'A', 'C', 'X', 'V', 'Z', 'Y'} {
		if state.offerShortcut(key, 0, input.ModifierControl) {
			t.Fatalf("editor key %c was offered to application", key)
		}
	}
	if calls != 0 {
		t.Fatal("application stole an editor shortcut")
	}
	state.textInput.Terminal = true
	if !state.offerShortcut('V', 0, input.ModifierControl) || calls != 1 {
		t.Fatal("terminal lost its application paste binding")
	}
	state.textInput.Terminal = false
	if !state.offerShortcut('P', 0, input.ModifierControl|input.ModifierShift) || calls != 2 {
		t.Fatal("non-editing application shortcut was lost")
	}
}

func TestNativeEditingKeysUseTheMessageModifiers(t *testing.T) {
	if value, _, ready := decodeUTF16Character(0, 0x7f); ready || value != "" {
		t.Fatal("Ctrl+Backspace's DEL character must not be inserted after deleting a word")
	}
	editor := text.NewEditor("host.example.com")
	state := windowState{textEnabled: true}
	state.textInput.OnEvent = func(event platform.TextEvent) (platform.TextState, error) {
		event.Apply(editor)
		return platform.EditorState(editor), errors.New("stop before presenting")
	}
	for _, step := range []struct {
		key   uintptr
		mods  input.Modifiers
		caret int
	}{
		{0x25, input.ModifierControl, 13},
		{0x25, input.ModifierControl | input.ModifierShift, 12},
		{0x24, 0, 0},
		{0x23, input.ModifierShift, 16},
	} {
		if !state.textKey(step.key, 0, step.mods) || editor.Caret() != step.caret {
			t.Fatalf("key=%x caret=%d", step.key, editor.Caret())
		}
	}
	if editor.Selection() != (text.Range{End: 16}) {
		t.Fatalf("shift end selection=%+v", editor.Selection())
	}
	state.textInput.Placed = true
	if state.textKey(0x24, 0, 0) {
		t.Fatal("placed editor lost visual Home navigation")
	}
	state.textInput.Placed = false
	if state.textKey(9, 0, 0) || state.textKey(13, 0, 0) {
		t.Fatal("single-line field swallowed Tab/Enter")
	}
}

func TestNativePointerSupportsDragShiftClickAndWordSelection(t *testing.T) {
	editor := text.NewEditor("one two")
	state := windowState{textEnabled: true, textInput: platform.TextInput{X: 10, Y: 20, Width: 100, Height: 30, FontSize: 16}, textEngine: fixedAdvanceEngine{advance: 10}}
	apply := func(event platform.PointerEvent) {
		t.Helper()
		state.textState = platform.EditorState(editor)
		edit, handled, err := state.textPointerEvent(event)
		if err != nil || !handled {
			t.Fatalf("pointer=%+v handled=%v err=%v", event, handled, err)
		}
		edit.Apply(editor)
	}
	apply(platform.PointerEvent{Kind: platform.PointerDown, Button: gfx.PointerButtonPrimary, X: 60, Y: 35})
	apply(platform.PointerEvent{Kind: platform.PointerMove, X: 20, Y: 35})
	if editor.Selection() != (text.Range{Start: 1, End: 5}) || editor.Caret() != 1 {
		t.Fatalf("backwards drag=%+v", platform.EditorState(editor))
	}
	apply(platform.PointerEvent{Kind: platform.PointerDown, Button: gfx.PointerButtonPrimary, Modifiers: input.ModifierShift, X: 30, Y: 35})
	if editor.Selection() != (text.Range{Start: 2, End: 5}) {
		t.Fatalf("shift click lost anchor: %+v", editor.Selection())
	}
	apply(platform.PointerEvent{Kind: platform.PointerDown, Button: gfx.PointerButtonPrimary, ClickCount: 2, X: 60, Y: 35})
	if editor.Selection() != (text.Range{Start: 4, End: 7}) {
		t.Fatalf("double click=%+v", editor.Selection())
	}
	apply(platform.PointerEvent{Kind: platform.PointerDown, Button: gfx.PointerButtonPrimary, ClickCount: 3, X: 60, Y: 35})
	if editor.Selection() != (text.Range{End: 7}) {
		t.Fatal("triple click did not select all")
	}
}

func TestNativeTextScrollClipsAndMasksSecureFields(t *testing.T) {
	editor := text.NewEditor("中文long-value")
	state := windowState{textEnabled: true, caretVisible: true, textInput: platform.TextInput{X: 10, Y: 20, Width: 40, Height: 30, FontSize: 16}, textEngine: fixedAdvanceEngine{advance: 10}}
	state.textLabels = make([]platform.TextLabel, 1)
	state.textRectangles = make([]render.Rectangle, textRectangleCount)
	state.textState = platform.EditorState(editor)
	if err := state.updateTextVisuals(); err != nil {
		t.Fatal(err)
	}
	if caret := state.textRectangles[1].Bounds; state.textScroll <= 0 || !state.textLabels[0].Clipped ||
		caret.X < state.textInput.X || caret.X+caret.Width > state.textInput.X+state.textInput.Width {
		t.Fatalf("caret not scrolled inside the field: %+v", caret)
	}
	// Replacing a scrolled value with short text must reveal its beginning.
	state.textState = platform.EditorState(text.NewEditor("abc"))
	state.updateTextVisuals()
	if state.textScroll != 0 || state.textLabels[0].X != state.textInput.X {
		t.Fatal("short replacement remained scrolled out of view")
	}
	editor.SetSelection(text.Range{Start: editor.RuneCount(), End: 0})
	state.textState = platform.EditorState(editor)
	state.updateTextVisuals()
	if state.textScroll != 0 || state.textRectangles[1].Bounds.X != 10 {
		t.Fatal("backwards selection caret was rendered at its anchor")
	}
	state.textInput.Secure = true
	state.updateTextVisuals()
	if got := state.textLabels[0].Value; got != "••••••••••••" {
		t.Fatalf("secure display = %q", got)
	}
	for _, rect := range state.textRectangles {
		if !rect.Clipped {
			t.Fatal("editor decoration can paint outside the field")
		}
	}
}

// The overlay rasterizes a scrolled line from its clip, not from its start: the
// texture holds the visible columns, so a long value costs its visible width, not
// its whole width, and the skip is whole device columns so no glyph moves.
func TestNativeTextOverlayRasterizesOnlyTheVisibleColumns(t *testing.T) {
	label := labelTexture{label: platform.TextLabel{
		Value: "value", X: -1490, Y: 0, Width: 1700, Height: 24, FontSize: 16,
		Clipped: true, Clip: gfx.Rect{X: 10, Y: 0, Width: 200, Height: 24},
	}}
	if skip, width, ok := label.window(2); !ok || skip != 3000 || width != 400 {
		t.Fatalf("window = %d, %d, %v; want 3000, 400, true", skip, width, ok)
	}
	label.label.X, label.label.Width = 10, 200
	if skip, width, ok := label.window(2); !ok || skip != 0 || width != 400 {
		t.Fatalf("unscrolled window = %d, %d, %v; want 0, 400, true", skip, width, ok)
	}
}
