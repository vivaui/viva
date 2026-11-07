package linux

import (
	"bytes"
	"testing"
	"unsafe"
)

type propertyWrite struct {
	window, property, kind uint32
	format                 uint8
	items                  uint32
	data                   []byte
}

// selectionRecorder is an X connection that records what a selection owner writes.
type selectionRecorder struct {
	writes     []propertyWrite
	masks      map[uint32]uint32
	sent       []xcbSelectionNotifyWire
	incrAtom   uint32
	ownerCalls [][2]uint32
}

func (recorder *selectionRecorder) api() *xcbAPI {
	reply := &xcbAtomReply{}
	return &xcbAPI{
		changeProperty: func(_ uintptr, _ uint8, window, property, kind uint32, format uint8, items uint32, data unsafe.Pointer) xcbVoidCookie {
			write := propertyWrite{window: window, property: property, kind: kind, format: format, items: items}
			if data != nil {
				length := int(items)
				if format == 32 {
					length *= 4
				}
				write.data = append([]byte(nil), unsafe.Slice((*byte)(data), length)...)
			}
			recorder.writes = append(recorder.writes, write)
			return xcbVoidCookie{}
		},
		changeWindow: func(_ uintptr, window, _ uint32, mask *uint32) xcbVoidCookie {
			recorder.masks[window] = *mask
			return xcbVoidCookie{}
		},
		sendEvent: func(_ uintptr, _ uint8, _, _ uint32, data *byte) xcbVoidCookie {
			var event xcbSelectionNotifyWire
			copy(unsafe.Slice((*byte)(unsafe.Pointer(&event)), 32), unsafe.Slice(data, 32))
			recorder.sent = append(recorder.sent, event)
			return xcbVoidCookie{}
		},
		setSelectionOwner: func(_ uintptr, owner, selection, _ uint32) xcbVoidCookie {
			recorder.ownerCalls = append(recorder.ownerCalls, [2]uint32{owner, selection})
			return xcbVoidCookie{}
		},
		getSelectionOwner: func(uintptr, uint32) xcbSelectionOwnerCookie { return xcbSelectionOwnerCookie{} },
		selectionOwnerReply: func(uintptr, xcbSelectionOwnerCookie, *unsafe.Pointer) *xcbSelectionOwnerReply {
			return &xcbSelectionOwnerReply{Owner: 2}
		},
		internAtom: func(uintptr, uint8, uint16, *byte) xcbAtomCookie { return xcbAtomCookie{} },
		internAtomReply: func(uintptr, xcbAtomCookie, *unsafe.Pointer) *xcbAtomReply {
			reply.Atom = recorder.incrAtom
			return reply
		},
		flush:              func(uintptr) int32 { return 1 },
		connectionHasError: func(uintptr) int32 { return 0 },
		free:               func(unsafe.Pointer) {},
	}
}

func newSelectionWindow(recorder *selectionRecorder, text string) *window {
	recorder.masks = map[uint32]uint32{}
	recorder.incrAtom = 77
	return &window{
		api: recorder.api(), connection: 1, id: 2, clipboard: text, ownsClipboard: true,
		clipboardAtom: 10, utf8Atom: 11, textAtom: 12, targetsAtom: 13, uriListAtom: 15, gnomeFilesAtom: 16,
	}
}

func TestAClipboardFileListIsOfferedAsURIsAndNames(t *testing.T) {
	recorder := &selectionRecorder{}
	win := newSelectionWindow(recorder, "a b.txt\n中.txt")
	win.clipboardFiles = []string{"/tmp/a b.txt", "/tmp/中.txt"}
	request := func(target uint32) propertyWrite {
		t.Helper()
		before := len(recorder.writes)
		if err := win.handleSelectionRequest(&xcbSelectionRequestEvent{Requestor: 3, Selection: 10, Target: target, Property: 14}); err != nil {
			t.Fatal(err)
		}
		if len(recorder.writes) != before+1 {
			t.Fatalf("target %d wrote %d times", target, len(recorder.writes)-before)
		}
		return recorder.writes[before]
	}
	targets := request(13)
	if targets.kind != xcbAtomAtom {
		t.Fatalf("TARGETS kind = %d, want atom", targets.kind)
	}
	var offered []uint32
	for offset := 0; offset+4 <= len(targets.data); offset += 4 {
		offered = append(offered, uint32(targets.data[offset])|uint32(targets.data[offset+1])<<8|uint32(targets.data[offset+2])<<16|uint32(targets.data[offset+3])<<24)
	}
	for _, want := range []uint32{15, 16} {
		found := false
		for _, atom := range offered {
			found = found || atom == want
		}
		if !found {
			t.Fatalf("TARGETS %v do not include %d", offered, want)
		}
	}
	if got := string(request(15).data); got != "file:///tmp/a%20b.txt\r\nfile:///tmp/%E4%B8%AD.txt\r\n" {
		t.Fatalf("text/uri-list = %q", got)
	}
	if got := string(request(16).data); got != "copy\nfile:///tmp/a%20b.txt\r\nfile:///tmp/%E4%B8%AD.txt\r\n" {
		t.Fatalf("gnome-copied-files = %q", got)
	}
	if got := string(request(11).data); got != "a b.txt\n中.txt" {
		t.Fatalf("UTF8_STRING = %q", got)
	}
}

func TestALargeClipboardGoesByINCRAndEndsWithAnEmptyPiece(t *testing.T) {
	text := string(bytes.Repeat([]byte("a"), incrChunk+10))
	recorder := &selectionRecorder{}
	win := newSelectionWindow(recorder, text)
	const requestor, property = 3, 14
	if err := win.handleSelectionRequest(&xcbSelectionRequestEvent{Time: 5, Owner: 2, Requestor: requestor, Selection: 10, Target: 11, Property: property}); err != nil {
		t.Fatal(err)
	}
	if len(recorder.writes) != 1 || recorder.writes[0].kind != recorder.incrAtom || recorder.writes[0].format != 32 {
		t.Fatalf("the first write is not the INCR announcement: %+v", recorder.writes)
	}
	if recorder.masks[requestor] != xcbEventPropertyChange {
		t.Fatalf("the requestor's property changes are not listened to: %#x", recorder.masks[requestor])
	}
	if len(recorder.sent) != 1 || recorder.sent[0].Property != property {
		t.Fatalf("the requestor was not told: %+v", recorder.sent)
	}

	// An unrelated change of the property is not a request for the next piece.
	if win.incrEvent(&xcbPropertyEvent{Window: requestor, Atom: property, State: 0}) {
		t.Fatal("a new value was taken for a deletion")
	}
	var collected []byte
	for step := 0; step < 5; step++ {
		before := len(recorder.writes)
		if !win.incrEvent(&xcbPropertyEvent{Window: requestor, Atom: property, State: xcbPropertyDelete}) {
			t.Fatalf("deletion %d was not taken", step)
		}
		write := recorder.writes[before]
		if write.kind != 11 || write.format != 8 {
			t.Fatalf("piece %d: %+v", step, write)
		}
		if write.items == 0 {
			break
		}
		collected = append(collected, write.data...)
	}
	if string(collected) != text {
		t.Fatalf("collected %d bytes of %d", len(collected), len(text))
	}
	if recorder.masks[requestor] != 0 {
		t.Fatalf("the requestor is still listened to: %#x", recorder.masks[requestor])
	}
	if win.incrEvent(&xcbPropertyEvent{Window: requestor, Atom: property, State: xcbPropertyDelete}) {
		t.Fatal("a finished stream wrote again")
	}
}

func TestASmallClipboardIsWrittenWhole(t *testing.T) {
	recorder := &selectionRecorder{}
	win := newSelectionWindow(recorder, "hello")
	if err := win.handleSelectionRequest(&xcbSelectionRequestEvent{Requestor: 3, Selection: 10, Target: 11, Property: 14}); err != nil {
		t.Fatal(err)
	}
	if len(recorder.writes) != 1 || recorder.writes[0].kind != 11 || string(recorder.writes[0].data) != "hello" || len(win.selections.incr.list) != 0 {
		t.Fatalf("writes %+v", recorder.writes)
	}
}

func TestThePrimarySelectionIsOfferedApartFromTheClipboard(t *testing.T) {
	recorder := &selectionRecorder{}
	win := newSelectionWindow(recorder, "clipboard text")
	win.selections.primary, win.selections.ownsPrimary = "selected text", true
	if err := win.handleSelectionRequest(&xcbSelectionRequestEvent{Requestor: 3, Selection: xcbAtomPrimary, Target: 11, Property: 14}); err != nil {
		t.Fatal(err)
	}
	if len(recorder.writes) != 1 || string(recorder.writes[0].data) != "selected text" {
		t.Fatalf("writes %+v", recorder.writes)
	}
	// A selection the window does not own is refused: the notify carries no property.
	recorder.writes, recorder.sent = nil, nil
	win.selections.ownsPrimary = false
	if err := win.handleSelectionRequest(&xcbSelectionRequestEvent{Requestor: 3, Selection: xcbAtomPrimary, Target: 11, Property: 14}); err != nil {
		t.Fatal(err)
	}
	if len(recorder.writes) != 0 || len(recorder.sent) != 1 || recorder.sent[0].Property != xcbAtomNone {
		t.Fatalf("writes %+v sent %+v", recorder.writes, recorder.sent)
	}
	win.handleSelectionClear(&xcbSelectionClearEvent{Selection: xcbAtomPrimary})
	win.selections.primary, win.selections.ownsPrimary = "again", true
	win.handleSelectionClear(&xcbSelectionClearEvent{Selection: xcbAtomPrimary})
	if win.selections.ownsPrimary || win.selections.primary != "" || !win.ownsClipboard {
		t.Fatalf("clearing PRIMARY touched the wrong selection: %+v clipboard owned %v", win.selections, win.ownsClipboard)
	}
}

func TestSetPrimaryTextClaimsPrimaryAndEmptyTextGivesItUp(t *testing.T) {
	recorder := &selectionRecorder{}
	win := newSelectionWindow(recorder, "")
	win.ownsClipboard = false
	activeWindow.Store(win)
	t.Cleanup(func() { activeWindow.Store(nil) })
	if err := SetPrimaryText("picked"); err != nil {
		t.Fatal(err)
	}
	if len(recorder.ownerCalls) != 1 || recorder.ownerCalls[0] != [2]uint32{2, xcbAtomPrimary} {
		t.Fatalf("owner calls %v", recorder.ownerCalls)
	}
	if text, err := PrimaryText(); err != nil || text != "picked" {
		t.Fatalf("PrimaryText = %q, %v", text, err)
	}
	if err := SetPrimaryText(""); err != nil {
		t.Fatal(err)
	}
	if len(recorder.ownerCalls) != 2 || recorder.ownerCalls[1] != [2]uint32{xcbAtomNone, xcbAtomPrimary} || win.selections.ownsPrimary {
		t.Fatalf("owner calls %v, owned %v", recorder.ownerCalls, win.selections.ownsPrimary)
	}
	if err := SetPrimaryText("bad\xff"); err == nil {
		t.Fatal("invalid UTF-8 was accepted")
	}
}
