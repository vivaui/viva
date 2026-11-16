package linux

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/dbus"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

func TestAtspiNativeResizePublishesFrames(t *testing.T) {
	if os.Getenv("VIVA_ATSPI_NATIVE") != "1" {
		t.Skip("set VIVA_ATSPI_NATIVE=1 with a private X11/AT-SPI session")
	}
	root := os.Getenv("VIVA_ATSPI_NATIVE_ROOT")
	if root == "" || !filepath.IsAbs(root) || !strings.HasPrefix(os.Getenv("HOME"), root+string(os.PathSeparator)) || !strings.HasPrefix(os.Getenv("XDG_CONFIG_HOME"), root+string(os.PathSeparator)) {
		t.Fatal("native accessibility tests require a private HOME/XDG profile")
	}
	t.Setenv("VIVA_ACCESSIBILITY", "")
	t.Setenv("VIVA_SOFTWARE_RENDERING", "1")
	t.Setenv("VIVA_SCALE", "1")
	session, err := dbus.DialSession()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	restoreA11yStatus(t, session, "IsEnabled")
	var status dbus.Encoder
	status.String(a11yStatus)
	status.String("IsEnabled")
	status.Variant("b")
	status.Bool(true)
	if _, err := session.Call(a11yBusName, a11yBusPath, dbusProperties, "Set", "ssv", status.Bytes()); err != nil {
		t.Fatal(err)
	}
	address, err := atspiAddress(session)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name     string
		children int
		extent   [4]int32
	}{
		{"changed", 1, [4]int32{0, 0, 640, 400}},
		{"nil preserves", 1, [4]int32{0, 0, 1280, 800}},
		{"empty clears", 0, [4]int32{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			resized := make(chan platform.Size, 1)
			result := make(chan error, 1)
			config := platform.Config{
				Title: "AT-SPI resize", Width: 1280, Height: 800, WindowMode: platform.WindowFrameless,
				ClearColor:    platform.Color{Alpha: 1},
				Rectangles:    []render.Rectangle{{Kind: scene.KindRectangle, Bounds: scene.Rect{Width: 1280, Height: 800}, Color: scene.Color{Green: .5, Alpha: 1}}},
				Accessibility: []platform.AccessibilityNode{{ID: gfx.Item(1), Role: platform.A11yRoleGroup, Label: "Content", Width: 1280, Height: 800}},
			}
			config.OnResize = func(size platform.Size) (platform.Frame, error) {
				if size.Width != 640 || size.Height != 400 {
					return platform.Frame{}, nil
				}
				select {
				case resized <- size:
				default:
				}
				switch test.name {
				case "nil preserves":
					return platform.Frame{}, nil
				case "empty clears":
					return platform.Frame{Accessibility: []platform.AccessibilityNode{}}, nil
				default:
					return platform.Frame{Accessibility: []platform.AccessibilityNode{{ID: gfx.Item(1), Role: platform.A11yRoleGroup, Label: "Content", Width: size.Width, Height: size.Height}}}, nil
				}
			}
			err := RunApplication(func(application platform.Application) error {
				control, err := application.OpenWindow(config)
				if err != nil {
					return err
				}
				state := control.(*nativeWindow).state
				if _, ok := state.renderer.(*cpuWindowRenderer); !ok {
					return fmt.Errorf("native regression requires the CPU window renderer")
				}
				host := state.application.atspi
				if host == nil || host.bridge == nil || host.windows[state] == nil {
					return fmt.Errorf("the real window did not join the private accessibility bridge")
				}
				name, frame := host.bridge.unique, host.bridge.framePath(host.windows[state])
				go func() {
					result <- atspiProbeResize(address, name, frame, control.(platform.WindowSizer), resized, test.children, test.extent)
					application.Quit()
				}()
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := <-result; err != nil {
				t.Fatal(err)
			}
		})
	}
}

func atspiProbeResize(address, name, frame string, window platform.WindowSizer, resized <-chan platform.Size, children int, extent [4]int32) error {
	client, err := dbus.Dial(address)
	if err != nil {
		return err
	}
	defer client.Close()
	initial, err := atspiNativeChildren(client, name, frame)
	if err != nil {
		return err
	}
	if len(initial) != 1 {
		return fmt.Errorf("initial accessibility children = %d; want 1", len(initial))
	}
	if got, err := atspiNativeExtent(client, initial[0]); err != nil || got != ([4]int32{0, 0, 1280, 800}) {
		return fmt.Errorf("initial accessibility extent = %v/%v; want [0 0 1280 800]", got, err)
	}
	if err := window.SetSize(640, 400); err != nil {
		return err
	}
	select {
	case <-resized:
	case <-time.After(5 * time.Second):
		return fmt.Errorf("the actual X11 resize did not reach OnResize")
	}
	next, err := atspiNativeChildren(client, name, frame)
	if err != nil {
		return err
	}
	if len(next) != children {
		return fmt.Errorf("resized accessibility children = %d; want %d", len(next), children)
	}
	if children != 0 {
		if got, err := atspiNativeExtent(client, next[0]); err != nil || got != extent {
			return fmt.Errorf("resized accessibility extent = %v/%v; want %v", got, err, extent)
		}
	}
	return nil
}

func atspiNativeChildren(client *dbus.Conn, name, frame string) ([]atspiRef, error) {
	reply, err := client.Call(name, frame, atspiIfaceAccessible, "GetChildren", "", nil)
	if err != nil {
		return nil, err
	}
	defer reply.CloseFDs()
	args := reply.Args()
	end := args.ArrayLength(8)
	var children []atspiRef
	for args.Pos() < end && args.Err() == nil {
		children = append(children, readRef(args))
	}
	return children, args.Err()
}

func atspiNativeExtent(client *dbus.Conn, node atspiRef) ([4]int32, error) {
	var request dbus.Encoder
	request.Uint32(1) // window coordinates
	reply, err := client.Call(node.Name, node.Path, atspiIfaceComponent, "GetExtents", "u", request.Bytes())
	if err != nil {
		return [4]int32{}, err
	}
	defer reply.CloseFDs()
	args := reply.Args()
	args.Align(8)
	extent := [4]int32{args.Int32(), args.Int32(), args.Int32(), args.Int32()}
	return extent, args.Err()
}
