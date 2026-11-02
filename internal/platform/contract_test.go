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
