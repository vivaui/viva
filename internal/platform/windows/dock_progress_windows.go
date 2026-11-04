//go:build windows

package windows

import (
	"math"
	"unsafe"
)

// The taskbar draws a bar across the application's button, which is what a
// download or an upload shows there. The shell owns the drawing: a program
// creates ITaskbarList3 once and tells it, per window, how far along it is.
// The counts are thousandths of the whole, which keeps them inside a machine
// word whatever the size of the transfer.
var (
	clsidTaskbarList = guid{
		Data1: 0x56FDF344, Data2: 0xFD6D, Data3: 0x11D0,
		Data4: [8]byte{0x95, 0x8A, 0x00, 0x60, 0x97, 0xC9, 0xA0, 0x90},
	}
	iidTaskbarList3 = guid{
		Data1: 0xEA1AFB91, Data2: 0x9E28, Data3: 0x4B86,
		Data4: [8]byte{0x90, 0xE9, 0x9E, 0x9F, 0x8A, 0x5E, 0xEA, 0x84},
	}
)

// ITaskbarList3's methods, after IUnknown's three, in the order the interface
// declares them.
const (
	taskbarInit             = 3
	taskbarSetProgressValue = 9
	taskbarSetProgressState = 10
)

const (
	// clsctxInprocServer is how CoCreateInstance is told the taskbar list lives
	// in this process, which it does: the shell's implementation is a proxy.
	clsctxInprocServer = 1
	tbpfNoProgress     = 0
	tbpfNormal         = 2
	// progressScale is the whole the bar is drawn against.
	progressScale = 10000
)

// SetProgress draws how much of the application's work is done on its taskbar
// buttons, one bar per window, and a negative fraction takes the bars away. The
// shell keeps a button for every top-level window, so each is told: a program
// with several windows reports the work of all of them, and each button shows
// the same bar.
func (application *windowsApplication) SetProgress(fraction float32) {
	list := application.taskbarList()
	if list == 0 {
		return
	}
	known := fraction >= 0 && !math.IsNaN(float64(fraction))
	if known && fraction > 1 {
		fraction = 1
	}
	state := uintptr(tbpfNoProgress)
	done := uintptr(0)
	if known {
		state = tbpfNormal
		done = uintptr(fraction * progressScale)
	}
	windowStates.Range(func(key, value any) bool {
		window := value.(*windowState)
		if window.application == application && window.window != 0 {
			comCall(list, taskbarSetProgressState, window.window, state)
			if known {
				comCall(list, taskbarSetProgressValue, window.window, done, progressScale)
			}
		}
		return true
	})
}

// taskbarList returns the application's ITaskbarList3, creating it the first
// time a bar is asked for. A shell that cannot make one leaves the program
// without bars rather than with an error: the work goes on either way, and
// nothing else in the window depends on it.
func (application *windowsApplication) taskbarList() uintptr {
	if application.taskbar != 0 || application.taskbarTried {
		return application.taskbar
	}
	application.taskbarTried = true
	if !application.oleReady {
		if status, _, _ := oleInitialize.Call(0); hresult(status) != nil {
			return 0
		}
		application.oleReady = true
	}
	var list uintptr
	status, _, _ := createCOM.Call(
		uintptr(unsafe.Pointer(&clsidTaskbarList)), 0, clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidTaskbarList3)), uintptr(unsafe.Pointer(&list)))
	if hresult(status) != nil || list == 0 {
		return 0
	}
	if status := comCall(list, taskbarInit); hresult(status) != nil {
		release(&list)
		return 0
	}
	application.taskbar = list
	return application.taskbar
}
