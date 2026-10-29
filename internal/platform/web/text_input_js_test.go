//go:build js && wasm

package web

import (
	"syscall/js"
	"testing"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

func browserTextElement(t *testing.T) js.Value {
	t.Helper()
	document := js.Global().Get("document")
	if !document.Truthy() {
		t.Skip("no document: run in a browser")
	}
	element := document.Call("createElement", "textarea")
	document.Get("body").Call("appendChild", element)
	t.Cleanup(func() { element.Call("remove") })
	return element
}

func TestBrowserTextSelectionKeepsBackwardActiveEnd(t *testing.T) {
	element := browserTextElement(t)
	element.Set("value", "abcdef")
	editor := text.NewEditor("abcdef")
	editor.SetSelection(text.Range{Start: 5, End: 2})
	host := newTextInputHost(nil)
	host.input.State = platform.EditorState(editor)
	events := 0
	host.input.OnEvent = func(event platform.TextEvent) (platform.TextState, error) {
		events++
		event.Apply(editor)
		return platform.EditorState(editor), nil
	}
	host.active = &accessibilityDOMNode{element: element}
	host.syncValueAndSelection(element)
	if got := element.Get("selectionDirection").String(); got != "backward" {
		t.Fatalf("selection direction = %q", got)
	}
	if host.applySelection(element) || events != 0 {
		t.Fatal("unchanged backward selection dispatched an edit")
	}
	if host.input.State.Selection != (text.Range{Start: 2, End: 5}) || !host.input.State.SelectionReversed {
		t.Fatalf("state = %+v", host.input.State)
	}
	// A browser can change the active end without changing its sorted range.
	element.Call("setSelectionRange", 2, 5, "forward")
	if !host.applySelection(element) || events != 1 || editor.Caret() != 5 {
		t.Fatalf("direction-only change: events=%d caret=%d", events, editor.Caret())
	}
	element.Call("setSelectionRange", 2, 5, "backward")
	if !host.applySelection(element) || editor.Caret() != 2 {
		t.Fatal("backward active end was lost")
	}
	// Shift+Right shrinks from the left active end, keeping the anchor at 5.
	editor.MoveRight(true)
	host.input.State = platform.EditorState(editor)
	host.syncValueAndSelection(element)
	if element.Get("selectionStart").Int() != 3 || element.Get("selectionEnd").Int() != 5 || element.Get("selectionDirection").String() != "backward" {
		t.Fatalf("extended selection lost its anchor: %+v", host.input.State)
	}
}

func TestBrowserTextApplyConsumesRedrawRequestEveryTime(t *testing.T) {
	count := 0
	host := newTextInputHost(nil)
	host.input.State = platform.TextState{Value: "same"}
	host.input.OnEvent = func(platform.TextEvent) (platform.TextState, error) {
		count++
		return platform.TextState{Value: "same", Redraw: true}, nil
	}
	if !host.apply(platform.TextEvent{Kind: platform.TextCommandEvent, Command: platform.TextUndo}) ||
		!host.apply(platform.TextEvent{Kind: platform.TextCommandEvent, Command: platform.TextUndo}) {
		t.Fatalf("redraw request was not consumed on both events, count=%d", count)
	}
	if host.input.State.Redraw {
		t.Fatal("redraw request leaked into persistent state")
	}
}

func TestTextSelectionDirectionFallsBackWithoutProperty(t *testing.T) {
	// This minimal control intentionally has no selectionDirection property.
	element := js.ValueOf(map[string]any{"value": "abc", "selectionStart": 1, "selectionEnd": 3})
	editor := text.NewEditor("abc")
	editor.SetSelection(text.Range{Start: 3, End: 1})
	host := newTextInputHost(nil)
	host.input.State = platform.EditorState(editor)
	events := 0
	host.input.OnEvent = func(event platform.TextEvent) (platform.TextState, error) {
		events++
		event.Apply(editor)
		return platform.EditorState(editor), nil
	}
	if host.applySelection(element) || events != 0 || editor.Caret() != 1 {
		t.Fatal("missing direction reset the existing active end")
	}
	element.Set("selectionStart", 0)
	if !host.applySelection(element) || events != 1 || editor.Selection() != (text.Range{Start: 0, End: 3}) {
		t.Fatal("control without direction cannot update selection")
	}
}

func TestTextKeyCompositionPrecedesApplicationShortcuts(t *testing.T) {
	for _, source := range []string{"host", "editor", "event", "legacy"} {
		t.Run(source, func(t *testing.T) {
			element := js.Global().Get("Object").New()
			host := newTextInputHost(nil)
			host.active = &accessibilityDOMNode{element: element}
			event := js.ValueOf(map[string]any{"target": element, "key": "a", "ctrlKey": true, "isComposing": false, "keyCode": 65})
			switch source {
			case "host":
				host.composing = true
			case "editor":
				host.input.State.Composing = true
			case "event":
				event.Set("isComposing", true)
			case "legacy":
				event.Set("keyCode", 229)
			}
			calls := 0
			config := Config{OnShortcut: func(ui.KeyEvent) ui.EventResult { calls++; return ui.EventResult{Handled: true} }}
			r := runtime{config: config, a11y: &accessibilityMirror{text: host}, painter: &painter{config: &config}}
			// No preventDefault stub is supplied: any attempt to take the IME key
			// would panic, and invoking OnShortcut would increment calls.
			r.keyDown(event)
			if calls != 0 {
				t.Fatal("application shortcut took an IME key")
			}
		})
	}
}

func TestTextRuntimeHistoryUndoesProgrammaticEdits(t *testing.T) {
	for _, primary := range []string{"ctrlKey", "metaKey"} {
		t.Run(primary, func(t *testing.T) {
			var element js.Value
			if js.Global().Get("document").Truthy() {
				element = browserTextElement(t)
			} else {
				// A minimal DOM text-control contract also executes the same
				// runtime path under Node; browsers use a real textarea above.
				element = js.Global().Get("Function").New(`return {value:"",selectionStart:0,selectionEnd:0,selectionDirection:"forward",setSelectionRange(start,end,direction){this.selectionStart=start;this.selectionEnd=end;this.selectionDirection=direction}}`).Invoke()
			}
			editor := text.NewEditor("hello")
			host := newTextInputHost(nil)
			host.active = &accessibilityDOMNode{element: element}
			host.input = TextInput{Multiline: true, State: platform.EditorState(editor), OnEvent: func(event TextEvent) (TextState, error) {
				event.Apply(editor)
				return platform.EditorState(editor), nil
			}}
			host.syncValueAndSelection(element)
			shortcutHandled := false
			config := Config{OnShortcut: func(ui.KeyEvent) ui.EventResult { return ui.EventResult{Handled: shortcutHandled} }}
			r := runtime{config: config, a11y: &accessibilityMirror{text: host}, painter: &painter{config: &config}, pending: true}
			press := func(key string, modified, shift, composing bool, want string, wantPrevented bool) {
				t.Helper()
				prevented := false
				prevent := js.FuncOf(func(js.Value, []js.Value) any { prevented = true; return nil })
				defer prevent.Release()
				event := js.ValueOf(map[string]any{"target": element, "key": key, "ctrlKey": false, "metaKey": false, "altKey": false, "shiftKey": shift, "repeat": false, "isComposing": composing, "keyCode": 0, "preventDefault": prevent})
				event.Set(primary, modified)
				r.keyDown(event)
				if editor.String() != want || element.Get("value").String() != want || prevented != wantPrevented {
					t.Fatalf("key %s: editor=%q DOM=%q prevented=%v, want %q/%v", key, editor.String(), element.Get("value").String(), prevented, want, wantPrevented)
				}
			}
			press("Backspace", false, false, false, "hell", true)
			press("z", true, false, false, "hello", true)
			press("Z", true, true, false, "hell", true)
			press("z", true, false, false, "hello", true)
			press("y", true, false, false, "hell", true)
			press("Enter", false, false, false, "hell\n", true)
			press("z", true, false, false, "hell", true)
			press("Z", true, true, false, "hell\n", true)
			press("z", true, false, true, "hell\n", false)
			shortcutHandled = true
			press("z", true, false, false, "hell\n", true)
		})
	}
}
