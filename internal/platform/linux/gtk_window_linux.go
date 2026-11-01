//go:build linux && vivagtk

package linux

import (
	"math"
	"os"
	"sync/atomic"
	"time"

	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

// gtkWindow is one top-level window of the GTK shell. GTK owns the window, the pointer and the
// keyboard; Viva's content is drawn by the CPU rasterizer (softScene) into a drawing area on X11 or
// into a Wayland subsurface of the window's surface (gtk_wayland_linux.go), the second of which can
// be at the fractional scale the compositor asks for.
type gtkWindow struct {
	app    *gtkApplication
	g      *gtkLibrary
	id     uintptr
	config platform.Config
	text   *linuxTextInput
	scene  *softScene
	// services is the window's own Pango context, for the label pictures.
	services *textServices

	toplevel, area uintptr
	// overlay holds the area and the web pages above it; it exists when the application may open pages.
	overlay uintptr
	ime     *gtkIME
	wl      *waylandSurface

	// unit is the device pixels in one unit of the widget's size: GTK's own scale on X11, where
	// the drawing area is the canvas; 1 on Wayland, where the area holds no pixels. scale is Viva's
	// device pixels per logical pixel and ratio the widget units per logical pixel (scale/unit on
	// X11, 1 on Wayland).
	unit  int32
	scale float32
	ratio float32
	// width and height are the content in logical pixels.
	width, height float32
	allocated0    bool
	cairoSurface  uintptr

	frame     atomic.Pointer[ui.Rect]
	maximized bool
	minimized bool
	focused   bool

	regionsDirty bool
	// dragging and dropping follow a drag of files over the window.
	dragging, dropping bool
	// iconData keeps the pixels the window's icon pixbufs point at; originPending says the window
	// was asked to open at a place and has not been checked there yet.
	iconData      [][]byte
	originPending bool
	// cursors are the GDK cursors made so far, by shape; cursorShape is the one in use plus one.
	cursors     [len(cursorNames)]uintptr
	cursorShape uint8

	redrawRequested atomic.Bool
	destroying      bool
	closed          bool

	pointer gtkPointer
	keys    gtkKeys

	// The web pages (gtk_web_linux.go): their places as the application gave them, brought to
	// whole widget units, and who has the keyboard.
	views      []*gtkWebView
	viewRects  []platform.NativeView
	snapped    []platform.NativeView
	places     []viewPlace
	nativeKeys bool
	webFocus   uintptr
	// pieces are the windows that carry what Viva draws over a page when GTK draws (X11).
	pieces      []*gtkPiece
	inputPieces []*gtkPiece
	pieceBoxes  []box
	coverBoxes  []box
}

func (a *gtkApplication) OpenWindow(config platform.Config) (platform.Window, error) {
	if err := validateConfig(config); err != nil {
		return nil, err
	}
	if err := a.initialize(config.Class); err != nil {
		return nil, err
	}
	g := a.g
	a.nextID++
	w := &gtkWindow{app: a, g: g, id: a.nextID, config: config}
	text, err := newLinuxTextInput(&w.config)
	if err != nil {
		return nil, err
	}
	w.text = text
	services, err := newTextServices()
	if err != nil {
		text.close()
		return nil, err
	}
	w.services = services
	w.pointer.clicks = &input.ClickCounter{Interval: 500 * time.Millisecond, Distance: 4}

	w.toplevel = g.windowNew(0)
	display := g.gdkDisplayDefault()
	w.unit, w.scale, w.ratio = 1, 1, 1
	if !a.wayland {
		w.unit, w.scale = w.x11Scale(display)
		w.ratio = w.scale / float32(w.unit)
	} else {
		factor := float32(1)
		if m := g.gdkDisplayPrimary(display); m != 0 {
			factor = float32(max(g.gdkMonitorScale(m), 1))
		}
		w.scale = factor
		if w.wl = newWaylandSurface(w); w.wl == nil {
			// Without what the subsurface needs the window draws with GTK, at GTK's whole scales.
			w.unit = int32(factor)
		}
	}
	cb := callbacks()
	title := cString(config.Title)
	g.windowSetTitle(w.toplevel, &title[0])
	if class := wmClass(config.Class); class != nil {
		name := class[:len(class)/2-1]
		g.windowSetWMClass(w.toplevel, &name[0], &name[0])
	}
	switch config.WindowMode {
	case platform.WindowFrameless, platform.WindowCustomTitlebar:
		g.windowSetDecorated(w.toplevel, 0)
	case platform.WindowPanel:
		g.windowSetDecorated(w.toplevel, 0)
		g.windowSetTypeHint(w.toplevel, 8) // GDK_WINDOW_TYPE_HINT_POPUP_MENU
		g.windowSkipTaskbar(w.toplevel, 1)
		g.windowKeepAbove(w.toplevel, 1)
	}
	w.setIcon()
	w.placeAtOrigin()
	g.windowSetDefault(w.toplevel, int32(math.Round(float64(float32(config.Width)*w.ratio))), int32(math.Round(float64(float32(config.Height)*w.ratio))))
	w.setMinimumSize()

	w.area = g.drawingAreaNew()
	g.widgetSetCanFocus(w.area, 0)
	g.widgetSetEvents(w.area, gdkExposureMask|gdkPointerMotionMask|gdkButtonPressMask|gdkButtonReleaseMask|gdkEnterNotifyMask|gdkLeaveNotifyMask|gdkScrollMask|gdkSmoothScrollMask)
	g.widgetAddEvents(w.toplevel, gdkKeyPressMask|gdkKeyReleaseMask|gdkFocusChangeMask|gdkStructureMask)
	if config.Web != nil {
		w.overlay = g.overlayNew()
		g.containerAdd(w.overlay, w.area)
		g.containerAdd(w.toplevel, w.overlay)
		config.Web.Open = w.openWebView
		w.config.Web = config.Web
	} else {
		g.containerAdd(w.toplevel, w.area)
	}

	data := w.id
	g.connect(w.area, "draw", cb.draw, data)
	g.connect(w.area, "size-allocate", cb.allocate, data)
	for _, signal := range []string{"button-press-event", "button-release-event", "motion-notify-event", "enter-notify-event", "leave-notify-event", "scroll-event"} {
		g.connect(w.area, signal, cb.event, data)
	}
	for _, signal := range []string{"key-press-event", "key-release-event", "focus-in-event", "focus-out-event", "window-state-event", "configure-event"} {
		g.connect(w.toplevel, signal, cb.event, data)
	}
	if config.OnFileDrop != nil {
		const actionCopy = 1 << 1
		g.dragDestSet(w.area, 0, 0, 0, actionCopy)
		g.dragDestAddURIs(w.area)
		g.connect(w.area, "drag-motion", cb.dragMotion, data)
		g.connect(w.area, "drag-leave", cb.dragLeave, data)
		g.connect(w.area, "drag-drop", cb.dragDrop, data)
		g.connect(w.area, "drag-data-received", cb.dragData, data)
	}
	g.connect(w.toplevel, "delete-event", cb.deleteEvent, data)
	g.connect(w.toplevel, "destroy", cb.destroy, data)
	g.connect(w.toplevel, "map", cb.mapped, data)
	g.connect(w.toplevel, "unmap", cb.unmapped, data)

	// The scene starts at the size asked for; the first allocation corrects it.
	w.width, w.height = float32(config.Width), float32(config.Height)
	rectangles, labels := text.initial()
	w.scene, err = newSoftScene(services, max(int(math.Round(float64(w.width*w.scale))), 1), max(int(math.Round(float64(w.height*w.scale))), 1), w.scale, config.ClearColor, rectangles, labels)
	if err != nil {
		w.cleanup()
		return nil, err
	}
	if config.Actions != nil {
		w.installActions(config.Actions)
	}
	if config.DynamicTextInput || config.TextInput != nil {
		w.ime = newGTKIME(w)
	}
	text.bindSettings(func() xsettings { return a.settings })
	text.bindCaretWaker(func(generation uint32) {
		a.enqueue(func() { w.blinkCaret(generation) })
	})
	if config.Waker != nil && config.OnRedraw != nil {
		config.Waker.Bind(w.wake)
	}
	a.windows[w.id] = w
	a.atspiOpened(w)
	w.viewRects = append(w.viewRects, config.NativeViews...)
	if config.Web != nil && config.Web.Ready != nil {
		a.enqueue(func() {
			if !w.closed {
				config.Web.Ready()
			}
		})
	}
	g.widgetShowAll(w.toplevel)
	return w, nil
}

// x11Scale decides the unit and the scale on X11 from the desktop's settings, in the order
// display_scale.go gives.
func (w *gtkWindow) x11Scale(display uintptr) (unit int32, scale float32) {
	g := w.g
	unit = 1
	var monitor monitorGeometry
	inputs := scaleInputs{
		VivaScale:            os.Getenv("VIVA_SCALE"),
		QtScaleFactor:        os.Getenv("QT_SCALE_FACTOR"),
		QtScreenScaleFactors: os.Getenv("QT_SCREEN_SCALE_FACTORS"),
		GdkScale:             os.Getenv("GDK_SCALE"),
	}
	if m := g.gdkDisplayPrimary(display); m != 0 {
		unit = max(g.gdkMonitorScale(m), 1)
		var rect [4]int32
		g.gdkMonitorGeometry(m, &rect)
		monitor = monitorGeometry{X: int(rect[0]), Y: int(rect[1]), Width: int(rect[2]) * int(unit), Height: int(rect[3]) * int(unit), WidthMM: int(g.gdkMonitorWidthMM(m)), HeightMM: int(g.gdkMonitorHeightMM(m)), valid: true}
	}
	inputs.Monitor = monitor
	inputs.XSettings = w.app.settings
	inputs.XSettings.WindowScale, inputs.XSettings.hasWindowScale = unit, true
	scale, _ = chooseScale(inputs)
	if unit > 1 {
		// GTK already draws the window at its own factor: the window is that many device pixels
		// to a widget unit, whatever else was asked for.
		scale = float32(unit)
	}
	return unit, scale
}

// Close, Hide and Show may be called from any goroutine.
func (w *gtkWindow) Close() {
	w.app.enqueue(func() { w.destroy() })
}

func (w *gtkWindow) Hide() {
	w.app.enqueue(func() {
		if !w.closed {
			w.g.widgetHide(w.toplevel)
		}
	})
}

func (w *gtkWindow) Show() {
	w.app.enqueue(func() {
		if !w.closed {
			switch {
			case w.g.widgetIsVisible(w.toplevel) == 0:
				w.g.widgetShowAll(w.toplevel)
			case w.minimized:
				w.g.windowDeiconify(w.toplevel)
			case !w.app.wayland:
				w.g.windowPresent(w.toplevel)
			}
			// A Wayland client cannot raise its window: gtk_window_present on a window that is
			// mapped never returns in GDK 3.24 (it spins in the event source), and the compositor
			// decides what is in front.
		}
	})
}

func (w *gtkWindow) Frame() (ui.Rect, bool) {
	if frame := w.frame.Load(); frame != nil {
		return *frame, true
	}
	return ui.Rect{}, false
}

// wake is the Waker: one OnRedraw on the loop's thread, however many wakes came before it ran.
func (w *gtkWindow) wake() {
	if w.redrawRequested.Swap(true) {
		return
	}
	w.g.idleAdd(callbacks().redraw, w.id)
}

func (w *gtkWindow) redraw() {
	if w.closed || w.config.OnRedraw == nil {
		return
	}
	frame, err := w.config.OnRedraw()
	w.present(frame, err)
}

func (w *gtkWindow) blinkCaret(generation uint32) {
	if w.closed {
		return
	}
	// The caret's frame is already the text layer's own scene: it does not go through prepare.
	if frame, changed := w.text.blinkCaret(generation); changed {
		w.apply(frame)
	}
}

// destroy closes the window and runs what its closing means.
func (w *gtkWindow) destroy() {
	if w.destroying || w.closed {
		return
	}
	w.destroying = true
	w.g.widgetDestroy(w.toplevel)
}

// destroyed is GTK saying the window is gone.
func (w *gtkWindow) destroyed() {
	w.cleanup()
	if w.config.OnClosed != nil {
		w.config.OnClosed()
	}
	if len(w.app.windows) == 0 {
		w.app.stop()
	}
}

// cleanup releases everything the window holds. It runs once.
func (w *gtkWindow) cleanup() {
	if w.closed {
		return
	}
	w.closed = true
	for _, v := range w.views {
		v.release()
	}
	w.views = nil
	w.app.atspiClosed(w)
	delete(w.app.windows, w.id)
	if w.config.Waker != nil && w.config.OnRedraw != nil {
		w.config.Waker.Unbind()
	}
	if w.ime != nil {
		w.ime.close()
		w.ime = nil
	}
	if w.wl != nil {
		w.wl.close()
		w.wl = nil
	}
	w.releaseSurface()
	if w.scene != nil {
		w.scene.close()
		w.scene = nil
	}
	if w.text != nil {
		w.text.close()
	}
	if w.services != nil {
		w.services.close()
		w.services = nil
	}
}

// closeRequested handles the window manager's close button. It reports whether the application
// took the decision.
func (w *gtkWindow) closeRequested() bool {
	if w.config.OnCloseRequest == nil || w.config.WindowMode == platform.WindowPanel {
		return false
	}
	w.config.OnCloseRequest(func() { w.Close() })
	return true
}

// mapChanged is the window being shown or hidden.
func (w *gtkWindow) mapChanged(on bool) {
	if on && w.originPending {
		// The window manager decorates the window after it is mapped; its frame is known a moment
		// later.
		time.AfterFunc(250*time.Millisecond, func() { w.app.enqueue(w.correctOrigin) })
	}
	if w.wl != nil {
		// Whether the compositor has a surface for the window decides whether there is anything to
		// attach to.
		w.wl.mapped(w, on)
	}
}

// installActions fills in what the controls Viva draws in a custom title bar call.
func (w *gtkWindow) installActions(actions *platform.WindowActions) {
	g := w.g
	actions.Minimize = func() { g.windowIconify(w.toplevel) }
	actions.ToggleMaximize = func() {
		if g.windowIsMaximized(w.toplevel) != 0 {
			g.windowUnmaximize(w.toplevel)
		} else {
			g.windowMaximize(w.toplevel)
		}
	}
	actions.RequestClose = func() {
		if !w.closeRequested() {
			w.Close()
		}
	}
	actions.Maximized = func() bool { return w.maximized }
}

// setMinimumSize applies MinWidth and MinHeight, the content area's least size in logical pixels.
func (w *gtkWindow) setMinimumSize() {
	if w.config.MinWidth <= 0 && w.config.MinHeight <= 0 {
		return
	}
	geometry := gdkGeometry{
		MinWidth:  int32(math.Ceil(float64(float32(w.config.MinWidth) * w.ratio))),
		MinHeight: int32(math.Ceil(float64(float32(w.config.MinHeight) * w.ratio))),
	}
	w.g.windowSetGeometry(w.toplevel, 0, &geometry, gdkHintMinSize)
}

// allocated is the drawing area getting a size: the window was shown, resized or rescaled.
func (w *gtkWindow) allocated(widget uintptr, x, y, width, height int32) {
	if w.closed || width <= 0 || height <= 0 {
		return
	}
	logicalWidth, logicalHeight := float32(width)/w.ratio, float32(height)/w.ratio
	if w.wl != nil {
		w.wl.positioned(x, y)
	}
	if w.allocated0 && logicalWidth == w.width && logicalHeight == w.height {
		return
	}
	w.allocated0 = true
	w.width, w.height = logicalWidth, logicalHeight
	w.resize()
}

// resize brings the canvas to the logical size and scale, tells the application, and paints.
func (w *gtkWindow) resize() {
	// On X11 the canvas is the drawing area, whole device pixels; on Wayland it is the logical size
	// at the compositor's scale, which the viewport maps onto it.
	deviceWidth := max(int(math.Round(float64(w.width*w.scale))), 1)
	deviceHeight := max(int(math.Round(float64(w.height*w.scale))), 1)
	if w.wl == nil {
		deviceWidth = max(int(math.Round(float64(w.width*w.ratio)))*int(w.unit), 1)
		deviceHeight = max(int(math.Round(float64(w.height*w.ratio)))*int(w.unit), 1)
	}
	w.releaseSurface()
	if err := w.scene.setScale(w.scale); err != nil {
		w.app.fail(err)
		return
	}
	if err := w.scene.resize(deviceWidth, deviceHeight); err != nil {
		w.app.fail(err)
		return
	}
	if len(w.viewRects) > 0 {
		w.layoutViews()
	}
	if w.config.OnResize != nil {
		frame, err := w.config.OnResize(platform.Size{Width: w.width, Height: w.height, Scale: w.scale})
		w.present(frame, err)
	}
	if w.wl != nil {
		w.wl.resized(w)
	}
	w.queuePaint()
}

// setScale moves the window to another scale (Wayland's preferred scale, or a desktop setting).
func (w *gtkWindow) setScale(scale float32) {
	if w.closed || scale == w.scale || !(scale >= 1) {
		return
	}
	w.scale = scale
	if w.allocated0 {
		w.resize()
	}
}

// present takes a frame the application returned: into the scene, and on to the screen.
func (w *gtkWindow) present(frame platform.Frame, err error) {
	if w.closed {
		return
	}
	if err != nil {
		w.app.fail(err)
		return
	}
	if w.ime != nil {
		defer w.ime.sync()
	}
	if frame.Accessibility != nil {
		w.app.atspiFrame(w, frame.Accessibility)
	}
	if frame.NativeViews != nil {
		w.setNativeViews(frame.NativeViews)
	}
	merged, dirty, err := w.text.prepare(frame)
	if err != nil {
		w.app.fail(err)
		return
	}
	if dirty {
		w.apply(merged)
	}
}

// apply puts a frame the text layer has merged into the scene, and on to the screen.
func (w *gtkWindow) apply(frame platform.Frame) {
	changed, err := w.scene.apply(frame)
	if err != nil {
		w.app.fail(err)
		return
	}
	if len(w.scene.views) > 0 {
		// What Viva draws over a page decides where the pointer is the page's.
		w.updateOverlayRegions()
	}
	if changed {
		w.queuePaint()
	}
}

// queuePaint asks for the scene's pending damage to reach the screen.
func (w *gtkWindow) queuePaint() {
	if w.closed || !w.scene.pending() {
		return
	}
	if w.wl != nil {
		w.wl.present(w)
		return
	}
	d := w.scene.damage
	unit := int(w.unit)
	left, top := d.Left/unit, d.Top/unit
	w.g.widgetQueueDrawArea(w.area, int32(left), int32(top), int32((d.Right+unit-1)/unit-left), int32((d.Bottom+unit-1)/unit-top))
	if len(w.pieces) > 0 {
		w.invalidatePieces(int32(left), int32(top), int32((d.Right+unit-1)/unit), int32((d.Bottom+unit-1)/unit))
	}
}

// draw is the drawing area's turn on X11: paint what changed and copy the canvas to the exposed
// part of the window.
func (w *gtkWindow) draw(widget, cr uintptr) {
	if w.wl != nil || w.closed {
		return
	}
	w.scene.paint()
	surface := w.canvasSurface()
	if surface == 0 {
		return
	}
	g := w.g
	g.cairoSurfaceFlush(surface)
	g.cairoSurfaceDirty(surface)
	// A piece shows its part of the canvas.
	originX, originY := w.eventOffset(widget)
	g.cairoSetSource(cr, surface, -originX, -originY)
	if widget != w.area {
		// A piece is a window of the display server's own, whose surface SRC does not treat as
		// the window's picture.
		g.cairoSetOperator(cr, cairoOperatorOver)
	} else {
		g.cairoSetOperator(cr, cairoOperatorSrc)
	}
	g.cairoPaint(cr)
}

// canvasSurface wraps the canvas for cairo. The surface shares the canvas's pixels, and goes when
// the canvas is replaced.
func (w *gtkWindow) canvasSurface() uintptr {
	if w.cairoSurface != 0 {
		return w.cairoSurface
	}
	canvas := w.scene.canvas
	surface := w.g.cairoSurfaceForData(&canvas.Pix[0], cairoFormatARGB32, int32(canvas.Width), int32(canvas.Height), int32(canvas.Stride))
	if surface == 0 {
		return 0
	}
	w.g.cairoDeviceScale(surface, float64(w.unit), float64(w.unit))
	w.cairoSurface = surface
	return surface
}

func (w *gtkWindow) releaseSurface() {
	if w.cairoSurface != 0 {
		w.g.cairoSurfaceDestroy(w.cairoSurface)
		w.cairoSurface = 0
	}
}

// reportFrame tells the application where the content is on the screen after the window moved or
// changed size, when the display server says.
func (w *gtkWindow) reportFrame() {
	if w.app.wayland || w.closed {
		return
	}

	window := w.g.widgetGetWindow(w.area)
	if window == 0 {
		return
	}
	var x, y int32
	w.g.gdkWindowOrigin(window, &x, &y)
	frame := ui.Rect{X: float32(x) / w.ratio, Y: float32(y) / w.ratio, Width: w.width, Height: w.height}
	if previous := w.frame.Load(); previous != nil && *previous == frame {
		return
	}
	w.frame.Store(&frame)
	if w.config.OnFrameChange != nil && !w.maximized {
		w.config.OnFrameChange(frame)
	}
}

// The window is a target of the accessibility bridge.

func (w *gtkWindow) atspiConfig() *platform.Config { return &w.config }

// atspiGeometry is the content's place on the screen in device pixels. Wayland does not tell a
// window where it is, so a reader gets coordinates relative to the window.
func (w *gtkWindow) atspiGeometry() (int, int, float32) {
	if w.app.wayland {
		return 0, 0, w.scale
	}
	if frame := w.frame.Load(); frame != nil {
		return int(frame.X * w.scale), int(frame.Y * w.scale), w.scale
	}
	return 0, 0, w.scale
}

func (w *gtkWindow) atspiPresent(frame platform.Frame) error {
	w.present(frame, nil)
	return nil
}

// refreshText draws the window's text again, after the desktop's fonts changed.
func (w *gtkWindow) refreshText() {
	if w.closed || w.scene == nil {
		return
	}
	if err := w.scene.refreshText(); err != nil {
		w.app.fail(err)
		return
	}
	w.queuePaint()
}

// rescale follows the desktop's scale changing under a window on X11: the logical size stays, the
// window takes the device size that gives at the new scale.
func (w *gtkWindow) rescale() {
	if w.closed || w.app.wayland {
		return
	}
	unit, scale := w.x11Scale(w.g.gdkDisplayDefault())
	if unit == w.unit && scale == w.scale {
		return
	}
	w.unit, w.scale, w.ratio = unit, scale, scale/float32(unit)
	w.setMinimumSize()
	w.allocated0 = false
	w.g.windowResize(w.toplevel, int32(math.Round(float64(w.width*w.ratio))), int32(math.Round(float64(w.height*w.ratio))))
}
