//go:build linux && !android

package linux

import (
	"encoding/binary"
	"io"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/vivaui/viva/internal/platform"
)

// dropTargetClient is another X client playing the file manager a drag from the
// application lands on: it says it takes XDND drags, answers the source's
// positions, and reads the file list from XdndSelection.
type dropTargetClient struct {
	t          *testing.T
	api        *xcbAPI
	connection uintptr
	screen     *xcbScreen
	id         uint32
	property   uint32
}

func (target *dropTargetClient) message(window, kind uint32, longs ...uint32) {
	event := xcbClientEvent{ResponseType: xcbClientMessage, Format: 32, Window: window, Type: kind}
	for index, value := range longs {
		binary.LittleEndian.PutUint32(event.Data[index*4:], value)
	}
	target.api.sendEvent(target.connection, 0, window, 0, (*byte)(unsafe.Pointer(&event)))
	target.api.flush(target.connection)
}

// wait returns the next client message of kind, skipping everything else the
// test client receives.
func (target *dropTargetClient) wait(kind uint32, timeout time.Duration) *xcbClientEvent {
	deadline := time.Now().Add(timeout)
	poll := pollFD{FD: target.api.fileDescriptor(target.connection), Events: xcbPollInput}
	for time.Now().Before(deadline) {
		if event := target.api.pollForEvent(target.connection); event != nil {
			if (*xcbGenericEvent)(event).ResponseType&0x7f == xcbClientMessage {
				message := (*xcbClientEvent)(event)
				kindSeen := message.Type
				copy := *message
				target.api.free(event)
				if kindSeen == kind {
					return &copy
				}
				continue
			}
			target.api.free(event)
			continue
		}
		target.api.poll(&poll, 1, 20)
	}
	return nil
}

// readSelection asks owner for text/uri-list and reads the property the owner
// writes back.
func (target *dropTargetClient) readSelection(owner, selection, uriList uint32) (string, bool) {
	request := xcbSelectionRequestEvent{
		ResponseType: xcbSelectionRequest,
		Time:         0,
		Owner:        owner,
		Requestor:    target.id,
		Selection:    selection,
		Target:       uriList,
		Property:     target.property,
	}
	target.api.sendEvent(target.connection, 0, owner, 0, (*byte)(unsafe.Pointer(&request)))
	target.api.flush(target.connection)
	deadline := time.Now().Add(5 * time.Second)
	poll := pollFD{FD: target.api.fileDescriptor(target.connection), Events: xcbPollInput}
	for time.Now().Before(deadline) {
		if event := target.api.pollForEvent(target.connection); event != nil {
			if (*xcbGenericEvent)(event).ResponseType&0x7f == xcbSelectionNotify {
				target.api.free(event)
				break
			}
			target.api.free(event)
			continue
		}
		target.api.poll(&poll, 1, 20)
	}
	var nativeError unsafe.Pointer
	reply := target.api.getPropertyReply(target.connection, target.api.getProperty(target.connection, 0, target.id, target.property, uriList, 0, 1<<20), &nativeError)
	if nativeError != nil {
		target.api.free(nativeError)
	}
	if reply == nil || reply.ValueLength == 0 {
		if reply != nil {
			target.api.free(unsafe.Pointer(reply))
		}
		return "", false
	}
	data := unsafe.Slice((*byte)(target.api.getPropertyValue(reply)), reply.ValueLength)
	list := string(data)
	target.api.free(unsafe.Pointer(reply))
	return list, true
}

// TestAFileDragOutReachesAnotherClient runs the source side against a real X
// server: the application starts a drag of paths, the pointer moves over a
// client that takes XDND drops, and that client reads the file list from the
// XdndSelection when the drop happens.
func TestAFileDragOutReachesAnotherClient(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no X server")
	}
	done := make(chan error, 1)
	go func() {
		done <- Run(platform.Config{Title: "drag out", Width: 300, Height: 200})
	}()
	var win *window
	for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		select {
		case err := <-done:
			t.Skipf("no window could be opened here: %v", err)
		default:
		}
		if win = activeWindow.Load(); win != nil {
			break
		}
	}
	if win == nil {
		t.Fatal("the window never showed")
	}

	api, connection, screen, err := openSystemConnection()
	if err != nil {
		t.Fatal(err)
	}
	defer api.disconnect(connection)
	atom := func(name string) uint32 {
		value, err := api.atom(connection, name)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	target := &dropTargetClient{t: t, api: api, connection: connection, screen: screen, id: api.generateID(connection)}
	// An override-redirect window is not moved or stacked by the window
	// manager, so it really is at this position when the pointer is warped to
	// it and above the application's window.
	const cwOverrideRedirect = 1 << 9
	values := [2]uint32{1, 0}
	api.createWindow(connection, xcbCopyFromParent, target.id, screen.Root, 10, 10, 200, 200, 0, xcbWindowClassInput, screen.RootVisual, cwOverrideRedirect|xcbCWEventMask, &values[0])
	aware, selection, uriList := atom("XdndAware"), atom("XdndSelection"), atom("text/uri-list")
	version := uint32(xdndVersion)
	api.changeProperty(connection, xcbPropertyReplace, target.id, aware, xcbAtomAtom, 32, 1, unsafe.Pointer(&version))
	target.property = atom("VIVA_TEST_DRAG_LIST")
	api.mapWindow(connection, target.id)
	api.flush(connection)
	time.Sleep(100 * time.Millisecond)

	// Put the pointer over the client that takes the drop: the source finds its
	// target the way it does for a real file manager.
	var warp func(uintptr, uint32, uint32, int16, int16, uint16, uint16, int16, int16) xcbVoidCookie
	purego.RegisterLibFunc(&warp, api.library, "xcb_warp_pointer")
	warp(connection, xcbAtomNone, target.id, 0, 0, 0, 0, 60, 60)
	api.flush(connection)
	time.Sleep(50 * time.Millisecond)

	promises := []platform.FilePromise{{Name: "报告.txt", Size: 5, Open: func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("hello")), nil
	}}}
	started := make(chan error, 1)
	win.application.Post(func() {
		win.lastRootX, win.lastRootY, win.lastEventTime = 60, 60, 0
		started <- win.startDragOut(nil, promises)
	})
	select {
	case err := <-started:
		if err != nil {
			t.Fatalf("start the drag: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the event loop never ran the drag start")
	}
	enter := target.wait(atom("XdndEnter"), 5*time.Second)
	if enter == nil {
		t.Fatal("the drag never entered the target")
	}
	position := target.wait(atom("XdndPosition"), 5*time.Second)
	if position == nil {
		t.Fatal("the drag never reported a position")
	}
	source := binary.LittleEndian.Uint32(enter.Data[:4])
	// The target takes the drop, as a file manager does over its window.
	var status [5]uint32
	status[0] = target.id
	status[1] = 1 // accept
	status[4] = atom("XdndActionCopy")
	target.message(source, atom("XdndStatus"), status[:]...)

	// Let the source see the status, then let go of the button. The status is a
	// message from another connection, so give the loop a moment to read it.
	time.Sleep(300 * time.Millisecond)
	win.application.Post(func() {
		win.sourceDragRelease(&xcbInputEvent{Time: 0})
	})
	if target.wait(atom("XdndDrop"), 5*time.Second) == nil {
		t.Fatal("the release did not drop")
	}
	list, ok := target.readSelection(source, selection, uriList)
	if !ok || !strings.Contains(list, "%E6%8A%A5%E5%91%8A.txt") {
		t.Fatalf("the drop carried %q (ok %v), want the staged file's URI", list, ok)
	}
	// The promised content was staged before the drop and is a real file the
	// file manager can copy.
	line, _, _ := strings.Cut(list, "\r\n")
	parsed, err := url.Parse(line)
	if err != nil || parsed.Scheme != "file" {
		t.Fatalf("the drop's URI %q is not a file URL: %v", line, err)
	}
	body, err := os.ReadFile(parsed.Path)
	if err != nil || string(body) != "hello" {
		t.Fatalf("the staged file read %q, %v", body, err)
	}
	var finished [5]uint32
	finished[0] = target.id
	finished[1] = 1
	finished[2] = atom("XdndActionCopy")
	target.message(source, atom("XdndFinished"), finished[:]...)

	win.postClientMessage(win.closeAtom, 0)
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the window did not close")
	}
}
