//go:build linux && !android

package linux

import (
	"slices"
	"testing"
	"unsafe"
)

// A configure that changes the display scale presents again at the new scale, and the
// window answers the window manager's sync request only after that frame, and only if it
// was presented.
func TestConfigureAnswersSyncAfterScaleRefresh(t *testing.T) {
	for _, failScaleFrame := range []bool{false, true} {
		name := "presented"
		if failScaleFrame {
			name = "presentation_failed"
		}
		t.Run(name, func(t *testing.T) {
			var order []string
			renderer := &fakeWindowRenderer{deviceWidth: 100, deviceHeight: 100, scale: 1, failAt2x: failScaleFrame, shared: &order}
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
			if win.displayScale() != 2 || renderer.scale != 2 {
				t.Fatalf("scale refresh did not reach 2x: window=%v renderer=%v", win.displayScale(), renderer.scale)
			}
			want := []string{"present", "present", "sync"}
			if failScaleFrame {
				want = want[:2]
			}
			// Only the frames and the answer to the window manager are in question.
			order = slices.DeleteFunc(order, func(call string) bool { return call != "present" && call != "sync" })
			if !slices.Equal(order, want) {
				t.Fatalf("configure order = %v, want %v", order, want)
			}
		})
	}
}
