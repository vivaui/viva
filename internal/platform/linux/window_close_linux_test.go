//go:build linux && !android

package linux

import (
	"encoding/binary"
	"testing"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

// deleteRequest is what the window manager sends when the user closes a window.
func deleteRequest(t *testing.T, mode platform.WindowMode, onClose platform.CloseRequestHandler) (closed bool, asked int) {
	t.Helper()
	win := &window{id: 7, wmProtocols: 100, wmDelete: 101}
	state := &linuxWindowState{win: win, config: platform.Config{WindowMode: mode}}
	if onClose != nil {
		state.config.OnCloseRequest = func(allow func()) {
			asked++
			onClose(allow)
		}
	}
	previous := activeWindow.Load()
	defer activeWindow.Store(previous)
	event := xcbClientEvent{ResponseType: xcbClientMessage, Format: 32, Window: win.id, Type: win.wmProtocols}
	binary.LittleEndian.PutUint32(event.Data[:4], win.wmDelete)
	closed, err := state.handleEvent(unsafe.Pointer(&event))
	if err != nil {
		t.Fatal(err)
	}
	return closed, asked
}

func TestClosingAPanelDoesNotAskTheCloseRequest(t *testing.T) {
	closed, asked := deleteRequest(t, platform.WindowPanel, func(func()) {})
	if !closed || asked != 0 {
		t.Fatalf("panel: closed %v, asked %d times; a panel closes at once and is never asked", closed, asked)
	}
}

func TestClosingAWindowAsksTheCloseRequestAndWaits(t *testing.T) {
	for _, mode := range []platform.WindowMode{platform.WindowTitled, platform.WindowCustomTitlebar} {
		closed, asked := deleteRequest(t, mode, func(func()) {})
		if closed || asked != 1 {
			t.Fatalf("mode %d: closed %v, asked %d times; the window stays until the request allows it", mode, closed, asked)
		}
	}
}

func TestClosingAWindowWithoutACloseRequestClosesIt(t *testing.T) {
	for _, mode := range []platform.WindowMode{platform.WindowTitled, platform.WindowCustomTitlebar, platform.WindowPanel} {
		if closed, _ := deleteRequest(t, mode, nil); !closed {
			t.Fatalf("mode %d: a window with no close request must close", mode)
		}
	}
}
