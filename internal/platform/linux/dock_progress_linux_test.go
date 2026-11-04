//go:build linux

package linux

import (
	"testing"

	"github.com/vivaui/viva/internal/dbus"
)

// readLauncherProperties reads the Update signal's body the way a desktop
// would: the application's entry, then the properties that changed.
func readLauncherProperties(t *testing.T, body []byte) (entry string, properties map[string]any) {
	t.Helper()
	args := dbus.NewDecoder(body)
	entry = args.String()
	properties = map[string]any{}
	end := args.ArrayLength(8)
	for args.Pos() < end {
		args.Align(8)
		key := args.String()
		switch signature := args.Signature(); signature {
		case "b":
			properties[key] = args.Bool()
		case "d":
			properties[key] = args.Float64()
		default:
			t.Fatalf("property %q has signature %q", key, signature)
		}
	}
	if err := args.Err(); err != nil {
		t.Fatalf("body: %v", err)
	}
	return entry, properties
}

func TestLauncherEntryBodyNamesTheDesktopFileAndTheFraction(t *testing.T) {
	entry, properties := readLauncherProperties(t, launcherEntryBody("motty.desktop", true, 0.25))
	if entry != "application://motty.desktop" {
		t.Fatalf("entry = %q", entry)
	}
	if visible, ok := properties["progress-visible"].(bool); !ok || !visible {
		t.Fatalf("progress-visible = %v", properties["progress-visible"])
	}
	if progress, ok := properties["progress"].(float64); !ok || progress != 0.25 {
		t.Fatalf("progress = %v", properties["progress"])
	}
}

// A hidden bar says so rather than saying zero: a bar at zero is a bar, and a
// launcher would draw its start rather than nothing.
func TestHiddenLauncherEntryHidesTheBarRatherThanEmptyingIt(t *testing.T) {
	_, properties := readLauncherProperties(t, launcherEntryBody("motty.desktop", false, 0))
	if visible, ok := properties["progress-visible"].(bool); !ok || visible {
		t.Fatalf("progress-visible = %v", properties["progress-visible"])
	}
	if progress, ok := properties["progress"].(float64); !ok || progress != 0 {
		t.Fatalf("progress = %v", properties["progress"])
	}
}

func TestLauncherEntryNamesTheDesktopFileAfterTheWindowClass(t *testing.T) {
	entry := launcherEntry{class: "dev.motty.app"}
	if id := entry.desktopID(); id != "dev.motty.app.desktop" {
		t.Fatalf("desktop id = %q", id)
	}
}
