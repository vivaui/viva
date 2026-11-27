//go:build !android

package linux

import (
	"os"
	"strconv"
	"unsafe"

	"github.com/ebitengine/purego"
)

// The X11 side of display scale discovery (display_scale.go decides, this
// reads): the XSETTINGS manager, the resource database and RandR. Each window
// has one tracker on its own connection. Nothing here polls: the settings are
// read when the window opens, again when the manager's property changes (the
// tracker listens to that one window) and when the window gains the focus, and
// the monitors are looked at again only when the window moves.

const (
	xcbConfigWindowX      = 1 << 0
	xcbConfigWindowY      = 1 << 1
	xcbConfigWindowWidth  = 1 << 2
	xcbConfigWindowHeight = 1 << 3
	// settingsWords is the most of an XSETTINGS property that is read, in
	// 32-bit units: 256 KiB, far beyond any real manager.
	settingsWords = 65536
)

type scaleTracker struct {
	api        *xcbAPI
	connection uintptr
	screen     *xcbScreen
	// environment is read once: it cannot change under a running process.
	environment scaleInputs

	managerAtom, settingsAtom, resourceAtom uint32
	owner                                   uint32

	settings xsettings
	resource string
	monitors []monitorGeometry
	randr    *randrAPI
	// origin is where the last answer came from; only RandR depends on where
	// the window is.
	origin scaleOrigin
}

// newScaleTracker reads the display's settings. A failure to read any one of them
// leaves it out of the choice.
func newScaleTracker(api *xcbAPI, connection uintptr, screen *xcbScreen, screenIndex int32) *scaleTracker {
	tracker := &scaleTracker{api: api, connection: connection, screen: screen}
	tracker.environment = scaleInputs{
		VivaScale:            os.Getenv("VIVA_SCALE"),
		QtScaleFactor:        os.Getenv("QT_SCALE_FACTOR"),
		QtScreenScaleFactors: os.Getenv("QT_SCREEN_SCALE_FACTORS"),
		GdkScale:             os.Getenv("GDK_SCALE"),
	}
	tracker.managerAtom, _ = api.atom(connection, "_XSETTINGS_S"+strconv.Itoa(int(screenIndex)))
	tracker.settingsAtom, _ = api.atom(connection, "_XSETTINGS_SETTINGS")
	tracker.resourceAtom, _ = api.atom(connection, "RESOURCE_MANAGER")
	tracker.randr = loadRandR(api, connection)
	tracker.reload()
	tracker.reloadMonitors()
	return tracker
}

// close releases the RandR library.
func (tracker *scaleTracker) close() {
	if tracker.randr != nil {
		purego.Dlclose(tracker.randr.library)
		tracker.randr = nil
	}
}

// reload reads the XSETTINGS manager's settings and the resource database. It
// reports whether the XSETTINGS or the resource text differ from before.
func (tracker *scaleTracker) reload() bool {
	api, connection := tracker.api, tracker.connection
	changed := false
	settings := xsettings{}
	if tracker.managerAtom != 0 && tracker.settingsAtom != 0 {
		cookie := api.getSelectionOwner(connection, tracker.managerAtom)
		var nativeError unsafe.Pointer
		if reply := api.selectionOwnerReply(connection, cookie, &nativeError); reply != nil {
			owner := reply.Owner
			api.free(unsafe.Pointer(reply))
			if owner != 0 && owner != tracker.owner {
				// Listen to the manager: it rewrites the property when the user
				// changes the scale.
				mask := uint32(xcbEventPropertyChange)
				api.changeWindow(connection, owner, xcbCWEventMask, &mask)
				api.flush(connection)
			}
			tracker.owner = owner
			if owner != 0 {
				if blob, ok := tracker.property(owner, tracker.settingsAtom, xcbAtomAny); ok {
					if parsed, err := parseXSettings(blob); err == nil {
						settings = parsed
					}
				}
			}
		}
		if nativeError != nil {
			api.free(nativeError)
		}
	}
	if settings != tracker.settings {
		tracker.settings, changed = settings, true
	}
	resource := ""
	if tracker.resourceAtom != 0 {
		if text, ok := tracker.property(tracker.screen.Root, tracker.resourceAtom, xcbAtomString); ok {
			resource = string(text)
		}
	}
	if resource != tracker.resource {
		tracker.resource, changed = resource, true
	}
	return changed
}

// property reads a whole property of a window. ok is false when it is absent.
func (tracker *scaleTracker) property(window, property, kind uint32) ([]byte, bool) {
	api, connection := tracker.api, tracker.connection
	cookie := api.getProperty(connection, 0, window, property, kind, 0, settingsWords)
	var nativeError unsafe.Pointer
	reply := api.getPropertyReply(connection, cookie, &nativeError)
	if nativeError != nil {
		api.free(nativeError)
	}
	if reply == nil {
		return nil, false
	}
	defer api.free(unsafe.Pointer(reply))
	length := int(api.getPropertyLength(reply))
	if length <= 0 || reply.BytesAfter != 0 {
		return nil, false
	}
	return append([]byte(nil), unsafe.Slice((*byte)(api.getPropertyValue(reply)), length)...), true
}

// reloadMonitors asks RandR for the monitors. Without RandR 1.5 the whole screen
// stands for one monitor, with the physical size the core protocol reports.
func (tracker *scaleTracker) reloadMonitors() {
	tracker.monitors = tracker.monitors[:0]
	if tracker.randr != nil {
		tracker.monitors = tracker.randr.monitors(tracker.api, tracker.connection, tracker.screen.Root, tracker.monitors)
	}
	if len(tracker.monitors) == 0 {
		screen := tracker.screen
		tracker.monitors = append(tracker.monitors, monitorGeometry{
			Width: int(screen.WidthPixels), Height: int(screen.HeightPixels),
			WidthMM: int(screen.WidthMillimeters), HeightMM: int(screen.HeightMillimeters), Primary: true, valid: true,
		})
	}
}

// scale is the display scale for a window whose center is at (x, y) in root
// coordinates.
func (tracker *scaleTracker) scale(x, y int) float32 {
	inputs := tracker.environment
	inputs.XSettings = tracker.settings
	inputs.ResourceDPI = resourceDPI(tracker.resource)
	monitor, _ := monitorAt(tracker.monitors, x, y)
	inputs.Monitor = monitor
	inputs.Screen = monitor.Name
	scale, origin := chooseScale(inputs)
	tracker.origin = origin
	return scale
}

// followsPosition says whether moving the window can change the scale.
func (tracker *scaleTracker) followsPosition() bool {
	return tracker.origin == scaleFromRandR && len(tracker.monitors) > 1
}

// settingsEvent says whether a PropertyNotify is the manager rewriting its
// settings.
func (tracker *scaleTracker) settingsEvent(event *xcbPropertyEvent) bool {
	return tracker.owner != 0 && event.Window == tracker.owner && event.Atom == tracker.settingsAtom
}

// cursorSize is the cursor size the resource database asks for, 0 for none.
func (tracker *scaleTracker) cursorSize() int { return resourceCursorSize(tracker.resource) }

// randrAPI is the part of libxcb-randr used for monitors. The library is
// optional: a machine without it uses the core protocol's screen size.
type randrAPI struct {
	library     uintptr
	queryVer    func(uintptr, uint32, uint32) xcbVoidCookie
	versionRep  func(uintptr, xcbVoidCookie, *unsafe.Pointer) *randrVersionReply
	getMonitors func(uintptr, uint32, uint8) xcbVoidCookie
	monitorsRep func(uintptr, xcbVoidCookie, *unsafe.Pointer) *randrMonitorsReply
	iterator    func(*randrMonitorsReply) randrMonitorIterator
	next        func(*randrMonitorIterator)
}

type randrVersionReply struct {
	ResponseType uint8
	Padding      uint8
	Sequence     uint16
	Length       uint32
	Major, Minor uint32
	Pad          [16]byte
}

type randrMonitorsReply struct {
	ResponseType uint8
	Padding      uint8
	Sequence     uint16
	Length       uint32
	Timestamp    uint32
	Monitors     uint32
	Outputs      uint32
	Pad          [12]byte
}

type randrMonitorInfo struct {
	Name          uint32
	Primary       uint8
	Automatic     uint8
	Outputs       uint16
	X, Y          int16
	Width, Height uint16
	WidthMM       uint32
	HeightMM      uint32
}

type randrMonitorIterator struct {
	Data  *randrMonitorInfo
	Rem   int32
	Index int32
}

type xcbExtensionReply struct {
	ResponseType uint8
	Padding      uint8
	Sequence     uint16
	Length       uint32
	Present      uint8
	MajorOpcode  uint8
	FirstEvent   uint8
	FirstError   uint8
}

// loadRandR opens libxcb-randr and checks the server speaks RandR 1.5. A request
// for an extension the server lacks would shut the connection down, so the
// extension is looked up first.
func loadRandR(api *xcbAPI, connection uintptr) *randrAPI {
	library, err := purego.Dlopen("libxcb-randr.so.0", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil
	}
	randr := &randrAPI{library: library}
	identity, err := purego.Dlsym(library, "xcb_randr_id")
	if err != nil || identity == 0 {
		purego.Dlclose(library)
		return nil
	}
	var extensionData func(uintptr, uintptr) *xcbExtensionReply
	purego.RegisterLibFunc(&extensionData, api.library, "xcb_get_extension_data")
	present := extensionData(connection, identity)
	if present == nil || present.Present == 0 {
		purego.Dlclose(library)
		return nil
	}
	purego.RegisterLibFunc(&randr.queryVer, library, "xcb_randr_query_version")
	purego.RegisterLibFunc(&randr.versionRep, library, "xcb_randr_query_version_reply")
	purego.RegisterLibFunc(&randr.getMonitors, library, "xcb_randr_get_monitors")
	purego.RegisterLibFunc(&randr.monitorsRep, library, "xcb_randr_get_monitors_reply")
	purego.RegisterLibFunc(&randr.iterator, library, "xcb_randr_get_monitors_monitors_iterator")
	purego.RegisterLibFunc(&randr.next, library, "xcb_randr_monitor_info_next")
	var nativeError unsafe.Pointer
	version := randr.versionRep(connection, randr.queryVer(connection, 1, 5), &nativeError)
	if nativeError != nil {
		api.free(nativeError)
	}
	if version == nil {
		purego.Dlclose(library)
		return nil
	}
	supported := version.Major > 1 || version.Major == 1 && version.Minor >= 5
	api.free(unsafe.Pointer(version))
	if !supported {
		purego.Dlclose(library)
		return nil
	}
	return randr
}

// monitors appends the active monitors to list.
func (randr *randrAPI) monitors(api *xcbAPI, connection uintptr, root uint32, list []monitorGeometry) []monitorGeometry {
	var nativeError unsafe.Pointer
	reply := randr.monitorsRep(connection, randr.getMonitors(connection, root, 1), &nativeError)
	if nativeError != nil {
		api.free(nativeError)
	}
	if reply == nil {
		return list
	}
	defer api.free(unsafe.Pointer(reply))
	iterator := randr.iterator(reply)
	for ; iterator.Rem > 0 && iterator.Data != nil; randr.next(&iterator) {
		info := iterator.Data
		monitor := monitorGeometry{
			Name: atomName(api, connection, info.Name),
			X:    int(info.X), Y: int(info.Y), Width: int(info.Width), Height: int(info.Height),
			WidthMM: int(info.WidthMM), HeightMM: int(info.HeightMM), Primary: info.Primary != 0, valid: true,
		}
		list = append(list, monitor)
	}
	return list
}

// atomName is the name of an atom, empty when the server does not answer.
func atomName(api *xcbAPI, connection uintptr, atom uint32) string {
	if atom == 0 || api.getAtomName == nil {
		return ""
	}
	var nativeError unsafe.Pointer
	reply := api.getAtomNameReply(connection, api.getAtomName(connection, atom), &nativeError)
	if nativeError != nil {
		api.free(nativeError)
	}
	if reply == nil {
		return ""
	}
	defer api.free(unsafe.Pointer(reply))
	length := int(api.atomNameLength(reply))
	if length <= 0 {
		return ""
	}
	return string(unsafe.Slice(api.atomNameValue(reply), length))
}
