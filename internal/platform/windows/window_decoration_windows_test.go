package windows

import (
	"fmt"
	"runtime"
	"testing"
	"time"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

func TestNativeWindowDecorationRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		width, height int
		above         bool
	}{{640, 400, false}, {1280, 800, true}} {
		t.Run(fmt.Sprintf("%dx%d", tc.width, tc.height), func(t *testing.T) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			var result error
			completed := false
			err := RunApplication(func(application platform.Application) error {
				control, err := application.OpenWindow(platform.Config{Title: "Viva decoration probe", Width: tc.width, Height: tc.height})
				if err != nil {
					return err
				}
				window := control.(*nativeWindow)
				hwnd := window.state.window
				setFocus.Call(hwnd)
				focusProc := user.NewProc("GetFocus")
				focus, _, _ := focusProc.Call()
				if focus != hwnd {
					return fmt.Errorf("test window did not receive keyboard focus")
				}
				if err := window.SetAlwaysOnTop(tc.above); err != nil {
					return err
				}
				frame, available := window.Frame()
				var outer rect
				if ok, _, _ := getWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&outer))); ok == 0 || !available {
					return fmt.Errorf("could not read initial geometry")
				}
				before, _, _ := fullscreenGetStyle.Call(hwnd, ^uintptr(15))
				finish := func(err error) {
					if !completed {
						completed, result = true, err
						window.Close()
					}
				}
				guard := time.AfterFunc(5*time.Second, func() { application.Post(func() { finish(fmt.Errorf("decoration callback did not run")) }) })
				window.state.config.OnClosed = func() { guard.Stop() }
				verify := func(decorated bool) error {
					style, _, _ := fullscreenGetStyle.Call(hwnd, ^uintptr(15))
					ex, _, _ := fullscreenGetStyle.Call(hwnd, ^uintptr(19))
					current, available := window.Frame()
					focused, _, _ := focusProc.Call()
					if window.state.window != hwnd || !available || current != frame || focused != focus || (ex&wsExTopmost != 0) != tc.above {
						return fmt.Errorf("geometry, focus, window identity or level changed: frame %v, want %v; focus %#x, want %#x; exstyle %#x", current, frame, focused, focus, ex)
					}
					if decorated && uint32(style) != uint32(before) || !decorated && style&wsOverlappedWindow != 0 {
						return fmt.Errorf("unexpected window style %#x, original %#x, decorated=%t", style, before, decorated)
					}
					var now rect
					if ok, _, _ := getWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&now))); ok == 0 || (now == outer) != decorated {
						return fmt.Errorf("outer rectangle %v, original %v, decorated=%t", now, outer, decorated)
					}
					return nil
				}
				if err := window.SetDecorated(false, func(err error) {
					if err == nil {
						err = verify(false)
					}
					if err != nil {
						finish(err)
						return
					}
					if err := window.SetDecorated(true, func(err error) {
						if err == nil {
							err = verify(true)
						}
						finish(err)
					}); err != nil {
						finish(err)
					}
				}); err != nil {
					return err
				}
				if err := window.SetDecorated(false, func(error) { finish(fmt.Errorf("rejected duplicate completed")) }); err == nil {
					return fmt.Errorf("accepted a duplicate request")
				}
				return nil
			})
			if err != nil || result != nil || !completed {
				t.Fatalf("Windows decoration round trip: completed=%t, loop=%v, request=%v", completed, err, result)
			}
		})
	}
}

func TestNativeWindowDecorationCompletesWhenClosing(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var outcomes []error
	err := RunApplication(func(application platform.Application) error {
		control, err := application.OpenWindow(platform.Config{Title: "Viva decoration close probe", Width: 320, Height: 240})
		if err != nil {
			return err
		}
		window := control.(*nativeWindow)
		if err := window.SetDecorated(false, func(err error) { outcomes = append(outcomes, err) }); err != nil {
			return err
		}
		destroyWindow.Call(window.state.window)
		return nil
	})
	if err != nil || len(outcomes) != 1 || outcomes[0] == nil {
		t.Fatalf("close before dispatch: loop=%v, completions=%v", err, outcomes)
	}
}
