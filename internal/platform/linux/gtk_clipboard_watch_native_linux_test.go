//go:build linux && vivagtk

package linux

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/vivaui/viva/internal/platform"
)

func TestGTKClipboardChangeNative(t *testing.T) {
	backend := os.Getenv("VIVA_TEST_GTK_CLIPBOARD_WATCH")
	if backend != "x11" && backend != "wayland" {
		t.Skip("requires a private GTK display")
	}
	first, second := make(chan struct{}, 8), make(chan struct{}, 8)
	result := make(chan error, 1)
	err := RunGTKApplication(func(app platform.Application) error {
		if _, err := app.OpenWindow(platform.Config{Title: "Clipboard watch test", Width: 160, Height: 100}); err != nil {
			return err
		}
		if gtkRunning.Load().wayland != (backend == "wayland") {
			return fmt.Errorf("GTK did not select the %s display", backend)
		}
		err := OnClipboardChange(func() { first <- struct{}{} })
		if backend == "wayland" {
			if !errors.Is(err, ErrUnsupported) {
				return fmt.Errorf("Wayland registration = %v, want ErrUnsupported", err)
			}
			app.Quit()
			return nil
		}
		if err != nil {
			return err
		}
		go func() {
			result <- exerciseGTKClipboardChange(first, second)
			app.Quit()
		}()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if backend == "x11" {
		if err := <-result; err != nil {
			t.Fatal(err)
		}
	}
}

func exerciseGTKClipboardChange(first, second chan struct{}) error {
	wait := func(ch chan struct{}) error {
		select {
		case <-ch:
			return nil
		case <-time.After(2 * time.Second):
			return errors.New("clipboard notification did not arrive")
		}
	}
	quiet := func(ch chan struct{}) error {
		select {
		case <-ch:
			return errors.New("unexpected clipboard notification")
		case <-time.After(40 * time.Millisecond):
			return nil
		}
	}
	api, conn, screen, err := openSystemConnection()
	if err != nil {
		return err
	}
	defer func() {
		if conn != 0 {
			api.disconnect(conn)
		}
	}()
	owner := api.generateID(conn)
	api.createWindow(conn, xcbCopyFromParent, owner, screen.Root, 0, 0, 1, 1, 0, xcbWindowClassInput, 0, 0, nil)
	claim := func(name string) error {
		atom, err := api.atom(conn, name)
		if err != nil {
			return err
		}
		api.setSelectionOwner(conn, owner, atom, 0)
		if got, err := selectionOwner(api, conn, atom); err != nil || got != owner {
			return fmt.Errorf("selection owner = %d, %v; want %d", got, err, owner)
		}
		return nil
	}
	for _, step := range []func() error{
		func() error { return claim("PRIMARY") },
		func() error { return quiet(first) },
		func() error { return claim("CLIPBOARD") },
		func() error { return wait(first) },
		func() error { return claim("CLIPBOARD") },
		func() error { return wait(first) },
		func() error { return OnClipboardChange(func() { second <- struct{}{} }) },
		func() error { return claim("CLIPBOARD") },
		func() error { return wait(second) },
		func() error { return quiet(first) },
		func() error { return OnClipboardChange(nil) },
		func() error { return claim("CLIPBOARD") },
		func() error { return quiet(second) },
		func() error { return OnClipboardChange(func() { second <- struct{}{} }) },
	} {
		if err := step(); err != nil {
			return err
		}
	}
	api.disconnect(conn)
	conn = 0
	return wait(second)
}
