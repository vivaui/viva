package input

// ModifierTap recognizes a modifier key pressed and released on its own: no
// other key, modifier, text, pointer press or scroll in between, and the focus
// kept. It follows one candidate at a time and never allocates.
//
// The caller feeds it the modifier presses and releases the platform reports,
// and Cancel for everything else that happens in the window. A key that
// repeats while held is ignored, so a held modifier is one candidate.
type ModifierTap struct {
	key    Key
	code   uint32
	active bool
}

// Press records a modifier key going down. held is the modifier set the
// platform reported with the press. The press starts a candidate when nothing
// else is down; a second modifier ends it, and so does a modifier that was
// already held. A press with no code cannot be paired with its release and
// starts nothing.
func (tap *ModifierTap) Press(key Key, code uint32, held Modifiers, repeat bool) {
	if repeat && tap.active && tap.code == code {
		return
	}
	if !key.IsModifier() || code == 0 || tap.active || held&^key.Modifier() != 0 {
		tap.active = false
		return
	}
	tap.key, tap.code, tap.active = key, code, true
}

// Cancel ends the candidate: something other than the modifier happened.
func (tap *ModifierTap) Cancel() { tap.active = false }

// Release reports the modifier that was tapped when the key with code goes up
// while it is still a candidate.
func (tap *ModifierTap) Release(code uint32) (Key, bool) {
	if !tap.active || tap.code != code {
		return KeyUnknown, false
	}
	tap.active = false
	return tap.key, true
}
