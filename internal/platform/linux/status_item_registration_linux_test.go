package linux

import (
	"errors"
	"strings"
	"testing"

	"github.com/vivaui/viva/internal/dbus"
	"github.com/vivaui/viva/internal/dbusmenu"
	"github.com/vivaui/viva/internal/platform"
)

// fakeStatusConnection is a session bus and a StatusNotifier host that answer
// on demand, so a test can leave a call unanswered for as long as it likes.
type fakeStatusConnection struct {
	owners   map[string]bool
	serial   uint32
	incoming []*dbus.Message
	sent     []sentCall
	replied  int
	last     *dbus.Message
	closes   int
}

type sentCall struct {
	destination, member string
	serial              uint32
}

func (conn *fakeStatusConnection) Call(_, _, _, member, _ string, body []byte) (*dbus.Message, error) {
	switch member {
	case "NameHasOwner":
		var e dbus.Encoder
		e.Bool(conn.owners[dbus.NewDecoder(body).String()])
		return &dbus.Message{Type: dbus.TypeReturn, Signature: "b", Body: e.Bytes()}, nil
	case "RequestName":
		var e dbus.Encoder
		e.Uint32(1) // the name was acquired
		return &dbus.Message{Type: dbus.TypeReturn, Signature: "u", Body: e.Bytes()}, nil
	default: // AddMatch, ReleaseName
		return &dbus.Message{Type: dbus.TypeReturn}, nil
	}
}

func (conn *fakeStatusConnection) Send(destination, _, _, member, _ string, _ []byte) (uint32, error) {
	conn.serial++
	conn.sent = append(conn.sent, sentCall{destination: destination, member: member, serial: conn.serial})
	return conn.serial, nil
}

func (conn *fakeStatusConnection) Receive() ([]*dbus.Message, error) {
	messages := conn.incoming
	conn.incoming = nil
	return messages, nil
}

func (conn *fakeStatusConnection) Reply(call *dbus.Message, signature string, body []byte) error {
	conn.replied++
	conn.last = &dbus.Message{Type: dbus.TypeReturn, ReplySerial: call.Serial, Signature: signature, Body: body}
	return nil
}

func (conn *fakeStatusConnection) ReplyError(*dbus.Message, string, string) error { return nil }
func (conn *fakeStatusConnection) Emit(string, string, string, string, []byte) error {
	return nil
}
func (conn *fakeStatusConnection) FD() int { return -1 }
func (conn *fakeStatusConnection) Close()  { conn.closes++ }

func statusTestIcon() platform.StatusIcon {
	pixels := make([]byte, 4*4*4)
	for index := 0; index < len(pixels); index += 4 {
		copy(pixels[index:], []byte{10, 20, 30, 255})
	}
	return platform.StatusIcon{Size: 4, RGBA: pixels}
}

// statusTestServices prepares desktop services whose session bus is owned by
// the test, with the given watcher names answering as owned.
func statusTestServices(t *testing.T, owned ...string) (*desktopServices, *fakeStatusConnection) {
	t.Helper()
	conn := &fakeStatusConnection{owners: make(map[string]bool)}
	for _, name := range owned {
		conn.owners[name] = true
	}
	previous := dialStatus
	dialStatus = func() (statusConnection, error) { return conn, nil }
	t.Cleanup(func() { dialStatus = previous })
	return &desktopServices{host: &fakeBusHost{posts: make(chan uint32, 4)}}, conn
}

// A host that has not acknowledged the registration yet still asks the icon
// for its properties, and the icon answers. Waiting for the acknowledgement
// before serving them would deadlock a host that defers its answer until its
// desktop has finished starting up, which is what GNOME Shell's extension does.
func TestStatusItemServesPropertiesBeforeTheHostAcknowledges(t *testing.T) {
	services, conn := statusTestServices(t, watcherNames[0])
	item, err := services.AddStatusItem(platform.StatusItemConfig{Icon: statusTestIcon()})
	if err != nil {
		t.Fatalf("AddStatusItem: %v", err)
	}
	if len(conn.sent) != 1 || conn.sent[0].member != "RegisterStatusNotifierItem" {
		t.Fatalf("sent %+v; want one registration", conn.sent)
	}

	var body dbus.Encoder
	body.String(itemInterface)
	body.String("IconPixmap")
	conn.incoming = []*dbus.Message{{
		Type: dbus.TypeCall, Serial: 9, Destination: item.(*linuxStatusItem).name,
		Path: itemPath, Interface: propsInterface, Member: "Get", Signature: "ss", Body: body.Bytes(),
	}}
	services.serveStatus()

	if conn.replied != 1 || conn.last == nil || conn.last.Signature != "v" {
		t.Fatalf("replies = %d, last = %+v; want the icon's pixmap answered", conn.replied, conn.last)
	}
	d := dbus.NewDecoder(conn.last.Body)
	if signature := d.Signature(); signature != "a(iiay)" {
		t.Fatalf("IconPixmap answered as %q", signature)
	}
}

func TestStatusItemServesMenuBeforeTheHostAcknowledges(t *testing.T) {
	services, conn := statusTestServices(t, watcherNames[0])
	label := "显示窗口"
	item, err := services.AddStatusItem(platform.StatusItemConfig{
		Icon: statusTestIcon(),
		Menu: func() []platform.MenuItem { return []platform.MenuItem{{Title: label}} },
	})
	if err != nil {
		t.Fatalf("AddStatusItem: %v", err)
	}
	name := item.(*linuxStatusItem).name
	var layout dbus.Encoder
	layout.Int32(0)
	layout.Int32(-1)
	names := layout.BeginArray(4)
	layout.String("type")
	layout.String("children-display")
	layout.EndArray(names)
	conn.incoming = []*dbus.Message{{
		Type: dbus.TypeCall, Serial: 10, Destination: name,
		Path: dbusmenu.Path, Interface: dbusmenu.Interface,
		Member: "GetLayout", Signature: "iias", Body: layout.Bytes(),
	}}
	services.serveStatus()
	if conn.last == nil || conn.last.ReplySerial != 10 || conn.last.Signature != "u(ia{sv}av)" {
		t.Fatalf("layout reply = %+v", conn.last)
	}
	var group dbus.Encoder
	group.EndArray(group.BeginArray(4))
	group.EndArray(group.BeginArray(4))
	conn.incoming = []*dbus.Message{
		{
			Type: dbus.TypeCall, Serial: 11, Destination: name,
			Path: dbusmenu.Path, Interface: dbusmenu.Interface,
			Member: "GetGroupProperties", Signature: "aias", Body: group.Bytes(),
		},
		{Type: dbus.TypeReturn, ReplySerial: conn.sent[0].serial},
	}
	services.serveStatus()
	if conn.replied != 2 || conn.last == nil || conn.last.ReplySerial != 11 || conn.last.Signature != "a(ia{sv})" {
		t.Fatalf("menu replies = %d, last = %+v", conn.replied, conn.last)
	}
	decoder := dbus.NewDecoder(conn.last.Body)
	decoder.Skip(conn.last.Signature)
	if decoder.Err() != nil || !decoder.Done() || !strings.Contains(string(conn.last.Body), label) {
		t.Fatalf("menu labels were not served alongside the registration reply: %v", decoder.Err())
	}
	if len(services.status.pending) != 0 {
		t.Fatal("the registration acknowledgement was not dispatched")
	}
}

func BenchmarkStatusRegistration(b *testing.B) {
	conn := &fakeStatusConnection{}
	notifier := &statusNotifier{conn: conn, pending: make(map[uint32]pendingRegistration)}
	item := &linuxStatusItem{name: "org.kde.StatusNotifierItem-1-1"}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := notifier.register(item); err != nil {
			b.Fatal(err)
		}
		notifier.settleRegistration(&dbus.Message{Type: dbus.TypeReturn, ReplySerial: conn.serial})
		conn.sent = conn.sent[:0]
	}
}

// A registration the first watcher name refuses is tried on the other one, and
// the refusal does not take the icon down.
func TestStatusRegistrationFallsBackToTheOtherWatcherName(t *testing.T) {
	services, conn := statusTestServices(t, watcherNames[1])
	if _, err := services.AddStatusItem(platform.StatusItemConfig{Icon: statusTestIcon()}); err != nil {
		t.Fatalf("AddStatusItem: %v", err)
	}
	if len(conn.sent) != 1 || conn.sent[0].destination != watcherNames[0] {
		t.Fatalf("sent %+v; want the first watcher name asked first", conn.sent)
	}
	conn.incoming = []*dbus.Message{{
		Type: dbus.TypeError, ReplySerial: conn.sent[0].serial,
		ErrorName: "org.freedesktop.DBus.Error.UnknownObject",
	}}
	services.serveStatus()

	if len(conn.sent) != 2 || conn.sent[1].destination != watcherNames[1] {
		t.Fatalf("sent %+v; want the second watcher name asked after the refusal", conn.sent)
	}
	if services.status == nil || len(services.status.items) != 1 {
		t.Fatal("a refused registration took the icon down")
	}
}

// A watcher that comes back is told about the icon again, even while an answer
// to the previous registration is still outstanding.
func TestStatusRegistrationFollowsTheWatcherAppearing(t *testing.T) {
	services, conn := statusTestServices(t, watcherNames[0])
	if _, err := services.AddStatusItem(platform.StatusItemConfig{Icon: statusTestIcon()}); err != nil {
		t.Fatalf("AddStatusItem: %v", err)
	}
	var body dbus.Encoder
	body.String(watcherNames[0])
	body.String(":1.9")
	body.String(":1.10")
	conn.incoming = []*dbus.Message{{
		Type: dbus.TypeSignal, Member: "NameOwnerChanged", Signature: "sss", Body: body.Bytes(),
	}}
	services.serveStatus()

	if len(conn.sent) != 2 || conn.sent[1].destination != watcherNames[0] {
		t.Fatalf("sent %+v; want the icon registered with the watcher that appeared", conn.sent)
	}
}

// Without a watcher name owned by anything there is no status area, and the
// connection made to ask is given back.
func TestStatusItemNeedsAStatusArea(t *testing.T) {
	services, conn := statusTestServices(t)
	_, err := services.AddStatusItem(platform.StatusItemConfig{Icon: statusTestIcon()})
	if !errors.Is(err, platform.ErrStatusAreaUnavailable) {
		t.Fatalf("AddStatusItem = %v, want the status area unavailable", err)
	}
	if conn.closes == 0 {
		t.Fatal("the bus connection stayed open without a host")
	}
}
