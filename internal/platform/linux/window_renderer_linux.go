package linux

import (
	"errors"
	"os"
	"sync/atomic"

	"github.com/vivaui/viva/internal/diag"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
)

// windowRenderer is what a window draws its scene with: Vulkan when the machine
// has a usable hardware device, the CPU rasterizer otherwise. A window state
// holds this interface, not either implementation.
type windowRenderer interface {
	apply(frame platform.Frame) (bool, error)
	stage(frame platform.Frame) (bool, error)
	present() error
	// presentFull draws and pushes the whole canvas again, after the display
	// server may have discarded the window's pixels.
	presentFull() error
	resize(width, height uint32) error
	setScale(scale float32) error
	setFrameMargin(margin uint32) error
	surfaceSize(width, height uint32) (uint32, uint32)
	refreshText() error
	// forgetSurfaceCapabilities drops surface capabilities read for a configure
	// event that did not resize; it is Vulkan state and empty on the CPU.
	forgetSurfaceCapabilities()
	// alpha reports whether the surface composites with alpha, which decides
	// whether a drawn frame margin can be kept.
	alpha() bool
	frameMargin() uint32
	deviceSize() (uint32, uint32)
	firstRectangle() (render.Rectangle, bool)
	stats() rendererStats
	close()
}

// rendererStats is what the startup profile reports about a window's renderer.
// The three counters are presents, full repaints and partial updates.
type rendererStats struct {
	backend  string
	presents uint64
	rebuilds uint64
	partials uint64
}

// softwareRenderingRequested says the application asked for the CPU renderer,
// as VIVA_SOFTWARE_RENDERING=1 asks for WARP on Windows.
func softwareRenderingRequested() bool { return os.Getenv("VIVA_SOFTWARE_RENDERING") == "1" }

// vulkanRenderingRequested keeps the software Vulkan device that the CPU
// renderer would otherwise replace, for comparing the two paths on one machine.
func vulkanRenderingRequested() bool { return os.Getenv("VIVA_LINUX_RENDERER") == "vulkan" }

// cpuRendererPreferred remembers that this process has no hardware Vulkan
// device, so later windows do not create and discard a Vulkan device to find
// out again.
var cpuRendererPreferred atomic.Bool

// newWindowPainter picks what this window draws with. Hardware Vulkan is the
// fast path and is never replaced. A software Vulkan device draws the whole
// window on the CPU per frame, which is slower than the damage-tracked rasterizer;
// it is used only when asked for explicitly. A machine without a usable Vulkan
// driver falls back to the CPU renderer rather than failing to open the window.
func newWindowPainter(win *window, scale float32, clear platform.Color, rectangles []render.Rectangle, labels []platform.TextLabel) (windowRenderer, error) {
	cpu := func() (windowRenderer, error) {
		return newCPUWindowRenderer(win, scale, clear, rectangles, labels)
	}
	if softwareRenderingRequested() {
		return cpu()
	}
	if cpuRendererPreferred.Load() && !vulkanRenderingRequested() {
		return cpu()
	}
	vulkan, err := newVulkanRenderer(win.connection, win.id, win.width, win.height, scale, clear, rectangles, labels, win.margin)
	if err == nil {
		if !vulkan.software || vulkanRenderingRequested() {
			return vulkan, nil
		}
		vulkan.close()
		cpuRendererPreferred.Store(true)
		renderer, cpuErr := cpu()
		if cpuErr != nil {
			return nil, errors.Join(errors.New("linux: Vulkan has only a software device and the CPU renderer failed"), cpuErr)
		}
		diag.Report(diag.Info, "render", "cpu-renderer", "the window draws on the CPU: Vulkan has only a software device")
		return renderer, nil
	}
	if !errors.Is(err, platform.ErrNoGraphics) {
		return nil, err
	}
	cpuRendererPreferred.Store(true)
	renderer, cpuErr := cpu()
	if cpuErr != nil {
		return nil, errors.Join(err, cpuErr)
	}
	diag.Report(diag.Info, "render", "cpu-fallback", "the window draws on the CPU", diag.String("reason", err.Error()))
	return renderer, nil
}
