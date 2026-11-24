//go:build linux && !android && vivaxembed

package linux

import (
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vivaui/viva/internal/platform"
)

// Run on a private, quiet X11 server with the named tray host. Count returns
// from the actual blocking descriptor wait, not just application redraws.
func TestStatusItemIdleWait(t *testing.T) {
	mode := os.Getenv("VIVA_TEST_IDLE_TRAY")
	if mode == "" {
		t.Skip("requires an isolated X11 tray host")
	}
	var wakes atomic.Int64
	var original func(*pollFD, uintptr, int32) int32
	var api *xcbAPI
	defer func() {
		if api != nil {
			api.poll = original
		}
	}()
	err := RunApplication(func(host platform.Application) error {
		a := host.(*linuxApplication)
		window, err := a.OpenWindow(platform.Config{Width: 64, Height: 64})
		if err != nil {
			return err
		}
		api = window.(*nativeWindow).state.win.api
		original = api.poll
		api.poll = func(fds *pollFD, count uintptr, timeout int32) int32 {
			result := original(fds, count, timeout)
			if result >= 0 {
				wakes.Add(1)
			}
			return result
		}
		config := platform.StatusItemConfig{Icon: platform.StatusIcon{Size: 1, RGBA: []byte{255, 255, 255, 255}}}
		switch mode {
		case "xembed":
			config.LinuxTrayBackend = platform.LinuxTrayXEmbed
		case "sni":
		default:
			return fmt.Errorf("unknown tray host %q", mode)
		}
		item, err := a.AddStatusItem(config)
		if err != nil {
			return err
		}
		if supported := item.SetHoverPanel(func(platform.ScreenRect) platform.Window { return nil }); supported != (mode == "xembed") {
			return fmt.Errorf("%s panel support = %v", mode, supported)
		}
		if (a.trayGTK != nil) != (mode == "xembed") {
			return fmt.Errorf("unexpected GTK tray loop on %s", mode)
		}
		go func() {
			time.Sleep(2 * time.Second) // allow map, registration and settings to settle
			start := wakes.Load()
			time.Sleep(5 * time.Second)
			if n := wakes.Load() - start; n != 0 {
				t.Errorf("%s: %d idle poll returns in 5s", mode, n)
			}
			item.Remove()
			a.Post(func() {
				if a.trayGTK != nil {
					t.Error("last item retained the embedded event source")
				}
				a.Quit()
			})
		}()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
