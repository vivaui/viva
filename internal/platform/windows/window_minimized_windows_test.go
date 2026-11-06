package windows

import (
	"runtime"
	"testing"
	"time"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

// swMinimize is SW_MINIMIZE: minimize the window and activate the next one.
const swMinimize = 6

// A paint caused by minimizing must not present at the minimized client size. A
// window with a custom frame reports the small minimized window itself (160x28),
// not an empty client area, so presenting would resize the swap chain to the
// corner of the visible frame; the taskbar preview of the window then shows that
// corner stretched. The window keeps the swap chain it had while visible until
// the restore's WM_SIZE paints it again.
func TestMinimizedPaintKeepsTheVisibleSwapChain(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err := RunApplication(func(application platform.Application) error {
		opened, err := application.OpenWindow(platform.Config{
			Title: "Viva minimized paint test", Width: 640, Height: 400,
			WindowMode: platform.WindowCustomTitlebar, TitlebarHeight: 32,
			ClearColor: platform.Color{Alpha: 1},
		})
		if err != nil {
			return err
		}
		defer opened.Close()
		state := opened.(*nativeWindow).state
		if err := state.renderer.present(640, 400, 1); err != nil {
			return err
		}
		showWindow.Call(state.window, swMinimize)
		defer showWindow.Call(state.window, swRestore)
		deadline := time.Now().Add(2 * time.Second)
		for {
			var client rect
			getClientRect.Call(state.window, uintptr(unsafe.Pointer(&client)))
			iconic, _, _ := isIconic.Call(state.window)
			if iconic != 0 && client.Right > 0 && client.Bottom > 0 && client.Right < 640 && client.Bottom < 400 {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("window did not settle minimized: client %dx%d, iconic %d", client.Right, client.Bottom, iconic)
			}
			time.Sleep(10 * time.Millisecond)
		}
		// A paint the resize caused can arrive before the WM_SIZE that sets the flag.
		// The window is already iconic; the paint must still keep the visible frame.
		state.minimized = false
		windowProc(state.window, wmPaint, 0, 0)
		if state.renderer.width != 640 || state.renderer.height != 400 {
			t.Fatalf("minimized paint resized the swap chain to %dx%d", state.renderer.width, state.renderer.height)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
