package linux

import (
	"errors"
	"testing"

	"github.com/vivaui/viva/internal/dbus"
	"github.com/vivaui/viva/internal/platform"
)

// These tests talk to the session bus of the machine they run on and skip without one. They
// check the messages against a real portal and notification service, which a recording of
// them cannot.

func liveBus(t *testing.T) *dbus.Conn {
	t.Helper()
	conn, err := dbus.DialSession()
	if err != nil {
		t.Skipf("no session bus: %v", err)
	}
	t.Cleanup(conn.Close)
	return conn
}

func TestThePortalAnswersTheSettingsVivaReads(t *testing.T) {
	values, result := askPortal()
	if result != portalReadOK {
		t.Skip("no settings portal on this machine")
	}
	t.Logf("portal: %+v", values)
	if dark, known := values.dark(); known {
		t.Logf("dark = %v", dark)
	}
}

func TestTheNotificationServiceTakesOurNotification(t *testing.T) {
	liveBus(t)
	err := notifyOverBus("viva-test", "Viva test notification", "A <b>body</b> & more")
	if errors.Is(err, errServiceMissing) {
		t.Skipf("no notification service: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func TestTheDesktopConnectionExistsOnlyWhileSomeoneWatches(t *testing.T) {
	liveBus(t)
	resetDesktopState(t)
	application := &linuxApplication{}
	application.syncDesktopWatch()
	if application.desktop != nil {
		t.Fatal("a connection was opened with nobody watching")
	}
	platform.SetAppearanceObserver(func() {})
	t.Cleanup(func() { platform.SetAppearanceObserver(nil) })
	application.syncDesktopWatch()
	if application.desktop == nil {
		t.Skip("no settings portal to watch on this machine")
	}
	if len(application.pollFDs) != 1 || int(application.pollFDs[0].FD) != application.desktop.conn.FD() {
		t.Fatalf("the loop polls %v, not the desktop connection", application.pollFDs)
	}
	desktopState.Lock()
	watching := desktopState.watching
	desktopState.Unlock()
	if !watching {
		t.Fatal("the answers are not marked current while the connection delivers changes")
	}
	platform.SetAppearanceObserver(nil)
	application.syncDesktopWatch()
	if application.desktop != nil || len(application.pollFDs) != 0 {
		t.Fatal("the connection outlived the last watcher")
	}
}

func TestAChangeIsReportedToWhoWatches(t *testing.T) {
	var appearance, desktop int
	var seen platform.DesktopSetting
	platform.SetAppearanceObserver(func() { appearance++ })
	platform.SetDesktopObserver(func(changed platform.DesktopSetting) { desktop++; seen |= changed })
	t.Cleanup(func() {
		platform.SetAppearanceObserver(nil)
		platform.SetDesktopObserver(nil)
	})
	application := &linuxApplication{}
	application.announce(0)
	application.announce(changedAppearance)
	if appearance != 1 || desktop != 0 {
		t.Fatalf("a look change reached %d appearance and %d desktop observers", appearance, desktop)
	}
	application.announce(changedMotion | changedFonts)
	if appearance != 1 || desktop != 1 || seen != platform.SettingMotion|platform.SettingFonts {
		t.Fatalf("appearance %d desktop %d saw %v", appearance, desktop, seen)
	}
}
