//go:build unix

package fcitx

import (
	"errors"
	"testing"
	"time"

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
	id       int32
}

type sentCall struct {
	serial                         uint32
	path, iface, member, signature string
	body                           []byte
}

func (f *fakeTransport) Call(destination, path, iface, member, signature string, body []byte) (*dbus.Message, error) {
	f.sent = append(f.sent, sentCall{path: path, iface: iface, member: member, signature: signature, body: body})
	switch member {
	case "AddMatch", "SetCapacity":
		return &dbus.Message{Type: dbus.TypeReturn}, nil
	case "CreateICv3":
		var reply dbus.Encoder
		reply.Int32(f.id)
		reply.Bool(true)
		reply.Uint32(32)
		reply.Uint32(4)
		reply.Uint32(0)
		reply.Uint32(0)
		return &dbus.Message{Type: dbus.TypeReturn, Signature: "ibuuuu", Body: reply.Bytes()}, nil
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

func commitSignal(path, text string) *dbus.Message {
	var body dbus.Encoder
	body.String(text)
	return &dbus.Message{Type: dbus.TypeSignal, Path: path, Interface: icIface, Member: "CommitString", Signature: "s", Body: body.Bytes()}
}

type preeditSegment struct {
	text   string
	format int32
}

func formattedPreeditSignal(path string, segments []preeditSegment, caret int32) *dbus.Message {
	var body dbus.Encoder
	array := body.BeginArray(8)
	for _, segment := range segments {
		body.Align(8)
		body.String(segment.text)
		body.Int32(segment.format)
	}
	body.EndArray(array)
	body.Int32(caret)
	return &dbus.Message{Type: dbus.TypeSignal, Path: path, Interface: icIface, Member: "UpdateFormattedPreedit", Signature: "a(si)i", Body: body.Bytes()}
}

func plainPreeditSignal(path, text string, caret int32) *dbus.Message {
	var body dbus.Encoder
	body.String(text)
	body.Int32(caret)
	return &dbus.Message{Type: dbus.TypeSignal, Path: path, Interface: icIface, Member: "UpdatePreedit", Signature: "si", Body: body.Bytes()}
}

func keyReply(serial uint32, handled bool) *dbus.Message {
	var body dbus.Encoder
	if handled {
		body.Int32(1)
	} else {
		body.Int32(0)
	}
	return &dbus.Message{Type: dbus.TypeReturn, ReplySerial: serial, Signature: "i", Body: body.Bytes()}
}

func newTestContext(t *testing.T, fake *fakeTransport) *Context {
	t.Helper()
	bus := newBus(fake)
	context, err := bus.NewContext("viva-test", Handler{})
	if err != nil {
		t.Fatal(err)
	}
	return context
}

func TestNewContextCreatesAnInputContextAndDeclaresCapacities(t *testing.T) {
	fake := &fakeTransport{id: 7}
	context := newTestContext(t, fake)
	if context.path != "/inputcontext_7" {
		t.Fatalf("context path = %q", context.path)
	}
	created := fake.calls("CreateICv3")
	if len(created) != 1 || created[0].path != imPath || created[0].iface != imIface || created[0].signature != "si" {
		t.Fatalf("CreateICv3 calls = %+v", created)
	}
	args := dbus.NewDecoder(created[0].body)
	if name := args.String(); name != "viva-test" {
		t.Fatalf("app name = %q", name)
	}
	if pid := args.Int32(); pid <= 0 {
		t.Fatalf("pid = %d", pid)
	}
	capacities := fake.calls("SetCapacity")
	if len(capacities) != 1 || capacities[0].signature != "u" {
		t.Fatalf("SetCapacity calls = %+v", capacities)
	}
	if got := dbus.NewDecoder(capacities[0].body).Uint32(); got != CapPreedit|CapFormattedPreedit|CapClientUnfocusCommit|CapSurroundingText {
		t.Fatalf("capacities = %#x; the client draws preedit and settles it on unfocus", got)
	}
}

func TestEngineTextArrivesBeforeTheKeysAnswer(t *testing.T) {
	fake := &fakeTransport{id: 3}
	path := "/inputcontext_3"
	var events []string
	var handled []bool
	bus := newBus(fake)
	context, err := bus.NewContext("viva-test", Handler{
		Commit: func(value string) { events = append(events, "commit:"+value) },
		Preedit: func(event PreeditEvent) {
			events = append(events, "preedit:"+event.Text)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := context.ProcessKey(Key{Keysym: 'n', Keycode: 57}, func(value bool) {
		handled = append(handled, value)
		events = append(events, "answer")
	}); err != nil {
		t.Fatal(err)
	}
	if len(handled) != 0 {
		t.Fatal("the engine answered before it was given the key")
	}
	serial := fake.calls("ProcessKeyEvent")[0].serial
	fake.incoming = []*dbus.Message{
		formattedPreeditSignal(path, []preeditSegment{{text: "n", format: 1 << 3}}, 1),
		commitSignal(path, "你"),
		keyReply(serial, true),
	}
	if err := bus.Dispatch(); err != nil {
		t.Fatal(err)
	}
	want := []string{"preedit:n", "commit:你", "answer"}
	for index, value := range want {
		if index >= len(events) || events[index] != value {
			t.Fatalf("events = %v; want the engine's text before its answer, in order %v", events, want)
		}
	}
	if !handled[0] {
		t.Fatal("the engine took the key")
	}
}

func TestReleaseKeyIsSentAsARelease(t *testing.T) {
	fake := &fakeTransport{id: 0}
	context := newTestContext(t, fake)
	if err := context.ProcessKey(Key{Keysym: 'n', Keycode: 57, Time: 99, Release: true}, nil); err != nil {
		t.Fatal(err)
	}
	calls := fake.calls("ProcessKeyEvent")
	if len(calls) != 1 || calls[0].signature != "uuuiu" {
		t.Fatalf("ProcessKeyEvent calls = %+v", calls)
	}
	args := dbus.NewDecoder(calls[0].body)
	if keysym, keycode, state := args.Uint32(), args.Uint32(), args.Uint32(); keysym != 'n' || keycode != 57 || state != 0 {
		t.Fatalf("key = %#x %d %#x", keysym, keycode, state)
	}
	if kind := args.Int32(); kind != 1 {
		t.Fatalf("key type = %d; want a release", kind)
	}
	if at := args.Uint32(); at != 99 {
		t.Fatalf("event time = %d; want the X event time", at)
	}
	if args.Err() != nil || !args.Done() {
		t.Fatalf("ProcessKeyEvent body = %x", calls[0].body)
	}
}

func TestFormattedPreeditJoinsSegmentsAndCountsTheCaretInCharacters(t *testing.T) {
	fake := &fakeTransport{id: 1}
	path := "/inputcontext_1"
	var got PreeditEvent
	bus := newBus(fake)
	if _, err := bus.NewContext("viva-test", Handler{Preedit: func(event PreeditEvent) { got = event }}); err != nil {
		t.Fatal(err)
	}
	// "你" is three bytes and one character; the caret sits after "h" in "hao".
	fake.incoming = []*dbus.Message{formattedPreeditSignal(path, []preeditSegment{
		{text: "你", format: 1 << 3},
		{text: "hao", format: 1<<3 | preeditHighlight},
	}, 1+3)}
	if err := bus.Dispatch(); err != nil {
		t.Fatal(err)
	}
	if got.Text != "你hao" || !got.Visible {
		t.Fatalf("preedit = %+v", got)
	}
	if got.Cursor != 2 {
		t.Fatalf("caret = %d; want 2 characters", got.Cursor)
	}
	if got.Selection != (Range{Start: 1, End: 4}) {
		t.Fatalf("selection = %+v; want the highlighted clause in characters", got.Selection)
	}
	fake.incoming = []*dbus.Message{formattedPreeditSignal(path, nil, 0)}
	if err := bus.Dispatch(); err != nil {
		t.Fatal(err)
	}
	if got.Text != "" || got.Visible {
		t.Fatalf("cleared preedit = %+v", got)
	}
}

func TestPlainPreeditIsUnderstood(t *testing.T) {
	fake := &fakeTransport{id: 2}
	var got PreeditEvent
	bus := newBus(fake)
	if _, err := bus.NewContext("viva-test", Handler{Preedit: func(event PreeditEvent) { got = event }}); err != nil {
		t.Fatal(err)
	}
	fake.incoming = []*dbus.Message{plainPreeditSignal("/inputcontext_2", "nǐhǎo", 3)}
	if err := bus.Dispatch(); err != nil {
		t.Fatal(err)
	}
	if got.Text != "nǐhǎo" || got.Cursor != 2 || got.Selection != (Range{Start: 2, End: 2}) {
		t.Fatalf("preedit = %+v", got)
	}
}

func TestDeleteSurroundingAndForwardAreDelivered(t *testing.T) {
	fake := &fakeTransport{id: 4}
	var deletes [][2]int
	var forwards [][3]uint32
	var releases []bool
	bus := newBus(fake)
	if _, err := bus.NewContext("viva-test", Handler{
		DeleteSurrounding: func(offset, count int) { deletes = append(deletes, [2]int{offset, count}) },
		Forward: func(keysym, state uint32, release bool) {
			forwards = append(forwards, [3]uint32{keysym, state, 0})
			releases = append(releases, release)
		},
	}); err != nil {
		t.Fatal(err)
	}
	var deleteBody dbus.Encoder
	deleteBody.Int32(-2)
	deleteBody.Uint32(3)
	var forwardBody dbus.Encoder
	forwardBody.Uint32(0xff0d)
	forwardBody.Uint32(0)
	forwardBody.Int32(1)
	fake.incoming = []*dbus.Message{
		{Type: dbus.TypeSignal, Path: "/inputcontext_4", Interface: icIface, Member: "DeleteSurroundingText", Signature: "iu", Body: deleteBody.Bytes()},
		{Type: dbus.TypeSignal, Path: "/inputcontext_4", Interface: icIface, Member: "ForwardKey", Signature: "uui", Body: forwardBody.Bytes()},
	}
	if err := bus.Dispatch(); err != nil {
		t.Fatal(err)
	}
	if len(deletes) != 1 || deletes[0] != [2]int{-2, 3} {
		t.Fatalf("deletes = %v", deletes)
	}
	if len(forwards) != 1 || forwards[0] != [3]uint32{0xff0d, 0, 0} || !releases[0] {
		t.Fatalf("forwards = %v releases = %v", forwards, releases)
	}
}

func TestSignalsForAnotherContextAreIgnored(t *testing.T) {
	fake := &fakeTransport{id: 5}
	commits := 0
	bus := newBus(fake)
	if _, err := bus.NewContext("viva-test", Handler{Commit: func(string) { commits++ }}); err != nil {
		t.Fatal(err)
	}
	fake.incoming = []*dbus.Message{
		commitSignal("/inputcontext_99", "stray"),
		{Type: dbus.TypeSignal, Path: "/inputcontext_5", Interface: "org.example.Other", Member: "CommitString", Signature: "s"},
	}
	if err := bus.Dispatch(); err != nil {
		t.Fatal(err)
	}
	if commits != 0 {
		t.Fatalf("a context got %d commits meant for another", commits)
	}
}

func TestExpiredKeyIsNotHandledAndClosesTheBus(t *testing.T) {
	fake := &fakeTransport{id: 6}
	bus := newBus(fake)
	bus.now = func() time.Time { return time.Unix(100, 0) }
	context, err := bus.NewContext("viva-test", Handler{})
	if err != nil {
		t.Fatal(err)
	}
	answered := -1
	if err := context.ProcessKey(Key{Keysym: 'n', Keycode: 57}, func(handled bool) {
		if handled {
			answered = 1
		} else {
			answered = 0
		}
	}); err != nil {
		t.Fatal(err)
	}
	if deadline, waiting := bus.Deadline(); !waiting || !deadline.Equal(time.Unix(100, 0).Add(keyTimeout)) {
		t.Fatalf("deadline = %v, waiting = %t", deadline, waiting)
	}
	bus.now = func() time.Time { return time.Unix(100, 0).Add(keyTimeout) }
	bus.Expire()
	if answered != 0 {
		t.Fatalf("expired key answered = %d; want a release as not handled", answered)
	}
	if bus.Err() == nil || !fake.closed {
		t.Fatalf("bus error = %v, closed = %t", bus.Err(), fake.closed)
	}
	if err := context.ProcessKey(Key{Keysym: 'n', Keycode: 57}, nil); err != ErrClosed {
		t.Fatalf("a key after the deadline returned %v", err)
	}
}

func TestCloseDestroysTheContext(t *testing.T) {
	fake := &fakeTransport{id: 8}
	context := newTestContext(t, fake)
	context.Close()
	fake.incoming = []*dbus.Message{commitSignal("/inputcontext_8", "late")}
	commits := 0
	context.handler.Commit = func(string) { commits++ }
	if err := context.bus.Dispatch(); err != nil {
		t.Fatal(err)
	}
	if commits != 0 {
		t.Fatal("a closed context still received its engine's text")
	}
	if calls := fake.calls("DestroyIC"); len(calls) != 1 || calls[0].path != "/inputcontext_8" {
		t.Fatalf("DestroyIC calls = %+v", calls)
	}
}

func BenchmarkProcessKey(b *testing.B) {
	fake := &fakeTransport{id: 9}
	bus := newBus(fake)
	bus.now = func() time.Time { return time.Unix(100, 0) }
	context, err := bus.NewContext("viva-bench", Handler{})
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for index := 0; index < b.N; index++ {
		fake.incoming = append(fake.incoming[:0], keyReply(fake.serial+1, false))
		if err := context.ProcessKey(Key{Keysym: 'n', Keycode: 57}, nil); err != nil {
			b.Fatal(err)
		}
		if err := bus.Dispatch(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFormattedPreedit(b *testing.B) {
	message := formattedPreeditSignal("/inputcontext_9", []preeditSegment{
		{text: "zhong", format: 1 << 3},
		{text: "guo", format: 1<<3 | preeditHighlight},
	}, 10)
	b.ReportAllocs()
	for index := 0; index < b.N; index++ {
		if _, ok := readFormattedPreedit(message.Args()); !ok {
			b.Fatal("preedit was not read")
		}
	}
}
