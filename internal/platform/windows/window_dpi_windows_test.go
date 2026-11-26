package windows

import (
	"runtime"
	"syscall"
	"testing"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

// Restoring a window can change monitors before the renderer is initialized.
// Exercise the actual message dispatch with a hidden window, without touching
// the interactive desktop's foreground window or the user's display settings.
func TestDPIChangeBeforeWindowReady(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	class, _ := syscall.UTF16PtrFromString("STATIC")
	handle, _, err := createWindow.Call(0, uintptr(unsafe.Pointer(class)), 0, wsPopup, 0, 0, 300, 200, 0, 0, 0, 0)
	if handle == 0 {
		t.Fatal(err)
	}
	defer destroyWindow.Call(handle)
	previous := active.Load()
	defer active.Store(previous)
	defer windowStates.Delete(handle)
	for _, test := range []struct {
		name         string
		before, want float32
		dpi          uint16
		fixed        bool
	}{
		{"200_to_150", 2, 1.5, 144, false},
		{"150_to_200", 1.5, 2, 192, false},
		{"explicit_override", 1.25, 1.25, 192, true},
		{"zero_dpi_ignored", 1.5, 1.5, 0, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			resizes := 0
			state := &windowState{window: handle, scale: test.before, scaleFixed: test.fixed,
				config: platform.Config{OnResize: func(platform.Size) (platform.Frame, error) {
					resizes++
					return platform.Frame{}, nil
				}},
			}
			state.configureClicks()
			windowStates.Store(handle, state)
			bounds := rect{Right: 300, Bottom: 200}
			windowProc(handle, wmDPIChanged, uintptr(test.dpi)|uintptr(test.dpi)<<16, uintptr(unsafe.Pointer(&bounds)))
			if state.scale != test.want {
				t.Fatalf("startup scale = %v, want %v", state.scale, test.want)
			}
			expectedClicks := windowState{scale: test.want}
			expectedClicks.configureClicks()
			if state.clicks.Distance != expectedClicks.clicks.Distance {
				t.Fatalf("click distance = %v, want %v for scale %v", state.clicks.Distance, expectedClicks.clicks.Distance, test.want)
			}
			if size := state.logicalSize(600, 300); size.Width != 600/test.want || size.Height != 300/test.want || size.Scale != test.want {
				t.Fatalf("first logical size = %+v at scale %v", size, test.want)
			}
			if resizes != 0 || state.ready || state.err != nil {
				t.Fatalf("startup DPI change reached the uninitialized renderer: resizes=%d ready=%v err=%v", resizes, state.ready, state.err)
			}
		})
	}
}
