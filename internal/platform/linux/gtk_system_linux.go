//go:build linux && vivagtk

package linux

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/vivaui/viva/internal/platform"
)

// The desktop services that go through the display server, on GTK: the clipboard, the primary
// selection and the cursor. GTK has to be called from the thread that runs its loop, so a call from
// another goroutine is posted to it and waited for.

// gtkClipboards are the entry points of the clipboard, bound with the first use.
type gtkClipboards struct {
	atomIntern func(name *byte, onlyIfExists int32) uintptr
	get        func(selection uintptr) uintptr
	waitText   func(clipboard uintptr) uintptr
	setText    func(clipboard uintptr, text *byte, length int32)
	hasImage   func(clipboard uintptr) int32
	waitImage  func(clipboard uintptr) uintptr
	setImage   func(clipboard, pixbuf uintptr)
	clear      func(clipboard uintptr)
	saveBuffer func(pixbuf uintptr, buffer *uintptr, size *uint64, kind *byte, keys, values uintptr, err *uintptr) int32

	clipboard, primary uintptr
	ok                 bool
}

var clipboards gtkClipboards

func (a *gtkApplication) clipboardAPI() (*gtkClipboards, error) {
	if clipboards.ok {
		return &clipboards, nil
	}
	g := a.g
	c := &clipboards
	for _, name := range []struct {
		function any
		symbol   string
	}{
		{&c.atomIntern, "gdk_atom_intern"}, {&c.get, "gtk_clipboard_get"}, {&c.waitText, "gtk_clipboard_wait_for_text"},
		{&c.setText, "gtk_clipboard_set_text"}, {&c.hasImage, "gtk_clipboard_wait_is_image_available"},
		{&c.waitImage, "gtk_clipboard_wait_for_image"}, {&c.setImage, "gtk_clipboard_set_image"}, {&c.clear, "gtk_clipboard_clear"},
	} {
		if !g.lookup(name.function, name.symbol) {
			return nil, fmt.Errorf("linux: the installed GTK lacks %s", name.symbol)
		}
	}
	selection := func(name string) uintptr {
		text := cString(name)
		return c.get(c.atomIntern(&text[0], 0))
	}
	c.clipboard, c.primary = selection("CLIPBOARD"), selection("PRIMARY")
	// Images need gdk-pixbuf, which GTK links but this program has not opened. Without it text
	// still works.
	if library, err := purego.Dlopen("libgdk_pixbuf-2.0.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL); err == nil {
		g.libraries = append(g.libraries, library)
		if !g.lookup(&c.saveBuffer, "gdk_pixbuf_save_to_bufferv") {
			c.saveBuffer = nil
		}
	}
	c.ok = true
	return c, nil
}

// onMain runs function on the loop's thread and waits for it. On that thread it just runs.
func (a *gtkApplication) onMain(function func()) error {
	if a.mainThread == syscall.Gettid() {
		function()
		return nil
	}
	done := make(chan struct{})
	a.enqueue(func() {
		function()
		close(done)
	})
	select {
	case <-done:
		return nil
	case <-time.After(10 * time.Second):
		return fmt.Errorf("linux: the window thread did not answer")
	}
}

// installHooks makes the desktop services the shell's.
func (a *gtkApplication) installHooks() {
	hooks := &desktopHooks{
		clipboardText: func() (value string, err error) {
			err = a.onMain(func() { value, err = a.readText(false) })
			return
		},
		setClipboardText: func(value string) (err error) {
			if e := a.onMain(func() { err = a.writeText(false, value) }); e != nil {
				return e
			}
			return
		},
		primaryText: func() (value string, err error) {
			err = a.onMain(func() { value, err = a.readText(true) })
			return
		},
		setPrimaryText: func(value string) (err error) {
			if e := a.onMain(func() { err = a.writeText(true, value) }); e != nil {
				return e
			}
			return
		},
		hasClipboardImage: func() (has bool) {
			_ = a.onMain(func() { has = a.imageAvailable() })
			return
		},
		clipboardImagePNG: func() (png []byte, err error) {
			if e := a.onMain(func() { png, err = a.readImage() }); e != nil {
				return nil, e
			}
			return
		},
		setCursorShape: func(shape uint8) (err error) {
			if e := a.onMain(func() { err = a.setCursor(shape) }); e != nil {
				return e
			}
			return
		},
	}
	shellHooks.Store(hooks)
}

func (a *gtkApplication) removeHooks() { shellHooks.Store(nil) }

func (a *gtkApplication) readText(primary bool) (string, error) {
	c, err := a.clipboardAPI()
	if err != nil {
		return "", err
	}
	board := c.clipboard
	if primary {
		board = c.primary
	}
	text := c.waitText(board)
	if text == 0 {
		return "", nil
	}
	defer a.g.gFree(text)
	return goString(text), nil
}

func (a *gtkApplication) writeText(primary bool, value string) error {
	c, err := a.clipboardAPI()
	if err != nil {
		return err
	}
	board := c.clipboard
	if primary {
		board = c.primary
	}
	if value == "" {
		c.clear(board)
		return nil
	}
	data := cString(value)
	c.setText(board, &data[0], int32(len(data)-1))
	return nil
}

func (a *gtkApplication) imageAvailable() bool {
	c, err := a.clipboardAPI()
	return err == nil && c.hasImage(c.clipboard) != 0
}

// readImage returns the clipboard's image as PNG. GTK hands over a pixbuf, whatever form the owner
// offered, and it is encoded again.
func (a *gtkApplication) readImage() ([]byte, error) {
	c, err := a.clipboardAPI()
	if err != nil {
		return nil, err
	}
	if c.saveBuffer == nil {
		return nil, platform.ErrNoClipboardImage
	}
	if c.hasImage(c.clipboard) == 0 {
		return nil, platform.ErrNoClipboardImage
	}
	pixbuf := c.waitImage(c.clipboard)
	if pixbuf == 0 {
		return nil, platform.ErrNoClipboardImage
	}
	defer a.g.objectUnref(pixbuf)
	var buffer uintptr
	var size uint64
	var failure uintptr
	kind := cString("png")
	if c.saveBuffer(pixbuf, &buffer, &size, &kind[0], 0, 0, &failure) == 0 || buffer == 0 {
		return nil, fmt.Errorf("linux: encode the clipboard image failed")
	}
	defer a.g.gFree(buffer)
	if size > platform.MaxClipboardImageBytes {
		return nil, platform.ErrClipboardImageTooLarge
	}
	return append([]byte(nil), unsafe.Slice((*byte)(unsafePointer(buffer)), size)...), nil
}

// cursorNames are in GDK's cursor-name terms the theme names of the cursors (cursor_linux.go).
func (a *gtkApplication) setCursor(shape uint8) error {
	if int(shape) >= len(cursorNames) {
		return fmt.Errorf("linux: invalid cursor %d", shape)
	}
	window := a.active
	if window == nil || window.closed {
		return fmt.Errorf("linux: cursor requires a running window")
	}
	window.setCursor(shape)
	return nil
}

func (w *gtkWindow) setCursor(shape uint8) {
	if w.cursorShape == shape+1 {
		return
	}
	w.cursorShape = shape + 1
	g := w.g
	if w.cursors[shape] == 0 {
		name := cString(cursorNames[shape])
		w.cursors[shape] = g.gdkCursorFromName(g.gdkDisplayDefault(), &name[0])
	}
	if window := g.widgetGetWindow(w.area); window != 0 {
		g.gdkWindowSetCursor(window, w.cursors[shape])
	}
	for _, p := range w.pieces {
		if p.window != 0 {
			g.gdkWindowSetCursor(p.window, w.cursors[shape])
		}
	}
	for _, p := range w.inputPieces {
		if window := g.widgetGetWindow(p.widget); window != 0 {
			g.gdkWindowSetCursor(window, w.cursors[shape])
		}
	}
	if w.wl != nil {
		w.wl.setCursor(shape)
	}
}
