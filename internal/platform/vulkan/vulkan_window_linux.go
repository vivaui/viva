package vulkan

import "github.com/vivaui/viva/internal/render"

// Stats is what the startup profile reports about a renderer: the presents it
// made, the full rebuilds of its scene, the partial updates and the label
// textures it made.
type Stats struct {
	Presents, Rebuilds, Partials, Textures uint64
}

// PresentFull presents the retained swapchain image again. Vulkan keeps the
// frame it last presented, so this is the same as Present.
func (renderer *Renderer) PresentFull() error { return renderer.Present() }

// ForgetSurfaceCapabilities drops surface capabilities read for a configure event
// that did not resize.
func (renderer *Renderer) ForgetSurfaceCapabilities() { renderer.hasPendingCapabilities = false }

// Alpha reports whether the surface composites with alpha.
func (renderer *Renderer) Alpha() bool { return renderer.frame.alpha }

// FrameMargin is the margin of the window frame the renderer draws, 0 for none.
func (renderer *Renderer) FrameMargin() uint32 { return renderer.frame.margin }

// DeviceSize is the surface's size in device pixels.
func (renderer *Renderer) DeviceSize() (uint32, uint32) { return renderer.width, renderer.height }

// FirstRectangle is the first shape of the scene, if there is one.
func (renderer *Renderer) FirstRectangle() (render.Rectangle, bool) {
	if len(renderer.rectangles) == 0 {
		return render.Rectangle{}, false
	}
	return renderer.rectangles[0], true
}

// Stats reports the renderer's counters.
func (renderer *Renderer) Stats() Stats {
	return Stats{Presents: renderer.submissions, Rebuilds: renderer.rebuilds, Partials: renderer.partials, Textures: renderer.textures}
}

// Software reports whether the device the renderer draws with is a CPU rasterizer,
// such as Mesa's lavapipe.
func (renderer *Renderer) Software() bool { return renderer.software }
