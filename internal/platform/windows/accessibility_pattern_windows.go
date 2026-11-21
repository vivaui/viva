package windows

import (
	"math"
	"sync"
	"unicode/utf16"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

// The control patterns an element can offer to UI Automation. Each is a COM
// interface of its own, a table of functions after IUnknown's three, found by
// QueryInterface or by GetPatternProvider with the pattern's number.
type uiaPatternKind uint8

const (
	patternInvoke uiaPatternKind = iota
	patternToggle
	patternValue
	patternRangeValue
	patternSelectionItem
	patternSelection
	patternExpandCollapse
	patternText
	patternKinds
)

const (
	resultAccessDenied = 0x80070005
	resultInvalidState = 0x80131509 // the operation is not valid for the element's state
)

// UIA_*PatternId.
var uiaPatternIDs = [patternKinds]int32{
	patternInvoke:         10000,
	patternToggle:         10015,
	patternValue:          10002,
	patternRangeValue:     10003,
	patternSelectionItem:  10010,
	patternSelection:      10001,
	patternExpandCollapse: 10005,
	patternText:           10014,
}

var uiaPatternIIDs = [patternKinds]guid{
	patternInvoke:         {0x54fcb24b, 0xe18e, 0x47a2, [8]byte{0xb4, 0xd3, 0xec, 0xcb, 0xe7, 0x75, 0x99, 0xa2}},
	patternToggle:         {0x56d00bd0, 0xc4f4, 0x433c, [8]byte{0xa8, 0x36, 0x1a, 0x52, 0xa5, 0x7e, 0x08, 0x92}},
	patternValue:          {0xc7935180, 0x6fb3, 0x4201, [8]byte{0xb1, 0x74, 0x7d, 0xf7, 0x3a, 0xdb, 0xf6, 0x4a}},
	patternRangeValue:     {0x36dc7aef, 0x33e6, 0x4691, [8]byte{0xaf, 0xe1, 0x2b, 0xe7, 0x27, 0x4b, 0x3d, 0x33}},
	patternSelectionItem:  {0x2acad808, 0xb2d4, 0x452d, [8]byte{0xa4, 0x07, 0x91, 0xff, 0x1a, 0xd1, 0x67, 0xb2}},
	patternSelection:      {0xfb8b03af, 0x3bdf, 0x48d4, [8]byte{0xbd, 0x36, 0x1a, 0x65, 0x79, 0x3b, 0xe1, 0x68}},
	patternExpandCollapse: {0xd847d3a5, 0xcab0, 0x4a98, [8]byte{0x8c, 0x32, 0xec, 0xb4, 0x5c, 0x59, 0xad, 0x24}},
	patternText:           {0x3589c92c, 0x63f3, 0x4367, [8]byte{0x99, 0xbb, 0xad, 0xa6, 0x53, 0xb7, 0x7c, 0xf2}},
}

// uiaPatternBits says which node capability each pattern stands for.
var uiaPatternBits = [patternKinds]platform.AccessibilityPatterns{
	patternInvoke:         platform.PatternInvoke,
	patternToggle:         platform.PatternToggle,
	patternValue:          platform.PatternValue,
	patternRangeValue:     platform.PatternRangeValue,
	patternSelectionItem:  platform.PatternSelectionItem,
	patternSelection:      platform.PatternSelection,
	patternExpandCollapse: platform.PatternExpandCollapse,
	patternText:           platform.PatternText,
}

var uiaPatternTables [patternKinds][]uintptr

// uiaRangeValueCallback is the function the SetValue thunk of the range value
// pattern tail-calls with the number moved into an integer register.
var uiaRangeValueCallback uintptr

func patternKindForID(id int32) (uiaPatternKind, bool) {
	for kind, candidate := range uiaPatternIDs {
		if candidate == id {
			return uiaPatternKind(kind), true
		}
	}
	return 0, false
}

func patternKindForIID(id guid) (uiaPatternKind, bool) {
	for kind, candidate := range uiaPatternIIDs {
		if candidate == id {
			return uiaPatternKind(kind), true
		}
	}
	return 0, false
}

// initializePatternTables builds the function tables, once. unknown is the
// IUnknown triple every table starts with.
func initializePatternTables(unknown []uintptr) {
	table := func(kind uiaPatternKind, methods ...uintptr) {
		entries := make([]uintptr, 0, 3+len(methods))
		entries = append(entries, unknown...)
		uiaPatternTables[kind] = append(entries, methods...)
	}
	callback := callbackFor
	table(patternInvoke, callback(uiaInvoke))
	table(patternToggle, callback(uiaToggle), callback(uiaGetToggleState))
	table(patternValue, callback(uiaSetValue), callback(uiaGetValue), callback(uiaGetValueIsReadOnly))
	uiaRangeValueCallback = callback(uiaRangeSetValueBits)
	table(patternRangeValue,
		uiaRangeValueThunkAddress(), callback(uiaGetRangeValue), callback(uiaGetRangeIsReadOnly),
		callback(uiaGetRangeMaximum), callback(uiaGetRangeMinimum), callback(uiaGetRangeLargeChange), callback(uiaGetRangeSmallChange))
	table(patternSelectionItem,
		callback(uiaSelect), callback(uiaAddToSelection), callback(uiaRemoveFromSelection),
		callback(uiaGetIsSelected), callback(uiaGetSelectionContainer))
	table(patternSelection, callback(uiaGetSelection), callback(uiaGetCanSelectMultiple), callback(uiaGetIsSelectionRequired))
	table(patternExpandCollapse, callback(uiaExpand), callback(uiaCollapse), callback(uiaGetExpandCollapseState))
	table(patternText,
		callback(uiaTextGetSelection), callback(uiaTextGetVisibleRanges), callback(uiaTextRangeFromChild),
		callback(uiaTextRangeFromPoint), callback(uiaTextGetDocumentRange), callback(uiaTextGetSupportedSelection))
	initializeTextRangeTable(unknown)
}

// callbackFor makes a function UI Automation can call. The function takes the
// interface pointer and the call's arguments, each as a uintptr.
func callbackFor(function any) uintptr {
	switch function := function.(type) {
	case func(uintptr) uintptr:
		return guarded1(uiaFault, function)
	case func(uintptr, uintptr) uintptr:
		return guarded2(uiaFault, function)
	case func(uintptr, uintptr, uintptr) uintptr:
		return guarded3(uiaFault, function)
	case func(uintptr, uintptr, uintptr, uintptr) uintptr:
		return guarded4(uiaFault, function)
	case func(uintptr, uintptr, uintptr, uintptr, uintptr) uintptr:
		return guarded5(uiaFault, function)
	}
	panic("windows: unsupported UI Automation callback shape")
}

// supports says whether the element offers a pattern now.
func (provider *uiaProvider) supports(kind uiaPatternKind) bool {
	if provider.Root {
		return false
	}
	node, ok := provider.snapshot()
	if !ok {
		return false
	}
	return provider.offers(node, kind)
}

func (provider *uiaProvider) offers(node platform.AccessibilityNode, kind uiaPatternKind) bool {
	patterns := node.Patterns()
	if !patterns.Has(uiaPatternBits[kind]) {
		return false
	}
	switch kind {
	case patternRangeValue:
		if node.Role == platform.A11yRoleProgress {
			// A progress bar reports a range and accepts no number.
			return node.Range.Set
		}
		fallthrough
	case patternInvoke, patternToggle, patternSelectionItem, patternExpandCollapse:
		// These act on the control, which a window with no way back to the
		// application cannot do.
		provider.Tree.mu.RLock()
		actionable := provider.Tree.actionable
		provider.Tree.mu.RUnlock()
		return actionable && !node.ID.IsZero()
	}
	return true
}

// patternPointer is the interface pointer of one pattern of the element.
func (provider *uiaProvider) patternPointer(kind uiaPatternKind) uintptr {
	patterns := provider.Patterns.Load()
	if patterns == nil {
		made := new([patternKinds]uiaInterface)
		for index := range made {
			made[index] = uiaInterface{Table: &uiaPatternTables[index][0], Provider: provider}
		}
		if provider.Patterns.CompareAndSwap(nil, made) {
			patterns = made
		} else {
			patterns = provider.Patterns.Load()
		}
	}
	return uintptr(unsafe.Pointer(&patterns[kind]))
}

// patternNode finds the element a pattern call is for, and its node.
func patternNode(this uintptr) (*uiaProvider, platform.AccessibilityNode, uintptr) {
	provider := providerFromInterface(this)
	if provider == nil || provider.Tree == nil {
		return nil, platform.AccessibilityNode{}, windowsResult(resultElementNotAvailable)
	}
	node, ok := provider.snapshot()
	if !ok {
		return nil, platform.AccessibilityNode{}, windowsResult(resultElementNotAvailable)
	}
	return provider, node, 0
}

// act sends the application a request for the element, unless it cannot take one.
func (provider *uiaProvider) act(node platform.AccessibilityNode, request platform.AccessibilityRequest) uintptr {
	switch {
	case node.ID.IsZero():
		return windowsResult(resultNotSupported)
	case node.Disabled:
		return windowsResult(resultElementNotEnabled)
	}
	request.Node = node.ID
	return provider.Tree.enqueue(request)
}

func (provider *uiaProvider) press(node platform.AccessibilityNode) uintptr {
	return provider.act(node, platform.AccessibilityRequest{Action: platform.AccessibilityPress})
}

func writeBool(result uintptr, value bool) uintptr {
	if result == 0 {
		return windowsResult(resultPointer)
	}
	if value {
		*(*int32)(pointer(result)) = 1
	} else {
		*(*int32)(pointer(result)) = 0
	}
	return 0
}

func writeInt(result uintptr, value int32) uintptr {
	if result == 0 {
		return windowsResult(resultPointer)
	}
	*(*int32)(pointer(result)) = value
	return 0
}

func writeDouble(result uintptr, value float64) uintptr {
	if result == 0 {
		return windowsResult(resultPointer)
	}
	*(*float64)(pointer(result)) = value
	return 0
}

// writeString gives the caller a BSTR it owns.
func writeString(result uintptr, value string) uintptr {
	if result == 0 {
		return windowsResult(resultPointer)
	}
	*(*uintptr)(pointer(result)) = 0
	allocated, ok := stringVariant(value)
	if !ok {
		return windowsResult(resultOutOfMemory)
	}
	*(*uintptr)(pointer(result)) = uintptr(allocated.Value)
	return 0
}

// utf16Pointer reads a NUL-terminated UTF-16 string, a BSTR or an LPCWSTR.
func utf16Pointer(address uintptr) string {
	if address == 0 {
		return ""
	}
	var units []uint16
	for offset := uintptr(0); offset < 1<<23; offset += 2 {
		unit := *(*uint16)(pointer(address + offset))
		if unit == 0 {
			break
		}
		units = append(units, unit)
	}
	return string(utf16.Decode(units))
}

func uiaInvoke(this uintptr) uintptr {
	provider, node, status := patternNode(this)
	if status != 0 {
		return status
	}
	return provider.press(node)
}

func uiaToggle(this uintptr) uintptr { return uiaInvoke(this) }

func uiaGetToggleState(this, result uintptr) uintptr {
	_, node, status := patternNode(this)
	if status != 0 {
		return status
	}
	return writeInt(result, toggleState(node))
}

func uiaSetValue(this, text uintptr) uintptr {
	provider, node, status := patternNode(this)
	if status != 0 {
		return status
	}
	if valueReadOnly(node) {
		return windowsResult(resultAccessDenied)
	}
	return provider.act(node, platform.AccessibilityRequest{Action: platform.AccessibilitySetText, Text: utf16Pointer(text)})
}

func uiaGetValue(this, result uintptr) uintptr {
	_, node, status := patternNode(this)
	if status != 0 {
		return status
	}
	return writeString(result, node.Value)
}

func uiaGetValueIsReadOnly(this, result uintptr) uintptr {
	_, node, status := patternNode(this)
	if status != 0 {
		return status
	}
	return writeBool(result, valueReadOnly(node))
}

// uiaRangeSetValueBits is SetValue of the range value pattern: its number
// arrives in an SSE register, which the thunk copies to bits.
func uiaRangeSetValueBits(this, bits uintptr) uintptr {
	provider, node, status := patternNode(this)
	if status != 0 {
		return status
	}
	number := math.Float64frombits(uint64(bits))
	if !node.Range.Set || node.Role == platform.A11yRoleProgress {
		return windowsResult(resultNotSupported)
	}
	if math.IsNaN(number) || number < node.Range.Min || number > node.Range.Max {
		return windowsResult(resultInvalidArgument)
	}
	return provider.act(node, platform.AccessibilityRequest{Action: platform.AccessibilitySetNumber, Number: number})
}

func uiaGetRangeValue(this, result uintptr) uintptr {
	_, node, status := patternNode(this)
	if status != 0 {
		return status
	}
	return writeDouble(result, node.Range.Now)
}

func uiaGetRangeIsReadOnly(this, result uintptr) uintptr {
	_, node, status := patternNode(this)
	if status != 0 {
		return status
	}
	if node.Role == platform.A11yRoleProgress {
		return writeBool(result, true)
	}
	return writeBool(result, node.Disabled)
}

func uiaGetRangeMaximum(this, result uintptr) uintptr {
	_, node, status := patternNode(this)
	if status != 0 {
		return status
	}
	return writeDouble(result, node.Range.Max)
}

func uiaGetRangeMinimum(this, result uintptr) uintptr {
	_, node, status := patternNode(this)
	if status != 0 {
		return status
	}
	return writeDouble(result, node.Range.Min)
}

func uiaGetRangeLargeChange(this, result uintptr) uintptr {
	_, node, status := patternNode(this)
	if status != 0 {
		return status
	}
	return writeDouble(result, max((node.Range.Max-node.Range.Min)/10, node.Range.Step))
}

func uiaGetRangeSmallChange(this, result uintptr) uintptr {
	_, node, status := patternNode(this)
	if status != 0 {
		return status
	}
	step := node.Range.Step
	if step <= 0 {
		step = 1
	}
	return writeDouble(result, step)
}

func uiaSelect(this uintptr) uintptr {
	provider, node, status := patternNode(this)
	if status != 0 {
		return status
	}
	if node.IsChosen() {
		return 0
	}
	return provider.press(node)
}

func uiaAddToSelection(this uintptr) uintptr { return uiaSelect(this) }

func uiaRemoveFromSelection(this uintptr) uintptr {
	_, node, status := patternNode(this)
	if status != 0 {
		return status
	}
	if !node.IsChosen() {
		return 0
	}
	// Choosing is how an item is selected, and another choice is how it stops.
	return windowsResult(resultInvalidState)
}

func uiaGetIsSelected(this, result uintptr) uintptr {
	_, node, status := patternNode(this)
	if status != 0 {
		return status
	}
	return writeBool(result, node.IsChosen())
}

// nodesLocked is the nodes of every element, in order. The caller holds tree.mu.
func (tree *uiaTree) nodesLocked() []platform.AccessibilityNode {
	nodes := make([]platform.AccessibilityNode, len(tree.children))
	for index, child := range tree.children {
		nodes[index] = child.Node
	}
	return nodes
}

func uiaGetSelectionContainer(this, result uintptr) uintptr {
	if result == 0 {
		return windowsResult(resultPointer)
	}
	*(*uintptr)(pointer(result)) = 0
	provider, _, status := patternNode(this)
	if status != 0 {
		return status
	}
	tree := provider.Tree
	tree.mu.RLock()
	defer tree.mu.RUnlock()
	if parent := platform.AccessibilityContainer(tree.nodesLocked(), provider.Index); parent >= 0 {
		container := tree.children[parent]
		container.addReference()
		*(*uintptr)(pointer(result)) = container.simplePointer()
	}
	return 0
}

func uiaGetSelection(this, result uintptr) uintptr {
	if result == 0 {
		return windowsResult(resultPointer)
	}
	*(*uintptr)(pointer(result)) = 0
	provider, _, status := patternNode(this)
	if status != 0 {
		return status
	}
	tree := provider.Tree
	tree.mu.RLock()
	nodes := tree.nodesLocked()
	var chosen []uintptr
	for index, node := range nodes {
		if node.IsChosen() && platform.AccessibilityContainer(nodes, index) == provider.Index {
			child := tree.children[index]
			child.addReference()
			chosen = append(chosen, child.simplePointer())
		}
	}
	tree.mu.RUnlock()
	array, ok := unknownArrayData(chosen)
	if !ok {
		return windowsResult(resultOutOfMemory)
	}
	*(*uintptr)(pointer(result)) = array
	return 0
}

func uiaGetCanSelectMultiple(this, result uintptr) uintptr { return writeBool(result, false) }

func uiaGetIsSelectionRequired(this, result uintptr) uintptr { return writeBool(result, false) }

func uiaExpand(this uintptr) uintptr {
	provider, node, status := patternNode(this)
	if status != 0 {
		return status
	}
	if node.Expanded {
		return 0
	}
	return provider.press(node)
}

func uiaCollapse(this uintptr) uintptr {
	provider, node, status := patternNode(this)
	if status != 0 {
		return status
	}
	if !node.Expanded {
		return 0
	}
	return provider.press(node)
}

func uiaGetExpandCollapseState(this, result uintptr) uintptr {
	_, node, status := patternNode(this)
	if status != 0 {
		return status
	}
	return writeInt(result, expandState(node))
}

// uiaRanges keeps the text ranges COM holds alive until they are released.
var uiaRanges sync.Map
