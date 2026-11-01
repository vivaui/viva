package linux

import (
	"os"

	"github.com/vivaui/viva/internal/dbus"
	"github.com/vivaui/viva/internal/platform"
)

// atspiHost connects the application to the accessibility bus when the desktop
// has accessibility on, and serves the windows' trees on it. With accessibility
// off it holds nothing: the application asks once, at its first window, and if
// the desktop has the service but it is off, keeps one idle connection to hear
// of its being switched on. No thread, timer or poll wakes it otherwise.
type atspiHost struct {
	bridge  *atspiBridge
	watcher *dbus.Conn
	windows map[atspiShell]*atspiWindow
	// latest is the newest nodes of each window while nothing is connected: the
	// application's own slice, kept by header, valid on this thread.
	latest map[atspiShell][]platform.AccessibilityNode
}

// atspiShell is a window the host serves. Both window shells implement it.
type atspiShell interface {
	// atspiConfig is the window's configuration: its title, its first nodes and the handler
	// of what a reader asks of them.
	atspiConfig() *platform.Config
	// atspiGeometry is where the client area lies on the screen, in device pixels, and the
	// window's scale. A shell that cannot know the position says 0, 0.
	atspiGeometry() (x, y int, scale float32)
	// atspiPresent shows the frame an accessibility request answered with.
	atspiPresent(platform.Frame) error
}

// atspiOwner is the application the host serves: the event loop that polls its descriptor.
type atspiOwner interface {
	// rebuildPoll tells the loop the host's descriptor changed.
	rebuildPoll()
	fail(error)
	atspiShells() []atspiShell
}

const (
	a11yBusName       = "org.a11y.Bus"
	a11yBusPath       = "/org/a11y/bus"
	a11yStatus        = "org.a11y.Status"
	dbusProperties    = "org.freedesktop.DBus.Properties"
	dbusPropertiesGet = "Get"
)

// atspiEnabled asks the session bus whether accessibility is switched on. An
// error means the desktop has no accessibility service to ask.
func atspiEnabled(session *dbus.Conn) (bool, error) {
	for _, property := range []string{"IsEnabled", "ScreenReaderEnabled"} {
		var request dbus.Encoder
		request.String(a11yStatus)
		request.String(property)
		reply, err := session.Call(a11yBusName, a11yBusPath, dbusProperties, dbusPropertiesGet, "ss", request.Bytes())
		if err != nil {
			return false, err
		}
		args := reply.Args()
		if args.Signature() == "b" && args.Bool() {
			return true, nil
		}
	}
	return false, nil
}

// atspiAddress is where the accessibility bus is.
func atspiAddress(session *dbus.Conn) (string, error) {
	reply, err := session.Call(a11yBusName, a11yBusPath, a11yBusName, "GetAddress", "", nil)
	if err != nil {
		return "", err
	}
	return reply.Args().String(), nil
}

// fd is the descriptor the event loop polls for the host, or -1.
func (host *atspiHost) fd() int {
	switch {
	case host == nil:
		return -1
	case host.bridge != nil && host.bridge.conn != nil:
		return host.bridge.conn.FD()
	case host.watcher != nil:
		return host.watcher.FD()
	}
	return -1
}

// engaged says the host has something to keep current.
func (host *atspiHost) engaged() bool {
	return host != nil && (host.bridge != nil || host.watcher != nil)
}

// atspiOpened is called when a window has opened. The first one makes the
// application ask the desktop whether accessibility is on. It returns the host.
func atspiOpened(host *atspiHost, owner atspiOwner, target atspiShell) *atspiHost {
	if os.Getenv("VIVA_ACCESSIBILITY") == "0" {
		return host
	}
	first := false
	if host == nil {
		host, first = &atspiHost{
			windows: make(map[atspiShell]*atspiWindow),
			latest:  make(map[atspiShell][]platform.AccessibilityNode),
		}, true
	}
	host.latest[target] = target.atspiConfig().Accessibility
	if first {
		host.probe(owner)
	}
	if !host.engaged() {
		delete(host.latest, target)
		return host
	}
	if host.bridge != nil && host.windows[target] == nil {
		host.attach(owner, target)
	}
	owner.rebuildPoll()
	return host
}

// probe asks the session bus about accessibility and connects, or waits.
func (host *atspiHost) probe(owner atspiOwner) {
	session, err := dbus.DialSession()
	if err != nil {
		return
	}
	enabled, err := atspiEnabled(session)
	if err != nil {
		session.Close()
		return
	}
	if enabled {
		address, err := atspiAddress(session)
		session.Close()
		if err == nil {
			host.connect(owner, address)
		}
		return
	}
	var match dbus.Encoder
	match.String("type='signal',sender='" + a11yBusName + "',path='" + a11yBusPath + "',interface='" + dbusProperties + "',member='PropertiesChanged'")
	if _, err := session.Call("org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus", "AddMatch", "s", match.Bytes()); err != nil {
		session.Close()
		return
	}
	host.watcher = session
}

// connect joins the accessibility bus and registers the application.
func (host *atspiHost) connect(owner atspiOwner, address string) bool {
	conn, err := dbus.Dial(address)
	if err != nil {
		return false
	}
	bridge := newAtspiBridge(conn)
	if err := bridge.register(); err != nil {
		bridge.close()
		return false
	}
	host.bridge = bridge
	for _, target := range owner.atspiShells() {
		host.attach(owner, target)
	}
	return true
}

// attach gives a window a frame on the bus.
func (host *atspiHost) attach(owner atspiOwner, target atspiShell) {
	var act func(platform.AccessibilityRequest) bool
	config := target.atspiConfig()
	if handler := config.OnAccessibility; handler != nil {
		act = func(request platform.AccessibilityRequest) bool {
			handled, err := handler(request, func(frame platform.Frame) error { return target.atspiPresent(frame) })
			if err != nil {
				owner.fail(err)
				return false
			}
			return handled
		}
	}
	host.windows[target] = host.bridge.addWindow(config.Title, host.latest[target], target.atspiGeometry, act)
}

// atspiFrame hands the host the accessibility nodes of a frame.
func (host *atspiHost) frame(target atspiShell, nodes []platform.AccessibilityNode) {
	if !host.engaged() {
		return
	}
	host.latest[target] = nodes
	if window := host.windows[target]; window != nil && host.bridge != nil {
		host.bridge.update(window, nodes)
	}
}

// active reports a window gaining or losing the keyboard.
func (host *atspiHost) active(target atspiShell, active bool) {
	if host == nil || host.bridge == nil {
		return
	}
	if window := host.windows[target]; window != nil {
		host.bridge.setActive(window, active)
	}
}

// closed removes a closed window.
func (host *atspiHost) closed(target atspiShell) {
	if host == nil {
		return
	}
	delete(host.latest, target)
	if window := host.windows[target]; window != nil {
		if host.bridge != nil {
			host.bridge.removeWindow(window)
		}
		delete(host.windows, target)
	}
}

// close lets go of the host's connections.
func (host *atspiHost) close() {
	if host == nil {
		return
	}
	host.dropBridge()
	host.dropWatcher()
}

func (host *atspiHost) dropBridge() {
	if host.bridge != nil {
		host.bridge.close()
		host.bridge = nil
	}
	clear(host.windows)
}

func (host *atspiHost) dropWatcher() {
	if host.watcher != nil {
		host.watcher.Close()
		host.watcher = nil
	}
}

// serve handles what arrived on the host's connection: a reader's calls, or word that
// accessibility was switched on.
func (host *atspiHost) serve(owner atspiOwner) {
	switch {
	case host.bridge != nil:
		if err := host.bridge.serve(); err != nil || host.bridge.send == nil {
			host.dropBridge()
			owner.rebuildPoll()
		}
	case host.watcher != nil:
		messages, err := host.watcher.Receive()
		if err != nil {
			host.dropWatcher()
			owner.rebuildPoll()
			return
		}
		for _, message := range messages {
			if message.Type != dbus.TypeSignal || message.Member != "PropertiesChanged" {
				continue
			}
			enabled, err := atspiEnabled(host.watcher)
			if err != nil || !enabled {
				continue
			}
			address, err := atspiAddress(host.watcher)
			if err == nil && host.connect(owner, address) {
				host.dropWatcher()
				owner.rebuildPoll()
			}
			return
		}
	}
}

// The XCB backend's window states are the host's targets.

func (state *linuxWindowState) atspiConfig() *platform.Config { return &state.config }

func (state *linuxWindowState) atspiGeometry() (int, int, float32) {
	x, y := state.win.rootOrigin()
	return int(x), int(y), state.win.displayScale()
}

func (state *linuxWindowState) atspiPresent(frame platform.Frame) error {
	return state.router.present(frame)
}

func (application *linuxApplication) atspiShells() []atspiShell {
	targets := make([]atspiShell, len(application.windows))
	for index, state := range application.windows {
		targets[index] = state
	}
	return targets
}

func (services *desktopServices) atspiOpened(shell atspiShell) {
	services.atspi = atspiOpened(services.atspi, services.host, shell)
}

func (services *desktopServices) atspiFrame(shell atspiShell, nodes []platform.AccessibilityNode) {
	services.atspi.frame(shell, nodes)
}

func (services *desktopServices) atspiActive(shell atspiShell, active bool) {
	services.atspi.active(shell, active)
}

func (services *desktopServices) atspiClosed(shell atspiShell) { services.atspi.closed(shell) }

func (services *desktopServices) atspiClose() {
	services.atspi.close()
	services.atspi = nil
}

func (services *desktopServices) serveAtspi() { services.atspi.serve(services.host) }
