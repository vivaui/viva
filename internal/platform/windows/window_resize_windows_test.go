package windows

import (
	"runtime"
	"testing"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
)

// A live resize reaches the window as a stream of WM_SIZE messages. The desktop
// keeps showing the previous swap-chain frame and scales it across the client
// area it now covers until the window presents one drawn at the new size. A drag
// therefore shows the whole interface stretched (the side bar included) and then
// snapping back when the paint finally arrives.
//
// The frame for a new size must reach the swap chain before WM_SIZE returns, so
// that the next composition has nothing to stretch.
func TestResizePresentsTheNewSizeBeforeReturning(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err := RunApplication(func(application platform.Application) error {
		opened, err := application.OpenWindow(platform.Config{
			Title: "Viva resize present test", Width: 640, Height: 400,
			WindowMode: platform.WindowCustomTitlebar, TitlebarHeight: 32,
			ClearColor: platform.Color{Alpha: 1},
			OnResize: func(platform.Size) (platform.Frame, error) {
				return platform.Frame{Update: render.Update{Rebuild: true}}, nil
			},
		})
		if err != nil {
			return err
		}
		defer opened.Close()
		state := opened.(*nativeWindow).state
		if err := state.renderer.present(640, 400, 1); err != nil {
			return err
		}
		// What the window manager sends while the user drags the edge.
		windowProc(state.window, wmSize, 0, uintptr(uint32(880)|uint32(520)<<16))
		if state.renderer.width != 880 || state.renderer.height != 520 {
			t.Fatalf("the swap chain is %dx%d after WM_SIZE, want 880x520: the previous frame would be stretched over the new client area", state.renderer.width, state.renderer.height)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
