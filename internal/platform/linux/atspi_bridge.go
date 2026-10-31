//go:build unix

package linux

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/vivaui/viva/internal/dbus"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

var errAtspiClosed = errors.New("atspi: connection closed")

type atspiTransport interface {
	Call(destination, path, iface, member, signature string, body []byte) (*dbus.Message, error)
	Emit(path, iface, member, signature string, body []byte) error
	UniqueName() string
	FD() int
	Receive() ([]*dbus.Message, error)
	Reply(call *dbus.Message, signature string, body []byte) error
	ReplyError(call *dbus.Message, name, text string) error
	Close()
}

// atspiBridge serves the application's accessibility tree on the accessibility
// bus, the way a toolkit does for a screen reader: every node is an object with
// a path, and the reader calls methods on the interfaces the node says it has.
// It speaks only when the desktop has switched accessibility on, and it runs on
// the event-loop thread, in the same poll as the windows.
type atspiBridge struct {
	conn atspiTransport
	// send emits a signal on the bus; it is the connection's, and a test's own.
	send    func(path, interfaceName, member, signature string, body []byte) error
	unique  string   // the connection's own name on the bus
	parent  atspiRef // the desktop root the registry gave the application
	name    string
	windows []*atspiWindow
	next    int
}

// atspiRef is a reference to an object on the bus: the connection that serves
// it and the path.
type atspiRef struct{ Name, Path string }

// atspiWindow is one window of the application: a frame that holds the nodes.
type atspiWindow struct {
	number  int
	title   string
	nodes   []platform.AccessibilityNode
	spare   []platform.AccessibilityNode
	parents []int32
	active  bool

	// geometry is where the client area lies on the screen, in device pixels,
	// and the device pixels per logical pixel.
	geometry func() (x, y int, scale float32)
	// act carries a request to the application; nil when it takes none.
	act func(platform.AccessibilityRequest) bool
}

const (
	atspiRootPath   = "/org/a11y/atspi/accessible/root"
	atspiNullPath   = "/org/a11y/atspi/null"
	atspiPathPrefix = "/org/a11y/atspi/accessible/"

	atspiCachePath  = "/org/a11y/atspi/cache"
	atspiIfaceCache = "org.a11y.atspi.Cache"

	atspiEventObject = "org.a11y.atspi.Event.Object"
	atspiEventWindow = "org.a11y.atspi.Event.Window"
	atspiProperties  = "org.freedesktop.DBus.Properties"
	atspiIntrospect  = "org.freedesktop.DBus.Introspectable"

	// maxChildEvents bounds the add and remove events one update sends: a
	// whole page replaced is a change a reader asks about afresh.
	maxChildEvents = 64
)

func newAtspiBridge(conn atspiTransport) *atspiBridge {
	name := "viva"
	if executable, err := os.Executable(); err == nil {
		name = filepath.Base(executable)
	}
	return &atspiBridge{conn: conn, send: conn.Emit, unique: conn.UniqueName(), name: name}
}

// register announces the application to the registry, which gives it a place
// under the desktop.
func (bridge *atspiBridge) register() error {
	var request dbus.Encoder
	request.Align(8)
	request.String(bridge.unique)
	request.String(atspiRootPath)
	reply, err := bridge.conn.Call("org.a11y.atspi.Registry", atspiRootPath, "org.a11y.atspi.Socket", "Embed", "(so)", request.Bytes())
	if err != nil {
		return err
	}
	args := reply.Args()
	args.Align(8)
	bridge.parent = atspiRef{Name: args.String(), Path: args.String()}
	return nil
}

func (bridge *atspiBridge) close() {
	bridge.send = nil
	if bridge.conn != nil {
		bridge.conn.Close()
		bridge.conn = nil
	}
}

// addWindow makes a frame for a window and returns it.
func (bridge *atspiBridge) addWindow(title string, nodes []platform.AccessibilityNode, geometry func() (int, int, float32), act func(platform.AccessibilityRequest) bool) *atspiWindow {
	window := &atspiWindow{number: bridge.next, title: title, geometry: geometry, act: act}
	bridge.next++
	window.nodes = append(window.nodes, nodes...)
	window.parents = platform.AccessibilityParents(window.parents, window.nodes)
	bridge.windows = append(bridge.windows, window)
	bridge.emit(atspiRootPath, atspiEventObject, "ChildrenChanged", "add", int32(len(bridge.windows)-1), 0, bridge.ref(bridge.framePath(window)))
	return window
}

func (bridge *atspiBridge) removeWindow(window *atspiWindow) {
	for index, candidate := range bridge.windows {
		if candidate == window {
			bridge.emit(atspiRootPath, atspiEventObject, "ChildrenChanged", "remove", int32(index), 0, bridge.ref(bridge.framePath(window)))
			bridge.windows = append(bridge.windows[:index], bridge.windows[index+1:]...)
			return
		}
	}
}

func (bridge *atspiBridge) framePath(window *atspiWindow) string {
	return atspiPathPrefix + "w" + strconv.Itoa(window.number)
}

func (bridge *atspiBridge) nodePath(window *atspiWindow, nodes []platform.AccessibilityNode, index int) string {
	if index < 0 {
		return bridge.framePath(window)
	}
	if id := nodes[index].ID; !id.IsZero() {
		return bridge.framePath(window) + "/i" + strconv.FormatUint(id.Bits(), 10)
	}
	ordinal := 0
	for before := 0; before < index; before++ {
		if nodes[before].ID.IsZero() {
			ordinal++
		}
	}
	return bridge.framePath(window) + "/a" + strconv.Itoa(ordinal)
}

func (bridge *atspiBridge) ref(path string) atspiRef {
	return atspiRef{Name: bridge.unique, Path: path}
}

// atspiTarget is the object a path names.
type atspiTarget struct {
	window *atspiWindow // nil for the application
	index  int          // -1 for the application or a frame
}

func (bridge *atspiBridge) resolve(path string) (atspiTarget, bool) {
	if path == atspiRootPath {
		return atspiTarget{index: -1}, true
	}
	rest, ok := strings.CutPrefix(path, atspiPathPrefix+"w")
	if !ok {
		return atspiTarget{}, false
	}
	numberText, nodeText, hasNode := strings.Cut(rest, "/")
	number, err := strconv.Atoi(numberText)
	if err != nil {
		return atspiTarget{}, false
	}
	var window *atspiWindow
	for _, candidate := range bridge.windows {
		if candidate.number == number {
			window = candidate
		}
	}
	if window == nil {
		return atspiTarget{}, false
	}
	if !hasNode {
		return atspiTarget{window: window, index: -1}, true
	}
	if len(nodeText) < 2 {
		return atspiTarget{}, false
	}
	value, err := strconv.ParseUint(nodeText[1:], 10, 64)
	if err != nil {
		return atspiTarget{}, false
	}
	switch nodeText[0] {
	case 'i':
		if index := platform.AccessibilityIndexOf(window.nodes, ui.IDFromBits(value)); index >= 0 {
			return atspiTarget{window: window, index: index}, true
		}
	case 'a':
		ordinal := uint64(0)
		for index := range window.nodes {
			if window.nodes[index].ID.IsZero() {
				if ordinal == value {
					return atspiTarget{window: window, index: index}, true
				}
				ordinal++
			}
		}
	}
	return atspiTarget{}, false
}

// children are the indexes of the nodes inside a node, or at the top of the
// window for index -1.
func (window *atspiWindow) children(parent int) []int {
	var children []int
	for index, container := range window.parents {
		if int(container) == parent {
			children = append(children, index)
		}
	}
	return children
}

func (window *atspiWindow) indexInParent(index int) int {
	position := 0
	for sibling, container := range window.parents {
		if container != window.parents[index] {
			continue
		}
		if sibling == index {
			return position
		}
		position++
	}
	return -1
}

// node is the node a target names, and false for the application or a frame.
func (target atspiTarget) node() (platform.AccessibilityNode, bool) {
	if target.window == nil || target.index < 0 {
		return platform.AccessibilityNode{}, false
	}
	return target.window.nodes[target.index], true
}

func (bridge *atspiBridge) refOf(window *atspiWindow, index int) atspiRef {
	return bridge.ref(bridge.nodePath(window, window.nodes, index))
}

// encodeRef writes a (so) struct.
func encodeRef(e *dbus.Encoder, ref atspiRef) {
	e.Align(8)
	e.String(ref.Name)
	e.String(ref.Path)
}

func encodeDouble(e *dbus.Encoder, value float64) {
	e.Align(8)
	bits := math.Float64bits(value)
	e.Uint32(uint32(bits))
	e.Uint32(uint32(bits >> 32))
}

func decodeDouble(d *dbus.Decoder) float64 {
	d.Align(8)
	low, high := d.Uint32(), d.Uint32()
	return math.Float64frombits(uint64(high)<<32 | uint64(low))
}

func encodeInt16(e *dbus.Encoder, value int16) {
	e.Align(2)
	e.Byte(byte(value))
	e.Byte(byte(uint16(value) >> 8))
}

func encodeStringMap(e *dbus.Encoder, pairs [][2]string) {
	mark := e.BeginArray(8)
	for _, pair := range pairs {
		e.Align(8)
		e.String(pair[0])
		e.String(pair[1])
	}
	e.EndArray(mark)
}

// nullRef is the reference to no object.
var nullRef = atspiRef{Name: "", Path: atspiNullPath}

// emit sends an event signal for the object at path. data is the value of the
// event: an int, a string, a double or an object reference.
func (bridge *atspiBridge) emit(path, interfaceName, member, detail string, detail1, detail2 int32, data any) {
	if bridge.send == nil {
		return
	}
	var body dbus.Encoder
	body.String(detail)
	body.Int32(detail1)
	body.Int32(detail2)
	switch value := data.(type) {
	case string:
		body.Variant("s")
		body.String(value)
	case float64:
		body.Variant("d")
		encodeDouble(&body, value)
	case atspiRef:
		body.Variant("(so)")
		encodeRef(&body, value)
	default:
		body.Variant("i")
		body.Int32(0)
	}
	mark := body.BeginArray(8)
	body.EndArray(mark)
	if bridge.send(path, interfaceName, member, "siiva{sv}", body.Bytes()) != nil {
		bridge.close()
	}
}

// setActive reports a window gaining or losing the keyboard.
func (bridge *atspiBridge) setActive(window *atspiWindow, active bool) {
	if window.active == active {
		return
	}
	window.active = active
	path := bridge.framePath(window)
	member := "Deactivate"
	flag := int32(0)
	if active {
		member, flag = "Activate", 1
	}
	bridge.emit(path, atspiEventWindow, member, "", 0, 0, nil)
	bridge.emit(path, atspiEventObject, "StateChanged", "active", flag, 0, nil)
}

// update replaces a window's nodes and tells the reader what changed.
func (bridge *atspiBridge) update(window *atspiWindow, nodes []platform.AccessibilityNode) {
	old := window.nodes
	oldPaths := make(map[ui.ID]int, len(old))
	for index := range old {
		if id := old[index].ID; !id.IsZero() {
			oldPaths[id] = index
		}
	}
	window.nodes, window.spare = append(window.spare[:0], nodes...), old
	window.parents = platform.AccessibilityParents(window.parents, window.nodes)
	if bridge.send == nil {
		return
	}

	// Nodes with no ID, the regions, are matched by their order among such nodes.
	var oldAnonymous []int
	for index := range old {
		if old[index].ID.IsZero() {
			oldAnonymous = append(oldAnonymous, index)
		}
	}
	events, anonymous := 0, 0
	seen := make(map[ui.ID]bool, len(nodes))
	for index, node := range window.nodes {
		path := bridge.nodePath(window, window.nodes, index)
		if node.ID.IsZero() {
			if anonymous < len(oldAnonymous) && old[oldAnonymous[anonymous]].Role == node.Role {
				bridge.announceChanges(window, path, old[oldAnonymous[anonymous]], node)
			}
			anonymous++
			continue
		}
		seen[node.ID] = true
		previous, existed := oldPaths[node.ID]
		if !existed {
			if events < maxChildEvents {
				events++
				bridge.emit(bridge.parentPath(window, index), atspiEventObject, "ChildrenChanged", "add", int32(window.indexInParent(index)), 0, bridge.ref(path))
			}
			continue
		}
		bridge.announceChanges(window, path, old[previous], node)
	}
	for index := range old {
		node := old[index]
		if node.ID.IsZero() || seen[node.ID] || events >= maxChildEvents {
			continue
		}
		events++
		bridge.emit(atspiPathPrefix+"w"+strconv.Itoa(window.number), atspiEventObject, "ChildrenChanged", "remove", int32(index), 0, bridge.ref(bridge.nodePath(window, old, index)))
	}
}

// parentPath is the path of the object that holds the node at index.
func (bridge *atspiBridge) parentPath(window *atspiWindow, index int) string {
	return bridge.nodePath(window, window.nodes, int(window.parents[index]))
}

// announceChanges emits the events that turn the reader's picture of a node
// from old into next.
func (bridge *atspiBridge) announceChanges(window *atspiWindow, path string, old, next platform.AccessibilityNode) {
	changed := platform.DiffAccessibility(old, next)
	if changed&^platform.ChangedBounds == 0 {
		return
	}
	state := func(name string, on bool) {
		flag := int32(0)
		if on {
			flag = 1
		}
		bridge.emit(path, atspiEventObject, "StateChanged", name, flag, 0, nil)
	}
	if changed&platform.ChangedFocus != 0 {
		state("focused", next.Focused)
	}
	if changed&platform.ChangedEnabled != 0 {
		state("enabled", !next.Disabled)
		state("sensitive", !next.Disabled)
	}
	if changed&platform.ChangedToggle != 0 {
		if old.Mixed != next.Mixed {
			state("indeterminate", next.Mixed)
		}
		state("checked", next.Checked)
	}
	if changed&platform.ChangedSelected != 0 {
		state("selected", next.IsChosen())
	}
	if changed&platform.ChangedExpand != 0 {
		state("expanded", next.Expanded)
		state("collapsed", !next.Expanded)
	}
	if changed&platform.ChangedName != 0 {
		bridge.emit(path, atspiEventObject, "PropertyChange", "accessible-name", 0, 0, next.Label)
	}
	if changed&(platform.ChangedHint|platform.ChangedDescription) != 0 {
		bridge.emit(path, atspiEventObject, "PropertyChange", "accessible-description", 0, 0, atspiDescription(next))
	}
	if changed&platform.ChangedRange != 0 && next.Range.Set {
		bridge.emit(path, atspiEventObject, "PropertyChange", "accessible-value", 0, 0, next.Range.Now)
	}
	isText := next.Patterns().Has(platform.PatternText)
	if isText && changed&platform.ChangedValue != 0 {
		before, after := []rune(old.Value), []rune(next.Value)
		start, removed, added := atspiTextEdit(before, after)
		if removed > 0 {
			bridge.emit(path, atspiEventObject, "TextChanged", "delete", int32(start), int32(removed), string(before[start:start+removed]))
		}
		if added > 0 {
			bridge.emit(path, atspiEventObject, "TextChanged", "insert", int32(start), int32(added), string(after[start:start+added]))
		}
	}
	if isText && changed&platform.ChangedSelection != 0 && next.Selection.Valid {
		bridge.emit(path, atspiEventObject, "TextCaretMoved", "", next.Selection.End, 0, nil)
		bridge.emit(path, atspiEventObject, "TextSelectionChanged", "", 0, 0, nil)
	}
	if next.Live != ui.LiveOff && changed&(platform.ChangedName|platform.ChangedValue) != 0 {
		words := next.Value
		if words == "" {
			words = next.Label
		}
		if words != "" {
			politeness := int32(1)
			if next.Live == ui.LiveAssertive {
				politeness = 2
			}
			bridge.emit(path, atspiEventObject, "Announcement", words, politeness, 0, words)
		}
	}
}

func atspiDescription(node platform.AccessibilityNode) string {
	switch {
	case node.Hint == "":
		return node.Description
	case node.Description == "":
		return node.Hint
	}
	return node.Hint + ". " + node.Description
}

// serve answers the calls that have arrived on the bus.
func (bridge *atspiBridge) serve() error {
	if bridge.conn == nil {
		return errAtspiClosed
	}
	messages, err := bridge.conn.Receive()
	if err != nil {
		return err
	}
	for _, message := range messages {
		if message.Type == dbus.TypeCall {
			bridge.handle(message)
			if bridge.conn == nil {
				return errAtspiClosed
			}
		}
	}
	return nil
}

// atspiAnswer is the result of a call: a signature and the encoded values.
type atspiAnswer struct {
	signature string
	body      []byte
	errorName string
	errorText string
}

func answerWith(signature string, fill func(e *dbus.Encoder)) atspiAnswer {
	var e dbus.Encoder
	fill(&e)
	return atspiAnswer{signature: signature, body: e.Bytes()}
}

func answerBool(value bool) atspiAnswer {
	return answerWith("b", func(e *dbus.Encoder) { e.Bool(value) })
}

func answerString(value string) atspiAnswer {
	return answerWith("s", func(e *dbus.Encoder) { e.String(value) })
}

func answerInt(value int32) atspiAnswer {
	return answerWith("i", func(e *dbus.Encoder) { e.Int32(value) })
}

func answerError(name, text string) atspiAnswer { return atspiAnswer{errorName: name, errorText: text} }

func (bridge *atspiBridge) handle(call *dbus.Message) {
	if bridge.conn == nil {
		return
	}
	answer := bridge.answer(call)
	if bridge.conn == nil {
		return
	}
	if answer.errorName != "" {
		if bridge.conn.ReplyError(call, answer.errorName, answer.errorText) != nil {
			bridge.close()
		}
		return
	}
	if bridge.conn.Reply(call, answer.signature, answer.body) != nil {
		bridge.close()
	}
}

// answer computes the reply to a call.
func (bridge *atspiBridge) answer(call *dbus.Message) atspiAnswer {
	if call.Path == atspiCachePath {
		if call.Interface == atspiIfaceCache && call.Member == "GetItems" {
			return bridge.answerCache()
		}
		return unknownMethod(call)
	}
	target, ok := bridge.resolve(call.Path)
	if !ok {
		return answerError("org.freedesktop.DBus.Error.UnknownObject", call.Path)
	}
	switch call.Interface {
	case atspiIntrospect:
		return answerString(`<node/>`)
	case atspiProperties:
		return bridge.answerProperties(target, call)
	case atspiIfaceAccessible:
		return bridge.answerAccessible(target, call)
	case atspiIfaceApplication:
		return bridge.answerApplication(target, call)
	case atspiIfaceComponent:
		return bridge.answerComponent(target, call)
	case atspiIfaceAction:
		return bridge.answerAction(target, call)
	case atspiIfaceText:
		return bridge.answerText(target, call)
	case atspiIfaceEditableText:
		return bridge.answerEditable(target, call)
	case atspiIfaceValue:
		return answerError("org.freedesktop.DBus.Error.UnknownMethod", call.Member)
	}
	return answerError("org.freedesktop.DBus.Error.UnknownInterface", call.Interface)
}

func unknownMethod(call *dbus.Message) atspiAnswer {
	return answerError("org.freedesktop.DBus.Error.UnknownMethod", call.Interface+"."+call.Member)
}

// nameOf, roleOf and the other accessors are the Accessible view of a target,
// whether the application, a frame or a node.
func (bridge *atspiBridge) nameOf(target atspiTarget) string {
	if node, ok := target.node(); ok {
		return node.Label
	}
	if target.window != nil {
		return target.window.title
	}
	return bridge.name
}

func (bridge *atspiBridge) roleOf(target atspiTarget) uint32 {
	if node, ok := target.node(); ok {
		return atspiRole(node)
	}
	if target.window != nil {
		return atspiRoleFrame
	}
	return atspiRoleApplication
}

func (bridge *atspiBridge) childrenOf(target atspiTarget) []atspiRef {
	var refs []atspiRef
	switch {
	case target.window == nil:
		for _, window := range bridge.windows {
			refs = append(refs, bridge.ref(bridge.framePath(window)))
		}
	default:
		for _, child := range target.window.children(target.index) {
			refs = append(refs, bridge.refOf(target.window, child))
		}
	}
	return refs
}

func (bridge *atspiBridge) parentOf(target atspiTarget) atspiRef {
	switch {
	case target.window == nil:
		return bridge.parent
	case target.index < 0:
		return bridge.ref(atspiRootPath)
	}
	if parent := int(target.window.parents[target.index]); parent >= 0 {
		return bridge.refOf(target.window, parent)
	}
	return bridge.ref(bridge.framePath(target.window))
}

func (bridge *atspiBridge) indexInParent(target atspiTarget) int32 {
	switch {
	case target.window == nil:
		return -1
	case target.index < 0:
		for index, window := range bridge.windows {
			if window == target.window {
				return int32(index)
			}
		}
		return -1
	}
	return int32(target.window.indexInParent(target.index))
}

func (bridge *atspiBridge) interfacesOf(target atspiTarget) []string {
	if node, ok := target.node(); ok {
		return atspiInterfaces(node, target.window.act != nil)
	}
	if target.window == nil {
		return []string{atspiIfaceAccessible, atspiIfaceApplication}
	}
	return []string{atspiIfaceAccessible, atspiIfaceComponent}
}

func (bridge *atspiBridge) statesOf(target atspiTarget) [2]uint32 {
	if node, ok := target.node(); ok {
		return atspiStates(node)
	}
	if target.window != nil {
		return atspiFrameStates(target.window.active)
	}
	return [2]uint32{1 << atspiStateEnabled, 0}
}

func (bridge *atspiBridge) descriptionOf(target atspiTarget) string {
	if node, ok := target.node(); ok {
		return atspiDescription(node)
	}
	return ""
}

func (bridge *atspiBridge) answerAccessible(target atspiTarget, call *dbus.Message) atspiAnswer {
	switch call.Member {
	case "GetChildAtIndex":
		index := int(call.Args().Int32())
		children := bridge.childrenOf(target)
		if index < 0 || index >= len(children) {
			return answerWith("(so)", func(e *dbus.Encoder) { encodeRef(e, nullRef) })
		}
		return answerWith("(so)", func(e *dbus.Encoder) { encodeRef(e, children[index]) })
	case "GetChildren":
		children := bridge.childrenOf(target)
		return answerWith("a(so)", func(e *dbus.Encoder) {
			mark := e.BeginArray(8)
			for _, child := range children {
				encodeRef(e, child)
			}
			e.EndArray(mark)
		})
	case "GetIndexInParent":
		return answerInt(bridge.indexInParent(target))
	case "GetRelationSet":
		return answerWith("a(ua(so))", func(e *dbus.Encoder) {
			mark := e.BeginArray(8)
			e.EndArray(mark)
		})
	case "GetRole":
		return answerWith("u", func(e *dbus.Encoder) { e.Uint32(bridge.roleOf(target)) })
	case "GetRoleName", "GetLocalizedRoleName":
		return answerString(atspiRoleName(bridge.roleOf(target)))
	case "GetState":
		states := bridge.statesOf(target)
		return answerWith("au", func(e *dbus.Encoder) {
			mark := e.BeginArray(4)
			e.Uint32(states[0])
			e.Uint32(states[1])
			e.EndArray(mark)
		})
	case "GetAttributes":
		var pairs [][2]string
		if node, ok := target.node(); ok {
			pairs = atspiAttributes(node)
		}
		return answerWith("a{ss}", func(e *dbus.Encoder) { encodeStringMap(e, pairs) })
	case "GetApplication":
		return answerWith("(so)", func(e *dbus.Encoder) { encodeRef(e, bridge.ref(atspiRootPath)) })
	case "GetInterfaces":
		interfaces := bridge.interfacesOf(target)
		return answerWith("as", func(e *dbus.Encoder) {
			mark := e.BeginArray(4)
			for _, name := range interfaces {
				e.String(name)
			}
			e.EndArray(mark)
		})
	}
	return unknownMethod(call)
}

func (bridge *atspiBridge) answerApplication(target atspiTarget, call *dbus.Message) atspiAnswer {
	switch call.Member {
	case "GetLocale":
		return answerString("")
	case "GetApplicationBusAddress":
		return answerString("")
	}
	return unknownMethod(call)
}

// extentsOf is a node's rectangle in the coordinates a client asks for: the
// screen, or the window. A frame is the whole client area.
func (bridge *atspiBridge) extentsOf(target atspiTarget, coordinates uint32) (x, y, width, height int32) {
	if target.window == nil {
		return 0, 0, 0, 0
	}
	originX, originY, scale := target.window.geometry()
	if coordinates != 0 {
		originX, originY = 0, 0
	}
	if node, ok := target.node(); ok {
		return int32(originX) + int32(math.Round(float64(node.X*scale))), int32(originY) + int32(math.Round(float64(node.Y*scale))),
			int32(math.Round(float64(node.Width * scale))), int32(math.Round(float64(node.Height * scale)))
	}
	var right, bottom float32
	for _, node := range target.window.nodes {
		right, bottom = max(right, node.X+node.Width), max(bottom, node.Y+node.Height)
	}
	return int32(originX), int32(originY), int32(math.Round(float64(right * scale))), int32(math.Round(float64(bottom * scale)))
}

func (bridge *atspiBridge) answerComponent(target atspiTarget, call *dbus.Message) atspiAnswer {
	switch call.Member {
	case "Contains":
		args := call.Args()
		x, y, coordinates := args.Int32(), args.Int32(), args.Uint32()
		left, top, width, height := bridge.extentsOf(target, coordinates)
		return answerBool(x >= left && x < left+width && y >= top && y < top+height)
	case "GetAccessibleAtPoint":
		args := call.Args()
		x, y, coordinates := args.Int32(), args.Int32(), args.Uint32()
		found := nullRef
		if target.window != nil {
			for index := len(target.window.nodes) - 1; index >= 0; index-- {
				left, top, width, height := bridge.extentsOf(atspiTarget{window: target.window, index: index}, coordinates)
				if x >= left && x < left+width && y >= top && y < top+height {
					found = bridge.refOf(target.window, index)
					break
				}
			}
		}
		return answerWith("(so)", func(e *dbus.Encoder) { encodeRef(e, found) })
	case "GetExtents":
		x, y, width, height := bridge.extentsOf(target, call.Args().Uint32())
		return answerWith("(iiii)", func(e *dbus.Encoder) {
			e.Align(8)
			e.Int32(x)
			e.Int32(y)
			e.Int32(width)
			e.Int32(height)
		})
	case "GetPosition":
		x, y, _, _ := bridge.extentsOf(target, call.Args().Uint32())
		return answerWith("ii", func(e *dbus.Encoder) { e.Int32(x); e.Int32(y) })
	case "GetSize":
		_, _, width, height := bridge.extentsOf(target, 0)
		return answerWith("ii", func(e *dbus.Encoder) { e.Int32(width); e.Int32(height) })
	case "GetLayer":
		return answerWith("u", func(e *dbus.Encoder) { e.Uint32(3) }) // ATSPI_LAYER_WIDGET
	case "GetMDIZOrder":
		return answerWith("n", func(e *dbus.Encoder) { encodeInt16(e, 0) })
	case "GrabFocus":
		node, ok := target.node()
		if !ok || !node.Focusable {
			return answerBool(false)
		}
		return answerBool(bridge.act(target, platform.AccessibilityRequest{Node: node.ID, Action: platform.AccessibilityFocus}))
	case "GetAlpha":
		return answerWith("d", func(e *dbus.Encoder) { encodeDouble(e, 1) })
	case "SetExtents", "SetPosition", "SetSize", "ScrollTo", "ScrollToPoint":
		return answerBool(false)
	}
	return unknownMethod(call)
}

// act sends the application a request for a node, after which the reader is
// told what changed through the next update.
func (bridge *atspiBridge) act(target atspiTarget, request platform.AccessibilityRequest) bool {
	if target.window == nil || target.window.act == nil {
		return false
	}
	return target.window.act(request)
}

func (bridge *atspiBridge) answerAction(target atspiTarget, call *dbus.Message) atspiAnswer {
	node, ok := target.node()
	if !ok {
		return unknownMethod(call)
	}
	actions := atspiActions(node)
	if target.window.act == nil {
		actions = nil
	}
	switch call.Member {
	case "GetDescription", "GetName", "GetLocalizedName", "GetKeyBinding":
		index := int(call.Args().Int32())
		if index < 0 || index >= len(actions) {
			return answerString("")
		}
		switch call.Member {
		case "GetDescription":
			return answerString(actions[index].Description)
		case "GetKeyBinding":
			return answerString("")
		}
		return answerString(actions[index].Name)
	case "GetActions":
		return answerWith("a(sss)", func(e *dbus.Encoder) {
			mark := e.BeginArray(8)
			for _, action := range actions {
				e.Align(8)
				e.String(action.Name)
				e.String(action.Description)
				e.String("")
			}
			e.EndArray(mark)
		})
	case "DoAction":
		index := int(call.Args().Int32())
		if index < 0 || index >= len(actions) {
			return answerBool(false)
		}
		request := platform.AccessibilityRequest{Node: node.ID, Action: platform.AccessibilityPress}
		if actions[index].Name == "focus" {
			request.Action = platform.AccessibilityFocus
		}
		return answerBool(bridge.act(target, request))
	}
	return unknownMethod(call)
}

// legacyGranularity maps AtspiTextBoundaryType to a granularity: characters,
// words (start or end), sentences and lines (start or end).
func legacyGranularity(boundary uint32) uint32 {
	switch boundary {
	case 0:
		return atspiGranularityChar
	case 1, 2:
		return atspiGranularityWord
	}
	return atspiGranularityLine
}

func answerSpan(text string, start, end int) atspiAnswer {
	return answerWith("sii", func(e *dbus.Encoder) {
		e.String(text)
		e.Int32(int32(start))
		e.Int32(int32(end))
	})
}

func (bridge *atspiBridge) answerText(target atspiTarget, call *dbus.Message) atspiAnswer {
	node, ok := target.node()
	if !ok {
		return unknownMethod(call)
	}
	text := []rune(node.Value)
	caret, anchor := 0, 0
	if node.Selection.Valid {
		caret, anchor = int(node.Selection.End), int(node.Selection.Start)
	}
	switch call.Member {
	case "GetText":
		args := call.Args()
		start, end := int(args.Int32()), int(args.Int32())
		if end < 0 || end > len(text) {
			end = len(text)
		}
		start = max(0, min(start, end))
		return answerString(string(text[start:end]))
	case "SetCaretOffset":
		offset := int(call.Args().Int32())
		return answerBool(bridge.act(target, platform.AccessibilityRequest{Node: node.ID, Action: platform.AccessibilitySetSelection, Start: int32(offset), End: int32(offset)}))
	case "GetStringAtOffset":
		args := call.Args()
		offset, granularity := int(args.Int32()), args.Uint32()
		return answerSpan(atspiTextAt(text, offset, granularity))
	case "GetTextAtOffset":
		args := call.Args()
		offset, boundary := int(args.Int32()), args.Uint32()
		return answerSpan(atspiTextAt(text, offset, legacyGranularity(boundary)))
	case "GetTextBeforeOffset":
		args := call.Args()
		offset, boundary := int(args.Int32()), args.Uint32()
		return answerSpan(atspiTextBefore(text, offset, legacyGranularity(boundary)))
	case "GetTextAfterOffset":
		args := call.Args()
		offset, boundary := int(args.Int32()), args.Uint32()
		return answerSpan(atspiTextAfter(text, offset, legacyGranularity(boundary)))
	case "GetCharacterAtOffset":
		offset := int(call.Args().Int32())
		if offset < 0 || offset >= len(text) {
			return answerInt(0)
		}
		return answerInt(int32(text[offset]))
	case "GetAttributeValue":
		return answerString("")
	case "GetAttributes", "GetAttributeRun":
		return answerWith("a{ss}ii", func(e *dbus.Encoder) {
			encodeStringMap(e, nil)
			e.Int32(0)
			e.Int32(int32(len(text)))
		})
	case "GetDefaultAttributes", "GetDefaultAttributeSet":
		return answerWith("a{ss}", func(e *dbus.Encoder) { encodeStringMap(e, nil) })
	case "GetCharacterExtents", "GetRangeExtents":
		// Where a glyph falls inside the field is the text layout's to say;
		// the field's own rectangle is the bound a reader can be given.
		args := call.Args()
		args.Int32()
		if call.Member == "GetRangeExtents" {
			args.Int32()
		}
		x, y, width, height := bridge.extentsOf(target, args.Uint32())
		return answerWith("iiii", func(e *dbus.Encoder) { e.Int32(x); e.Int32(y); e.Int32(width); e.Int32(height) })
	case "GetOffsetAtPoint":
		return answerInt(int32(caret))
	case "GetNSelections":
		if node.Selection.Valid && node.Selection.Start != node.Selection.End {
			return answerInt(1)
		}
		return answerInt(0)
	case "GetSelection":
		if call.Args().Int32() != 0 || !node.Selection.Valid || node.Selection.Start == node.Selection.End {
			return answerError("org.freedesktop.DBus.Error.InvalidArgs", "no such selection")
		}
		start, end := min(caret, anchor), max(caret, anchor)
		return answerWith("ii", func(e *dbus.Encoder) { e.Int32(int32(start)); e.Int32(int32(end)) })
	case "AddSelection", "SetSelection":
		args := call.Args()
		if call.Member == "SetSelection" && args.Int32() != 0 {
			return answerBool(false)
		}
		start, end := args.Int32(), args.Int32()
		return answerBool(bridge.act(target, platform.AccessibilityRequest{Node: node.ID, Action: platform.AccessibilitySetSelection, Start: start, End: end}))
	case "RemoveSelection":
		return answerBool(bridge.act(target, platform.AccessibilityRequest{Node: node.ID, Action: platform.AccessibilitySetSelection, Start: int32(caret), End: int32(caret)}))
	case "GetBoundedRanges":
		return answerWith("a(iisv)", func(e *dbus.Encoder) {
			mark := e.BeginArray(8)
			e.EndArray(mark)
		})
	case "ScrollSubstringTo", "ScrollSubstringToPoint":
		return answerBool(false)
	}
	return unknownMethod(call)
}

func (bridge *atspiBridge) answerEditable(target atspiTarget, call *dbus.Message) atspiAnswer {
	node, ok := target.node()
	if !ok {
		return unknownMethod(call)
	}
	text := []rune(node.Value)
	set := func(value string) atspiAnswer {
		if node.ReadOnly {
			return answerBool(false)
		}
		return answerBool(bridge.act(target, platform.AccessibilityRequest{Node: node.ID, Action: platform.AccessibilitySetText, Text: value}))
	}
	switch call.Member {
	case "SetTextContents":
		args := call.Args()
		return set(args.String())
	case "InsertText":
		args := call.Args()
		position, value := int(args.Int32()), args.String()
		position = max(0, min(position, len(text)))
		return set(string(text[:position]) + value + string(text[position:]))
	case "DeleteText":
		args := call.Args()
		start, end := int(args.Int32()), int(args.Int32())
		start, end = max(0, min(start, len(text))), max(0, min(end, len(text)))
		if start > end {
			start, end = end, start
		}
		return set(string(text[:start]) + string(text[end:]))
	case "CopyText":
		return answerWith("", func(e *dbus.Encoder) {})
	case "CutText", "PasteText":
		return answerBool(false)
	}
	return unknownMethod(call)
}

// property reads one property of an interface of a target as a variant: its
// signature and an encoder for its value.
func (bridge *atspiBridge) property(target atspiTarget, interfaceName, name string) (signature string, write func(e *dbus.Encoder), ok bool) {
	str := func(value string) (string, func(e *dbus.Encoder), bool) {
		return "s", func(e *dbus.Encoder) { e.String(value) }, true
	}
	number := func(value int32) (string, func(e *dbus.Encoder), bool) {
		return "i", func(e *dbus.Encoder) { e.Int32(value) }, true
	}
	double := func(value float64) (string, func(e *dbus.Encoder), bool) {
		return "d", func(e *dbus.Encoder) { encodeDouble(e, value) }, true
	}
	node, isNode := target.node()
	switch interfaceName {
	case atspiIfaceAccessible:
		switch name {
		case "Name":
			return str(bridge.nameOf(target))
		case "Description":
			return str(bridge.descriptionOf(target))
		case "Parent":
			parent := bridge.parentOf(target)
			return "(so)", func(e *dbus.Encoder) { encodeRef(e, parent) }, true
		case "ChildCount":
			return number(int32(len(bridge.childrenOf(target))))
		case "Locale":
			return str("")
		case "AccessibleId":
			return str("")
		case "HelpText":
			return str("")
		}
	case atspiIfaceApplication:
		switch name {
		case "ToolkitName":
			return str("viva")
		case "Version", "ToolkitVersion":
			return str("1")
		case "AtspiVersion":
			return str("2.1")
		case "Id":
			return number(-1)
		}
	case atspiIfaceAction:
		if name == "NActions" && isNode && target.window.act != nil {
			return number(int32(len(atspiActions(node))))
		}
	case atspiIfaceText:
		if isNode {
			switch name {
			case "CharacterCount":
				return number(int32(len([]rune(node.Value))))
			case "CaretOffset":
				if node.Selection.Valid {
					return number(node.Selection.End)
				}
				return number(0)
			}
		}
	case atspiIfaceValue:
		if isNode && node.Range.Set {
			switch name {
			case "MinimumValue":
				return double(node.Range.Min)
			case "MaximumValue":
				return double(node.Range.Max)
			case "MinimumIncrement":
				return double(max(node.Range.Step, 0))
			case "CurrentValue":
				return double(node.Range.Now)
			case "Text":
				return str(node.Value)
			}
		}
	}
	return "", nil, false
}

var atspiPropertyNames = map[string][]string{
	atspiIfaceAccessible:  {"Name", "Description", "Parent", "ChildCount", "Locale", "AccessibleId", "HelpText"},
	atspiIfaceApplication: {"ToolkitName", "Version", "ToolkitVersion", "AtspiVersion", "Id"},
	atspiIfaceAction:      {"NActions"},
	atspiIfaceText:        {"CharacterCount", "CaretOffset"},
	atspiIfaceValue:       {"MinimumValue", "MaximumValue", "MinimumIncrement", "CurrentValue", "Text"},
}

func (bridge *atspiBridge) answerProperties(target atspiTarget, call *dbus.Message) atspiAnswer {
	args := call.Args()
	interfaceName := args.String()
	switch call.Member {
	case "Get":
		name := args.String()
		signature, write, ok := bridge.property(target, interfaceName, name)
		if !ok {
			return answerError("org.freedesktop.DBus.Error.UnknownProperty", interfaceName+"."+name)
		}
		return answerWith("v", func(e *dbus.Encoder) {
			e.Variant(signature)
			write(e)
		})
	case "GetAll":
		return answerWith("a{sv}", func(e *dbus.Encoder) {
			mark := e.BeginArray(8)
			for _, name := range atspiPropertyNames[interfaceName] {
				signature, write, ok := bridge.property(target, interfaceName, name)
				if !ok {
					continue
				}
				e.Align(8)
				e.String(name)
				e.Variant(signature)
				write(e)
			}
			e.EndArray(mark)
		})
	case "Set":
		name := args.String()
		signature := args.Signature()
		node, ok := target.node()
		if interfaceName != atspiIfaceValue || name != "CurrentValue" || signature != "d" || !ok || !node.Range.Set {
			return answerError("org.freedesktop.DBus.Error.PropertyReadOnly", interfaceName+"."+name)
		}
		value := decodeDouble(args)
		if args.Err() != nil || math.IsNaN(value) {
			return answerError("org.freedesktop.DBus.Error.InvalidArgs", "CurrentValue")
		}
		bridge.act(target, platform.AccessibilityRequest{Node: node.ID, Action: platform.AccessibilitySetNumber, Number: min(max(value, node.Range.Min), node.Range.Max)})
		return answerWith("", func(e *dbus.Encoder) {})
	}
	return unknownMethod(call)
}

// answerCache is the whole tree in one reply, so a reader that asks for the
// application does not make a call for each of its objects: for every object
// its reference, the application, its parent, its place among its siblings, the
// number of children, its interfaces, name, role, description and states.
func (bridge *atspiBridge) answerCache() atspiAnswer {
	return answerWith("a((so)(so)(so)iiassusau)", func(e *dbus.Encoder) {
		mark := e.BeginArray(8)
		item := func(ref atspiRef, target atspiTarget) {
			children := bridge.childrenOf(target)
			interfaces := bridge.interfacesOf(target)
			states := bridge.statesOf(target)
			e.Align(8)
			encodeRef(e, ref)
			encodeRef(e, bridge.ref(atspiRootPath))
			encodeRef(e, bridge.parentOf(target))
			e.Int32(bridge.indexInParent(target))
			e.Int32(int32(len(children)))
			names := e.BeginArray(4)
			for _, name := range interfaces {
				e.String(name)
			}
			e.EndArray(names)
			e.String(bridge.nameOf(target))
			e.Uint32(bridge.roleOf(target))
			e.String(bridge.descriptionOf(target))
			set := e.BeginArray(4)
			e.Uint32(states[0])
			e.Uint32(states[1])
			e.EndArray(set)
		}
		item(bridge.ref(atspiRootPath), atspiTarget{index: -1})
		for _, window := range bridge.windows {
			item(bridge.ref(bridge.framePath(window)), atspiTarget{window: window, index: -1})
			for index := range window.nodes {
				item(bridge.refOf(window, index), atspiTarget{window: window, index: index})
			}
		}
		e.EndArray(mark)
	})
}
