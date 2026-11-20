package windows

import (
	"runtime"
	"testing"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
)

func TestWindowIconUpdatesOnTheWindowThread(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err := RunApplication(func(application platform.Application) error {
		opened, err := application.OpenWindow(platform.Config{
			Title: "Viva icon test", Width: 320, Height: 200,
			OnResize: func(platform.Size) (platform.Frame, error) {
				return platform.Frame{Update: render.Update{Rebuild: true}}, nil
			},
		})
		if err != nil {
			return err
		}
		defer opened.Close()
		window := opened.(*nativeWindow)
		for _, value := range []byte{50, 200, 80} {
			icon := platform.WindowIcon{Size: 1, RGBA: []byte{value, 100, 30, 255}}
			previous := window.state.icons
			if err := window.SetIcon(icon); err != nil {
				t.Fatal(err)
			}
			if window.state.icons != previous {
				t.Fatal("the caller changed icons outside the window loop")
			}
			pumpWindowMessages()
			if window.state.icons[0] == 0 || window.state.icons[1] == 0 || window.state.icons[1] == previous[1] {
				t.Fatalf("native icon handles did not change: %v -> %v", previous, window.state.icons)
			}
			for kind, handle := range window.state.icons {
				got, _, _ := sendMessage.Call(window.state.window, 0x007f /* WM_GETICON */, uintptr(kind), 0)
				if got != handle {
					t.Fatalf("Windows icon %d = %d, want %d", kind, got, handle)
				}
			}
			if previous[1] != 0 {
				if copied, _, _ := user.NewProc("CopyIcon").Call(previous[1]); copied != 0 {
					destroyIcon.Call(copied)
					t.Fatal("replacing the window icon leaked the previous handle")
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
