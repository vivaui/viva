package windows

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

// customCursors are the cursors this package made for application pictures: the one the window
// shows and the one it showed before. A cursor is destroyed only once a later one has replaced
// it and a further one has followed, so the window, which applies a new cursor on its own
// thread a moment after it is stored, is never left holding a handle that is gone.
var customCursors struct {
	sync.Mutex
	shown, previous uintptr
	image           *gfx.CursorImage
	scale           float32
}

// SetCursorImage makes the window's pointer the picture, in device pixels: a picture made for
// another scale than the window's is resampled to it, so it covers the same logical size.
func SetCursorImage(image *gfx.CursorImage) error {
	state := active.Load()
	if state == nil || state.window == 0 {
		return fmt.Errorf("windows: cursor requires a running window")
	}
	scale := state.scale
	if scale <= 0 {
		scale = 1
	}
	customCursors.Lock()
	defer customCursors.Unlock()
	if customCursors.shown != 0 && customCursors.image == image && customCursors.scale == scale && state.cursor.Load() == customCursors.shown {
		return nil
	}
	handle, err := newCursorHandle(image, scale)
	if err != nil {
		return err
	}
	state.cursor.Store(handle)
	if posted, _, callErr := postMessage.Call(state.window, wmApplyCursor, 0, 0); posted == 0 {
		return windowsCallError("apply cursor", callErr)
	}
	retireCustomCursor(handle, image, scale)
	return nil
}

// retireCustomCursor records handle as the cursor in use and destroys the one before the one it
// replaces. customCursors is locked.
func retireCustomCursor(handle uintptr, image *gfx.CursorImage, scale float32) {
	if customCursors.previous != 0 {
		destroyIcon.Call(customCursors.previous)
	}
	customCursors.previous, customCursors.shown = customCursors.shown, handle
	customCursors.image, customCursors.scale = image, scale
}

// grabbingCursorImage draws the closed-hand cursor that Win32 does not expose as
// a stock shape. Its 16x16 alpha bitmap is built once, on the first request, and
// the HCURSOR made of it is kept for the process: stock cursors are shared by
// the system and never destroyed, so a preset recreated per request would leak
// one per change.
var grabbingCursor = struct {
	once    sync.Once
	image   *gfx.CursorImage
	handle  uintptr
	failure error
}{}

func grabbingCursorHandle() (uintptr, error) {
	grabbingCursor.once.Do(func() {
		image, err := gfx.NewCursorImage(16, 16, 6, 3, 1, grabbingCursorPixels())
		if err != nil {
			grabbingCursor.failure = err
			return
		}
		handle, err := newCursorHandle(image, 1)
		if err != nil {
			grabbingCursor.failure = err
			return
		}
		grabbingCursor.image, grabbingCursor.handle = image, handle
	})
	return grabbingCursor.handle, grabbingCursor.failure
}

// closeGrabbingCursor destroys the cached cursor as the process exits its last
// window; the stock shapes need nothing.
func closeGrabbingCursor() {
	if handle := grabbingCursor.handle; handle != 0 {
		grabbingCursor.handle = 0
		destroyIcon.Call(handle)
	}
}

func grabbingCursorPixels() []byte {
	pixels := make([]byte, 16*16*4)
	inside := func(x, y int) bool {
		if x < 0 || x >= 16 || y < 0 || y >= 16 {
			return false
		}
		switch {
		case y == 2 && x >= 5 && x <= 6: // index finger
			return true
		case y == 3 && x >= 4 && x <= 7:
			return true
		case y >= 4 && y <= 5 && x >= 4 && x <= 7:
			return true
		case y >= 6 && y <= 8 && x >= 3 && x <= 12:
			return true
		case y >= 9 && y <= 10 && x >= 4 && x <= 12:
			return true
		case y == 11 && x >= 5 && x <= 11:
			return true
		case y >= 12 && y <= 13 && x >= 6 && x <= 11:
			return true
		}
		return false
	}
	set := func(x, y int, shade byte) {
		at := (y*16 + x) * 4
		pixels[at], pixels[at+1], pixels[at+2], pixels[at+3] = shade, shade, shade, 255
	}
	for y := 16; y > 0; {
		y--
		for x := 16; x > 0; {
			x--
			if inside(x, y) {
				set(x, y, 255)
				continue
			}
			if inside(x-1, y) || inside(x+1, y) || inside(x, y-1) || inside(x, y+1) {
				set(x, y, 0)
			}
		}
	}
	return pixels
}

// newCursorHandle makes an HCURSOR of the picture with its hotspot. A 32-bit cursor bitmap is
// read as plain ARGB, the color not multiplied by alpha, which the cursor tests read back
// from Windows itself.
func newCursorHandle(image *gfx.CursorImage, scale float32) (uintptr, error) {
	width, height, hotX, hotY, pixels := platform.CursorPixels(image, scale, false)
	header := bitmapV4Header{
		Size: uint32(unsafe.Sizeof(bitmapV4Header{})), Width: int32(width), Height: -int32(height),
		Planes: 1, Bits: 32, Compression: 3, // BI_BITFIELDS
		RedMask: 0x00ff0000, GreenMask: 0x0000ff00, BlueMask: 0x000000ff, AlphaMask: 0xff000000,
	}
	screen, _, _ := getDC.Call(0)
	defer releaseDC.Call(0, screen)
	var bits unsafe.Pointer
	color, _, callErr := createDIBSection.Call(screen, uintptr(unsafe.Pointer(&header)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if color == 0 || bits == nil {
		return 0, windowsCallError("create the cursor bitmap", callErr)
	}
	defer deleteObject.Call(color)
	copy(unsafe.Slice((*byte)(bits), len(pixels)), pixels)
	// The mask of a cursor with its own alpha is not read, but the call insists on one.
	mask, _, callErr := createBitmap.Call(uintptr(width), uintptr(height), 1, 1, 0)
	if mask == 0 {
		return 0, windowsCallError("create the cursor mask", callErr)
	}
	defer deleteObject.Call(mask)
	info := iconInfo{Icon: 0, XHotspot: uint32(hotX), YHotspot: uint32(hotY), Mask: mask, Color: color}
	handle, _, callErr := createIconIndirect.Call(uintptr(unsafe.Pointer(&info)))
	if handle == 0 {
		return 0, windowsCallError("create the cursor", callErr)
	}
	return handle, nil
}
