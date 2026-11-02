package windows

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

// customCursors are the cursors this package made for application pictures: the one the window
// shows and the one it showed before. A cursor is destroyed only once a later one has replaced
// it and a further one has followed, so the window, which applies a new cursor on its own
// thread a moment after it is stored, is never left holding a handle that is gone.
var customCursors struct {
	sync.Mutex
	shown, previous uintptr
	image           *ui.CursorImage
	scale           float32
}

// SetCursorImage makes the window's pointer the picture, in device pixels: a picture made for
// another scale than the window's is resampled to it, so it covers the same logical size.
func SetCursorImage(image *ui.CursorImage) error {
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
func retireCustomCursor(handle uintptr, image *ui.CursorImage, scale float32) {
	if customCursors.previous != 0 {
		destroyIcon.Call(customCursors.previous)
	}
	customCursors.previous, customCursors.shown = customCursors.shown, handle
	customCursors.image, customCursors.scale = image, scale
}

// newCursorHandle makes an HCURSOR of the picture with its hotspot. A 32-bit cursor bitmap is
// read as plain ARGB, the color not multiplied by alpha, which the cursor tests read back
// from Windows itself.
func newCursorHandle(image *ui.CursorImage, scale float32) (uintptr, error) {
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
