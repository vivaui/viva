//go:build linux && !vivagtk

package linux

import (
	"errors"
	"os"
	"testing"
	"time"
)

func TestX11ClipboardChangeWithoutExtension(t *testing.T) {
	if os.Getenv("VIVA_LINUX_XCB_NO_XFIXES") != "1" {
		t.Skip("requires a private X server started with -extension XFIXES")
	}
	api, conn, _, err := openSystemConnection()
	if err != nil {
		t.Fatal(err)
	}
	api.disconnect(conn)
	defer OnClipboardChange(nil)
	if err := OnClipboardChange(func() {}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("clipboard watch without XFixes = %v, want ErrUnsupported", err)
	}
}

func TestX11ClipboardChangeLifecycle(t *testing.T) {
	if testing.Short() || os.Getenv("VIVA_LINUX_XCB_SMOKE") != "1" {
		t.Skip("set VIVA_LINUX_XCB_SMOKE=1 with a private X11 display and XFixes")
	}
	first, second := make(chan struct{}, 8), make(chan struct{}, 8)
	register := func(events chan struct{}) {
		t.Helper()
		if err := OnClipboardChange(func() { events <- struct{}{} }); err != nil {
			t.Fatal(err)
		}
	}
	wait := func(events chan struct{}) {
		t.Helper()
		select {
		case <-events:
		case <-time.After(2 * time.Second):
			t.Fatal("clipboard notification did not arrive")
		}
	}
	quiet := func(events chan struct{}) {
		t.Helper()
		select {
		case <-events:
			t.Fatal("unexpected clipboard notification")
		case <-time.After(40 * time.Millisecond):
		}
	}
	t.Cleanup(func() {
		if err := OnClipboardChange(nil); err != nil {
			t.Error(err)
		}
	})
	register(first) // Registration does not require an application window.
	quiet(first)

	api, conn, screen, err := openSystemConnection()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if conn != 0 {
			api.disconnect(conn)
		}
	}()
	owner := api.generateID(conn)
	api.createWindow(conn, xcbCopyFromParent, owner, screen.Root, 0, 0, 1, 1, 0, xcbWindowClassInput, 0, 0, nil)
	clipboard, err := api.atom(conn, "CLIPBOARD")
	if err != nil {
		t.Fatal(err)
	}
	primary, err := api.atom(conn, "PRIMARY")
	if err != nil {
		t.Fatal(err)
	}
	claim := func(selection uint32) {
		t.Helper()
		api.setSelectionOwner(conn, owner, selection, 0)
		if got, err := selectionOwner(api, conn, selection); err != nil || got != owner {
			t.Fatalf("selection owner = %d, %v; want %d", got, err, owner)
		}
	}
	claim(primary)
	quiet(first)
	claim(clipboard)
	claim(clipboard)
	wait(first)
	wait(first)
	quiet(first)

	register(second)
	claim(clipboard)
	wait(second)
	quiet(first)
	if err := OnClipboardChange(nil); err != nil {
		t.Fatal(err)
	}
	claim(clipboard)
	quiet(second)

	stopped := make(chan error, 1)
	if err := OnClipboardChange(func() { stopped <- OnClipboardChange(nil) }); err != nil {
		t.Fatal(err)
	}
	api.disconnect(conn)
	conn = 0
	select {
	case err := <-stopped:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("owner exit notification did not arrive or could not unregister")
	}
}
