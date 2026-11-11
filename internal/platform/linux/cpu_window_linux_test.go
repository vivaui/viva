package linux

import (
	"bytes"
	"encoding/binary"
	"testing"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/raster"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

type putCall struct {
	width, height uint16
	x, y          int16
	depth         uint8
	data          []byte
}

// fakePutImageAPI records what a CPU renderer sends. capture off leaves the
// pixels alone, for allocation measurements.
func fakePutImageAPI(calls *[]putCall, capture bool) *xcbAPI {
	return &xcbAPI{
		generateID: func(uintptr) uint32 { return 1 },
		createGC:   func(uintptr, uint32, uint32, uint32, *uint32) xcbVoidCookie { return xcbVoidCookie{} },
		freeGC:     func(uintptr, uint32) xcbVoidCookie { return xcbVoidCookie{} },
		flush:      func(uintptr) int32 { return 0 },
		requestCheck: func(uintptr, xcbVoidCookie) *xcbGenericError {
			return nil
		},
		putImage: func(_ uintptr, _ uint8, _ uint32, _ uint32, width, height uint16, x, y int16, _ uint8, depth uint8, length uint32, data *byte) xcbVoidCookie {
			call := putCall{width: width, height: height, x: x, y: y, depth: depth}
			if capture {
				call.data = append([]byte(nil), unsafe.Slice(data, int(length))...)
				*calls = append(*calls, call)
			}
			return xcbVoidCookie{}
		},
	}
}

func testCPUWindow(canvas *raster.Canvas, calls *[]putCall, capture bool) *cpuWindowRenderer {
	return &cpuWindowRenderer{
		scene: &softScene{canvas: canvas, scale: 1, clear: scene.Color{Alpha: 1}},
		win:   &window{api: fakePutImageAPI(calls, capture), connection: 1, id: 2},
		gc:    3,
		depth: 24,
	}
}

func patternedCanvas(width, height int) *raster.Canvas {
	canvas := raster.NewCanvas(width, height)
	for i := range canvas.Pix {
		canvas.Pix[i] = byte(i * 7)
	}
	return canvas
}

// A damaged rectangle narrower than the canvas is packed into the scratch
// buffer row by row, and what the server receives is exactly those canvas
// pixels at the rectangle's place.
func TestCPUWindowPushPacksTheDamagedRectangle(t *testing.T) {
	canvas := patternedCanvas(8, 6)
	var calls []putCall
	renderer := testCPUWindow(canvas, &calls, true)
	damage := render.Scissor{Left: 2, Top: 1, Right: 6, Bottom: 4}

	if err := renderer.push(damage); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 {
		t.Fatalf("%d requests, want 1", len(calls))
	}
	call := calls[0]
	if call.width != 4 || call.height != 3 || call.x != 2 || call.y != 1 || call.depth != 24 {
		t.Fatalf("request %dx%d at %d,%d depth %d", call.width, call.height, call.x, call.y, call.depth)
	}
	want := make([]byte, 4*4*3)
	for row := range 3 {
		from := (1+row)*canvas.Stride + 2*4
		copy(want[row*16:(row+1)*16], canvas.Pix[from:from+16])
	}
	if !bytes.Equal(call.data, want) {
		t.Fatalf("sent pixels differ from the canvas: %v", call.data)
	}
	if cap(renderer.scratch) == 0 {
		t.Fatal("a narrow rectangle was not packed")
	}
}

// A rectangle as wide as the canvas is sent straight from the canvas rows,
// without the scratch copy.
func TestCPUWindowPushSendsFullWidthRowsDirectly(t *testing.T) {
	canvas := patternedCanvas(8, 6)
	var calls []putCall
	renderer := testCPUWindow(canvas, &calls, true)
	damage := render.Scissor{Left: 0, Top: 2, Right: 8, Bottom: 5}

	if err := renderer.push(damage); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 {
		t.Fatalf("%d requests, want 1", len(calls))
	}
	if renderer.scratch != nil {
		t.Fatal("a full-width rectangle was copied")
	}
	want := canvas.Pix[2*canvas.Stride : 5*canvas.Stride]
	if !bytes.Equal(calls[0].data, want) {
		t.Fatal("sent pixels differ from the canvas rows")
	}
}

// present paints the pending damage, sends it, and leaves nothing pending; a
// second present with no new frame sends nothing.
func TestCPUWindowPresentPaintsAndClearsDamage(t *testing.T) {
	canvas := raster.NewCanvas(8, 6)
	var calls []putCall
	renderer := testCPUWindow(canvas, &calls, true)
	renderer.scene.rects = []render.Rectangle{{
		Kind:   scene.KindRectangle,
		Bounds: scene.Rect{Width: 8, Height: 6},
		Color:  scene.Color{Red: 1, Alpha: 1},
	}}
	renderer.scene.damageAll()

	if err := renderer.present(); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 {
		t.Fatalf("%d requests, want 1", len(calls))
	}
	if call := calls[0]; call.width != 8 || call.height != 6 {
		t.Fatalf("sent %dx%d, want the whole 8x6 canvas", call.width, call.height)
	}
	// The scene's red reached the server: BGRA order.
	if pixel := calls[0].data[:4]; pixel[0] != 0 || pixel[1] != 0 || pixel[2] != 255 {
		t.Fatalf("first pixel %v, want red", pixel)
	}
	if renderer.scene.pending() {
		t.Fatal("damage remains after present")
	}
	if err := renderer.present(); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 {
		t.Fatalf("a present with no damage sent %d requests", len(calls)-1)
	}
	if renderer.rebuilds != 1 || renderer.partials != 0 {
		t.Fatalf("counters rebuilds=%d partials=%d", renderer.rebuilds, renderer.partials)
	}
}

// presentFull paints and sends the whole canvas even when no damage is
// pending, as after the display server discarded the window's pixels.
func TestCPUWindowPresentFullSendsEverything(t *testing.T) {
	canvas := raster.NewCanvas(8, 6)
	var calls []putCall
	renderer := testCPUWindow(canvas, &calls, true)
	renderer.scene.rects = []render.Rectangle{{
		Kind:   scene.KindRectangle,
		Bounds: scene.Rect{Width: 8, Height: 6},
		Color:  scene.Color{Green: 1, Alpha: 1},
	}}
	renderer.scene.damageAll()
	if err := renderer.present(); err != nil {
		t.Fatal(err)
	}

	if err := renderer.presentFull(); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 {
		t.Fatalf("%d requests, want 2", len(calls))
	}
	if call := calls[1]; call.width != 8 || call.height != 6 {
		t.Fatalf("presentFull sent %dx%d", call.width, call.height)
	}
}

// A steady window that redraws the same scene and presents it allocates
// nothing: the scene, the scratch buffer and the rasterizer are retained.
func TestCPUWindowSteadyPresentAllocatesNothing(t *testing.T) {
	canvas := raster.NewCanvas(64, 48)
	var calls []putCall
	renderer := testCPUWindow(canvas, &calls, false)
	rects := []render.Rectangle{{Kind: scene.KindRectangle, Bounds: scene.Rect{Width: 64, Height: 48}, Color: scene.Color{Red: 0.2, Green: 0.3, Blue: 0.4, Alpha: 1}}}
	frame := platform.Frame{Update: render.Update{Count: 1, Rebuild: true}, Rectangles: rects}
	steady := func() {
		if _, err := renderer.apply(frame); err != nil {
			t.Fatal(err)
		}
		if err := renderer.present(); err != nil {
			t.Fatal(err)
		}
	}
	steady()
	// The first present may grow the rasterizer's scratch lists; measure after it.
	if allocs := testing.AllocsPerRun(50, steady); allocs != 0 {
		t.Fatalf("%g allocations per steady present, want 0", allocs)
	}
}

// BenchmarkCPUWindowPresent measures the presenter around the rasterizer: a
// whole-window repaint, and the band a text field redraws as the user types.
// PutImage is a no-op here, so the numbers cover the scene paint and the copy
// into the request, not the X server.
func BenchmarkCPUWindowPresent(b *testing.B) {
	canvas := raster.NewCanvas(1280, 800)
	var calls []putCall
	renderer := testCPUWindow(canvas, &calls, false)
	renderer.scene.rects = []render.Rectangle{
		{Kind: scene.KindRectangle, Bounds: scene.Rect{Width: 1280, Height: 800}, Color: scene.Color{Red: 0.97, Green: 0.97, Blue: 0.98, Alpha: 1}},
		{Kind: scene.KindRectangle, Bounds: scene.Rect{Width: 220, Height: 800}, Color: scene.Color{Red: 0.93, Green: 0.94, Blue: 0.96, Alpha: 1}},
	}
	for i := range 24 {
		renderer.scene.rects = append(renderer.scene.rects, render.Rectangle{
			Kind: scene.KindRectangle, Bounds: scene.Rect{X: 12, Y: float32(40 + i*28), Width: 196, Height: 24},
			Color: scene.Color{Red: 0.1, Green: 0.25, Blue: 0.9, Alpha: 0.12}, Radius: 6,
		})
	}
	b.Run("full", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(1280 * 800 * 4)
		for range b.N {
			renderer.scene.damageAll()
			if err := renderer.present(); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("typing", func(b *testing.B) {
		damage := render.Scissor{Left: 260, Top: 300, Right: 460, Bottom: 332}
		b.ReportAllocs()
		b.SetBytes(int64((damage.Right - damage.Left) * (damage.Bottom - damage.Top) * 4))
		for range b.N {
			renderer.scene.damage = damage
			if err := renderer.present(); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// fakeVulkanAPI answers just enough for selectDevice: the given physical
// devices in order and one graphics family that presents everywhere.
func fakeVulkanAPI(devices []uintptr, kinds map[uintptr]uint32) *vulkanAPI {
	return &vulkanAPI{
		enumeratePhysicalDevices: func(_ uintptr, count *uint32, list *uintptr) int32 {
			*count = uint32(len(devices))
			if list != nil {
				copy(unsafe.Slice(list, len(devices)), devices)
			}
			return vkSuccess
		},
		getQueueFamilyProperties: func(_ uintptr, count *uint32, families *vkQueueFamilyProperties) {
			*count = 1
			if families != nil {
				*families = vkQueueFamilyProperties{Flags: vkQueueGraphics, Count: 1}
			}
		},
		getSurfaceSupport: func(_ uintptr, _ uint32, _ uintptr, supported *uint32) int32 {
			*supported = 1
			return vkSuccess
		},
		getPhysicalDeviceProperties: func(device uintptr, properties *[vkPropertiesSize]byte) {
			binary.LittleEndian.PutUint32(properties[vkPropertiesDeviceTypeOffset:], kinds[device])
		},
	}
}

// A software Vulkan device is never preferred over a hardware one, and a machine
// whose only device is software is reported as such, so the CPU renderer can
// take over.
func TestSelectDevicePrefersHardwareAndReportsSoftware(t *testing.T) {
	hardware := uint32(2) // VK_PHYSICAL_DEVICE_TYPE_DISCRETE_GPU
	for _, test := range []struct {
		name     string
		devices  []uintptr
		kinds    map[uintptr]uint32
		physical uintptr
		software bool
	}{
		{"software first", []uintptr{11, 22}, map[uintptr]uint32{11: vkPhysicalDeviceTypeCPU, 22: hardware}, 22, false},
		{"hardware first", []uintptr{22, 11}, map[uintptr]uint32{22: hardware, 11: vkPhysicalDeviceTypeCPU}, 22, false},
		{"only software", []uintptr{11}, map[uintptr]uint32{11: vkPhysicalDeviceTypeCPU}, 11, true},
	} {
		renderer := &vulkanRenderer{api: fakeVulkanAPI(test.devices, test.kinds), instance: 1, surface: 2}
		if err := renderer.selectDevice(); err != nil {
			t.Fatalf("%s: %v", test.name, err)
		}
		if renderer.physical != test.physical || renderer.software != test.software {
			t.Errorf("%s: device %d software=%v, want %d software=%v", test.name, renderer.physical, renderer.software, test.physical, test.software)
		}
	}
}
