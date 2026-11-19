package app

import (
	"sync"
	"sync/atomic"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/window"
)

// Appearance is the look the system is set to: light or dark.
type Appearance = window.Appearance

const (
	// AppearanceUnknown means the system does not say: a desktop without a
	// preference, or a platform that cannot be asked.
	AppearanceUnknown = window.AppearanceUnknown
	AppearanceLight   = window.AppearanceLight
	AppearanceDark    = window.AppearanceDark
)

// SystemAppearance reports whether the system is set to a light or a dark look,
// so an application can follow it until the user picks one. It reads the setting
// when called and is cheap enough for a frame, but a change is only heard
// through WatchSystemAppearance.
//
// macOS reports the application's effective appearance, Windows the "choose your
// default app mode" setting, the browser prefers-color-scheme, and Linux the
// desktop portal's org.freedesktop.appearance color-scheme, then, without one,
// the XSETTINGS manager, GTK_THEME and the GTK and KDE configuration files
// (unknown when none says). A headless Driver reports what SetSystemAppearance
// last set, unknown at first, so a test never depends on the machine it runs on.
func SystemAppearance() Appearance {
	if appearance, driven := driverSystemAppearance(); driven {
		return appearance
	}
	return nativeAppearance()
}

// SetNativeAppearance pins the look of what the system draws for the
// application, as opposed to what the application draws itself: window buttons,
// native menus, file dialogs, native text editing. An application that lets the
// user choose light or dark regardless of the system calls it when that choice
// changes, so those parts match the window; AppearanceUnknown gives them back to
// the system. SystemAppearance keeps reporting the system's own look meanwhile.
//
// macOS applies it. Windows and Linux draw the window's buttons with Viva, so
// there is nothing for it to do there, and the browser's page colors are the
// application's own.
func SetNativeAppearance(appearance Appearance) {
	if driven := recordDriverNativeAppearance(appearance); driven {
		return
	}
	nativeSetAppearance(appearance)
}

// WatchSystemAppearance calls changed whenever the system switches between its
// light and dark looks, or turns high contrast on or off (HighContrast), and not
// when it merely says so again, until stop is
// called. changed runs on the event-loop thread: it should note the change and
// request a redraw (see Invalidator), then read SystemAppearance while painting.
//
// macOS and the browser are told of the switch, Windows hears the settings
// change message, and Linux listens to the settings portal's signal on one
// connection to the session bus that the event loop polls, open only while
// something watches (no thread), plus the XSETTINGS manager's change and the
// window gaining focus for desktops without a portal.
func WatchSystemAppearance(changed func()) (stop func()) {
	if changed == nil {
		return func() {}
	}
	appearanceWatchers.Lock()
	appearanceWatchers.nextID++
	id := appearanceWatchers.nextID
	appearanceWatchers.list = append(appearanceWatchers.list, appearanceWatcher{id: id, changed: changed})
	first := len(appearanceWatchers.list) == 1
	appearanceWatchers.Unlock()
	if first {
		appearanceWatchers.last.Store(appearanceState())
		platform.SetAppearanceObserver(appearanceMaybeChanged)
	}
	if _, driven := driverSystemAppearance(); !driven {
		// A driver-backed run has no system to ask, and a second window made
		// there would wait on a main thread that is not running its loop.
		nativeWatchAppearance()
	}
	return sync.OnceFunc(func() {
		appearanceWatchers.Lock()
		for index, watcher := range appearanceWatchers.list {
			if watcher.id == id {
				appearanceWatchers.list = append(appearanceWatchers.list[:index], appearanceWatchers.list[index+1:]...)
				break
			}
		}
		none := len(appearanceWatchers.list) == 0
		appearanceWatchers.Unlock()
		if none {
			platform.SetAppearanceObserver(nil)
		}
	})
}

type appearanceWatcher struct {
	id      uint64
	changed func()
}

var appearanceWatchers struct {
	sync.Mutex
	nextID uint64
	list   []appearanceWatcher
	last   atomic.Uint32 // the appearance the watchers were last told of
}

// appearanceMaybeChanged is what an adapter's notification comes to: it reads the
// setting and tells the watchers only if it is not what they were last told.
func appearanceMaybeChanged() {
	current := appearanceState()
	if appearanceWatchers.last.Swap(current) == current {
		return
	}
	appearanceWatchers.Lock()
	watchers := append([]appearanceWatcher(nil), appearanceWatchers.list...)
	appearanceWatchers.Unlock()
	for _, watcher := range watchers {
		watcher.changed()
	}
}

// driverAppearance is the system appearance of headless runs: while any Driver is
// open, or a tool runner is installed, SystemAppearance answers from here and not
// from the machine.
var driverAppearance struct {
	sync.Mutex
	open   int
	value  Appearance
	native Appearance // what SetNativeAppearance last asked for
}

// recordDriverNativeAppearance notes the request of an application under a
// Driver, which never touches the real system, and reports whether there is one.
func recordDriverNativeAppearance(appearance Appearance) bool {
	driverAppearance.Lock()
	defer driverAppearance.Unlock()
	if driverAppearance.open == 0 && !runnerReplaced() {
		return false
	}
	driverAppearance.native = appearance
	return true
}

func driverSystemAppearance() (Appearance, bool) {
	driverAppearance.Lock()
	defer driverAppearance.Unlock()
	return driverAppearance.value, driverAppearance.open > 0 || runnerReplaced()
}

// runnerReplaced is whether a tool build has put its own runner in place of the
// native window loop (UseRunner, UseApplicationRunner). Such a process has no
// system to ask from its first line on, before any Driver exists: an application
// reads and pins the appearance while it builds its window, and a call to the
// native side there waits on a main thread that nobody runs.
func runnerReplaced() bool { return runner != nil || applicationRunner != nil }

func enterDriverAppearance() {
	liveDriverAppearance.Acquire()
	driverAppearance.Lock()
	driverAppearance.open++
	driverAppearance.Unlock()
}

func leaveDriverAppearance() {
	liveDriverAppearance.Release()
	driverAppearance.Lock()
	driverAppearance.open--
	if driverAppearance.open == 0 {
		driverAppearance.value = AppearanceUnknown
		driverAppearance.native = AppearanceUnknown
	}
	driverAppearance.Unlock()
}

// appearanceOf turns an adapter's answer, whether it is dark and whether it
// knows, into an Appearance.
func appearanceOf(dark, known bool) Appearance {
	switch {
	case !known:
		return AppearanceUnknown
	case dark:
		return AppearanceDark
	}
	return AppearanceLight
}
