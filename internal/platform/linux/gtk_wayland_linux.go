//go:build linux && vivagtk

package linux

import (
	"fmt"
	"math"
	"slices"
	"syscall"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
)

// On Wayland the window's content is a wl_subsurface of the GTK window's wl_surface, drawn at the
// scale the compositor asks for (wp_fractional_scale_v1) and mapped to the window's logical size
// with a wp_viewport, which GTK 3, with its integer scales, cannot do. GTK keeps the window, the
// keyboard and the input method; the subsurface takes the pointer, through a wl_pointer of its own
// on GDK's seat, except over the parts of the window the window manager's gestures need (the
// title bar and the edges of a window whose frame Viva draws), which GTK keeps so that the press
// that starts a move or a resize is one GDK saw and has a serial for.

// wlSession is what is shared by the windows of the process: the globals of the compositor and the
// pointer.
type wlSession struct {
	c       *wlClient
	g       *gtkLibrary
	display uintptr

	compositor, subcompositor, shm, viewporter, fractionalManager, cursorManager uintptr
	seat, pointer, cursorDevice, registry                                        uintptr
	dataManager, dataDevice                                                      uintptr
	drag                                                                         wlDrag
	cursorTheme                                                                  wlCursorTheme

	surfaces    map[uintptr]*waylandSurface // by the subsurface's wl_surface
	focus       *waylandSurface
	enterSerial uint32
	pressed     uint32 // pointer buttons down, by GDK button number
	axis        wlAxis

	surfaceOf func(window uintptr) uintptr
	modifiers func(keymap uintptr) uint32
}

// wlAxis collects the axis events of one wl_pointer.frame.
type wlAxis struct {
	source               uint32
	hasSource            bool
	dx, dy               float64
	stepX, stepY         float64
	seenX, seenY, stepsX bool
	stepsY               bool
	time                 uint32
}

var (
	wlCurrent       *wlSession
	wlSessionFailed error
	wlLists         struct{ registry, fractional, buffer, pointer, dataDevice, dataOffer uintptr }
)

// newWaylandSession connects to the compositor's globals through GDK's connection. It fails, with a
// reason, when the compositor lacks what the subsurface needs; the window then draws with GTK as
// it does on X11.
func newWaylandSession(g *gtkLibrary) (*wlSession, error) {
	if wlCurrent != nil || wlSessionFailed != nil {
		return wlCurrent, wlSessionFailed
	}
	session, err := openWaylandSession(g)
	if err != nil {
		wlCurrent, wlSessionFailed = nil, err
		return nil, err
	}
	return session, nil
}

func openWaylandSession(g *gtkLibrary) (*wlSession, error) {
	c, err := loadWayland()
	if err != nil {
		return nil, err
	}
	s := &wlSession{c: c, g: g, surfaces: map[uintptr]*waylandSurface{}}
	var displayOf, seatOf func(uintptr) uintptr
	for _, binding := range []struct {
		function any
		name     string
	}{
		{&displayOf, "gdk_wayland_display_get_wl_display"},
		{&s.surfaceOf, "gdk_wayland_window_get_wl_surface"},
		{&seatOf, "gdk_wayland_seat_get_wl_seat"},
		{&s.modifiers, "gdk_keymap_get_modifier_state"},
	} {
		found := false
		for _, library := range g.libraries {
			if _, err := purego.Dlsym(library, binding.name); err == nil {
				purego.RegisterLibFunc(binding.function, library, binding.name)
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("linux: GDK lacks %s", binding.name)
		}
	}
	display := g.gdkDisplayDefault()
	s.display = displayOf(display)
	s.seat = seatOf(g.gdkDisplaySeat(display))
	if s.display == 0 || s.seat == 0 {
		return nil, fmt.Errorf("linux: GDK has no Wayland display or seat")
	}
	if wlLists.registry == 0 {
		wlLists.registry = wlListener(
			func(data, registry, name, iface, version uintptr) uintptr {
				wlCurrent.global(name, goString(iface), uint32(version))
				return 0
			},
			func(data, registry, name uintptr) uintptr { return 0 },
		)
		wlLists.fractional = wlListener(func(data, proxy, scale uintptr) uintptr {
			if surface, ok := wlObjects[data].(*waylandSurface); ok {
				surface.preferredScale(uint32(scale))
			}
			return 0
		})
		wlLists.buffer = wlListener(func(data, buffer uintptr) uintptr {
			if b, ok := wlObjects[data].(*wlBuffer); ok {
				b.released()
			}
			return 0
		})
		wlLists.pointer = wlListener(
			func(data, pointer, serial, surface, x, y uintptr) uintptr {
				wlCurrent.pointerEnter(uint32(serial), surface, wlFixed(x), wlFixed(y))
				return 0
			},
			func(data, pointer, serial, surface uintptr) uintptr { wlCurrent.pointerLeave(surface); return 0 },
			func(data, pointer, t, x, y uintptr) uintptr {
				wlCurrent.pointerMotion(uint32(t), wlFixed(x), wlFixed(y))
				return 0
			},
			func(data, pointer, serial, t, button, state uintptr) uintptr {
				wlCurrent.pointerButton(uint32(serial), uint32(t), uint32(button), uint32(state))
				return 0
			},
			func(data, pointer, t, axis, value uintptr) uintptr {
				wlCurrent.pointerAxis(uint32(t), uint32(axis), wlFixed(value))
				return 0
			},
			func(data, pointer uintptr) uintptr { wlCurrent.pointerFrame(); return 0 },
			func(data, pointer, source uintptr) uintptr {
				wlCurrent.axis.source, wlCurrent.axis.hasSource = uint32(source), true
				return 0
			},
			func(data, pointer, t, axis uintptr) uintptr { return 0 },
			func(data, pointer, axis, discrete uintptr) uintptr {
				wlCurrent.pointerDiscrete(uint32(axis), int32(discrete))
				return 0
			},
			func(data, pointer, axis, value120 uintptr) uintptr { return 0 },
			func(data, pointer, axis, direction uintptr) uintptr { return 0 },
		)
	}
	wlCurrent = s // the callbacks find the session here
	registry := c.create(s.display, 1, c.core["wl_registry"], 1, 0)
	c.addListener(registry, wlLists.registry, 0)
	s.registry = registry
	c.roundtrip(s.display)
	if s.compositor == 0 || s.subcompositor == 0 || s.shm == 0 || s.viewporter == 0 {
		return nil, fmt.Errorf("linux: the compositor lacks wl_compositor, wl_subcompositor, wl_shm or wp_viewporter")
	}
	s.pointer = c.create(s.seat, 0, c.core["wl_pointer"], c.version(s.seat), 0)
	c.addListener(s.pointer, wlLists.pointer, 0)
	wlDataSetup(s)
	if s.cursorManager != 0 {
		s.cursorDevice = c.create(s.cursorManager, 1, c.cursorDevice, 1, 0, wlArg(s.pointer))
	}
	return s, nil
}

// global binds the globals the shell uses.
func (s *wlSession) global(name uintptr, iface string, version uint32) {
	c := s.c
	bind := func(interfaceName string, proxyInterface uintptr, wanted uint32) uintptr {
		version = min(version, wanted)
		var arguments [4]wlArg
		arguments[0], arguments[1], arguments[2] = wlArg(name), wlArg(wlCStr(interfaceName)), wlArg(version)
		return c.marshal(s.registry, 0, proxyInterface, version, 0, &arguments[0])
	}
	switch iface {
	case "wl_compositor":
		s.compositor = bind(iface, c.core["wl_compositor"], 4)
	case "wl_subcompositor":
		s.subcompositor = bind(iface, c.core["wl_subcompositor"], 1)
	case "wl_shm":
		s.shm = bind(iface, c.core["wl_shm"], 1)
	case "wp_viewporter":
		s.viewporter = bind(iface, c.viewporter, 1)
	case "wp_fractional_scale_manager_v1":
		s.fractionalManager = bind(iface, c.fractionalManager, 1)
	case "wl_data_device_manager":
		s.dataManager = bind(iface, c.core["wl_data_device_manager"], 3)
	case "wp_cursor_shape_manager_v1":
		s.cursorManager = bind(iface, c.cursorManager, 1)
	}
}

// Pointer button numbers of evdev, and the GDK numbers they stand for.
func gdkButtonOfEvdev(button uint32) uint32 {
	switch button {
	case 0x110:
		return 1
	case 0x112:
		return 2
	case 0x111:
		return 3
	case 0x113, 0x116:
		return 8
	case 0x114, 0x115:
		return 9
	}
	return 0
}

func (s *wlSession) modifierState() input.Modifiers {
	return gdkModifiers(s.modifiers(s.g.gdkKeymapDefault()))
}

func (s *wlSession) pointerEnter(serial uint32, surface uintptr, x, y float64) {
	target := s.surfaces[surface]
	if target == nil {
		return
	}
	s.focus, s.enterSerial = target, serial
	target.x, target.y = x, y
	target.applyCursor()
	target.w.pointerMoveAt(float32(x), float32(y), s.modifierState())
}

func (s *wlSession) pointerLeave(surface uintptr) {
	if s.focus != nil && s.focus.child == surface {
		focus := s.focus
		s.focus = nil
		focus.w.pointerExit()
	}
}

func (s *wlSession) pointerMotion(t uint32, x, y float64) {
	if s.focus != nil {
		s.focus.x, s.focus.y = x, y
		s.focus.w.pointerMoveAt(float32(x), float32(y), s.modifierState())
	}
}

func (s *wlSession) pointerButton(serial, t, button, state uint32) {
	if s.focus == nil {
		return
	}
	gdkButton := gdkButtonOfEvdev(button)
	if gdkButton == 0 {
		return
	}
	down := state == 1
	if down {
		s.pressed |= 1 << gdkButton
	} else {
		s.pressed &^= 1 << gdkButton
	}
	focus := s.focus
	focus.w.pointerButtonAt(float32(focus.x), float32(focus.y), gdkButton, down, t, s.modifierState())
}

func (s *wlSession) pointerAxis(t uint32, axis uint32, value float64) {
	if axis == 0 {
		s.axis.dy, s.axis.seenY = s.axis.dy+value, true
	} else {
		s.axis.dx, s.axis.seenX = s.axis.dx+value, true
	}
	s.axis.time = t
	if s.c.version(s.pointer) < 5 {
		s.pointerFrame()
	}
}

func (s *wlSession) pointerDiscrete(axis uint32, steps int32) {
	if axis == 0 {
		s.axis.stepY, s.axis.stepsY = s.axis.stepY+float64(steps), true
	} else {
		s.axis.stepX, s.axis.stepsX = s.axis.stepX+float64(steps), true
	}
}

// pointerFrame ends a group of axis events: one scroll.
func (s *wlSession) pointerFrame() {
	axis := s.axis
	s.axis = wlAxis{}
	if s.focus == nil || !(axis.seenX || axis.seenY || axis.stepsX || axis.stepsY) {
		return
	}
	scroll := platform.ScrollEvent{Modifiers: s.modifierState(), X: float32(s.focus.x), Y: float32(s.focus.y)}
	const wheel = 0 // wl_pointer.axis_source: wheel
	if axis.hasSource && axis.source == wheel || axis.stepsX || axis.stepsY {
		// A wheel scrolls in notches; the value of an axis event is a distance the compositor
		// chose for them.
		scroll.DeltaX, scroll.DeltaY = float32(axis.stepX*gtkWheelStep), float32(axis.stepY*gtkWheelStep)
		if !axis.stepsX && axis.dx != 0 {
			scroll.DeltaX = float32(axis.dx / 10 * gtkWheelStep)
		}
		if !axis.stepsY && axis.dy != 0 {
			scroll.DeltaY = float32(axis.dy / 10 * gtkWheelStep)
		}
	} else {
		scroll.DeltaX, scroll.DeltaY, scroll.Precise = float32(axis.dx), float32(axis.dy), true
	}
	if scroll.DeltaX == 0 && scroll.DeltaY == 0 {
		return
	}
	s.focus.w.scrollAt(scroll)
}

// waylandSurface is one window's subsurface.
type waylandSurface struct {
	s  *wlSession
	w  *gtkWindow
	id uintptr

	parent, child, sub, viewport, fractional uintptr
	scale120                                 uint32
	offsetX, offsetY                         int32
	viewportWidth, viewportHeight            int32
	buffers                                  []*wlBuffer
	deferred                                 bool
	x, y                                     float64 // the pointer in the subsurface
	cursor                                   uint8
	haveCursor                               bool
	regionWidth, regionHeight                int32
	// holes are the pages' places and the parts of them Viva covers, as the input region last
	// took them (holeKey), for noticing a change; -1 marks a part Viva covers.
	holes, holeKey []int32
}

// newWaylandSurface prepares the Wayland presenter for a window; nil means the window draws with
// GTK as on X11.
func newWaylandSurface(w *gtkWindow) *waylandSurface {
	session, err := newWaylandSession(w.g)
	if err != nil {
		return nil
	}
	s := &waylandSurface{s: session, w: w, scale120: 120}
	s.id = wlRegister(s)
	return s
}

// mapped follows the GTK window being shown and hidden. GDK destroys the window's wl_surface when
// it is hidden and makes another when it is shown, so what hangs on the surface is made and
// destroyed with it.
func (s *waylandSurface) mapped(w *gtkWindow, on bool) {
	if !on {
		s.detach()
		return
	}
	if s.child == 0 {
		s.attach()
	}
}

func (s *waylandSurface) attach() {
	session, c, w := s.s, s.s.c, s.w
	gdkWindow := w.g.widgetGetWindow(w.toplevel)
	if gdkWindow == 0 {
		return
	}
	s.parent = session.surfaceOf(gdkWindow)
	if s.parent == 0 {
		return
	}
	s.child = c.create(session.compositor, 0, c.core["wl_surface"], c.version(session.compositor), 0)
	s.sub = c.create(session.subcompositor, 1, c.core["wl_subsurface"], 1, 0, wlArg(s.child), wlArg(s.parent))
	c.request(s.sub, 5) // set_desync: the window's content is committed on its own
	s.viewport = c.create(session.viewporter, 1, c.viewport, 1, 0, wlArg(s.child))
	session.surfaces[s.child] = s
	if session.fractionalManager != 0 {
		s.fractional = c.create(session.fractionalManager, 1, c.fractional, 1, 0, wlArg(s.child))
		c.addListener(s.fractional, wlLists.fractional, s.id)
	}
	s.viewportWidth, s.viewportHeight, s.regionWidth, s.regionHeight = 0, 0, 0, 0
	s.positioned(s.offsetX, s.offsetY)
	s.w.scene.damageAll()
	s.resized(w)
	// A new subsurface takes effect with the next commit of its parent: make GTK draw once more.
	w.g.widgetQueueDraw(w.toplevel)
}

func (s *waylandSurface) detach() {
	if s.child == 0 {
		return
	}
	session, c := s.s, s.s.c
	for _, b := range s.buffers {
		b.destroy()
	}
	s.buffers = nil
	delete(session.surfaces, s.child)
	if session.focus == s {
		session.focus = nil
	}
	if s.fractional != 0 {
		c.destroy(s.fractional, 0)
		s.fractional = 0
	}
	c.destroy(s.viewport, 0)
	c.destroy(s.sub, 0)
	c.destroy(s.child, 0)
	s.viewport, s.sub, s.child, s.parent = 0, 0, 0, 0
	c.flush(session.display)
}

func (s *waylandSurface) close() {
	s.detach()
	wlUnregister(s.id)
}

// positioned moves the subsurface to where the window's content is in the surface: below a title
// bar GTK draws, inside the shadow it draws.
func (s *waylandSurface) positioned(x, y int32) {
	s.offsetX, s.offsetY = x, y
	if s.child != 0 {
		s.s.c.request(s.sub, 1, wlArg(uint32(x)), wlArg(uint32(y)))
		s.w.g.widgetQueueDraw(s.w.toplevel)
	}
}

// preferredScale is the compositor saying how many device pixels it wants to a logical one, in
// 120ths.
func (s *waylandSurface) preferredScale(scale120 uint32) {
	if scale120 == 0 || scale120 == s.scale120 {
		return
	}
	s.scale120 = scale120
	s.w.setScale(max(float32(scale120)/120, 1))
}

// resized follows the canvas changing size or scale: the pictures are the wrong size, the viewport
// and the input region are in logical pixels.
func (s *waylandSurface) resized(w *gtkWindow) {
	if s.child == 0 {
		return
	}
	for _, b := range s.buffers {
		b.destroy()
	}
	s.buffers = nil
	width, height := int32(w.width+0.5), int32(w.height+0.5)
	s.regionWidth, s.regionHeight = 0, 0
	s.updateRegions(w, width, height)
	s.present(w)
}

// updateRegions sets where the pointer is the subsurface's: the whole content, except the strips GTK
// keeps. No opaque region is declared although the content is opaque: a compositor that finds
// the window's own surface covered stops sending it frame callbacks, and GDK holds back the
// pointer motion of the strips it handles until the next one.
func (s *waylandSurface) updateRegions(w *gtkWindow, width, height int32) {
	if s.child == 0 || width <= 0 || height <= 0 {
		return
	}
	c := s.s.c
	// A page under the content is a hole in the subsurface's input, except where something Viva
	// draws over the page is: the pointer belongs to the page there and to Viva over it.
	holes := s.holes[:0]
	scale := float64(w.scale)
	for i, view := range w.scene.views {
		b := view.Bounds
		holes = append(holes, int32(math.Floor(float64(b.X))), int32(math.Floor(float64(b.Y))), int32(math.Ceil(float64(b.X+b.Width))), int32(math.Ceil(float64(b.Y+b.Height))))
		for _, o := range w.scene.overlays(i) {
			holes = append(holes, -1, int32(math.Floor(float64(o.Left)/scale)), int32(math.Floor(float64(o.Top)/scale)), int32(math.Ceil(float64(o.Right)/scale)), int32(math.Ceil(float64(o.Bottom)/scale)))
		}
		for _, c := range view.Covers {
			holes = append(holes, -1, int32(math.Floor(float64(c.X))), int32(math.Floor(float64(c.Y))), int32(math.Ceil(float64(c.X+c.Width))), int32(math.Ceil(float64(c.Y+c.Height))))
		}
	}
	s.holes = holes
	if width == s.regionWidth && height == s.regionHeight && !w.regionsDirty && slices.Equal(holes, s.holeKey) {
		return
	}
	s.holeKey = append(s.holeKey[:0], holes...)
	s.regionWidth, s.regionHeight, w.regionsDirty = width, height, false
	if width != s.viewportWidth || height != s.viewportHeight {
		s.viewportWidth, s.viewportHeight = width, height
		c.request(s.viewport, 2, wlArg(uint32(width)), wlArg(uint32(height)))
	}
	region := c.create(s.s.compositor, 1, c.core["wl_region"], c.version(s.s.compositor), 0)
	add := func(x, y, rectWidth, rectHeight int32) {
		c.request(region, 1, wlArg(uint32(x)), wlArg(uint32(y)), wlArg(uint32(rectWidth)), wlArg(uint32(rectHeight)))
	}
	remove := func(x, y, rectWidth, rectHeight int32) {
		c.request(region, 2, wlArg(uint32(x)), wlArg(uint32(y)), wlArg(uint32(rectWidth)), wlArg(uint32(rectHeight)))
	}
	add(0, 0, width, height)
	if w.config.WindowMode == platform.WindowCustomTitlebar {
		strip := int32(w.config.TitlebarStrip() + 0.999)
		remove(0, 0, width, min(strip, height))
		if !w.maximized {
			border := int32(platform.EdgeBorder)
			remove(0, 0, border, height)
			remove(width-border, 0, border, height)
			remove(0, height-border, width, border)
		}
	}
	for i := 0; i < len(holes); {
		if holes[i] == -1 {
			add(holes[i+1], holes[i+2], holes[i+3]-holes[i+1], holes[i+4]-holes[i+2])
			i += 5
			continue
		}
		remove(holes[i], holes[i+1], holes[i+2]-holes[i], holes[i+3]-holes[i+1])
		i += 4
	}
	c.request(s.child, 5, wlArg(region)) // set_input_region
	c.destroy(region, 0)
}

// wlBuffer is one wl_shm buffer the size of the canvas.
type wlBuffer struct {
	s             *waylandSurface
	id            uintptr
	buffer        uintptr
	mem           []byte
	width, height int
	stride        int
	busy, dead    bool
	// pending is what changed in the canvas since the buffer was last filled.
	pending render.Scissor
}

const wlShmArgb8888 = 0

func (s *waylandSurface) newBuffer(width, height int) *wlBuffer {
	c := s.s.c
	stride := width * 4
	size := stride * height
	name := []byte("viva\x00")
	fd, _, errno := syscall.Syscall(sysMemfdCreate, uintptr(unsafe.Pointer(&name[0])), 0, 0)
	if errno != 0 {
		return nil
	}
	defer syscall.Close(int(fd))
	if err := syscall.Ftruncate(int(fd), int64(size)); err != nil {
		return nil
	}
	mem, err := syscall.Mmap(int(fd), 0, size, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		return nil
	}
	pool := c.create(s.s.shm, 0, c.core["wl_shm_pool"], c.version(s.s.shm), 0, wlArg(fd), wlArg(uint32(size)))
	b := &wlBuffer{s: s, mem: mem, width: width, height: height, stride: stride, pending: render.Scissor{Right: width, Bottom: height}}
	b.buffer = c.create(pool, 0, c.core["wl_buffer"], c.version(pool), 0, 0, wlArg(uint32(width)), wlArg(uint32(height)), wlArg(uint32(stride)), wlShmArgb8888)
	c.destroy(pool, 1)
	b.id = wlRegister(b)
	c.addListener(b.buffer, wlLists.buffer, b.id)
	return b
}

func (b *wlBuffer) released() {
	b.busy = false
	if b.dead {
		b.free()
		return
	}
	if b.s.deferred {
		b.s.deferred = false
		b.s.present(b.s.w)
	}
}

// destroy retires the buffer; the compositor may still be reading it, in which case it goes when
// released.
func (b *wlBuffer) destroy() {
	b.dead = true
	if !b.busy {
		b.free()
	}
}

func (b *wlBuffer) free() {
	if b.buffer != 0 {
		b.s.s.c.destroy(b.buffer, 0)
		b.buffer = 0
	}
	if b.mem != nil {
		syscall.Munmap(b.mem)
		b.mem = nil
	}
	wlUnregister(b.id)
}

// maxBuffers bounds the buffers a window may have in flight.
const maxBuffers = 4

// freeBuffer finds a buffer the compositor is done with, or makes one.
func (s *waylandSurface) freeBuffer(width, height int) *wlBuffer {
	for _, b := range s.buffers {
		if !b.busy && !b.dead && b.width == width && b.height == height {
			return b
		}
	}
	if len(s.buffers) >= maxBuffers {
		return nil
	}
	b := s.newBuffer(width, height)
	if b != nil {
		s.buffers = append(s.buffers, b)
	}
	return b
}

// present paints what changed and shows it.
func (s *waylandSurface) present(w *gtkWindow) {
	if s.child == 0 || w.scene == nil {
		return
	}
	canvas := w.scene.canvas
	damage := w.scene.paint()
	c := s.s.c
	hasDamage := damage.Right > damage.Left && damage.Bottom > damage.Top
	for _, b := range s.buffers {
		if hasDamage {
			b.pending = unionScissor(b.pending, damage)
		}
	}
	b := s.freeBuffer(canvas.Width, canvas.Height)
	if b == nil {
		// Every buffer is with the compositor: the next release shows the canvas as it is by then.
		s.deferred = true
		return
	}
	area := intersectScissor(b.pending, canvas.Bounds())
	if area.Right <= area.Left || area.Bottom <= area.Top {
		return // the buffer already holds the canvas as it is
	}
	for y := area.Top; y < area.Bottom; y++ {
		copy(b.mem[y*b.stride+area.Left*4:y*b.stride+area.Right*4], canvas.Pix[y*canvas.Stride+area.Left*4:y*canvas.Stride+area.Right*4])
	}
	b.pending = render.Scissor{}
	damage = area
	b.busy = true
	c.request(s.child, 1, wlArg(b.buffer), 0, 0) // attach
	c.request(s.child, 9, wlArg(uint32(damage.Left)), wlArg(uint32(damage.Top)), wlArg(uint32(damage.Right-damage.Left)), wlArg(uint32(damage.Bottom-damage.Top)))
	c.request(s.child, 6) // commit
	c.flush(s.s.display)
}

func unionScissor(a, b render.Scissor) render.Scissor {
	if a.Right <= a.Left || a.Bottom <= a.Top {
		return b
	}
	if b.Right <= b.Left || b.Bottom <= b.Top {
		return a
	}
	return render.Scissor{Left: min(a.Left, b.Left), Top: min(a.Top, b.Top), Right: max(a.Right, b.Right), Bottom: max(a.Bottom, b.Bottom)}
}

func intersectScissor(a, b render.Scissor) render.Scissor {
	return render.Scissor{Left: max(a.Left, b.Left), Top: max(a.Top, b.Top), Right: min(a.Right, b.Right), Bottom: min(a.Bottom, b.Bottom)}
}

// wp_cursor_shape_device_v1.shape for the cursors the application asks for, in app.Cursor order.
var cursorShapes = [...]uint32{1 /* default */, 9 /* text */, 4 /* pointer */, 8 /* crosshair */, 6 /* wait */, 26 /* ew-resize */, 27 /* ns-resize */, 29 /* nwse-resize */, 28 /* nesw-resize */, 15 /* not-allowed */}

// applyCursor shows the window's cursor on the subsurface, which GDK does not manage.
func (s *waylandSurface) applyCursor() {
	session := s.s
	if session.focus != s {
		return
	}
	shape := uint8(0)
	if s.haveCursor {
		shape = s.cursor
	}
	if int(shape) >= len(cursorShapes) {
		shape = 0
	}
	if session.cursorDevice == 0 {
		session.themeCursor(shape)
		return
	}
	session.c.request(session.cursorDevice, 1, wlArg(session.enterSerial), wlArg(cursorShapes[shape]))
	session.c.flush(session.display)
}

// setCursor changes the cursor of the subsurface.
func (s *waylandSurface) setCursor(shape uint8) {
	s.cursor, s.haveCursor = shape, true
	s.applyCursor()
}
