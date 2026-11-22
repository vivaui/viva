package base

import (
	"cmp"
	"slices"
	"strings"
	"unsafe"

	"github.com/vivaui/viva/gfx"
)

// The bookkeeping a Page names its elements with (see page.go).

// slotQuarantine is how many paints a released number waits before another key
// or value takes it: by then no hit, press or focus is left on its old identity.
const slotQuarantine = 2

// slotPool hands out numbers, and takes them back to give again after the quarantine.
// The free list is a queue: free[head:] waits in release order, and take pops
// its front without moving the rest, so a paint that releases and claims
// thousands of rows stays linear.
type slotPool struct {
	Next uint32
	free []freeSlot
	head int
	// unsorted marks a release of this paint that came after a higher number of
	// the same paint: a sweep releases in map order, and the numbers a paint
	// hands out should not depend on it.
	unsorted bool
}

type freeSlot struct{ slot, at uint32 }

func (pool *slotPool) take(epoch uint32) uint32 {
	if pool.head < len(pool.free) {
		if pool.unsorted {
			slices.SortFunc(pool.free[pool.head:], func(a, b freeSlot) int {
				if c := cmp.Compare(a.at, b.at); c != 0 {
					return c
				}
				return cmp.Compare(a.slot, b.slot)
			})
			pool.unsorted = false
		}
		if front := pool.free[pool.head]; epoch-front.at >= slotQuarantine {
			pool.head++
			if pool.head == len(pool.free) {
				pool.free, pool.head = pool.free[:0], 0
			}
			return front.slot
		}
	}
	slot := pool.Next
	pool.Next++
	return slot
}

func (pool *slotPool) give(slot, epoch uint32) {
	if pool.head > 0 && pool.head >= len(pool.free)-pool.head {
		// More taken than waiting: move the waiting ones down so the queue
		// reuses its array, at a cost the takes already paid for.
		n := copy(pool.free, pool.free[pool.head:])
		pool.free, pool.head = pool.free[:n], 0
	}
	if last := len(pool.free) - 1; last >= pool.head {
		if prev := pool.free[last]; prev.at == epoch && prev.slot > slot {
			pool.unsorted = true
		}
	}
	pool.free = append(pool.free, freeSlot{slot: slot, at: epoch})
}

// keyKey is one row of Key or KeyIndex: its parent scope and its key.
type keyKey struct {
	Parent gfx.Scope
	Text   string
	number int
	isText bool
}

type keyEntry struct{ slot, epoch uint32 }

// boundKey is one value a control is bound to; n counts the controls of the same
// kind bound to it in one paint, which share the value but not the identity.
type boundKey struct {
	Value unsafe.Pointer
	Kind  uint8
	n     uint32
}

// armedButton is an unbound button the user holds (pressed or focused) and its label.
type armedButton struct {
	ID    gfx.ID
	Label string
	moved bool // the number moved to another button: nothing it holds is delivered
	// relabel is the label the button's place showed this paint when it differed
	// from Label: a new label, or the place now naming another button. Which one
	// is known only once the paint has declared the whole scope (settleHeld).
	relabel    string
	relabeling bool
}

func (key keyKey) describe() string {
	if key.isText {
		return `"` + key.Text + `"`
	}
	digits := [20]byte{}
	at := len(digits)
	value := key.number
	negative := value < 0
	if negative {
		value = -value
	}
	for {
		at--
		digits[at] = byte('0' + value%10)
		value /= 10
		if value == 0 {
			break
		}
	}
	if negative {
		at--
		digits[at] = '-'
	}
	return string(digits[at:])
}

// checkName stops a name the application gives from taking the page's own.
func checkName(call, Name string) {
	if strings.HasPrefix(Name, "~") {
		panic("ui: " + call + "(" + Name + "): names that start with '~' are the page's own")
	}
}

// movedFrom reports whether label was another number's than number in the last paint.
func movedFrom(prev []string, Label string, number int) bool {
	for at, seen := range prev {
		if seen == Label && at != number {
			return true
		}
	}
	return false
}

type fieldEntry struct {
	ID    gfx.ID
	field *Field
	box   int32                              // the field's element, for routing inside a wheel panel
	key   func(gfx.KeyEvent) gfx.EventResult // Box.OnKey, while this field has the keyboard
}
