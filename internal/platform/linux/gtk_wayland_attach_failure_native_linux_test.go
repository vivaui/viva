//go:build linux && vivagtk

package linux

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/internal/platform"
)

const gtkAttachFaultMarker = "GDK content-window attach fault injected"

func TestGTKNativeWaylandAttachFailure(t *testing.T) {
	if os.Getenv("VIVA_GTK_ATTACH_NATIVE") != "1" || os.Getenv("VIVA_FCITX_BACKEND") != "wayland" {
		t.Skip("requires the private Wayland GTK attach-failure fixture")
	}
	if os.Getenv("VIVA_GTK_ATTACH_CHILD") == "1" {
		gtkAttachFailureChild(t)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestGTKNativeWaylandAttachFailure$", "-test.v", "-test.timeout=15s")
	cmd.Env = append(os.Environ(), "VIVA_GTK_ATTACH_CHILD=1")
	output, err := cmd.CombinedOutput()
	if !strings.Contains(string(output), gtkAttachFaultMarker) {
		t.Fatalf("native GDK content fault was not injected; child error=%v, output=%.1200s", err, output)
	}
	if ctx.Err() != nil {
		t.Fatalf("OpenWindow/GTK loop hung after the injected content fault: %v; output=%.1200s", ctx.Err(), output)
	}
	if err != nil {
		t.Fatalf("native content attach-failure contract: %v; output=%.1200s", err, output)
	}
}

func gtkAttachFailureChild(t *testing.T) {
	var original func(uintptr, *gdkWindowAttr, int32) uintptr
	var library *gtkLibrary
	var openErr error
	var window platform.Window
	faults := 0
	runErr := RunGTKApplication(func(application platform.Application) error {
		a := application.(*gtkApplication)
		if err := a.initialize("Viva attach failure"); err != nil {
			return err
		}
		if !a.wayland {
			return fmt.Errorf("native attach-failure test needs GdkWaylandDisplay")
		}
		library, original = a.g, a.g.gdkWindowNew
		a.g.gdkWindowNew = func(parent uintptr, attrs *gdkWindowAttr, mask int32) uintptr {
			if attrs.WindowType == 6 { // GDK_WINDOW_SUBSURFACE
				faults++
				if faults == 1 {
					fmt.Fprintln(os.Stdout, gtkAttachFaultMarker)
				}
				return 0
			}
			return original(parent, attrs, mask)
		}
		window, openErr = application.OpenWindow(platform.Config{
			Title: "Viva attach-failure probe", Width: 240, Height: 160,
			ClearColor: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
			OnRedraw:   func() (platform.Frame, error) { return platform.Frame{}, nil },
		})
		return openErr
	})
	if library != nil {
		library.gdkWindowNew = original
	}
	if faults != 1 {
		t.Errorf("content-window fault calls=%d, want 1", faults)
	}
	if window != nil || openErr == nil {
		t.Errorf("OpenWindow returned window=%t error=%v after GDK content failure", window != nil, openErr)
	}
	if openErr != nil && !strings.Contains(openErr.Error(), "GDK Wayland content window") {
		t.Errorf("OpenWindow error=%v, want a content-window creation error", openErr)
	}
	if runErr == nil || runErr != openErr {
		t.Errorf("RunGTKApplication error=%v, want OpenWindow error=%v", runErr, openErr)
	}
}
