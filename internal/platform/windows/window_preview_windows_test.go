package windows

import (
	"runtime"
	"testing"
	"unsafe"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

func TestDragPreviewWindowIsClickThroughTopmostAndMovable(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	getWindowLong := user.NewProc("GetWindowLongPtrW")
	getWindowRect := user.NewProc("GetWindowRect")
	sendMessage := user.NewProc("SendMessageW")
	err := RunApplication(func(application platform.Application) error {
		if _, ok := application.(interface{ SupportsDragPreview() }); !ok {
			t.Fatal("the Windows adapter did not report drag preview support")
		}
		opened, err := application.OpenWindow(platform.Config{
			Title: "Viva drag preview test", Width: 120, Height: 40, WindowMode: platform.WindowPreview,
		})
		if err != nil {
			if err == errTransparentPreviewUnavailable {
				t.Skip("this desktop has no compositor for a transparent preview")
			}
			return err
		}
		defer opened.Close()
		state := opened.(*nativeWindow).state
		window := state.window
		style, _, _ := getWindowLong.Call(window, ^uintptr(19)) // GWL_EXSTYLE
		for _, required := range [...]struct {
			name  string
			value uintptr
		}{
			{"topmost", wsExTopmost},
			{"tool window", wsExToolWindow},
			{"no activate", wsExNoActivate},
			{"click through", wsExTransparent},
			{"composited", wsExNoRedirectionBitmap},
		} {
			if style&required.value == 0 {
				t.Errorf("preview style %#x has no %s bit", style, required.name)
			}
		}
		if hit, _, _ := sendMessage.Call(window, wmNCHitTest, 0, 0); hit != htTransparent {
			t.Fatalf("preview hit test = %#x, want HTTRANSPARENT", hit)
		}

		var before, after rect
		if ok, _, _ := getWindowRect.Call(window, uintptr(unsafe.Pointer(&before))); ok == 0 {
			t.Fatal("cannot read the preview frame")
		}
		opened.(platform.WindowMover).MoveTo(gfx.Point{X: 240, Y: 160})
		if ok, _, _ := getWindowRect.Call(window, uintptr(unsafe.Pointer(&after))); ok == 0 {
			t.Fatal("cannot read the moved preview frame")
		}
		if after.Left != int32(240*state.scale) || after.Top != int32(160*state.scale) ||
			after.Right-after.Left != before.Right-before.Left ||
			after.Bottom-after.Top != before.Bottom-before.Top {
			t.Fatalf("preview frame moved from %+v to %+v", before, after)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
