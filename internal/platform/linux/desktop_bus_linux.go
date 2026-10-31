package linux

import (
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/vivaui/viva/internal/dbus"
	"github.com/vivaui/viva/internal/platform"
)

// The desktop's look, its motion preference and its fonts change while an application
// runs, and the application hears of it without a thread of its own: while somebody
// watches (platform.DesktopWatched) the event loop polls one more connection, to the
// session bus, along with the windows' X connections; the XSETTINGS manager's
// PropertyNotify, which the windows already hear, covers the desktops without the settings
// portal. Nobody watching means no connection and no wake-up.

// desktopConnection is the watcher-owned session bus connection.
type desktopConnection interface {
	portalCaller
	Receive() ([]*dbus.Message, error)
	FD() int
	Close()
}

var dialDesktop = func() (desktopConnection, error) { return dbus.DialSession() }

// desktopWatch is the connection to the session bus that delivers the portal's changes.
type desktopWatch struct {
	conn desktopConnection
}

const (
	desktopRetryMin = 100 * time.Millisecond
	desktopRetryMax = 30 * time.Second
)

// desktopRetry belongs to one event loop. Its deadline is included in that
// loop's poll timeout; no timer goroutine or repeated Dial is needed.
type desktopRetry struct {
	failures uint
	next     time.Time
}

func (retry *desktopRetry) ready(now time.Time) bool {
	return retry.next.IsZero() || !now.Before(retry.next)
}

func (retry *desktopRetry) fail(now time.Time) {
	delay := desktopRetryMin << min(retry.failures, 9)
	if delay > desktopRetryMax {
		delay = desktopRetryMax
	}
	if retry.failures < 9 {
		retry.failures++
	}
	retry.next = now.Add(delay)
}

func (retry *desktopRetry) reset() {
	retry.failures = 0
	retry.next = time.Time{}
}

func (retry *desktopRetry) deadline(wanted, connected bool) (time.Time, bool) {
	if !wanted || connected || retry.next.IsZero() || desktopPortalGone() {
		return time.Time{}, false
	}
	return retry.next, true
}

func desktopPortalGone() bool {
	desktopState.Lock()
	defer desktopState.Unlock()
	return desktopState.portalGone
}

func (application *linuxApplication) timeNow() time.Time {
	if application.now != nil {
		return application.now()
	}
	return time.Now()
}

// syncDesktopWatch opens the connection when someone starts to watch and closes it when
// the last one stops. Temporary failures wait for their bounded retry deadline.
func (application *linuxApplication) syncDesktopWatch() {
	if application.desktopRetryRequested.Swap(false) {
		application.desktopRetry.reset()
	}
	wanted := platform.DesktopWatched()
	if !wanted {
		if application.desktop != nil {
			application.stopDesktopWatch()
		}
		application.desktopRetry.reset()
		return
	}
	if application.desktop == nil && application.desktopRetry.ready(application.timeNow()) {
		application.startDesktopWatch()
	}
}

// retryDesktopWatch lets an observer re-registration or a new window bypass a
// temporary backoff. A permanently unsupported portal remains latched.
func (application *linuxApplication) retryDesktopWatch() {
	application.desktopRetry.reset()
	application.syncDesktopWatch()
}

func (application *linuxApplication) startDesktopWatch() {
	if desktopPortalGone() {
		application.desktopRetry.reset()
		return
	}
	conn, err := dialDesktop()
	if err == nil {
		err = addMatch(conn, settingsMatch)
	}
	if err != nil {
		if conn != nil {
			conn.Close()
		}
		application.desktopRetry.fail(application.timeNow())
		return
	}
	values, result := readPortalValues(conn)
	if result != portalReadOK {
		conn.Close()
		if result == portalReadUnsupported {
			desktopState.Lock()
			desktopState.portalGone = true
			desktopState.Unlock()
			application.desktopRetry.reset()
		} else {
			application.desktopRetry.fail(application.timeNow())
		}
		return
	}
	change := replacePortal(values, os.Getenv)
	desktopState.Lock()
	desktopState.watching = true
	desktopState.Unlock()
	application.desktopRetry.reset()
	application.desktop = &desktopWatch{conn: conn}
	application.rebuildPoll()
	application.announce(change)
}

func (application *linuxApplication) stopDesktopWatch() {
	watch := application.desktop
	if watch == nil {
		return
	}
	application.desktop = nil
	application.rebuildPoll()
	watch.conn.Close()
	desktopState.Lock()
	desktopState.watching = false
	desktopState.Unlock()
}

// serveDesktop reads what the bus sent: the portal saying a setting changed.
func (application *linuxApplication) serveDesktop() {
	watch := application.desktop
	if watch == nil {
		return
	}
	messages, err := watch.conn.Receive()
	if err != nil {
		application.stopDesktopWatch()
		application.desktopRetry.fail(application.timeNow())
		return
	}
	var change settingsChange
	for _, message := range messages {
		if message.Type != dbus.TypeSignal || message.Member != "SettingChanged" || message.Signature != "ssv" {
			continue
		}
		if namespace, key, value, ok := parseSettingChanged(message.Body); ok {
			change |= applyPortal(namespace, key, value, os.Getenv)
		}
	}
	application.announce(change)
}

// announce tells what a change in the desktop's settings touches: the windows' text, and
// whoever watches.
func (application *linuxApplication) announce(change settingsChange) {
	if change == 0 {
		return
	}
	if change&changedFonts != 0 {
		refreshTextEngine()
		for _, state := range application.windows {
			state.refreshText()
		}
	}
	if change&changedAppearance != 0 {
		platform.NotifyAppearanceChanged()
	}
	platform.NotifyDesktopChanged(change.platformChange())
}

// closeDesktop ends the loop's connection to the bus.
func (application *linuxApplication) closeDesktop() {
	if application.desktop != nil {
		application.stopDesktopWatch()
	}
	application.desktopRetry.reset()
}

// publish gives the settings this window's tracker read to the rest of the process and says
// what that changed.
func (tracker *scaleTracker) publish() settingsChange {
	return publishSettings(tracker.settings, tracker.resource, os.Getenv)
}

// refreshTextEngine drops the lines the shared engine shaped with the old fonts, and gives its
// Pango context the new options.
func refreshTextEngine() {
	engine := sharedTextEngine()
	if engine == nil {
		return
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()
	engine.cache.Clear()
	engine.services.useFonts(systemFonts())
}

// refreshText rasterizes the window's labels again with the fonts now in force.
func (state *linuxWindowState) refreshText() {
	if state.renderer == nil {
		return
	}
	if err := state.renderer.refreshText(); err == nil && !state.hidden {
		_ = state.renderer.present()
	}
}

// DarkAppearance reports the desktop's light or dark preference: the settings portal
// (org.freedesktop.appearance, color-scheme: 1 is dark, 2 is light, 0 is no preference),
// then, for a desktop without one, its XSETTINGS manager, GTK_THEME and the toolkits'
// configuration files. known is false when none says. While the application watches, the
// portal's changes keep the answer current; otherwise it is asked again when it is older
// than a few seconds.
func DarkAppearance() (dark, known bool) {
	ensureDesktopFresh()
	desktopState.Lock()
	defer desktopState.Unlock()
	dark, known, _ = resolveLocked(os.Getenv)
	return dark, known
}

// ReduceMotion reports whether the desktop asks for less animation, from the same sources.
func ReduceMotion() bool {
	ensureDesktopFresh()
	desktopState.Lock()
	defer desktopState.Unlock()
	_, _, reduce := resolveLocked(os.Getenv)
	return reduce == triTrue
}

// ensureDesktopFresh asks the portal and reads the configuration files when what is held is
// older than stale and nothing keeps it current.
func ensureDesktopFresh() {
	desktopState.Lock()
	needFiles := desktopState.filesAt.IsZero() || time.Since(desktopState.filesAt) > stale
	needPortal := !desktopState.watching && !desktopState.portalGone &&
		(!desktopState.portalRead || time.Since(desktopState.portalAt) > stale)
	desktopState.Unlock()
	if needFiles {
		files := readDesktopFiles()
		desktopState.Lock()
		desktopState.files, desktopState.filesAt = files, time.Now()
		desktopState.Unlock()
	}
	if !needPortal {
		return
	}
	values, result := askPortal()
	if result != portalReadOK {
		desktopState.Lock()
		desktopState.portalRead, desktopState.portalAt = true, time.Now()
		if result == portalReadUnsupported {
			desktopState.portalGone = true
		}
		desktopState.Unlock()
		return
	}
	replacePortal(values, os.Getenv)
}

// askPortal reads the portal's settings over a connection of its own.
func askPortal() (values portalValues, result portalReadResult) {
	conn, err := dialDesktop()
	if err != nil {
		return portalValues{}, portalReadRetry
	}
	defer conn.Close()
	return readPortalValues(conn)
}

// readDesktopFiles reads the toolkits' configuration files that say how the desktop looks.
func readDesktopFiles() desktopFiles {
	config := os.Getenv("XDG_CONFIG_HOME")
	if config == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return desktopFiles{}
		}
		config = filepath.Join(home, ".config")
	}
	return desktopFiles{
		GTK3:       readSmallFile(filepath.Join(config, "gtk-3.0", "settings.ini")),
		GTK4:       readSmallFile(filepath.Join(config, "gtk-4.0", "settings.ini")),
		KDEGlobals: readSmallFile(filepath.Join(config, "kdeglobals")),
	}
}

// readSmallFile is the start of a configuration file, empty when it cannot be read.
func readSmallFile(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	text, _ := io.ReadAll(io.LimitReader(file, 256<<10))
	return string(text)
}

// notificationAppName is the application's name as the notification service shows it: the
// executable's file name, as the window class defaults to.
func notificationAppName() string {
	if class := wmClass(""); len(class) > 1 {
		return string(class[:len(class)/2-1])
	}
	return "Viva"
}
