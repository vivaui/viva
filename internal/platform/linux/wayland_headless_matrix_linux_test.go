//go:build linux

package linux

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/vivaui/viva/internal/platform"
)

// waylandHeadlessCompositor is one compositor process started for an
// integration scenario. Its socket lives in a private runtime directory.
type waylandHeadlessCompositor struct {
	name       string
	runtimeDir string
	socket     string
	cmd        *exec.Cmd
	log        bytes.Buffer
	done       chan struct{}
}

func startWaylandHeadlessCompositor(t *testing.T, name string, extraArgs ...string) *waylandHeadlessCompositor {
	t.Helper()
	path, err := exec.LookPath(name)
	if err != nil {
		t.Fatalf("%s is required for integration validation: %v", name, err)
	}
	runtimeDir := t.TempDir()
	if err := os.Chmod(runtimeDir, 0o700); err != nil {
		t.Fatalf("runtime directory permissions: %v", err)
	}
	socket := fmt.Sprintf("viva-test-%d", os.Getpid())
	args := append(waylandCompositorArgs(path, socket), extraArgs...)
	cmd := exec.Command(path, args...)
	// Wrappers such as unshare fork the real compositor; kill the whole group
	// so a wrapper cannot leave a compositor behind holding the GPU.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Env = append(os.Environ(), "XDG_RUNTIME_DIR="+runtimeDir, "WAYLAND_DISPLAY="+socket, "LIBGL_ALWAYS_SOFTWARE=1", "HOME="+runtimeDir)
	compositor := &waylandHeadlessCompositor{
		name:       name,
		runtimeDir: runtimeDir,
		socket:     socket,
		cmd:        cmd,
		done:       make(chan struct{}),
	}
	cmd.Stdout = &compositor.log
	cmd.Stderr = &compositor.log
	if err := cmd.Start(); err != nil {
		t.Fatalf("start %s: %v", name, err)
	}
	t.Logf("integration compositor: %s %v", path, cmd.Args[1:])
	go func() {
		_ = cmd.Wait()
		close(compositor.done)
	}()
	t.Cleanup(func() {
		_ = killWaylandHeadlessCompositor(cmd)
		<-compositor.done
		if compositor.log.Len() != 0 {
			t.Logf("%s output:\n%s", name, compositor.log.String())
		}
	})
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	t.Setenv("WAYLAND_DISPLAY", socket)
	t.Setenv("LIBGL_ALWAYS_SOFTWARE", "1")

	socketPath := filepath.Join(runtimeDir, socket)
	deadline := time.Now().Add(20 * time.Second)
	for {
		if _, err := os.Stat(socketPath); err == nil {
			return compositor
		}
		select {
		case <-compositor.done:
			t.Fatalf("%s exited before creating its socket:\n%s", name, compositor.log.String())
		case <-time.After(25 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for the %s socket", name)
		}
	}
}

// killWaylandHeadlessCompositor terminates the compositor's process group so a
// launcher that forks instead of exec'ing cannot leave the compositor running.
func killWaylandHeadlessCompositor(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	if err == nil || errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return cmd.Process.Kill()
}

func waylandIntegrationCompositor(t *testing.T) string {
	t.Helper()
	if os.Getenv("VIVA_WAYLAND_INTEGRATION") != "1" {
		t.Skip("set VIVA_WAYLAND_INTEGRATION=1 to run the headless compositor/Vulkan integration test")
	}
	name := os.Getenv("VIVA_WAYLAND_COMPOSITOR")
	if name == "" {
		name = "weston"
	}
	return name
}

func waylandEnvFloat(t *testing.T, name string, fallback float64) float64 {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 32)
	if err != nil {
		t.Fatalf("invalid %s=%q: %v", name, value, err)
	}
	return parsed
}

func waylandEnvInt(t *testing.T, name string, fallback int) int {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		t.Fatalf("invalid %s=%q: %v", name, value, err)
	}
	return parsed
}

// waylandHeadlessSettled reports whether the presented device size matches the
// most recent complete configure at the current scale. A compositor may send a
// transient configure (for example the unmaximized size) around a state change,
// so waits must not treat an earlier present with the new configure serial as
// the settled result. A zero configure means the compositor lets the client pick
// the size; there is no announced geometry to wait for in that case.
func waylandHeadlessSettled(s waylandHeadlessSnapshot) bool {
	if s.configuredWidth <= 0 || s.configuredHeight <= 0 {
		return true
	}
	width := int32(deviceLength(int(s.configuredWidth), s.scale)) + 2*s.margin
	height := int32(deviceLength(int(s.configuredHeight), s.scale)) + 2*s.margin
	return int32(s.width) == width && int32(s.height) == height
}

// TestWaylandHeadlessScaledOutputs checks that the client picks up the
// compositor's output scale and output count, and that maximized geometry is
// the configured logical size in device pixels. Expectations are supplied by
// VIVA_WAYLAND_EXPECT_SCALE and VIVA_WAYLAND_EXPECT_OUTPUTS so the same test
// can run against each compositor profile.
func TestWaylandHeadlessScaledOutputs(t *testing.T) {
	name := waylandIntegrationCompositor(t)
	expectedScale := float32(waylandEnvFloat(t, "VIVA_WAYLAND_EXPECT_SCALE", 1))
	expectedOutputs := waylandEnvInt(t, "VIVA_WAYLAND_EXPECT_OUTPUTS", 1)

	var extraArgs []string
	if strings.Contains(filepath.Base(name), "weston") {
		// The headless backend takes scale from the output section rather than
		// a command-line switch.
		config := filepath.Join(t.TempDir(), "weston.ini")
		content := fmt.Sprintf("[core]\nidle-time=0\n[output]\nname=headless\nscale=%s\n",
			strconv.FormatFloat(float64(expectedScale), 'f', -1, 32))
		if err := os.WriteFile(config, []byte(content), 0o600); err != nil {
			t.Fatalf("write weston config: %v", err)
		}
		extraArgs = append(extraArgs, "--config="+config)
	}
	compositor := startWaylandHeadlessCompositor(t, name, extraArgs...)

	app, err := newWaylandApplication()
	if err != nil {
		t.Fatalf("connect to %s: %v", name, err)
	}
	runDone := make(chan struct{})
	result := make(chan error, 1)
	started := false
	var win *waylandWindow
	var snapshot waylandHeadlessSnapshot
	runErr := app.Run(func(platform.Application) error {
		opened, err := app.openWindow(platform.Config{Width: 320, Height: 240})
		if err != nil {
			return err
		}
		win = opened
		started = true
		go func() {
			defer app.Quit()
			initial, err := waitWaylandHeadlessState(app, win, runDone, func(s waylandHeadlessSnapshot) bool {
				return s.surfaceState == waylandSurfaceMapped && s.rendererSubmissions > 0 &&
					waylandHeadlessSettled(s) &&
					s.rendererWidth == s.width && s.rendererHeight == s.height
			})
			if err != nil {
				result <- fmt.Errorf("initial present: %w", err)
				return
			}
			app.post(func() { win.setMaximized(true) })
			maximized, err := waitWaylandHeadlessState(app, win, runDone, func(s waylandHeadlessSnapshot) bool {
				return s.maximized && s.rendererSubmissions > initial.rendererSubmissions &&
					waylandHeadlessSettled(s) &&
					s.rendererWidth == s.width && s.rendererHeight == s.height
			})
			if err != nil {
				result <- fmt.Errorf("maximized present: %w", err)
				return
			}
			snapshot = maximized
			result <- nil
		}()
		return nil
	})
	close(runDone)
	if !started {
		t.Fatalf("scaled outputs did not start: %v", runErr)
	}
	if err := <-result; err != nil {
		t.Fatalf("%s scaled outputs: %v (application error: %v)", name, err, runErr)
	}
	if runErr != nil {
		t.Fatalf("clean quit returned an error: %v", runErr)
	}
	if win == nil || !win.closed.Load() || win.state != nil {
		t.Fatal("window was not released after quit")
	}
	if math.Abs(float64(snapshot.scale-expectedScale)) > 0.001 {
		t.Fatalf("window scale = %v, want %v", snapshot.scale, expectedScale)
	}
	if snapshot.outputs < expectedOutputs {
		t.Fatalf("compositor outputs = %d, want at least %d", snapshot.outputs, expectedOutputs)
	}
	if snapshot.configuredWidth > 0 {
		wantWidth := deviceLength(int(snapshot.configuredWidth), snapshot.scale)
		wantHeight := deviceLength(int(snapshot.configuredHeight), snapshot.scale)
		if snapshot.width != wantWidth || snapshot.height != wantHeight {
			t.Fatalf("maximized device size = %dx%d, want %dx%d (configured %dx%d at scale %v)",
				snapshot.width, snapshot.height, wantWidth, wantHeight,
				snapshot.configuredWidth, snapshot.configuredHeight, snapshot.scale)
		}
	}
	t.Logf("%s scaled outputs: scale=%v outputs=%d maximized %dx%d (configured %dx%d) submissions=%d",
		compositor.name, snapshot.scale, snapshot.outputs, snapshot.width, snapshot.height,
		snapshot.configuredWidth, snapshot.configuredHeight, snapshot.rendererSubmissions)
}

// TestWaylandHeadlessEventInterleaving drives maximize/restore and Hide/Show
// back to back, then queues all four transitions in one command, so configure
// and frame pairing cannot depend on a quiet sequence.
func TestWaylandHeadlessEventInterleaving(t *testing.T) {
	name := waylandIntegrationCompositor(t)
	startWaylandHeadlessCompositor(t, name)

	app, err := newWaylandApplication()
	if err != nil {
		t.Fatalf("connect to %s: %v", name, err)
	}
	runDone := make(chan struct{})
	result := make(chan error, 1)
	started := false
	var win *waylandWindow
	runErr := app.Run(func(platform.Application) error {
		opened, err := app.openWindow(platform.Config{Width: 320, Height: 240})
		if err != nil {
			return err
		}
		win = opened
		started = true
		go func() {
			defer app.Quit()
			result <- driveWaylandHeadlessInterleaving(t, app, win, runDone)
		}()
		return nil
	})
	close(runDone)
	if !started {
		t.Fatalf("event interleaving did not start: %v", runErr)
	}
	if err := <-result; err != nil {
		t.Fatalf("%s event interleaving: %v (application error: %v)", name, err, runErr)
	}
	if runErr != nil {
		t.Fatalf("clean quit returned an error: %v", runErr)
	}
	if win == nil || !win.closed.Load() || win.state != nil {
		t.Fatal("window was not released after quit")
	}
}

func driveWaylandHeadlessInterleaving(t *testing.T, app *waylandApplication, win *waylandWindow, runDone <-chan struct{}) error {
	wait := func(what string, check func(waylandHeadlessSnapshot) bool) (waylandHeadlessSnapshot, error) {
		snapshot, err := waitWaylandHeadlessState(app, win, runDone, check)
		if err != nil {
			return snapshot, fmt.Errorf("%s: %w", what, err)
		}
		return snapshot, nil
	}
	previous, err := wait("initial present", func(s waylandHeadlessSnapshot) bool {
		return s.surfaceState == waylandSurfaceMapped && s.rendererSubmissions > 0 &&
			waylandHeadlessSettled(s) &&
			s.rendererWidth == s.width && s.rendererHeight == s.height
	})
	if err != nil {
		return err
	}
	for cycle := 0; cycle < 3; cycle++ {
		app.post(func() { win.setMaximized(true) })
		maximized, err := wait(fmt.Sprintf("maximize %d", cycle), func(s waylandHeadlessSnapshot) bool {
			return s.maximized && s.rendererSubmissions > previous.rendererSubmissions &&
				waylandHeadlessSettled(s) &&
				s.rendererWidth == s.width && s.rendererHeight == s.height
		})
		if err != nil {
			return err
		}
		app.post(func() { win.setMaximized(false) })
		restored, err := wait(fmt.Sprintf("restore %d", cycle), func(s waylandHeadlessSnapshot) bool {
			return !s.maximized && s.rendererSubmissions > maximized.rendererSubmissions &&
				waylandHeadlessSettled(s) &&
				s.rendererWidth == s.width && s.rendererHeight == s.height
		})
		if err != nil {
			return err
		}
		win.Hide()
		if _, err := wait(fmt.Sprintf("hide %d", cycle), func(s waylandHeadlessSnapshot) bool {
			return s.surfaceState == waylandSurfaceUnmapped
		}); err != nil {
			return err
		}
		win.Show()
		remapped, err := wait(fmt.Sprintf("show %d", cycle), func(s waylandHeadlessSnapshot) bool {
			return s.surfaceState == waylandSurfaceMapped && s.rendererSubmissions > restored.rendererSubmissions &&
				waylandHeadlessSettled(s) &&
				s.rendererWidth == s.width && s.rendererHeight == s.height
		})
		if err != nil {
			return err
		}
		previous = remapped
	}

	app.post(func() {
		win.setMaximized(true)
		win.setMaximized(false)
		win.Hide()
		win.Show()
	})
	final, err := wait("queued transitions", func(s waylandHeadlessSnapshot) bool {
		return s.surfaceState == waylandSurfaceMapped && s.rendererSubmissions > previous.rendererSubmissions &&
			waylandHeadlessSettled(s) &&
			s.rendererWidth == s.width && s.rendererHeight == s.height
	})
	if err != nil {
		return err
	}
	t.Logf("interleaving: 3 cycles plus queued maximize/restore/hide/show, %dx%d submissions=%d",
		final.width, final.height, final.rendererSubmissions)
	return nil
}

func readContextSwitches(path string) (voluntary, involuntary uint64, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		value, parseErr := strconv.ParseUint(fields[1], 10, 64)
		if parseErr != nil {
			continue
		}
		switch fields[0] {
		case "voluntary_ctxt_switches:":
			voluntary = value
		case "nonvoluntary_ctxt_switches:":
			involuntary = value
		}
	}
	return voluntary, involuntary, nil
}

// TestWaylandHeadlessPerformance measures the observable frame pipeline:
// first-present and resize latency, hide/show remap latency, idle context
// switches, frame-callback coalescing and the wait for the next presentation.
// It is a measurement, not a timing benchmark; the log line is the result.
func TestWaylandHeadlessPerformance(t *testing.T) {
	name := waylandIntegrationCompositor(t)
	startWaylandHeadlessCompositor(t, name)

	app, err := newWaylandApplication()
	if err != nil {
		t.Fatalf("connect to %s: %v", name, err)
	}
	runDone := make(chan struct{})
	result := make(chan error, 1)
	started := false
	var win *waylandWindow
	runErr := app.Run(func(platform.Application) error {
		opened, err := app.openWindow(platform.Config{Width: 320, Height: 240})
		if err != nil {
			return err
		}
		win = opened
		started = true
		go func() {
			defer app.Quit()
			result <- driveWaylandHeadlessPerformance(t, app, win, runDone)
		}()
		return nil
	})
	close(runDone)
	if !started {
		t.Fatalf("performance did not start: %v", runErr)
	}
	if err := <-result; err != nil {
		t.Fatalf("%s performance: %v (application error: %v)", name, err, runErr)
	}
	if runErr != nil {
		t.Fatalf("clean quit returned an error: %v", runErr)
	}
	if win == nil || !win.closed.Load() || win.state != nil {
		t.Fatal("window was not released after quit")
	}
}

func driveWaylandHeadlessPerformance(t *testing.T, app *waylandApplication, win *waylandWindow, runDone <-chan struct{}) error {
	wait := func(what string, check func(waylandHeadlessSnapshot) bool) (waylandHeadlessSnapshot, error) {
		snapshot, err := waitWaylandHeadlessStateEvery(app, win, runDone, time.Millisecond, check)
		if err != nil {
			return snapshot, fmt.Errorf("%s: %w", what, err)
		}
		return snapshot, nil
	}
	openStart := time.Now()
	initial, err := wait("initial present", func(s waylandHeadlessSnapshot) bool {
		return s.surfaceState == waylandSurfaceMapped && s.rendererSubmissions > 0 &&
			waylandHeadlessSettled(s) &&
			s.rendererWidth == s.width && s.rendererHeight == s.height
	})
	if err != nil {
		return err
	}
	firstPresent := time.Since(openStart)

	start := time.Now()
	app.post(func() { win.setMaximized(true) })
	maximized, err := wait("maximize present", func(s waylandHeadlessSnapshot) bool {
		return s.maximized && s.rendererSubmissions > initial.rendererSubmissions &&
			waylandHeadlessSettled(s) &&
			s.rendererWidth == s.width && s.rendererHeight == s.height
	})
	if err != nil {
		return err
	}
	maximizeLatency := time.Since(start)

	start = time.Now()
	app.post(func() { win.setMaximized(false) })
	restored, err := wait("restore present", func(s waylandHeadlessSnapshot) bool {
		return !s.maximized && s.rendererSubmissions > maximized.rendererSubmissions &&
			waylandHeadlessSettled(s) &&
			s.rendererWidth == s.width && s.rendererHeight == s.height
	})
	if err != nil {
		return err
	}
	restoreLatency := time.Since(start)

	win.Hide()
	if _, err := wait("hide", func(s waylandHeadlessSnapshot) bool {
		return s.surfaceState == waylandSurfaceUnmapped
	}); err != nil {
		return err
	}
	start = time.Now()
	win.Show()
	remapped, err := wait("show present", func(s waylandHeadlessSnapshot) bool {
		return s.surfaceState == waylandSurfaceMapped && s.rendererSubmissions > restored.rendererSubmissions &&
			waylandHeadlessSettled(s) &&
			s.rendererWidth == s.width && s.rendererHeight == s.height
	})
	if err != nil {
		return err
	}
	remapLatency := time.Since(start)

	voluntaryBefore, involuntaryBefore, err := readContextSwitches("/proc/self/status")
	if err != nil {
		return fmt.Errorf("read context switches: %w", err)
	}
	time.Sleep(2 * time.Second)
	voluntaryAfter, involuntaryAfter, err := readContextSwitches("/proc/self/status")
	if err != nil {
		return fmt.Errorf("read context switches: %w", err)
	}

	const requested = 32
	before := remapped
	start = time.Now()
	app.post(func() {
		for i := 0; i < requested; i++ {
			win.renderFrame()
		}
	})
	if _, err := wait("coalesced present", func(s waylandHeadlessSnapshot) bool {
		return s.rendererSubmissions > before.rendererSubmissions
	}); err != nil {
		return err
	}
	frameWait := time.Since(start)
	time.Sleep(250 * time.Millisecond)
	settled, err := waylandHeadlessState(app, win, runDone)
	if err != nil {
		return err
	}
	presentedFrames := settled.rendererSubmissions - before.rendererSubmissions

	t.Logf("performance: firstPresent=%s maximize=%s restore=%s remap=%s idleVoluntary=%d idleInvoluntary=%d requested=%d presented=%d frameWait=%s poll=1ms",
		firstPresent.Round(time.Millisecond), maximizeLatency.Round(time.Millisecond),
		restoreLatency.Round(time.Millisecond), remapLatency.Round(time.Millisecond),
		voluntaryAfter-voluntaryBefore, involuntaryAfter-involuntaryBefore,
		requested, presentedFrames, frameWait.Round(time.Millisecond))
	if firstPresent > 10*time.Second || maximizeLatency > 10*time.Second ||
		restoreLatency > 10*time.Second || remapLatency > 10*time.Second {
		return errors.New("frame latency exceeded 10s")
	}
	if voluntaryAfter-voluntaryBefore > 10000 || involuntaryAfter-involuntaryBefore > 10000 {
		return errors.New("idle context switches exceeded 10000 in 2s")
	}
	if presentedFrames < 1 || presentedFrames >= requested {
		return fmt.Errorf("no coalescing: presented %d frames for %d requests", presentedFrames, requested)
	}
	return nil
}
