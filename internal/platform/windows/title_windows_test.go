//go:build windows

package windows

import (
	"runtime"
	"syscall"
	"testing"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
)

// windowsMessage is the MSG structure the message loop reads.
type windowsMessage struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	X, Y    int32
}

// pumpWindowMessages dispatches what is queued for this thread, so the title the window
// thread was asked for is applied before the test reads it.
func pumpWindowMessages() {
	peek := user.NewProc("PeekMessageW")
	const removeMessage = 1
	var message windowsMessage
	for {
		if ok, _, _ := peek.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0, removeMessage); ok == 0 {
			return
		}
		translateMessage.Call(uintptr(unsafe.Pointer(&message)))
		dispatchMessage.Call(uintptr(unsafe.Pointer(&message)))
	}
}

func TestSetTitleReachesTheWindowText(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err := RunApplication(func(application platform.Application) error {
		opened, err := application.OpenWindow(platform.Config{
			Title: "Viva title test", Width: 640, Height: 400,
			OnResize: func(platform.Size) (platform.Frame, error) {
				return platform.Frame{Update: render.Update{Rebuild: true}}, nil
			},
		})
		if err != nil {
			return err
		}
		defer opened.Close()
		window := opened.(*nativeWindow)
		getText := user.NewProc("GetWindowTextW")
		titleOf := func() string {
			buffer := make([]uint16, 256)
			length, _, _ := getText.Call(window.state.window, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
			return syscall.UTF16ToString(buffer[:length])
		}
		if got := titleOf(); got != "Viva title test" {
			t.Fatalf("opening title = %q", got)
		}
		const want = "Viva – 标题"
		if err := window.SetTitle(want); err != nil {
			t.Fatal(err)
		}
		pumpWindowMessages()
		if got := titleOf(); got != want {
			t.Fatalf("title after SetTitle = %q, want %q", got, want)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
