//go:build linux && !android && vivaxembed

package linux

import (
	"os"
	"strconv"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/vivaui/viva/internal/platform"
)

// GTK's XEmbed icon owns an input window and follows the tray manager
// across panel restarts. Load GTK only if this X screen has a tray.
type embeddedTrayAPI struct {
	initCheck    func(uintptr, uintptr) int32
	context      func() uintptr
	acquire      func(uintptr) int32
	release      func(uintptr)
	prepare      func(uintptr, *int32) int32
	query        func(uintptr, int32, *int32, *pollFD, int32) int32
	check        func(uintptr, int32, *pollFD, int32) int32
	dispatch     func(uintptr)
	newIcon      func() uintptr
	iconPixels   func(uintptr, uintptr)
	iconTooltip  func(uintptr, *byte)
	iconTitle    func(uintptr, *byte)
	iconVisible  func(uintptr, int32)
	iconXID      func(uintptr) uint32
	iconGeometry func(uintptr, uintptr, *[4]int32, uintptr) int32
	display      func() uintptr
	x11Type      func() uintptr
	isA          func(uintptr, uintptr) int32
	xwindow      func(uintptr, uintptr) uintptr
	windowWidget func(uintptr, *uintptr)
	windowScale  func(uintptr) int32
	addEvents    func(uintptr, int32)
	connect      func(uintptr, *byte, uintptr, uintptr, uintptr, int32) uint64
	pixbuf       func(int32, int32, int32, int32, int32) uintptr
	pixels       func(uintptr) *byte
	stride       func(uintptr) int32
	unref        func(uintptr)
	refSink      func(uintptr) uintptr
	newMenu      func() uintptr
	menuAppend   func(uintptr, uintptr)
	menuItem     func(*byte) uintptr
	checkItem    func(*byte) uintptr
	setChecked   func(uintptr, int32)
	separator    func() uintptr
	submenu      func(uintptr, uintptr)
	sensitive    func(uintptr, int32)
	showAll      func(uintptr)
	destroy      func(uintptr)
	popup        func(uintptr, uintptr, uintptr, uintptr, uintptr, uint32, uint32)
	positionMenu uintptr
}

var loadEmbeddedTray = sync.OnceValue(func() *embeddedTrayAPI {
	lib, err := purego.Dlopen("libgtk-3.so.0", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil
	}
	g := new(embeddedTrayAPI)
	for _, sym := range []struct {
		name string
		dst  any
	}{
		{"gtk_init_check", &g.initCheck},
		{"g_main_context_default", &g.context},
		{"g_main_context_acquire", &g.acquire},
		{"g_main_context_release", &g.release},
		{"g_main_context_prepare", &g.prepare},
		{"g_main_context_query", &g.query},
		{"g_main_context_check", &g.check},
		{"g_main_context_dispatch", &g.dispatch},
		{"gtk_status_icon_new", &g.newIcon},
		{"gtk_status_icon_set_from_pixbuf", &g.iconPixels},
		{"gtk_status_icon_set_tooltip_text", &g.iconTooltip},
		{"gtk_status_icon_set_title", &g.iconTitle},
		{"gtk_status_icon_set_visible", &g.iconVisible},
		{"gtk_status_icon_get_x11_window_id", &g.iconXID},
		{"gtk_status_icon_get_geometry", &g.iconGeometry},
		{"gdk_display_get_default", &g.display},
		{"gdk_x11_display_get_type", &g.x11Type},
		{"g_type_check_instance_is_a", &g.isA},
		{"gdk_x11_window_lookup_for_display", &g.xwindow},
		{"gdk_window_get_user_data", &g.windowWidget},
		{"gdk_window_get_scale_factor", &g.windowScale},
		{"gtk_widget_add_events", &g.addEvents},
		{"g_signal_connect_data", &g.connect},
		{"gdk_pixbuf_new", &g.pixbuf},
		{"gdk_pixbuf_get_pixels", &g.pixels},
		{"gdk_pixbuf_get_rowstride", &g.stride},
		{"g_object_unref", &g.unref},
		{"g_object_ref_sink", &g.refSink},
		{"gtk_menu_new", &g.newMenu},
		{"gtk_menu_shell_append", &g.menuAppend},
		{"gtk_menu_item_new_with_label", &g.menuItem},
		{"gtk_check_menu_item_new_with_label", &g.checkItem},
		{"gtk_check_menu_item_set_active", &g.setChecked},
		{"gtk_separator_menu_item_new", &g.separator},
		{"gtk_menu_item_set_submenu", &g.submenu},
		{"gtk_widget_set_sensitive", &g.sensitive},
		{"gtk_widget_show_all", &g.showAll},
		{"gtk_widget_destroy", &g.destroy},
		{"gtk_menu_popup", &g.popup},
	} {
		s, err := purego.Dlsym(lib, sym.name)
		if err != nil {
			purego.Dlclose(lib)
			return nil
		}
		purego.RegisterFunc(sym.dst, s)
	}
	if g.positionMenu, err = purego.Dlsym(lib, "gtk_status_icon_position_menu"); err != nil {
		purego.Dlclose(lib)
		return nil
	}
	return g
})

type trayApplication struct{ trayGTK *embeddedTrayLoop }
type trayWindow struct{ hoverItem *embeddedTrayItem }

func (s *trayWindow) detachTrayPanel() {
	if s.hoverItem != nil {
		s.hoverItem.detachPanel()
	}
}
func (s *trayWindow) trayPanelCrossing(inside bool) {
	if s.hoverItem != nil {
		s.hoverItem.panelCrossing(inside)
	}
}
func (a *trayApplication) prepareTrayWait(fds []pollFD, timeout int32) ([]pollFD, int32) {
	if a.trayGTK == nil {
		return fds, timeout
	}
	return a.trayGTK.prepare(fds, timeout)
}
func (a *trayApplication) dispatchTray(ready, fds []pollFD) {
	if a.trayGTK != nil {
		a.trayGTK.dispatch(ready, fds)
	}
}
func (a *linuxApplication) openEmbeddedTray(config platform.StatusItemConfig) (platform.StatusItem, error) {
	if item := a.addEmbeddedTray(config); item != nil {
		return item, nil
	}
	return nil, platform.ErrStatusAreaUnavailable
}

type embeddedTrayLoop struct {
	app      *linuxApplication
	g        *embeddedTrayAPI
	context  uintptr
	priority int32
	fds      []pollFD
	items    map[uintptr]*embeddedTrayItem
}

func (a *linuxApplication) addEmbeddedTray(config platform.StatusItemConfig) *embeddedTrayItem {
	session := os.Getenv("XDG_SESSION_TYPE")
	if session == "wayland" || session == "" && os.Getenv("WAYLAND_DISPLAY") != "" || len(a.windows) == 0 {
		return nil
	}
	if a.trayGTK == nil {
		win := a.windows[0].win
		// The tray selection is per X screen, not per RandR monitor.
		iter := win.api.rootsIterator(win.api.getSetup(win.connection))
		screen := 0
		for iter.Remaining > 0 && iter.Data.Root != win.screen.Root {
			win.api.screenNext(&iter)
			screen++
		}
		at, err := win.api.atom(win.connection, "_NET_SYSTEM_TRAY_S"+strconv.Itoa(screen))
		if err != nil {
			return nil
		}
		reply := win.api.selectionOwnerReply(win.connection, win.api.getSelectionOwner(win.connection, at), nil)
		if reply == nil {
			return nil
		}
		manager := reply.Owner
		win.api.free(unsafe.Pointer(reply))
		if manager == 0 {
			return nil
		}
		g := loadEmbeddedTray()
		if g == nil || g.initCheck(0, 0) == 0 {
			return nil
		}
		if display := g.display(); display == 0 || g.isA(display, g.x11Type()) == 0 {
			return nil
		}
		ctx := g.context()
		if g.acquire(ctx) == 0 {
			return nil
		}
		a.trayGTK = &embeddedTrayLoop{app: a, g: g, context: ctx, items: make(map[uintptr]*embeddedTrayItem)}
	}
	item := a.trayGTK.add(config)
	if item == nil && len(a.trayGTK.items) == 0 {
		a.closeEmbeddedTray()
	}
	return item
}

// Merge GLib's descriptors and next deadline into the existing blocking poll.
func (l *embeddedTrayLoop) prepare(appFDs []pollFD, timeout int32) ([]pollFD, int32) {
	pending := l.g.prepare(l.context, &l.priority) != 0
	var deadline int32
	n := len(appFDs)
	for {
		if cap(l.fds) <= n {
			l.fds = make([]pollFD, n+8)
		}
		fds := l.fds[:cap(l.fds)]
		count := int(l.g.query(l.context, l.priority, &deadline, &fds[n], int32(len(fds)-n)))
		if n+count > len(fds) {
			l.fds = make([]pollFD, n+count)
			continue
		}
		copy(fds, appFDs)
		if pending {
			timeout = 0
		} else if deadline >= 0 && (timeout < 0 || deadline < timeout) {
			timeout = deadline
		}
		return fds[:n+count], timeout
	}
}

func (l *embeddedTrayLoop) dispatch(ready, appFDs []pollFD) {
	copy(appFDs, ready[:len(appFDs)])
	gtkFDs := ready[len(appFDs):]
	if l.g.check(l.context, l.priority, unsafe.SliceData(gtkFDs), int32(len(gtkFDs))) != 0 {
		l.g.dispatch(l.context)
	}
}

func (a *linuxApplication) closeEmbeddedTray() {
	if l := a.trayGTK; l != nil {
		for _, i := range l.items {
			i.dispose()
		}
		l.g.release(l.context)
		a.trayGTK = nil
	}
}
