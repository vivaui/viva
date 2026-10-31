//go:build linux

package linux

import (
	"testing"
	"unsafe"
)

func TestWaylandCursorNamesCoverage(t *testing.T) {
	if len(waylandCursorNames) != 10 {
		t.Fatalf("expected 10 cursor names matching PointerCursor definitions, got %d", len(waylandCursorNames))
	}
	for i, names := range waylandCursorNames {
		if names[0] == "" || names[1] == "" {
			t.Errorf("cursor index %d has empty name pair: %+v", i, names)
		}
	}
}

func TestWaylandCursorSetCursorMock(t *testing.T) {
	type marshalCall struct {
		proxy  uintptr
		opcode uint32
		args   []any
	}
	var marshaled []marshalCall

	var destroyedProxies []uintptr
	api := &waylandAPI{
		proxyMarshalFlags: func(proxy uintptr, opcode uint32, interfacePtr uintptr, version uint32, flags uint32, args ...any) uintptr {
			marshaled = append(marshaled, marshalCall{proxy, opcode, args})
			if proxy == 999 && opcode == wlCompositorCreateSurfaceOpcode {
				return 1001 // return dummy cursor surface
			}
			return 0
		},
		proxyDestroy: func(proxy uintptr) {
			destroyedProxies = append(destroyedProxies, proxy)
		},
	}

	fakeImage := wlCursorImage{
		Width:    48,
		Height:   48,
		HotspotX: 10,
		HotspotY: 12,
		Delay:    0,
	}
	fakeImgPtr := unsafe.Pointer(&fakeImage)
	fakeCursor := wlCursor{
		ImageCount: 1,
		Images:     &fakeImgPtr,
	}
	fakeCursorPtr := unsafe.Pointer(&fakeCursor)
	fakeThemePtr := uintptr(555)
	fakeBufferPtr := uintptr(777)

	var (
		loadedThemeSize int32
		requestedName   string
		destroyedTheme  uintptr
	)

	cursorAPI := &waylandCursorAPI{
		themeLoad: func(name *byte, size int32, shm uintptr) uintptr {
			loadedThemeSize = size
			return fakeThemePtr
		},
		themeDestroy: func(theme uintptr) {
			destroyedTheme = theme
		},
		themeGetCursor: func(theme uintptr, name *byte) unsafe.Pointer {
			if name != nil {
				requestedName = goStringFromCString(name)
			}
			return fakeCursorPtr
		},
		imageGetBuffer: func(image unsafe.Pointer) uintptr {
			return fakeBufferPtr
		},
	}

	app := &waylandApplication{
		api:           api,
		pointer:       123,
		pointerSerial: 456,
		globals: waylandGlobals{
			compositor: 999,
			shm:        888,
		},
		cursorMgr: &waylandCursorManager{
			api: cursorAPI,
		},
	}

	// Fake window with scale = 2.0
	win := &waylandWindow{}
	win.storeScale(2.0)
	app.pointerFocus = win

	// Test shape 1 (text / xterm)
	app.setCursor(1)

	if loadedThemeSize != 48 { // 24 * scale 2
		t.Fatalf("expected loadedThemeSize = 48, got %d", loadedThemeSize)
	}
	if requestedName != "text" {
		t.Fatalf("expected requestedName = 'text', got %q", requestedName)
	}

	var (
		foundAttach      bool
		foundBufferScale bool
		foundDamage      bool
		foundCommit      bool
		foundSetCursor   bool
	)

	for _, call := range marshaled {
		if call.proxy == 1001 {
			switch call.opcode {
			case wlSurfaceAttachOpcode:
				if len(call.args) >= 1 && call.args[0] == fakeBufferPtr {
					foundAttach = true
				}
			case wlSurfaceSetBufferScaleOpcode:
				if len(call.args) >= 1 && call.args[0] == int32(2) {
					foundBufferScale = true
				}
			case wlSurfaceDamageOpcode:
				if len(call.args) >= 4 && call.args[2] == int32(48) && call.args[3] == int32(48) {
					foundDamage = true
				}
			case wlSurfaceCommitOpcode:
				foundCommit = true
			}
		}
		if call.proxy == 123 && call.opcode == wlPointerSetCursorOpcode {
			// serial, surface, hotspotX, hotspotY
			if len(call.args) >= 4 {
				serial := call.args[0].(uint32)
				surface := call.args[1].(uintptr)
				hotX := call.args[2].(int32)
				hotY := call.args[3].(int32)
				if serial == 456 && surface == 1001 && hotX == 5 && hotY == 6 {
					foundSetCursor = true
				}
			}
		}
	}

	if !foundAttach {
		t.Errorf("attach buffer not found or incorrect in calls: %+v", marshaled)
	}
	if !foundBufferScale {
		t.Errorf("setBufferScale not found or incorrect in calls: %+v", marshaled)
	}
	if !foundDamage {
		t.Errorf("surfaceDamage not found or incorrect in calls: %+v", marshaled)
	}
	if !foundCommit {
		t.Errorf("surfaceCommit not found in calls: %+v", marshaled)
	}
	if !foundSetCursor {
		t.Errorf("pointerSetCursor not found or incorrect in calls: %+v", marshaled)
	}

	// Test destroy
	app.cursorMgr.destroy(api)
	if destroyedTheme != fakeThemePtr {
		t.Fatalf("expected destroyedTheme = %d, got %d", fakeThemePtr, destroyedTheme)
	}
	if app.cursorMgr.cursorSurface != 0 || app.cursorMgr.theme != 0 {
		t.Fatalf("expected cleared cursor surface and theme")
	}
}

func TestWaylandSetCursorShapeSystemIntegration(t *testing.T) {
	app := &waylandApplication{
		windows: []*waylandWindowState{
			{win: &waylandWindow{}},
		},
	}
	activeWaylandApp.Store(app)
	t.Cleanup(func() {
		activeWaylandApp.Store(nil)
	})

	if err := SetCursorShape(2); err != nil {
		t.Fatalf("SetCursorShape(2) failed: %v", err)
	}
	if app.currentCursorShape != 2 {
		t.Fatalf("expected currentCursorShape = 2, got %d", app.currentCursorShape)
	}

	if err := SetCursorShape(255); err == nil {
		t.Fatalf("expected error for invalid cursor shape 255")
	}
}

func goStringFromCString(cStr *byte) string {
	if cStr == nil {
		return ""
	}
	var b []byte
	ptr := cStr
	for *ptr != 0 {
		b = append(b, *ptr)
		ptr = (*byte)(unsafe.Pointer(uintptr(unsafe.Pointer(ptr)) + 1))
	}
	return string(b)
}
