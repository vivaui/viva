package interaction

import (
	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/ui"
)

// Keys follows the keys a terminal asked to see as presses and releases. The
// zero value tracks nothing, so a page without such a terminal pays nothing.
type Keys struct {
	tracker    input.KeyTracker
	target     ui.ID
	releaseDue bool
}

// Len is the number of keys down.
func (k *Keys) Len() int { return k.tracker.Len() }

// Press records a key going down. It reports whether this is an auto-repeat of
// the same physical key and whether the key could be tracked. The target that
// owns the first held key is remembered so a later paint can tell that the
// keyboard moved away from it.
func (k *Keys) Press(held input.HeldKey) (repeat, tracked bool) {
	repeat, tracked = k.tracker.Press(held)
	if tracked && !repeat && k.tracker.Len() == 1 {
		k.target = held.Target
	}
	return repeat, tracked
}

// Release returns the press that corresponds to a physical key going up. An
// orphan release, such as the end of a key an input method consumed, reports
// false.
func (k *Keys) Release(code uint32) (input.HeldKey, bool) { return k.tracker.Release(code) }

// Take removes every key still down, in the order they went down, so callers
// can release them last-pressed first.
func (k *Keys) Take(dst []input.HeldKey) []input.HeldKey { return k.tracker.Take(dst) }

// NoteTarget records, after a paint, whether the held keys still have a
// terminal that wants them. If not, the next repaint ends their presses.
func (k *Keys) NoteTarget(target ui.ID, wanted bool) {
	if k.tracker.Len() > 0 && (!wanted || target != k.target) {
		k.releaseDue = true
	}
}

// TakeDue reports and clears a request to release the held keys.
func (k *Keys) TakeDue() bool {
	due := k.releaseDue
	k.releaseDue = false
	return due
}
