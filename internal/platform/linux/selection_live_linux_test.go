//go:build !android

package linux

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
)

// These tests make a real window own a selection on the X server and have xclip, another
// process, read it, and the other way round. They need what the other XCB smoke tests need.

func liveSelectionWindow(t *testing.T) *window {
	t.Helper()
	if testing.Short() || os.Getenv("VIVA_LINUX_XCB_SMOKE") == "" {
		t.Skip("set VIVA_LINUX_XCB_SMOKE=1 with an X11 display and xclip")
	}
	if _, err := exec.LookPath("xclip"); err != nil {
		t.Skip("xclip is not installed")
	}
	win, err := openWindow(platform.Config{Title: "Viva selection smoke", Width: 160, Height: 100, WindowMode: platform.WindowTitled})
	if err != nil {
		t.Fatal(err)
	}
	if !activeWindow.CompareAndSwap(nil, win) {
		win.close()
		t.Fatal("an active window already exists")
	}
	t.Cleanup(func() {
		activeWindow.Store(nil)
		win.close()
	})
	return win
}

// pumpSelectionEvents answers the requests for the window's selections, as its event loop does.
func pumpSelectionEvents(win *window) (stop func()) {
	done := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		file := pollFD{FD: win.api.fileDescriptor(win.connection), Events: xcbPollInput}
		for {
			select {
			case <-done:
				return
			default:
			}
			for event := win.api.pollForEvent(win.connection); event != nil; event = win.api.pollForEvent(win.connection) {
				switch (*xcbGenericEvent)(event).ResponseType & 0x7f {
				case xcbSelectionRequest:
					_ = win.handleSelectionRequest((*xcbSelectionRequestEvent)(event))
				case xcbPropertyNotify:
					win.incrEvent((*xcbPropertyEvent)(event))
				case xcbSelectionClear:
					win.handleSelectionClear((*xcbSelectionClearEvent)(event))
				}
				win.api.free(event)
			}
			win.api.poll(&file, 1, 20)
		}
	}()
	var once sync.Once
	return func() { once.Do(func() { close(done); <-finished }) }
}

func xclipOut(t *testing.T, selection string) []byte {
	t.Helper()
	output, err := exec.Command("xclip", "-o", "-selection", selection).Output()
	if err != nil {
		t.Fatalf("xclip -o -selection %s: %v", selection, err)
	}
	return output
}

// xclipIn makes xclip own a selection with text. It stays in the background until another
// owner takes the selection over.
func xclipIn(t *testing.T, selection string, text []byte) {
	t.Helper()
	command := exec.Command("xclip", "-i", "-selection", selection)
	command.Stdin = bytes.NewReader(text)
	if err := command.Run(); err != nil {
		t.Fatalf("xclip -i -selection %s: %v", selection, err)
	}
	time.Sleep(100 * time.Millisecond)
}

func TestThePrimarySelectionIsPastedByAnotherProgramAndReadBackFromIt(t *testing.T) {
	win := liveSelectionWindow(t)
	stop := pumpSelectionEvents(win)
	defer stop()

	const selected = "选中的 text"
	value := selected + strings.Repeat("a", 1<<20)
	previous := platform.TextState{Value: value}
	next := platform.TextState{Value: value, Selection: text.Range{Start: len([]rune(selected))}}
	input := &linuxTextInput{}
	input.publishPrimary(previous, next)
	if got := string(xclipOut(t, "primary")); got != selected {
		t.Fatalf("xclip read %q from PRIMARY", got)
	}
	// An empty CLIPBOARD makes xclip fail; what matters is that it does not hold the selection.
	if leaked, _ := exec.Command("xclip", "-o", "-selection", "clipboard").Output(); string(leaked) == "选中的 text" {
		t.Fatal("PRIMARY leaked into CLIPBOARD")
	}

	// Another program takes the selection over: the window learns it lost it.
	xclipIn(t, "primary", []byte("from xclip"))
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		win.mu.Lock()
		owned := win.selections.ownsPrimary
		win.mu.Unlock()
		if !owned {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	activeWindow.Store(nil)
	got, err := PrimaryText()
	activeWindow.Store(win)
	if err != nil || got != "from xclip" {
		t.Fatalf("PrimaryText = %q, %v", got, err)
	}
}

// tkClipboardDigest reads the CLIPBOARD with Tk, a toolkit that follows the INCR protocol, and
// returns the SHA-256 of the text. xclip is not used to read a large selection: it sometimes
// stops after some of the pieces of a stream that Tk and Viva's own reader take whole.
func tkClipboardDigest(t *testing.T) [sha256.Size]byte {
	t.Helper()
	script := "import tkinter, hashlib; r = tkinter.Tk(); r.withdraw(); print(hashlib.sha256(r.clipboard_get().encode()).hexdigest())"
	output, err := exec.Command("python3", "-c", script).CombinedOutput()
	if err != nil {
		t.Skipf("no python3 with tkinter: %v: %s", err, output)
	}
	var digest [sha256.Size]byte
	if decoded, err := hex.DecodeString(strings.TrimSpace(string(output))); err != nil || len(decoded) != len(digest) {
		t.Fatalf("tk answered %q", output)
	} else {
		copy(digest[:], decoded)
	}
	return digest
}

func TestAHugeClipboardCrossesTheServerInBothDirectionsByINCR(t *testing.T) {
	win := liveSelectionWindow(t)
	stop := pumpSelectionEvents(win)
	defer stop()

	big := strings.Repeat("0123456789abcdef 你好\n", 60_000) // about 1.4 MB
	if err := SetClipboardText(big); err != nil {
		t.Fatal(err)
	}
	if got := tkClipboardDigest(t); got != sha256.Sum256([]byte(big)) {
		t.Fatal("Tk did not read the whole clipboard")
	}
	// The window's own reader takes it whole as well, from the same owner.
	if value, err := readSelectionTextX11("CLIPBOARD"); err != nil || value != big {
		t.Fatalf("the reader returned %d bytes of %d, %v", len(value), len(big), err)
	}

	// And into the window: xclip serves a large selection by INCR.
	other := strings.Repeat("ABCDEFGH ", 90_000)
	xclipIn(t, "clipboard", []byte(other))
	activeWindow.Store(nil)
	value, err := ClipboardText()
	activeWindow.Store(win)
	if err != nil {
		t.Fatal(err)
	}
	if len(value) != len(other) || value != other {
		t.Fatalf("ClipboardText returned %d bytes of %d", len(value), len(other))
	}
}

func TestAClosingWindowHandsItsClipboardToTheManagerIfThereIsOne(t *testing.T) {
	win := liveSelectionWindow(t)
	manager, err := win.api.atom(win.connection, "CLIPBOARD_MANAGER")
	if err != nil {
		t.Fatal(err)
	}
	if owner, err := selectionOwner(win.api, win.connection, manager); err != nil || owner == xcbAtomNone {
		t.Skip("no clipboard manager on this display")
	}
	if err := SetClipboardText("outlives the window"); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	win.saveClipboardToManager()
	t.Logf("the manager answered after %v", time.Since(start))
	activeWindow.Store(nil)
	win.close()
	if got := string(xclipOut(t, "clipboard")); got != "outlives the window" {
		t.Fatalf("after the window closed the clipboard holds %q", got)
	}
}

// fakeClipboardManager is what a desktop's clipboard manager does when a client asks it to
// save the clipboard (the freedesktop clipboard manager protocol): it owns CLIPBOARD_MANAGER,
// reads the client's data when asked to convert SAVE_TARGETS, answers the client, then owns the
// CLIPBOARD itself and serves what it saved.
type fakeClipboardManager struct {
	api        *xcbAPI
	connection uintptr
	window     uint32
	saved      chan string
	stop       chan struct{}
	finished   chan struct{}
}

func startFakeClipboardManager(t *testing.T) *fakeClipboardManager {
	t.Helper()
	api, connection, screen, err := openSystemConnection()
	if err != nil {
		t.Fatal(err)
	}
	manager := &fakeClipboardManager{api: api, connection: connection, saved: make(chan string, 1), stop: make(chan struct{}), finished: make(chan struct{})}
	manager.window = api.generateID(connection)
	eventMask := uint32(xcbEventPropertyChange)
	api.createWindow(connection, xcbCopyFromParent, manager.window, screen.Root, 0, 0, 1, 1, 0, xcbWindowClassInput, screen.RootVisual, xcbCWEventMask, &eventMask)
	atom := func(name string) uint32 {
		value, err := api.atom(connection, name)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	managerAtom, save, clipboard := atom("CLIPBOARD_MANAGER"), atom("SAVE_TARGETS"), atom("CLIPBOARD")
	utf8, targets := atom("UTF8_STRING"), atom("TARGETS")
	if owner, _ := selectionOwner(api, connection, managerAtom); owner != xcbAtomNone {
		api.destroyWindow(connection, manager.window)
		api.disconnect(connection)
		t.Skip("the display has a clipboard manager of its own")
	}
	api.setSelectionOwner(connection, manager.window, managerAtom, 0)
	api.flush(connection)
	go func() {
		defer close(manager.finished)
		file := pollFD{FD: api.fileDescriptor(connection), Events: xcbPollInput}
		var text string
		reply := func(request *xcbSelectionRequestEvent, property uint32) {
			notify := xcbSelectionNotifyWire{xcbSelectionNotifyEvent: xcbSelectionNotifyEvent{
				ResponseType: xcbSelectionNotify, Time: request.Time, Requestor: request.Requestor,
				Selection: request.Selection, Target: request.Target, Property: property,
			}}
			api.sendEvent(connection, 0, request.Requestor, 0, (*byte)(unsafe.Pointer(&notify)))
			api.flush(connection)
		}
		for {
			select {
			case <-manager.stop:
				return
			default:
			}
			for event := api.pollForEvent(connection); event != nil; event = api.pollForEvent(connection) {
				if (*xcbGenericEvent)(event).ResponseType&0x7f == xcbSelectionRequest {
					request := (*xcbSelectionRequestEvent)(event)
					switch {
					case request.Selection == managerAtom && request.Target == save:
						// Read the client's data, as the protocol has the manager do, then say it is saved.
						property := atom("VIVA_FAKE_MANAGER")
						value, accepted, err := readSelection(api, connection, manager.window, clipboard, utf8, property)
						if err == nil && accepted {
							text = value
							manager.saved <- value
							api.setSelectionOwner(connection, manager.window, clipboard, 0)
						}
						reply(request, request.Property)
					case request.Selection == clipboard && request.Target == utf8 && text != "":
						api.changeProperty(connection, xcbPropertyReplace, request.Requestor, request.Property, utf8, 8, uint32(len(text)), unsafe.Pointer(unsafe.StringData(text)))
						reply(request, request.Property)
					case request.Selection == clipboard && request.Target == targets && text != "":
						list := [...]uint32{targets, utf8}
						api.changeProperty(connection, xcbPropertyReplace, request.Requestor, request.Property, xcbAtomAtom, 32, uint32(len(list)), unsafe.Pointer(&list[0]))
						reply(request, request.Property)
					default:
						reply(request, xcbAtomNone)
					}
				}
				api.free(event)
			}
			api.poll(&file, 1, 20)
		}
	}()
	t.Cleanup(func() {
		close(manager.stop)
		<-manager.finished
		api.destroyWindow(connection, manager.window)
		api.flush(connection)
		api.disconnect(connection)
	})
	return manager
}

func TestAClosingWindowLeavesItsClipboardWithAManagerThatFollowsTheProtocol(t *testing.T) {
	win := liveSelectionWindow(t)
	manager := startFakeClipboardManager(t)
	if err := SetClipboardText("outlives 窗口"); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	win.saveClipboardToManager()
	if elapsed := time.Since(start); elapsed >= managerWait {
		t.Fatalf("the window waited %v: the manager's answer was not recognized", elapsed)
	}
	select {
	case saved := <-manager.saved:
		if saved != "outlives 窗口" {
			t.Fatalf("the manager saved %q", saved)
		}
	default:
		t.Fatal("the manager was never asked for the data")
	}
	activeWindow.Store(nil)
	win.close()
	if got := string(xclipOut(t, "clipboard")); got != "outlives 窗口" {
		t.Fatalf("after the window closed the clipboard holds %q", got)
	}
}

// Reads of the clipboard share one X connection and its window, made by the
// first read and kept, so a paste costs no connection of its own.
func TestClipboardReadsShareOneConnection(t *testing.T) {
	if os.Getenv("VIVA_LINUX_XCB_SMOKE") != "1" || os.Getenv("DISPLAY") == "" {
		t.Skip("set VIVA_LINUX_XCB_SMOKE=1 with an X11 display")
	}
	if _, err := readClipboardX11(); err != nil {
		t.Fatal(err)
	}
	selections.mu.Lock()
	first, window := selections.connection, selections.requestor
	selections.mu.Unlock()
	if _, err := readClipboardX11(); err != nil {
		t.Fatal(err)
	}
	selections.mu.Lock()
	defer selections.mu.Unlock()
	if first == 0 || selections.connection != first || selections.requestor != window {
		t.Fatalf("the second read used connection %#x window %d, the first %#x window %d", selections.connection, selections.requestor, first, window)
	}
}
