//go:build windows || linux

package app

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
)

func TestNativeProgrammaticWindowSize(t *testing.T) {
	if os.Getenv("VIVA_NATIVE_WINDOW_SIZE_TEST") != "1" {
		t.Skip("requires a native desktop session")
	}
	var canvas gfx.Canvas
	var got gfx.Size
	err := RunApplication(func(application *Application) error {
		window, err := application.OpenWindow(Config{
			Title: "Viva content resize test", Width: 320, Height: 240,
			Paint: func(*gfx.Canvas, gfx.Size) {},
			OnResize: func(size gfx.Size) {
				if size.Width >= 419 && size.Width <= 421 && size.Height >= 299 && size.Height <= 301 {
					got = size
					application.Quit()
				}
			},
		}, &canvas)
		if err != nil {
			return err
		}
		if err := window.SetSize(420, 300); err != nil {
			application.Quit()
			return fmt.Errorf("request size: %w", err)
		}
		timer := time.AfterFunc(8*time.Second, application.Quit)
		t.Cleanup(func() { timer.Stop() })
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Width == 0 {
		t.Fatal("native resize never reached OnResize with the requested content size")
	}
}
