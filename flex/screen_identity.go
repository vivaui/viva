package flex

import (
	"slices"
	"strings"
	"unsafe"

	"github.com/vivaui/viva/internal/diag"
	"github.com/vivaui/viva/ui"
)

// How a Screen names its controls without the application naming them. Every
// identity is still an exact ui.ID in the Screen's Tree; the three rules below only
// choose it, so that an identity never moves to another control without the user
// knowing (see private/docs/decisions/0067-screen-identity.md).
//
//  1. Inside Key or KeyIndex, a control is numbered among the controls of its kind
//     in that row: the key holds the row, and a row's layout is fixed.
//  2. Elsewhere, a control bound to a value (a Checkbox to a *bool, a Field to a
//     *Field...) is named by that value's address: a Field shown or hidden above a
//     password field does not hand the password's identity, its focus and its
//     typing, to the new one.
//  3. Elsewhere, a control bound to nothing (a Button) is numbered among the
//     controls of its kind in its scope, and guarded: a held press or focus whose
//     identity now names a different button is not delivered (see guard).
//
// Every name the Screen makes starts with '~', which no name the application gives
// may start with, so the two never meet.

// screenKind is a kind of control, numbered apart from the others.
type screenKind uint8

const (
	skButton screenKind = iota
	skCheckbox
	skSwitch
	skChoice
	skTabs
	skSelect
	skField
	skSlider
	skSplitter
	skTable
	skPopover
	skModal
	screenKinds
)

var screenKindNames = [screenKinds]string{
	"~button", "~checkbox", "~switch", "~choice", "~tabs", "~select",
	"~field", "~slider", "~splitter", "~table", "~popover", "~modal",
}

// slotQuarantine is how many paints a released number waits before another key
// or value takes it: by then no hit, press or focus is left on its old identity.
const slotQuarantine = 2

// scopeRecord is what a Screen keeps for one scope: how many controls of each kind
// it has declared in this paint, and the scopes it made under it.
type scopeRecord struct {
	scope  ui.Scope
	epoch  uint32
	keyed  bool // a Key or KeyIndex row: rule 1
	counts [screenKinds]uint32
	kinds  [screenKinds]ui.Scope // the "~button"... scope of each kind, made when first used
	keySub ui.Scope              // the "~key" scope the rows of Key and KeyIndex go under
	keys   slotPool
	// labels are the labels of the unbound buttons of this paint (rule 3), by
	// number: two the same are a loop that forgot Key, and the last paint's (prev,
	// painted in paint prevEpoch) tell a button that moved from one that was relabeled.
	labels    []string
	prev      []string
	prevEpoch uint32
	dupWarned bool
}

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
	parent ui.Scope
	text   string
	number int
	isText bool
}

type keyEntry struct{ slot, epoch uint32 }

// boundKey is one value a control is bound to; n counts the controls of the same
// kind bound to it in one paint, which share the value but not the identity.
type boundKey struct {
	value unsafe.Pointer
	kind  screenKind
	n     uint32
}

type boundEntry struct{ slot, epoch uint32 }

// armedButton is an unbound button the user holds (pressed or focused) and its label.
type armedButton struct {
	id    ui.ID
	label string
	moved bool // the number moved to another button: nothing it holds is delivered
}

// recordOf returns the index of the record of scope, making it the first time.
func (sh *screenShared) recordOf(scope ui.Scope, keyed bool) int32 {
	if index, ok := sh.records[scope]; ok {
		return index
	}
	index := int32(len(sh.list))
	sh.list = append(sh.list, scopeRecord{scope: scope, keyed: keyed, epoch: sh.epoch - 1})
	if sh.records == nil {
		sh.records = make(map[ui.Scope]int32)
	}
	sh.records[scope] = index
	return index
}

// record returns the record of the Screen's scope, its counts started for this paint.
func (s Screen) record() *scopeRecord {
	rec := &s.sh.list[s.rec]
	if rec.epoch != s.sh.epoch {
		clear(rec.prev)
		rec.prev, rec.labels, rec.prevEpoch = rec.labels, rec.prev[:0], rec.epoch
		rec.epoch = s.sh.epoch
		rec.counts = [screenKinds]uint32{}
	}
	return rec
}

// kindScope returns the scope the controls of kind are numbered in under rec.
func (sh *screenShared) kindScope(rec *scopeRecord, kind screenKind) ui.Scope {
	if rec.kinds[kind] == 0 {
		rec.kinds[kind] = sh.tree.Sub(rec.scope, sh.kindNames[kind])
	}
	return rec.kinds[kind]
}

// anonID numbers a control among those of its kind in the Screen's scope: rules 1 and 3.
func (s Screen) anonID(kind screenKind) ui.ID {
	rec := s.record()
	n := rec.counts[kind]
	rec.counts[kind]++
	return s.sh.kindScope(rec, kind).At(int(n))
}

// boundID names a control bound to value: rule 2, or rule 1 inside a Key row.
func (s Screen) boundID(kind screenKind, value unsafe.Pointer) ui.ID {
	if s.record().keyed {
		return s.anonID(kind)
	}
	sh := s.sh
	key := boundKey{value: value, kind: kind}
	for {
		entry, ok := sh.bound[key]
		if ok && entry.epoch == sh.epoch {
			key.n++ // declared already in this paint: the next one bound to it
			continue
		}
		if !ok {
			entry.slot = sh.boundSlots[kind].take(sh.epoch)
			if sh.bound == nil {
				sh.bound = make(map[boundKey]boundEntry)
			}
		}
		entry.epoch = sh.epoch
		sh.bound[key] = entry
		return sh.kindScope(&sh.list[sh.rootRec], kind).At(int(entry.slot))
	}
}

// child returns the Screen for the scope made by a Key, KeyIndex or Scoped call.
func (s Screen) child(scope ui.Scope, keyed bool) Screen {
	index := s.sh.recordOf(scope, keyed)
	return Screen{f: s.f, sh: s.sh, scope: scope, rec: index}
}

// keyed returns the Screen of the row named by key under the Screen's scope.
func (s Screen) keyed(key keyKey) Screen {
	s.frame()
	sh := s.sh
	key.parent = s.scope
	entry, ok := sh.keys[key]
	if ok && entry.epoch == sh.epoch {
		panic("flex: Screen.Key: two rows of " + sh.tree.Path(s.scope) + " have the key " + key.describe() +
			" in one paint; give each row a key of its own (an item's ID, not its position)")
	}
	rec := s.record()
	if !ok {
		entry.slot = rec.keys.take(sh.epoch)
		if sh.keys == nil {
			sh.keys = make(map[keyKey]keyEntry)
		}
	}
	entry.epoch = sh.epoch
	sh.keys[key] = entry
	if rec.keySub == 0 {
		rec.keySub = sh.tree.Sub(s.scope, sh.keyName)
	}
	return s.child(sh.tree.Sub(rec.keySub, ui.Index(int(entry.slot))), true)
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

// checkName stops a name the application gives from taking the Screen's own.
func checkName(call, name string) {
	if strings.HasPrefix(name, "~") {
		panic("flex: Screen." + call + "(" + name + "): names that start with '~' are the Screen's own")
	}
}

// guard is rule 3's protection for an unbound button with identity id: while the
// user holds it (presses it, or it has the focus), the label it had is kept. When
// the button with that number has another label and that label was another
// number's in the last paint, the buttons moved (one above came or went) and the
// number names another button now: it does not take the held press (Clicked is
// false) and gives the focus up. A label that is new is the same button relabeled
// (a counter, a countdown) and keeps it. guard also reports two unbound buttons
// with one label, which it cannot tell apart, so Key must.
func (s Screen) guard(b Box, id ui.ID, label string) {
	sh := s.sh
	canvas := s.f.canvas
	if canvas == nil {
		return
	}
	rec := s.record()
	if rec.keyed {
		return // a Key row holds its buttons: its layout does not move under them
	}
	if !rec.dupWarned && slices.Contains(rec.labels, label) {
		rec.dupWarned = true
		diag.Report(diag.Warn, "flex", "screen-duplicate-button",
			"two buttons labeled \""+label+"\" in "+sh.tree.Path(s.scope)+" have no Key: wrap each row in s.Key(item.ID) or s.KeyIndex(item.ID), "+
				"so a row added or removed above cannot move a click to the wrong row")
	}
	number := len(rec.labels)
	rec.labels = append(rec.labels, label)
	focused := canvas.Focused() == id
	if !focused && canvas.Pressed() != id {
		return
	}
	held := sh.armed(id)
	switch {
	case held == nil:
		sh.arm(armedButton{id: id, label: label})
	case held.moved:
		b.n().flags |= flagVeto
	case held.label == label:
	case rec.prevEpoch+1 == sh.epoch && movedFrom(rec.prev, label, number):
		held.moved = true
		b.n().flags |= flagVeto
		if focused {
			canvas.ReleaseFocus(id)
		}
	default:
		held.label = label
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

func (sh *screenShared) armed(id ui.ID) *armedButton {
	for i := range sh.held {
		if sh.held[i].id == id {
			return &sh.held[i]
		}
	}
	return nil
}

func (sh *screenShared) arm(button armedButton) {
	for i := range sh.held {
		if sh.held[i].id.IsZero() {
			sh.held[i] = button
			return
		}
	}
	sh.held[0] = button
}

// beginIdentity starts a paint: a new epoch, and the held buttons the user let go of
// are forgotten.
func (sh *screenShared) beginIdentity(canvas *ui.Canvas) {
	sh.epoch++
	for i := range sh.held {
		if id := sh.held[i].id; !id.IsZero() && (canvas == nil || id != canvas.Pressed() && id != canvas.Focused()) {
			sh.held[i] = armedButton{}
		}
	}
}

// endIdentity ends a paint: the rows and values it did not declare give their
// numbers back, and a click that reached a bound control whose identity changed
// under it is reported.
func (sh *screenShared) endIdentity(canvas *ui.Canvas) {
	for key, entry := range sh.keys {
		if entry.epoch != sh.epoch {
			delete(sh.keys, key)
			if index, ok := sh.records[key.parent]; ok {
				sh.list[index].keys.give(entry.slot, sh.epoch)
			}
		}
	}
	for key, entry := range sh.bound {
		if entry.epoch != sh.epoch {
			delete(sh.bound, key)
			sh.boundSlots[key.kind].give(entry.slot, sh.epoch)
		}
	}
	if canvas == nil || sh.lostWarned {
		return
	}
	target, ok := canvas.ClickTarget()
	if !ok {
		return
	}
	root := &sh.list[sh.rootRec]
	for kind := range screenKinds {
		if kind == skButton || root.kinds[kind] == 0 || target.Scope() != root.kinds[kind] {
			continue
		}
		slot, _ := target.Name().Index()
		for key, entry := range sh.bound {
			if key.kind == kind && int(entry.slot) == slot {
				return
			}
		}
		sh.lostWarned = true
		diag.Report(diag.Warn, "flex", "screen-lost-click",
			"a click on a "+screenKindNames[kind][1:]+" was lost: the value it is bound to has another address in every paint "+
				"(a local copy, such as v := m[k]; s.Checkbox(&v, ...)). Bind the model's own field, or declare the control inside s.Key(...)")
		return
	}
}

// Describe writes an identity a Screen made as its path, with the keys of Key rows
// in place of their numbers, such as "settings/~key/\"edge-01\"/~button/#0": for
// test failures and logs. It allocates.
func (s *Screen) Describe(id ui.ID) string {
	sh := s.sh
	if sh == nil || sh.tree == nil {
		return "none"
	}
	text := sh.tree.Text(id)
	for key, entry := range sh.keys {
		index, ok := sh.records[key.parent]
		if !ok || sh.list[index].keySub == 0 {
			continue
		}
		path := sh.tree.Path(sh.list[index].keySub) + "/" + sh.tree.NameText(ui.Index(int(entry.slot)))
		if strings.HasPrefix(text, path+"/") || text == path {
			text = sh.tree.Path(sh.list[index].keySub) + "/" + key.describe() + text[len(path):]
		}
	}
	return text
}
