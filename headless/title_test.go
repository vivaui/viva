//go:build darwin || windows || linux

package headless

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
)

func TestWindowTitleIsListedAndUnusableTitlesChangeNothing(t *testing.T) {
	host := newApplicationSession(Options{ReferenceText: true})
	var window *app.Window
	app.UseApplicationRunner(func(start func(app.Host) error) error { return start(host) })
	defer app.UseApplicationRunner(nil)
	err := app.RunApplication(func(application *app.Application) error {
		var err error
		window, err = application.OpenWindow(app.Config{Title: "before", Width: 500, Height: 300}, nil)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	defer host.closeAll()
	if err := window.SetTitle("after"); err != nil {
		t.Fatal(err)
	}
	if text := run(t, host, "windows\n"); !strings.Contains(text, `title="after"`) {
		t.Fatalf("windows output does not list the new title:\n%s", text)
	}
	for _, bad := range []string{"bad\x00title", "bad\xfftitle"} {
		if err := window.SetTitle(bad); err == nil {
			t.Fatalf("title %q was accepted", bad)
		}
	}
	if text := run(t, host, "windows\n"); !strings.Contains(text, `title="after"`) {
		t.Fatalf("a rejected title changed the window:\n%s", text)
	}
	host.closeAll()
	if err := window.SetTitle("late"); err == nil {
		t.Fatal("closed window accepted a title")
	}
}
