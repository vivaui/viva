package keymap

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

type action uint8

const (
	actionNone action = iota
	actionOpen
	actionClose
	actionFind
	actionCount
)

func newTestTable() Table[action] {
	table := NewTable[action](int(actionCount))
	table.Set(actionOpen, Chord{Modifiers: Primary, Char: 'o'})
	table.Set(actionClose, Chord{Modifiers: Primary, Char: 'w'})
	return table
}

func TestLookupFindsTheActionABindingRuns(t *testing.T) {
	table := newTestTable()
	if got := table.Lookup(Chord{Modifiers: Primary, Char: 'w'}); got != actionClose {
		t.Errorf("Lookup = %v, want close", got)
	}
	if got := table.Lookup(Chord{Modifiers: Primary, Char: 'x'}); got != actionNone {
		t.Errorf("an unbound chord found %v", got)
	}
	if got := table.Lookup(Chord{}); got != actionNone {
		t.Errorf("the zero chord found %v", got)
	}
}

func TestBindNamesTheActionThatHoldsTheChord(t *testing.T) {
	table := newTestTable()
	open := table.Chord(actionOpen)
	conflict, ok := table.Bind(actionFind, open)
	if ok || conflict != actionOpen {
		t.Fatalf("Bind onto a held chord = %v, %v; want conflict with open", conflict, ok)
	}
	if table.Chord(actionFind) != (Chord{}) {
		t.Fatalf("a refused bind changed the table: %+v", table.Chord(actionFind))
	}
	if conflict, ok := table.Bind(actionOpen, open); !ok || conflict != actionNone {
		t.Fatalf("binding the chord an action already holds = %v, %v", conflict, ok)
	}
	free := Chord{Modifiers: Primary, Char: 'f'}
	if _, ok := table.Bind(actionFind, free); !ok || table.Lookup(free) != actionFind {
		t.Fatalf("a free chord was not bound")
	}
}

func TestBindRefusesWhatIsNotBindable(t *testing.T) {
	table := newTestTable()
	for _, test := range []struct {
		action action
		chord  Chord
	}{
		{actionFind, Chord{Char: 'F'}},
		{actionNone, Chord{Char: 'a'}},
		{actionCount, Chord{Char: 'a'}},
	} {
		if _, ok := table.Bind(test.action, test.chord); ok {
			t.Errorf("Bind(%v, %+v) succeeded", test.action, test.chord)
		}
	}
}

func TestUnbindFreesTheChord(t *testing.T) {
	table := newTestTable()
	open := table.Chord(actionOpen)
	if !table.Unbind(actionOpen) || table.Chord(actionOpen) != (Chord{}) || table.Lookup(open) != actionNone {
		t.Fatal("Unbind left the chord bound")
	}
	if _, ok := table.Bind(actionFind, open); !ok {
		t.Fatal("a chord an action gave up is not free")
	}
	if table.Unbind(actionNone) || table.Unbind(actionCount) {
		t.Fatal("Unbind accepted an action that does not exist")
	}
}

func TestCloneIsIndependent(t *testing.T) {
	table := newTestTable()
	clone := table.Clone()
	if !table.Equal(clone) {
		t.Fatal("a clone differs from its source")
	}
	table.Unbind(actionOpen)
	if clone.Chord(actionOpen) == (Chord{}) || table.Equal(clone) {
		t.Fatal("changing the source changed the clone")
	}
}

func TestSetKeepsTheReservedActionUnbound(t *testing.T) {
	table := newTestTable()
	if table.Set(actionNone, Chord{Modifiers: ui.ModifierAlt, Char: 'a'}) {
		t.Fatal("the reserved action took a chord")
	}
}
