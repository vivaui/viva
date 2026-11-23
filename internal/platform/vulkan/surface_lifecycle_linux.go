package vulkan

import "fmt"

// A window system can take a surface back while the application lives on: Android
// destroys the surface of a window that goes behind another and gives it a new one
// when it returns. ReleaseSurface and ReplaceSurface keep the device, the scene and
// the label textures through that; only what hangs on the surface is made again.

// ReleaseSurface lets go of the swapchain and the surface, and everything made for
// the swapchain's images, before the window system takes the surface away. The device
// is kept, so is what the renderer would draw. Nothing is drawn until ReplaceSurface.
func (renderer *Renderer) ReleaseSurface() {
	if renderer == nil || renderer.api == nil || renderer.device == 0 {
		return
	}
	renderer.api.deviceWaitIdle(renderer.device)
	renderer.destroySwapchain()
	if renderer.surface != 0 && renderer.instance != 0 {
		renderer.api.destroySurface(renderer.instance, renderer.surface, nil)
	}
	renderer.surface = 0
	renderer.hasSurfaceFormat = false
	renderer.hasPresentMode = false
	renderer.hasPendingCapabilities = false
	renderer.recorded = nil
}

// ReplaceSurface makes the surface again from the window system's current one, which
// is width x height device pixels, and a swapchain on it. The scene and the label
// textures the renderer kept are drawn on it; the application should apply a whole
// frame next, as the one it applied last may have been before the surface went.
func (renderer *Renderer) ReplaceSurface(width, height uint32) error {
	for losses := 0; ; losses++ {
		retry, err := renderer.recoverFrom(renderer.replaceSurfaceOnce(width, height), losses)
		if !retry {
			return err
		}
	}
}

func (renderer *Renderer) replaceSurfaceOnce(width, height uint32) error {
	if renderer.source.Create == nil || renderer.shared == nil {
		return fmt.Errorf("vulkan: only a window renderer has a surface to replace")
	}
	renderer.ReleaseSurface()
	surface, err := renderer.source.Create(renderer.instance)
	if err != nil {
		return err
	}
	renderer.surface = surface
	if err := renderer.shared.attach(surface); err != nil {
		return err
	}
	renderer.width, renderer.height = width, height
	return renderer.createSwapchain()
}
