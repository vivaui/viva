package windows

import (
	"os"
	"runtime"
	"strconv"
	"syscall"
	"testing"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

// Window focus belongs to a desktop. The test thread creates its own hidden
// one so an interactive user's editor is never raised, hidden or unfocused.
func TestNotificationNativeWindows(t *testing.T) {
	if os.Getenv("VIVA_TEST_NOTIFICATION_DESKTOP") != "1" {
		t.Skip("requires private Win32 desktop creation rights")
	}
	runtime.LockOSThread()
	threadID, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetCurrentThreadId").Call()
	previous, _, _ := user.NewProc("GetThreadDesktop").Call(threadID)
	desktopName, _ := syscall.UTF16PtrFromString("VivaNotificationTest-" + strconv.Itoa(os.Getpid()))
	desktop, _, err := user.NewProc("CreateDesktopW").Call(uintptr(unsafe.Pointer(desktopName)), 0, 0, 0, 0x01ff, 0)
	if desktop == 0 {
		runtime.UnlockOSThread()
		t.Fatal(err)
	}
	// Return the original desktop before releasing the thread. If a system
	// library retains a hidden window, the runtime discards this locked thread.
	defer func() {
		if ok, _, _ := user.NewProc("SetThreadDesktop").Call(previous); ok != 0 {
			runtime.UnlockOSThread()
		}
		user.NewProc("CloseDesktop").Call(desktop)
	}()
	if ok, _, err := user.NewProc("SetThreadDesktop").Call(desktop); ok == 0 {
		t.Fatal(err)
	}
	send := user.NewProc("SendMessageW")
	focus := user.NewProc("GetFocus")
	err = RunApplication(func(app platform.Application) error {
		owner, err := app.OpenWindow(platform.Config{Title: "Typing", Width: 200, Height: 100})
		if err != nil {
			return err
		}
		defer owner.Close()
		handle := owner.(*nativeWindow).state.window
		setFocus.Call(handle)
		if before, _, _ := focus.Call(); before != handle {
			t.Fatalf("expected original keyboard focus %x, got %x", handle, before)
		}
		clicks := 0
		popup, err := app.OpenWindow(platform.Config{Title: "Incoming", Width: 336, Height: 136, WindowMode: platform.WindowNotification, OnPointer: func(e platform.PointerEvent) (platform.Frame, error) {
			if e.Kind == platform.PointerDown {
				clicks++
			}
			return platform.Frame{}, nil
		}})
		if err != nil {
			return err
		}
		defer popup.Close()
		hwnd := popup.(*nativeWindow).state.window
		if after, _, _ := focus.Call(); after != handle {
			t.Fatal("opening the popup stole focus")
		}
		if activate, _, _ := send.Call(hwnd, 0x0021, handle, 0); activate != 3 {
			t.Fatalf("mouse activation response %d", activate)
		}
		send.Call(hwnd, wmLeftDown, mkLButton, 10<<16|10)
		send.Call(hwnd, wmLeftUp, 0, 10<<16|10)
		if clicks != 1 {
			t.Fatalf("notification received %d click(s)", clicks)
		}
		if after, _, _ := focus.Call(); after != handle {
			t.Fatal("interacting with notification stole focus")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
