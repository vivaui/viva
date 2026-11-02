package linux

import (
	"encoding/binary"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

type minimumSizeCache struct {
	normalHints                             [18]uint32
	monitor                                 monitorGeometry
	scale                                   float32
	margin                                  uint32
	frameAtom                               uint32
	workAreaAtom, desktopAtom, geometryAtom uint32
	listening                               bool
	valid                                   bool
}

// Only screen/work-area or decoration changes invalidate the cached limits.
// In particular WM_NORMAL_HINTS, which we publish ourselves, does not.
func (win *window) minimumSizeProperty(config platform.Config, event *xcbPropertyEvent) {
	if config.MinWidth <= 0 && config.MinHeight <= 0 {
		return
	}
	cache := win.minimumSize
	if cache == nil {
		return
	}
	rootChanged := event.Window == win.screen.Root && event.Atom != 0 && (event.Atom == cache.workAreaAtom || event.Atom == cache.desktopAtom || event.Atom == cache.geometryAtom)
	if rootChanged || event.Window == win.id && event.Atom != 0 && event.Atom == cache.frameAtom {
		win.minimumSize.valid = false
		if event.Window == win.screen.Root && win.tracker != nil {
			win.tracker.reloadMonitors()
		}
		win.updateMinimumSize(config)
	}
}

// updateMinimumSize publishes ICCCM PMinSize without discarding a saved-position
// hint. X11 (including XWayland) lets the window manager enforce the border drag.
func (win *window) updateMinimumSize(config platform.Config) {
	win.updateMinimumSizeAt(config, 0, 0, false)
}

// updateMinimumSizeAt is updateMinimumSize with the window's root position
// already known: a ConfigureNotify that reported its frame passes the position
// it read (positioned), so a resize does not query the X server for it a second
// time. Otherwise the position is asked for only when it picks the monitor, which
// it does only on a desktop with several; a window on a lone monitor, or one whose
// cached hints still fit, never reaches the X server.
func (win *window) updateMinimumSizeAt(config platform.Config, rootX, rootY int32, positioned bool) {
	if config.MinWidth <= 0 && config.MinHeight <= 0 {
		return
	}
	if win.minimumSize == nil {
		win.minimumSize = &minimumSizeCache{}
	}
	monitors := win.placementMonitors()
	var monitor monitorGeometry
	if len(monitors) == 1 {
		monitor = monitors[0]
	} else {
		if !positioned {
			rootX, rootY = win.rootOrigin()
		}
		monitor, _ = monitorAt(monitors, int(rootX)+int(win.width)/2, int(rootY)+int(win.height)/2)
	}
	scale := win.displayScale()
	cached := win.minimumSize
	if cached.valid && cached.monitor == monitor && cached.scale == scale && cached.margin == win.margin {
		return
	}
	if !cached.listening {
		cached.workAreaAtom, _ = win.api.atom(win.connection, "_NET_WORKAREA")
		cached.desktopAtom, _ = win.api.atom(win.connection, "_NET_CURRENT_DESKTOP")
		cached.geometryAtom, _ = win.api.atom(win.connection, "_NET_DESKTOP_GEOMETRY")
		mask := uint32(xcbEventPropertyChange)
		win.api.changeWindow(win.connection, win.screen.Root, xcbCWEventMask, &mask)
		cached.listening = true
	}
	margin := int(win.margin) * 2
	extraWidth, extraHeight := margin, margin
	if config.WindowMode == platform.WindowTitled && win.tracker != nil {
		if atom, err := win.api.atom(win.connection, "_NET_FRAME_EXTENTS"); err == nil {
			cached.frameAtom = atom
			if data, ok := win.tracker.property(win.id, atom, xcbAtomCardinal); ok && len(data) >= 16 {
				extraWidth += int(binary.LittleEndian.Uint32(data)) + int(binary.LittleEndian.Uint32(data[4:]))
				extraHeight += int(binary.LittleEndian.Uint32(data[8:])) + int(binary.LittleEndian.Uint32(data[12:]))
			}
		}
	}
	var availableWidth, availableHeight int
	if monitor.Width > 0 && monitor.Height > 0 {
		work, haveWork := win.workArea()
		area := usableArea(monitorRect(monitor), work, haveWork)
		availableWidth, availableHeight = max(1, area.width-extraWidth), max(1, area.height-extraHeight)
	}
	width, height := config.MinimumContentSize(scale, availableWidth, availableHeight)
	hints := cached.normalHints
	hints[0] |= 1 << 4 // PMinSize
	// XSizeHints constrains both axes together. One pixel leaves an unset axis
	// unrestricted; the client-side shadow margin is outside the app's content.
	hints[5], hints[6] = uint32(max(1, width)+margin), uint32(max(1, height)+margin)
	if hints != cached.normalHints {
		cached.normalHints = hints
		win.api.changeProperty(win.connection, xcbPropertyReplace, win.id, xcbAtomWMNormalHints, xcbAtomWMSizeHints, 32, uint32(len(hints)), unsafe.Pointer(&hints[0]))
		win.api.flush(win.connection)
	}
	cached.monitor, cached.scale, cached.margin, cached.valid = monitor, scale, win.margin, true
}
