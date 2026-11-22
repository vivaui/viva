package linux

import (
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/platform/vulkan"
	"github.com/vivaui/viva/internal/render"
)

const (
	vkStructureXcbSurfaceCreateInfo = 1000005000
	vkSuccess                       = 0
)

type vkXcbSurfaceCreateInfo struct {
	SType      uint32
	Padding0   uint32
	Next       uintptr
	Flags      uint32
	Padding1   uint32
	Connection uintptr
	Window     uint32
	Padding2   uint32
}

// xcbSurfaceSource makes the surface of an X window for the Vulkan renderer.
func xcbSurfaceSource(connection uintptr, window uint32) (vulkan.SurfaceSource, error) {
	var create func(uintptr, *vkXcbSurfaceCreateInfo, unsafe.Pointer, *uintptr) int32
	if err := vulkan.Bind(&create, "vkCreateXcbSurfaceKHR"); err != nil {
		return vulkan.SurfaceSource{}, err
	}
	return vulkan.SurfaceSource{
		Extension: "VK_KHR_xcb_surface",
		Create: func(instance uintptr) (uintptr, error) {
			info := vkXcbSurfaceCreateInfo{SType: vkStructureXcbSurfaceCreateInfo, Connection: connection, Window: window}
			var surface uintptr
			if err := vulkan.Check(create(instance, &info, nil, &surface), "create XCB surface"); err != nil {
				return 0, err
			}
			return surface, nil
		},
	}, nil
}

// newVulkanWindow makes the Vulkan renderer of an X window, with the text services
// of this desktop drawing its glyphs.
func newVulkanWindow(win *window, scale float32, clear platform.Color, rectangles []render.Rectangle, labels []platform.TextLabel) (*vulkanWindow, error) {
	source, err := xcbSurfaceSource(win.connection, win.id)
	if err != nil {
		return nil, err
	}
	renderer, err := vulkan.New(vulkan.Config{
		Surface: source, Width: win.width, Height: win.height, Scale: scale, Clear: clear,
		Rectangles: rectangles, Labels: labels, FrameMargin: win.margin, Rasterizer: newWindowRasterizer,
	})
	if err != nil {
		return nil, err
	}
	return &vulkanWindow{renderer}, nil
}

// newWindowRasterizer makes the text services a Vulkan renderer draws glyphs with.
func newWindowRasterizer() (vulkan.TextRasterizer, error) {
	services, err := newTextServices()
	if err != nil {
		return nil, err
	}
	return services, nil
}

// vulkanWindow adapts the Vulkan renderer to the window's seam (windowRenderer).
type vulkanWindow struct{ renderer *vulkan.Renderer }

func (window *vulkanWindow) apply(frame platform.Frame) (bool, error) {
	return window.renderer.Apply(frame)
}
func (window *vulkanWindow) stage(frame platform.Frame) (bool, error) {
	return window.renderer.Stage(frame)
}
func (window *vulkanWindow) present() error     { return window.renderer.Present() }
func (window *vulkanWindow) presentFull() error { return window.renderer.PresentFull() }
func (window *vulkanWindow) resize(width, height uint32) error {
	return window.renderer.Resize(width, height)
}
func (window *vulkanWindow) setScale(scale float32) error { return window.renderer.SetScale(scale) }
func (window *vulkanWindow) setFrameMargin(margin uint32) error {
	return window.renderer.SetFrameMargin(margin)
}
func (window *vulkanWindow) surfaceSize(width, height uint32) (uint32, uint32) {
	return window.renderer.SurfaceSize(width, height)
}
func (window *vulkanWindow) refreshText() error         { return window.renderer.RefreshText() }
func (window *vulkanWindow) forgetSurfaceCapabilities() { window.renderer.ForgetSurfaceCapabilities() }
func (window *vulkanWindow) alpha() bool                { return window.renderer.Alpha() }
func (window *vulkanWindow) frameMargin() uint32        { return window.renderer.FrameMargin() }
func (window *vulkanWindow) deviceSize() (uint32, uint32) {
	return window.renderer.DeviceSize()
}
func (window *vulkanWindow) firstRectangle() (render.Rectangle, bool) {
	return window.renderer.FirstRectangle()
}
func (window *vulkanWindow) stats() rendererStats {
	stats := window.renderer.Stats()
	return rendererStats{backend: "vulkan", presents: stats.Presents, rebuilds: stats.Rebuilds, partials: stats.Partials}
}
func (window *vulkanWindow) close() { window.renderer.Close() }

// software reports whether the device is a CPU rasterizer (lavapipe).
func (window *vulkanWindow) software() bool { return window.renderer.Software() }
