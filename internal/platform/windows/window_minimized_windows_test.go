package windows

import (
	"fmt"
	"runtime"
	"testing"
	"time"
	"unsafe"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

// swMinimize is SW_MINIMIZE: minimize the window and activate the next one.
const swMinimize = 6

// A window being minimized must not report the small iconic rectangle as its
// content frame. WM_MOVE can run before WM_SIZE sets state.minimized, so a
// frame handler that trusts the flag alone records the off-screen sentinel
// position as where the window is; the window manager already knows it is
// iconic.
func TestMinimizeDoesNotReportTheIconicFrame(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var frames []gfx.Rect
	minimized := false
	err := RunApplication(func(application platform.Application) error {
		opened, err := application.OpenWindow(platform.Config{
			Title: "Viva minimized frame test", Width: 640, Height: 400,
			ClearColor: platform.Color{Alpha: 1},
			OnFrameChange: func(frame gfx.Rect) {
				frames = append(frames, frame)
			},
			OnMinimize: func(value bool) {
				if value {
					minimized = true
				}
			},
		})
		if err != nil {
			return err
		}
		defer opened.Close()
		state := opened.(*nativeWindow).state
		showWindow.Call(state.window, swMinimize)
		deadline := time.Now().Add(2 * time.Second)
		for !minimized {
			if time.Now().After(deadline) {
				return fmt.Errorf("window did not report minimization")
			}
			time.Sleep(10 * time.Millisecond)
		}
		for _, frame := range frames {
			if frame.X <= -30000 || frame.Y <= -30000 || frame.Width < 1 || frame.Height < 1 {
				return fmt.Errorf("reported the iconic frame %+v as the window's content area", frame)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

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
			iconic, _, _ := isIconic.Call(state.window)
			if iconic != 0 {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("window did not settle minimized: iconic %d", iconic)
			}
			time.Sleep(10 * time.Millisecond)
		}
		// The corner-stretched preview this guards needs a client area that is small but
		// still non-empty: presenting at it is what shrinks the swap chain. That is the
		// window manager's answer for a custom frame, not a fact about every desktop, so
		// a desktop that answers differently has nothing for this test to check.
		var client rect
		for {
			getClientRect.Call(state.window, uintptr(unsafe.Pointer(&client)))
			if client.Right <= 0 || client.Bottom <= 0 {
				t.Skipf("this desktop reports an empty client area for a minimized window (%dx%d), so no frame can be presented at it", client.Right, client.Bottom)
			}
			if client.Right < 640 && client.Bottom < 400 {
				break
			}
			if time.Now().After(deadline) {
				t.Skipf("this desktop keeps a minimized window's client area at its visible size (%dx%d)", client.Right, client.Bottom)
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
