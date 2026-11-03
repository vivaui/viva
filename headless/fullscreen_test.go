//go:build darwin || windows || linux

package headless

import (
	"strings"
	"sync"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

func TestApplicationFullscreenRequestRunsOnDriverAndRestores(t *testing.T) {
	host := newApplicationSession(Options{ReferenceText: true})
	var window *app.Window
	var events []string
	app.UseApplicationRunner(func(start func(app.Host) error) error { return start(host) })
	defer app.UseApplicationRunner(nil)
	err := app.RunApplication(func(application *app.Application) error {
		var err error
		window, err = application.OpenWindow(app.Config{Title: "fullscreen", Width: 500, Height: 300,
			OnFullscreen: func(fullscreen bool) {
				if window.Fullscreen() != fullscreen {
					t.Fatal("host state was not published before callback")
				}
				events = append(events, "fullscreen")
			},
			OnResize: func(ui.Size) {
				if window != nil {
					events = append(events, "resize")
				}
			},
		}, nil)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	defer host.closeAll()
	entry := host.windows[1]
	original := entry.session.driver.Frame()
	if err := entry.session.driver.SetFullscreenMonitor(ui.Rect{Width: 1600, Height: 900}); err != nil {
		t.Fatal(err)
	}
	var requests sync.WaitGroup
	for range 8 {
		requests.Go(func() {
			if err := window.SetFullscreen(true); err != nil {
				t.Error(err)
			}
			_ = window.Fullscreen()
		})
	}
	requests.Wait()
	if window.Fullscreen() || len(events) != 0 {
		t.Fatal("background request ran application callbacks off the driver thread")
	}
	if err := entry.session.driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if !window.Fullscreen() || entry.session.driver.Frame().Width != 1600 ||
		len(events) != 2 || events[0] != "fullscreen" || events[1] != "resize" {
		t.Fatalf("entry state/events: %v, %v", window.Fullscreen(), events)
	}
	monitor := ui.Rect{X: -1600, Width: 1600, Height: 900}
	if err := entry.session.driver.SetFullscreenMonitor(monitor); err != nil {
		t.Fatal(err)
	}
	if frame, ok := window.Frame(); !ok || frame != monitor {
		t.Fatal("same-size monitor change left the host Frame stale")
	}
	if err := window.SetFullscreen(false); err != nil {
		t.Fatal(err)
	}
	if err := entry.session.driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if window.Fullscreen() || entry.session.driver.Frame() != original {
		t.Fatal("host did not restore normal placement")
	}
	done := make(chan struct{})
	var readers sync.WaitGroup
	readers.Go(func() {
		for {
			select {
			case <-done:
				return
			default:
				frame, ok := window.Frame()
				if !ok || frame.Width < 1 || frame.Height < 1 {
					t.Error("concurrent Frame returned invalid live geometry")
					return
				}
			}
		}
	})
	for index := range 20 {
		if err := window.SetFullscreen(index%2 == 0); err != nil {
			t.Fatal(err)
		}
		if err := entry.session.driver.Pump(); err != nil {
			t.Fatal(err)
		}
	}
	close(done)
	readers.Wait()
}

func TestFullscreenProtocolUsesMonitorBoundsAndRestores(t *testing.T) {
	session, err := Open(app.Config{Title: "fullscreen", Width: 500, Height: 300}, nil, Options{ReferenceText: true})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	original := session.driver.Frame()
	for _, command := range []string{"fullscreenmonitor -1600 0 1600 900", "fullscreen on"} {
		reply, _ := execute(session, command)
		if !strings.HasPrefix(reply, "ok") {
			t.Fatalf("%s: %s", command, reply)
		}
	}
	if !session.driver.Fullscreen() || session.driver.Frame() != (ui.Rect{X: -1600, Width: 1600, Height: 900}) {
		t.Fatal("protocol did not enter full monitor bounds")
	}
	reply, _ := execute(session, "fullscreen off")
	if !strings.HasPrefix(reply, "ok") || session.driver.Frame() != original {
		t.Fatalf("restore: %s, %+v", reply, session.driver.Frame())
	}
	reply, _ = execute(session, "fullscreen invalid")
	if !strings.HasPrefix(reply, "error") {
		t.Fatalf("invalid fullscreen command: %s", reply)
	}
}
