//go:build js && wasm

package web

import (
	"syscall/js"
	"testing"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
)

func TestBrowserClipboardEventCopiesAndCutsOverride(t *testing.T) {
	copied := ""
	prevented := 0
	setData := js.FuncOf(func(_ js.Value, arguments []js.Value) any {
		if arguments[0].String() != "text/plain" {
			t.Fatalf("clipboard type = %q", arguments[0].String())
		}
		copied = arguments[1].String()
		return nil
	})
	preventDefault := js.FuncOf(func(js.Value, []js.Value) any {
		prevented++
		return nil
	})
	t.Cleanup(setData.Release)
	t.Cleanup(preventDefault.Release)
	data := js.ValueOf(map[string]any{"setData": setData})
	event := js.ValueOf(map[string]any{"clipboardData": data, "preventDefault": preventDefault})
	element := js.ValueOf(map[string]any{"value": "abc", "selectionStart": 2, "selectionEnd": 2})
	setSelectionRange := js.FuncOf(func(_ js.Value, arguments []js.Value) any {
		element.Set("selectionStart", arguments[0].Int())
		element.Set("selectionEnd", arguments[1].Int())
		return nil
	})
	t.Cleanup(setSelectionRange.Release)
	element.Set("setSelectionRange", setSelectionRange)
	node := &accessibilityDOMNode{element: element}
	edits := 0
	host := &textInputHost{active: node, input: platform.TextInput{
		CopyOverride: true, CopyText: func() string { return "one\ntwo" },
		State: platform.TextState{Value: "abc", Selection: text.Range{Start: 2, End: 2}},
		OnEvent: func(event platform.TextEvent) (platform.TextState, error) {
			if event.Kind != platform.TextPaste || event.Text != "" {
				t.Fatalf("cut event = %+v", event)
			}
			edits++
			return platform.TextState{Value: "a", Selection: text.Range{Start: 1, End: 1}}, nil
		},
	}}
	host.copyEvent(node, event)
	if copied != "one\ntwo" || prevented != 1 || edits != 0 {
		t.Fatalf("copy: copied=%q prevented=%d edits=%d", copied, prevented, edits)
	}
	host.cutEvent(node, event)
	if copied != "one\ntwo" || prevented != 2 || edits != 1 || element.Get("value").String() != "a" {
		t.Fatalf("cut: copied=%q prevented=%d edits=%d value=%q", copied, prevented, edits, element.Get("value").String())
	}
	copied = ""
	host.input.Secure = true
	host.input.CopyOverride = false
	host.copyEvent(node, event)
	if copied != "" || prevented != 3 || edits != 1 {
		t.Fatalf("secure copy: copied=%q prevented=%d edits=%d", copied, prevented, edits)
	}
	host.input.Secure = false
	host.input.CopyOverride = true
	host.input.CopyText = nil
	host.input.State.Selection = text.Range{End: 1}
	copied = "keep"
	host.copyEvent(node, event)
	host.cutEvent(node, event)
	if copied != "keep" || prevented != 5 || edits != 1 || element.Get("value").String() != "a" {
		t.Fatalf("nil override: copied=%q prevented=%d edits=%d value=%q", copied, prevented, edits, element.Get("value").String())
	}
}
