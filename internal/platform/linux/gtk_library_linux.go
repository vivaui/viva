//go:build linux && vivagtk

package linux

import (
	"fmt"
	"strings"
	"sync"

	"github.com/ebitengine/purego"
)

// The GTK 3 shell (ADR 0064) reaches GTK, GDK, GLib, GObject and cairo through the system's shared
// libraries, loaded when the first window opens, the way the XCB backend reaches libxcb: a program
// that never opens a window on this backend never needs them, and nothing is linked.

// gtkLibrary holds the entry points the shell uses. Every field is bound by loadGTK; a missing
// symbol fails the load with its name, so an incompatible GTK is reported once and not at the first
// call.
type gtkLibrary struct {
	libraries []uintptr
	err       error

	// GLib and GObject.
	signalConnect         func(instance uintptr, signal *byte, handler, data, destroy uintptr, flags int32) uint64
	signalDisc            func(instance uintptr, id uint64)
	idleAdd               func(function, data uintptr) uint32
	timeoutAdd            func(interval uint32, function, data uintptr) uint32
	sourceRemove          func(id uint32) int32
	gFree                 func(pointer uintptr)
	objectUnref           func(object uintptr)
	setPrgname            func(name *byte)
	typeName              func(gtype uintptr) uintptr
	slistFree             func(list uintptr)
	attrListUnref         func(list uintptr)
	attrListGetAttributes func(list uintptr) uintptr
	unixFDAdd             func(fd int32, condition uint32, function, data uintptr) uint32

	// GTK: application and windows.
	initCheck           func(argc *int32, argv uintptr) int32
	main                func()
	mainQuit            func()
	windowNew           func(kind int32) uintptr
	windowSetTitle      func(window uintptr, title *byte)
	windowSetDefault    func(window uintptr, width, height int32)
	windowResize        func(window uintptr, width, height int32)
	windowMove          func(window uintptr, x, y int32)
	windowSetDecorated  func(window uintptr, decorated int32)
	windowSetGeometry   func(window, widget uintptr, geometry *gdkGeometry, mask uint32)
	windowSetTypeHint   func(window uintptr, hint int32)
	windowSkipTaskbar   func(window uintptr, skip int32)
	windowKeepAbove     func(window uintptr, keep int32)
	windowAcceptFocus   func(window uintptr, accept int32)
	windowSetWMClass    func(window uintptr, name, class *byte)
	windowSetIconList   func(window, list uintptr)
	windowIconify       func(window uintptr)
	windowDeiconify     func(window uintptr)
	windowMaximize      func(window uintptr)
	windowUnmaximize    func(window uintptr)
	windowIsMaximized   func(window uintptr) int32
	windowPresent       func(window uintptr)
	windowGetPosition   func(window uintptr, x, y *int32)
	windowGetSize       func(window uintptr, width, height *int32)
	windowBeginMove     func(window uintptr, button, rootX, rootY int32, timestamp uint32)
	windowBeginResize   func(window uintptr, edge, button, rootX, rootY int32, timestamp uint32)
	windowSetFocus      func(window, widget uintptr)
	windowSetHasFrame   func(window uintptr, frame int32)
	windowSetTransient  func(window, parent uintptr)
	windowSetAppPaint   func(widget uintptr, paintable int32)
	windowSetVisual     func(widget, visual uintptr)
	windowSetIcon       func(window, pixbuf uintptr)
	windowSetTitlebar   func(window, titlebar uintptr)
	windowGetModalHint  func(window uintptr) int32
	widgetShow          func(widget uintptr)
	widgetShowAll       func(widget uintptr)
	widgetHide          func(widget uintptr)
	widgetDestroy       func(widget uintptr)
	widgetRealize       func(widget uintptr)
	widgetQueueDraw     func(widget uintptr)
	widgetQueueDrawArea func(widget uintptr, x, y, width, height int32)
	widgetGetWindow     func(widget uintptr) uintptr
	widgetAddEvents     func(widget uintptr, events int32)
	widgetSetEvents     func(widget uintptr, events int32)
	widgetSetSize       func(widget uintptr, width, height int32)
	widgetSetCanFocus   func(widget uintptr, can int32)
	widgetGrabFocus     func(widget uintptr)
	widgetAllocW        func(widget uintptr) int32
	widgetAllocH        func(widget uintptr) int32
	widgetGetScale      func(widget uintptr) int32
	widgetIsVisible     func(widget uintptr) int32
	widgetTranslate     func(src, dest uintptr, x, y int32, destX, destY *int32) int32
	widgetGetToplevel   func(widget uintptr) uintptr
	widgetGetScreen     func(widget uintptr) uintptr
	widgetGetDisplay    func(widget uintptr) uintptr
	widgetSetHExpand    func(widget uintptr, expand int32)
	widgetSetVExpand    func(widget uintptr, expand int32)
	widgetSetName       func(widget uintptr, name *byte)
	widgetStyleContext  func(widget uintptr) uintptr
	containerAdd        func(container, widget uintptr)
	overlayNew          func() uintptr
	eventBoxNew         func() uintptr
	eventBoxVisible     func(box uintptr, visible int32)
	overlayAdd          func(overlay, widget uintptr)
	widgetSetHAlign     func(widget uintptr, align int32)
	widgetSetVAlign     func(widget uintptr, align int32)
	widgetSetMarginX    func(widget uintptr, margin int32)
	widgetSetMarginY    func(widget uintptr, margin int32)
	errorFree           func(err uintptr)
	drawingAreaNew      func() uintptr
	fixedNew            func() uintptr
	cssProviderNew      func() uintptr
	cssProviderLoad     func(provider uintptr, data *byte, length int64, err uintptr) int32
	styleAddProvider    func(context, provider uintptr, priority uint32)
	styleAddProviderScr func(screen, provider uintptr, priority uint32)

	// GTK: input methods.
	dragDestSet       func(widget uintptr, flags int32, targets uintptr, count int32, actions int32)
	dragDestAddURIs   func(widget uintptr)
	dragGetData       func(widget, context, target uintptr, time uint32)
	dragFinish        func(context uintptr, success, del int32, time uint32)
	dragStatus        func(context uintptr, action int32, time uint32)
	selectionData     func(data uintptr) uintptr
	selectionLength   func(data uintptr) int32
	atomIntern        func(name *byte, onlyIfExists int32) uintptr
	imMulticontextNew func() uintptr
	imSetClientWindow func(ctx, window uintptr)
	imFocusIn         func(ctx uintptr)
	imFocusOut        func(ctx uintptr)
	imFilterKeypress  func(ctx, event uintptr) int32
	imGetPreedit      func(ctx uintptr, text, attrs *uintptr, cursor *int32)
	imSetCursorRect   func(ctx uintptr, rect *[4]int32)
	imReset           func(ctx uintptr)
	imSetUsePreedit   func(ctx uintptr, use int32)
	imSetSurrounding  func(ctx uintptr, text *byte, length, cursor int32)

	// GDK.
	gdkDisplayDefault    func() uintptr
	gdkDisplayPrimary    func(display uintptr) uintptr
	gdkMonitorScale      func(monitor uintptr) int32
	gdkMonitorWidthMM    func(monitor uintptr) int32
	gdkMonitorHeightMM   func(monitor uintptr) int32
	gdkScreenResolution  func(screen uintptr) float64
	gdkEventSourceDevice func(event uintptr) uintptr
	gdkDeviceSource      func(device uintptr) int32
	gdkScreenDefault     func() uintptr
	gdkDisplayName       func(display uintptr) uintptr
	gdkDisplaySeat       func(display uintptr) uintptr
	gdkDisplayMonitorAt  func(display, window uintptr) uintptr
	gdkMonitorGeometry   func(monitor uintptr, rect *[4]int32)
	gdkWindowSetCursor   func(window, cursor uintptr)
	gdkCursorFromName    func(display uintptr, name *byte) uintptr
	gdkCursorFromPixbuf  func(display, pixbuf uintptr, x, y int32) uintptr
	gdkWindowOrigin      func(window uintptr, x, y *int32) int32
	gdkWindowRootOrigin  func(window uintptr, x, y *int32)
	gdkWindowInputShape  func(window, region uintptr, dx, dy int32)
	gdkWindowNative      func(window uintptr)
	gdkWindowScale       func(window uintptr) int32
	gdkWindowSetOpaque   func(window, region uintptr)
	gdkKeyvalToUnicode   func(keyval uint32) uint32
	gdkKeymapDefault     func() uintptr
	gdkKeymapEntries     func(keymap uintptr, keycode uint32, keys *uintptr, keyvals *uintptr, count *int32) int32
	gdkSetProgramClass   func(class *byte)
	gdkScreenRGBAVisual  func(screen uintptr) uintptr
	gdkScreenComposited  func(screen uintptr) int32
	gdkSeatPointer       func(seat uintptr) uintptr
	gdkDeviceWindowAt    func(device uintptr, x, y *int32) uintptr
	gdkDevicePosition    func(device uintptr, screen *uintptr, x, y *int32)
	gdkPixbufNewData     func(data uintptr, colorspace, alpha, bits, width, height, stride int32, destroy, destroyData uintptr) uintptr
	settingsDefault      func() uintptr
	objectGetProperty    func(object uintptr, name *byte, value *gValue)
	valueInit            func(value *gValue, gtype uintptr) uintptr
	valueInt             func(value *gValue) int32
	valueString          func(value *gValue) uintptr
	valueBool            func(value *gValue) int32
	valueUnset           func(value *gValue)
	listAppend           func(list, data uintptr) uintptr
	listFree             func(list uintptr)

	// cairo.
	cairoSurfaceForData func(data *byte, format, width, height, stride int32) uintptr
	cairoSurfaceDestroy func(surface uintptr)
	cairoSurfaceFlush   func(surface uintptr)
	cairoSurfaceDirty   func(surface uintptr)
	cairoDeviceScale    func(surface uintptr, x, y float64)
	cairoSetSource      func(cr, surface uintptr, x, y float64)
	cairoSetOperator    func(cr uintptr, operator int32)
	cairoRectangle      func(cr uintptr, x, y, width, height float64)
	cairoClip           func(cr uintptr)
	cairoFill           func(cr uintptr)
	cairoPaint          func(cr uintptr)
	cairoSave           func(cr uintptr)
	cairoRestore        func(cr uintptr)
	cairoClipExtents    func(cr uintptr, x1, y1, x2, y2 *float64)
	cairoRegionRect     func(rect *[4]int32) uintptr
	cairoRegionSubtract func(region uintptr, rect *[4]int32) int32
	cairoRegionUnion    func(region uintptr, rect *[4]int32) int32
	cairoRegionDestroy  func(region uintptr)
}

// gValue is GValue: a type and 16 bytes of data.
type gValue struct {
	gtype uintptr
	data  [2]uint64
}

// GType of the fundamental types GtkSettings properties have.
const (
	gTypeBoolean = 5 << 2
	gTypeInt     = 6 << 2
	gTypeString  = 16 << 2
)

// gdkGeometry is GdkGeometry.
type gdkGeometry struct {
	MinWidth, MinHeight   int32
	MaxWidth, MaxHeight   int32
	BaseWidth, BaseHeight int32
	WidthInc, HeightInc   int32
	MinAspect, MaxAspect  float64
	WinGravity            int32
	_                     [4]byte
}

const (
	gdkHintMinSize = 1 << 1
	gdkHintPos     = 1 << 0

	cairoFormatARGB32 = 0
	cairoOperatorOver = 2
	cairoOperatorSrc  = 1

	gioIn = 1
)

var (
	gtkOnce    sync.Once
	gtkLoaded  *gtkLibrary
	gtkLoadErr error
)

// loadGTK opens the libraries once. The error names what to install.
func loadGTK() (*gtkLibrary, error) {
	gtkOnce.Do(func() {
		g := &gtkLibrary{}
		for _, name := range []string{"libglib-2.0.so.0", "libgobject-2.0.so.0", "libcairo.so.2", "libpango-1.0.so.0", "libgdk-3.so.0", "libgtk-3.so.0"} {
			handle, err := purego.Dlopen(name, purego.RTLD_NOW|purego.RTLD_GLOBAL)
			if err != nil {
				gtkLoadErr = fmt.Errorf("linux: GTK 3 is not installed (cannot open %s): install libgtk-3-0 (Debian, Ubuntu) or gtk3 (Fedora, Arch): %w", name, err)
				return
			}
			g.libraries = append(g.libraries, handle)
		}
		g.bindAll()
		if g.err != nil {
			gtkLoadErr = g.err
			return
		}
		gtkLoaded = g
	})
	return gtkLoaded, gtkLoadErr
}

// bind registers one symbol, found in whichever library has it. The first missing symbol is kept.
func (g *gtkLibrary) bind(function any, name string) {
	if g.err != nil {
		return
	}
	for _, library := range g.libraries {
		if _, err := purego.Dlsym(library, name); err == nil {
			purego.RegisterLibFunc(function, library, name)
			return
		}
	}
	g.err = fmt.Errorf("linux: the installed GTK lacks %s (GTK 3.22 or newer is required)", name)
}

// lookup registers one symbol if some library has it, and says whether one did.
func (g *gtkLibrary) lookup(function any, name string) bool {
	for _, library := range g.libraries {
		if _, err := purego.Dlsym(library, name); err == nil {
			purego.RegisterLibFunc(function, library, name)
			return true
		}
	}
	return false
}

func (g *gtkLibrary) bindAll() {
	b := g.bind
	b(&g.signalConnect, "g_signal_connect_data")
	b(&g.signalDisc, "g_signal_handler_disconnect")
	b(&g.idleAdd, "g_idle_add")
	b(&g.timeoutAdd, "g_timeout_add")
	b(&g.sourceRemove, "g_source_remove")
	b(&g.gFree, "g_free")
	b(&g.objectUnref, "g_object_unref")
	b(&g.setPrgname, "g_set_prgname")
	b(&g.typeName, "g_type_name")
	b(&g.slistFree, "g_slist_free")
	b(&g.attrListUnref, "pango_attr_list_unref")
	b(&g.attrListGetAttributes, "pango_attr_list_get_attributes")
	b(&g.unixFDAdd, "g_unix_fd_add")

	b(&g.initCheck, "gtk_init_check")
	b(&g.main, "gtk_main")
	b(&g.mainQuit, "gtk_main_quit")
	b(&g.windowNew, "gtk_window_new")
	b(&g.windowSetTitle, "gtk_window_set_title")
	b(&g.windowSetDefault, "gtk_window_set_default_size")
	b(&g.windowResize, "gtk_window_resize")
	b(&g.windowMove, "gtk_window_move")
	b(&g.windowSetDecorated, "gtk_window_set_decorated")
	b(&g.windowSetGeometry, "gtk_window_set_geometry_hints")
	b(&g.windowSetTypeHint, "gtk_window_set_type_hint")
	b(&g.windowSkipTaskbar, "gtk_window_set_skip_taskbar_hint")
	b(&g.windowKeepAbove, "gtk_window_set_keep_above")
	b(&g.windowAcceptFocus, "gtk_window_set_accept_focus")
	b(&g.windowSetWMClass, "gtk_window_set_wmclass")
	b(&g.windowSetIconList, "gtk_window_set_icon_list")
	b(&g.windowIconify, "gtk_window_iconify")
	b(&g.windowDeiconify, "gtk_window_deiconify")
	b(&g.windowMaximize, "gtk_window_maximize")
	b(&g.windowUnmaximize, "gtk_window_unmaximize")
	b(&g.windowIsMaximized, "gtk_window_is_maximized")
	b(&g.windowPresent, "gtk_window_present")
	b(&g.windowGetPosition, "gtk_window_get_position")
	b(&g.windowGetSize, "gtk_window_get_size")
	b(&g.windowBeginMove, "gtk_window_begin_move_drag")
	b(&g.windowBeginResize, "gtk_window_begin_resize_drag")
	b(&g.windowSetFocus, "gtk_window_set_focus")
	b(&g.windowSetTransient, "gtk_window_set_transient_for")
	b(&g.windowSetAppPaint, "gtk_widget_set_app_paintable")
	b(&g.windowSetVisual, "gtk_widget_set_visual")
	b(&g.windowSetTitlebar, "gtk_window_set_titlebar")
	b(&g.widgetShow, "gtk_widget_show")
	b(&g.widgetShowAll, "gtk_widget_show_all")
	b(&g.widgetHide, "gtk_widget_hide")
	b(&g.widgetDestroy, "gtk_widget_destroy")
	b(&g.widgetRealize, "gtk_widget_realize")
	b(&g.widgetQueueDraw, "gtk_widget_queue_draw")
	b(&g.widgetQueueDrawArea, "gtk_widget_queue_draw_area")
	b(&g.widgetGetWindow, "gtk_widget_get_window")
	b(&g.widgetAddEvents, "gtk_widget_add_events")
	b(&g.widgetSetEvents, "gtk_widget_set_events")
	b(&g.widgetSetSize, "gtk_widget_set_size_request")
	b(&g.widgetSetCanFocus, "gtk_widget_set_can_focus")
	b(&g.widgetGrabFocus, "gtk_widget_grab_focus")
	b(&g.widgetAllocW, "gtk_widget_get_allocated_width")
	b(&g.widgetAllocH, "gtk_widget_get_allocated_height")
	b(&g.widgetGetScale, "gtk_widget_get_scale_factor")
	b(&g.widgetIsVisible, "gtk_widget_is_visible")
	b(&g.widgetTranslate, "gtk_widget_translate_coordinates")
	b(&g.widgetGetToplevel, "gtk_widget_get_toplevel")
	b(&g.widgetGetScreen, "gtk_widget_get_screen")
	b(&g.widgetGetDisplay, "gtk_widget_get_display")
	b(&g.widgetSetHExpand, "gtk_widget_set_hexpand")
	b(&g.widgetSetVExpand, "gtk_widget_set_vexpand")
	b(&g.widgetSetName, "gtk_widget_set_name")
	b(&g.widgetStyleContext, "gtk_widget_get_style_context")
	b(&g.containerAdd, "gtk_container_add")
	b(&g.overlayNew, "gtk_overlay_new")
	b(&g.eventBoxNew, "gtk_event_box_new")
	b(&g.eventBoxVisible, "gtk_event_box_set_visible_window")
	b(&g.overlayAdd, "gtk_overlay_add_overlay")
	b(&g.widgetSetHAlign, "gtk_widget_set_halign")
	b(&g.widgetSetVAlign, "gtk_widget_set_valign")
	b(&g.widgetSetMarginX, "gtk_widget_set_margin_start")
	b(&g.widgetSetMarginY, "gtk_widget_set_margin_top")
	b(&g.errorFree, "g_error_free")
	b(&g.drawingAreaNew, "gtk_drawing_area_new")
	b(&g.fixedNew, "gtk_fixed_new")
	b(&g.cssProviderNew, "gtk_css_provider_new")
	b(&g.cssProviderLoad, "gtk_css_provider_load_from_data")
	b(&g.styleAddProvider, "gtk_style_context_add_provider")
	b(&g.styleAddProviderScr, "gtk_style_context_add_provider_for_screen")

	b(&g.dragDestSet, "gtk_drag_dest_set")
	b(&g.dragDestAddURIs, "gtk_drag_dest_add_uri_targets")
	b(&g.dragGetData, "gtk_drag_get_data")
	b(&g.dragFinish, "gtk_drag_finish")
	b(&g.dragStatus, "gdk_drag_status")
	b(&g.selectionData, "gtk_selection_data_get_data")
	b(&g.selectionLength, "gtk_selection_data_get_length")
	b(&g.atomIntern, "gdk_atom_intern")
	b(&g.imMulticontextNew, "gtk_im_multicontext_new")
	b(&g.imSetClientWindow, "gtk_im_context_set_client_window")
	b(&g.imFocusIn, "gtk_im_context_focus_in")
	b(&g.imFocusOut, "gtk_im_context_focus_out")
	b(&g.imFilterKeypress, "gtk_im_context_filter_keypress")
	b(&g.imGetPreedit, "gtk_im_context_get_preedit_string")
	b(&g.imSetCursorRect, "gtk_im_context_set_cursor_location")
	b(&g.imReset, "gtk_im_context_reset")
	b(&g.imSetUsePreedit, "gtk_im_context_set_use_preedit")
	b(&g.imSetSurrounding, "gtk_im_context_set_surrounding")

	b(&g.gdkDisplayDefault, "gdk_display_get_default")
	b(&g.gdkDisplayPrimary, "gdk_display_get_primary_monitor")
	b(&g.gdkMonitorScale, "gdk_monitor_get_scale_factor")
	b(&g.gdkMonitorWidthMM, "gdk_monitor_get_width_mm")
	b(&g.gdkMonitorHeightMM, "gdk_monitor_get_height_mm")
	b(&g.gdkScreenResolution, "gdk_screen_get_resolution")
	b(&g.gdkEventSourceDevice, "gdk_event_get_source_device")
	b(&g.gdkDeviceSource, "gdk_device_get_source")
	b(&g.gdkScreenDefault, "gdk_screen_get_default")
	b(&g.gdkDisplayName, "gdk_display_get_name")
	b(&g.gdkDisplaySeat, "gdk_display_get_default_seat")
	b(&g.gdkDisplayMonitorAt, "gdk_display_get_monitor_at_window")
	b(&g.gdkMonitorGeometry, "gdk_monitor_get_geometry")
	b(&g.gdkWindowSetCursor, "gdk_window_set_cursor")
	b(&g.gdkCursorFromName, "gdk_cursor_new_from_name")
	b(&g.gdkCursorFromPixbuf, "gdk_cursor_new_from_pixbuf")
	b(&g.gdkWindowOrigin, "gdk_window_get_origin")
	b(&g.gdkWindowRootOrigin, "gdk_window_get_root_origin")
	b(&g.gdkWindowInputShape, "gdk_window_input_shape_combine_region")
	b(&g.gdkWindowNative, "gdk_window_ensure_native")
	b(&g.gdkWindowScale, "gdk_window_get_scale_factor")
	b(&g.gdkWindowSetOpaque, "gdk_window_set_opaque_region")
	b(&g.gdkKeyvalToUnicode, "gdk_keyval_to_unicode")
	b(&g.gdkKeymapDefault, "gdk_keymap_get_default")
	b(&g.gdkKeymapEntries, "gdk_keymap_get_entries_for_keycode")
	b(&g.gdkSetProgramClass, "gdk_set_program_class")
	b(&g.gdkScreenRGBAVisual, "gdk_screen_get_rgba_visual")
	b(&g.gdkScreenComposited, "gdk_screen_is_composited")
	b(&g.gdkSeatPointer, "gdk_seat_get_pointer")
	b(&g.gdkDeviceWindowAt, "gdk_device_get_window_at_position")
	b(&g.gdkDevicePosition, "gdk_device_get_position")
	b(&g.settingsDefault, "gtk_settings_get_default")
	b(&g.objectGetProperty, "g_object_get_property")
	b(&g.valueInit, "g_value_init")
	b(&g.valueInt, "g_value_get_int")
	b(&g.valueString, "g_value_get_string")
	b(&g.valueBool, "g_value_get_boolean")
	b(&g.valueUnset, "g_value_unset")
	b(&g.listAppend, "g_list_append")
	b(&g.listFree, "g_list_free")

	b(&g.cairoSurfaceForData, "cairo_image_surface_create_for_data")
	b(&g.cairoSurfaceDestroy, "cairo_surface_destroy")
	b(&g.cairoSurfaceFlush, "cairo_surface_flush")
	b(&g.cairoSurfaceDirty, "cairo_surface_mark_dirty")
	b(&g.cairoDeviceScale, "cairo_surface_set_device_scale")
	b(&g.cairoSetSource, "cairo_set_source_surface")
	b(&g.cairoSetOperator, "cairo_set_operator")
	b(&g.cairoRectangle, "cairo_rectangle")
	b(&g.cairoClip, "cairo_clip")
	b(&g.cairoFill, "cairo_fill")
	b(&g.cairoPaint, "cairo_paint")
	b(&g.cairoSave, "cairo_save")
	b(&g.cairoRestore, "cairo_restore")
	b(&g.cairoClipExtents, "cairo_clip_extents")
	b(&g.cairoRegionRect, "cairo_region_create_rectangle")
	b(&g.cairoRegionSubtract, "cairo_region_subtract_rectangle")
	b(&g.cairoRegionUnion, "cairo_region_union_rectangle")
	b(&g.cairoRegionDestroy, "cairo_region_destroy")
}

// symbol returns the address of a function in the loaded libraries, 0 if none has it.
func (g *gtkLibrary) symbol(name string) uintptr {
	for _, library := range g.libraries {
		if address, err := purego.Dlsym(library, name); err == nil {
			return address
		}
	}
	return 0
}

// connect attaches a handler to a signal of a GObject. data is passed back to the handler as its
// last argument.
func (g *gtkLibrary) connect(instance uintptr, signal string, handler uintptr, data uintptr) {
	name := cString(signal)
	g.signalConnect(instance, &name[0], handler, data, 0, 0)
}

// goString reads a NUL-terminated C string.
func goString(pointer uintptr) string {
	if pointer == 0 {
		return ""
	}
	var builder strings.Builder
	for offset := uintptr(0); ; offset++ {
		b := *(*byte)(unsafePointer(pointer + offset))
		if b == 0 {
			return builder.String()
		}
		builder.WriteByte(b)
	}
}
