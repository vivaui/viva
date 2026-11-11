package platform

import (
	"errors"
	"testing"

	"github.com/vivaui/viva/text"
)

func TestTextInputClipboardStateUsesExplicitCopyOverride(t *testing.T) {
	called := 0
	input := TextInput{State: TextState{Value: "abc", Selection: text.Range{Start: 1, End: 1}}, CopyOverride: true, CopyText: func() string {
		called++
		return "one\ntwo"
	}}
	if called != 0 {
		t.Fatal("copy text was requested before a clipboard action")
	}
	for _, action := range []byte{0, 'a', 'v', 'z'} {
		if state := input.ClipboardState(action); state != input.State || called != 0 {
			t.Fatalf("action %q materialized override: state=%+v calls=%d", action, state, called)
		}
	}
	state := input.ClipboardState('c')
	if state.Value != "one\ntwo" || state.Selection != (text.Range{Start: 0, End: 7}) || called != 1 {
		t.Fatalf("override state = %+v", state)
	}
	input.CopyOverride = false
	if state = input.ClipboardState('c'); state != input.State {
		t.Fatalf("ordinary state changed: %+v", state)
	}
	input.CopyOverride = true
	problem := errors.New("clipboard busy")
	if _, changed, err := TextClipboardEvent(input.ClipboardState('x'), false, 'x', nil, func(string) error { return problem }); changed || !errors.Is(err, problem) {
		t.Fatalf("failed custom cut changed editor: changed=%v err=%v", changed, err)
	}
	input.Secure = true
	before := called
	if _, changed, err := TextClipboardEvent(input.ClipboardState('c'), true, 'c', nil, func(string) error { t.Fatal("secure copy"); return nil }); changed || err != nil {
		t.Fatalf("secure override copied: changed=%v err=%v", changed, err)
	}
	if state := input.ClipboardState('x'); state != input.State || called != before {
		t.Fatalf("secure copy materialized override: state=%+v calls=%d, want %d", state, called, before)
	}
}

func TestTextInputNilCopyOverrideDoesNotFallBackToNativeSelection(t *testing.T) {
	input := TextInput{
		State:        TextState{Value: "native selection", Selection: text.Range{End: 16}},
		CopyOverride: true,
	}
	for _, action := range []byte{'c', 'x'} {
		writes := 0
		event, changed, err := TextClipboardEvent(input.ClipboardState(action), false, action, nil, func(string) error {
			writes++
			return nil
		})
		if err != nil || changed || writes != 0 || event != (TextEvent{}) {
			t.Fatalf("nil override action %c: event=%+v changed=%v writes=%d err=%v", action, event, changed, writes, err)
		}
	}
}
