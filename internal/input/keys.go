package input

import "github.com/vivaui/viva/gfx"

// MaxHeldKeys is how many keys a KeyTracker follows at once. A keyboard cannot
// report more than a handful; presses past the limit are delivered but not
// tracked.
const MaxHeldKeys = 32

// HeldKey is a key that is down: the physical key's code, the identity the
// application saw when it went down, and the hit that had the keyboard then.
type HeldKey struct {
	// Code names the physical key in the platform's own numbering. It is
	// nonzero, and the same for the press and the release of one key.
	Code   uint32
	Key    Key
	Text   string
	Target gfx.ID
	Win32  gfx.Win32Key
}

// KeyTracker follows the keys that are down so that every release the
// application sees matches a press it saw, whatever order the platform reports
// things in. It never allocates.
type KeyTracker struct {
	held  [MaxHeldKeys]HeldKey
	count int
}

// Len is the number of keys down.
func (tracker *KeyTracker) Len() int { return tracker.count }

// Press records a key going down. It reports repeat when the same physical key
// was already down (auto-repeat), and tracked false when the key could not be
// recorded because the table is full or the code is zero.
func (tracker *KeyTracker) Press(key HeldKey) (repeat, tracked bool) {
	if key.Code == 0 {
		return false, false
	}
	for index := 0; index < tracker.count; index++ {
		if tracker.held[index].Code == key.Code {
			return true, true
		}
	}
	if tracker.count == len(tracker.held) {
		return false, false
	}
	tracker.held[tracker.count] = key
	tracker.count++
	return false, true
}

// Release forgets the key with the code and returns what it was. A code that
// is not down (an orphan release, such as the end of a key an input method
// consumed) reports false.
func (tracker *KeyTracker) Release(code uint32) (HeldKey, bool) {
	for index := 0; index < tracker.count; index++ {
		if tracker.held[index].Code != code {
			continue
		}
		key := tracker.held[index]
		copy(tracker.held[index:tracker.count], tracker.held[index+1:tracker.count])
		tracker.count--
		tracker.held[tracker.count] = HeldKey{}
		return key, true
	}
	return HeldKey{}, false
}

// Take removes every key from the tracker and appends them to dst in the order
// they went down, which is the order a remote should see them released in
// reverse of; callers release them last-pressed first.
func (tracker *KeyTracker) Take(dst []HeldKey) []HeldKey {
	dst = append(dst, tracker.held[:tracker.count]...)
	tracker.held = [MaxHeldKeys]HeldKey{}
	tracker.count = 0
	return dst
}
