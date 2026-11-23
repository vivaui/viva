//go:build !android

package linux

import (
	"encoding/binary"
	"io"
	"net"
	"strconv"
	"testing"
	"time"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

// A server that speaks just enough of the X protocol, over the abstract socket libxcb
// tries first, for a client to look the X Input extension up, negotiate version 2.2 and
// select touch events, and that then sends a touch event as a real server does: a
// generic event of 32 bytes and the words its length says follow. What the test reads
// back is what libxcb makes of that, which is what the window reads, so the offsets the
// backend uses are tested against the library and not against this file's idea of them.
const fakeXInputOpcode = 131

type fakeXServer struct {
	selected chan []byte // the body of the client's XISelectEvents
	event    []byte      // the wire event to send once the client has selected
}

func startFakeXServer(t *testing.T, display int, event []byte) *fakeXServer {
	t.Helper()
	listener, err := net.Listen("unix", "@/tmp/.X11-unix/X"+strconv.Itoa(display))
	if err != nil {
		t.Skipf("no abstract socket for a fake X server: %v", err)
	}
	server := &fakeXServer{selected: make(chan []byte, 1), event: event}
	t.Cleanup(func() { listener.Close() })
	go func() {
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		defer connection.Close()
		server.serve(connection)
	}()
	return server
}

func (server *fakeXServer) serve(connection net.Conn) {
	if _, err := io.ReadFull(connection, make([]byte, 12)); err != nil {
		return
	}
	// Setup success: one bare screen, no pixmap formats and a four-byte vendor.
	body := make([]byte, 32+4+40)
	binary.LittleEndian.PutUint32(body[0:], 1)          // release
	binary.LittleEndian.PutUint32(body[4:], 0x00200000) // resource id base
	binary.LittleEndian.PutUint32(body[8:], 0x001fffff) // resource id mask
	binary.LittleEndian.PutUint32(body[12:], 256)       // motion buffer
	binary.LittleEndian.PutUint16(body[16:], 4)         // vendor length
	binary.LittleEndian.PutUint16(body[18:], 65535)     // maximum request length
	body[20] = 1                                        // one screen
	copy(body[32:], "fake")
	binary.LittleEndian.PutUint32(body[36:], 0x100) // its root window
	reply := make([]byte, 8)
	reply[0] = 1
	binary.LittleEndian.PutUint16(reply[2:], 11)
	binary.LittleEndian.PutUint16(reply[6:], uint16(len(body)/4))
	connection.Write(append(reply, body...))
	var sequence uint16
	for {
		header := make([]byte, 4)
		if _, err := io.ReadFull(connection, header); err != nil {
			return
		}
		rest := make([]byte, int(binary.LittleEndian.Uint16(header[2:]))*4-4)
		if _, err := io.ReadFull(connection, rest); err != nil {
			return
		}
		sequence++
		answer := func(fill func(r []byte)) {
			r := make([]byte, 32)
			r[0] = 1
			binary.LittleEndian.PutUint16(r[2:], sequence)
			fill(r)
			connection.Write(r)
		}
		switch {
		case header[0] == 98: // QueryExtension
			answer(func(r []byte) { r[8], r[9], r[10] = 1, fakeXInputOpcode, 64 })
		case header[0] == fakeXInputOpcode && header[1] == 47: // XIQueryVersion
			answer(func(r []byte) {
				binary.LittleEndian.PutUint16(r[8:], 2)
				binary.LittleEndian.PutUint16(r[10:], 4)
			})
		case header[0] == fakeXInputOpcode && header[1] == 46: // XISelectEvents
			server.selected <- rest
			binary.LittleEndian.PutUint16(server.event[2:], sequence)
			connection.Write(server.event)
		}
	}
}

// wireTouchEvent is an XI_TouchBegin as the X Input protocol puts it on the wire.
func wireTouchEvent(eventType uint16, id uint32, x, y float32, effectiveMods uint32) []byte {
	event := make([]byte, 80)
	event[0], event[1] = xcbGenericEventType, fakeXInputOpcode
	binary.LittleEndian.PutUint32(event[4:], 12) // the words after the first 32 bytes
	binary.LittleEndian.PutUint16(event[8:], eventType)
	binary.LittleEndian.PutUint16(event[10:], 2)
	binary.LittleEndian.PutUint32(event[12:], 4242)
	binary.LittleEndian.PutUint32(event[16:], id)
	fixed := func(offset int, value float32) {
		binary.LittleEndian.PutUint32(event[offset:], uint32(int32(value*65536)))
	}
	fixed(32, x+100) // root_x
	fixed(36, y+200)
	fixed(40, x) // event_x
	fixed(44, y)
	binary.LittleEndian.PutUint32(event[72:], effectiveMods)
	return event
}

func TestATouchEventFromTheWireIsReadAsLibxcbDeliversIt(t *testing.T) {
	const display = 89
	server := startFakeXServer(t, display, wireTouchEvent(xiTouchBegin, 7, 12.5, -3.25, 0x5))
	t.Setenv("DISPLAY", ":"+strconv.Itoa(display))
	api, err := loadXCB()
	if err != nil {
		t.Skip(err)
	}
	var screen int32
	connection := api.connect(nil, &screen)
	if connection == 0 || api.connectionHasError(connection) != 0 {
		t.Skip("libxcb could not talk to the fake server")
	}
	defer api.disconnect(connection)
	touch := loadXInputTouch(api, connection, 0x200001)
	if touch == nil {
		t.Skip("libxcb-xinput is not installed")
	}
	if touch.opcode != fakeXInputOpcode {
		t.Fatalf("opcode = %d, want the one the server gave", touch.opcode)
	}
	select {
	case mask := <-server.selected:
		// The window, the number of masks, then the mask: the device, its length, the bits.
		if binary.LittleEndian.Uint32(mask[0:]) != 0x200001 || binary.LittleEndian.Uint16(mask[4:]) != 1 ||
			binary.LittleEndian.Uint16(mask[8:]) != xiAllMasterDevices || binary.LittleEndian.Uint16(mask[10:]) != 1 ||
			binary.LittleEndian.Uint32(mask[12:]) != xiTouchMask {
			t.Fatalf("the selection the server got = % x, want the touch events of all master devices", mask)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the server never got the selection")
	}
	var event unsafe.Pointer
	for deadline := time.Now().Add(2 * time.Second); event == nil && time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		event = api.pollForEvent(connection)
	}
	if event == nil {
		t.Fatal("libxcb delivered no event")
	}
	defer api.free(event)
	got, ok := touch.read(event)
	if !ok || got.kind != platform.PointerDown || got.id != 7 || got.x != 12.5 || got.y != -3.25 ||
		got.rootX != 112.5 || got.rootY != 196.75 || got.modifiers != 0x5 || got.timestamp != 4242 {
		t.Fatalf("read %+v, %v; want a touch begin of touch 7 at 12.5, -3.25 (112.5, 196.75 on the screen) with modifiers 0x5 at time 4242", got, ok)
	}
}
