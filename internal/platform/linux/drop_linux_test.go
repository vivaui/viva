//go:build linux && !android

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
	t               *testing.T
	api             *xcbAPI
	connection      uintptr
	screen          *xcbScreen
	id              uint32
	beforeReply     func()
	lastRequestTime uint32
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
				s.lastRequestTime = request.Time
				if s.beforeReply != nil {
					s.beforeReply()
				}
				data := []byte(list)
				s.api.changeProperty(s.connection, xcbPropertyReplace, request.Requestor, request.Property, listAtom, 8, uint32(len(data)), unsafe.Pointer(&data[0]))
				notify := xcbSelectionNotifyWire{xcbSelectionNotifyEvent: xcbSelectionNotifyEvent{
					ResponseType: xcbSelectionNotify, Time: request.Time, Requestor: request.Requestor, Selection: request.Selection,
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

// reply waits for the target's XdndStatus or XdndFinished and returns its
// payload words.
func (s *source) reply(kind, selectionAtom, listAtom uint32, list string) (data [5]uint32, found bool) {
	for {
		event, ok := s.next(selectionAtom, listAtom, list)
		if !ok {
			return data, false
		}
		message := (*xcbClientEvent)(event)
		isReply := (*xcbGenericEvent)(event).ResponseType&0x7f == xcbClientMessage && message.Type == kind
		for i := range data {
			data[i] = binary.LittleEndian.Uint32(message.Data[i*4:])
		}
		s.api.free(event)
		if isReply {
			return data, true
		}
	}
}

// TestAFileDragFromAnotherClientIsAcceptedAndDropped runs the whole protocol
// against a real window on a real X server: the source enters, moves, and drops
// a list of files, and the application hears the hover and the paths.
func TestAFileDragFromAnotherClientIsAcceptedAndDropped(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no X server")
	}
	events := make(chan platform.FileDropEvent, 8)
	redrawn := make(chan struct{}, 1)
	done := make(chan error, 1)
	go func() {
		dropped := false
		done <- Run(platform.Config{
			Title: "drop", Width: 300, Height: 200,
			Waker: &platform.Waker{},
			OnRedraw: func() (platform.Frame, error) {
				if dropped {
					select {
					case redrawn <- struct{}{}:
					default:
					}
				}
				return platform.Frame{}, nil
			},
			OnFileDrop: func(event platform.FileDropEvent) (platform.Frame, bool, error) {
				dropped = event.Phase == platform.FileDropDrop
				events <- event
				return platform.Frame{}, event.Phase != platform.FileDropLeave, nil
			},
		})
	}()
	var target *window
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
	statusData, ok := src.reply(status, selection, uriList, list)
	if !ok || statusData[1]&1 == 0 || statusData[4] != copyAction {
		t.Fatalf("XdndStatus: found %v data %v: the point should take files with copy action %d", ok, statusData, copyAction)
	}
	hover := <-events
	if hover.Phase != platform.FileDropOver || hover.X < 59 || hover.X > 61 || hover.Y < 29 || hover.Y > 31 || len(hover.Paths) != 0 {
		t.Fatalf("hover = %+v", hover)
	}

	const dropTime uint32 = 4242
	src.beforeReply = func() { src.message(target.id, target.redrawAtom) }
	src.message(target.id, drop, src.id, 0, dropTime)
	finishedData, ok := src.reply(finished, selection, uriList, list)
	if !ok || finishedData[1]&1 == 0 || finishedData[2] != copyAction {
		t.Fatalf("XdndFinished: found %v data %v, want accepted copy action %d in data[2]", ok, finishedData, copyAction)
	}
	if src.lastRequestTime != dropTime {
		t.Fatalf("SelectionRequest time = %d, want XdndDrop timestamp %d", src.lastRequestTime, dropTime)
	}
	dropped := <-events
	if dropped.Phase != platform.FileDropDrop || !reflect.DeepEqual(dropped.Paths, []string{"/tmp/a b.txt", "/tmp/dir"}) {
		t.Fatalf("drop = %+v", dropped)
	}
	select {
	case <-redrawn:
	case <-time.After(2 * time.Second):
		t.Error("redraw sent during the drop selection transfer was lost")
	}

	// A drag that leaves ends the hover.
	src.message(target.id, enter, src.id, xdndVersion<<24, uriList)
	src.message(target.id, atom("XdndLeave"), src.id)
	if leave := <-events; leave.Phase != platform.FileDropLeave {
		t.Fatalf("leave = %+v", leave)
	}

	target.postClientMessage(target.closeAtom, 0)
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the window did not close")
	}
}

func TestXdndDropSelectionHandlingWithoutXServer(t *testing.T) {
	const (
		sourceWin     uint32 = 77
		targetWin     uint32 = 88
		requestorWin  uint32 = 99
		dropAtom      uint32 = 201
		finishedAtom  uint32 = 202
		selectionAtom uint32 = 203
		actionCopy    uint32 = 204
		uriListAtom   uint32 = 205
		dropProperty  uint32 = 206
		dropTime      uint32 = 987654
	)

	makeDropMessage := func() *xcbClientEvent {
		msg := &xcbClientEvent{
			ResponseType: xcbClientMessage,
			Format:       32,
			Window:       targetWin,
			Type:         dropAtom,
		}
		binary.LittleEndian.PutUint32(msg.Data[0:], sourceWin)
		binary.LittleEndian.PutUint32(msg.Data[8:], dropTime)
		return msg
	}

	installSelectionAPI := func(t *testing.T, api *xcbAPI) {
		t.Helper()
		selections.mu.Lock()
		prevAPI, prevConn, prevReq := selections.api, selections.connection, selections.requestor
		selections.api, selections.connection, selections.requestor = api, 1, requestorWin
		selections.mu.Unlock()
		t.Cleanup(func() {
			selections.mu.Lock()
			selections.api, selections.connection, selections.requestor = prevAPI, prevConn, prevReq
			selections.mu.Unlock()
		})
	}

	t.Run("forwards drop timestamp and writes action to XdndFinished data[2]", func(t *testing.T) {
		payload := []byte("file:///tmp/demo%20file.bin\r\n")
		var gotTimestamp uint32
		notify := xcbSelectionNotifyEvent{
			ResponseType: xcbSelectionNotify,
			Time:         dropTime,
			Requestor:    requestorWin,
			Selection:    selectionAtom,
			Target:       uriListAtom,
			Property:     dropProperty,
		}
		converted := false
		delivered := false
		selAPI := &xcbAPI{
			atoms:              map[string]uint32{"XdndSelection": selectionAtom, "INCR": 999},
			connectionHasError: func(uintptr) int32 { return 0 },
			flush:              func(uintptr) int32 { return 1 },
			fileDescriptor:     func(uintptr) int32 { return 1 },
			free:               func(unsafe.Pointer) {},
			getSelectionOwner:  func(uintptr, uint32) xcbSelectionOwnerCookie { return xcbSelectionOwnerCookie{} },
			selectionOwnerReply: func(uintptr, xcbSelectionOwnerCookie, *unsafe.Pointer) *xcbSelectionOwnerReply {
				return &xcbSelectionOwnerReply{Owner: sourceWin}
			},
			convertSelection: func(_ uintptr, _, _, _, _, timestamp uint32) xcbVoidCookie {
				gotTimestamp = timestamp
				converted = true
				return xcbVoidCookie{}
			},
			pollForEvent: func(uintptr) unsafe.Pointer {
				if !converted || delivered {
					return nil
				}
				delivered = true
				return unsafe.Pointer(&notify)
			},
			getProperty: func(uintptr, uint8, uint32, uint32, uint32, uint32, uint32) xcbPropertyCookie {
				return xcbPropertyCookie{}
			},
			getPropertyReply: func(uintptr, xcbPropertyCookie, *unsafe.Pointer) *xcbPropertyReply {
				return &xcbPropertyReply{Format: 8, Type: uriListAtom, ValueLength: uint32(len(payload))}
			},
			getPropertyLength: func(*xcbPropertyReply) int32 { return int32(len(payload)) },
			getPropertyValue:  func(*xcbPropertyReply) unsafe.Pointer { return unsafe.Pointer(&payload[0]) },
		}
		installSelectionAPI(t, selAPI)

		var sentReply xcbClientEvent
		winAPI := &xcbAPI{
			sendEvent: func(_ uintptr, _ uint8, _ uint32, _ uint32, data *byte) xcbVoidCookie {
				copy(unsafe.Slice((*byte)(unsafe.Pointer(&sentReply)), unsafe.Sizeof(sentReply)), unsafe.Slice(data, unsafe.Sizeof(sentReply)))
				return xcbVoidCookie{}
			},
			flush: func(uintptr) int32 { return 1 },
		}
		var gotEvents []platform.FileDropEvent
		state := &linuxWindowState{
			win: &window{
				api:        winAPI,
				connection: 2,
				id:         targetWin,
				xdnd: xdndAtoms{
					drop:         dropAtom,
					finished:     finishedAtom,
					selection:    selectionAtom,
					actionCopy:   actionCopy,
					uriList:      uriListAtom,
					dropProperty: dropProperty,
				},
			},
			config: platform.Config{
				OnFileDrop: func(event platform.FileDropEvent) (platform.Frame, bool, error) {
					gotEvents = append(gotEvents, event)
					return platform.Frame{}, true, nil
				},
			},
			router: eventRouter{present: func(platform.Frame) error { return nil }},
			drag:   xdndDrag{source: sourceWin, wantsURIs: true, x: 12, y: 34},
		}

		if err := state.handleDragMessage(makeDropMessage()); err != nil {
			t.Fatalf("handleDragMessage returned error: %v", err)
		}
		if gotTimestamp != dropTime {
			t.Fatalf("convertSelection timestamp = %d, want %d", gotTimestamp, dropTime)
		}
		if len(gotEvents) != 1 || gotEvents[0].Phase != platform.FileDropDrop || !reflect.DeepEqual(gotEvents[0].Paths, []string{"/tmp/demo file.bin"}) {
			t.Fatalf("OnFileDrop events = %+v, want one FileDropDrop with /tmp/demo file.bin", gotEvents)
		}
		if sentReply.Type != finishedAtom ||
			clientLong(&sentReply, 0) != targetWin ||
			clientLong(&sentReply, 1) != 1 ||
			clientLong(&sentReply, 2) != actionCopy ||
			clientLong(&sentReply, 4) != 0 {
			t.Fatalf("XdndFinished reply = type %d data [%d %d %d %d %d]",
				sentReply.Type,
				clientLong(&sentReply, 0), clientLong(&sentReply, 1), clientLong(&sentReply, 2),
				clientLong(&sentReply, 3), clientLong(&sentReply, 4))
		}
	})

	for _, scenario := range []struct {
		name      string
		setupPoll func(t *testing.T, converted *bool) (pollEvent func(uintptr) unsafe.Pointer, poll func(*pollFD, uintptr, int32) int32)
	}{
		{
			name: "selection timeout clears hover without failing the event loop",
			setupPoll: func(_ *testing.T, _ *bool) (func(uintptr) unsafe.Pointer, func(*pollFD, uintptr, int32) int32) {
				return func(uintptr) unsafe.Pointer { return nil },
					func(*pollFD, uintptr, int32) int32 { return 0 }
			},
		},
		{
			name: "target-none refusal clears hover immediately without waiting for timeout",
			setupPoll: func(t *testing.T, converted *bool) (func(uintptr) unsafe.Pointer, func(*pollFD, uintptr, int32) int32) {
				refusal := xcbSelectionNotifyEvent{
					ResponseType: xcbSelectionNotify,
					Requestor:    requestorWin,
					Selection:    selectionAtom,
					Target:       xcbAtomNone,
					Property:     xcbAtomNone,
				}
				delivered := false
				return func(uintptr) unsafe.Pointer {
						if !*converted || delivered {
							return nil
						}
						delivered = true
						return unsafe.Pointer(&refusal)
					},
					func(*pollFD, uintptr, int32) int32 {
						t.Fatal("poll should not run after a Target=None refusal")
						return 0
					}
			},
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			converted := false
			pollEvent, pollFn := scenario.setupPoll(t, &converted)
			selAPI := &xcbAPI{
				atoms:              map[string]uint32{"XdndSelection": selectionAtom},
				connectionHasError: func(uintptr) int32 { return 0 },
				flush:              func(uintptr) int32 { return 1 },
				fileDescriptor:     func(uintptr) int32 { return 1 },
				free:               func(unsafe.Pointer) {},
				getSelectionOwner:  func(uintptr, uint32) xcbSelectionOwnerCookie { return xcbSelectionOwnerCookie{} },
				selectionOwnerReply: func(uintptr, xcbSelectionOwnerCookie, *unsafe.Pointer) *xcbSelectionOwnerReply {
					return &xcbSelectionOwnerReply{Owner: sourceWin}
				},
				convertSelection: func(uintptr, uint32, uint32, uint32, uint32, uint32) xcbVoidCookie {
					converted = true
					return xcbVoidCookie{}
				},
				pollForEvent: pollEvent,
				poll:         pollFn,
			}
			installSelectionAPI(t, selAPI)

			var sentReply xcbClientEvent
			winAPI := &xcbAPI{
				sendEvent: func(_ uintptr, _ uint8, _ uint32, _ uint32, data *byte) xcbVoidCookie {
					copy(unsafe.Slice((*byte)(unsafe.Pointer(&sentReply)), unsafe.Sizeof(sentReply)), unsafe.Slice(data, unsafe.Sizeof(sentReply)))
					return xcbVoidCookie{}
				},
				flush: func(uintptr) int32 { return 1 },
			}
			var gotEvents []platform.FileDropEvent
			presented := 0
			state := &linuxWindowState{
				win: &window{
					api:        winAPI,
					connection: 2,
					id:         targetWin,
					xdnd: xdndAtoms{
						drop:         dropAtom,
						finished:     finishedAtom,
						selection:    selectionAtom,
						actionCopy:   actionCopy,
						uriList:      uriListAtom,
						dropProperty: dropProperty,
					},
				},
				config: platform.Config{
					OnFileDrop: func(event platform.FileDropEvent) (platform.Frame, bool, error) {
						gotEvents = append(gotEvents, event)
						return platform.Frame{}, false, nil
					},
				},
				router: eventRouter{present: func(platform.Frame) error {
					presented++
					return nil
				}},
				drag: xdndDrag{source: sourceWin, wantsURIs: true, x: 12, y: 34},
			}

			if err := state.handleDragMessage(makeDropMessage()); err != nil {
				t.Fatalf("handleDragMessage returned error on failed selection read: %v", err)
			}
			if len(gotEvents) != 1 || gotEvents[0].Phase != platform.FileDropLeave || presented != 1 {
				t.Fatalf("events = %+v presented=%d, want one FileDropLeave and one present", gotEvents, presented)
			}
			if sentReply.Type != finishedAtom || clientLong(&sentReply, 1) != 0 || clientLong(&sentReply, 2) != 0 {
				t.Fatalf("XdndFinished reply = type %d flags %d action %d, want rejected",
					sentReply.Type, clientLong(&sentReply, 1), clientLong(&sentReply, 2))
			}
		})
	}
}
