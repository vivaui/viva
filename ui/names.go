package ui

import (
	"strings"
	"unsafe"

	"github.com/vivaui/viva/gfx"
)

// The bookkeeping a Page names its elements with (see page.go).

// slotQuarantine is how many paints a released number waits before another key
// or value takes it: by then no hit, press or focus is left on its old identity.
const slotQuarantine = 2

// slotPool hands out numbers, and takes them back to give again after the quarantine.
type slotPool struct {
	next uint32
	free []freeSlot
}

type freeSlot struct{ slot, at uint32 }

func (pool *slotPool) take(epoch uint32) uint32 {
	if len(pool.free) > 0 && epoch-pool.free[0].at >= slotQuarantine {
		slot := pool.free[0].slot
		copy(pool.free, pool.free[1:])
		pool.free = pool.free[:len(pool.free)-1]
		return slot
	}
	slot := pool.next
	pool.next++
	return slot
}

func (pool *slotPool) give(slot, epoch uint32) {
	pool.free = append(pool.free, freeSlot{slot: slot, at: epoch})
}

// keyKey is one row of Key or KeyIndex: its parent scope and its key.
type keyKey struct {
	parent gfx.Scope
	text   string
	number int
	isText bool
}

type keyEntry struct{ slot, epoch uint32 }

// boundKey is one value a control is bound to; n counts the controls of the same
// kind bound to it in one paint, which share the value but not the identity.
type boundKey struct {
	value unsafe.Pointer
	kind  uint8
	n     uint32
}

// armedButton is an unbound button the user holds (pressed or focused) and its label.
type armedButton struct {
	id    gfx.ID
	label string
	moved bool // the number moved to another button: nothing it holds is delivered
}

func (key keyKey) describe() string {
	if key.isText {
		return `"` + key.text + `"`
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
func checkName(call, name string) {
	if strings.HasPrefix(name, "~") {
		panic("ui: " + call + "(" + name + "): names that start with '~' are the page's own")
	}
}

// movedFrom reports whether label was another number's than number in the last paint.
func movedFrom(prev []string, label string, number int) bool {
	for at, seen := range prev {
		if seen == label && at != number {
			return true
		}
	}
	return false
}

type fieldEntry struct {
	id    gfx.ID
	field *Field
}
