//go:build linux

package linux

import (
	"encoding/binary"
	"os"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

func TestURIListsBecomeLocalPaths(t *testing.T) {
	got := parseURIList("# a comment\r\nfile:///tmp/a%20b.txt\r\nfile://localhost/tmp/dir\nhttp://example.com/x\nfile://other-host/tmp/y\nfile:relative\n\nfile:///tmp/caf%C3%A9\n")
	want := []string{"/tmp/a b.txt", "/tmp/dir", "/tmp/café"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("paths = %q, want %q", got, want)
	}
	if parseURIList("") != nil {
		t.Fatal("an empty list has paths")
	}
}

// source is another X client playing the program a drag starts from.
type source struct {
	t          *testing.T
	api        *xcbAPI
	connection uintptr
	screen     *xcbScreen
	id         uint32
}

func (s *source) message(target, kind uint32, longs ...uint32) {
	event := xcbClientEvent{ResponseType: xcbClientMessage, Format: 32, Window: target, Type: kind}
	for index, value := range longs {
		binary.LittleEndian.PutUint32(event.Data[index*4:], value)
	}
	s.api.sendEvent(s.connection, 0, target, 0, (*byte)(unsafe.Pointer(&event)))
	s.api.flush(s.connection)
}

// next waits for an event, answering the target's request for the dragged data.
func (s *source) next(selectionAtom, listAtom uint32, list string) (unsafe.Pointer, bool) {
	deadline := time.Now().Add(5 * time.Second)
	file := pollFD{FD: s.api.fileDescriptor(s.connection), Events: xcbPollInput}
	for time.Now().Before(deadline) {
		if event := s.api.pollForEvent(s.connection); event != nil {
			if (*xcbGenericEvent)(event).ResponseType&0x7f == xcbSelectionRequest {
				request := (*xcbSelectionRequestEvent)(event)
				data := []byte(list)
				s.api.changeProperty(s.connection, xcbPropertyReplace, request.Requestor, request.Property, listAtom, 8, uint32(len(data)), unsafe.Pointer(&data[0]))
				notify := xcbSelectionNotifyWire{xcbSelectionNotifyEvent: xcbSelectionNotifyEvent{
					ResponseType: xcbSelectionNotify, Requestor: request.Requestor, Selection: request.Selection,
					Target: request.Target, Property: request.Property,
				}}
				s.api.sendEvent(s.connection, 0, request.Requestor, 0, (*byte)(unsafe.Pointer(&notify)))
				s.api.flush(s.connection)
				s.api.free(event)
				continue
			}
			return event, true
		}
		s.api.poll(&file, 1, 20)
	}
	return nil, false
}

// reply waits for the target's XdndStatus or XdndFinished and returns its flags.
func (s *source) reply(kind, selectionAtom, listAtom uint32, list string) (flags uint32, found bool) {
	for {
		event, ok := s.next(selectionAtom, listAtom, list)
		if !ok {
			return 0, false
		}
		message := (*xcbClientEvent)(event)
		isReply := (*xcbGenericEvent)(event).ResponseType&0x7f == xcbClientMessage && message.Type == kind
		flags = binary.LittleEndian.Uint32(message.Data[4:])
		s.api.free(event)
		if isReply {
			return flags, true
		}
	}
}

// nextDropEvent takes the next hover or drop report, failing the test instead
// of blocking the whole package when the protocol goes quiet.
func nextDropEvent(t *testing.T, events <-chan platform.FileDropEvent) platform.FileDropEvent {
	t.Helper()
	select {
	case event := <-events:
		return event
	case <-time.After(10 * time.Second):
		t.Fatal("the application never reported the drag")
		return platform.FileDropEvent{}
	}
}

// TestAFileDragFromAnotherClientIsAcceptedAndDropped runs the whole protocol
// against a real window on a real X server: the source enters, moves, and drops
// a list of files, and the application hears the hover and the paths.
func TestAFileDragFromAnotherClientIsAcceptedAndDropped(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no X server")
	}
	t.Setenv("VIVA_LINUX_BACKEND", "x11")
	activeWindow.Store(nil)
	t.Cleanup(func() { activeWindow.Store(nil) })
	events := make(chan platform.FileDropEvent, 8)
	done := make(chan error, 1)
	go func() {
		done <- Run(platform.Config{
			Title: "drop", Width: 300, Height: 200,
			OnFileDrop: func(event platform.FileDropEvent) (platform.Frame, bool, error) {
				events <- event
				return platform.Frame{}, event.Phase != platform.FileDropLeave, nil
			},
		})
	}()
	// Always release the real application, including after a failed protocol
	// assertion, so a failed run cannot strand its event loop in this process.
	var target *window
	t.Cleanup(func() {
		if target == nil {
			return
		}
		for i := 0; i < 10; i++ {
			select {
			case <-done:
				return
			default:
			}
			target.postClientMessage(target.closeAtom, 0)
			select {
			case <-done:
				return
			case <-time.After(100 * time.Millisecond):
			}
		}
	})
	for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		select {
		case err := <-done:
			t.Skipf("no window could be opened here: %v", err)
		default:
		}
		if target = activeWindow.Load(); target != nil {
			break
		}
	}
	if target == nil {
		t.Fatal("the window never showed")
	}
	api, connection, screen, err := openSystemConnection()
	if err != nil {
		t.Fatal(err)
	}
	defer api.disconnect(connection)
	src := &source{t: t, api: api, connection: connection, screen: screen, id: api.generateID(connection)}
	values := [1]uint32{0}
	api.createWindow(connection, xcbCopyFromParent, src.id, screen.Root, 0, 0, 10, 10, 0, xcbWindowClassInput, screen.RootVisual, xcbCWEventMask, &values[0])
	atom := func(name string) uint32 {
		value, err := api.atom(connection, name)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	enter, position, status, drop, finished := atom("XdndEnter"), atom("XdndPosition"), atom("XdndStatus"), atom("XdndDrop"), atom("XdndFinished")
	selection, uriList, copyAction, aware := atom("XdndSelection"), atom("text/uri-list"), atom("XdndActionCopy"), atom("XdndAware")
	api.setSelectionOwner(connection, src.id, selection, 0)
	api.flush(connection)

	// The window said it takes drags, and at which version.
	cookie := api.getProperty(connection, 0, target.id, aware, xcbAtomAtom, 0, 1)
	var nativeError unsafe.Pointer
	reply := api.getPropertyReply(connection, cookie, &nativeError)
	if reply == nil || reply.Format != 32 || *(*uint32)(api.getPropertyValue(reply)) != xdndVersion {
		t.Fatalf("XdndAware not set: %+v", reply)
	}
	api.free(unsafe.Pointer(reply))

	list := "file:///tmp/a%20b.txt\r\nfile:///tmp/dir\r\n"
	src.message(target.id, enter, src.id, xdndVersion<<24, uriList)
	rootCookie := api.translateCoordinates(connection, target.id, screen.Root, 60, 30)
	rootReply := api.translateCoordinatesReply(connection, rootCookie, &nativeError)
	if rootReply == nil {
		t.Fatal("cannot translate coordinates")
	}
	rootX, rootY := uint32(uint16(rootReply.DstX)), uint32(uint16(rootReply.DstY))
	api.free(unsafe.Pointer(rootReply))
	src.message(target.id, position, src.id, 0, rootX<<16|rootY, 0, copyAction)
	flags, ok := src.reply(status, selection, uriList, list)
	if !ok || flags&1 == 0 {
		t.Fatalf("XdndStatus: found %v flags %d: the point should take files", ok, flags)
	}
	hover := nextDropEvent(t, events)
	scale := target.displayScale()
	expectedX := float32(60) / scale
	expectedY := float32(30) / scale
	if hover.Phase != platform.FileDropOver || hover.X < expectedX-1 || hover.X > expectedX+1 || hover.Y < expectedY-1 || hover.Y > expectedY+1 || len(hover.Paths) != 0 {
		t.Fatalf("hover = %+v, expected around (%f, %f)", hover, expectedX, expectedY)
	}

	src.message(target.id, drop, src.id, 0, 0)
	flags, ok = src.reply(finished, selection, uriList, list)
	if !ok || flags&1 == 0 {
		t.Fatalf("XdndFinished: found %v flags %d", ok, flags)
	}
	dropped := nextDropEvent(t, events)
	if dropped.Phase != platform.FileDropDrop || !reflect.DeepEqual(dropped.Paths, []string{"/tmp/a b.txt", "/tmp/dir"}) {
		t.Fatalf("drop = %+v", dropped)
	}

	// A drag that leaves ends the hover.
	src.message(target.id, enter, src.id, xdndVersion<<24, uriList)
	src.message(target.id, atom("XdndLeave"), src.id)
	if leave := nextDropEvent(t, events); leave.Phase != platform.FileDropLeave {
		t.Fatalf("leave = %+v", leave)
	}

	for i := 0; i < 10; i++ {
		target.postClientMessage(target.closeAtom, 0)
		select {
		case <-done:
			return
		case <-time.After(500 * time.Millisecond):
		}
	}
	t.Fatal("the window did not close")
}
