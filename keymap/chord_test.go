package keymap

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

func TestFromEventNamesTheKeyAWindowDelivers(t *testing.T) {
	for _, test := range []struct {
		name  string
		event ui.KeyEvent
		want  Chord
		ok    bool
	}{
		{"printable chord", ui.KeyEvent{Modifiers: Primary, Text: "p"}, Chord{Modifiers: Primary, Char: 'p'}, true},
		{"shift reports the capital", ui.KeyEvent{Modifiers: Primary | ui.ModifierShift, Text: "P"}, Chord{Modifiers: Primary | ui.ModifierShift, Char: 'p'}, true},
		{"named key", ui.KeyEvent{Modifiers: ui.ModifierControl, Key: ui.KeyTab}, Chord{Modifiers: ui.ModifierControl, Key: ui.KeyTab}, true},
		{"space is a character", ui.KeyEvent{Modifiers: ui.ModifierAlt, Key: ui.KeySpace}, Chord{Modifiers: ui.ModifierAlt, Char: ' '}, true},
		{"a modifier alone", ui.KeyEvent{Key: ui.KeyShiftLeft}, Chord{}, false},
		{"composed text", ui.KeyEvent{Text: "é"}, Chord{}, false},
		{"nothing pressed", ui.KeyEvent{}, Chord{}, false},
	} {
		got, ok := FromEvent(test.event)
		if ok != test.ok || got != test.want {
			t.Errorf("%s: FromEvent = %+v, %v; want %+v, %v", test.name, got, ok, test.want, test.ok)
		}
	}
}

func TestValidRejectsWhatCannotBeBound(t *testing.T) {
	for _, chord := range []Chord{
		{},
		{Char: 'P'},
		{Char: 0x7f},
		{Char: 'a', Key: ui.KeyTab},
		{Key: ui.KeyShiftLeft},
		{Modifiers: 1 << 6, Char: 'a'},
	} {
		if chord.Valid() {
			t.Errorf("%+v is valid", chord)
		}
		if chord.Label() != "" || chord.Name() != "" || len(chord.Keycaps(nil)) != 0 {
			t.Errorf("%+v writes itself although it is invalid", chord)
		}
	}
}

func TestNameAndParseRoundTrip(t *testing.T) {
	var chords []Chord
	for index := range namedKeys {
		chords = append(chords, Chord{Modifiers: ui.ModifierAlt, Key: namedKeys[index].key})
	}
	for char := byte(' '); char <= '~'; char++ {
		if char >= 'A' && char <= 'Z' {
			continue
		}
		chords = append(chords, Chord{Modifiers: Primary | ui.ModifierShift, Char: char})
	}
	for _, chord := range chords {
		got, ok := Parse(chord.Modifiers, chord.Name())
		if !ok || got != chord {
			t.Errorf("Parse(%q) = %+v, %v; want %+v", chord.Name(), got, ok, chord)
		}
	}
	if got, ok := Parse(0, "P"); !ok || got.Char != 'p' {
		t.Errorf("an upper-case name is not read as its letter: %+v, %v", got, ok)
	}
	for _, name := range []string{"", "pageup", "ab", "shift"} {
		if chord, ok := Parse(0, name); ok {
			t.Errorf("Parse(%q) = %+v, want no chord", name, chord)
		}
	}
}

func TestLabelListsModifiersBeforeTheKey(t *testing.T) {
	if got := (Chord{Modifiers: ui.ModifierMeta | ui.ModifierControl | ui.ModifierAlt | ui.ModifierShift, Char: 'p'}).Label(); got != "Cmd+Ctrl+Alt+Shift+P" {
		t.Errorf("label = %q", got)
	}
	if got := (Chord{Modifiers: ui.ModifierControl, Key: ui.KeyPageUp}).Label(); got != "Ctrl+PageUp" {
		t.Errorf("label = %q", got)
	}
	if got := (Chord{Char: ' '}).Label(); got != "Space" {
		t.Errorf("label = %q", got)
	}
}

func TestKeycapsHaveOneCapPerModifierThenTheKey(t *testing.T) {
	chord := Chord{Modifiers: Primary | ui.ModifierShift | ui.ModifierAlt, Char: 'p'}
	caps := chord.Keycaps(nil)
	if len(caps) != 4 || caps[3] != "P" {
		t.Fatalf("keycaps = %q", caps)
	}
	seen := map[string]bool{}
	for _, cap := range caps[:3] {
		if cap == "" || seen[cap] {
			t.Fatalf("modifier keycaps are not distinct: %q", caps)
		}
		seen[cap] = true
	}
	if got := (Chord{Key: ui.KeyLeft}).Keycaps(nil); len(got) != 1 || got[0] != "←" {
		t.Errorf("named keycap = %q", got)
	}
	prefix := []string{"x"}
	if got := (Chord{Char: 'a'}).Keycaps(prefix); len(got) != 2 || got[0] != "x" || got[1] != "A" {
		t.Errorf("Keycaps does not append: %q", got)
	}
}

func TestPrimaryIsOneOfTheModifiers(t *testing.T) {
	if Primary != ui.ModifierMeta && Primary != ui.ModifierControl {
		t.Fatalf("Primary = %v", Primary)
	}
}
