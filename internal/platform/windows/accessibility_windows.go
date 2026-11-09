package windows

import (
	"math"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

const (
	uiaRootObjectID = -25

	// ProviderOptions_ServerSideProvider. 0x1 is the client-side option, which
	// makes UI Automation drop every event the provider raises.
	providerOptionsServerSide      = 0x2
	providerOptionsUseCOMThreading = 0x20

	uiaNavigateParent          = 0
	uiaNavigateNextSibling     = 1
	uiaNavigatePreviousSibling = 2
	uiaNavigateFirstChild      = 3
	uiaNavigateLastChild       = 4

	uiaControlTypeProperty         = 30003
	uiaAutomationIDProperty        = 30011
	uiaNameProperty                = 30005
	uiaHasKeyboardFocusProperty    = 30008
	uiaIsKeyboardFocusableProperty = 30009
	uiaIsEnabledProperty           = 30010
	uiaClassNameProperty           = 30012
	uiaHelpTextProperty            = 30013
	uiaIsControlElementProperty    = 30016
	uiaIsContentElementProperty    = 30017
	uiaIsPasswordProperty          = 30019
	uiaIsOffscreenProperty         = 30022
	uiaFrameworkIDProperty         = 30024
	uiaValueValueProperty          = 30045
	uiaValueIsReadOnlyProperty     = 30046
	uiaRangeValueValueProperty     = 30047
	uiaExpandStateProperty         = 30070
	uiaSelectionStateProperty      = 30079
	uiaToggleStateProperty         = 30086
	uiaPositionInSetProperty       = 30152
	uiaSizeOfSetProperty           = 30153
	uiaLiveSettingProperty         = 30135
	uiaFullDescriptionProperty     = 30159
	uiaHeadingLevelProperty        = 30173

	// UIA_HeadingLevel1Id is 80051; "none" is one below it.
	uiaHeadingLevelNone = 80050

	uiaFocusChangedEvent         = 20005
	uiaTextSelectionChangedEvent = 20014
	uiaTextChangedEvent          = 20015
	uiaLiveRegionChangedEvent    = 20024

	uiaButtonControlType    = 50000
	uiaCheckBoxControlType  = 50002
	uiaComboBoxControlType  = 50003
	uiaEditControlType      = 50004
	uiaHyperlinkControlType = 50005
	uiaImageControlType     = 50006
	uiaListItemControlType  = 50007
	uiaListControlType      = 50008
	uiaMenuControlType      = 50009
	uiaMenuBarControlType   = 50010
	uiaMenuItemControlType  = 50011
	uiaRadioControlType     = 50013
	uiaSliderControlType    = 50015
	uiaTabItemControlType   = 50019
	uiaTextControlType      = 50020
	uiaGroupControlType     = 50026
	uiaDataItemControlType  = 50029
	uiaWindowControlType    = 50032
	uiaSeparatorControlType = 50038

	variantEmpty = 0
	variantI4    = 3
	variantR8    = 5
	variantBSTR  = 8
	variantBool  = 11
	variantArray = 0x2000

	// The element types of the SAFEARRAYs this file hands to UI Automation.
	variantUnknown = 13

	structureChildrenInvalidated = 2
	structureChildrenReordered   = 5
	uiaAppendRuntimeID           = 3

	resultNoInterface         = 0x80004002
	resultPointer             = 0x80004003
	resultInvalidArgument     = 0x80070057
	resultOutOfMemory         = 0x8007000e
	resultElementNotEnabled   = 0x80040200
	resultElementNotAvailable = 0x80040201
	resultNotSupported        = 0x80040204

	// wmAccessibility carries a request from a UI Automation client thread to
	// the window thread, which owns the application's state.
	wmAccessibility = 0x8000 + 0x40
)

var (
	iidIUnknown                       = guid{Data4: [8]byte{0xc0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidRawElementProviderSimple       = guid{0xd6dd68d1, 0x86fd, 0x4332, [8]byte{0x86, 0x66, 0x9a, 0xbe, 0xde, 0xa2, 0xd2, 0x4c}}
	iidRawElementProviderFragment     = guid{0xf7063da8, 0x8359, 0x439c, [8]byte{0x92, 0x97, 0xbb, 0xc5, 0x29, 0x9a, 0x7d, 0x87}}
	iidRawElementProviderFragmentRoot = guid{0x620ce2a5, 0xab8f, 0x40a9, [8]byte{0x86, 0xcb, 0xde, 0x3c, 0x75, 0x59, 0x9b, 0x58}}
)

type variant struct {
	Type     uint16
	Reserved [3]uint16
	Value    uint64
	Extra    uint64
}

type uiaRectangle struct {
	X, Y, Width, Height float64
}

type uiaInterface struct {
	Table    *uintptr
	Provider *uiaProvider
}

type uiaProvider struct {
	Simple       uiaInterface
	Fragment     uiaInterface
	FragmentRoot uiaInterface
	References   atomic.Uint32
	Tree         *uiaTree
	Index        int
	RuntimeID    int32
	Root         bool
	Node         platform.AccessibilityNode
	// Patterns are the control pattern interfaces, made the first time a client
	// asks for one: most elements are never asked.
	Patterns atomic.Pointer[[patternKinds]uiaInterface]
}

type uiaTree struct {
	mu          sync.RWMutex
	window      uintptr
	title       string
	scale       float32
	closed      bool
	supported   bool
	actionable  bool
	nextRuntime int32
	root        *uiaProvider
	children    []*uiaProvider
	byID        map[gfx.ID]*uiaProvider

	// engaged is set when a client first asks for the window's elements. Until
	// then the tree is only the application's newest nodes, kept by reference
	// (the window thread that stores them is the one that reads them), and an
	// application nobody is automating pays for nothing else.
	engaged bool
	latest  []platform.AccessibilityNode

	jobMu sync.Mutex
	jobs  []platform.AccessibilityRequest
}

type uiaPropertyChange struct {
	provider *uiaProvider
	property int32
	oldNode  platform.AccessibilityNode
	newNode  platform.AccessibilityNode
}

// uiaEvent is an automation event to raise after the tree is updated.
type uiaEvent struct {
	provider *uiaProvider
	event    int32
}

var uiaCore = systemDLL("UIAutomationCore.dll")
var uiaReturnRawElementProvider = uiaCore.NewProc("UiaReturnRawElementProvider")
var uiaHostProviderFromWindow = uiaCore.NewProc("UiaHostProviderFromHwnd")
var uiaRaiseAutomationEvent = uiaCore.NewProc("UiaRaiseAutomationEvent")
var uiaRaisePropertyChanged = uiaCore.NewProc("UiaRaiseAutomationPropertyChangedEvent")
var uiaRaiseStructureChanged = uiaCore.NewProc("UiaRaiseStructureChangedEvent")
var uiaDisconnectProvider = uiaCore.NewProc("UiaDisconnectProvider")
var uiaClientsAreListening = uiaCore.NewProc("UiaClientsAreListening")
var uiaGetReservedNotSupportedValue = uiaCore.NewProc("UiaGetReservedNotSupportedValue")

var automation = systemDLL("oleaut32.dll")
var safeArrayCreateVector = automation.NewProc("SafeArrayCreateVector")
var safeArrayAccessData = automation.NewProc("SafeArrayAccessData")
var safeArrayUnaccessData = automation.NewProc("SafeArrayUnaccessData")
var systemAllocateString = automation.NewProc("SysAllocStringLen")
var variantClear = automation.NewProc("VariantClear")

var virtualQuery = systemDLL("kernel32.dll").NewProc("VirtualQuery")

const (
	memCommit            = 0x1000
	pageReadWrite        = 0x04
	pageWriteCopy        = 0x08
	pageExecuteReadWrite = 0x40
	pageExecuteWriteCopy = 0x80
	pageGuard            = 0x100
)

// memoryBasicInformation is MEMORY_BASIC_INFORMATION. Windows 10 1903 and
// later put PartitionId between AllocationProtect and RegionSize; the size is
// 48 bytes on amd64 and arm64 either way.
type memoryBasicInformation struct {
	BaseAddress       uintptr
	AllocationBase    uintptr
	AllocationProtect uint32
	PartitionID       uint16
	RegionSize        uintptr
	State             uint32
	Protect           uint32
	Type              uint32
}

// writableMemory reports whether size bytes at address are committed and may be
// written. UI Automation passes the out-pointers of provider callbacks; a
// corrupted or foreign caller can pass an address this process cannot write,
// and a Go callback cannot recover from the resulting access violation because
// it runs inside a libcall (Go's canpanic is false there). Checking the page
// turns that process-ending fault into the E_POINTER the caller should receive.
func writableMemory(address, size uintptr) bool {
	if address == 0 || size == 0 {
		return false
	}
	var information memoryBasicInformation
	if result, _, _ := virtualQuery.Call(address, uintptr(unsafe.Pointer(&information)), unsafe.Sizeof(information)); result == 0 {
		return false
	}
	if information.State != memCommit || information.Protect&pageGuard != 0 {
		return false
	}
	switch information.Protect & 0xff {
	case pageReadWrite, pageWriteCopy, pageExecuteReadWrite, pageExecuteWriteCopy:
	default:
		return false
	}
	offset := address - information.BaseAddress
	return offset < information.RegionSize && size <= information.RegionSize-offset
}

var getWindowRectangle = user.NewProc("GetWindowRect")
var clientToScreen = user.NewProc("ClientToScreen")

// uiaFault names the boundary of a fault in a UI Automation callback.
const uiaFault = "UI Automation provider"

var uiaCallbackOnce sync.Once
var uiaSimpleTable [7]uintptr
var uiaFragmentTable [9]uintptr
var uiaFragmentRootTable [5]uintptr
var uiaProviders sync.Map
var uiaFromPointCallback uintptr

func initializeUIACallbacks() bool {
	// VIVA_ACCESSIBILITY=0 leaves the window without a UI Automation tree, as on
	// Linux: a way to tell whether an assistive client is what a fault depends on.
	if os.Getenv("VIVA_ACCESSIBILITY") == "0" {
		return false
	}
	entry := uiaFromPointThunkAddress()
	if entry == 0 {
		return false
	}
	uiaCallbackOnce.Do(func() {
		unknown := [...]uintptr{
			guarded3(uiaFault, uiaQueryInterface),
			guarded1(uiaFault, uiaAddReference),
			guarded1(uiaFault, uiaReleaseReference),
		}
		copy(uiaSimpleTable[:3], unknown[:])
		uiaSimpleTable[3] = guarded2(uiaFault, uiaGetProviderOptions)
		uiaSimpleTable[4] = guarded3(uiaFault, uiaGetPatternProvider)
		uiaSimpleTable[5] = guarded3(uiaFault, uiaGetPropertyValue)
		uiaSimpleTable[6] = guarded2(uiaFault, uiaGetHostProvider)

		copy(uiaFragmentTable[:3], unknown[:])
		uiaFragmentTable[3] = guarded3(uiaFault, uiaNavigate)
		uiaFragmentTable[4] = guarded2(uiaFault, uiaGetRuntimeID)
		uiaFragmentTable[5] = guarded2(uiaFault, uiaGetBoundingRectangle)
		uiaFragmentTable[6] = guarded2(uiaFault, uiaGetEmbeddedRoots)
		uiaFragmentTable[7] = guarded1(uiaFault, uiaSetFocus)
		uiaFragmentTable[8] = guarded2(uiaFault, uiaGetFragmentRoot)

		// IRawElementProviderFragmentRoot derives from IUnknown, not from
		// IRawElementProviderFragment: its two methods follow the IUnknown ones
		// directly, so a client that calls slot 3 means ElementProviderFromPoint.
		copy(uiaFragmentRootTable[:3], unknown[:])
		uiaFromPointCallback = guarded4(uiaFault, uiaElementProviderFromPointBits)
		uiaFragmentRootTable[3] = entry
		uiaFragmentRootTable[4] = guarded2(uiaFault, uiaGetFocus)

		initializePatternTables(unknown[:])
	})
	return true
}

func (tree *uiaTree) initialize(state *windowState, nodes []platform.AccessibilityNode) error {
	if !initializeUIACallbacks() {
		// Where the callbacks cannot be built (arm64: the floating-point thunk
		// exists for amd64 only) the window simply has no UI Automation tree.
		// It is a missing capability, not a reason to refuse to open.
		return nil
	}
	tree.mu.Lock()
	tree.window = state.window
	tree.title = state.config.Title
	tree.scale = state.scale
	tree.supported = true
	tree.actionable = state.config.OnAccessibility != nil
	tree.nextRuntime = 1
	tree.root = tree.newProviderLocked(-1, true)
	tree.latest = nodes
	tree.mu.Unlock()
	return nil
}

// engageLocked builds the elements from the newest nodes. It runs on the first
// request from a client, on the window thread.
func (tree *uiaTree) engageLocked() {
	if tree.engaged {
		return
	}
	tree.engaged = true
	tree.byID = make(map[gfx.ID]*uiaProvider, len(tree.latest))
	for index, node := range tree.latest {
		provider := tree.newProviderLocked(index, false)
		provider.Node = node
		tree.children = append(tree.children, provider)
		if !node.ID.IsZero() {
			tree.byID[node.ID] = provider
		}
	}
	tree.latest = nil
}

func (tree *uiaTree) newProviderLocked(index int, root bool) *uiaProvider {
	tree.nextRuntime++
	provider := &uiaProvider{Tree: tree, Index: index, RuntimeID: tree.nextRuntime, Root: root}
	provider.Simple = uiaInterface{Table: &uiaSimpleTable[0], Provider: provider}
	provider.Fragment = uiaInterface{Table: &uiaFragmentTable[0], Provider: provider}
	provider.FragmentRoot = uiaInterface{Table: &uiaFragmentRootTable[0], Provider: provider}
	provider.References.Store(1)
	uiaProviders.Store(uintptr(unsafe.Pointer(provider)), provider)
	return provider
}

// snapshot returns the node of a live element, or false when the element is
// gone, the tree is closed, or the provider is the window itself.
func (provider *uiaProvider) snapshot() (platform.AccessibilityNode, bool) {
	tree := provider.Tree
	if tree == nil {
		return platform.AccessibilityNode{}, false
	}
	tree.mu.RLock()
	defer tree.mu.RUnlock()
	if tree.closed || provider.Root || provider.Index < 0 || provider.Index >= len(tree.children) || tree.children[provider.Index] != provider {
		return platform.AccessibilityNode{}, false
	}
	return provider.Node, true
}

func (tree *uiaTree) update(nodes []platform.AccessibilityNode) error {
	tree.mu.Lock()
	if tree.closed {
		tree.mu.Unlock()
		return nil
	}
	if !tree.supported {
		tree.mu.Unlock()
		return nil
	}
	if !tree.engaged {
		tree.latest = nodes
		tree.mu.Unlock()
		return nil
	}

	// Elements follow the control they stand for, not their place in the list,
	// so a client holding one keeps it as the list shifts. A node with no ID
	// (a region) takes the next such element in order.
	previous := tree.children
	var anonymous []*uiaProvider
	for _, provider := range previous {
		if provider.Node.ID.IsZero() {
			anonymous = append(anonymous, provider)
		}
	}
	pool := tree.byID
	tree.byID = make(map[gfx.ID]*uiaProvider, len(nodes))
	children := make([]*uiaProvider, 0, len(nodes))
	var changes []uiaPropertyChange
	var events []uiaEvent
	structureChanged := len(nodes) != len(previous)
	for index, node := range nodes {
		var provider *uiaProvider
		if node.ID.IsZero() {
			if len(anonymous) != 0 {
				provider, anonymous = anonymous[0], anonymous[1:]
			}
		} else if found, ok := pool[node.ID]; ok {
			provider = found
			delete(pool, node.ID)
		}
		if provider == nil {
			provider = tree.newProviderLocked(index, false)
			provider.Node = node
			structureChanged = true
			if node.Focused {
				events = append(events, uiaEvent{provider, uiaFocusChangedEvent})
			}
		} else {
			oldNode := provider.Node
			if provider.Index != index {
				structureChanged = true
			}
			provider.Index = index
			provider.Node = node
			changes = appendPropertyChanges(changes, provider, oldNode, node)
			events = appendNodeEvents(events, provider, oldNode, node)
		}
		children = append(children, provider)
		if !node.ID.IsZero() {
			tree.byID[node.ID] = provider
		}
	}
	var removed []*uiaProvider
	for _, provider := range pool {
		removed = append(removed, provider)
	}
	removed = append(removed, anonymous...)
	for _, provider := range removed {
		provider.Index = -1
	}
	if len(removed) != 0 {
		structureChanged = true
	}
	tree.children = children
	root := tree.root
	tree.mu.Unlock()

	if uiaClientsAreListening.Find() == nil {
		if listening, _, _ := uiaClientsAreListening.Call(); listening == 0 {
			// Nobody listens for events: the elements are kept current, and
			// there is nothing to say to anyone.
			for _, provider := range removed {
				tree.disconnect(provider)
			}
			return nil
		}
	}
	for _, change := range changes {
		tree.raisePropertyChange(change)
	}
	for _, event := range events {
		if uiaRaiseAutomationEvent.Find() == nil {
			uiaRaiseAutomationEvent.Call(event.provider.simplePointer(), uintptr(event.event))
		}
	}
	if structureChanged && root != nil && uiaRaiseStructureChanged.Find() == nil {
		uiaRaiseStructureChanged.Call(root.simplePointer(), structureChildrenInvalidated, 0, 0)
	}
	for _, provider := range removed {
		tree.disconnect(provider)
	}
	return nil
}

// appendNodeEvents adds the automation events a change of node calls for that
// are not property changes: the keyboard arriving, text and its selection
// changing, a live region speaking.
func appendNodeEvents(events []uiaEvent, provider *uiaProvider, oldNode, newNode platform.AccessibilityNode) []uiaEvent {
	changed := platform.DiffAccessibility(oldNode, newNode)
	if !oldNode.Focused && newNode.Focused {
		events = append(events, uiaEvent{provider, uiaFocusChangedEvent})
	}
	if newNode.Patterns().Has(platform.PatternText) {
		if changed&platform.ChangedValue != 0 {
			events = append(events, uiaEvent{provider, uiaTextChangedEvent})
		}
		if changed&platform.ChangedSelection != 0 {
			events = append(events, uiaEvent{provider, uiaTextSelectionChangedEvent})
		}
	}
	if newNode.Live != gfx.LiveOff && changed&(platform.ChangedName|platform.ChangedValue) != 0 {
		events = append(events, uiaEvent{provider, uiaLiveRegionChangedEvent})
	}
	return events
}

// propertyOrder is the order property changes are raised in, with the node
// differences that call for each.
var propertyOrder = [...]struct {
	property int32
	changed  platform.AccessibilityChange
}{
	{uiaNameProperty, platform.ChangedName},
	{uiaControlTypeProperty, platform.ChangedRole},
	{uiaHasKeyboardFocusProperty, platform.ChangedFocus},
	{uiaIsEnabledProperty, platform.ChangedEnabled},
	{uiaValueValueProperty, platform.ChangedValue},
	{uiaHelpTextProperty, platform.ChangedHint},
	{uiaValueIsReadOnlyProperty, platform.ChangedReadOnly},
	{uiaIsPasswordProperty, platform.ChangedPassword},
	{uiaExpandStateProperty, platform.ChangedExpand},
	{uiaSelectionStateProperty, platform.ChangedSelected},
	{uiaToggleStateProperty, platform.ChangedToggle},
	{uiaFullDescriptionProperty, platform.ChangedDescription},
	{uiaRangeValueValueProperty, platform.ChangedRange},
	{uiaHeadingLevelProperty, platform.ChangedLevel},
	{uiaPositionInSetProperty, platform.ChangedPosition},
	{uiaSizeOfSetProperty, platform.ChangedPosition},
	{uiaLiveSettingProperty, platform.ChangedLive},
}

func appendPropertyChanges(changes []uiaPropertyChange, provider *uiaProvider, oldNode, newNode platform.AccessibilityNode) []uiaPropertyChange {
	changed := platform.DiffAccessibility(oldNode, newNode)
	for _, entry := range propertyOrder {
		if changed&entry.changed != 0 {
			changes = append(changes, uiaPropertyChange{provider: provider, property: entry.property, oldNode: oldNode, newNode: newNode})
		}
	}
	return changes
}

func (tree *uiaTree) raisePropertyChange(change uiaPropertyChange) {
	if uiaRaisePropertyChanged.Find() != nil {
		return
	}
	oldValue, oldOK := propertyVariant(change.property, change.oldNode)
	newValue, newOK := propertyVariant(change.property, change.newNode)
	if !oldOK || !newOK {
		clearVariant(&oldValue)
		clearVariant(&newValue)
		return
	}
	uiaRaisePropertyChanged.Call(change.provider.simplePointer(), uintptr(change.property), uintptr(unsafe.Pointer(&oldValue)), uintptr(unsafe.Pointer(&newValue)))
	clearVariant(&oldValue)
	clearVariant(&newValue)
}

func (tree *uiaTree) setScale(scale float32) {
	tree.mu.Lock()
	tree.scale = scale
	tree.mu.Unlock()
}

func (tree *uiaTree) close() {
	tree.mu.Lock()
	if tree.closed {
		tree.mu.Unlock()
		return
	}
	tree.closed = true
	window := tree.window
	root := tree.root
	children := tree.children
	tree.window = 0
	tree.root = nil
	tree.children = nil
	tree.latest = nil
	tree.byID = nil
	for _, provider := range children {
		provider.Index = -1
	}
	tree.mu.Unlock()

	if window != 0 && uiaReturnRawElementProvider.Find() == nil {
		uiaReturnRawElementProvider.Call(window, 0, 0, 0)
	}
	if root != nil {
		tree.disconnect(root)
	}
	for _, provider := range children {
		tree.disconnect(provider)
	}
}

func (tree *uiaTree) disconnect(provider *uiaProvider) {
	if uiaDisconnectProvider.Find() == nil {
		uiaDisconnectProvider.Call(provider.simplePointer())
	}
	provider.release()
}

func (tree *uiaTree) returnRawProvider(window, first, second uintptr) (uintptr, bool) {
	if int32(second) != uiaRootObjectID || uiaReturnRawElementProvider.Find() != nil {
		return 0, false
	}
	tree.mu.Lock()
	root := tree.root
	closed := tree.closed
	if !closed && root != nil {
		tree.engageLocked()
	}
	tree.mu.Unlock()
	if closed || root == nil {
		return 0, false
	}
	result, _, _ := uiaReturnRawElementProvider.Call(window, first, second, root.simplePointer())
	return result, true
}

// enqueue hands a request to the window thread. It is called on whatever
// thread UI Automation calls a pattern from.
func (tree *uiaTree) enqueue(request platform.AccessibilityRequest) uintptr {
	tree.mu.RLock()
	window, closed, actionable := tree.window, tree.closed, tree.actionable
	tree.mu.RUnlock()
	if closed || window == 0 {
		return windowsResult(resultElementNotAvailable)
	}
	if !actionable {
		return windowsResult(resultNotSupported)
	}
	tree.jobMu.Lock()
	tree.jobs = append(tree.jobs, request)
	tree.jobMu.Unlock()
	postMessage.Call(window, wmAccessibility, 0, 0)
	return 0
}

func (tree *uiaTree) takeJobs() []platform.AccessibilityRequest {
	tree.jobMu.Lock()
	jobs := tree.jobs
	tree.jobs = nil
	tree.jobMu.Unlock()
	return jobs
}

// runAccessibility carries out the requests clients have made, on the window
// thread, and presents what they change.
func (state *windowState) runAccessibility() {
	handler := state.config.OnAccessibility
	for _, request := range state.accessibility.takeJobs() {
		if handler == nil {
			continue
		}
		_, err := handler(request, func(frame platform.Frame) error {
			state.frame(frame, nil)
			return state.err
		})
		if err != nil {
			state.fail(err)
			return
		}
	}
}

func (provider *uiaProvider) simplePointer() uintptr {
	return uintptr(unsafe.Pointer(&provider.Simple))
}

// fragmentPointer is the element's IRawElementProviderFragment, the type of every
// element UI Automation is handed back, the window's own included.
func (provider *uiaProvider) fragmentPointer() uintptr {
	return uintptr(unsafe.Pointer(&provider.Fragment))
}

// fragmentRootPointer is the window element's IRawElementProviderFragmentRoot,
// a separate interface with its own table.
func (provider *uiaProvider) fragmentRootPointer() uintptr {
	return uintptr(unsafe.Pointer(&provider.FragmentRoot))
}

func (provider *uiaProvider) addReference() uint32 {
	return provider.References.Add(1)
}

func (provider *uiaProvider) release() uint32 {
	for {
		current := provider.References.Load()
		if current == 0 {
			return 0
		}
		if provider.References.CompareAndSwap(current, current-1) {
			if current == 1 {
				uiaProviders.Delete(uintptr(unsafe.Pointer(provider)))
			}
			return current - 1
		}
	}
}

func providerFromInterface(value uintptr) *uiaProvider {
	if value == 0 {
		return nil
	}
	return (*uiaInterface)(pointer(value)).Provider
}

func uiaQueryInterface(this, requested, result uintptr) uintptr {
	if result == 0 {
		return windowsResult(resultPointer)
	}
	*(*uintptr)(pointer(result)) = 0
	provider := providerFromInterface(this)
	if provider == nil || requested == 0 {
		return windowsResult(resultNoInterface)
	}
	id := *(*guid)(pointer(requested))
	var selected uintptr
	switch id {
	case iidIUnknown, iidRawElementProviderSimple:
		selected = provider.simplePointer()
	case iidRawElementProviderFragment:
		selected = provider.fragmentPointer()
	case iidRawElementProviderFragmentRoot:
		if provider.Root {
			selected = provider.fragmentRootPointer()
		}
	default:
		if kind, ok := patternKindForIID(id); ok && provider.supports(kind) {
			selected = provider.patternPointer(kind)
		}
	}
	if selected == 0 {
		return windowsResult(resultNoInterface)
	}
	provider.addReference()
	*(*uintptr)(pointer(result)) = selected
	return 0
}

func uiaAddReference(this uintptr) uintptr {
	provider := providerFromInterface(this)
	if provider == nil {
		return 0
	}
	return uintptr(provider.addReference())
}

func uiaReleaseReference(this uintptr) uintptr {
	provider := providerFromInterface(this)
	if provider == nil {
		return 0
	}
	return uintptr(provider.release())
}

func uiaGetProviderOptions(this, result uintptr) uintptr {
	if result == 0 {
		return windowsResult(resultPointer)
	}
	*(*int32)(pointer(result)) = providerOptionsServerSide | providerOptionsUseCOMThreading
	return 0
}

func uiaGetPatternProvider(this, pattern, result uintptr) uintptr {
	if result == 0 {
		return windowsResult(resultPointer)
	}
	*(*uintptr)(pointer(result)) = 0
	provider := providerFromInterface(this)
	if provider == nil {
		return 0
	}
	kind, ok := patternKindForID(int32(pattern))
	if !ok || !provider.supports(kind) {
		return 0
	}
	provider.addReference()
	*(*uintptr)(pointer(result)) = provider.patternPointer(kind)
	return 0
}

func uiaGetPropertyValue(this, property, result uintptr) uintptr {
	if result == 0 {
		return windowsResult(resultPointer)
	}
	output := (*variant)(pointer(result))
	*output = variant{Type: variantEmpty}
	provider := providerFromInterface(this)
	if provider == nil || provider.Tree == nil {
		return windowsResult(resultElementNotAvailable)
	}
	tree := provider.Tree
	var node platform.AccessibilityNode
	var title string
	if provider.Root {
		tree.mu.RLock()
		closed := tree.closed
		title = tree.title
		tree.mu.RUnlock()
		if closed {
			return windowsResult(resultElementNotAvailable)
		}
	} else {
		var ok bool
		if node, ok = provider.snapshot(); !ok {
			return windowsResult(resultElementNotAvailable)
		}
	}

	var value variant
	ok := true
	switch int32(property) {
	case uiaControlTypeProperty:
		if provider.Root {
			value = integerVariant(uiaWindowControlType)
		} else {
			value = integerVariant(controlType(node.Role))
		}
	case uiaNameProperty:
		if provider.Root {
			value, ok = stringVariant(title)
		} else {
			value, ok = stringVariant(node.Label)
		}
	case uiaHasKeyboardFocusProperty:
		value = booleanVariant(!provider.Root && node.Focused)
	case uiaIsEnabledProperty:
		value = booleanVariant(provider.Root || !node.Disabled)
	case uiaIsKeyboardFocusableProperty:
		value = booleanVariant(!provider.Root && node.Focusable && !node.Disabled)
	case uiaIsOffscreenProperty:
		value = booleanVariant(false)
	case uiaIsControlElementProperty, uiaIsContentElementProperty:
		value = booleanVariant(true)
	case uiaClassNameProperty:
		if provider.Root {
			value, ok = stringVariant("VivaNativeWindow")
		} else {
			value, ok = stringVariant("VivaElement")
		}
	case uiaFrameworkIDProperty:
		value, ok = stringVariant("Viva")
	default:
		if provider.Root {
			ok = false
		} else {
			value, ok = propertyVariant(int32(property), node)
		}
	}
	if ok {
		*output = value
	}
	return 0
}

func uiaGetHostProvider(this, result uintptr) uintptr {
	if result == 0 {
		return windowsResult(resultPointer)
	}
	*(*uintptr)(pointer(result)) = 0
	provider := providerFromInterface(this)
	if provider == nil || !provider.Root || provider.Tree == nil || uiaHostProviderFromWindow.Find() != nil {
		return 0
	}
	provider.Tree.mu.RLock()
	window := provider.Tree.window
	closed := provider.Tree.closed
	provider.Tree.mu.RUnlock()
	if closed || window == 0 {
		return windowsResult(resultElementNotAvailable)
	}
	status, _, _ := uiaHostProviderFromWindow.Call(window, result)
	return status
}

func uiaNavigate(this, direction, result uintptr) uintptr {
	if !writableMemory(result, unsafe.Sizeof(uintptr(0))) {
		return windowsResult(resultPointer)
	}
	*(*uintptr)(pointer(result)) = 0
	provider := providerFromInterface(this)
	if provider == nil || provider.Tree == nil {
		return windowsResult(resultElementNotAvailable)
	}
	tree := provider.Tree
	tree.mu.RLock()
	if tree.closed {
		tree.mu.RUnlock()
		return windowsResult(resultElementNotAvailable)
	}
	var target *uiaProvider
	if provider.Root {
		if len(tree.children) != 0 && int32(direction) == uiaNavigateFirstChild {
			target = tree.children[0]
		} else if len(tree.children) != 0 && int32(direction) == uiaNavigateLastChild {
			target = tree.children[len(tree.children)-1]
		}
	} else if provider.Index >= 0 && provider.Index < len(tree.children) && tree.children[provider.Index] == provider {
		switch int32(direction) {
		case uiaNavigateParent:
			target = tree.root
		case uiaNavigateNextSibling:
			if provider.Index+1 < len(tree.children) {
				target = tree.children[provider.Index+1]
			}
		case uiaNavigatePreviousSibling:
			if provider.Index > 0 {
				target = tree.children[provider.Index-1]
			}
		}
	}
	if target != nil {
		target.addReference()
		*(*uintptr)(pointer(result)) = target.fragmentPointer()
	}
	tree.mu.RUnlock()
	return 0
}

func uiaGetRuntimeID(this, result uintptr) uintptr {
	if result == 0 {
		return windowsResult(resultPointer)
	}
	*(*uintptr)(pointer(result)) = 0
	provider := providerFromInterface(this)
	if provider == nil || provider.Root {
		return 0
	}
	array, ok := integerArrayData([]int32{uiaAppendRuntimeID, provider.RuntimeID})
	if !ok {
		return windowsResult(resultOutOfMemory)
	}
	*(*uintptr)(pointer(result)) = array
	return 0
}

func uiaGetBoundingRectangle(this, result uintptr) uintptr {
	if result == 0 {
		return windowsResult(resultPointer)
	}
	provider := providerFromInterface(this)
	if provider == nil || provider.Tree == nil {
		return windowsResult(resultElementNotAvailable)
	}
	var node platform.AccessibilityNode
	if !provider.Root {
		var ok bool
		if node, ok = provider.snapshot(); !ok {
			return windowsResult(resultElementNotAvailable)
		}
	} else {
		provider.Tree.mu.RLock()
		closed := provider.Tree.closed
		provider.Tree.mu.RUnlock()
		if closed {
			return windowsResult(resultElementNotAvailable)
		}
	}
	*(*uiaRectangle)(pointer(result)) = provider.Tree.providerBounds(provider, node)
	return 0
}

func uiaGetEmbeddedRoots(this, result uintptr) uintptr {
	if result == 0 {
		return windowsResult(resultPointer)
	}
	*(*uintptr)(pointer(result)) = 0
	return 0
}

func uiaSetFocus(this uintptr) uintptr {
	provider := providerFromInterface(this)
	if provider == nil || provider.Tree == nil {
		return windowsResult(resultElementNotAvailable)
	}
	provider.Tree.mu.RLock()
	window := provider.Tree.window
	closed := provider.Tree.closed
	provider.Tree.mu.RUnlock()
	if closed || window == 0 {
		return windowsResult(resultElementNotAvailable)
	}
	setFocus.Call(window)
	if provider.Root {
		return 0
	}
	node, ok := provider.snapshot()
	switch {
	case !ok:
		return windowsResult(resultElementNotAvailable)
	case node.Focused:
		return 0
	case !node.Focusable || !node.Operable():
		return windowsResult(resultNotSupported)
	}
	return provider.Tree.enqueue(platform.AccessibilityRequest{Node: node.ID, Action: platform.AccessibilityFocus})
}

func uiaGetFragmentRoot(this, result uintptr) uintptr {
	if result == 0 {
		return windowsResult(resultPointer)
	}
	*(*uintptr)(pointer(result)) = 0
	provider := providerFromInterface(this)
	if provider == nil || provider.Tree == nil {
		return windowsResult(resultElementNotAvailable)
	}
	provider.Tree.mu.RLock()
	root := provider.Tree.root
	closed := provider.Tree.closed
	if !closed && root != nil {
		root.addReference()
		*(*uintptr)(pointer(result)) = root.fragmentRootPointer()
	}
	provider.Tree.mu.RUnlock()
	if closed || root == nil {
		return windowsResult(resultElementNotAvailable)
	}
	return 0
}

func uiaElementProviderFromPointBits(this, xBits, yBits, result uintptr) uintptr {
	return uiaElementProviderFromPoint(this, math.Float64frombits(uint64(xBits)), math.Float64frombits(uint64(yBits)), result)
}

func uiaElementProviderFromPoint(this uintptr, x, y float64, result uintptr) uintptr {
	if result == 0 {
		return windowsResult(resultPointer)
	}
	*(*uintptr)(pointer(result)) = 0
	provider := providerFromInterface(this)
	if provider == nil || provider.Tree == nil {
		return windowsResult(resultElementNotAvailable)
	}
	tree := provider.Tree
	tree.mu.RLock()
	if tree.closed {
		tree.mu.RUnlock()
		return windowsResult(resultElementNotAvailable)
	}
	target := tree.root
	for index := len(tree.children) - 1; index >= 0; index-- {
		candidate := tree.children[index]
		bounds := tree.boundsLocked(candidate.Node)
		if x >= bounds.X && x < bounds.X+bounds.Width && y >= bounds.Y && y < bounds.Y+bounds.Height {
			target = candidate
			break
		}
	}
	if target != nil {
		target.addReference()
		*(*uintptr)(pointer(result)) = target.fragmentPointer()
	}
	tree.mu.RUnlock()
	return 0
}

func uiaGetFocus(this, result uintptr) uintptr {
	if result == 0 {
		return windowsResult(resultPointer)
	}
	*(*uintptr)(pointer(result)) = 0
	provider := providerFromInterface(this)
	if provider == nil || provider.Tree == nil {
		return windowsResult(resultElementNotAvailable)
	}
	tree := provider.Tree
	tree.mu.RLock()
	if tree.closed {
		tree.mu.RUnlock()
		return windowsResult(resultElementNotAvailable)
	}
	target := tree.root
	for _, child := range tree.children {
		if child.Node.Focused {
			target = child
			break
		}
	}
	if target != nil {
		target.addReference()
		*(*uintptr)(pointer(result)) = target.fragmentPointer()
	}
	tree.mu.RUnlock()
	return 0
}

func (tree *uiaTree) providerBounds(provider *uiaProvider, node platform.AccessibilityNode) uiaRectangle {
	tree.mu.RLock()
	defer tree.mu.RUnlock()
	if tree.closed {
		return uiaRectangle{}
	}
	if provider.Root {
		var bounds rect
		if result, _, _ := getWindowRectangle.Call(tree.window, uintptr(unsafe.Pointer(&bounds))); result != 0 {
			return uiaRectangle{X: float64(bounds.Left), Y: float64(bounds.Top), Width: float64(bounds.Right - bounds.Left), Height: float64(bounds.Bottom - bounds.Top)}
		}
		return uiaRectangle{}
	}
	return tree.boundsLocked(node)
}

func (tree *uiaTree) boundsLocked(node platform.AccessibilityNode) uiaRectangle {
	origin := point{}
	clientToScreen.Call(tree.window, uintptr(unsafe.Pointer(&origin)))
	scale := float64(tree.scale)
	return uiaRectangle{
		X:      float64(origin.X) + float64(node.X)*scale,
		Y:      float64(origin.Y) + float64(node.Y)*scale,
		Width:  math.Max(0, float64(node.Width)*scale),
		Height: math.Max(0, float64(node.Height)*scale),
	}
}

// controlType is the UI Automation control type for a node role (see
// platform.A11yRole*). Roles with no control type of their own are the closest
// one: UI Automation has no heading, status or switch.
func controlType(role uint8) int32 {
	switch role {
	case platform.A11yRoleApplication:
		return uiaWindowControlType
	case platform.A11yRoleGroup, platform.A11yRoleGrid:
		return uiaGroupControlType
	case platform.A11yRoleButton:
		return uiaButtonControlType
	case platform.A11yRoleText, platform.A11yRoleHeading, platform.A11yRoleStatus:
		return uiaTextControlType
	case platform.A11yRoleTextField:
		return uiaEditControlType
	case platform.A11yRoleCheckbox, platform.A11yRoleSwitch:
		return uiaCheckBoxControlType
	case platform.A11yRoleSlider:
		return uiaSliderControlType
	case platform.A11yRoleImage:
		return uiaImageControlType
	case platform.A11yRoleLink:
		return uiaHyperlinkControlType
	case platform.A11yRoleRadio:
		return uiaRadioControlType
	case platform.A11yRoleTab:
		return uiaTabItemControlType
	case platform.A11yRoleMenuItem:
		return uiaMenuItemControlType
	case platform.A11yRoleCombobox:
		return uiaComboBoxControlType
	case platform.A11yRoleMenu:
		return uiaMenuControlType
	case platform.A11yRoleMenubar:
		return uiaMenuBarControlType
	case platform.A11yRoleListBox:
		return uiaListControlType
	case platform.A11yRoleOption:
		return uiaListItemControlType
	case platform.A11yRoleGridCell:
		return uiaDataItemControlType
	case platform.A11yRoleSeparator:
		return uiaSeparatorControlType
	default:
		return uiaGroupControlType
	}
}

func toggleState(node platform.AccessibilityNode) int32 {
	switch {
	case node.Mixed:
		return 2 // ToggleState_Indeterminate
	case node.Checked:
		return 1
	}
	return 0
}

func expandState(node platform.AccessibilityNode) int32 {
	if node.Expanded {
		return 1
	}
	return 0
}

// valueReadOnly says whether the text value of a node cannot be set: only a
// text field or a combobox that is not read-only takes one.
func valueReadOnly(node platform.AccessibilityNode) bool {
	switch node.Role {
	case platform.A11yRoleTextField, platform.A11yRoleCombobox:
		return node.ReadOnly || node.Disabled
	}
	return true
}

func headingLevel(node platform.AccessibilityNode) int32 {
	if node.Role != platform.A11yRoleHeading || node.Level == 0 {
		return uiaHeadingLevelNone
	}
	return uiaHeadingLevelNone + int32(min(node.Level, 9))
}

func propertyVariant(property int32, node platform.AccessibilityNode) (variant, bool) {
	switch property {
	case uiaNameProperty:
		return stringVariant(node.Label)
	case uiaControlTypeProperty:
		return integerVariant(controlType(node.Role)), true
	case uiaHasKeyboardFocusProperty:
		return booleanVariant(node.Focused), true
	case uiaIsKeyboardFocusableProperty:
		return booleanVariant(node.Focusable && !node.Disabled), true
	case uiaIsEnabledProperty:
		return booleanVariant(!node.Disabled), true
	case uiaHelpTextProperty:
		return stringVariant(node.Hint)
	case uiaFullDescriptionProperty:
		return stringVariant(node.Description)
	case uiaAutomationIDProperty:
		if node.ID.IsZero() {
			return variant{}, false
		}
		return stringVariant("viva-" + strconv.FormatUint(node.ID.Bits(), 10))
	case uiaValueValueProperty:
		return stringVariant(node.Value)
	case uiaValueIsReadOnlyProperty:
		return booleanVariant(valueReadOnly(node)), true
	case uiaIsPasswordProperty:
		return booleanVariant(node.Secure), true
	case uiaExpandStateProperty:
		if !node.Patterns().Has(platform.PatternExpandCollapse) {
			return variant{}, false
		}
		return integerVariant(expandState(node)), true
	case uiaSelectionStateProperty:
		if !node.Patterns().Has(platform.PatternSelectionItem) {
			return variant{}, false
		}
		return booleanVariant(node.IsChosen()), true
	case uiaToggleStateProperty:
		if !node.Patterns().Has(platform.PatternToggle) {
			return variant{}, false
		}
		return integerVariant(toggleState(node)), true
	case uiaRangeValueValueProperty:
		if !node.Range.Set {
			return variant{}, false
		}
		return doubleVariant(node.Range.Now), true
	case uiaLiveSettingProperty:
		return integerVariant(int32(node.Live)), true
	case uiaHeadingLevelProperty:
		return integerVariant(headingLevel(node)), true
	case uiaPositionInSetProperty:
		if node.PosInSet <= 0 {
			return variant{}, false
		}
		return integerVariant(node.PosInSet), true
	case uiaSizeOfSetProperty:
		if node.SetSize <= 0 {
			return variant{}, false
		}
		return integerVariant(node.SetSize), true
	default:
		return variant{}, false
	}
}

func integerVariant(value int32) variant {
	return variant{Type: variantI4, Value: uint64(uint32(value))}
}

func doubleVariant(value float64) variant {
	return variant{Type: variantR8, Value: math.Float64bits(value)}
}

func booleanVariant(value bool) variant {
	if value {
		return variant{Type: variantBool, Value: uint64(uint16(0xffff))}
	}
	return variant{Type: variantBool}
}

func stringVariant(value string) (variant, bool) {
	characters, err := syscall.UTF16FromString(value)
	if err != nil {
		return variant{}, false
	}
	length := len(characters) - 1
	var source uintptr
	if length > 0 {
		source = uintptr(unsafe.Pointer(&characters[0]))
	}
	allocated, _, _ := systemAllocateString.Call(source, uintptr(length))
	if allocated == 0 && length != 0 {
		return variant{}, false
	}
	return variant{Type: variantBSTR, Value: uint64(allocated)}, true
}

func integerArrayData(values []int32) (uintptr, bool) {
	array, _, _ := safeArrayCreateVector.Call(variantI4, 0, uintptr(len(values)))
	if array == 0 {
		return 0, false
	}
	var data uintptr
	status, _, _ := safeArrayAccessData.Call(array, uintptr(unsafe.Pointer(&data)))
	if hresult(status) != nil || data == 0 {
		value := variant{Type: variantArray | variantI4, Value: uint64(array)}
		clearVariant(&value)
		return 0, false
	}
	copy(unsafe.Slice((*int32)(pointer(data)), len(values)), values)
	safeArrayUnaccessData.Call(array)
	return array, true
}

// doubleArrayData makes a SAFEARRAY of doubles, the shape of a text range's
// bounding rectangles.
func doubleArrayData(values []float64) (uintptr, bool) {
	array, _, _ := safeArrayCreateVector.Call(variantR8, 0, uintptr(len(values)))
	if array == 0 {
		return 0, false
	}
	if len(values) == 0 {
		return array, true
	}
	var data uintptr
	status, _, _ := safeArrayAccessData.Call(array, uintptr(unsafe.Pointer(&data)))
	if hresult(status) != nil || data == 0 {
		value := variant{Type: variantArray | variantR8, Value: uint64(array)}
		clearVariant(&value)
		return 0, false
	}
	copy(unsafe.Slice((*float64)(pointer(data)), len(values)), values)
	safeArrayUnaccessData.Call(array)
	return array, true
}

// unknownArrayData makes a SAFEARRAY of interface pointers. Each pointer was
// AddRef'd for the array, which gives it up when the client clears it.
func unknownArrayData(pointers []uintptr) (uintptr, bool) {
	array, _, _ := safeArrayCreateVector.Call(variantUnknown, 0, uintptr(len(pointers)))
	if array == 0 {
		return 0, false
	}
	if len(pointers) == 0 {
		return array, true
	}
	var data uintptr
	status, _, _ := safeArrayAccessData.Call(array, uintptr(unsafe.Pointer(&data)))
	if hresult(status) != nil || data == 0 {
		value := variant{Type: variantArray | variantUnknown, Value: uint64(array)}
		clearVariant(&value)
		return 0, false
	}
	copy(unsafe.Slice((*uintptr)(pointer(data)), len(pointers)), pointers)
	safeArrayUnaccessData.Call(array)
	return array, true
}

func clearVariant(value *variant) {
	if value.Type == variantEmpty {
		return
	}
	variantClear.Call(uintptr(unsafe.Pointer(value)))
	*value = variant{}
}

func windowsResult(value uint32) uintptr {
	return uintptr(int64(int32(value)))
}
