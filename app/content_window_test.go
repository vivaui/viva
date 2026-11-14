//go:build darwin || windows || linux

package app

import (
	"sync"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestContentWindowDeliversUpdatesAndHonorsClosePolicy(t *testing.T) {
	host := &fakeHost{}
	useHost(t, host)
	err := RunApplication(func(application *Application) error {
		var allow func()
		wakes, closed := 0, 0
		content, err := application.OpenContentWindow(Config{Title: "document", OnWake: func() { wakes++ }, OnClosed: func() { closed++ }, OnCloseRequest: func(next func()) { allow = next }}, nil)
		if err != nil {
			return err
		}
		second, err := application.OpenContentWindow(Config{Title: "inspector"}, nil)
		if err != nil {
			return err
		}
		if host.windows[0].config.Width != 960 || host.windows[0].config.Height != 720 {
			t.Fatal("missing default size")
		}
		var wg sync.WaitGroup
		count := 0
		for range 32 {
			wg.Go(func() {
				if !content.Post(func() { count++ }) {
					t.Error("rejected live update")
				}
			})
		}
		wg.Wait()
		if count != 0 {
			t.Fatal("background worker changed UI state")
		}
		host.windows[0].config.OnWake()
		if count != 32 || wakes != 1 {
			t.Fatal("updates were not delivered before wake", count, wakes)
		}
		content.RequestClose()
		if content.Closed() || allow == nil {
			t.Fatal("close policy was bypassed")
		}
		allow()
		// The fake host records native destruction separately from application bookkeeping.
		host.windows[0].config.OnClosed()
		if !content.Closed() || closed != 1 || second.Closed() {
			t.Fatal("independent lifetime lost")
		}
		if content.Post(func() { t.Error("closed update ran") }) {
			t.Fatal("accepted closed update")
		}
		host.windows[0].config.OnWake()
		if wakes != 1 {
			t.Fatal("closed window woke")
		}
		second.Close()
		host.windows[1].config.OnClosed()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestContentWindowReportsFrameAndMinimizeState(t *testing.T) {
	host := &fakeHost{}
	useHost(t, host)
	if err := RunApplication(func(application *Application) error {
		origin := gfx.Point{X: 120, Y: 240}
		content, err := application.OpenContentWindow(Config{Width: 500, Height: 300, Origin: &origin}, nil)
		if err != nil {
			return err
		}
		frame, ok := content.Frame()
		if !ok || frame != (gfx.Rect{X: 120, Y: 240, Width: 500, Height: 300}) {
			t.Fatalf("frame = %+v, ok = %t", frame, ok)
		}
		if content.Minimized() {
			t.Fatal("a new content window reports minimized")
		}
		host.windows[0].config.OnMinimize(true)
		if !content.Minimized() {
			t.Fatal("minimize state did not follow the window")
		}
		host.windows[0].config.OnMinimize(false)
		if content.Minimized() {
			t.Fatal("minimize state stayed after restore")
		}
		content.Close()
		host.windows[0].config.OnClosed()
		if _, ok := content.Frame(); ok {
			t.Fatal("closed content window still reports a frame")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestContentWindowRejectedOpenAndPendingUpdateTeardown(t *testing.T) {
	host := &fakeHost{reject: "rejected"}
	useHost(t, host)
	if err := RunApplication(func(application *Application) error {
		if content, err := application.OpenContentWindow(Config{Title: "rejected"}, nil); err == nil || content != nil {
			t.Fatal("rejected native open retained")
		}
		content, err := application.OpenContentWindow(Config{}, nil)
		if err != nil {
			return err
		}
		content.Post(func() { t.Error("update outlived native content") })
		host.windows[0].config.OnClosed()
		content.Close()
		host.windows[0].config.OnWake()
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestContentWindowPostQueueIsBoundedAndReusable(t *testing.T) {
	host := &fakeHost{}
	useHost(t, host)
	if err := RunApplication(func(application *Application) error {
		content, err := application.OpenContentWindow(Config{}, nil)
		if err != nil {
			return err
		}
		delivered := 0
		for range contentWindowPostLimit {
			if !content.Post(func() { delivered++ }) {
				t.Fatal("rejected update before queue reached its limit")
			}
		}
		if content.Post(func() { t.Error("overflow update ran") }) {
			t.Fatal("accepted update beyond queue limit")
		}
		host.windows[0].config.OnWake()
		if delivered != contentWindowPostLimit {
			t.Fatalf("delivered %d queued updates", delivered)
		}
		if !content.Post(func() { delivered++ }) {
			t.Fatal("drained queue did not accept another update")
		}
		host.windows[0].config.OnWake()
		if delivered != contentWindowPostLimit+1 {
			t.Fatal("reused queue did not deliver its update")
		}
		content.Close()
		host.windows[0].config.OnClosed()
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
