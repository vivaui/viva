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

func TestStatusHoverNativePanel(t *testing.T) {
	if os.Getenv("VIVA_TEST_NOTIFICATION_DESKTOP") != "1" {
		t.Skip("requires a private Windows desktop")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	thread, _, _ := kernel.NewProc("GetCurrentThreadId").Call()
	previous, _, _ := user.NewProc("GetThreadDesktop").Call(thread)
	name, _ := syscall.UTF16PtrFromString("VivaStatusHover-" + strconv.Itoa(os.Getpid()))
	desktop, _, err := user.NewProc("CreateDesktopW").Call(uintptr(unsafe.Pointer(name)), 0, 0, 0, 0x01ff, 0)
	if desktop == 0 {
		t.Fatal(err)
	}
	defer user.NewProc("CloseDesktop").Call(desktop)
	if ok, _, err := user.NewProc("SetThreadDesktop").Call(desktop); ok == 0 {
		t.Fatal(err)
	}
	defer user.NewProc("SetThreadDesktop").Call(previous)

	err = RunApplication(func(a platform.Application) error {
		app := a.(*windowsApplication)
		owner, err := app.OpenWindow(platform.Config{Title: "Typing", Width: 200, Height: 100})
		if err != nil {
			return err
		}
		defer owner.Close()
		if err := app.ensureStatusWindow(); err != nil {
			return err
		}
		state := app.status
		handle := owner.(*nativeWindow).state.window
		getFocus := user.NewProc("GetFocus")
		setFocus.Call(handle)
		item := &statusItem{id: 2, hoverPanel: func(anchor platform.ScreenRect) platform.Window {
			panel, err := app.OpenWindow(platform.Config{Width: 140, Height: 60, WindowMode: platform.WindowNotification, HasAnchor: true, Anchor: anchor})
			if err != nil {
				t.Error(err)
				return nil
			}
			return panel
		}}
		state.items[item.id] = item
		defer delete(state.items, item.id)
		state.openHover(item, platform.ScreenRect{X: 50, Y: 100, Width: 16, Height: 16})
		panel := state.hover.panel
		if panel == nil {
			t.Fatal("no native panel")
		}
		if focus, _, _ := getFocus.Call(); focus != handle {
			t.Error("opening a hover panel stole keyboard focus")
		}
		state.notified(state.window, 0, uintptr(item.id)<<16|ninPopupClose)
		if state.hover.gapUntil == 0 {
			t.Error("shell close missed the grace period")
		}
		windowProc(panel.state.window, wmMouseMove, 0, 0)
		if state.hover.gapUntil != 0 {
			t.Error("entering the panel did not cancel its close")
		}
		windowProc(panel.state.window, wmMouseLeave, 0, 0)
		if state.hover.gapUntil == 0 {
			t.Error("leaving the panel did not schedule a close")
		}
		state.stopItemHover(item.id)
		if state.hover.panel != nil || panel.state.hoverStatus != nil {
			t.Error("panel still attached after disabling hover")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
