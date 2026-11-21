package base

// attrSet records the generic attributes a caller set explicitly where the
// element's value cannot say so: a control's constructor writes its row defaults
// (Gap, Items, Justify) before the caller's calls, and LineGap outside a Flow is
// dropped by the builder itself. The set lives beside PadSet and stays in the
// node's existing header padding, so a Node does not grow. Only dev builds read
// it; marking one is an empty call that inlines away in release builds.
const (
	attrGap uint8 = 1 << iota
	attrItems
	attrJustify
	attrLineGap
	attrScroll
	attrHit
)

// SwitchCheckedSlot is the animation slot that holds a switch thumb's travel
// toward on, 0 (off) to 1 (on): the engine animates it to dress the track, and
// the control's painter reads the same slot to place the thumb.
const SwitchCheckedSlot uint8 = 0
