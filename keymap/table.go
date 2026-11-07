package keymap

// Table holds the chord of each action of an application whose actions are
// small dense integers. Action 0 is reserved to mean no action: it has no chord
// and Lookup returns it for a chord nothing is bound to.
//
// A Table is made with NewTable and copied with Clone; assigning one shares the
// chords. It is not safe for concurrent use.
type Table[A ~uint8] struct {
	chords []Chord
}

// NewTable returns a table for actions 1 through count-1, none bound.
func NewTable[A ~uint8](count int) Table[A] {
	return Table[A]{chords: make([]Chord, count)}
}

// Count is the number of action values the table has room for, the reserved 0
// included.
func (table Table[A]) Count() int { return len(table.chords) }

// Chord is the chord bound to action: the zero Chord when it has none or is out
// of range.
func (table Table[A]) Chord(action A) Chord {
	if int(action) >= len(table.chords) {
		return Chord{}
	}
	return table.chords[action]
}

// Set gives action chord without checking for a conflict, the way defaults and
// a saved table are loaded. It reports false for an action out of range or the
// reserved 0.
func (table *Table[A]) Set(action A, chord Chord) bool {
	if action == 0 || int(action) >= len(table.chords) {
		return false
	}
	table.chords[action] = chord
	return true
}

// Lookup is the action bound to chord, or 0 for none. An invalid chord is never
// bound.
func (table Table[A]) Lookup(chord Chord) A {
	if !chord.Valid() {
		return 0
	}
	for action := 1; action < len(table.chords); action++ {
		if table.chords[action] == chord {
			return A(action)
		}
	}
	return 0
}

// Bind gives action chord. It refuses an invalid chord, an action out of range,
// and a chord another action holds, in which case conflict is that action; a
// chord the action already holds binds again without a conflict.
func (table *Table[A]) Bind(action A, chord Chord) (conflict A, ok bool) {
	if action == 0 || int(action) >= len(table.chords) || !chord.Valid() {
		return 0, false
	}
	if owner := table.Lookup(chord); owner != 0 && owner != action {
		return owner, false
	}
	table.chords[action] = chord
	return 0, true
}

// Unbind takes the chord away from action, which then runs nothing until it is
// bound again. It reports false for an action out of range or the reserved 0.
func (table *Table[A]) Unbind(action A) bool { return table.Set(action, Chord{}) }

// Clone is an independent copy of the table.
func (table Table[A]) Clone() Table[A] {
	return Table[A]{chords: append([]Chord(nil), table.chords...)}
}

// Equal reports whether both tables bind every action to the same chord.
func (table Table[A]) Equal(other Table[A]) bool {
	if len(table.chords) != len(other.chords) {
		return false
	}
	for action := range table.chords {
		if table.chords[action] != other.chords[action] {
			return false
		}
	}
	return true
}
