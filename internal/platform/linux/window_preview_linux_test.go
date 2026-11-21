package linux

import (
	"errors"
	"os"
	"testing"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

func TestPreviewMovePreservesSizeAndFocus(t *testing.T) {
	var mask uint16
	var values [2]uint32
	win := &window{connection: 1, id: 2, preview: true, width: 180, height: 90}
	win.storeScale(1.5)
	win.api = &xcbAPI{
		configureWindow: func(_ uintptr, _ uint32, m uint16, p *uint32) xcbVoidCookie {
			mask = m
			copy(values[:], unsafe.Slice(p, 2))
			return xcbVoidCookie{}
		},
		flush: func(uintptr) int32 { return 1 },
	}
	native := &nativeWindow{state: &linuxWindowState{win: win}}
	native.MoveTo(gfx.Point{X: -20, Y: 30})
	if mask != 3 || int32(values[0]) != -30 || values[1] != 45 || win.width != 180 || win.height != 90 {
		t.Fatalf("move mask=%d position=%v size=%dx%d", mask, values, win.width, win.height)
	}
	native.closed.Store(true)
	mask = 0
	native.MoveTo(gfx.Point{})
	if mask != 0 {
		t.Fatal("closed preview was moved")
	}
}

// Run only on a private Xvfb server. Claiming the compositor selection exercises
// the X11 protocol without taking over a real desktop's compositor ownership.
func TestPreviewNativeX11Contract(t *testing.T) {
	if os.Getenv("VIVA_TEST_PREVIEW_XVFB") != "1" {
		t.Skip("requires a private Xvfb server")
	}
	owner, err := openWindow(platform.Config{Width: 100, Height: 100})
	if err != nil {
		t.Fatal(err)
	}
	defer owner.close()
	var setFocus func(uintptr, uint8, uint32, uint32) xcbVoidCookie
	var getFocus func(uintptr) xcbVoidCookie
	var focusReply func(uintptr, xcbVoidCookie, *unsafe.Pointer) unsafe.Pointer
	purego.RegisterLibFunc(&setFocus, owner.api.library, "xcb_set_input_focus")
	purego.RegisterLibFunc(&getFocus, owner.api.library, "xcb_get_input_focus")
	purego.RegisterLibFunc(&focusReply, owner.api.library, "xcb_get_input_focus_reply")
	setFocus(owner.connection, 0, owner.id, 0)
	checkFocus := func() {
		t.Helper()
		result := focusReply(owner.connection, getFocus(owner.connection), nil)
		if result == nil {
			t.Fatal("missing keyboard focus reply")
		}
		focus := *(*uint32)(unsafe.Add(result, 8))
		owner.api.free(result)
		if focus != owner.id {
			t.Fatalf("keyboard focus=%d, want owner %d", focus, owner.id)
		}
	}
	checkFocus()
	config := platform.Config{Width: 80, Height: 40, WindowMode: platform.WindowPreview, HasOrigin: true, Origin: gfx.Point{X: 20, Y: 30}}
	if unavailable, err := openWindow(config); !errors.Is(err, platform.ErrDragPreviewUnsupported) {
		if unavailable != nil {
			unavailable.close()
		}
		t.Fatalf("no compositor: %v", err)
	}
	atom, err := owner.api.atom(owner.connection, "_NET_WM_CM_S0")
	if err != nil {
		t.Fatal(err)
	}
	owner.api.setSelectionOwner(owner.connection, owner.id, atom, 0)
	owner.api.flush(owner.connection)
	if !owner.compositing(0) {
		t.Fatal("compositor selection was not claimed")
	}
	win, err := openWindow(config)
	if err != nil {
		t.Fatal(err)
	}
	defer win.close()
	if win.depth != 32 || win.margin != 0 {
		t.Fatalf("preview depth=%d margin=%d", win.depth, win.margin)
	}
	var attributes func(uintptr, uint32) xcbVoidCookie
	var reply func(uintptr, xcbVoidCookie, *unsafe.Pointer) unsafe.Pointer
	purego.RegisterLibFunc(&attributes, win.api.library, "xcb_get_window_attributes")
	purego.RegisterLibFunc(&reply, win.api.library, "xcb_get_window_attributes_reply")
	result := reply(win.connection, attributes(win.connection, win.id), nil)
	if result == nil {
		t.Fatal("missing window attributes")
	}
	data := unsafe.Slice((*byte)(result), 44)
	override := data[27]
	win.api.free(result)
	if override != 1 {
		t.Fatalf("preview override_redirect=%d", override)
	}
	// A resize/configure notification must not restore a normal input region.
	win.updateInputRegion()
	var rectangles func(uintptr, uint32, uint8) xcbVoidCookie
	purego.RegisterLibFunc(&rectangles, win.shapeLibrary, "xcb_shape_get_rectangles")
	purego.RegisterLibFunc(&reply, win.shapeLibrary, "xcb_shape_get_rectangles_reply")
	result = reply(win.connection, rectangles(win.connection, win.id, shapeInput), nil)
	if result == nil {
		t.Fatal("missing input shape")
	}
	count := *(*uint32)(unsafe.Add(result, 8))
	win.api.free(result)
	if count != 0 {
		t.Fatalf("preview intercepts input: %d rectangles", count)
	}
	renderer, err := newWindowPainter(win, win.displayScale(), platform.Color{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer renderer.close()
	if !renderer.alpha() {
		t.Fatal("preview cannot present alpha")
	}
	if err := renderer.present(); err != nil {
		t.Fatal(err)
	}
	cpu := renderer.(*cpuWindowRenderer)
	if cpu.scene.canvas.Pix[3] != 0 {
		t.Fatal("transparent preview corner is opaque")
	}
	control := &nativeWindow{state: &linuxWindowState{win: win}}
	control.MoveTo(gfx.Point{X: 40, Y: 50})
	x, y := win.rootOrigin()
	if x != 40 || y != 50 {
		t.Fatalf("origin=%d,%d", x, y)
	}
	checkFocus()
}
