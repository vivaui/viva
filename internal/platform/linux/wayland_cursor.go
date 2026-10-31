//go:build linux

package linux

import (
	"fmt"
	"math"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

type waylandCursorAPI struct {
	library uintptr

	themeLoad      func(name *byte, size int32, shm uintptr) uintptr
	themeDestroy   func(theme uintptr)
	themeGetCursor func(theme uintptr, name *byte) unsafe.Pointer
	imageGetBuffer func(image unsafe.Pointer) uintptr
}

func loadWaylandCursor() (*waylandCursorAPI, error) {
	lib, err := purego.Dlopen("libwayland-cursor.so.0", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil, fmt.Errorf("linux: failed to load libwayland-cursor.so.0: %w", err)
	}
	api := &waylandCursorAPI{library: lib}
	purego.RegisterLibFunc(&api.themeLoad, lib, "wl_cursor_theme_load")
	purego.RegisterLibFunc(&api.themeDestroy, lib, "wl_cursor_theme_destroy")
	purego.RegisterLibFunc(&api.themeGetCursor, lib, "wl_cursor_theme_get_cursor")
	purego.RegisterLibFunc(&api.imageGetBuffer, lib, "wl_cursor_image_get_buffer")
	return api, nil
}

func (api *waylandCursorAPI) close() {
	if api != nil && api.library != 0 {
		purego.Dlclose(api.library)
		api.library = 0
	}
}

type wlCursorImage struct {
	Width    uint32
	Height   uint32
	HotspotX uint32
	HotspotY uint32
	Delay    uint32
}

type wlCursor struct {
	ImageCount uint32
	Images     *unsafe.Pointer
	Name       *byte
}

// Cursor name pairs: primary modern FreeDesktop name, fallback legacy X11 name
var waylandCursorNames = [...][2]string{
	{"default", "left_ptr"},
	{"text", "xterm"},
	{"pointer", "hand2"},
	{"crosshair", "crosshair"},
	{"wait", "watch"},
	{"ew-resize", "sb_h_double_arrow"},
	{"ns-resize", "sb_v_double_arrow"},
	{"nwse-resize", "size_fdiag"},
	{"nesw-resize", "size_bdiag"},
	{"not-allowed", "crossed_circle"},
}

type waylandCursorManager struct {
	mu sync.Mutex

	api           *waylandCursorAPI
	theme         uintptr
	themeScale    float32
	cursorSurface uintptr
	currentShape  uint8
}

func (app *waylandApplication) initCursorManager() {
	if app.cursorMgr != nil {
		return
	}
	api, err := loadWaylandCursor()
	if err != nil {
		return
	}
	app.cursorMgr = &waylandCursorManager{api: api}
}

func (app *waylandApplication) setCursor(shape uint8) {
	if int(shape) >= len(waylandCursorNames) {
		shape = 0
	}
	app.currentCursorShape = shape

	if app.pointer == 0 || app.pointerSerial == 0 || app.globals.shm == 0 || app.globals.compositor == 0 || app.api == nil {
		return
	}
	if app.cursorMgr == nil {
		app.initCursorManager()
		if app.cursorMgr == nil {
			return
		}
	}

	mgr := app.cursorMgr
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if mgr.api == nil {
		return
	}

	scale := float32(1.0)
	if app.pointerFocus != nil {
		scale = app.pointerFocus.displayScale()
	}
	intScale := int32(math.Ceil(float64(scale)))
	if intScale < 1 {
		intScale = 1
	}

	// 1. Ensure cursor surface exists
	if mgr.cursorSurface == 0 {
		mgr.cursorSurface = app.api.createSurface(app.globals.compositor)
		if mgr.cursorSurface == 0 {
			return
		}
	}

	// 2. Ensure cursor theme is loaded for current scale
	targetSize := int32(24 * intScale)
	if mgr.theme == 0 || mgr.themeScale != float32(intScale) {
		if mgr.theme != 0 && mgr.api.themeDestroy != nil {
			mgr.api.themeDestroy(mgr.theme)
		}
		if mgr.api.themeLoad != nil {
			mgr.theme = mgr.api.themeLoad(nil, targetSize, app.globals.shm)
		}
		mgr.themeScale = float32(intScale)
		if mgr.theme == 0 {
			return
		}
	}

	mgr.currentShape = shape

	// 3. Resolve cursor image
	names := waylandCursorNames[shape]
	var cursorPtr unsafe.Pointer
	for _, n := range names {
		if n == "" {
			continue
		}
		cName := append([]byte(n), 0)
		if mgr.api.themeGetCursor != nil {
			cursorPtr = mgr.api.themeGetCursor(mgr.theme, &cName[0])
		}
		if cursorPtr != nil {
			break
		}
	}
	if cursorPtr == nil {
		return
	}

	cursor := (*wlCursor)(cursorPtr)
	if cursor.ImageCount == 0 || cursor.Images == nil {
		return
	}

	imgPtr := *cursor.Images
	if imgPtr == nil {
		return
	}
	img := (*wlCursorImage)(imgPtr)
	var buffer uintptr
	if mgr.api.imageGetBuffer != nil {
		buffer = mgr.api.imageGetBuffer(imgPtr)
	}
	if buffer == 0 {
		return
	}

	// 4. Attach buffer to cursor surface, set buffer scale, damage and commit
	app.api.surfaceAttach(mgr.cursorSurface, buffer, 0, 0)
	app.api.surfaceSetBufferScale(mgr.cursorSurface, intScale)
	app.api.surfaceDamage(mgr.cursorSurface, 0, 0, int32(img.Width), int32(img.Height))
	app.api.surfaceCommit(mgr.cursorSurface)

	// 5. Set cursor on pointer: hotspot is in surface-local coordinates
	hotspotX := int32(img.HotspotX) / intScale
	hotspotY := int32(img.HotspotY) / intScale
	app.api.pointerSetCursor(app.pointer, app.pointerSerial, mgr.cursorSurface, hotspotX, hotspotY)
	if app.api.displayFlush != nil && app.display != 0 {
		app.api.displayFlush(app.display)
	}
}

func (mgr *waylandCursorManager) destroy(api *waylandAPI) {
	if mgr == nil {
		return
	}
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if mgr.cursorSurface != 0 && api != nil {
		api.surfaceDestroy(mgr.cursorSurface)
		mgr.cursorSurface = 0
	}
	if mgr.theme != 0 && mgr.api != nil && mgr.api.themeDestroy != nil {
		mgr.api.themeDestroy(mgr.theme)
		mgr.theme = 0
	}
	if mgr.api != nil {
		mgr.api.close()
		mgr.api = nil
	}
}
