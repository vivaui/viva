//go:build linux

package linux

import (
	"encoding/binary"
	"os"
	"testing"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

func openWindow(config platform.Config) (*window, error) {
	return (&linuxApplication{}).openWindow(config)
}

// windowProperty reads a property of the window as raw bytes and its format.
func windowProperty(t *testing.T, win *window, name string, kind uint32) ([]byte, uint8) {
	t.Helper()
	atom, err := win.api.atom(win.connection, name)
	if err != nil {
		t.Fatal(err)
	}
	cookie := win.api.getProperty(win.connection, 0, win.id, atom, kind, 0, 1024)
	var nativeError unsafe.Pointer
	reply := win.api.getPropertyReply(win.connection, cookie, &nativeError)
	if nativeError != nil {
		win.api.free(nativeError)
	}
	if reply == nil {
		return nil, 0
	}
	defer win.api.free(unsafe.Pointer(reply))
	length := int(win.api.getPropertyLength(reply))
	if length <= 0 {
		return nil, reply.Format
	}
	return append([]byte(nil), unsafe.Slice((*byte)(win.api.getPropertyValue(reply)), length)...), reply.Format
}

func TestWindowTellsTheWindowManagerWhoItIs(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no X display")
	}
	win, err := openWindow(platform.Config{Title: "终端 Viva", Width: 120, Height: 80})
	if err != nil {
		t.Skipf("no X window: %v", err)
	}
	defer win.close()
	name, _ := windowProperty(t, win, "_NET_WM_NAME", 0)
	if string(name) != "终端 Viva" {
		t.Fatalf("_NET_WM_NAME = %q", name)
	}
	legacy, _ := windowProperty(t, win, "WM_NAME", xcbAtomString)
	if string(legacy) != "?? Viva" {
		t.Fatalf("WM_NAME = %q", legacy)
	}
	pid, format := windowProperty(t, win, "_NET_WM_PID", xcbAtomCardinal)
	if format != 32 || len(pid) != 4 || int(binary.LittleEndian.Uint32(pid)) != os.Getpid() {
		t.Fatalf("_NET_WM_PID = %v (format %d), want %d", pid, format, os.Getpid())
	}
	if machine, _ := windowProperty(t, win, "WM_CLIENT_MACHINE", xcbAtomString); len(machine) == 0 {
		t.Fatal("WM_CLIENT_MACHINE is empty")
	}
	protocols, _ := windowProperty(t, win, "WM_PROTOCOLS", xcbAtomAtom)
	have := map[uint32]bool{}
	for offset := 0; offset+4 <= len(protocols); offset += 4 {
		have[binary.LittleEndian.Uint32(protocols[offset:])] = true
	}
	if !have[win.wmDelete] || win.pingAtom == 0 || !have[win.pingAtom] {
		t.Fatalf("WM_PROTOCOLS %v lacks the close request or the ping", protocols)
	}
	if win.syncCounter == nil {
		t.Log("no XSYNC on this server: resizes are not paced")
		return
	}
	counter, _ := windowProperty(t, win, "_NET_WM_SYNC_REQUEST_COUNTER", xcbAtomCardinal)
	if len(counter) != 4 || binary.LittleEndian.Uint32(counter) != win.syncCounter.id || !have[win.syncRequestAtom] {
		t.Fatalf("sync counter %v (want %d), protocols %v", counter, win.syncCounter.id, protocols)
	}
}

func TestWindowAnswersSyncRequestsAndPings(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no X display")
	}
	win, err := openWindow(platform.Config{Title: "Viva sync", Width: 120, Height: 80})
	if err != nil {
		t.Skipf("no X window: %v", err)
	}
	defer win.close()
	if win.syncCounter == nil {
		t.Skip("no XSYNC on this server")
	}
	var message xcbClientEvent
	message.ResponseType, message.Format, message.Window, message.Type = xcbClientMessage, 32, win.id, win.wmProtocols
	binary.LittleEndian.PutUint32(message.Data[0:], win.syncRequestAtom)
	binary.LittleEndian.PutUint32(message.Data[8:], 6)
	if !win.protocolMessage(&message) || !win.syncPending.pending {
		t.Fatal("the sync request was not taken")
	}
	win.answerSync()
	if win.syncPending.pending {
		t.Fatal("the sync request was not answered")
	}
	binary.LittleEndian.PutUint32(message.Data[0:], win.pingAtom)
	if !win.protocolMessage(&message) {
		t.Fatal("the ping was not answered")
	}
	binary.LittleEndian.PutUint32(message.Data[0:], win.wmDelete)
	if win.protocolMessage(&message) {
		t.Fatal("a close request is not a protocol message of this kind")
	}
}
