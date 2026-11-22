package windows

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
)

// A window asked to open maximized is the zoomed window Win32 would have shown,
// and the application is told the state it opened in, as it is when maximizing
// later.
func TestOpeningMaximizedShowsAZoomedWindow(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var maximizes []bool
	err := RunApplication(func(application platform.Application) error {
		opened, err := application.OpenWindow(platform.Config{
			Title: "Viva maximized test", Width: 640, Height: 400, Maximized: true,
			OnResize: func(platform.Size) (platform.Frame, error) {
				return platform.Frame{Update: render.Update{Rebuild: true}}, nil
			},
			OnMaximize: func(maximized bool) { maximizes = append(maximizes, maximized) },
		})
		if err != nil {
			return err
		}
		defer opened.Close()
		window := opened.(*nativeWindow)
		zoomed, _, _ := isZoomed.Call(window.state.window)
		if zoomed == 0 {
			return fmt.Errorf("IsZoomed = 0, want the window shown maximized")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(maximizes) != 1 || !maximizes[0] {
		t.Fatalf("maximize reports = %v, want [true]", maximizes)
	}
}
