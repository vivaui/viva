//go:build linux && vivagtk

package linux

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/ebitengine/purego"
	"github.com/vivaui/viva/internal/platform"
)

// RunGTK runs one window on the GTK shell (ADR 0064).
func RunGTK(config platform.Config) error {
	return RunGTKApplication(func(application platform.Application) error {
		_, err := application.OpenWindow(config)
		return err
	})
}

// gtkApplication owns the GLib main loop and the windows. Everything on it runs on the thread that
// called RunGTKApplication, except post and Quit.
type gtkApplication struct {
	// desktopServices are the session bus connections: the status icons, the watch on the
	// desktop's settings and the accessibility bridge.
	desktopServices
	g        *gtkLibrary
	windows  map[uintptr]*gtkWindow
	nextID   uintptr
	firstErr error
	wayland  bool
	started  bool
	stopping bool
	// mainThread is the thread that runs the loop, which is where GTK may be called.
	mainThread int
	// active is the window the keyboard or the pointer was last in.
	active *gtkWindow

	tasks gtkTasks
	// settings are the desktop's settings as last read; settingsPending says a change is being
	// handled.
	settings        xsettings
	settingsPending bool
	// watched are the descriptors of the session bus connections the loop listens to, by the id
	// of the GLib source that does.
	watched map[int]uint32
}

// gtkTasks are the functions other goroutines asked the loop to run.
type gtkTasks struct {
	mu      sync.Mutex
	next    uintptr
	waiting map[uintptr]func()
}

func (t *gtkTasks) add(function func()) uintptr {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.next++
	if t.waiting == nil {
		t.waiting = map[uintptr]func(){}
	}
	t.waiting[t.next] = function
	return t.next
}

func (t *gtkTasks) take(id uintptr) func() {
	t.mu.Lock()
	defer t.mu.Unlock()
	function := t.waiting[id]
	delete(t.waiting, id)
	return function
}

// gtkRunning is the application the callbacks belong to. A process has one GTK main loop.
var gtkRunning atomic.Pointer[gtkApplication]

// RunGTKApplication is the GTK shell's RunApplication.
func RunGTKApplication(start platform.ApplicationStart) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	g, err := loadGTK()
	if err != nil {
		return err
	}
	application := &gtkApplication{g: g, windows: map[uintptr]*gtkWindow{}, watched: map[int]uint32{}}
	application.host = application
	if !gtkRunning.CompareAndSwap(nil, application) {
		return fmt.Errorf("linux: the GTK shell is already running")
	}
	defer gtkRunning.Store(nil)
	callbacks()
	application.mainThread = syscall.Gettid()
	application.installHooks()
	defer application.removeHooks()
	// Whoever starts to watch the desktop's settings, from any goroutine, wakes the loop to open the
	// connection that hears them.
	platform.SetWatchHook(func() { application.enqueue(application.syncDesktopWatch) })
	defer platform.SetWatchHook(nil)
	defer application.closeServices()
	if err := start(application); err != nil {
		application.destroyAll()
		return err
	}
	if len(application.windows) > 0 {
		g.main()
	}
	application.destroyAll()
	return application.firstErr
}

// initialize starts GTK on the first window, when the application's name is known: it is the
// class a window manager groups windows under and the application id Wayland compositors do.
func (a *gtkApplication) initialize(class string) error {
	if a.started {
		return nil
	}
	if class == "" {
		if executable, err := os.Executable(); err == nil {
			class = filepath.Base(executable)
		}
	}
	if class != "" {
		name := cString(class)
		a.g.setPrgname(&name[0])
		a.g.gdkSetProgramClass(&name[0])
	}
	if a.g.initCheck(nil, 0) == 0 {
		return fmt.Errorf("linux: GTK could not open a display (neither WAYLAND_DISPLAY nor DISPLAY is usable)")
	}
	display := a.g.gdkDisplayDefault()
	a.wayland = typeNameOf(a.g, display) == "GdkWaylandDisplay"
	a.started = true
	a.watchSettings()
	return nil
}

// enqueue runs function on the loop's thread. It is safe from any goroutine.
func (a *gtkApplication) enqueue(function func()) {
	a.g.idleAdd(callbacks().task, a.tasks.add(function))
}

// fail ends the loop with an error, once.
func (a *gtkApplication) fail(err error) {
	if a.firstErr == nil {
		a.firstErr = err
	}
	a.stop()
}

// stop ends the main loop; the windows left are destroyed by RunGTKApplication.
func (a *gtkApplication) stop() {
	if !a.stopping {
		a.stopping = true
		a.g.mainQuit()
	}
}

func (a *gtkApplication) destroyAll() {
	for len(a.windows) > 0 {
		for _, window := range a.windows {
			window.destroy()
			break
		}
	}
}

// Quit closes every window, whatever they would answer a close request, and ends the loop.
func (a *gtkApplication) Quit() {
	a.enqueue(func() {
		a.destroyAll()
		a.stop()
	})
}

func (a *gtkApplication) SetDockBadge(string) {}

// post is a command to the loop, from the services (see commandHide and the rest).
func (a *gtkApplication) post(command uint32) {
	switch command {
	case commandStatus:
		a.enqueue(a.drainStatusRemovals)
	case commandDesktop:
		a.enqueue(a.syncDesktopWatch)
	case commandQuit:
		a.Quit()
	}
}

// refreshText draws every window's text again, with the fonts now in force.
func (a *gtkApplication) refreshText() {
	for _, w := range a.windows {
		w.refreshText()
	}
}

func (a *gtkApplication) atspiShells() []atspiShell {
	shells := make([]atspiShell, 0, len(a.windows))
	for _, w := range a.windows {
		shells = append(shells, w)
	}
	return shells
}

// rebuildPoll brings the descriptors the GLib loop listens to in line with the services' connections.
func (a *gtkApplication) rebuildPoll() {
	wanted := map[int]bool{}
	if fd := a.atspi.fd(); fd >= 0 {
		wanted[fd] = true
	}
	if a.desktop != nil {
		wanted[a.desktop.conn.FD()] = true
	}
	if a.status != nil {
		wanted[a.status.conn.FD()] = true
	}
	for fd, source := range a.watched {
		if !wanted[fd] {
			a.g.sourceRemove(source)
			delete(a.watched, fd)
		}
	}
	for fd := range wanted {
		if _, ok := a.watched[fd]; !ok {
			const gioIn, gioHup, gioErr = 1, 16, 8
			a.watched[fd] = a.g.unixFDAdd(int32(fd), gioIn|gioHup|gioErr, callbacks().descriptor, uintptr(fd))
		}
	}
}

// serveDescriptor handles what arrived on one of the services' connections.
func (a *gtkApplication) serveDescriptor(fd int) {
	switch {
	case a.atspi.fd() == fd:
		a.serveAtspi()
	case a.desktop != nil && a.desktop.conn.FD() == fd:
		a.serveDesktop()
	case a.status != nil && a.status.conn.FD() == fd:
		a.serveStatus()
	}
}

// closeServices lets go of the services' connections when the loop has ended.
func (a *gtkApplication) closeServices() {
	a.closeDesktop()
	a.closeStatus()
	a.atspiClose()
	for fd, source := range a.watched {
		a.g.sourceRemove(source)
		delete(a.watched, fd)
	}
}

// typeNameOf is the name of an object's GType, such as GdkWaylandDisplay.
func typeNameOf(g *gtkLibrary, object uintptr) string {
	class := *(*uintptr)(unsafePointer(object))
	return goString(g.typeName(*(*uintptr)(unsafePointer(class))))
}

// gtkCallbacks are the C-callable functions the shell hands GTK. Each is made once: purego never
// frees a callback and allows a fixed number, so a window must not make its own. The window a call
// is for travels in the data pointer, as its id.
type gtkCallbacks struct {
	task, descriptor, settings, dragMotion, dragLeave, dragDrop, dragData, event, draw, allocate, destroy, deleteEvent, mapped, unmapped, commit, preedit, surrounding, deleteSurrounding, redraw, webMessage, webLoad, webScheme uintptr
}

var (
	callbackOnce sync.Once
	callbackSet  gtkCallbacks
)

func callbacks() *gtkCallbacks {
	callbackOnce.Do(func() {
		callbackSet = gtkCallbacks{
			// gboolean (*GSourceFunc)(gpointer): return FALSE to remove the source.
			task: purego.NewCallback(func(id uintptr) uintptr {
				if a := gtkRunning.Load(); a != nil {
					if function := a.tasks.take(id); function != nil {
						function()
					}
				}
				return 0
			}),
			// gboolean (*GUnixFDSourceFunc)(gint fd, GIOCondition condition, gpointer data): TRUE keeps
			// the source.
			descriptor: purego.NewCallback(func(fd, condition, data uintptr) uintptr {
				if a := gtkRunning.Load(); a != nil {
					a.serveDescriptor(int(fd))
				}
				return 1
			}),
			// void (*notify)(GObject *settings, GParamSpec *, gpointer).
			settings: purego.NewCallback(func(object, pspec, data uintptr) uintptr {
				if a := gtkRunning.Load(); a != nil {
					a.settingsChanged()
				}
				return 0
			}),
			// gboolean drag-motion(widget, context, x, y, time) and drag-drop(same).
			dragMotion: purego.NewCallback(func(widget, context, x, y, t, id uintptr) uintptr {
				if w := gtkWindowByID(id); w != nil {
					return w.dragMotion(context, int32(x), int32(y), uint32(t))
				}
				return 0
			}),
			dragLeave: purego.NewCallback(func(widget, context, t, id uintptr) uintptr {
				if w := gtkWindowByID(id); w != nil {
					w.dragLeave()
				}
				return 0
			}),
			dragDrop: purego.NewCallback(func(widget, context, x, y, t, id uintptr) uintptr {
				if w := gtkWindowByID(id); w != nil {
					return w.dragDrop(widget, context, int32(x), int32(y), uint32(t))
				}
				return 0
			}),
			// void drag-data-received(widget, context, x, y, data, info, time).
			dragData: purego.NewCallback(func(widget, context, x, y, data, info, t, id uintptr) uintptr {
				if w := gtkWindowByID(id); w != nil {
					w.dragData(context, int32(x), int32(y), data, uint32(t))
				}
				return 0
			}),
			redraw: purego.NewCallback(func(id uintptr) uintptr {
				if w := gtkWindowByID(id); w != nil {
					w.redrawRequested.Store(false)
					w.redraw()
				}
				return 0
			}),
			event: purego.NewCallback(func(widget, event, id uintptr) uintptr {
				if w := gtkWindowByID(id); w != nil {
					return w.handleEvent(widget, event)
				}
				return 0
			}),
			draw: purego.NewCallback(func(widget, cr, id uintptr) uintptr {
				if w := gtkWindowByID(id); w != nil {
					w.draw(widget, cr)
				}
				return 1
			}),
			allocate: purego.NewCallback(func(widget, rect, id uintptr) uintptr {
				if w := gtkWindowByID(id); w != nil {
					r := (*[4]int32)(unsafePointer(rect))
					w.allocated(widget, r[0], r[1], r[2], r[3])
				}
				return 0
			}),
			destroy: purego.NewCallback(func(widget, id uintptr) uintptr {
				if w := gtkWindowByID(id); w != nil {
					w.destroyed()
				}
				return 0
			}),
			deleteEvent: purego.NewCallback(func(widget, event, id uintptr) uintptr {
				if w := gtkWindowByID(id); w != nil && w.closeRequested() {
					return 1 // the application decides
				}
				return 0
			}),
			mapped: purego.NewCallback(func(widget, id uintptr) uintptr {
				if w := gtkWindowByID(id); w != nil && widget == w.toplevel {
					w.mapChanged(true)
				}
				return 0
			}),
			unmapped: purego.NewCallback(func(widget, id uintptr) uintptr {
				if w := gtkWindowByID(id); w != nil && widget == w.toplevel {
					w.mapChanged(false)
				}
				return 0
			}),
			// void script-message-received(manager, value, handle).
			webMessage: purego.NewCallback(func(manager, value, handle uintptr) uintptr {
				if v := webViews[handle]; v != nil {
					v.message(value)
				}
				return 0
			}),
			// void load-changed(view, WebKitLoadEvent, handle).
			webLoad: purego.NewCallback(func(view, event, handle uintptr) uintptr {
				if v := webViews[handle]; v != nil {
					v.loadChanged(int32(event))
				}
				return 0
			}),
			// void (*WebKitURISchemeRequestCallback)(request, user_data).
			webScheme: purego.NewCallback(func(request, data uintptr) uintptr {
				if a := gtkRunning.Load(); a != nil {
					serveScheme(a.g, request)
				}
				return 0
			}),
			commit: purego.NewCallback(func(context, text, id uintptr) uintptr {
				if w := gtkWindowByID(id); w != nil && w.ime != nil {
					w.ime.commit(goString(text))
				}
				return 0
			}),
			preedit: purego.NewCallback(func(context, id uintptr) uintptr {
				if w := gtkWindowByID(id); w != nil && w.ime != nil {
					w.ime.preeditChanged()
				}
				return 0
			}),
			surrounding: purego.NewCallback(func(context, id uintptr) uintptr {
				if w := gtkWindowByID(id); w != nil && w.ime != nil {
					return w.ime.retrieveSurrounding()
				}
				return 0
			}),
			deleteSurrounding: purego.NewCallback(func(context, offset, count, id uintptr) uintptr {
				if w := gtkWindowByID(id); w != nil && w.ime != nil {
					return w.ime.deleteSurrounding(int(int32(offset)), int(int32(count)))
				}
				return 0
			}),
		}
	})
	return &callbackSet
}

// gtkWindowByID finds the window a callback is for, nil once it is gone. Only the loop's thread
// calls it.
func gtkWindowByID(id uintptr) *gtkWindow {
	if a := gtkRunning.Load(); a != nil {
		return a.windows[id]
	}
	return nil
}
