package linux

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/internal/dbus"
	"github.com/vivaui/viva/internal/platform"
)

type atspiInitialPollHost struct {
	fakeBusHost
	services    *desktopServices
	descriptors []int
}

func (host *atspiInitialPollHost) rebuildPoll() {
	host.descriptors = append(host.descriptors, host.services.atspi.fd())
}

type atspiInitialPollShell struct{ config platform.Config }

// restoreA11yStatus puts the session's org.a11y.Status values back when the
// test ends. These tests switch accessibility on; an opt-in run on a developer
// session must not leave the desktop's settings changed.
func restoreA11yStatus(t *testing.T, connection *dbus.Conn, names ...string) {
	t.Helper()
	values := make(map[string]bool, len(names))
	for _, name := range names {
		var request dbus.Encoder
		request.String(a11yStatus)
		request.String(name)
		reply, err := connection.Call(a11yBusName, a11yBusPath, dbusProperties, dbusPropertiesGet, "ss", request.Bytes())
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		args := reply.Args()
		if args.Signature() != "b" {
			t.Fatalf("read %s: signature %q", name, args.Signature())
		}
		values[name] = args.Bool()
	}
	t.Cleanup(func() {
		for _, name := range names {
			var request dbus.Encoder
			request.String(a11yStatus)
			request.String(name)
			request.Variant("b")
			request.Bool(values[name])
			if _, err := connection.Call(a11yBusName, a11yBusPath, dbusProperties, "Set", "ssv", request.Bytes()); err != nil {
				t.Errorf("restore %s: %v", name, err)
			}
		}
	})
}

func (shell *atspiInitialPollShell) atspiConfig() *platform.Config { return &shell.config }
func (*atspiInitialPollShell) atspiGeometry() (int, int, float32)  { return 0, 0, 1 }
func (*atspiInitialPollShell) atspiPresent(platform.Frame) error   { return nil }

// The first window must publish the bridge or watcher before its loop rebuilds
// the descriptor set. A live connection which nobody polls cannot serve readers
// or notice that accessibility was enabled later.
func TestAtspiNativeFirstWindowPolling(t *testing.T) {
	if os.Getenv("VIVA_ATSPI_NATIVE") != "1" {
		t.Skip("set VIVA_ATSPI_NATIVE=1 with a private AT-SPI session bus")
	}
	root := os.Getenv("VIVA_ATSPI_NATIVE_ROOT")
	if root == "" || !filepath.IsAbs(root) || !strings.HasPrefix(os.Getenv("XDG_CONFIG_HOME"), root+string(os.PathSeparator)) {
		t.Fatal("native accessibility tests require a private HOME/XDG profile")
	}
	t.Setenv("VIVA_ACCESSIBILITY", "")
	connection, err := dbus.DialSession()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { connection.Close() })
	restoreA11yStatus(t, connection, "IsEnabled", "ScreenReaderEnabled")
	setStatus := func(name string, value bool) {
		t.Helper()
		var request dbus.Encoder
		request.String(a11yStatus)
		request.String(name)
		request.Variant("b")
		request.Bool(value)
		if _, err := connection.Call(a11yBusName, a11yBusPath, dbusProperties, "Set", "ssv", request.Bytes()); err != nil {
			t.Fatal(err)
		}
	}
	for _, enabled := range []bool{true, false} {
		name := "watcher"
		if enabled {
			name = "bridge"
		}
		t.Run(name, func(t *testing.T) {
			setStatus("ScreenReaderEnabled", false)
			setStatus("IsEnabled", enabled)
			services := &desktopServices{}
			owner := &atspiInitialPollHost{services: services}
			services.host = owner
			t.Cleanup(services.atspiClose)
			services.atspiOpened(&atspiInitialPollShell{config: platform.Config{Title: "First accessibility window"}})
			if services.atspi == nil || services.atspi.fd() < 0 {
				t.Fatal("the real private accessibility connection did not open")
			}
			if enabled && services.atspi.bridge == nil || !enabled && services.atspi.watcher == nil {
				t.Fatal("the native connection did not use the requested bridge/watcher mode")
			}
			fd := services.atspi.fd()
			if len(owner.descriptors) == 0 || owner.descriptors[len(owner.descriptors)-1] != fd {
				t.Fatalf("first window opened descriptor %d but the loop observed %v", fd, owner.descriptors)
			}
		})
	}
	t.Run("forced off", func(t *testing.T) {
		t.Setenv("VIVA_ACCESSIBILITY", "0")
		services := &desktopServices{}
		owner := &atspiInitialPollHost{services: services}
		services.host = owner
		services.atspiOpened(&atspiInitialPollShell{})
		if services.atspi != nil || len(owner.descriptors) != 0 {
			t.Fatal("forced-off accessibility created a host or changed polling")
		}
	})
	t.Run("no service", func(t *testing.T) {
		t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+filepath.Join(root, "absent-bus"))
		services := &desktopServices{}
		owner := &atspiInitialPollHost{services: services}
		services.host = owner
		t.Cleanup(services.atspiClose)
		services.atspiOpened(&atspiInitialPollShell{})
		if services.atspi.fd() != -1 || len(owner.descriptors) != 0 {
			t.Fatal("an unavailable accessibility service left a polled descriptor")
		}
	})
}
