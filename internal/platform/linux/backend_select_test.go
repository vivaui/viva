//go:build linux

package linux

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/vivaui/viva/internal/platform"
)

func TestSelectBackendForcedX11(t *testing.T) {
	t.Setenv("VIVA_LINUX_BACKEND", "x11")
	t.Setenv("DISPLAY", ":0")

	backend, err := selectBackend()
	if err != nil {
		t.Fatalf("selectBackend failed with forced x11: %v", err)
	}
	if _, ok := backend.(*linuxApplication); !ok {
		t.Fatalf("expected *linuxApplication, got %T", backend)
	}

	// Forced x11 with no DISPLAY -> ErrNoGraphics
	t.Setenv("DISPLAY", "")
	_, err = selectBackend()
	if !errors.Is(err, platform.ErrNoGraphics) {
		t.Fatalf("expected ErrNoGraphics when DISPLAY unset, got %v", err)
	}
}

func TestSelectBackendForcedWaylandFailure(t *testing.T) {
	t.Setenv("VIVA_LINUX_BACKEND", "wayland")
	t.Setenv("WAYLAND_DISPLAY", "nonexistent-wayland-socket")
	t.Setenv("XDG_RUNTIME_DIR", "/nonexistent-runtime-dir")
	t.Setenv("DISPLAY", ":0") // DISPLAY is present, but should NOT fallback to X11!

	connectErr := errors.New("Wayland display connection failed")
	_, err := selectBackendWith(func() (*waylandApplication, error) {
		return nil, connectErr
	}, func(string, ...any) {
		t.Fatal("forced Wayland failure reported an X11 fallback")
	})
	if !errors.Is(err, connectErr) || !strings.Contains(err.Error(), "forced Wayland backend failed") {
		t.Fatalf("expected forced Wayland failure preserving its cause without fallback, got %v", err)
	}
}

func TestSelectBackendInvalidName(t *testing.T) {
	t.Setenv("VIVA_LINUX_BACKEND", "mir")

	_, err := selectBackend()
	if err == nil || !strings.Contains(err.Error(), "unknown VIVA_LINUX_BACKEND") {
		t.Fatalf("expected unknown backend error, got %v", err)
	}
}

func TestSelectBackendNoDisplayEnvironment(t *testing.T) {
	t.Setenv("VIVA_LINUX_BACKEND", "")
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("XDG_RUNTIME_DIR", "/nonexistent-dir")
	t.Setenv("DISPLAY", "")

	backend, err := selectBackend()
	if backend != nil {
		t.Fatalf("expected nil backend, got %v", backend)
	}
	if !errors.Is(err, platform.ErrNoGraphics) {
		t.Fatalf("expected ErrNoGraphics, got %v", err)
	}
}

func TestSelectBackendFallbackToX11WhenWaylandAbsent(t *testing.T) {
	t.Setenv("VIVA_LINUX_BACKEND", "")
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("XDG_RUNTIME_DIR", "/nonexistent-dir")
	t.Setenv("DISPLAY", ":0")

	backend, err := selectBackend()
	if err != nil {
		t.Fatalf("expected successful fallback to X11, got err: %v", err)
	}
	if _, ok := backend.(*linuxApplication); !ok {
		t.Fatalf("expected *linuxApplication on fallback, got %T", backend)
	}
}

func TestSelectBackendAutomaticFailureWithoutX11PreservesCause(t *testing.T) {
	t.Setenv("VIVA_LINUX_BACKEND", "")
	t.Setenv("WAYLAND_DISPLAY", "wayland-test")
	t.Setenv("DISPLAY", "")

	connectErr := errors.New("Wayland display connection failed")
	backend, err := selectBackendWith(func() (*waylandApplication, error) {
		return nil, connectErr
	}, func(string, ...any) {
		t.Fatal("failure without X11 reported a successful fallback")
	})
	if backend != nil {
		t.Fatalf("expected no backend, got %T", backend)
	}
	if !errors.Is(err, connectErr) {
		t.Fatalf("expected wrapped Wayland connection error, got %v", err)
	}
	if errors.Is(err, platform.ErrNoGraphics) {
		t.Fatalf("connection failure was misreported as no graphics: %v", err)
	}
}

func TestSelectBackendAutomaticFallbackReportsWaylandCause(t *testing.T) {
	t.Setenv("VIVA_LINUX_BACKEND", "")
	t.Setenv("WAYLAND_DISPLAY", "wayland-test")
	t.Setenv("DISPLAY", ":0")

	connectErr := errors.New("Wayland display connection failed")
	var diagnostic string
	backend, err := selectBackendWith(func() (*waylandApplication, error) {
		return nil, connectErr
	}, func(format string, args ...any) {
		diagnostic = fmt.Sprintf(format, args...)
	})
	if err != nil {
		t.Fatalf("fallback to X11 failed: %v", err)
	}
	if _, ok := backend.(*linuxApplication); !ok {
		t.Fatalf("expected *linuxApplication on fallback, got %T", backend)
	}
	if !strings.Contains(diagnostic, connectErr.Error()) || !strings.Contains(diagnostic, "falling back to X11") {
		t.Fatalf("fallback diagnostic %q does not preserve Wayland cause %q", diagnostic, connectErr)
	}
}

func TestWaylandRunScopesDesktopServicesAndPreservesDeviceError(t *testing.T) {
	platform.SetWatchHook(nil)
	platform.SetAppearanceObserver(nil)
	t.Cleanup(func() {
		platform.SetWatchHook(nil)
		platform.SetAppearanceObserver(nil)
	})

	app := &waylandApplication{}
	if app.watchHookInstalled || activeWaylandApp.Load() == app {
		t.Fatal("unstarted Wayland application owns global runtime state")
	}
	deviceErr := errors.New("Vulkan device initialization failed")
	err := app.Run(func(platform.Application) error {
		// OpenWindow returns Vulkan/device failures through this startup callback.
		if !app.watchHookInstalled || activeWaylandApp.Load() != app {
			t.Fatal("running Wayland application did not acquire global runtime state")
		}
		if app.desktop != nil {
			t.Fatal("desktop watcher started without an appearance observer")
		}
		return deviceErr
	})
	if !errors.Is(err, deviceErr) || errors.Is(err, platform.ErrNoGraphics) {
		t.Fatalf("startup device error chain = %v, want original device error", err)
	}
	if app.watchHookInstalled || activeWaylandApp.Load() == app {
		t.Fatal("closed Wayland application retained global runtime state")
	}
}

func TestWaylandCloseBeforeRunPreservesExistingWatchHook(t *testing.T) {
	platform.SetAppearanceObserver(nil)
	calls := 0
	platform.SetWatchHook(func() { calls++ })
	t.Cleanup(func() {
		platform.SetWatchHook(nil)
		platform.SetAppearanceObserver(nil)
	})

	app := &waylandApplication{}
	app.close()
	platform.SetAppearanceObserver(nil)
	if calls != 1 {
		t.Fatalf("initialization cleanup replaced an existing watch hook: calls=%d, want 1", calls)
	}
}
