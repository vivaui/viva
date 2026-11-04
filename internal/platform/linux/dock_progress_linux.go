//go:build linux

package linux

import (
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/vivaui/viva/internal/dbus"
)

// A Linux desktop has no Dock tile to draw on. The launcher entry is what comes
// closest: a desktop that implements Unity's LauncherEntry interface (KDE
// Plasma and Unity among them) draws a bar across the application's icon from
// the "progress" property, and an application says how far along it is by
// emitting the interface's Update signal on the session bus. Nothing is asked
// of the desktop, so a session whose shell does not know the interface simply
// never draws anything: the signal goes out and is dropped, which is the same
// nothing SetDockBadge does on Linux.
const (
	unityLauncherInterface = "com.canonical.Unity.LauncherEntry"
	unityLauncherPath      = "/"
	// launcherRetry is how long a program waits before trying the session bus
	// again after a dial or a write failed, so a machine without a bus costs one
	// attempt every few seconds rather than one per update.
	launcherRetry = 5 * time.Second
)

// launcherProgress is the application's launcher entry. A process has one
// application, and every window of it reports the same work, so the entry is
// one value for the program rather than one per window.
var launcherProgress launcherEntry

// launcherEntry is the bar on the application's launcher icon, and the session
// bus connection that carries it.
type launcherEntry struct {
	mu    sync.Mutex
	class string
	conn  *dbus.Conn
	// next is when the bus may be tried again after a failure.
	next time.Time
	// sent is the state the desktop was last told, and reported says it was
	// told anything at all: a fraction that has not changed is not sent again.
	sent     float32
	visible  bool
	reported bool
}

// noteClass records the name the desktop knows the application by, which is the
// one the window's class gives it (see platform.Config.Class): the launcher
// entry is the desktop file of that name.
func (entry *launcherEntry) noteClass(class string) {
	if class == "" {
		return
	}
	entry.mu.Lock()
	entry.class = class
	entry.mu.Unlock()
}

// set tells the desktop how much of the application's work is done. A negative
// fraction, or one that is not a number, hides the bar.
func (entry *launcherEntry) set(fraction float32) {
	known := fraction >= 0 && !math.IsNaN(float64(fraction))
	if known && fraction > 1 {
		fraction = 1
	}
	if !known {
		// A hidden bar has no length; zero keeps the value comparable, so a
		// repeated request to hide it is not sent again.
		fraction = 0
	}
	entry.mu.Lock()
	defer entry.mu.Unlock()
	if entry.reported && entry.visible == known && entry.sent == fraction {
		return
	}
	connection := entry.connection()
	if connection == nil {
		return
	}
	body := launcherEntryBody(entry.desktopID(), known, fraction)
	if err := connection.Emit(unityLauncherPath, unityLauncherInterface, "Update", "sa{sv}", body); err != nil {
		// The bus went away under us: drop the connection so the next update
		// dials again, and leave the desktop showing what it last heard.
		entry.close()
		return
	}
	entry.sent, entry.visible, entry.reported = fraction, known, true
}

// connection is the session bus connection, dialled the first time a bar is
// asked for and kept for the updates that follow.
func (entry *launcherEntry) connection() *dbus.Conn {
	if entry.conn != nil {
		return entry.conn
	}
	if time.Now().Before(entry.next) {
		return nil
	}
	connection, err := dbus.DialSession()
	if err != nil {
		entry.next = time.Now().Add(launcherRetry)
		return nil
	}
	entry.conn = connection
	return connection
}

func (entry *launcherEntry) close() {
	if entry.conn != nil {
		entry.conn.Close()
		entry.conn = nil
	}
	entry.next = time.Now().Add(launcherRetry)
}

// desktopID is the desktop file the launcher matches the application by: the
// class of its windows, which viva package writes into the entry's name. A
// program run without one is named after its executable.
func (entry *launcherEntry) desktopID() string {
	class := entry.class
	if class == "" {
		if executable, err := os.Executable(); err == nil {
			class = filepath.Base(executable)
		}
	}
	return class + ".desktop"
}

// launcherEntryBody is the Update signal's body: the application's entry, and
// the properties that changed. A hidden bar is sent as progress-visible false
// rather than as a zero, because a bar at zero is a bar.
func launcherEntryBody(desktopID string, visible bool, fraction float32) []byte {
	var body dbus.Encoder
	body.String("application://" + desktopID)
	entries := body.BeginArray(8)
	body.Align(8)
	body.String("progress-visible")
	body.Variant("b")
	body.Bool(visible)
	body.Align(8)
	body.String("progress")
	body.Variant("d")
	body.Float64(float64(fraction))
	body.EndArray(entries)
	return body.Bytes()
}

// SetProgress draws how much of the application's work is done on its launcher
// icon, on a desktop that has a launcher entry to draw it on, and a negative
// fraction takes the bar away.
func (application *linuxApplication) SetProgress(fraction float32) { launcherProgress.set(fraction) }
