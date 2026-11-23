//go:build !android

package linux

import (
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vivaui/viva/internal/platform"
)

// A round trip made after opening a tray window can consume the socket bytes
// into libxcb's private queue. No mouse or keyboard event should be needed to
// dispatch the redraw already waiting in that queue.
func TestTrayWindowQueuedRedrawBeforeInput(t *testing.T) {
	if os.Getenv("VIVA_TEST_NOTIFICATION_XVFB") != "1" {
		t.Skip("requires a private Xvfb server")
	}
	t.Setenv("VIVA_SOFTWARE_RENDERING", "1")
	var redraws atomic.Int32
	var observed atomic.Int32
	err := RunApplication(func(app platform.Application) error {
		owner, err := app.OpenWindow(platform.Config{Width: 80, Height: 60})
		if err != nil {
			return err
		}
		owner.Hide()
		app.Post(func() {
			panel, err := app.OpenWindow(platform.Config{Width: 180, Height: 66, WindowMode: platform.WindowNotification,
				Waker:    &platform.Waker{},
				OnRedraw: func() (platform.Frame, error) { redraws.Add(1); return platform.Frame{}, nil },
			})
			if err != nil {
				t.Error(err)
				app.Quit()
				return
			}
			win := panel.(*nativeWindow).state.win
			win.postRedraw()
			if _, err := win.api.atom(win.connection, "_VIVA_TEST_AFTER_TRAY_OPEN"); err != nil {
				t.Error(err)
			}
			time.AfterFunc(time.Second, func() { observed.Store(redraws.Load()); app.Quit() })
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if observed.Load() == 0 {
		t.Fatal("tray redraw stayed buffered until another input event")
	}
}
