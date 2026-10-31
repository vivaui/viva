//go:build unix

package linux

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/vivaui/viva/internal/dbus"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

type atspiSignal struct {
	path, iface, member, detail string
	detail1, detail2            int32
	signature                   string
}

// testBridge is a bridge over one window of nodes, with every request recorded
// and every signal captured.
type testBridge struct {
	*atspiBridge
	window   *atspiWindow
	requests []platform.AccessibilityRequest
	signals  []atspiSignal
}

type fakeAtspiTransport struct {
	messages        []*dbus.Message
	receiveErr      error
	replyErr        error
	replyErrorErr   error
	fd              int
	closed          bool
	replyCalls      int
	replyErrorCalls int
}

func (conn *fakeAtspiTransport) Call(string, string, string, string, string, []byte) (*dbus.Message, error) {
	return nil, errors.New("unexpected AT-SPI call")
}
func (conn *fakeAtspiTransport) Emit(string, string, string, string, []byte) error {
	return nil
}
func (conn *fakeAtspiTransport) UniqueName() string { return ":1.7" }
func (conn *fakeAtspiTransport) FD() int {
	if conn.closed {
		return -1
	}
	return conn.fd
}
func (conn *fakeAtspiTransport) Receive() ([]*dbus.Message, error) {
	if conn.receiveErr != nil {
		return nil, conn.receiveErr
	}
	if conn.closed {
		return nil, errAtspiClosed
	}
	messages := conn.messages
	conn.messages = nil
	return messages, nil
}
func (conn *fakeAtspiTransport) Reply(*dbus.Message, string, []byte) error {
	conn.replyCalls++
	return conn.replyErr
}
func (conn *fakeAtspiTransport) ReplyError(*dbus.Message, string, string) error {
	conn.replyErrorCalls++
	return conn.replyErrorErr
}
func (conn *fakeAtspiTransport) Close() {
	conn.closed = true
	conn.fd = -1
}

func newTestBridge(nodes []platform.AccessibilityNode) *testBridge {
	test := &testBridge{atspiBridge: &atspiBridge{unique: ":1.7", name: "demo", parent: atspiRef{Name: ":1.1", Path: atspiRootPath}}}
	test.send = func(path, iface, member, signature string, body []byte) error {
		args := dbus.NewDecoder(body)
		signal := atspiSignal{path: path, iface: iface, member: member, signature: signature}
		signal.detail, signal.detail1, signal.detail2 = args.String(), args.Int32(), args.Int32()
		test.signals = append(test.signals, signal)
		return nil
	}
	test.window = test.addWindow("Demo", nodes, func() (int, int, float32) { return 100, 50, 2 },
		func(request platform.AccessibilityRequest) bool {
			test.requests = append(test.requests, request)
			return true
		})
	return test
}

func (test *testBridge) call(path, iface, member string, arguments func(e *dbus.Encoder)) atspiAnswer {
	var e dbus.Encoder
	if arguments != nil {
		arguments(&e)
	}
	return test.answer(&dbus.Message{Type: dbus.TypeCall, Path: path, Interface: iface, Member: member, Body: e.Bytes()})
}

func (test *testBridge) path(index int) string {
	return test.nodePath(test.window, test.window.nodes, index)
}

func readRef(d *dbus.Decoder) atspiRef {
	d.Align(8)
	return atspiRef{Name: d.String(), Path: d.String()}
}

func readStrings(d *dbus.Decoder) []string {
	end := d.ArrayLength(4)
	var values []string
	for d.Pos() < end && d.Err() == nil {
		values = append(values, d.String())
	}
	return values
}

func demoNodes() []platform.AccessibilityNode {
	return []platform.AccessibilityNode{
		{Role: platform.A11yRoleListBox, X: 0, Y: 100, Width: 200, Height: 100, Label: "Fruit"},
		{ID: ui.Item(1), Role: platform.A11yRoleOption, X: 0, Y: 100, Width: 200, Height: 20, Label: "Apple", Selected: true, Focusable: true},
		{ID: ui.Item(2), Role: platform.A11yRoleOption, X: 0, Y: 120, Width: 200, Height: 20, Label: "Pear", Focusable: true},
		{ID: ui.Item(3), Role: platform.A11yRoleButton, X: 10, Y: 10, Width: 80, Height: 24, Label: "Save", Focusable: true},
		{ID: ui.Item(4), Role: platform.A11yRoleTextField, X: 10, Y: 40, Width: 150, Height: 24, Label: "Name", Value: "hello wide world", Focusable: true,
			Selection: platform.AccessibilitySelection{Start: 6, End: 6, Valid: true}},
		{ID: ui.Item(5), Role: platform.A11yRoleSlider, X: 10, Y: 70, Width: 150, Height: 16, Label: "Volume", Focusable: true,
			Range: ui.ValueRange{Now: 30, Min: 0, Max: 100, Step: 5, Set: true}},
	}
}

func TestAtspiTreeFollowsContainersAndSurvivesShifts(t *testing.T) {
	test := newTestBridge(demoNodes())
	root := test.call(atspiRootPath, atspiIfaceAccessible, "GetChildren", nil)
	d := dbus.NewDecoder(root.body)
	d.ArrayLength(8)
	if frame := readRef(d); frame.Path != test.framePath(test.window) || frame.Name != ":1.7" {
		t.Fatalf("the application's child = %+v", frame)
	}

	top := test.call(test.framePath(test.window), atspiIfaceAccessible, "GetChildren", nil)
	d = dbus.NewDecoder(top.body)
	d.ArrayLength(8)
	var paths []string
	for end := len(top.body); d.Pos() < end && d.Err() == nil; {
		paths = append(paths, readRef(d).Path)
	}
	if len(paths) != 4 { // the list, the button, the field, the slider
		t.Fatalf("top-level children = %v", paths)
	}

	list := test.call(test.path(0), atspiIfaceAccessible, "GetChildren", nil)
	d = dbus.NewDecoder(list.body)
	d.ArrayLength(8)
	first := readRef(d)
	if !strings.HasSuffix(first.Path, "/i"+strconv.FormatUint(ui.Item(1).Bits(), 10)) {
		t.Fatalf("a control's path must name its identity, got %q", first.Path)
	}

	// The path of a control stays valid when the list shifts under it.
	shifted := append([]platform.AccessibilityNode{{ID: ui.Item(9), Role: platform.A11yRoleButton, Label: "New"}}, demoNodes()...)
	test.update(test.window, shifted)
	answer := test.call(first.Path, atspiIfaceAccessible, "GetRole", nil)
	if role := dbus.NewDecoder(answer.body).Uint32(); role != atspiRoleListItem {
		t.Fatalf("role after a shift = %d", role)
	}
	if gone := test.call(atspiPathPrefix+"w9", atspiIfaceAccessible, "GetRole", nil); gone.errorName == "" {
		t.Error("a window that does not exist answered")
	}
}

func TestAtspiStatesAndRolesFromNodes(t *testing.T) {
	test := newTestBridge(demoNodes())
	state := func(index int) uint64 {
		answer := test.call(test.path(index), atspiIfaceAccessible, "GetState", nil)
		d := dbus.NewDecoder(answer.body)
		d.ArrayLength(4)
		low, high := d.Uint32(), d.Uint32()
		return uint64(high)<<32 | uint64(low)
	}
	if bits := state(1); bits&(1<<atspiStateSelected) == 0 || bits&(1<<atspiStateSelectable) == 0 || bits&(1<<atspiStateFocusable) == 0 {
		t.Errorf("option state = %b", bits)
	}
	if bits := state(2); bits&(1<<atspiStateSelected) != 0 {
		t.Errorf("an unchosen option is selected: %b", bits)
	}
	if bits := state(4); bits&(1<<atspiStateEditable) == 0 || bits&(1<<atspiStateSingleLine) == 0 {
		t.Errorf("text field state = %b", bits)
	}
	for index, want := range map[int]uint32{0: atspiRoleList, 3: atspiRoleButton, 4: atspiRoleEntry, 5: atspiRoleSlider} {
		answer := test.call(test.path(index), atspiIfaceAccessible, "GetRole", nil)
		if role := dbus.NewDecoder(answer.body).Uint32(); role != want {
			t.Errorf("node %d role = %d, want %d", index, role, want)
		}
	}
	if role := atspiRole(platform.AccessibilityNode{Role: platform.A11yRoleTextField, Secure: true}); role != atspiRolePassword {
		t.Errorf("secure field role = %d", role)
	}
}

func TestAtspiInterfacesFollowWhatANodeCanDo(t *testing.T) {
	test := newTestBridge(demoNodes())
	interfaces := func(index int) []string {
		answer := test.call(test.path(index), atspiIfaceAccessible, "GetInterfaces", nil)
		return readStrings(dbus.NewDecoder(answer.body))
	}
	has := func(list []string, name string) bool {
		for _, candidate := range list {
			if candidate == name {
				return true
			}
		}
		return false
	}
	if list := interfaces(3); !has(list, atspiIfaceAction) || has(list, atspiIfaceText) || has(list, atspiIfaceValue) {
		t.Errorf("button interfaces = %v", list)
	}
	if list := interfaces(4); !has(list, atspiIfaceText) || !has(list, atspiIfaceEditableText) {
		t.Errorf("text field interfaces = %v", list)
	}
	if list := interfaces(5); !has(list, atspiIfaceValue) {
		t.Errorf("slider interfaces = %v", list)
	}
	if list := interfaces(0); has(list, atspiIfaceAction) {
		t.Errorf("a list offers an action: %v", list)
	}
}

func TestAtspiComponentExtentsAreInScreenPixels(t *testing.T) {
	test := newTestBridge(demoNodes())
	answer := test.call(test.path(3), atspiIfaceComponent, "GetExtents", func(e *dbus.Encoder) { e.Uint32(0) })
	d := dbus.NewDecoder(answer.body)
	d.Align(8)
	x, y, width, height := d.Int32(), d.Int32(), d.Int32(), d.Int32()
	// Window origin (100,50) at scale 2: logical (10,10,80,24).
	if x != 120 || y != 70 || width != 160 || height != 48 {
		t.Fatalf("extents = %d %d %d %d", x, y, width, height)
	}
	at := test.call(test.framePath(test.window), atspiIfaceComponent, "GetAccessibleAtPoint", func(e *dbus.Encoder) {
		e.Int32(130)
		e.Int32(80)
		e.Uint32(0)
	})
	if ref := readRef(dbus.NewDecoder(at.body)); ref.Path != test.path(3) {
		t.Fatalf("the object under the point = %q", ref.Path)
	}
}

func TestAtspiActionsReachTheApplication(t *testing.T) {
	test := newTestBridge(demoNodes())
	answer := test.call(test.path(3), atspiIfaceAction, "DoAction", func(e *dbus.Encoder) { e.Int32(0) })
	if !dbus.NewDecoder(answer.body).Bool() {
		t.Fatal("DoAction was refused")
	}
	if len(test.requests) != 1 || test.requests[0].Node != ui.Item(3) || test.requests[0].Action != platform.AccessibilityPress {
		t.Fatalf("requests = %+v", test.requests)
	}
	test.requests = nil
	test.call(test.path(5), atspiProperties, "Set", func(e *dbus.Encoder) {
		e.String(atspiIfaceValue)
		e.String("CurrentValue")
		e.Variant("d")
		encodeDouble(e, 55)
	})
	if len(test.requests) != 1 || test.requests[0].Action != platform.AccessibilitySetNumber || test.requests[0].Number != 55 {
		t.Fatalf("slider requests = %+v", test.requests)
	}
	test.requests = nil
	test.call(test.path(4), atspiIfaceEditableText, "SetTextContents", func(e *dbus.Encoder) { e.String("new") })
	test.call(test.path(4), atspiIfaceEditableText, "InsertText", func(e *dbus.Encoder) {
		e.Int32(5)
		e.String("!")
		e.Int32(1)
	})
	if len(test.requests) != 2 || test.requests[0].Text != "new" || test.requests[1].Text != "hello! wide world" {
		t.Fatalf("text requests = %+v", test.requests)
	}
}

func TestAtspiTextReading(t *testing.T) {
	test := newTestBridge(demoNodes())
	path := test.path(4)
	text := test.call(path, atspiIfaceText, "GetText", func(e *dbus.Encoder) { e.Int32(0); e.Int32(5) })
	if got := dbus.NewDecoder(text.body).String(); got != "hello" {
		t.Errorf("GetText = %q", got)
	}
	word := test.call(path, atspiIfaceText, "GetStringAtOffset", func(e *dbus.Encoder) { e.Int32(7); e.Uint32(atspiGranularityWord) })
	d := dbus.NewDecoder(word.body)
	if got, start, end := d.String(), d.Int32(), d.Int32(); got != "wide " || start != 6 || end != 11 {
		t.Errorf("word at 7 = %q %d..%d", got, start, end)
	}
	before := test.call(path, atspiIfaceText, "GetTextBeforeOffset", func(e *dbus.Encoder) { e.Int32(7); e.Uint32(1) })
	d = dbus.NewDecoder(before.body)
	if got := d.String(); got != "hello " {
		t.Errorf("word before = %q", got)
	}
	props := test.call(path, atspiProperties, "Get", func(e *dbus.Encoder) { e.String(atspiIfaceText); e.String("CaretOffset") })
	d = dbus.NewDecoder(props.body)
	if signature := d.Signature(); signature != "i" || d.Int32() != 6 {
		t.Errorf("caret offset = %q", signature)
	}
}

func TestAtspiEventsNameWhatChanged(t *testing.T) {
	test := newTestBridge(demoNodes())
	test.signals = nil
	changed := demoNodes()
	changed[3].Focused = true
	changed[4].Value = "hello brave world"
	changed[4].Selection = platform.AccessibilitySelection{Start: 11, End: 11, Valid: true}
	changed[5].Range.Now = 35
	changed = changed[:5+1]
	changed = append(changed, platform.AccessibilityNode{ID: ui.Item(7), Role: platform.A11yRoleButton, Label: "Added"})
	test.update(test.window, changed)

	seen := map[string]atspiSignal{}
	for _, signal := range test.signals {
		seen[signal.member+":"+signal.detail] = signal
	}
	for _, key := range []string{
		"StateChanged:focused", "TextChanged:delete", "TextChanged:insert", "TextCaretMoved:",
		"PropertyChange:accessible-value", "ChildrenChanged:add",
	} {
		if _, ok := seen[key]; !ok {
			t.Errorf("no %s event; got %v", key, test.signals)
		}
	}
	if focus := seen["StateChanged:focused"]; focus.detail1 != 1 || focus.path != test.path(3) {
		t.Errorf("focus event = %+v", focus)
	}
	if insert := seen["TextChanged:insert"]; insert.detail1 != 6 || insert.detail2 != 4 {
		t.Errorf("insert event = %+v", insert)
	}
	if seen["StateChanged:focused"].signature != "siiva{sv}" {
		t.Errorf("signal signature = %q", seen["StateChanged:focused"].signature)
	}

	test.signals = nil
	test.update(test.window, changed)
	if len(test.signals) != 0 {
		t.Errorf("an unchanged tree announced %v", test.signals)
	}
}

func TestAtspiLiveRegionAnnounces(t *testing.T) {
	test := newTestBridge([]platform.AccessibilityNode{{Role: platform.A11yRoleStatus, Label: "Saved", X: 0, Y: 0, Width: 10, Height: 10, Live: ui.LivePolite}})
	test.signals = nil
	test.update(test.window, []platform.AccessibilityNode{{Role: platform.A11yRoleStatus, Label: "Saving failed", X: 0, Y: 0, Width: 10, Height: 10, Live: ui.LiveAssertive}})
	var announced bool
	for _, signal := range test.signals {
		if signal.member == "Announcement" && signal.detail == "Saving failed" && signal.detail1 == 2 {
			announced = true
		}
	}
	if !announced {
		t.Fatalf("no announcement: %v", test.signals)
	}
}

func TestAtspiServeStopsAfterReplyFailure(t *testing.T) {
	call := &dbus.Message{Type: dbus.TypeCall, Path: atspiRootPath, Interface: atspiIntrospect, Member: "Introspect"}
	conn := &fakeAtspiTransport{
		messages: []*dbus.Message{call, call},
		replyErr: errors.New("reply write failed"),
		fd:       42,
	}
	bridge := newAtspiBridge(conn)
	if err := bridge.serve(); !errors.Is(err, errAtspiClosed) {
		t.Fatalf("serve error = %v, want errAtspiClosed", err)
	}
	if conn.replyCalls != 1 || !conn.closed || bridge.conn != nil {
		t.Fatalf("batch continued after failure: replies=%d closed=%v conn=%v", conn.replyCalls, conn.closed, bridge.conn != nil)
	}
}

func TestAtspiErrorReplyFailureClosesBridge(t *testing.T) {
	call := &dbus.Message{Type: dbus.TypeCall, Path: "/missing", Interface: atspiIntrospect, Member: "Introspect"}
	conn := &fakeAtspiTransport{
		messages:      []*dbus.Message{call},
		replyErrorErr: errors.New("error reply write failed"),
		fd:            42,
	}
	bridge := newAtspiBridge(conn)
	if err := bridge.serve(); !errors.Is(err, errAtspiClosed) {
		t.Fatalf("serve error = %v, want errAtspiClosed", err)
	}
	if conn.replyErrorCalls != 1 || !conn.closed || bridge.conn != nil {
		t.Fatalf("error reply failure was ignored: replies=%d closed=%v conn=%v", conn.replyErrorCalls, conn.closed, bridge.conn != nil)
	}
}

func TestAtspiCacheListsEveryObjectOnce(t *testing.T) {
	test := newTestBridge(demoNodes())
	call := &dbus.Message{Type: dbus.TypeCall, Path: atspiCachePath, Interface: atspiIfaceCache, Member: "GetItems"}
	answer := test.answer(call)
	if answer.errorName != "" || answer.signature != "a((so)(so)(so)iiassusau)" {
		t.Fatalf("answer = %+v", answer)
	}
	d := dbus.NewDecoder(answer.body)
	end := d.ArrayLength(8)
	var paths []string
	for d.Pos() < end && d.Err() == nil {
		d.Align(8)
		reference := readRef(d)
		application := readRef(d)
		parent := readRef(d)
		index, children := d.Int32(), d.Int32()
		interfaces := readStrings(d)
		name, role, description := d.String(), d.Uint32(), d.String()
		d.ArrayLength(4)
		d.Uint32()
		d.Uint32()
		if d.Err() != nil {
			t.Fatalf("decoding: %v", d.Err())
		}
		paths = append(paths, reference.Path)
		if application.Path != atspiRootPath || len(interfaces) == 0 || role == 0 || description != "" && name == "" {
			t.Errorf("item %q: app %v interfaces %v role %d", reference.Path, application, interfaces, role)
		}
		if reference.Path == test.path(1) && (parent.Path != test.path(0) || index != 0 || children != 0 || name != "Apple") {
			t.Errorf("the first option = parent %q index %d children %d name %q", parent.Path, index, children, name)
		}
	}
	// The application, the window and the six nodes.
	if len(paths) != 8 {
		t.Fatalf("items = %d: %v", len(paths), paths)
	}
}
