package windows

import (
	"fmt"
	"sync"
	"syscall"

	"github.com/vivaui/viva/internal/platform"
)

// The shortcuts Windows keeps for itself, taken for the window the way mstsc and FreeRDP take
// them for a remote desktop: a low-level keyboard hook that runs while the window is the
// foreground one, eats the shortcut before the system acts on it, and re-posts the same key to
// the window, which then reads it as any other key: the Windows key as a Meta key, Tab with Alt
// held as Tab. Nothing else changes in how the window hears the keyboard.
//
// The hook is installed on the thread that runs the windows, since Windows delivers a low-level
// hook through that thread's message pump, and only while a window that asked for it has the
// focus: while the hook exists every key of the desktop passes through it.

var (
	setWindowsHookEx    = user.NewProc("SetWindowsHookExW")
	unhookWindowsHookEx = user.NewProc("UnhookWindowsHookEx")
	callNextHookEx      = user.NewProc("CallNextHookEx")
	getForegroundWindow = user.NewProc("GetForegroundWindow")
	getAsyncKeyState    = user.NewProc("GetAsyncKeyState")
)

const (
	wmCaptureShortcuts = 32775
	whKeyboardLL       = 13
	llkhfExtended      = 0x01
	llkhfAltDown       = 0x20
	vkTab              = 0x09
	vkEscape           = 0x1b
	vkLeftWin          = 0x5b
	vkRightWin         = 0x5c
)

// kbdLLHook is the KBDLLHOOKSTRUCT a low-level keyboard hook receives.
type kbdLLHook struct {
	VkCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

// shortcutHook is the hook shared by every window. Its fields belong to the thread that runs the
// windows, which installs it and receives what it sees.
var shortcutHook struct {
	handle   uintptr
	users    int
	callback uintptr
	once     sync.Once
	// taken is the keys whose press was eaten and whose release has not come yet, so the
	// release is eaten too: the Windows key opens the Start menu when it is let go.
	taken [256]bool
}

// CaptureSystemShortcuts has the window take the system's shortcuts while it has the keyboard
// focus, and give them back as soon as it has not. It is safe to call from any goroutine.
func (window *nativeWindow) CaptureSystemShortcuts(capture bool) error {
	if window == nil || window.closed.Load() || window.state.window == 0 {
		return fmt.Errorf("windows: shortcut capture requires a running window")
	}
	window.state.shortcuts.Store(capture)
	if posted, _, callErr := postMessage.Call(window.state.window, wmCaptureShortcuts, 0, 0); posted == 0 {
		return windowsCallError("capture shortcuts", callErr)
	}
	return nil
}

// syncShortcutHook installs the hook while this window wants the shortcuts and has the focus, and
// removes it when no window does. It runs on the windows' thread.
func (state *windowState) syncShortcutHook() {
	want := state.shortcuts.Load() && state.focused && state.window != 0
	if want == state.shortcutsHeld {
		return
	}
	state.shortcutsHeld = want
	if want {
		shortcutHook.users++
		if shortcutHook.handle == 0 {
			shortcutHook.once.Do(func() { shortcutHook.callback = syscall.NewCallback(shortcutHookProc) })
			handle, _, _ := setWindowsHookEx.Call(whKeyboardLL, shortcutHook.callback, state.application.instance, 0)
			shortcutHook.handle = handle // 0 leaves the shortcuts with the system
		}
		return
	}
	shortcutHook.users--
	if shortcutHook.users <= 0 {
		shortcutHook.users = 0
		if shortcutHook.handle != 0 {
			unhookWindowsHookEx.Call(shortcutHook.handle)
			shortcutHook.handle = 0
		}
		shortcutHook.taken = [256]bool{}
	}
}

// shortcutHookProc is the hook's callback. It must be quick, and it must not fail: a callback
// that stalls is removed by Windows, and a panic would end the process.
func shortcutHookProc(code, kind, data uintptr) (result uintptr) {
	defer func() {
		if recovered := recover(); recovered != nil {
			platform.ReportFault("shortcut hook", recovered)
			result = 0
		}
	}()
	if int32(code) >= 0 {
		event := (*kbdLLHook)(pointer(data))
		if shortcutTaken(kind, event) {
			return 1
		}
	}
	result, _, _ = callNextHookEx.Call(shortcutHook.handle, code, kind, data)
	return result
}

// shortcutTaken decides whether this key belongs to the system's shortcuts and, if so, hands it
// to the foreground window as an ordinary key message.
func shortcutTaken(kind uintptr, event *kbdLLHook) bool {
	if event.VkCode > 0xff {
		return false
	}
	foreground, _, _ := getForegroundWindow.Call()
	value, found := windowStates.Load(foreground)
	if !found {
		return false
	}
	state := value.(*windowState)
	if !state.shortcuts.Load() || !state.focused {
		return false
	}
	down := kind == wmKeyDown || kind == wmSysKeyDown
	vk := event.VkCode
	if !shortcutKeyBelongsToSystem(vk, down, shortcutHook.taken[vk]) {
		return false
	}
	shortcutHook.taken[vk] = down
	altDown := event.Flags&llkhfAltDown != 0
	message := uintptr(wmKeyDown)
	switch {
	case down && altDown:
		message = wmSysKeyDown
	case !down && altDown:
		message = wmSysKeyUp
	case !down:
		message = wmKeyUp
	}
	postMessage.Call(foreground, message, uintptr(vk), shortcutKeyLParam(event, down, altDown))
	return true
}

// shortcutKeyBelongsToSystem says whether the key is part of a shortcut the system keeps: the
// Windows key and every chord it starts, Alt+Tab, Alt+Esc and Ctrl+Esc (which includes
// Ctrl+Shift+Esc). The release of a key whose press was eaten is eaten as well.
func shortcutKeyBelongsToSystem(vk uint32, down, tookPress bool) bool {
	if !down && tookPress {
		return true
	}
	if vk == vkLeftWin || vk == vkRightWin {
		return true // the Windows key alone opens the Start menu
	}
	if !down {
		return false
	}
	held := func(keys ...uintptr) bool {
		for _, key := range keys {
			if state, _, _ := getAsyncKeyState.Call(key); state&0x8000 != 0 {
				return true
			}
		}
		return false
	}
	switch {
	case held(vkLeftWin, vkRightWin):
		return true
	case held(0xa4, 0xa5) && (vk == vkTab || vk == vkEscape): // Alt, left and right
		return true
	case held(0xa2, 0xa3) && vk == vkEscape: // Control, left and right
		return true
	}
	return false
}

// shortcutKeyLParam is the lParam Windows would have given the key's message: repeat count,
// scan code, the extended-key flag, the Alt context, the previous state and the transition.
func shortcutKeyLParam(event *kbdLLHook, down, altDown bool) uintptr {
	lParam := uintptr(1) | uintptr(event.ScanCode&0xff)<<16
	if event.Flags&llkhfExtended != 0 {
		lParam |= 1 << 24
	}
	if altDown {
		lParam |= 1 << 29
	}
	if !down {
		lParam |= 1<<30 | 1<<31
	} else if shortcutHook.taken[event.VkCode] {
		lParam |= 1 << 30 // already down: this press repeats
	}
	return lParam
}
