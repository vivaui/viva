//go:build darwin || windows || linux

package headless

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// ui.OpenWindow opens a page's window from an application without the caller
// touching a canvas, and the page reaches its application to open another.
func TestUIOpenWindowOpensPagesAndHandsThemTheirApplication(t *testing.T) {
	host := newApplicationSession(Options{ReferenceText: true})
	app.UseApplicationRunner(func(start func(app.Host) error) error { return start(host) })
	defer app.UseApplicationRunner(nil)
	defer host.closeAll()

	var seen *app.Application
	err := app.RunApplication(func(application *app.Application) error {
		_, err := ui.OpenWindow(application, app.Config{Title: "main", Width: 300, Height: 200}, ui.PageConfig{
			View: func(c ui.C) {
				seen = ui.Application(c)
				ui.Text(c, "hello")
			},
		})
		if err != nil {
			return err
		}
		if seen != application {
			t.Errorf("C.Application = %p, want the opening application %p", seen, application)
		}
		_, err = ui.OpenWindow(seen, app.Config{Title: "second", Width: 300, Height: 200}, ui.PageConfig{View: func(c ui.C) { ui.Text(c, "two") }})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(host.windows); got != 2 {
		t.Fatalf("windows = %d, want 2", got)
	}
}

func TestUIOpenWindowReportsAFailureToOpen(t *testing.T) {
	if _, err := ui.OpenWindow(nil, app.Config{Title: "none"}, ui.PageConfig{View: func(ui.C) {}}); err == nil {
		t.Fatal("OpenWindow without a running application returned no error")
	}
}
