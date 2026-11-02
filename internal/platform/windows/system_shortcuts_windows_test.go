package windows

import (
	"runtime"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
)

var (
	sendInput       = user.NewProc("SendInput")
	findWindow      = user.NewProc("FindWindowW")
	switchToThisWnd = user.NewProc("SwitchToThisWindow")
)

// keyInput is INPUT for a keyboard event on 64-bit Windows.
type keyInput struct {
	Type  uint32
	_     uint32
	Vk    uint16
	Scan  uint16
	Flags uint32
	Time  uint32
	_     uint32
	Extra uintptr
	_     [8]byte
}

func sendKeys(t *testing.T, keys ...struct {
	vk   uint16
	down bool
}) {
	t.Helper()
	for _, key := range keys {
		event := keyInput{Type: 1, Vk: key.vk}
		if !key.down {
			event.Flags = 2 // KEYEVENTF_KEYUP
		}
		if sent, _, _ := sendInput.Call(1, uintptr(unsafe.Pointer(&event)), unsafe.Sizeof(event)); sent != 1 {
			t.Fatal("SendInput refused the key")
		}
		time.Sleep(40 * time.Millisecond)
	}
}

// The Windows key and the chords it starts belong to the shell until a window that asked for them
// has the focus: then Win+F2 and Ctrl+Esc reach the window as the keys they end in, and the shell
// does not take the foreground for its Start menu. The hook is gone again once the window gives
// the shortcuts back.
func TestAFocusedWindowTakesTheWindowsKeyChords(t *testing.T) {
	var mu sync.Mutex
	var keys []input.Key
	done := make(chan error, 1)
	ready := make(chan *nativeWindow, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		done <- RunApplication(func(application platform.Application) error {
			window, err := application.OpenWindow(platform.Config{
				Title: "shortcuts", Width: 300, Height: 200,
				OnKey: func(event platform.KeyEvent) (platform.Frame, error) {
					mu.Lock()
					keys = append(keys, event.Key)
					mu.Unlock()
					return platform.Frame{}, nil
				},
			})
			if err == nil {
				ready <- window.(*nativeWindow)
			}
			return err
		})
	}()
	var window *nativeWindow
	select {
	case window = <-ready:
	case err := <-done:
		t.Fatalf("window failed to open: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("the window never opened")
	}
	defer func() {
		postMessage.Call(window.state.window, wmClose, 0, 0)
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("the window did not close")
		}
	}()

	switchToThisWnd.Call(window.state.window, 1)
	foreground := func() bool {
		handle, _, _ := getForegroundWindow.Call()
		return handle == window.state.window
	}
	deadline := time.Now().Add(5 * time.Second)
	for !foreground() && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if !foreground() {
		t.Skip("this session lets no window take the foreground")
	}
	if err := window.CaptureSystemShortcuts(true); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond) // the hook is installed on the window's thread

	const vkWin, vkF2, vkControl, vkEscapeKey = 0x5b, 0x71, 0x11, 0x1b
	type press = struct {
		vk   uint16
		down bool
	}
	sendKeys(t, press{vkWin, true}, press{vkF2, true}, press{vkF2, false}, press{vkWin, false})
	sendKeys(t, press{vkControl, true}, press{vkEscapeKey, true}, press{vkEscapeKey, false}, press{vkControl, false})
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	got := append([]input.Key(nil), keys...)
	mu.Unlock()
	if !containsKey(got, input.KeyF1+1) {
		t.Fatalf("Win+F2 did not reach the window as F2: keys %v", got)
	}
	if !containsKey(got, input.KeyEscape) {
		t.Fatalf("Ctrl+Esc did not reach the window as Escape: keys %v", got)
	}
	if !foreground() {
		t.Error("the window lost the foreground to a shortcut it took")
	}

	if err := window.CaptureSystemShortcuts(false); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(3 * time.Second)
	for shortcutHook.handle != 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if shortcutHook.handle != 0 {
		t.Error("the keyboard hook stayed installed after the shortcuts were given back")
	}
}

func containsKey(keys []input.Key, want input.Key) bool {
	for _, key := range keys {
		if key == want {
			return true
		}
	}
	return false
}

// The press of a shortcut key is eaten with its release, so the Start menu does not open when
// the Windows key is let go; and the rules name exactly the keys the system keeps.
func TestTheShortcutRulesNameTheKeysTheSystemKeeps(t *testing.T) {
	if !shortcutKeyBelongsToSystem(vkLeftWin, true, false) || !shortcutKeyBelongsToSystem(vkRightWin, false, false) {
		t.Error("the Windows key alone is not taken")
	}
	if !shortcutKeyBelongsToSystem(0x41, false, true) {
		t.Error("the release of a key whose press was eaten was not eaten")
	}
	if shortcutKeyBelongsToSystem(0x41, true, false) || shortcutKeyBelongsToSystem(0x41, false, false) {
		t.Error("an ordinary key was taken")
	}
	if shortcutKeyBelongsToSystem(vkTab, true, false) || shortcutKeyBelongsToSystem(vkEscape, true, false) {
		t.Error("Tab or Esc with no modifier held was taken")
	}
}
