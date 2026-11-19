//go:build darwin || windows || linux

package app

import (
	"errors"
	"fmt"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

// movableWindow is a fake window that can be moved, as a preview's is.
type movableWindow struct {
	*fakeWindow
	moves []gfx.Point
}

func (window *movableWindow) MoveTo(origin gfx.Point) { window.moves = append(window.moves, origin) }

// previewingHost is a host whose adapter can show a drag preview.
type previewingHost struct {
	*fakeHost
	opened []*movableWindow
}

func (host *previewingHost) CanPreview() bool { return true }

func (host *previewingHost) OpenWindow(config Config, canvas *gfx.Canvas, closed func()) (WindowControl, error) {
	control, err := host.fakeHost.OpenWindow(config, canvas, closed)
	if err != nil {
		return nil, err
	}
	window := &movableWindow{fakeWindow: control.(*fakeWindow)}
	host.opened = append(host.opened, window)
	return window, nil
}

// resultPreviewingHost mimics the native host that wraps an adapter window in
// resultWindow before handing it to an application.
type resultPreviewingHost struct {
	*fakeHost
}

func (host *resultPreviewingHost) CanPreview() bool { return true }

func (host *resultPreviewingHost) OpenWindow(config Config, canvas *gfx.Canvas, closed func()) (WindowControl, error) {
	control, err := host.fakeHost.OpenWindow(config, canvas, closed)
	if err != nil {
		return nil, err
	}
	return resultWindow{WindowControl: control}, nil
}

func usePreviewHost(t *testing.T, host *previewingHost) {
	t.Helper()
	previous := applicationRunner
	t.Cleanup(func() {
		applicationRunner = previous
		for _, window := range host.windows {
			window.Close()
		}
	})
	applicationRunner = func(start func(Host) error) error { return start(host) }
}

func TestADragPreviewFollowsMovesAndGoesWhenClosed(t *testing.T) {
	host := &previewingHost{fakeHost: &fakeHost{}}
	usePreviewHost(t, host)
	var preview *DragPreview
	err := RunApplication(func(application *Application) error {
		if _, err := application.OpenWindow(Config{Title: "main"}, nil); err != nil {
			return err
		}
		var err error
		preview, err = application.OpenDragPreview(DragPreviewConfig{Width: 120, Height: 28, Origin: gfx.Point{X: 30, Y: 40}})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	config := host.windows[1].config
	if config.WindowMode != windowPreview || config.Width != 120 || config.Height != 28 || config.originPoint() != (gfx.Point{X: 30, Y: 40}) {
		t.Fatalf("the preview window was asked for as %+v", config)
	}
	if normalized, err := config.normalize(); err != nil || normalized.ClearColor.Alpha != 0 {
		t.Fatalf("a preview must stay transparent: %+v, %v", normalized.ClearColor, err)
	}
	preview.MoveTo(gfx.Point{X: 500, Y: 600})
	if moves := host.opened[1].moves; len(moves) != 1 || moves[0] != (gfx.Point{X: 500, Y: 600}) {
		t.Fatalf("moves = %v", moves)
	}
	preview.Close()
	preview.Close()
	preview.MoveTo(gfx.Point{X: 1, Y: 1})
	if !host.windows[1].done || len(host.opened[1].moves) != 1 {
		t.Fatalf("closed %v, moves after close %v", host.windows[1].done, host.opened[1].moves)
	}
}

func TestADragPreviewRejectsANativeWindowThatCannotMove(t *testing.T) {
	host := &resultPreviewingHost{fakeHost: &fakeHost{}}
	previous := applicationRunner
	t.Cleanup(func() {
		applicationRunner = previous
		for _, window := range host.windows {
			window.Close()
		}
	})
	applicationRunner = func(start func(Host) error) error { return start(host) }

	err := RunApplication(func(application *Application) error {
		if _, err := application.OpenWindow(Config{Title: "main"}, nil); err != nil {
			return err
		}
		if _, err := application.OpenDragPreview(DragPreviewConfig{Width: 10, Height: 10}); !errors.Is(err, ErrUnsupported) {
			t.Fatalf("OpenDragPreview error = %v, want ErrUnsupported", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(host.windows) != 2 || !host.windows[1].done {
		t.Fatalf("unmovable preview was not closed: windows=%d done=%v", len(host.windows), len(host.windows) > 1 && host.windows[1].done)
	}
}

func TestADragPreviewIsRefusedWhereTheHostCannotShowOne(t *testing.T) {
	host := &fakeHost{}
	useHost(t, host)
	err := RunApplication(func(application *Application) error {
		if _, err := application.OpenWindow(Config{Title: "main"}, nil); err != nil {
			return err
		}
		_, err := application.OpenDragPreview(DragPreviewConfig{Width: 10, Height: 10})
		if !errors.Is(err, ErrUnsupported) {
			t.Fatalf("OpenDragPreview error = %v, want ErrUnsupported", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(host.windows) != 1 {
		t.Fatalf("a refused preview still opened %d windows", len(host.windows))
	}
}

func TestANativeWindowResultStillMovesAPreview(t *testing.T) {
	moving := &movableWindow{fakeWindow: &fakeWindow{}}
	control := resultWindow{WindowControl: moving}
	mover, ok := any(control).(platform.WindowMover)
	if !ok {
		t.Fatal("a native window result hid the platform's MoveTo")
	}
	mover.MoveTo(gfx.Point{X: 30, Y: 40})
	if len(moving.moves) != 1 || moving.moves[0] != (gfx.Point{X: 30, Y: 40}) {
		t.Fatalf("moves = %v", moving.moves)
	}
}

func BenchmarkDragPreviewMoveTo(b *testing.B) {
	host := &previewingHost{fakeHost: &fakeHost{}}
	var preview *DragPreview
	if err := func() error {
		application := &Application{host: host}
		var err error
		preview, err = application.OpenDragPreview(DragPreviewConfig{Width: 10, Height: 10})
		return err
	}(); err != nil {
		b.Fatal(err)
	}
	defer preview.Close()
	host.opened[0].moves = make([]gfx.Point, 0, b.N)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		preview.MoveTo(gfx.Point{X: float32(i), Y: 1})
	}
}

// unavailablePreviewHost is a host that advertises previews but cannot create
// one: the compositor is missing.
type unavailablePreviewHost struct{ *fakeHost }

func (host *unavailablePreviewHost) CanPreview() bool { return true }

func (host *unavailablePreviewHost) OpenWindow(config Config, canvas *gfx.Canvas, closed func()) (WindowControl, error) {
	if config.WindowMode == windowPreview {
		return nil, fmt.Errorf("%w: no compositor", platform.ErrDragPreviewUnsupported)
	}
	return host.fakeHost.OpenWindow(config, canvas, closed)
}

func TestADragPreviewTheHostCannotShowKeepsTheDrag(t *testing.T) {
	host := &unavailablePreviewHost{fakeHost: &fakeHost{}}
	previous := applicationRunner
	t.Cleanup(func() {
		applicationRunner = previous
		for _, window := range host.windows {
			window.Close()
		}
	})
	applicationRunner = func(start func(Host) error) error { return start(host) }
	err := RunApplication(func(application *Application) error {
		if _, err := application.OpenWindow(Config{Title: "main"}, nil); err != nil {
			return err
		}
		_, err := application.OpenDragPreview(DragPreviewConfig{Width: 120, Height: 28})
		if !errors.Is(err, ErrNoDragPreview) || !errors.Is(err, ErrUnsupported) {
			t.Fatalf("open error = %v, want ErrNoDragPreview", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
