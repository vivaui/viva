//go:build linux

package linux

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/vivaui/viva/internal/platform"
)

type platformBackend interface {
	platform.Application
	Run(start platform.ApplicationStart) error
}

func selectBackend() (platformBackend, error) {
	return selectBackendWith(newWaylandApplication, log.Printf)
}

func selectBackendWith(newWayland func() (*waylandApplication, error), logf func(string, ...any)) (platformBackend, error) {
	target := os.Getenv("VIVA_LINUX_BACKEND")

	switch target {
	case "wayland":
		waylandApp, err := newWayland()
		if err != nil {
			return nil, fmt.Errorf("linux: forced Wayland backend failed: %w", err)
		}
		return waylandApp, nil

	case "x11":
		if os.Getenv("DISPLAY") == "" {
			return nil, platform.ErrNoGraphics
		}
		return &linuxApplication{}, nil

	case "":
		// Automatic probing: prefer Wayland, fallback to X11.
		var waylandErr error
		if hasWaylandEnvironment() {
			waylandApp, err := newWayland()
			if err == nil {
				return waylandApp, nil
			}
			waylandErr = err
		}

		if os.Getenv("DISPLAY") != "" {
			if waylandErr != nil {
				logf("linux: Wayland backend initialization failed; falling back to X11: %v", waylandErr)
			}
			return &linuxApplication{}, nil
		}
		if waylandErr != nil {
			return nil, fmt.Errorf("linux: Wayland backend initialization failed and X11 is unavailable: %w", waylandErr)
		}

		return nil, platform.ErrNoGraphics

	default:
		return nil, fmt.Errorf("linux: unknown VIVA_LINUX_BACKEND %q (must be 'wayland' or 'x11')", target)
	}
}

func hasWaylandEnvironment() bool {
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		return true
	}
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir != "" {
		socketPath := filepath.Join(runtimeDir, "wayland-0")
		if _, err := os.Stat(socketPath); err == nil {
			return true
		}
	}
	return false
}
