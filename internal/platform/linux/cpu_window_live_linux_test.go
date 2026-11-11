package linux

import (
	"os"
	"testing"
	"unsafe"

	"github.com/ebitengine/purego"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

type xcbGetImageCookie struct{ Sequence uint32 }

type xcbGetImageReply struct {
	ResponseType uint8
	Depth        uint8
	Sequence     uint16
	Length       uint32
	Visual       uint32
	Pad          [20]byte
}

// readWindowPixels reads a rectangle of a window through the X server, as
// another client sees it.
func readWindowPixels(t *testing.T, win *window, x, y int16, width, height uint16) []byte {
	t.Helper()
	var getImage func(uintptr, uint8, uint32, int16, int16, uint16, uint16, uint32) xcbGetImageCookie
	purego.RegisterLibFunc(&getImage, win.api.library, "xcb_get_image")
	var getImageReply func(uintptr, xcbGetImageCookie, *unsafe.Pointer) *xcbGetImageReply
	purego.RegisterLibFunc(&getImageReply, win.api.library, "xcb_get_image_reply")
	var imageData func(*xcbGetImageReply) *byte
	purego.RegisterLibFunc(&imageData, win.api.library, "xcb_get_image_data")
	var dataLength func(*xcbGetImageReply) int32
	purego.RegisterLibFunc(&dataLength, win.api.library, "xcb_get_image_data_length")

	cookie := getImage(win.connection, xcbImageFormatZPixmap, win.id, x, y, width, height, ^uint32(0))
	var nativeError unsafe.Pointer
	reply := getImageReply(win.connection, cookie, &nativeError)
	if nativeError != nil {
		win.api.free(nativeError)
	}
	if reply == nil {
		t.Fatal("the X server returned no image")
	}
	defer win.api.free(unsafe.Pointer(reply))
	if reply.Depth != win.depth {
		t.Fatalf("image depth %d, window depth %d", reply.Depth, win.depth)
	}
	length := int(dataLength(reply))
	if want := int(width) * int(height) * 4; length < want {
		t.Fatalf("image has %d bytes, want at least %d", length, want)
	}
	return append([]byte(nil), unsafe.Slice(imageData(reply), length)...)
}

// With the CPU renderer forced, a window still opens and shows what the scene
// paints: the pixels read back from the server are the scene's colors, both in
// a partial update and outside it.
func TestSoftwareRendererOpensAWindow(t *testing.T) {
	if testing.Short() || os.Getenv("VIVA_LINUX_XCB_SMOKE") == "" {
		t.Skip("set VIVA_LINUX_XCB_SMOKE=1 with an X11 display")
	}
	t.Setenv("VIVA_SOFTWARE_RENDERING", "1")
	application := &linuxApplication{}
	blue := scene.Color{Blue: 1, Alpha: 1}
	control, err := application.OpenWindow(platform.Config{
		Title:      "Viva software smoke",
		Width:      64,
		Height:     48,
		WindowMode: platform.WindowTitled,
		Rectangles: []render.Rectangle{{Kind: scene.KindRectangle, Bounds: scene.Rect{Width: 64, Height: 48}, Color: blue}},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(application.closeAll)
	state := control.(*nativeWindow).state
	if _, ok := state.renderer.(*cpuWindowRenderer); !ok {
		t.Fatalf("renderer is %T, want the CPU renderer", state.renderer)
	}
	red := scene.Color{Red: 1, Alpha: 1}
	rectangles := []render.Rectangle{
		{Kind: scene.KindRectangle, Bounds: scene.Rect{Width: 64, Height: 48}, Color: blue},
		{Kind: scene.KindRectangle, Bounds: scene.Rect{X: 16, Y: 12, Width: 16, Height: 12}, Color: red},
	}
	if _, err := state.renderer.apply(platform.Frame{Update: render.Update{Count: len(rectangles), Rebuild: true}, Rectangles: rectangles}); err != nil {
		t.Fatal(err)
	}
	if err := state.renderer.present(); err != nil {
		t.Fatal(err)
	}
	// GetImage is the same connection's request after PutImage: the server
	// applies them in order.
	inside := readWindowPixels(t, state.win, 24, 18, 1, 1)
	if inside[0] != 0 || inside[1] != 0 || inside[2] != 255 {
		t.Fatalf("pixel inside the red rectangle is %v, want red", inside[:4])
	}
	outside := readWindowPixels(t, state.win, 2, 2, 1, 1)
	if outside[0] != 255 || outside[1] != 0 || outside[2] != 0 {
		t.Fatalf("pixel outside the red rectangle is %v, want blue", outside[:4])
	}
}

// A Vulkan device that is a CPU rasterizer (lavapipe) is not used for windows:
// the CPU renderer draws the same scene with damage tracking. A machine with a
// hardware Vulkan device is the fast path and is left alone.
func TestSoftwareVulkanDeviceDrawsOnTheCPU(t *testing.T) {
	if testing.Short() || os.Getenv("VIVA_LINUX_XCB_SMOKE") == "" {
		t.Skip("set VIVA_LINUX_XCB_SMOKE=1 with an X11 display")
	}
	t.Setenv("VIVA_SOFTWARE_RENDERING", "")
	t.Setenv("VIVA_LINUX_RENDERER", "")
	application := &linuxApplication{}
	control, err := application.OpenWindow(platform.Config{Title: "Viva device choice", Width: 64, Height: 48, WindowMode: platform.WindowTitled})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(application.closeAll)
	state := control.(*nativeWindow).state
	if !vulkanDeviceIsSoftware() {
		t.Skip("this machine has a hardware Vulkan device")
	}
	if _, ok := state.renderer.(*cpuWindowRenderer); !ok {
		t.Fatalf("no hardware Vulkan device produced %T, want the CPU renderer", state.renderer)
	}
}
