package linux

import (
	"slices"
	"testing"
	"unsafe"
)

func TestConfigureAnswersSyncAfterScaleRefresh(t *testing.T) {
	for _, failScaleFrame := range []bool{false, true} {
		name := "presented"
		if failScaleFrame {
			name = "presentation_failed"
		}
		t.Run(name, func(t *testing.T) {
			var order []string
			recreations := 0
			renderer := newSuboptimalTestRenderer(vkSuccess, vkSuccess, &recreations)
			renderer.physical, renderer.surface = 1, 2
			renderer.width, renderer.height = 100, 100
			baseQuery := renderer.api.getSurfaceCapabilities
			renderer.api.getSurfaceCapabilities = func(device, surface uintptr, caps *vkSurfaceCapabilities) int32 {
				status := baseQuery(device, surface, caps)
				caps.Current.Width = 120
				return status
			}
			renderer.api.queuePresent = func(uintptr, *vkPresentInfo) int32 {
				order = append(order, "present")
				if failScaleFrame && renderer.pixelScale() == 2 {
					return vkErrorDeviceLost
				}
				return vkSuccess
			}
			win := &window{
				id: 1, width: 100, height: 100,
				api: &xcbAPI{
					configureWindow: func(uintptr, uint32, uint16, *uint32) xcbVoidCookie { return xcbVoidCookie{} },
					flush:           func(uintptr) int32 { return 1 },
				},
				tracker: &scaleTracker{
					origin:   scaleFromRandR,
					monitors: make([]monitorGeometry, 2),
					// Refreshing a previously multi-monitor layout now reports a
					// 192-DPI screen, so this configure must present again at 2x.
					screen: &xcbScreen{WidthPixels: 3840, HeightPixels: 2160, WidthMillimeters: 508, HeightMillimeters: 286},
				},
				syncCounter: &syncCounter{id: 7, set: func(_ uintptr, _ uint32, value uint64) xcbVoidCookie {
					if value != syncInt64(101, 0) {
						t.Fatalf("sync value = %d, want 101", value)
					}
					order = append(order, "sync")
					return xcbVoidCookie{}
				}},
			}
			win.syncPending.request(101, 0)
			state := &linuxWindowState{win: win, renderer: renderer}
			event := xcbConfigureEvent{ResponseType: xcbConfigureNotify | 0x80, Window: 1, X: 10, Width: 120, Height: 100}
			closed, err := state.handleEvent(unsafe.Pointer(&event))
			if closed || (err != nil) != failScaleFrame {
				t.Fatalf("configure: closed=%v err=%v, want failure=%v", closed, err, failScaleFrame)
			}
			if win.displayScale() != 2 || renderer.pixelScale() != 2 {
				t.Fatalf("scale refresh did not reach 2x: window=%v renderer=%v", win.displayScale(), renderer.pixelScale())
			}
			want := []string{"present", "present", "sync"}
			if failScaleFrame {
				want = want[:2]
			}
			if !slices.Equal(order, want) {
				t.Fatalf("configure order = %v, want %v", order, want)
			}
		})
	}
}
