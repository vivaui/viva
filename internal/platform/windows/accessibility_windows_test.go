package windows

import (
	"math"
	"syscall"
	"testing"
	"unsafe"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

func TestUIAStructureABI(t *testing.T) {
	for _, layout := range []struct {
		name string
		size uintptr
		want uintptr
	}{
		{"VARIANT", unsafe.Sizeof(variant{}), 24},
		{"UiaRect", unsafe.Sizeof(uiaRectangle{}), 32},
		{"COM interface tear-off", unsafe.Sizeof(uiaInterface{}), 16},
		{"MEMORY_BASIC_INFORMATION", unsafe.Sizeof(memoryBasicInformation{}), 48},
	} {
		if layout.size != layout.want {
			t.Errorf("%s: size %d, want %d", layout.name, layout.size, layout.want)
		}
	}
	if unsafe.Offsetof(variant{}.Value) != 8 || unsafe.Offsetof(uiaInterface{}.Provider) != 8 {
		t.Fatal("UI Automation member alignment mismatch")
	}
}

func TestUIAControlTypeMapping(t *testing.T) {
	tests := []struct {
		role uint8
		want int32
	}{
		{1, uiaWindowControlType},
		{2, uiaGroupControlType},
		{3, uiaButtonControlType},
		{4, uiaTextControlType},
		{5, uiaEditControlType},
		{6, uiaCheckBoxControlType},
		{7, uiaSliderControlType},
		{8, uiaImageControlType},
		{9, uiaHyperlinkControlType},
		{10, uiaRadioControlType},
		{11, uiaTabItemControlType},
		{12, uiaMenuItemControlType},
		{13, uiaComboBoxControlType},
		{14, uiaCheckBoxControlType},
		{platform.A11yRoleListBox, uiaListControlType},
		{platform.A11yRoleOption, uiaListItemControlType},
		{platform.A11yRoleHeading, uiaTextControlType},
		{platform.A11yRoleSeparator, uiaSeparatorControlType},
		{0, uiaGroupControlType},
		{255, uiaGroupControlType},
	}
	for _, test := range tests {
		if got := controlType(test.role); got != test.want {
			t.Errorf("controlType(%d) = %d, want %d", test.role, got, test.want)
		}
	}
}

func TestUIAQueuesTypedStateChanges(t *testing.T) {
	provider := &uiaProvider{}
	oldNode := platform.AccessibilityNode{}
	newNode := platform.AccessibilityNode{
		Value: "42", Hint: "Current value", Disabled: true,
		Checked: true, Selected: true, Expanded: true, ReadOnly: true,
	}
	events := appendPropertyChanges(nil, provider, oldNode, newNode)
	want := []int32{
		uiaIsEnabledProperty, uiaValueValueProperty, uiaHelpTextProperty,
		uiaValueIsReadOnlyProperty, uiaExpandStateProperty,
		uiaSelectionStateProperty, uiaToggleStateProperty,
	}
	if len(events) != len(want) {
		t.Fatalf("events = %d, want %d: %+v", len(events), len(want), events)
	}
	for index, property := range want {
		if events[index].property != property {
			t.Fatalf("event %d property = %d, want %d", index, events[index].property, property)
		}
	}
}

func TestUIAQueuesOnlyObservablePropertyChanges(t *testing.T) {
	provider := &uiaProvider{}
	oldNode := platform.AccessibilityNode{Role: 2, X: 1, Y: 2, Width: 3, Height: 4, Label: "Old"}
	newNode := platform.AccessibilityNode{Role: 3, X: 5, Y: 6, Width: 7, Height: 8, Label: "New", Focused: true}
	events := appendPropertyChanges(nil, provider, oldNode, newNode)
	want := []int32{uiaNameProperty, uiaControlTypeProperty, uiaHasKeyboardFocusProperty}
	if len(events) != len(want) {
		t.Fatalf("events = %d, want %d", len(events), len(want))
	}
	for index, property := range want {
		if events[index].provider != provider || events[index].property != property {
			t.Errorf("event %d = %+v, want property %d", index, events[index], property)
		}
	}
	for _, event := range events {
		if event.oldNode.X != oldNode.X || event.newNode.X != newNode.X {
			t.Fatalf("event snapshots = %+v", event)
		}
	}
}

func TestUIADoesNotAdvertiseUnsupportedPatterns(t *testing.T) {
	var result uintptr = 1
	if status := uiaGetPatternProvider(0, 10000, uintptr(unsafe.Pointer(&result))); status != 0 {
		t.Fatalf("GetPatternProvider status = %#x", status)
	}
	if result != 0 {
		t.Fatalf("GetPatternProvider returned unsupported provider %#x", result)
	}
}

func TestUIAVariantScalars(t *testing.T) {
	integer := integerVariant(-7)
	if integer.Type != variantI4 || uint32(integer.Value) != uint32(^uint32(6)) {
		t.Fatalf("integer variant = %+v", integer)
	}
	if value := booleanVariant(true); value.Type != variantBool || uint16(value.Value) != 0xffff {
		t.Fatalf("true variant = %+v", value)
	}
	if value := booleanVariant(false); value.Type != variantBool || value.Value != 0 {
		t.Fatalf("false variant = %+v", value)
	}
}

// engagedTree is a tree that a client has already asked for, over nodes.
func engagedTree(t *testing.T, actionable bool, nodes []platform.AccessibilityNode) *uiaTree {
	t.Helper()
	if !initializeUIACallbacks() {
		t.Skip("UI Automation is unsupported on this architecture")
	}
	tree := &uiaTree{supported: true, scale: 1, window: 1, actionable: actionable, nextRuntime: 1}
	tree.root = tree.newProviderLocked(-1, true)
	tree.latest = nodes
	tree.mu.Lock()
	tree.engageLocked()
	tree.mu.Unlock()
	return tree
}

func TestUIABuildsNoElementsUntilAClientAsks(t *testing.T) {
	if !initializeUIACallbacks() {
		t.Skip("UI Automation is unsupported on this architecture")
	}
	tree := &uiaTree{supported: true, scale: 1, nextRuntime: 1}
	tree.root = tree.newProviderLocked(-1, true)
	nodes := []platform.AccessibilityNode{{ID: gfx.Item(1), Role: platform.A11yRoleButton}}
	if err := tree.update(nodes); err != nil {
		t.Fatal(err)
	}
	if len(tree.children) != 0 || tree.engaged {
		t.Fatal("elements were built for an application nobody is automating")
	}
	tree.mu.Lock()
	tree.engageLocked()
	tree.mu.Unlock()
	if len(tree.children) != 1 || tree.children[0].Node.ID != gfx.Item(1) {
		t.Fatalf("children = %d", len(tree.children))
	}
}

func TestUIAElementsFollowTheirControlAsTheListShifts(t *testing.T) {
	tree := engagedTree(t, true, []platform.AccessibilityNode{
		{ID: gfx.Item(1), Role: platform.A11yRoleButton, Label: "A"},
		{ID: gfx.Item(2), Role: platform.A11yRoleButton, Label: "B"},
	})
	first, second := tree.children[0], tree.children[1]
	if err := tree.update([]platform.AccessibilityNode{
		{ID: gfx.Item(3), Role: platform.A11yRoleButton, Label: "C"},
		{ID: gfx.Item(2), Role: platform.A11yRoleButton, Label: "B"},
	}); err != nil {
		t.Fatal(err)
	}
	if tree.children[1] != second || second.Index != 1 {
		t.Error("the element of a control that stayed was replaced")
	}
	if first.Index != -1 {
		t.Error("the element of a control that left is still in the tree")
	}
	if tree.children[0] == first || tree.children[0].Node.ID != gfx.Item(3) {
		t.Error("a new control reused another's element")
	}
}

func TestUIAOffersPatternsFromTheNode(t *testing.T) {
	nodes := []platform.AccessibilityNode{
		{ID: gfx.Item(1), Role: platform.A11yRoleButton},
		{ID: gfx.Item(2), Role: platform.A11yRoleCheckbox},
		{ID: gfx.Item(3), Role: platform.A11yRoleSlider, Range: gfx.ValueRange{Now: 1, Max: 10, Set: true}},
		{ID: gfx.Item(4), Role: platform.A11yRoleTextField, Value: "hello"},
		{Role: platform.A11yRoleHeading},
	}
	tree := engagedTree(t, true, nodes)
	want := [][]uiaPatternKind{
		{patternInvoke},
		{patternToggle},
		{patternRangeValue, patternValue},
		{patternValue, patternText},
		{},
	}
	for index, kinds := range want {
		for kind := uiaPatternKind(0); kind < patternKinds; kind++ {
			offered := tree.children[index].supports(kind)
			expected := false
			for _, candidate := range kinds {
				expected = expected || candidate == kind
			}
			if offered != expected {
				t.Errorf("node %d pattern %d: offered %v, want %v", index, kind, offered, expected)
			}
		}
	}

	// A window with no way back to the application offers nothing that acts.
	passive := engagedTree(t, false, nodes)
	if passive.children[0].supports(patternInvoke) || passive.children[2].supports(patternRangeValue) {
		t.Error("an action pattern is offered without a handler")
	}
	if !passive.children[3].supports(patternText) {
		t.Error("reading text needs no handler")
	}
}

func TestUIAPatternCallsBecomeRequests(t *testing.T) {
	tree := engagedTree(t, true, []platform.AccessibilityNode{
		{ID: gfx.Item(1), Role: platform.A11yRoleButton},
		{ID: gfx.Item(2), Role: platform.A11yRoleButton, Disabled: true},
		{ID: gfx.Item(3), Role: platform.A11yRoleTextField, Value: "x"},
	})
	if status := uiaInvoke(tree.children[0].patternPointer(patternInvoke)); status != 0 {
		t.Fatalf("Invoke = %#x", status)
	}
	if status := uiaInvoke(tree.children[1].patternPointer(patternInvoke)); uint32(status) != resultElementNotEnabled {
		t.Fatalf("Invoke on a disabled element = %#x", status)
	}
	text := []uint16{'h', 'i', 0}
	if status := uiaSetValue(tree.children[2].patternPointer(patternValue), uintptr(unsafe.Pointer(&text[0]))); status != 0 {
		t.Fatalf("SetValue = %#x", status)
	}
	jobs := tree.takeJobs()
	if len(jobs) != 2 || jobs[0].Action != platform.AccessibilityPress || jobs[0].Node != gfx.Item(1) ||
		jobs[1].Action != platform.AccessibilitySetText || jobs[1].Text != "hi" {
		t.Fatalf("jobs = %+v", jobs)
	}
}

func TestUIARangeValueReportsTheNode(t *testing.T) {
	tree := engagedTree(t, true, []platform.AccessibilityNode{
		{ID: gfx.Item(1), Role: platform.A11yRoleSlider, Range: gfx.ValueRange{Now: 4, Min: 0, Max: 20, Step: 2, Set: true}},
	})
	this := tree.children[0].patternPointer(patternRangeValue)
	var value float64
	for _, check := range []struct {
		call func(uintptr, uintptr) uintptr
		want float64
	}{{uiaGetRangeValue, 4}, {uiaGetRangeMinimum, 0}, {uiaGetRangeMaximum, 20}, {uiaGetRangeSmallChange, 2}} {
		if status := check.call(this, uintptr(unsafe.Pointer(&value))); status != 0 || value != check.want {
			t.Errorf("got %v (status %#x), want %v", value, status, check.want)
		}
	}
	if status := uiaRangeSetValueBits(this, uintptr(math.Float64bits(50))); uint32(status) != resultInvalidArgument {
		t.Errorf("SetValue out of range = %#x", status)
	}
}

func TestUIATextRangesMoveByUnitAndSelect(t *testing.T) {
	tree := engagedTree(t, true, []platform.AccessibilityNode{
		{ID: gfx.Item(1), Role: platform.A11yRoleTextField, Value: "one two three", Selection: platform.AccessibilitySelection{Start: 4, End: 4, Valid: true}},
	})
	provider := tree.children[0]
	var rangePointer uintptr
	if status := uiaTextGetDocumentRange(provider.patternPointer(patternText), uintptr(unsafe.Pointer(&rangePointer))); status != 0 || rangePointer == 0 {
		t.Fatalf("DocumentRange = %#x", status)
	}
	var moved int32
	if status := uiaRangeMove(rangePointer, 2, 1, uintptr(unsafe.Pointer(&moved))); status != 0 || moved != 1 {
		t.Fatalf("Move = %#x, moved %d", status, moved)
	}
	var text uintptr
	if status := uiaRangeGetText(rangePointer, ^uintptr(0), uintptr(unsafe.Pointer(&text))); status != 0 || utf16Pointer(text) != "two " {
		t.Fatalf("GetText = %#x %q", status, utf16Pointer(text))
	}
	if status := uiaRangeSelect(rangePointer); status != 0 {
		t.Fatalf("Select = %#x", status)
	}
	jobs := tree.takeJobs()
	if len(jobs) != 1 || jobs[0].Action != platform.AccessibilitySetSelection || jobs[0].Start != 4 || jobs[0].End != 8 {
		t.Fatalf("jobs = %+v", jobs)
	}
	uiaTextRangeRelease(rangePointer)
	if rangeAt(rangePointer) != nil {
		t.Error("a released range is still kept")
	}
}

func TestUIAPatternTablesHaveEveryFunction(t *testing.T) {
	if !initializeUIACallbacks() {
		t.Skip("UI Automation is unsupported on this architecture")
	}
	wantMethods := [patternKinds]int{
		patternInvoke: 1, patternToggle: 2, patternValue: 3, patternRangeValue: 7,
		patternSelectionItem: 5, patternSelection: 3, patternExpandCollapse: 3, patternText: 6,
	}
	for kind, methods := range wantMethods {
		table := uiaPatternTables[kind]
		if len(table) != 3+methods {
			t.Errorf("pattern %d: %d functions, want %d", kind, len(table)-3, methods)
		}
		for index, entry := range table {
			if entry == 0 {
				t.Errorf("pattern %d: function %d is nil", kind, index)
			}
		}
		for other := kind + 1; other < int(patternKinds); other++ {
			if uiaPatternIIDs[kind] == uiaPatternIIDs[other] || uiaPatternIDs[kind] == uiaPatternIDs[other] {
				t.Errorf("patterns %d and %d share an identity", kind, other)
			}
		}
	}
	for index, entry := range uiaTextRangeTable {
		if entry == 0 {
			t.Errorf("text range function %d is nil", index)
		}
	}
}

// Until a client asks, a frame costs the tree one slice header.
func TestUIAUpdateCostsNothingUntilAClientAsks(t *testing.T) {
	if !initializeUIACallbacks() {
		t.Skip("UI Automation is unsupported on this architecture")
	}
	tree := &uiaTree{supported: true, scale: 1, nextRuntime: 1}
	tree.root = tree.newProviderLocked(-1, true)
	nodes := make([]platform.AccessibilityNode, 200)
	allocations := testing.AllocsPerRun(100, func() { tree.update(nodes) })
	if allocations != 0 {
		t.Fatalf("%v allocations per frame with no client", allocations)
	}
	if len(tree.children) != 0 {
		t.Fatal("elements exist with no client")
	}
}

func TestUIANavigateRejectsAnUnwritableResult(t *testing.T) {
	var result uintptr
	if status := uiaNavigate(0, 0, uintptr(unsafe.Pointer(&result))); uint32(status) != resultElementNotAvailable {
		t.Fatalf("valid result without a provider: status %#x, want element not available", status)
	}
	if status := uiaNavigate(0, 0, ^uintptr(0)); uint32(status) != resultPointer {
		t.Fatalf("unwritable result: status %#x, want E_POINTER", status)
	}
}

// The window's element offers IRawElementProviderFragmentRoot, whose table is
// IUnknown followed by ElementProviderFromPoint and GetFocus: it does not
// repeat the fragment's methods. A client that calls slot 4 of that interface
// means GetFocus. Handing out one eleven-entry table for both interfaces made
// UI Automation's call to ElementProviderFromPoint run Navigate with whatever
// the registers held, which wrote through a stale pointer into the heap.
func TestUIAFragmentRootHasItsOwnInterfaceTable(t *testing.T) {
	tree := engagedTree(t, true, []platform.AccessibilityNode{{ID: gfx.Item(1), Role: platform.A11yRoleButton}})
	root := tree.root
	simple := root.simplePointer()
	query := func(iid guid) uintptr {
		var out uintptr
		status, _, _ := syscall.SyscallN(vtable(simple, 0), simple, uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&out)))
		if status != 0 {
			t.Fatalf("QueryInterface failed: %#x", status)
		}
		return out
	}
	fragment := query(iidRawElementProviderFragment)
	fragmentRoot := query(iidRawElementProviderFragmentRoot)
	if fragment == fragmentRoot {
		t.Fatal("the fragment and the fragment root are the same interface pointer")
	}
	if fragment != root.fragmentPointer() || fragmentRoot != root.fragmentRootPointer() {
		t.Fatal("QueryInterface did not return the element's own interfaces")
	}

	var focused uintptr
	if status, _, _ := syscall.SyscallN(vtable(fragmentRoot, 4), fragmentRoot, uintptr(unsafe.Pointer(&focused))); status != 0 {
		t.Fatalf("GetFocus through the fragment root: %#x", status)
	}
	if focused != root.fragmentPointer() {
		t.Errorf("GetFocus returned %#x, want the root's IRawElementProviderFragment %#x", focused, root.fragmentPointer())
	}

	var gotRoot uintptr
	if status, _, _ := syscall.SyscallN(vtable(fragment, 8), fragment, uintptr(unsafe.Pointer(&gotRoot))); status != 0 {
		t.Fatalf("get_FragmentRoot: %#x", status)
	}
	if gotRoot != root.fragmentRootPointer() {
		t.Errorf("get_FragmentRoot returned %#x, want an IRawElementProviderFragmentRoot %#x", gotRoot, root.fragmentRootPointer())
	}

	// Elements are handed back as fragments, the window's own included.
	var first uintptr
	syscall.SyscallN(vtable(fragment, 3), fragment, uintptr(uiaNavigateFirstChild), uintptr(unsafe.Pointer(&first)))
	if first != tree.children[0].fragmentPointer() {
		t.Errorf("Navigate to the first child returned %#x, want %#x", first, tree.children[0].fragmentPointer())
	}
}
