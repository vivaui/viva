//go:build android

package android

/*
#include "jni_android.h"
*/
import "C"

import (
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

// The flags of a node, as the host's Accessibility reads them.
const (
	a11yFocused = 1 << iota
	a11yDisabled
	a11yChecked
	a11yMixed
	a11ySelected
	a11yExpanded
	a11yMultiline
	a11yReadOnly
	a11yInvalid
	a11ySecure
	a11yFocusable
	a11yExpandable
	a11yHasRange
)

func a11yFlags(node platform.AccessibilityNode) int32 {
	var flags int32
	for _, flag := range []struct {
		on  bool
		bit int32
	}{
		{node.Focused, a11yFocused}, {node.Disabled, a11yDisabled}, {node.Checked, a11yChecked}, {node.Mixed, a11yMixed},
		{node.Selected, a11ySelected}, {node.Expanded, a11yExpanded}, {node.Multiline, a11yMultiline}, {node.ReadOnly, a11yReadOnly},
		{node.Invalid, a11yInvalid}, {node.Secure, a11ySecure}, {node.Focusable, a11yFocusable}, {node.Expandable, a11yExpandable},
		{node.Range.Set, a11yHasRange},
	} {
		if flag.on {
			flags |= flag.bit
		}
	}
	return flags
}

// pushTree gives the host the accessibility tree a screen reader reads, in the window's
// pixels. The nodes are the application's to reuse after this returns, so what is kept is
// the place of each in the tree, which is what a request of the screen reader names.
func (win *window) pushTree(nodes []platform.AccessibilityNode) {
	count := len(nodes)
	win.treeIDs = win.treeIDs[:0]
	bounds := make([]float32, 0, 4*count)
	ints := make([]int32, 0, 5*count)
	ranges := make([]float32, 0, 3*count)
	offsets := make([]int32, 0, 4*count+1)
	var chars []uint16
	scale := win.density
	for _, node := range nodes {
		win.treeIDs = append(win.treeIDs, node.ID)
		bounds = append(bounds, node.X*scale, node.Y*scale, node.Width*scale, node.Height*scale)
		ints = append(ints, int32(node.Role), a11yFlags(node), int32(node.Level), node.PosInSet, node.SetSize)
		ranges = append(ranges, float32(node.Range.Min), float32(node.Range.Max), float32(node.Range.Now))
		for _, text := range [4]string{node.Label, node.Value, node.Hint, node.Description} {
			offsets = append(offsets, int32(len(chars)))
			chars = append(chars, utf16Units(text)...)
		}
	}
	offsets = append(offsets, int32(len(chars)))
	var boundsPointer, rangesPointer *C.float
	var intsPointer *C.int32_t
	var charsPointer *C.uint16_t
	if count > 0 {
		boundsPointer = (*C.float)(unsafe.Pointer(&bounds[0]))
		intsPointer = (*C.int32_t)(unsafe.Pointer(&ints[0]))
		rangesPointer = (*C.float)(unsafe.Pointer(&ranges[0]))
	}
	if len(chars) > 0 {
		charsPointer = (*C.uint16_t)(unsafe.Pointer(&chars[0]))
	}
	C.viva_host_tree(C.int(count), boundsPointer, intsPointer, rangesPointer, charsPointer, C.int(len(chars)), (*C.int32_t)(unsafe.Pointer(&offsets[0])))
}

// accessibilityState follows a screen reader starting or stopping. Starting, the program
// asks the application for the tree it has, as frames carry it only when it changes.
func (application *application) accessibilityState(on bool) {
	win := application.window
	if win == nil || win.closed {
		return
	}
	win.a11y = on
	if on && win.config.AccessibilityTree != nil {
		win.pushTree(win.config.AccessibilityTree())
	}
}

// accessible carries out a screen reader's request of the node at index in the tree last
// sent, and shows what the application paints for it.
func (application *application) accessible(ev event) {
	win := application.window
	if win == nil || win.closed || win.config.OnAccessibility == nil || ev.index < 0 || int(ev.index) >= len(win.treeIDs) {
		return
	}
	defer application.guard()
	action := [...]platform.AccessibilityAction{
		1: platform.AccessibilityPress, 2: platform.AccessibilityFocus, 3: platform.AccessibilityIncrement,
		4: platform.AccessibilityDecrement, 5: platform.AccessibilitySetNumber, 6: platform.AccessibilitySetText,
		7: platform.AccessibilitySetSelection,
	}
	if ev.action < 1 || int(ev.action) >= len(action) {
		return
	}
	request := platform.AccessibilityRequest{
		Node: win.treeIDs[ev.index], Action: action[ev.action], Number: float64(ev.dx), Text: ev.text, Start: ev.start, End: ev.end,
	}
	if _, err := win.config.OnAccessibility(request, win.present); err != nil {
		application.fail(err)
	}
}
