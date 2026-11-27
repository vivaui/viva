package windows

import (
	"runtime"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

// This test needs a Windows desktop. It also reports the bounded resize work
// requested by the native-overlay design; it is not a steady-frame benchmark.
func TestWatermarkWindowTransparencyStackingAndResizeTiming(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err := RunApplication(func(application platform.Application) error {
		opened, err := application.OpenWindow(platform.Config{Title: "Watermark native test", Width: 640, Height: 400, ClearColor: platform.Color{Alpha: 1}, Watermark: platform.Watermark{Text: "测试用户 10086", Color: platform.Color{Alpha: .1}, FontWeight: -.4, Angle: -25}})
		if err != nil {
			return err
		}
		defer opened.Close()
		state := opened.(*nativeWindow).state
		overlay := state.watermark
		if overlay == nil || overlay.target == nil {
			t.Fatal("initial watermark missing")
		}
		if watermarkProc(overlay.window, wmNCHitTest, 0, 0) != ^uintptr(0) || watermarkProc(overlay.window, 0x21, 0, 0) != 3 {
			t.Fatal("watermark intercepts pointer or activation")
		}
		// A newly created owned popup may reorder the overlay; restore the invariant.
		host := &webHost{state: state}
		cover := host.newCover()
		if cover == 0 {
			t.Fatal("create owned popup")
		}
		defer destroyWindow.Call(cover)
		showWindow.Call(cover, swShowNoActivate)
		state.raiseWatermark()
		previous, _, _ := user.NewProc("GetWindow").Call(overlay.window, 3) // GW_HWNDPREV
		if previous == cover {
			t.Fatal("owned popup is above watermark")
		}
		for _, enabled := range []bool{true, false, true, false} {
			if !state.setAlwaysOnTop(enabled) {
				t.Fatal("owner topmost change failed")
			}
			style, err := readDecorationStyle(overlay.window, ^uintptr(19))
			if err != nil || (style&wsExTopmost != 0) != enabled {
				t.Fatal("overlay did not follow owner topmost level", enabled, style, err)
			}
		}
		// Concurrent callers only enqueue requests; the window thread applies the latest one.
		var jobs sync.WaitGroup
		failures := make(chan error, 8)
		for range 8 {
			jobs.Add(1)
			go func() {
				defer jobs.Done()
				if err := opened.(*nativeWindow).SetWatermark(state.config.Watermark); err != nil {
					failures <- err
				}
			}()
		}
		jobs.Wait()
		close(failures)
		for err := range failures {
			t.Fatal(err)
		}
		state.applyWatermark()
		retained := overlay.target.scratch
		state.moveWatermark()
		if overlay.target.scratch != retained {
			t.Fatal("translation reallocated surface")
		}
		if err := overlay.render(state.config.Watermark, 1.5, 320, 200, true); err != nil {
			return err
		}
		for _, size := range []struct{ width, height int }{{1600, 960}, {2560, 1440}} {
			start := time.Now()
			if err := overlay.render(state.config.Watermark, 1.5, size.width*3/2, size.height*3/2, false); err != nil {
				return err
			}
			destination := point{}
			source := point{}
			dimensions := point{X: int32(size.width * 3 / 2), Y: int32(size.height * 3 / 2)}
			blend := struct{ Operation, Flags, Alpha, Format byte }{Alpha: 255, Format: 1}
			if ok, _, err := updateWatermarkLayer.Call(overlay.window, 0, uintptr(unsafe.Pointer(&destination)), uintptr(unsafe.Pointer(&dimensions)), overlay.target.scratch.dc, uintptr(unsafe.Pointer(&source)), 0, uintptr(unsafe.Pointer(&blend)), 2); ok == 0 {
				return err
			}
			t.Logf("resize %dx%d @150%% fill + submit: %v; retained DIB: %d bytes", size.width, size.height, time.Since(start), len(overlay.target.scratch.Pixels))
		}
		overlay.suspend()
		if overlay.target != nil || overlay.brush != 0 || overlay.bitmap != 0 {
			t.Fatal("hidden overlay retained client-sized resources")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestWatermarkSetterRejectsRetiredHandle(t *testing.T) {
	window := &nativeWindow{state: &windowState{}}
	if err := window.SetWatermark(platform.Watermark{}); err == nil {
		t.Fatal("retired native handle accepted an update")
	}
}

func TestWatermarkCompositionDoesNotRebuildTextTile(t *testing.T) {
	original := platform.Watermark{Text: "DRAFT", FontSize: 12, TileWidth: 240, TileHeight: 180}
	changed := original
	changed.Angle = -25
	changed.Insets.Left = 40
	if watermarkTileChanged(original, changed) {
		t.Fatal("composition-only update rebuilt text")
	}
	changed.FontFamily = "Arial"
	if !watermarkTileChanged(original, changed) {
		t.Fatal("font family update reused stale text")
	}
}
