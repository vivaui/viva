package linux

import "github.com/vivaui/viva/internal/render"

// The windowRenderer methods that only adapt *vulkanRenderer to the window's
// seam. Everything the window actually does (apply, present, resize, scale,
// frame margin) lives with the Vulkan implementation.

// presentFull presents the retained swapchain image again. Vulkan keeps the
// frame it last presented, so this is the same as present.
func (renderer *vulkanRenderer) presentFull() error { return renderer.present() }

func (renderer *vulkanRenderer) forgetSurfaceCapabilities() { renderer.hasPendingCapabilities = false }

func (renderer *vulkanRenderer) alpha() bool { return renderer.frame.alpha }

func (renderer *vulkanRenderer) frameMargin() uint32 { return renderer.frame.margin }

func (renderer *vulkanRenderer) deviceSize() (uint32, uint32) { return renderer.width, renderer.height }

func (renderer *vulkanRenderer) firstRectangle() (render.Rectangle, bool) {
	if len(renderer.rectangles) == 0 {
		return render.Rectangle{}, false
	}
	return renderer.rectangles[0], true
}

func (renderer *vulkanRenderer) stats() rendererStats {
	return rendererStats{backend: "vulkan", presents: renderer.submissions, rebuilds: renderer.rebuilds, partials: renderer.partials}
}
