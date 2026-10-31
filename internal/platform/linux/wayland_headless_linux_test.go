//go:build linux

package linux

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/internal/platform"
)

// waylandHeadlessSnapshot is one consistent read of the window and renderer
// state owned by the event-loop thread.
type waylandHeadlessSnapshot struct {
	width, height       uint32
	surfaceState        waylandSurfaceState
	maximized           bool
	rendererWidth       uint32
	rendererHeight      uint32
	rendererSubmissions uint64
	configureSerial     uint32
	toplevelPending     bool
	scale               float32
	outputs             int
	configuredWidth     int32
	configuredHeight    int32
	margin              int32
}

func readWaylandHeadlessSnapshot(win *waylandWindow) waylandHeadlessSnapshot {
	snapshot := waylandHeadlessSnapshot{
		width:            win.width,
		height:           win.height,
		surfaceState:     win.surfaceState,
		maximized:        win.maximized,
		configureSerial:  win.configure.serial,
		toplevelPending:  win.configure.toplevelPending,
		scale:            win.displayScale(),
		configuredWidth:  win.configure.width,
		configuredHeight: win.configure.height,
		margin:           int32(win.margin),
	}
	if win.app != nil {
		snapshot.outputs = len(win.app.globals.outputs)
	}
	if win.state != nil && win.state.renderer != nil {
		renderer := win.state.renderer
		snapshot.rendererWidth = renderer.width
		snapshot.rendererHeight = renderer.height
		snapshot.rendererSubmissions = renderer.submissions
	}
	return snapshot
}

func waylandHeadlessState(app *waylandApplication, win *waylandWindow, runDone <-chan struct{}) (waylandHeadlessSnapshot, error) {
	result := make(chan waylandHeadlessSnapshot, 1)
	app.post(func() { result <- readWaylandHeadlessSnapshot(win) })
	select {
	case snapshot := <-result:
		return snapshot, nil
	case <-runDone:
		return waylandHeadlessSnapshot{}, errors.New("application stopped while reading window state")
	case <-time.After(20 * time.Second):
		return waylandHeadlessSnapshot{}, errors.New("timed out reading window state")
	}
}

func waitWaylandHeadlessState(app *waylandApplication, win *waylandWindow, runDone <-chan struct{}, check func(waylandHeadlessSnapshot) bool) (waylandHeadlessSnapshot, error) {
	return waitWaylandHeadlessStateEvery(app, win, runDone, 20*time.Millisecond, check)
}

func waitWaylandHeadlessStateEvery(app *waylandApplication, win *waylandWindow, runDone <-chan struct{}, interval time.Duration, check func(waylandHeadlessSnapshot) bool) (waylandHeadlessSnapshot, error) {
	deadline := time.Now().Add(20 * time.Second)
	for {
		snapshot, err := waylandHeadlessState(app, win, runDone)
		if err != nil {
			return snapshot, err
		}
		if check(snapshot) {
			return snapshot, nil
		}
		if time.Now().After(deadline) {
			return snapshot, fmt.Errorf("timed out waiting for window state (last %dx%d mapped=%v maximized=%v renderer=%dx%d submissions=%d serial=%d toplevelPending=%v)",
				snapshot.width, snapshot.height, snapshot.surfaceState == waylandSurfaceMapped, snapshot.maximized,
				snapshot.rendererWidth, snapshot.rendererHeight, snapshot.rendererSubmissions,
				snapshot.configureSerial, snapshot.toplevelPending)
		}
		select {
		case <-runDone:
			return snapshot, errors.New("application stopped before the expected state")
		case <-time.After(interval):
		}
	}
}

// driveWaylandHeadlessWindow checks that the window maps and presents, that
// maximize and restore run the swapchain rebuild transaction, that Hide/Show
// remaps, that a maximized window can be hidden and remapped, and that killing
// the compositor ends the application.
func driveWaylandHeadlessWindow(t *testing.T, app *waylandApplication, win *waylandWindow, runDone <-chan struct{}, compositor *exec.Cmd) error {
	wait := func(what string, check func(waylandHeadlessSnapshot) bool) (waylandHeadlessSnapshot, error) {
		snapshot, err := waitWaylandHeadlessState(app, win, runDone, check)
		if err != nil {
			return snapshot, fmt.Errorf("%s: %w", what, err)
		}
		return snapshot, nil
	}

	initial, err := wait("initial map and present", func(snapshot waylandHeadlessSnapshot) bool {
		return snapshot.surfaceState == waylandSurfaceMapped &&
			snapshot.rendererSubmissions > 0 &&
			snapshot.rendererWidth == snapshot.width && snapshot.rendererHeight == snapshot.height
	})
	if err != nil {
		return err
	}
	before := [2]uint32{initial.width, initial.height}

	// Maximize and restore run before Hide/Show. A compositor may re-derive the
	// normal client size when a window is remapped (KWin subtracts its server-side
	// decoration extents), so restoring to the pre-maximize size after a remap is
	// compositor-defined rather than a property of the client transaction.
	app.post(func() { win.setMaximized(true) })
	maximized, err := wait("maximize resize", func(snapshot waylandHeadlessSnapshot) bool {
		return snapshot.maximized &&
			(snapshot.width != before[0] || snapshot.height != before[1]) &&
			snapshot.rendererWidth == snapshot.width && snapshot.rendererHeight == snapshot.height &&
			snapshot.rendererSubmissions > initial.rendererSubmissions
	})
	if err != nil {
		return err
	}

	app.post(func() { win.setMaximized(false) })
	restored, err := wait("restore resize", func(snapshot waylandHeadlessSnapshot) bool {
		return !snapshot.maximized &&
			snapshot.width == before[0] && snapshot.height == before[1] &&
			snapshot.rendererWidth == snapshot.width && snapshot.rendererHeight == snapshot.height &&
			snapshot.rendererSubmissions > maximized.rendererSubmissions
	})
	if err != nil {
		return err
	}

	win.Hide()
	if _, err := wait("hide", func(snapshot waylandHeadlessSnapshot) bool {
		return snapshot.surfaceState == waylandSurfaceUnmapped
	}); err != nil {
		return err
	}
	win.Show()
	remapped, err := wait("show and present", func(snapshot waylandHeadlessSnapshot) bool {
		return snapshot.surfaceState == waylandSurfaceMapped &&
			snapshot.rendererSubmissions > restored.rendererSubmissions &&
			snapshot.rendererWidth == snapshot.width && snapshot.rendererHeight == snapshot.height
	})
	if err != nil {
		return err
	}

	// Weston keeps the configured toplevel state across the unmap and enforces
	// the retained geometry when the window is remapped; other compositors
	// answer the remap with a fresh configure. Either way a remap while
	// maximized must present without a display error.
	app.post(func() { win.setMaximized(true) })
	remaximized, err := wait("maximize before hide", func(snapshot waylandHeadlessSnapshot) bool {
		return snapshot.maximized &&
			snapshot.width != before[0] &&
			snapshot.rendererWidth == snapshot.width && snapshot.rendererHeight == snapshot.height &&
			snapshot.rendererSubmissions > remapped.rendererSubmissions
	})
	if err != nil {
		return err
	}
	win.Hide()
	if _, err := wait("hide maximized", func(snapshot waylandHeadlessSnapshot) bool {
		return snapshot.surfaceState == waylandSurfaceUnmapped
	}); err != nil {
		return err
	}
	win.Show()
	maximizedRemap, err := wait("show maximized and present", func(snapshot waylandHeadlessSnapshot) bool {
		return snapshot.surfaceState == waylandSurfaceMapped &&
			snapshot.rendererSubmissions > remaximized.rendererSubmissions &&
			snapshot.rendererWidth == snapshot.width && snapshot.rendererHeight == snapshot.height
	})
	if err != nil {
		return err
	}
	if app.globals.weston && (!maximizedRemap.maximized ||
		maximizedRemap.width != remaximized.width || maximizedRemap.height != remaximized.height) {
		return fmt.Errorf("weston remap changed the retained toplevel state: maximized=%v %dx%d, want maximized=true %dx%d",
			maximizedRemap.maximized, maximizedRemap.width, maximizedRemap.height, remaximized.width, remaximized.height)
	}

	// A protocol error for the remap is queued behind our own request stream, so
	// the client-side present can complete before the compositor's error event
	// arrives. Roundtrip so any display error surfaces before the test kills the
	// compositor, otherwise the kill would mask it.
	if err := app.invoke(func() error {
		if app.api == nil || app.display == 0 || app.api.displayRoundtrip == nil {
			return errors.New("wayland display is unavailable")
		}
		if app.api.displayRoundtrip(app.display) < 0 {
			return fmt.Errorf("display roundtrip failed (error %d)", app.api.displayGetError(app.display))
		}
		return nil
	}); err != nil {
		return fmt.Errorf("display error after maximized remap: %w", err)
	}
	t.Logf("initial %dx%d, maximized %dx%d, restored %dx%d, remapped %dx%d, maximized remap %dx%d, submissions %d",
		initial.width, initial.height, maximized.width, maximized.height,
		restored.width, restored.height, remapped.width, remapped.height,
		maximizedRemap.width, maximizedRemap.height, maximizedRemap.rendererSubmissions)

	if err := killWaylandHeadlessCompositor(compositor); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("stop compositor: %w", err)
	}
	return nil
}

// waylandCompositorArgs returns the headless arguments for the compositor named
// by VIVA_WAYLAND_COMPOSITOR, which defaults to Weston. VIVA_WAYLAND_COMPOSITOR_ARGS
// overrides them for compositors without a built-in profile; arguments are split
// on whitespace (no quoting) and {socket} is replaced with the socket the test
// waits for.
func waylandCompositorArgs(name, socket string) []string {
	override := os.Getenv("VIVA_WAYLAND_COMPOSITOR_ARGS")
	if override != "" {
		args := make([]string, 0, 4)
		for _, field := range strings.Fields(override) {
			args = append(args, strings.ReplaceAll(field, "{socket}", socket))
		}
		return args
	}
	if filepath.Base(name) == "kwin_wayland" {
		return []string{"--virtual", "--socket=" + socket, "--width=1024", "--height=768", "--no-lockscreen", "--no-global-shortcuts"}
	}
	return []string{"--backend=headless", "--renderer=pixman", "--socket=" + socket, "--idle-time=0"}
}

// TestWaylandHeadlessIntegration exercises real libwayland, xdg-shell and
// Vulkan WSI against a headless compositor and the software Vulkan driver
// selected by the environment. Weston is the default; set VIVA_WAYLAND_COMPOSITOR
// to run the same entry against another compositor with a headless backend.
func TestWaylandHeadlessIntegration(t *testing.T) {
	if os.Getenv("VIVA_WAYLAND_INTEGRATION") != "1" {
		t.Skip("set VIVA_WAYLAND_INTEGRATION=1 to run the headless compositor/Vulkan integration test")
	}
	compositorName := os.Getenv("VIVA_WAYLAND_COMPOSITOR")
	if compositorName == "" {
		compositorName = "weston"
	}
	compositor := startWaylandHeadlessCompositor(t, compositorName)

	app, err := newWaylandApplication()
	if err != nil {
		t.Fatalf("connect to %s: %v", compositorName, err)
	}
	runDone := make(chan struct{})
	driverResult := make(chan error, 1)
	driverStarted := false
	var win *waylandWindow
	var renderer *vulkanRenderer
	runErr := app.Run(func(platform.Application) error {
		opened, err := app.openWindow(platform.Config{Width: 320, Height: 240})
		if err != nil {
			return err
		}
		win = opened
		renderer = opened.state.renderer
		driverStarted = true
		go func() {
			if err := driveWaylandHeadlessWindow(t, app, win, runDone, compositor.cmd); err != nil {
				driverResult <- err
				app.Quit()
				return
			}
			driverResult <- nil
		}()
		return nil
	})
	close(runDone)
	if !driverStarted {
		t.Fatalf("window integration did not start: %v", runErr)
	}
	if err := <-driverResult; err != nil {
		t.Fatalf("window integration: %v (application error: %v)", err, runErr)
	}
	if runErr == nil {
		t.Fatal("compositor disconnect returned a normal application exit")
	}
	t.Logf("application error after compositor disconnect: %v", runErr)
	if win == nil || !win.closed.Load() || win.state != nil {
		t.Fatal("window stayed open after compositor disconnect")
	}
	if renderer == nil || renderer.api != nil {
		t.Fatal("Vulkan renderer was not released after compositor disconnect")
	}
}
