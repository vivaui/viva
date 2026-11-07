package windows

import (
	"runtime"
	"testing"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
)

func TestAlwaysOnTopChangesOnlyTheWindowLevel(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err := RunApplication(func(application platform.Application) error {
		opened, err := application.OpenWindow(platform.Config{
			Title: "Viva topmost test", Width: 640, Height: 400,
			OnResize: func(platform.Size) (platform.Frame, error) {
				return platform.Frame{Update: render.Update{Rebuild: true}}, nil
			},
		})
		if err != nil {
			return err
		}
		defer opened.Close()
		window := opened.(*nativeWindow)
		getStyle := user.NewProc("GetWindowLongPtrW")
		isTopmost := func() bool {
			style, _, _ := getStyle.Call(window.state.window, ^uintptr(19)) // GWL_EXSTYLE
			return style&wsExTopmost != 0
		}
		if isTopmost() {
			t.Fatal("ordinary window starts topmost")
		}
		if err := window.SetAlwaysOnTop(true); err != nil || !isTopmost() {
			t.Fatalf("enable topmost: %v, style=%v", err, isTopmost())
		}
		if err := window.SetAlwaysOnTop(false); err != nil || isTopmost() {
			t.Fatalf("disable topmost: %v, style=%v", err, isTopmost())
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
