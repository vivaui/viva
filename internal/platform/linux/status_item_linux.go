package linux

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/dbus"
	"github.com/vivaui/viva/internal/dbusmenu"
	"github.com/vivaui/viva/internal/platform"
)

const (
	xcbNotifyGrab     = 1
	xcbNotifyUngrab   = 2
	xcbNotifyInferior = 2
	// FocusIn details that say this window itself gained the keyboard.
	xcbNotifyAncestor  = 0
	xcbNotifyNonlinear = 3

	xcbAtomWMNormalHints = 40
	xcbAtomWMSizeHints   = 41
	xcbAtomCardinal      = 6

	sizeHintUSPosition = 1 << 0
	sizeHintPPosition  = 1 << 2
	sizeHintPMinSize   = 1 << 4
	sizeHintPMaxSize   = 1 << 5

	itemPath          = "/StatusNotifierItem"
	itemInterface     = "org.kde.StatusNotifierItem"
	propsInterface    = "org.freedesktop.DBus.Properties"
	watcherPath       = "/StatusNotifierWatcher"
	busName           = "org.freedesktop.DBus"
	requestDoNotQueue = 4
)

// The watcher answers under the KDE name; the freedesktop name is its
// standardized alias that some hosts own instead.
var watcherNames = [...]string{"org.kde.StatusNotifierWatcher", "org.freedesktop.StatusNotifierWatcher"}

// statusIconSizes are the sizes offered to the host, which draws the nearest.
var statusIconSizes = [...]int{22, 44}

var panelDismissedAt atomic.Int64
var processStart = time.Now()

func monotonicNow() int64 { return int64(time.Since(processStart)) }

const panelDismissWindow = 350 * time.Millisecond

// panelOrigin is where a panel's window is created: beside its anchor, or in the
// middle of the screen without one.
func (win *window) panelOrigin(config platform.Config) (x, y int16) {
	monitors := win.placementMonitors()
	if config.WindowMode != platform.WindowPanel {
		if config.HasOrigin {
			// The origin is in logical screen coordinates, like Frame's result.
			scale := win.displayScale()
			// The origin is the content's; the window starts a margin before it.
			originX, originY := int(config.Origin.X*scale)-int(win.margin), int(config.Origin.Y*scale)-int(win.margin)
			// A position remembered on a monitor that is no longer there would
			// open the window out of reach.
			originX, originY, _ = visiblePosition(monitors, originX, originY, int(win.width), int(win.height))
			return clampInt16(originX), clampInt16(originY)
		}
		return 0, 0
	}
	// A panel is placed in device pixels, as the host reports the anchor, on the
	// monitor the anchor is on (the pointer's, without one) and within the part
	// of it the desktop's panels leave free.
	width, height := float32(win.width), float32(win.height)
	area := platform.ScreenRect{Width: float32(win.screen.WidthPixels), Height: float32(win.screen.HeightPixels)}
	focusX, focusY := int(area.Width)/2, int(area.Height)/2
	if config.HasAnchor {
		focusX, focusY = int(config.Anchor.X+config.Anchor.Width/2), int(config.Anchor.Y+config.Anchor.Height/2)
	} else if pointerX, pointerY, ok := win.pointerPosition(); ok {
		focusX, focusY = pointerX, pointerY
	}
	if monitor, ok := monitorAt(monitors, focusX, focusY); ok && monitor.Width > 0 && monitor.Height > 0 {
		work, haveWork := win.workArea()
		usable := usableArea(monitorRect(monitor), work, haveWork)
		area = platform.ScreenRect{X: float32(usable.x), Y: float32(usable.y), Width: float32(usable.width), Height: float32(usable.height)}
	}
	var fx, fy float32
	if config.HasAnchor {
		fx, fy = platform.PlacePanel(config.Anchor, width, height, area)
	} else {
		fx, fy = area.X+(area.Width-width)/2, area.Y+(area.Height-height)/2
	}
	return clampInt16(int(fx)), clampInt16(int(fy))
}

// clampInt16 is a root coordinate as the 16 bits X gives a window position;
// a monitor left of or above the primary one has negative ones.
func clampInt16(value int) int16 {
	return int16(min(max(value, math.MinInt16), math.MaxInt16))
}

// markPanel tells the window manager what a panel is: a small floating dialog
// that stays above others, has no taskbar entry, appears where it asked and keeps
// its size. It is a dialog rather than a utility or popup window because window
// managers do not give those the focus on mapping, and a panel that cannot take
// the focus cannot notice losing it.
func (win *window) markPanel(config platform.Config, x, y int16) error {
	api, connection := win.api, win.connection
	setAtoms := func(property string, values ...string) error {
		name, err := api.atom(connection, property)
		if err != nil {
			return err
		}
		atoms := make([]uint32, len(values))
		for index, value := range values {
			if atoms[index], err = api.atom(connection, value); err != nil {
				return err
			}
		}
		api.changeProperty(connection, xcbPropertyReplace, win.id, name, xcbAtomAtom, 32, uint32(len(atoms)), unsafe.Pointer(&atoms[0]))
		return nil
	}
	if err := setAtoms("_NET_WM_WINDOW_TYPE", "_NET_WM_WINDOW_TYPE_DIALOG"); err != nil {
		return err
	}
	if err := setAtoms("_NET_WM_STATE", "_NET_WM_STATE_ABOVE", "_NET_WM_STATE_SKIP_TASKBAR", "_NET_WM_STATE_SKIP_PAGER"); err != nil {
		return err
	}
	// WM_SIZE_HINTS: a user-specified position, and a fixed size.
	var hints [18]uint32
	hints[0] = sizeHintUSPosition | sizeHintPPosition | sizeHintPMinSize | sizeHintPMaxSize
	hints[1], hints[2] = uint32(int32(x)), uint32(int32(y))
	hints[3], hints[4] = win.width, win.height
	hints[5], hints[6], hints[7], hints[8] = win.width, win.height, win.width, win.height
	api.changeProperty(connection, xcbPropertyReplace, win.id, xcbAtomWMNormalHints, xcbAtomWMSizeHints, 32, uint32(len(hints)), unsafe.Pointer(&hints[0]))
	return nil
}

// command runs one request another goroutine posted to this window's loop.
func (state *linuxWindowState) command(command uint32, data []byte) error {
	win := state.win
	switch command {
	case commandSetSize:
		if len(data) < 12 || win.frame.maximized || win.fullscreen.Load() {
			return nil
		}
		width := int(binary.LittleEndian.Uint32(data[4:8]))
		height := int(binary.LittleEndian.Uint32(data[8:12]))
		margin := uint32(0)
		if win.margin > 0 {
			margin = frameMargin(win.displayScale())
		}
		if width < 1 || height < 1 {
			return nil
		}
		size := [2]uint32{
			min(deviceLength(width, win.displayScale())+2*margin, 65535),
			min(deviceLength(height, win.displayScale())+2*margin, 65535),
		}
		win.api.configureWindow(win.connection, win.id,
			xcbConfigWindowWidth|xcbConfigWindowHeight, &size[0])
		win.api.flush(win.connection)
	case commandEnterFullscreen, commandExitFullscreen:
		action := uint32(0) // _NET_WM_STATE_REMOVE
		if command == commandEnterFullscreen {
			action = 1 // _NET_WM_STATE_ADD
		}
		win.sendToRoot(win.frame.stateAtom, [5]uint32{action, win.frame.fullscreen, 0, sourceApp})
		if command == commandExitFullscreen && win.topmostConfigured.Load() && win.topmost.Load() {
			win.sendToRoot(win.frame.stateAtom, [5]uint32{1, win.frame.above, 0, sourceApp})
		}
	case commandHide:
		state.hidden = true
		win.api.unmapWindow(win.connection, win.id)
		win.api.flush(win.connection)
	case commandShow:
		win.api.mapWindow(win.connection, win.id)
		if win.topmostConfigured.Load() {
			action := uint32(0)
			if win.topmost.Load() {
				action = 1
			}
			win.sendToRoot(win.frame.stateAtom, [5]uint32{action, win.frame.above, 0, sourceApp})
		}
		if state.config.WindowMode != platform.WindowPanel {
			if err := win.requestActivation(); err != nil {
				return err
			}
		}
		win.api.flush(win.connection)
	case commandQuit:
		state.application.quit = true
	case commandStatus:
		state.application.drainStatusRemovals()
	case commandDesktop:
		state.application.syncDesktopWatch()
	}
	return nil
}

// requestActivation asks the window manager to bring this window forward, as a
// pager does: with a _NET_ACTIVE_WINDOW message to the root window.
func (win *window) requestActivation() error {
	active, err := win.api.atom(win.connection, "_NET_ACTIVE_WINDOW")
	if err != nil {
		return err
	}
	event := xcbClientEvent{ResponseType: xcbClientMessage, Format: 32, Window: win.id, Type: active}
	event.Data[0] = 1 // the request comes from an application
	win.api.sendEvent(win.connection, 0, win.screen.Root, xcbEventSubstructureRedirect|xcbEventSubstructureNotify, (*byte)(unsafe.Pointer(&event)))
	return nil
}

// Frame is the content area in screen coordinates. It asks the X server, so any
// goroutine may call it; the connection is guarded by the window lock.
func (window *nativeWindow) Frame() (gfx.Rect, bool) {
	if window == nil || window.closed.Load() || window.state == nil || window.state.win == nil {
		return gfx.Rect{}, false
	}
	win := window.state.win
	win.mu.Lock()
	defer win.mu.Unlock()
	if win.connection == 0 {
		return gfx.Rect{}, false
	}
	x, y := win.contentOrigin()
	scale := win.displayScale()
	return gfx.Rect{X: float32(x) / scale, Y: float32(y) / scale, Width: float32(contentSize(win.width, win.margin)) / scale, Height: float32(contentSize(win.height, win.margin)) / scale}, true
}

// Hide and Show post to the window's loop, so they may be called anywhere.
func (window *nativeWindow) Hide() {
	if window != nil && !window.closed.Load() {
		window.state.win.postClientMessage(window.state.win.commandAtom, commandHide)
	}
}

func (window *nativeWindow) Show() {
	if window != nil && !window.closed.Load() {
		window.state.win.postClientMessage(window.state.win.commandAtom, commandShow)
	}
}

// Quit closes every window and ends the loop. It posts to the windows' loops, so
// it may be called anywhere.
func (application *linuxApplication) Quit() { application.post(commandQuit) }

// post sends a command to every window that is still open and lets whichever
// handles it first act. Posting to just one would lose the request when that
// window closes before it reads it, as the tray's panel does when it quits.
func (application *linuxApplication) post(command uint32) {
	application.liveMu.Lock()
	windows := append([]*window(nil), application.live...)
	application.liveMu.Unlock()
	for _, win := range windows {
		win.postClientMessage(win.commandAtom, command)
	}
}

// statusConnection is the session bus connection the status icons own. It is an
// interface so a test can script the bus and the host, as the desktop watch's
// connection lets its tests do.
type statusConnection interface {
	Call(destination, path, iface, member, signature string, body []byte) (*dbus.Message, error)
	Send(destination, path, iface, member, signature string, body []byte) (uint32, error)
	Receive() ([]*dbus.Message, error)
	Reply(call *dbus.Message, signature string, body []byte) error
	ReplyError(call *dbus.Message, name, text string) error
	Emit(path, iface, member, signature string, body []byte) error
	FD() int
	Close()
}

var dialStatus = func() (statusConnection, error) { return dbus.DialSession() }

// statusNotifier is the application's connection to the session bus and the
// icons it has registered with the desktop's StatusNotifier host.
type statusNotifier struct {
	conn  statusConnection
	items map[string]*linuxStatusItem // by the bus name each is registered under
	post  func(command uint32)        // wakes the event loop from another goroutine
	// pending is the registration call in flight to each watcher, by the serial
	// the call was sent with. The host may answer it long after it was sent,
	// while the icon already serves the properties the host asks for.
	pending map[uint32]pendingRegistration

	mu       sync.Mutex
	removals []string
	nextID   uint32
}

// pendingRegistration is one item's call to one watcher, still unanswered.
type pendingRegistration struct {
	item    *linuxStatusItem
	watcher int // index in watcherNames the call was sent to
}

type linuxStatusItem struct {
	notifier *statusNotifier
	name     string
	tooltip  string
	icon     platform.StatusIcon
	onClick  func(platform.StatusClick)
	menu     *dbusmenu.Object // nil for an icon without a menu
	removed  atomic.Bool
}

// AddStatusItem registers an icon with the desktop's StatusNotifier host.
func (services *desktopServices) AddStatusItem(config platform.StatusItemConfig) (platform.StatusItem, error) {
	if !config.Icon.Valid() {
		return nil, errors.New("linux: status icon pixels do not match its size")
	}
	if services.status == nil {
		conn, err := dialStatus()
		if err != nil {
			return nil, fmt.Errorf("%w: %v", platform.ErrStatusAreaUnavailable, err)
		}
		// A host that starts again forgets its items; its name appearing is the cue
		// to register them anew.
		var match dbus.Encoder
		match.String("type='signal',sender='" + busName + "',interface='" + busName + "',member='NameOwnerChanged'")
		if _, err := conn.Call(busName, "/org/freedesktop/DBus", busName, "AddMatch", "s", match.Bytes()); err != nil {
			conn.Close()
			return nil, err
		}
		services.status = &statusNotifier{
			conn: conn, items: make(map[string]*linuxStatusItem), pending: make(map[uint32]pendingRegistration), post: services.host.post,
		}
		services.host.rebuildPoll()
	}
	notifier := services.status
	if !notifier.hostPresent() {
		// No desktop is drawing status icons. Saying so keeps a window from
		// disappearing into a status area that is not there.
		services.dropStatusIfEmpty()
		return nil, fmt.Errorf("%w: no StatusNotifier host", platform.ErrStatusAreaUnavailable)
	}
	notifier.nextID++
	item := &linuxStatusItem{
		notifier: notifier, tooltip: config.Tooltip, icon: config.Icon, onClick: config.OnClick,
		name: fmt.Sprintf("org.kde.StatusNotifierItem-%d-%d", os.Getpid(), notifier.nextID),
	}
	if config.Menu != nil {
		item.menu = dbusmenu.New(config.Menu)
	}
	var request dbus.Encoder
	request.String(item.name)
	request.Uint32(requestDoNotQueue)
	reply, err := notifier.conn.Call(busName, "/org/freedesktop/DBus", busName, "RequestName", "su", request.Bytes())
	if err != nil {
		services.dropStatusIfEmpty()
		return nil, err
	}
	if reply.Args().Uint32() != 1 {
		services.dropStatusIfEmpty()
		return nil, fmt.Errorf("linux: the bus name %s is taken", item.name)
	}
	notifier.items[item.name] = item
	if err := notifier.register(item); err != nil {
		notifier.release(item)
		delete(notifier.items, item.name)
		services.dropStatusIfEmpty()
		return nil, fmt.Errorf("%w: %v", platform.ErrStatusAreaUnavailable, err)
	}
	return item, nil
}

// dropStatusIfEmpty closes a bus connection that has no item to serve.
func (services *desktopServices) dropStatusIfEmpty() {
	if status := services.status; status != nil && len(status.items) == 0 {
		services.closeStatus()
	}
}

// hostPresent asks the bus whether a desktop is drawing status icons. The
// question goes to the bus daemon, which answers at once even while the host
// itself is still starting and slow to answer the calls made to it.
func (notifier *statusNotifier) hostPresent() bool {
	for _, watcher := range watcherNames {
		var arguments dbus.Encoder
		arguments.String(watcher)
		reply, err := notifier.conn.Call(busName, "/org/freedesktop/DBus", busName, "NameHasOwner", "s", arguments.Bytes())
		if err != nil {
			continue
		}
		if reply.Args().Bool() {
			return true
		}
	}
	return false
}

// register tells a watcher about an item without waiting for its answer. A host
// that is still starting can take seconds to answer, and the icon has to read
// the properties the host asks for meanwhile; serveStatus reads the answer and
// tries the other watcher name when one refuses.
func (notifier *statusNotifier) register(item *linuxStatusItem) error {
	if notifier.sendRegistration(item, 0) {
		return nil
	}
	return errors.New("linux: no StatusNotifier watcher took the registration")
}

// sendRegistration sends the registration call to the first watcher name from
// watcher on that is not already waiting for an answer, and reports whether a
// call was sent or is already in flight.
func (notifier *statusNotifier) sendRegistration(item *linuxStatusItem, watcher int) bool {
	for ; watcher < len(watcherNames); watcher++ {
		if notifier.awaitingAnswer(item, watcher) {
			return true
		}
		var arguments dbus.Encoder
		arguments.String(item.name)
		name := watcherNames[watcher]
		serial, err := notifier.conn.Send(name, watcherPath, name, "RegisterStatusNotifierItem", "s", arguments.Bytes())
		if err != nil {
			continue
		}
		notifier.pending[serial] = pendingRegistration{item: item, watcher: watcher}
		return true
	}
	return false
}

// awaitingAnswer reports whether this item's call to this watcher name is
// still unanswered.
func (notifier *statusNotifier) awaitingAnswer(item *linuxStatusItem, watcher int) bool {
	for _, pending := range notifier.pending {
		if pending.item == item && pending.watcher == watcher {
			return true
		}
	}
	return false
}

// settleRegistration reads the answer to a registration that was in flight. A
// refusal is tried on the next watcher name; an acceptance, and an item that
// was removed meanwhile, need nothing more.
func (notifier *statusNotifier) settleRegistration(message *dbus.Message) {
	pending, ok := notifier.pending[message.ReplySerial]
	if !ok {
		return
	}
	delete(notifier.pending, message.ReplySerial)
	if message.Type == dbus.TypeError && !pending.item.removed.Load() {
		notifier.sendRegistration(pending.item, pending.watcher+1)
	}
}

// forgetPending drops the registrations waiting on one watcher name: when its
// owner changes, the call that was in flight went to the owner that is gone.
func (notifier *statusNotifier) forgetPending(watcher string) {
	for serial, pending := range notifier.pending {
		if watcherNames[pending.watcher] == watcher {
			delete(notifier.pending, serial)
		}
	}
}

// forgetItem drops the registrations of an item that is gone. Its calls may
// never be answered, and no answer can matter after its bus name is released.
func (notifier *statusNotifier) forgetItem(item *linuxStatusItem) {
	for serial, pending := range notifier.pending {
		if pending.item == item {
			delete(notifier.pending, serial)
		}
	}
}

func (notifier *statusNotifier) release(item *linuxStatusItem) {
	var name dbus.Encoder
	name.String(item.name)
	notifier.conn.Call(busName, "/org/freedesktop/DBus", busName, "ReleaseName", "s", name.Bytes())
}

// closeStatus drops the bus connection, and with it every icon.
func (services *desktopServices) closeStatus() {
	if services.status != nil {
		services.status.conn.Close()
		services.status = nil
		services.host.rebuildPoll()
	}
}

// serveStatus handles what the bus has sent: calls to an icon, and the watcher
// coming back.
func (services *desktopServices) serveStatus() {
	notifier := services.status
	messages, err := notifier.conn.Receive()
	if err != nil {
		// The session bus went away; the application goes on without its icons.
		services.closeStatus()
		return
	}
	for _, message := range messages {
		switch message.Type {
		case dbus.TypeCall:
			notifier.handle(message)
		case dbus.TypeReturn, dbus.TypeError:
			notifier.settleRegistration(message)
		case dbus.TypeSignal:
			if message.Member == "NameOwnerChanged" && message.Signature == "sss" {
				args := message.Args()
				name, _, owner := args.String(), args.String(), args.String()
				for _, watcher := range watcherNames {
					if name != watcher {
						continue
					}
					notifier.forgetPending(watcher)
					if owner == "" {
						continue
					}
					for _, item := range notifier.items {
						if !item.removed.Load() {
							notifier.register(item)
						}
					}
				}
			}
		}
	}
}

// drainStatusRemovals releases the names of items removed from other goroutines.
func (services *desktopServices) drainStatusRemovals() {
	notifier := services.status
	if notifier == nil {
		return
	}
	notifier.mu.Lock()
	names := notifier.removals
	notifier.removals = nil
	notifier.mu.Unlock()
	for _, name := range names {
		if item := notifier.items[name]; item != nil {
			notifier.release(item)
			notifier.forgetItem(item)
			delete(notifier.items, name)
		}
	}
	services.dropStatusIfEmpty()
}

// handle answers one call made to an icon.
func (notifier *statusNotifier) handle(call *dbus.Message) {
	conn := notifier.conn
	item := notifier.items[call.Destination]
	if item == nil && len(notifier.items) == 1 {
		// A call addressed to the connection's own name, not the one registered.
		for _, only := range notifier.items {
			item = only
		}
	}
	if item != nil && item.menu != nil && call.Path == dbusmenu.Path && call.Interface != "org.freedesktop.DBus.Peer" {
		item.handleMenu(call)
		return
	}
	if item == nil || call.Path != itemPath && call.Interface != "org.freedesktop.DBus.Peer" {
		conn.ReplyError(call, "org.freedesktop.DBus.Error.UnknownObject", "no such object")
		return
	}
	switch call.Interface {
	case "org.freedesktop.DBus.Peer":
		switch call.Member {
		case "Ping":
			conn.Reply(call, "", nil)
		case "GetMachineId":
			var id dbus.Encoder
			id.String("00000000000000000000000000000000")
			conn.Reply(call, "s", id.Bytes())
		default:
			conn.ReplyError(call, "org.freedesktop.DBus.Error.UnknownMethod", call.Member)
		}
	case "org.freedesktop.DBus.Introspectable":
		var xml dbus.Encoder
		xml.String(introspection)
		conn.Reply(call, "s", xml.Bytes())
	case propsInterface:
		item.handleProperties(call)
	case itemInterface:
		item.handleMethod(call)
	default:
		conn.ReplyError(call, "org.freedesktop.DBus.Error.UnknownInterface", call.Interface)
	}
}

// handleMenu answers a call made to the icon's menu, which runs the application's
// handlers for it here, on the event-loop thread.
func (item *linuxStatusItem) handleMenu(call *dbus.Message) {
	conn := item.notifier.conn
	if call.Interface == "org.freedesktop.DBus.Introspectable" {
		var xml dbus.Encoder
		xml.String(menuIntrospection)
		conn.Reply(call, "s", xml.Bytes())
		return
	}
	result := item.menu.Call(call)
	if result.ErrName != "" {
		conn.ReplyError(call, result.ErrName, result.ErrText)
		return
	}
	conn.Reply(call, result.Signature, result.Body)
	if result.LayoutUpdated != 0 {
		// A host that kept the ids of the last layout is told, the way the
		// interface says, that the menu it is showing was rebuilt and that its
		// ids no longer apply. The reply comes first, so a host that reads the
		// layout in answer to this signal sees the build this call made.
		var signal dbus.Encoder
		signal.Uint32(result.LayoutUpdated)
		signal.Int32(0)
		conn.Emit(dbusmenu.Path, dbusmenu.Interface, "LayoutUpdated", "ui", signal.Bytes())
	}
	if len(result.ItemsUpdated) != 0 {
		// The entries kept the ids the host holds; only what it draws changed.
		// The host applies the properties where they are, and does not have to
		// read the layout again to keep choosing entries by their ids.
		conn.Emit(dbusmenu.Path, dbusmenu.Interface, "ItemsPropertiesUpdated", dbusmenu.ItemsUpdatedSignature, result.ItemsUpdated)
	}
}

func (item *linuxStatusItem) handleMethod(call *dbus.Message) {
	conn := item.notifier.conn
	var button gfx.PointerButton
	switch call.Member {
	case "Activate":
		button = gfx.PointerButtonPrimary
	case "ContextMenu", "SecondaryActivate":
		button = gfx.PointerButtonSecondary
	case "Scroll":
		conn.Reply(call, "", nil)
		return
	default:
		conn.ReplyError(call, "org.freedesktop.DBus.Error.UnknownMethod", call.Member)
		return
	}
	args := call.Args()
	x, y := args.Int32(), args.Int32()
	if args.Err() != nil || call.Signature != "ii" {
		conn.ReplyError(call, "org.freedesktop.DBus.Error.InvalidArgs", "expected (ii)")
		return
	}
	conn.Reply(call, "", nil)
	if item.removed.Load() || item.onClick == nil {
		return
	}
	dismissed := time.Duration(monotonicNow()-panelDismissedAt.Load()) < panelDismissWindow
	// The host reports where the click was, not the icon's rectangle.
	item.onClick(platform.StatusClick{
		Button:         button,
		Anchor:         platform.ScreenRect{X: float32(x), Y: float32(y)},
		PanelDismissed: dismissed,
	})
}

func (item *linuxStatusItem) handleProperties(call *dbus.Message) {
	conn := item.notifier.conn
	args := call.Args()
	switch call.Member {
	case "Get":
		iface, name := args.String(), args.String()
		var out dbus.Encoder
		if args.Err() != nil || iface != itemInterface || !item.writeProperty(&out, name) {
			conn.ReplyError(call, "org.freedesktop.DBus.Error.UnknownProperty", name)
			return
		}
		conn.Reply(call, "v", out.Bytes())
	case "GetAll":
		iface := args.String()
		var out dbus.Encoder
		mark := out.BeginArray(8)
		if iface == itemInterface {
			for _, name := range propertyNames {
				out.Align(8)
				out.String(name)
				item.writeProperty(&out, name)
			}
		}
		out.EndArray(mark)
		conn.Reply(call, "a{sv}", out.Bytes())
	case "Set":
		conn.ReplyError(call, "org.freedesktop.DBus.Error.PropertyReadOnly", "read-only")
	default:
		conn.ReplyError(call, "org.freedesktop.DBus.Error.UnknownMethod", call.Member)
	}
}

var propertyNames = [...]string{
	"Category", "Id", "Title", "Status", "WindowId", "IconName", "IconPixmap",
	"OverlayIconName", "OverlayIconPixmap", "AttentionIconName", "AttentionIconPixmap",
	"AttentionMovieName", "ToolTip", "ItemIsMenu", "Menu",
}

// writeProperty writes the property as a variant, and reports whether the item
// has such a property.
func (item *linuxStatusItem) writeProperty(e *dbus.Encoder, name string) bool {
	switch name {
	case "Category":
		e.Variant("s")
		e.String("ApplicationStatus")
	case "Id":
		e.Variant("s")
		e.String(item.name)
	case "Title":
		e.Variant("s")
		e.String(item.tooltip)
	case "Status":
		e.Variant("s")
		e.String("Active")
	case "WindowId":
		e.Variant("u")
		e.Uint32(0)
	case "IconName", "OverlayIconName", "AttentionIconName", "AttentionMovieName":
		e.Variant("s")
		e.String("")
	case "IconPixmap":
		e.Variant("a(iiay)")
		item.writePixmaps(e)
	case "OverlayIconPixmap", "AttentionIconPixmap":
		e.Variant("a(iiay)")
		e.EndArray(e.BeginArray(8))
	case "ToolTip":
		e.Variant("(sa(iiay)ss)")
		e.Align(8)
		e.String("")
		e.EndArray(e.BeginArray(8))
		e.String(item.tooltip)
		e.String("")
	case "ItemIsMenu":
		// True asks a host to show the menu for a primary press too, and to
		// send no Activate. An icon that has a click handler of its own wants
		// that press, and the menu stays on the secondary one.
		e.Variant("b")
		e.Bool(item.menu != nil && item.onClick == nil)
	case "Menu":
		e.Variant("o")
		if item.menu != nil {
			e.String(dbusmenu.Path)
		} else {
			e.String("/NO_DBUSMENU")
		}
	default:
		return false
	}
	return true
}

// writePixmaps writes the icon at each size as (width, height, ARGB32 bytes).
func (item *linuxStatusItem) writePixmaps(e *dbus.Encoder) {
	mark := e.BeginArray(8)
	for _, size := range statusIconSizes {
		scaled := item.icon.Scaled(size)
		e.Align(8)
		e.Int32(int32(size))
		e.Int32(int32(size))
		data := e.BeginArray(1)
		for index := 0; index < len(scaled.RGBA); index += 4 {
			e.Byte(scaled.RGBA[index+3])
			e.Byte(scaled.RGBA[index])
			e.Byte(scaled.RGBA[index+1])
			e.Byte(scaled.RGBA[index+2])
		}
		e.EndArray(data)
	}
	e.EndArray(mark)
}

// menuIntrospection is what the icon's menu object answers to Introspect.
const menuIntrospection = `<!DOCTYPE node PUBLIC "-//freedesktop//DTD D-BUS Object Introspection 1.0//EN" "http://www.freedesktop.org/standards/dbus/1.0/introspect.dtd">
<node>
 <interface name="org.freedesktop.DBus.Introspectable"><method name="Introspect"><arg type="s" direction="out"/></method></interface>
 <interface name="org.freedesktop.DBus.Properties">
  <method name="Get"><arg type="s" direction="in"/><arg type="s" direction="in"/><arg type="v" direction="out"/></method>
  <method name="GetAll"><arg type="s" direction="in"/><arg type="a{sv}" direction="out"/></method>
  <method name="Set"><arg type="s" direction="in"/><arg type="s" direction="in"/><arg type="v" direction="in"/></method>
 </interface>
` + dbusmenu.Introspection + `
</node>`

const introspection = `<!DOCTYPE node PUBLIC "-//freedesktop//DTD D-BUS Object Introspection 1.0//EN" "http://www.freedesktop.org/standards/dbus/1.0/introspect.dtd">
<node>
 <interface name="org.freedesktop.DBus.Introspectable"><method name="Introspect"><arg type="s" direction="out"/></method></interface>
 <interface name="org.freedesktop.DBus.Properties">
  <method name="Get"><arg type="s" direction="in"/><arg type="s" direction="in"/><arg type="v" direction="out"/></method>
  <method name="GetAll"><arg type="s" direction="in"/><arg type="a{sv}" direction="out"/></method>
  <method name="Set"><arg type="s" direction="in"/><arg type="s" direction="in"/><arg type="v" direction="in"/></method>
 </interface>
 <interface name="org.kde.StatusNotifierItem">
  <property name="Category" type="s" access="read"/>
  <property name="Id" type="s" access="read"/>
  <property name="Title" type="s" access="read"/>
  <property name="Status" type="s" access="read"/>
  <property name="WindowId" type="u" access="read"/>
  <property name="IconName" type="s" access="read"/>
  <property name="IconPixmap" type="a(iiay)" access="read"/>
  <property name="OverlayIconName" type="s" access="read"/>
  <property name="OverlayIconPixmap" type="a(iiay)" access="read"/>
  <property name="AttentionIconName" type="s" access="read"/>
  <property name="AttentionIconPixmap" type="a(iiay)" access="read"/>
  <property name="AttentionMovieName" type="s" access="read"/>
  <property name="ToolTip" type="(sa(iiay)ss)" access="read"/>
  <property name="ItemIsMenu" type="b" access="read"/>
  <property name="Menu" type="o" access="read"/>
  <method name="ContextMenu"><arg type="i" direction="in"/><arg type="i" direction="in"/></method>
  <method name="Activate"><arg type="i" direction="in"/><arg type="i" direction="in"/></method>
  <method name="SecondaryActivate"><arg type="i" direction="in"/><arg type="i" direction="in"/></method>
  <method name="Scroll"><arg type="i" direction="in"/><arg type="s" direction="in"/></method>
  <signal name="NewTitle"/><signal name="NewIcon"/><signal name="NewToolTip"/><signal name="NewStatus"><arg type="s"/></signal>
 </interface>
</node>`

// SetIcon replaces the icon and tells the host. It must run on the event-loop
// thread.
func (item *linuxStatusItem) SetIcon(icon platform.StatusIcon) error {
	if !icon.Valid() {
		return errors.New("linux: status icon pixels do not match its size")
	}
	if item.removed.Load() {
		return errors.New("linux: status item is removed")
	}
	item.icon = icon
	return item.notifier.conn.Emit(itemPath, itemInterface, "NewIcon", "", nil)
}

// SetTooltip replaces the tooltip and tells the host. It must run on the
// event-loop thread.
func (item *linuxStatusItem) SetTooltip(tooltip string) {
	if item.removed.Load() || tooltip == item.tooltip {
		return
	}
	item.tooltip = tooltip
	conn := item.notifier.conn
	conn.Emit(itemPath, itemInterface, "NewTitle", "", nil)
	conn.Emit(itemPath, itemInterface, "NewToolTip", "", nil)
}

// Remove queues the item for removal and wakes the event loop, so it may be
// called from any goroutine.
func (item *linuxStatusItem) Remove() {
	if item.removed.Swap(true) {
		return
	}
	notifier := item.notifier
	notifier.mu.Lock()
	notifier.removals = append(notifier.removals, item.name)
	notifier.mu.Unlock()
	notifier.post(commandStatus)
}
