//go:build darwin || windows || linux

package headless

import (
	"testing"

	"github.com/vivaui/viva/app"
)

func TestOpenFileReachesTheApplicationsHandler(t *testing.T) {
	host := newApplicationSession(Options{Scale: 1})
	t.Cleanup(host.closeAll)
	app.UseApplicationRunner(func(start func(app.Host) error) error { return start(host) })
	t.Cleanup(func() { app.UseApplicationRunner(nil) })
	var got []string
	err := app.RunApplication(func(application *app.Application) error {
		application.OnOpenFile(func(path string) { got = append(got, path) })
		_, err := application.OpenWindow(app.Config{Title: "main", Width: 100, Height: 80}, nil)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if reply, _ := executeApplication(host, "openfile /tmp/notes example.md"); reply != "ok\n" {
		t.Fatalf("reply %q", reply)
	}
	if len(got) != 1 || got[0] != "/tmp/notes example.md" {
		t.Fatalf("handler got %q", got)
	}
	if reply, _ := executeApplication(host, "openfile"); reply[:5] != "error" {
		t.Fatalf("reply %q", reply)
	}
}

func TestOpenFileWithoutAHandlerIsAnError(t *testing.T) {
	host := newApplicationSession(Options{Scale: 1})
	t.Cleanup(host.closeAll)
	if reply, _ := executeApplication(host, "openfile /tmp/a.md"); reply[:5] != "error" {
		t.Fatalf("reply %q", reply)
	}
}
