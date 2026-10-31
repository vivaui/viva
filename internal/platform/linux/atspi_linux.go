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
	// waylandFD is the descriptor registered with the Wayland loop. Keep it
	// after a service closes its connection so the loop can unregister it.
	waylandFD    int32
	waylandFDSet bool
	windows      map[any]*atspiWindow
	// latest is the newest nodes of each window while nothing is connected: the
	// application's own slice, kept by header, valid on this thread.
	latest map[any][]platform.AccessibilityNode
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
// application ask the desktop whether accessibility is on.
func (application *linuxApplication) atspiOpened(state *linuxWindowState) {
	if os.Getenv("VIVA_ACCESSIBILITY") == "0" {
		return
	}
	host, first := application.atspi, false
	if host == nil {
		host, first = &atspiHost{
			windows: make(map[any]*atspiWindow),
			latest:  make(map[any][]platform.AccessibilityNode),
		}, true
		application.atspi = host
	}
	host.latest[state] = state.config.Accessibility
	if first {
		host.probe(application)
	}
	if !host.engaged() {
		delete(host.latest, state)
		return
	}
	if host.bridge != nil && host.windows[state] == nil {
		host.attach(application, state)
	}
	application.rebuildPoll()
}

// probe asks the session bus about accessibility and connects, or waits.
func (host *atspiHost) probe(application *linuxApplication) {
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
			host.connect(application, address)
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
func (host *atspiHost) connect(application *linuxApplication, address string) bool {
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
	for _, state := range application.windows {
		host.attach(application, state)
	}
	return true
}

// attach gives a window a frame on the bus.
func (host *atspiHost) attach(application *linuxApplication, state *linuxWindowState) {
	geometry := func() (int, int, float32) {
		x, y := state.win.rootOrigin()
		return int(x), int(y), state.win.displayScale()
	}
	var act func(platform.AccessibilityRequest) bool
	if handler := state.config.OnAccessibility; handler != nil {
		act = func(request platform.AccessibilityRequest) bool {
			handled, err := handler(request, func(frame platform.Frame) error { return state.router.present(frame) })
			if err != nil {
				application.fail(err)
				return false
			}
			return handled
		}
	}
	host.windows[state] = host.bridge.addWindow(state.config.Title, host.latest[state], geometry, act)
}

// atspiFrame hands the host the accessibility nodes of a frame.
func (application *linuxApplication) atspiFrame(state *linuxWindowState, nodes []platform.AccessibilityNode) {
	host := application.atspi
	if !host.engaged() {
		return
	}
	host.latest[state] = nodes
	if window := host.windows[state]; window != nil && host.bridge != nil {
		host.bridge.update(window, nodes)
	}
}

// atspiActive reports a window gaining or losing the keyboard.
func (application *linuxApplication) atspiActive(state *linuxWindowState, active bool) {
	host := application.atspi
	if host == nil || host.bridge == nil {
		return
	}
	if window := host.windows[state]; window != nil {
		host.bridge.setActive(window, active)
	}
}

// atspiClosed removes a closed window.
func (application *linuxApplication) atspiClosed(state *linuxWindowState) {
	host := application.atspi
	if host == nil {
		return
	}
	delete(host.latest, state)
	if window := host.windows[state]; window != nil {
		if host.bridge != nil {
			host.bridge.removeWindow(window)
		}
		delete(host.windows, state)
	}
}

// atspiClose lets go of the host's connections.
func (application *linuxApplication) atspiClose() {
	host := application.atspi
	if host == nil {
		return
	}
	host.dropBridge()
	host.dropWatcher()
	application.atspi = nil
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

// serveAtspi handles what arrived on the host's connection: a reader's calls,
// or word that accessibility was switched on.
func (application *linuxApplication) serveAtspi() {
	host := application.atspi
	switch {
	case host.bridge != nil:
		if err := host.bridge.serve(); err != nil || host.bridge.send == nil {
			host.dropBridge()
			application.rebuildPoll()
		}
	case host.watcher != nil:
		messages, err := host.watcher.Receive()
		if err != nil {
			host.dropWatcher()
			application.rebuildPoll()
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
			if err == nil && host.connect(application, address) {
				host.dropWatcher()
				application.rebuildPoll()
			}
			return
		}
	}
}

func (app *waylandApplication) atspiOpened(state *waylandWindowState) {
	if os.Getenv("VIVA_ACCESSIBILITY") == "0" {
		return
	}
	host, first := app.atspi, false
	if host == nil {
		host, first = &atspiHost{
			windows: make(map[any]*atspiWindow),
			latest:  make(map[any][]platform.AccessibilityNode),
		}, true
		app.atspi = host
	}
	host.latest[state] = state.config.Accessibility
	if first {
		host.probeForWayland(app)
	}
	if !host.engaged() {
		delete(host.latest, state)
		return
	}
	if host.bridge != nil && host.windows[state] == nil {
		host.attachWayland(app, state)
	}
}

func (app *waylandApplication) addAtspiFD(host *atspiHost) {
	if app.loop == nil || host == nil || host.waylandFDSet {
		return
	}
	fd := int32(host.fd())
	if fd < 0 {
		return
	}
	app.loop.addExtraFD(fd, app.serveAtspi)
	host.waylandFD = fd
	host.waylandFDSet = true
}

func (app *waylandApplication) removeAtspiFD(host *atspiHost) {
	if host == nil {
		return
	}
	if host.waylandFDSet && app.loop != nil {
		app.loop.removeExtraFD(host.waylandFD)
	}
	host.waylandFD = 0
	host.waylandFDSet = false
}

func (host *atspiHost) probeForWayland(app *waylandApplication) {
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
			host.connectWayland(app, address)
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
	app.addAtspiFD(host)
}

func (host *atspiHost) connectWayland(app *waylandApplication, address string) bool {
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
	app.addAtspiFD(host)
	app.mu.Lock()
	windows := append([]*waylandWindowState(nil), app.windows...)
	app.mu.Unlock()
	for _, state := range windows {
		host.attachWayland(app, state)
	}
	return true
}

func (host *atspiHost) attachWayland(app *waylandApplication, state *waylandWindowState) {
	geometry := func() (int, int, float32) {
		scale := state.win.displayScale()
		margin := int(float32(state.win.margin) / scale)
		return margin, margin, scale
	}
	var act func(platform.AccessibilityRequest) bool
	if handler := state.config.OnAccessibility; handler != nil {
		act = func(request platform.AccessibilityRequest) bool {
			handled, err := handler(request, func(frame platform.Frame) error {
				if state.router.present != nil {
					return state.router.present(frame)
				}
				return nil
			})
			if err != nil {
				app.fail(err)
				return false
			}
			return handled
		}
	}
	host.windows[state] = host.bridge.addWindow(state.config.Title, host.latest[state], geometry, act)
}

func (app *waylandApplication) atspiFrame(state *waylandWindowState, nodes []platform.AccessibilityNode) {
	host := app.atspi
	if host == nil || !host.engaged() {
		return
	}
	host.latest[state] = nodes
	if window := host.windows[state]; window != nil && host.bridge != nil {
		host.bridge.update(window, nodes)
	}
}

func (app *waylandApplication) atspiActive(state *waylandWindowState, active bool) {
	host := app.atspi
	if host == nil || host.bridge == nil {
		return
	}
	if window := host.windows[state]; window != nil {
		host.bridge.setActive(window, active)
	}
}

func (app *waylandApplication) atspiClosed(state *waylandWindowState) {
	host := app.atspi
	if host == nil {
		return
	}
	delete(host.latest, state)
	if window := host.windows[state]; window != nil {
		if host.bridge != nil {
			host.bridge.removeWindow(window)
		}
		delete(host.windows, state)
	}
}

func (app *waylandApplication) atspiClose() {
	host := app.atspi
	if host == nil {
		return
	}
	app.removeAtspiFD(host)
	host.dropBridge()
	host.dropWatcher()
	app.atspi = nil
}

func (app *waylandApplication) connectAtspi(host *atspiHost, address string) {
	if host == nil {
		return
	}
	app.removeAtspiFD(host)
	if host.connectWayland(app, address) {
		host.dropWatcher()
		return
	}
	// Keep listening for the next PropertiesChanged after a failed reconnect.
	app.addAtspiFD(host)
}

func (app *waylandApplication) serveAtspi() {
	host := app.atspi
	if host == nil {
		return
	}
	switch {
	case host.bridge != nil:
		if err := host.bridge.serve(); err != nil || host.bridge.send == nil {
			app.removeAtspiFD(host)
			host.dropBridge()
		}
	case host.watcher != nil:
		messages, err := host.watcher.Receive()
		if err != nil {
			app.removeAtspiFD(host)
			host.dropWatcher()
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
			if err == nil {
				app.connectAtspi(host, address)
			}
			return
		}
	}
}
