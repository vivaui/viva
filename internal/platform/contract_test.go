package platform

import (
	"testing"

	"github.com/vivaui/viva/internal/input"
)

func TestShortcutCandidateOffersControlAndMetaEnterButNeverPlainEnter(t *testing.T) {
	for _, test := range []struct {
		event KeyEvent
		want  bool
	}{
		{KeyEvent{Key: input.KeyEnter, Modifiers: input.ModifierControl}, true},
		{KeyEvent{Key: input.KeyEnter, Modifiers: input.ModifierMeta | input.ModifierShift}, true},
		{KeyEvent{Key: input.KeyEnter}, false},
		{KeyEvent{Key: input.KeyEnter, Modifiers: input.ModifierShift}, false},
		{KeyEvent{Key: input.KeyEnter, Modifiers: input.ModifierAlt}, false},
		{KeyEvent{Key: input.KeyLeft, Modifiers: input.ModifierControl}, false},
		{KeyEvent{Modifiers: input.ModifierControl, Text: "s"}, true},
	} {
		if got := ShortcutCandidate(test.event); got != test.want {
			t.Errorf("ShortcutCandidate(%+v) = %v, want %v", test.event, got, test.want)
		}
	}
}

func TestPrintableShortcutsAcceptAnyModifiersAndLeaveNamedKeys(t *testing.T) {
	for _, event := range []KeyEvent{
		{Text: "j"}, {Text: "j", Modifiers: input.ModifierAlt},
		{Text: "J", Modifiers: input.ModifierShift}, {Text: "/"}, {Text: " "},
	} {
		if !ShortcutCandidate(event) {
			t.Errorf("printable shortcut was not offered: %+v", event)
		}
	}
	for _, event := range []KeyEvent{
		{Key: input.KeyLeft}, {Key: input.KeyF8}, {Key: input.KeyEnter},
		{Text: "dead"}, {Text: "j", Release: true},
	} {
		if ShortcutCandidate(event) {
			t.Errorf("non-printable event was offered: %+v", event)
		}
	}
}
