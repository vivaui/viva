package linux

import (
	"encoding/binary"
	"fmt"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

// An application picture is an X cursor through the RENDER extension, the way every toolkit makes
// a cursor with a soft edge and its own colors: the pixels go to a 32-bit pixmap, RENDER names a
// picture of it, and a cursor is made from the picture. libxcb-render is optional, like the
// cursor theme library; without it, or on a server without RENDER, the picture is refused and
// the pointer keeps its shape.

// xcbRender is libxcb-render and the format RENDER keeps for 32-bit color with alpha.
type xcbRender struct {
	library           uintptr
	argb32            uint32
	queryVersion      func(uintptr, uint32, uint32) xcbVoidCookie
	queryVersionReply func(uintptr, xcbVoidCookie, *unsafe.Pointer) unsafe.Pointer
	queryFormats      func(uintptr) xcbVoidCookie
	queryFormatsReply func(uintptr, xcbVoidCookie, *unsafe.Pointer) unsafe.Pointer
	createPicture     func(uintptr, uint32, uint32, uint32, uint32, *uint32) xcbVoidCookie
	freePicture       func(uintptr, uint32) xcbVoidCookie
	createCursor      func(uintptr, uint32, uint32, uint16, uint16) xcbVoidCookie
}

const (
	renderFormatInfoSize = 28 // xcb_render_pictforminfo_t
	renderFormatDirect   = 1
)

// openRender loads libxcb-render and asks the server for its 32-bit ARGB picture format. It
// returns nil, and why, when the picture cannot be made.
func openRender(api *xcbAPI, connection uintptr) (*xcbRender, error) {
	library, err := purego.Dlopen("libxcb-render.so.0", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil, fmt.Errorf("load libxcb-render: %w", err)
	}
	render := &xcbRender{library: library}
	purego.RegisterLibFunc(&render.queryVersion, library, "xcb_render_query_version")
	purego.RegisterLibFunc(&render.queryVersionReply, library, "xcb_render_query_version_reply")
	purego.RegisterLibFunc(&render.queryFormats, library, "xcb_render_query_pict_formats")
	purego.RegisterLibFunc(&render.queryFormatsReply, library, "xcb_render_query_pict_formats_reply")
	purego.RegisterLibFunc(&render.createPicture, library, "xcb_render_create_picture_checked")
	purego.RegisterLibFunc(&render.freePicture, library, "xcb_render_free_picture")
	purego.RegisterLibFunc(&render.createCursor, library, "xcb_render_create_cursor_checked")
	var failure unsafe.Pointer
	version := render.queryVersionReply(connection, render.queryVersion(connection, 0, 11), &failure)
	if version == nil {
		freeReply(api, failure)
		purego.Dlclose(library)
		return nil, fmt.Errorf("the server has no RENDER extension")
	}
	api.free(version)
	reply := render.queryFormatsReply(connection, render.queryFormats(connection), &failure)
	if reply == nil {
		freeReply(api, failure)
		purego.Dlclose(library)
		return nil, fmt.Errorf("the server did not list its picture formats")
	}
	defer api.free(reply)
	// xcb_render_query_pict_formats_reply_t: the header is 32 bytes and num_formats is at 8,
	// followed by that many format records.
	count := int(binary.LittleEndian.Uint32(unsafe.Slice((*byte)(reply), 32)[8:]))
	formats := unsafe.Slice((*byte)(unsafe.Add(reply, 32)), count*renderFormatInfoSize)
	id, found := findARGB32(formats)
	if !found {
		purego.Dlclose(library)
		return nil, fmt.Errorf("the server has no 32-bit color format with alpha")
	}
	render.argb32 = id
	return render, nil
}

func freeReply(api *xcbAPI, reply unsafe.Pointer) {
	if reply != nil {
		api.free(reply)
	}
}

// findARGB32 is the id of the direct-color format of depth 32 with 8 bits each of alpha (at bit
// 24), red, green and blue, in formats, which holds xcb_render_pictforminfo_t records.
func findARGB32(formats []byte) (uint32, bool) {
	for offset := 0; offset+renderFormatInfoSize <= len(formats); offset += renderFormatInfoSize {
		record := formats[offset : offset+renderFormatInfoSize]
		field := func(at int) uint16 { return binary.LittleEndian.Uint16(record[at:]) }
		if record[4] != renderFormatDirect || record[5] != 32 {
			continue
		}
		if field(8) == 16 && field(10) == 0xff && field(12) == 8 && field(14) == 0xff &&
			field(16) == 0 && field(18) == 0xff && field(20) == 24 && field(22) == 0xff {
			return binary.LittleEndian.Uint32(record[0:]), true
		}
	}
	return 0, false
}

const (
	xcbImageFormatZPixmap = 2
	putImageStrip         = 32 * 1024 // bytes of pixels a request carries, under any server's limit
)

// SetCursorImage makes the picture the pointer over the window. The X server draws it from its
// own pixels, so a picture made for another scale than the window's is resampled to it.
func SetCursorImage(image *ui.CursorImage) error {
	if hooks := shellHooks.Load(); hooks != nil {
		return hooks.setCursorImage(image)
	}
	win := activeWindow.Load()
	if win == nil {
		return fmt.Errorf("linux: cursor requires a running window")
	}
	win.mu.Lock()
	defer win.mu.Unlock()
	if activeWindow.Load() != win || win.connection == 0 {
		return fmt.Errorf("linux: cursor requires a running window")
	}
	return win.applyCursorImage(image)
}

func (win *window) applyCursorImage(image *ui.CursorImage) error {
	if !win.renderTried {
		win.renderTried = true
		render, err := openRender(win.api, win.connection)
		if err != nil {
			win.renderErr = err
		}
		win.render = render
	}
	if win.render == nil {
		return unsupported(fmt.Sprintf("an application cursor needs RENDER: %v", win.renderErr))
	}
	width, height, hotX, hotY, pixels := platform.CursorPixels(image, win.displayScale(), true)
	api, connection, render := win.api, win.connection, win.render
	pixmap := api.generateID(connection)
	if err := win.checkRequest("create cursor pixmap", api.createPixmap(connection, 32, pixmap, win.screen.Root, uint16(width), uint16(height))); err != nil {
		return err
	}
	cleanup := func() {
		api.freePixmap(connection, pixmap)
		api.flush(connection)
	}
	gc := api.generateID(connection)
	if err := win.checkRequest("create cursor context", api.createGC(connection, gc, pixmap, 0, nil)); err != nil {
		cleanup()
		return err
	}
	rows := max(putImageStrip/(width*4), 1)
	for top := 0; top < height; top += rows {
		count := min(rows, height-top)
		strip := pixels[top*width*4 : (top+count)*width*4]
		if err := win.checkRequest("put cursor image", api.putImage(connection, xcbImageFormatZPixmap, pixmap, gc,
			uint16(width), uint16(count), 0, int16(top), 0, 32, uint32(len(strip)), &strip[0])); err != nil {
			api.freeGC(connection, gc)
			cleanup()
			return err
		}
	}
	api.freeGC(connection, gc)
	picture := api.generateID(connection)
	if err := win.checkRequest("create cursor picture", render.createPicture(connection, picture, pixmap, render.argb32, 0, nil)); err != nil {
		cleanup()
		return err
	}
	cursor := api.generateID(connection)
	err := win.checkRequest("create cursor", render.createCursor(connection, cursor, picture, uint16(hotX), uint16(hotY)))
	render.freePicture(connection, picture)
	api.freePixmap(connection, pixmap)
	if err != nil {
		api.flush(connection)
		return err
	}
	if err := win.checkRequest("apply cursor", api.changeWindow(connection, win.id, xcbCWCursor, &cursor)); err != nil {
		api.freeCursor(connection, cursor)
		api.flush(connection)
		return err
	}
	if previous := win.cursor; previous != 0 {
		api.freeCursor(connection, previous)
	}
	if api.flush(connection) <= 0 || api.connectionHasError(connection) != 0 {
		api.freeCursor(connection, cursor)
		return fmt.Errorf("linux: apply cursor: XCB connection failed")
	}
	win.cursor = cursor
	return nil
}
