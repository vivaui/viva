//go:build darwin || windows || linux

package headless

import (
	"sync"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

func TestApplicationSizeRequestRunsOnDriver(t *testing.T) {
	host := newApplicationSession(Options{ReferenceText: true})
	var window *app.Window
	var sizes []gfx.Size
	app.UseApplicationRunner(func(start func(app.Host) error) error { return start(host) })
	defer app.UseApplicationRunner(nil)
	err := app.RunApplication(func(application *app.Application) error {
		var err error
		window, err = application.OpenWindow(app.Config{Title: "size", Width: 500, Height: 300,
			OnResize: func(size gfx.Size) { sizes = append(sizes, size) },
		}, nil)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	defer host.closeAll()
	entry := host.windows[1]
	var requests sync.WaitGroup
	for range 8 {
		requests.Go(func() {
			if err := window.SetSize(420, 240); err != nil {
				t.Error(err)
			}
		})
	}
	requests.Wait()
	if len(sizes) != 0 {
		t.Fatal("background request ran OnResize off the driver thread")
	}
	if err := entry.session.driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if len(sizes) == 0 || sizes[len(sizes)-1].Width != 420 || sizes[len(sizes)-1].Height != 240 {
		t.Fatalf("OnResize = %v, want 420x240", sizes)
	}
	if frame, ok := window.Frame(); !ok || frame.Width != 420 || frame.Height != 240 {
		t.Fatalf("Frame = %v, %v", frame, ok)
	}
}
