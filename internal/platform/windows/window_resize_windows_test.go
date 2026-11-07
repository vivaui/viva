package windows

import (
	"runtime"
	"testing"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
)

// A live resize reaches the window as a stream of WM_SIZE messages. The swap
// chain is created with DXGI_SCALING_NONE, so a frame that has not been drawn
// for the new size is shown at its own size instead of stretched over the new
// client area. The frame for a new size must also reach the swap chain before
// WM_SIZE returns, or the desktop has only the previous one to show.
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
			t.Fatalf("the swap chain is %dx%d after WM_SIZE, want 880x520: the previous frame would be shown at the old size over the new client area", state.renderer.width, state.renderer.height)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// The window's swap chain must not scale its contents: DXGI_SCALING_STRETCH is
// what let the desktop stretch the previous frame over the new client area
// during a live resize. A frame that is late then shows at its own size in the
// top-left corner instead.
func TestWindowSwapChainDoesNotScale(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err := RunApplication(func(application platform.Application) error {
		opened, err := application.OpenWindow(platform.Config{
			Title: "Viva swap chain scaling test", Width: 320, Height: 200,
			WindowMode: platform.WindowCustomTitlebar, TitlebarHeight: 32,
			ClearColor: platform.Color{Alpha: 1},
		})
		if err != nil {
			return err
		}
		defer opened.Close()
		state := opened.(*nativeWindow).state
		var description swapChainDesc1
		// IDXGISwapChain1::GetDesc1.
		if err := hresult(comCall(state.renderer.swap, 18, uintptr(unsafe.Pointer(&description)))); err != nil {
			return err
		}
		if description.Scaling != 1 { // DXGI_SCALING_NONE
			t.Fatalf("swap chain scaling = %d, want DXGI_SCALING_NONE (1)", description.Scaling)
		}
		if description.SwapEffect != 3 { // DXGI_SWAP_EFFECT_FLIP_SEQUENTIAL
			t.Fatalf("swap chain effect = %d, want DXGI_SWAP_EFFECT_FLIP_SEQUENTIAL (3)", description.SwapEffect)
		}
		if description.Buffers != 2 {
			t.Fatalf("swap chain buffers = %d, want 2", description.Buffers)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// BenchmarkResizePresent measures what one live-resize step costs on the window
// thread: apply a frame and present it at a new client size, including
// ResizeBuffers. A drag is one such step per pointer move.
func BenchmarkResizePresent(b *testing.B) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err := RunApplication(func(application platform.Application) error {
		opened, err := application.OpenWindow(platform.Config{
			Title: "Viva resize benchmark", Width: 640, Height: 400,
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
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			width := 640 + i%320
			height := 400 + i%240
			windowProc(state.window, wmSize, 0, uintptr(uint32(width)|uint32(height)<<16))
		}
		b.StopTimer()
		return nil
	})
	if err != nil {
		b.Fatal(err)
	}
}
