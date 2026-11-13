//go:build unix

package ibus

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/vivaui/viva/internal/dbus"
)

// fakeTransport is a daemon the test scripts: it records the calls the client
// sends and hands back the messages the test queues.
type fakeTransport struct {
	serial   uint32
	sent     []sentCall
	incoming []*dbus.Message
	closed   bool
	sendErr  error
	// createPath is what CreateInputContext answers.
	createPath string
}

type sentCall struct {
	serial                         uint32
	path, iface, member, signature string
	body                           []byte
}

func (f *fakeTransport) Call(destination, path, iface, member, signature string, body []byte) (*dbus.Message, error) {
	f.sent = append(f.sent, sentCall{path: path, iface: iface, member: member, signature: signature, body: body})
	switch member {
	case "CreateInputContext":
		var reply dbus.Encoder
		reply.String(f.createPath)
		return &dbus.Message{Type: dbus.TypeReturn, Signature: "o", Body: reply.Bytes()}, nil
	case "AddMatch":
		return &dbus.Message{Type: dbus.TypeReturn}, nil
	}
	return nil, errors.New("unexpected call " + member)
}

func (f *fakeTransport) Send(destination, path, iface, member, signature string, body []byte) (uint32, error) {
	if f.sendErr != nil {
		return 0, f.sendErr
	}
	f.serial++
	f.sent = append(f.sent, sentCall{serial: f.serial, path: path, iface: iface, member: member, signature: signature, body: body})
	return f.serial, nil
}

func (f *fakeTransport) Receive() ([]*dbus.Message, error) {
	messages := f.incoming
	f.incoming = nil
	return messages, nil
}

func (f *fakeTransport) FD() int { return 7 }
func (f *fakeTransport) Close()  { f.closed = true }

func (f *fakeTransport) calls(member string) []sentCall {
	var found []sentCall
	for _, call := range f.sent {
		if call.member == member {
			found = append(found, call)
		}
	}
	return found
}

// ibusText encodes an IBusText the way the daemon sends one.
func ibusText(e *dbus.Encoder, text string) {
	e.Variant("(sa{sv}sv)")
	e.Align(8)
	e.String("IBusText")
	empty := e.BeginArray(8)
	e.EndArray(empty)
	e.String(text)
	e.Variant("(sa{sv}av)")
	e.Align(8)
	e.String("IBusAttrList")
	attachments := e.BeginArray(8)
	e.EndArray(attachments)
	attributes := e.BeginArray(1)
	e.EndArray(attributes)
}

func commitSignal(path, text string) *dbus.Message {
	var body dbus.Encoder
	ibusText(&body, text)
	return &dbus.Message{Type: dbus.TypeSignal, Path: path, Interface: contextInterface, Member: "CommitText", Signature: "v", Body: body.Bytes()}
}

func preeditSignal(path, text string, cursor uint32, visible bool) *dbus.Message {
	var body dbus.Encoder
	ibusText(&body, text)
	body.Uint32(cursor)
	body.Bool(visible)
	return &dbus.Message{Type: dbus.TypeSignal, Path: path, Interface: contextInterface, Member: "UpdatePreeditText", Signature: "vub", Body: body.Bytes()}
}

func preeditModeSignal(path, value string, cursor uint32, visible bool, mode uint32) *dbus.Message {
	var body dbus.Encoder
	ibusText(&body, value)
	body.Uint32(cursor)
	body.Bool(visible)
	body.Uint32(mode)
	return &dbus.Message{Type: dbus.TypeSignal, Path: path, Interface: contextInterface, Member: "UpdatePreeditTextWithMode", Signature: "vubu", Body: body.Bytes()}
}

func keyAnswer(serial uint32, handled bool) *dbus.Message {
	var body dbus.Encoder
	body.Bool(handled)
	return &dbus.Message{Type: dbus.TypeReturn, ReplySerial: serial, Signature: "b", Body: body.Bytes()}
}

const testContext = "/org/freedesktop/IBus/InputContext_1"

func newTestBus(t *testing.T, handler Handler) (*Bus, *Context, *fakeTransport) {
	t.Helper()
	transport := &fakeTransport{createPath: testContext}
	bus := newBus(transport)
	context, err := bus.NewContext("viva", handler)
	if err != nil {
		t.Fatal(err)
	}
	return bus, context, transport
}

func TestNewContextAsksForPreeditFocusAndSurroundingTextCapabilities(t *testing.T) {
	_, _, transport := newTestBus(t, Handler{})
	create := transport.calls("CreateInputContext")
	if len(create) != 1 || dbus.NewDecoder(create[0].body).String() != "viva" {
		t.Fatalf("CreateInputContext = %+v", create)
	}
	capabilities := transport.calls("SetCapabilities")
	if len(capabilities) != 1 || capabilities[0].path != testContext {
		t.Fatalf("SetCapabilities = %+v", capabilities)
	}
	if got := dbus.NewDecoder(capabilities[0].body).Uint32(); got != CapPreeditText|CapFocus|CapSurroundingText {
		t.Fatalf("capabilities = %#x; the client draws no candidate list, so it must not ask for one", got)
	}
}

func TestKeyAnswersArriveInTheOrderTheKeysWereTyped(t *testing.T) {
	bus, context, transport := newTestBus(t, Handler{})
	var order []string
	for _, name := range []string{"n", "i", "h"} {
		if err := context.ProcessKey(uint32(name[0]), 0x31, 0, func(handled bool) {
			order = append(order, name+map[bool]string{true: "+", false: "-"}[handled])
		}); err != nil {
			t.Fatal(err)
		}
	}
	keys := transport.calls("ProcessKeyEvent")
	if len(keys) != 3 {
		t.Fatalf("%d keys sent", len(keys))
	}
	// The daemon answers the second key first; the application must still see
	// n, then i, then h.
	transport.incoming = []*dbus.Message{keyAnswer(keys[1].serial, true)}
	if err := bus.Dispatch(); err != nil {
		t.Fatal(err)
	}
	if len(order) != 0 {
		t.Fatalf("a key was released before the one typed before it: %v", order)
	}
	transport.incoming = []*dbus.Message{keyAnswer(keys[0].serial, false), keyAnswer(keys[2].serial, true)}
	if err := bus.Dispatch(); err != nil {
		t.Fatal(err)
	}
	if want := []string{"n-", "i+", "h+"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("keys reached the application as %v, want %v", order, want)
	}
}

func TestKeyBodyCarriesKeysymKeycodeAndState(t *testing.T) {
	_, context, transport := newTestBus(t, Handler{})
	if err := context.ProcessKey(0x6e, 57, 0x5, nil); err != nil {
		t.Fatal(err)
	}
	if err := context.ProcessKey(0xffe1, 50, 0x1|ReleaseMask, nil); err != nil {
		t.Fatal(err)
	}
	keys := transport.calls("ProcessKeyEvent")
	for index, want := range [][3]uint32{{0x6e, 57, 0x5}, {0xffe1, 50, 0x1 | ReleaseMask}} {
		args := dbus.NewDecoder(keys[index].body)
		if got := [3]uint32{args.Uint32(), args.Uint32(), args.Uint32()}; got != want || keys[index].signature != "uuu" {
			t.Errorf("key %d = %v (%s), want %v", index, got, keys[index].signature, want)
		}
	}
}

func TestAnErrorReplyMeansNotHandled(t *testing.T) {
	bus, context, transport := newTestBus(t, Handler{})
	var verdict *bool
	if err := context.ProcessKey(0x61, 38, 0, func(handled bool) { verdict = &handled }); err != nil {
		t.Fatal(err)
	}
	transport.incoming = []*dbus.Message{{Type: dbus.TypeError, ReplySerial: transport.serial, ErrorName: "org.freedesktop.DBus.Error.Failed"}}
	if err := bus.Dispatch(); err != nil {
		t.Fatal(err)
	}
	if verdict == nil || *verdict {
		t.Fatalf("verdict = %v, want not handled", verdict)
	}
}

func TestCommitAndPreeditSignalsReachTheHandler(t *testing.T) {
	var commits []string
	type preedit struct {
		text    string
		cursor  int
		visible bool
	}
	var preedits []preedit
	bus, _, transport := newTestBus(t, Handler{
		Commit: func(text string) { commits = append(commits, text) },
		Preedit: func(event PreeditEvent) {
			preedits = append(preedits, preedit{event.Text, event.Cursor, event.Visible})
		},
	})
	hide := &dbus.Message{Type: dbus.TypeSignal, Path: testContext, Interface: contextInterface, Member: "HidePreeditText"}
	show := &dbus.Message{Type: dbus.TypeSignal, Path: testContext, Interface: contextInterface, Member: "ShowPreeditText"}
	transport.incoming = []*dbus.Message{
		preeditSignal(testContext, "nǐ hǎo", 3, true),
		hide,
		show,
		commitSignal(testContext, "你好"),
		preeditSignal(testContext, "", 0, false),
		// A signal for another context, or from another interface, is not ours.
		commitSignal("/org/freedesktop/IBus/InputContext_9", "other"),
		{Type: dbus.TypeSignal, Path: testContext, Interface: "org.example.Other", Member: "CommitText"},
	}
	if err := bus.Dispatch(); err != nil {
		t.Fatal(err)
	}
	wantPreedit := []preedit{{"nǐ hǎo", 3, true}, {"nǐ hǎo", 3, false}, {"nǐ hǎo", 3, true}, {"", 0, false}}
	if !reflect.DeepEqual(preedits, wantPreedit) {
		t.Fatalf("preedit = %+v, want %+v", preedits, wantPreedit)
	}
	if !reflect.DeepEqual(commits, []string{"你好"}) {
		t.Fatalf("commits = %q", commits)
	}
}

func TestPreeditWithModeSignalsReachTheHandler(t *testing.T) {
	var got []string
	var events []PreeditEvent
	bus, _, transport := newTestBus(t, Handler{Preedit: func(event PreeditEvent) {
		got = append(got, fmt.Sprintf("%s/%t", event.Text, event.Visible))
		events = append(events, event)
	}})
	update := preeditModeSignal(testContext, "n", 1, true, PreeditModeCommit)
	transport.incoming = []*dbus.Message{
		update,
		{Type: dbus.TypeSignal, Path: testContext, Interface: contextInterface, Member: "HidePreeditText"},
		{Type: dbus.TypeSignal, Path: testContext, Interface: contextInterface, Member: "ShowPreeditText"},
	}
	if err := bus.Dispatch(); err != nil {
		t.Fatal(err)
	}
	if want := []string{"n/true", "n/false", "n/true"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("mode-bearing preedit = %v, want %v", got, want)
	}
	for index, event := range events {
		if event.Kind != []PreeditKind{PreeditUpdate, PreeditHide, PreeditShow}[index] ||
			!event.HasMode || event.Mode != PreeditModeCommit || event.Cursor != 1 {
			t.Fatalf("preedit provenance[%d] = %+v", index, event)
		}
	}
}

func TestPreeditEventsPreserveEmptyUpdatesAndResetModeProvenance(t *testing.T) {
	var events []PreeditEvent
	bus, _, transport := newTestBus(t, Handler{Preedit: func(event PreeditEvent) { events = append(events, event) }})
	transport.incoming = []*dbus.Message{
		preeditModeSignal(testContext, "n", 1, true, PreeditModeCommit),
		preeditSignal(testContext, "", 0, true),
		{Type: dbus.TypeSignal, Path: testContext, Interface: contextInterface, Member: "HidePreeditText"},
		preeditModeSignal(testContext, "", 0, false, ^uint32(0)),
	}
	if err := bus.Dispatch(); err != nil {
		t.Fatal(err)
	}
	want := []PreeditEvent{
		{Text: "n", Cursor: 1, Visible: true, Mode: PreeditModeCommit, HasMode: true},
		{Text: "", Visible: true},
		{Text: "", Kind: PreeditHide},
		{Text: "", Mode: ^uint32(0), HasMode: true},
	}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("preedit provenance = %+v, want %+v", events, want)
	}
}

func TestMalformedPreeditSignalCannotReplaceTheCachedSnapshot(t *testing.T) {
	for _, name := range []string{"missing mode", "truncated mode", "mode on old member", "trailing body", "hide payload", "show signature"} {
		t.Run(name, func(t *testing.T) {
			var events []PreeditEvent
			bus, _, transport := newTestBus(t, Handler{Preedit: func(event PreeditEvent) { events = append(events, event) }})
			bad := preeditModeSignal(testContext, "bad", 1, true, PreeditModeClear)
			switch name {
			case "missing mode":
				bad = preeditSignal(testContext, "bad", 1, true)
				bad.Member = "UpdatePreeditTextWithMode"
			case "truncated mode":
				bad.Body = bad.Body[:len(bad.Body)-1]
			case "mode on old member":
				bad.Member = "UpdatePreeditText"
			case "trailing body":
				bad.Body = append(bad.Body, 0, 0, 0, 0)
			case "hide payload":
				bad.Member = "HidePreeditText"
			case "show signature":
				bad.Member, bad.Signature, bad.Body = "ShowPreeditText", "u", nil
			}
			transport.incoming = []*dbus.Message{
				preeditModeSignal(testContext, "keep", 2, true, PreeditModeCommit), bad,
				{Type: dbus.TypeSignal, Path: testContext, Interface: contextInterface, Member: "ShowPreeditText"},
			}
			if err := bus.Dispatch(); err != nil {
				t.Fatal(err)
			}
			if len(events) != 2 || events[1].Kind != PreeditShow || events[1].Text != "keep" ||
				!events[1].HasMode || events[1].Mode != PreeditModeCommit || events[1].Cursor != 2 {
				t.Fatalf("malformed preedit changed cached provenance: %+v", events)
			}
		})
	}
}

// styledPreeditSignal is an UpdatePreeditText whose text carries attributes, as an
// engine marking the clause being converted sends it.
func styledPreeditSignal(path, text string, cursor uint32, attrs [][4]uint32) *dbus.Message {
	var body dbus.Encoder
	body.Variant("(sa{sv}sv)")
	body.Align(8)
	body.String("IBusText")
	none := body.BeginArray(8)
	body.EndArray(none)
	body.String(text)
	body.Variant("(sa{sv}av)")
	body.Align(8)
	body.String("IBusAttrList")
	none = body.BeginArray(8)
	body.EndArray(none)
	list := body.BeginArray(1)
	for _, attr := range attrs {
		body.Variant("(sa{sv}uuuu)")
		body.Align(8)
		body.String("IBusAttribute")
		none = body.BeginArray(8)
		body.EndArray(none)
		for _, value := range attr {
			body.Uint32(value)
		}
	}
	body.EndArray(list)
	body.Uint32(cursor)
	body.Bool(true)
	return &dbus.Message{Type: dbus.TypeSignal, Path: path, Interface: contextInterface, Member: "UpdatePreeditText", Signature: "vub", Body: body.Bytes()}
}

func TestPreeditAttributesReachTheHandler(t *testing.T) {
	var got []Attribute
	var gotText string
	bus, _, transport := newTestBus(t, Handler{Preedit: func(event PreeditEvent) { gotText, got = event.Text, event.Attributes }})
	transport.incoming = []*dbus.Message{styledPreeditSignal(testContext, "きょうは", 4, [][4]uint32{
		{AttrUnderline, 1, 0, 4},
		{AttrBackground, 0xd1eaff, 0, 2},
	})}
	if err := bus.Dispatch(); err != nil {
		t.Fatal(err)
	}
	want := []Attribute{{Kind: AttrUnderline, Value: 1, Start: 0, End: 4}, {Kind: AttrBackground, Value: 0xd1eaff, Start: 0, End: 2}}
	if gotText != "きょうは" || !reflect.DeepEqual(got, want) {
		t.Fatalf("preedit %q attributes %+v, want %+v", gotText, got, want)
	}
}

func TestPreeditModeVisibilityKeepsReadOnlyAttributeSnapshots(t *testing.T) {
	var events []PreeditEvent
	bus, _, transport := newTestBus(t, Handler{Preedit: func(event PreeditEvent) { events = append(events, event) }})
	update := styledPreeditSignal(testContext, "ab", 1, [][4]uint32{{AttrBackground, 0x123456, 0, 1}})
	var mode dbus.Encoder
	mode.Uint32(PreeditModeCommit)
	update.Member, update.Signature = "UpdatePreeditTextWithMode", "vubu"
	update.Body = append(update.Body, mode.Bytes()...)
	transport.incoming = []*dbus.Message{
		update,
		{Type: dbus.TypeSignal, Path: testContext, Interface: contextInterface, Member: "HidePreeditText"},
		{Type: dbus.TypeSignal, Path: testContext, Interface: contextInterface, Member: "ShowPreeditText"},
		styledPreeditSignal(testContext, "cd", 2, [][4]uint32{{AttrUnderline, 2, 1, 2}}),
	}
	if err := bus.Dispatch(); err != nil {
		t.Fatal(err)
	}
	if len(events) != 4 {
		t.Fatalf("preedit events = %+v", events)
	}
	for index, event := range events[:3] {
		if !event.HasMode || event.Mode != PreeditModeCommit || event.Text != "ab" ||
			event.Kind != []PreeditKind{PreeditUpdate, PreeditHide, PreeditShow}[index] ||
			!reflect.DeepEqual(event.Attributes, []Attribute{{Kind: AttrBackground, Value: 0x123456, Start: 0, End: 1}}) {
			t.Fatalf("saved preedit attributes[%d] changed: %+v", index, event)
		}
	}
	if events[3].HasMode || events[3].Text != "cd" || !reflect.DeepEqual(events[3].Attributes, []Attribute{{Kind: AttrUnderline, Value: 2, Start: 1, End: 2}}) {
		t.Fatalf("new update retained old metadata: %+v", events[3])
	}
}

var preeditBenchmarkText string
var preeditBenchmarkContext *Context

func BenchmarkPreeditSignalDispatch(b *testing.B) {
	context := &Context{handler: Handler{Preedit: func(event PreeditEvent) { preeditBenchmarkText = event.Text }}}
	message := preeditSignal(testContext, "preedit", 3, true)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		context.signal(message)
	}
	b.ReportMetric(float64(unsafe.Sizeof(Context{})), "context-B")
	b.ReportMetric(float64(unsafe.Sizeof(Handler{})), "handler-B")
}

func BenchmarkPreeditContextConstruction(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		preeditBenchmarkContext = new(Context)
	}
}

func TestDeleteSurroundingTextReachesTheHandler(t *testing.T) {
	var offset, count int
	bus, _, transport := newTestBus(t, Handler{DeleteSurrounding: func(o, c int) { offset, count = o, c }})
	var body dbus.Encoder
	body.Int32(-3)
	body.Uint32(2)
	transport.incoming = []*dbus.Message{{Type: dbus.TypeSignal, Path: testContext, Interface: contextInterface, Member: "DeleteSurroundingText", Signature: "iu", Body: body.Bytes()}}
	if err := bus.Dispatch(); err != nil {
		t.Fatal(err)
	}
	if offset != -3 || count != 2 {
		t.Fatalf("delete surrounding = %d, %d", offset, count)
	}
}

func TestSetSurroundingTextSendsAnIBusTextAndTheCaretAndAnchor(t *testing.T) {
	_, context, transport := newTestBus(t, Handler{})
	if err := context.SetSurroundingText("日本語", 2, 1); err != nil {
		t.Fatal(err)
	}
	calls := transport.calls("SetSurroundingText")
	if len(calls) != 1 || calls[0].signature != "vuu" {
		t.Fatalf("SetSurroundingText = %+v", calls)
	}
	args := dbus.NewDecoder(calls[0].body)
	text, ok := readText(args, "v")
	if !ok || text != "日本語" {
		t.Fatalf("text = %q, %v", text, ok)
	}
	if cursor, anchor := args.Uint32(), args.Uint32(); cursor != 2 || anchor != 1 || args.Err() != nil {
		t.Fatalf("cursor %d anchor %d err %v", cursor, anchor, args.Err())
	}
}

func TestForwardKeyEventReachesTheHandler(t *testing.T) {
	var got [3]uint32
	bus, _, transport := newTestBus(t, Handler{Forward: func(keysym, keycode, state uint32) { got = [3]uint32{keysym, keycode, state} }})
	var body dbus.Encoder
	body.Uint32(0xff0d)
	body.Uint32(28)
	body.Uint32(0)
	transport.incoming = []*dbus.Message{{Type: dbus.TypeSignal, Path: testContext, Interface: contextInterface, Member: "ForwardKeyEvent", Signature: "uuu", Body: body.Bytes()}}
	if err := bus.Dispatch(); err != nil {
		t.Fatal(err)
	}
	if got != [3]uint32{0xff0d, 28, 0} {
		t.Fatalf("forwarded key = %#v", got)
	}
}

func TestMalformedSignalsAreIgnoredNotFatal(t *testing.T) {
	called := false
	bus, _, transport := newTestBus(t, Handler{
		Commit:  func(string) { called = true },
		Preedit: func(PreeditEvent) { called = true },
		Forward: func(_, _, _ uint32) { called = true },
	})
	truncated := commitSignal(testContext, "你好")
	truncated.Body = truncated.Body[:len(truncated.Body)/2]
	notText := &dbus.Message{Type: dbus.TypeSignal, Path: testContext, Interface: contextInterface, Member: "CommitText", Signature: "s", Body: []byte{0, 0, 0, 0, 0}}
	shortPreedit := preeditSignal(testContext, "x", 0, true)
	shortPreedit.Body = shortPreedit.Body[:len(shortPreedit.Body)-12]
	transport.incoming = []*dbus.Message{truncated, notText, shortPreedit, {Type: dbus.TypeSignal, Path: testContext, Interface: contextInterface, Member: "ForwardKeyEvent", Signature: "uuu", Body: []byte{1}}}
	if err := bus.Dispatch(); err != nil {
		t.Fatalf("a malformed signal broke the bus: %v", err)
	}
	if called {
		t.Fatal("a handler ran for a signal that could not be read")
	}
}

func TestADaemonThatStopsAnsweringReleasesEveryKeyAndClosesTheBus(t *testing.T) {
	bus, context, transport := newTestBus(t, Handler{})
	now := time.Unix(1000, 0)
	bus.now = func() time.Time { return now }
	var verdicts []bool
	for range 2 {
		if err := context.ProcessKey(0x61, 38, 0, func(handled bool) { verdicts = append(verdicts, handled) }); err != nil {
			t.Fatal(err)
		}
	}
	deadline, waiting := bus.Deadline()
	if !waiting || !deadline.Equal(now.Add(keyTimeout)) {
		t.Fatalf("deadline = %v, %v", deadline, waiting)
	}
	now = now.Add(keyTimeout - time.Millisecond)
	bus.Expire()
	if bus.Err() != nil || len(verdicts) != 0 {
		t.Fatal("the bus gave up before the deadline")
	}
	now = now.Add(time.Millisecond)
	bus.Expire()
	if bus.Err() == nil || !transport.closed || !reflect.DeepEqual(verdicts, []bool{false, false}) {
		t.Fatalf("err=%v closed=%v verdicts=%v", bus.Err(), transport.closed, verdicts)
	}
	if err := context.ProcessKey(0x61, 38, 0, nil); !errors.Is(err, ErrClosed) {
		t.Fatalf("a closed bus accepted a key: %v", err)
	}
	if _, waiting := bus.Deadline(); waiting {
		t.Fatal("a closed bus still has a deadline")
	}
}

func TestAFailedSendClosesTheBusAndReleasesPendingKeys(t *testing.T) {
	bus, context, transport := newTestBus(t, Handler{})
	released := false
	if err := context.ProcessKey(0x61, 38, 0, func(handled bool) { released = !handled }); err != nil {
		t.Fatal(err)
	}
	transport.sendErr = errors.New("broken pipe")
	if err := context.FocusIn(); err == nil {
		t.Fatal("a failed send reported success")
	}
	if bus.Err() == nil || !released || !transport.closed {
		t.Fatalf("err=%v released=%v closed=%v", bus.Err(), released, transport.closed)
	}
}

func TestContextCalls(t *testing.T) {
	_, context, transport := newTestBus(t, Handler{})
	if err := context.FocusIn(); err != nil {
		t.Fatal(err)
	}
	if err := context.SetCursorLocation(100, -20, 2, 18); err != nil {
		t.Fatal(err)
	}
	if err := context.Reset(); err != nil {
		t.Fatal(err)
	}
	if err := context.FocusOut(); err != nil {
		t.Fatal(err)
	}
	context.Close()
	var members []string
	for _, call := range transport.sent {
		if call.path == testContext {
			members = append(members, call.member)
		}
	}
	if want := []string{"SetCapabilities", "FocusIn", "SetCursorLocation", "Reset", "FocusOut", "Destroy"}; !reflect.DeepEqual(members, want) {
		t.Fatalf("calls = %v, want %v", members, want)
	}
	location := dbus.NewDecoder(transport.calls("SetCursorLocation")[0].body)
	if got := [4]int32{location.Int32(), location.Int32(), location.Int32(), location.Int32()}; got != [4]int32{100, -20, 2, 18} {
		t.Fatalf("cursor location = %v", got)
	}
}

func TestAddressPrefersTheEnvironmentThenTheDisplaysFile(t *testing.T) {
	files := map[string]string{
		"/var/lib/dbus/machine-id":               "abc123\n",
		"/home/u/.config/ibus/bus/abc123-unix-1": "# comment\nIBUS_ADDRESS=unix:path=/home/u/.cache/ibus/dbus-1,guid=g\nIBUS_DAEMON_PID=99\n",
		"/xdg/ibus/bus/abc123-remote-0":          "IBUS_ADDRESS=unix:abstract=/tmp/dbus-x,guid=h\n",
		"/home/u/.config/ibus/bus/abc123-unix-0": "IBUS_DAEMON_PID=1\n",
	}
	read := func(name string) ([]byte, error) {
		if data, ok := files[name]; ok {
			return []byte(data), nil
		}
		return nil, os.ErrNotExist
	}
	env := func(values map[string]string) func(string) string {
		return func(key string) string { return values[key] }
	}
	for _, test := range []struct {
		name string
		env  map[string]string
		want string
		ok   bool
	}{
		{"environment", map[string]string{"IBUS_ADDRESS": "unix:path=/x", "DISPLAY": ":1", "HOME": "/home/u"}, "unix:path=/x", true},
		{"local display", map[string]string{"DISPLAY": ":1", "HOME": "/home/u"}, "unix:path=/home/u/.cache/ibus/dbus-1,guid=g", true},
		{"screen suffix is ignored", map[string]string{"DISPLAY": ":1.0", "HOME": "/home/u"}, "unix:path=/home/u/.cache/ibus/dbus-1,guid=g", true},
		{"XDG_CONFIG_HOME and a host", map[string]string{"DISPLAY": "remote:0", "XDG_CONFIG_HOME": "/xdg"}, "unix:abstract=/tmp/dbus-x,guid=h", true},
		{"file without an address", map[string]string{"DISPLAY": ":0", "HOME": "/home/u"}, "", false},
		{"no file for this display", map[string]string{"DISPLAY": ":7", "HOME": "/home/u"}, "", false},
		{"no home", map[string]string{"DISPLAY": ":1"}, "", false},
	} {
		got, ok := Address(env(test.env), read)
		if got != test.want || ok != test.ok {
			t.Errorf("%s: Address = %q, %v; want %q, %v", test.name, got, ok, test.want, test.ok)
		}
	}
	if _, ok := Address(func(string) string { return "" }, func(string) ([]byte, error) { return nil, os.ErrNotExist }); ok {
		t.Error("an address was found with no machine id")
	}
}

func TestDisplayParts(t *testing.T) {
	for display, want := range map[string][2]string{
		"":         {"unix", "0"},
		":0":       {"unix", "0"},
		":10.2":    {"unix", "10"},
		"host:3.0": {"host", "3"},
		"nonsense": {"unix", "0"},
		"host:":    {"host", "0"},
	} {
		if host, number := displayParts(display); [2]string{host, number} != want {
			t.Errorf("displayParts(%q) = %q, %q; want %v", display, host, number, want)
		}
	}
}

func FuzzDispatchNeverPanics(f *testing.F) {
	good := commitSignal(testContext, "你好")
	f.Add(good.Body, "v", "CommitText")
	preedit := preeditSignal(testContext, "ni", 1, true)
	f.Add(preedit.Body, "vub", "UpdatePreeditText")
	f.Add(styledPreeditSignal(testContext, "ni", 1, [][4]uint32{{AttrBackground, 1, 0, 2}}).Body, "vub", "UpdatePreeditText")
	f.Add(preeditModeSignal(testContext, "ni", 1, true, PreeditModeCommit).Body, "vubu", "UpdatePreeditTextWithMode")
	f.Add([]byte{1, 2, 3}, "uuu", "ForwardKeyEvent")
	f.Add([]byte{1, 2, 3}, "iu", "DeleteSurroundingText")
	f.Fuzz(func(t *testing.T, body []byte, signature, member string) {
		var seen int
		bus, _, transport := newTestBus(t, Handler{
			Commit:  func(string) { seen++ },
			Preedit: func(PreeditEvent) { seen++ },
			Forward: func(_, _, _ uint32) { seen++ },

			DeleteSurrounding: func(_, _ int) { seen++ },
		})
		transport.incoming = []*dbus.Message{{Type: dbus.TypeSignal, Path: testContext, Interface: contextInterface, Member: member, Signature: signature, Body: body}}
		if err := bus.Dispatch(); err != nil {
			t.Fatalf("a signal broke the bus: %v", err)
		}
	})
}
