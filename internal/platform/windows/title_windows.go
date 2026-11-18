package windows

import (
	"fmt"
	"sync"
	"syscall"
	"unsafe"
)

// wmSetTitle asks the window thread to apply the title SetTitle stored.
// SetWindowTextW from another thread would send WM_SETTEXT and wait for the
// window thread, so the change is posted instead. Its number is the next free
// one after wmSetSize (0x8052): the window procedure switches on every private
// message, and two of them with one number do not compile.
const wmSetTitle = 0x8053

var setWindowText = user.NewProc("SetWindowTextW")

// titleRequest holds the latest title asked for until the window thread applies it.
type titleRequest struct {
	mu      sync.Mutex
	title   string
	pending bool
}

func (window *nativeWindow) SetTitle(title string) error {
	if window == nil || window.closed.Load() {
		return fmt.Errorf("windows: window is closed")
	}
	request := &window.state.pendingTitle
	request.mu.Lock()
	request.title, request.pending = title, true
	request.mu.Unlock()
	if ok, _, err := postMessage.Call(window.state.window, wmSetTitle, 0, 0); ok == 0 {
		return fmt.Errorf("windows: request title: %w", err)
	}
	return nil
}

// applyTitle sets the stored title on the window and in the accessibility tree.
func (state *windowState) applyTitle() {
	request := &state.pendingTitle
	request.mu.Lock()
	title, pending := request.title, request.pending
	request.pending = false
	request.mu.Unlock()
	if !pending || state.window == 0 {
		return
	}
	value, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return
	}
	state.config.Title = title
	setWindowText.Call(state.window, uintptr(unsafe.Pointer(value)))
}
