package linux

import (
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/platform/vulkan"
)

// Offscreen renders scenes to pixels with the window's own Vulkan pipelines and
// command recording but without an X window, a surface or a swapchain, so a
// screenshot or a pixel test shows what the window would. It is not safe for
// concurrent use.
type Offscreen = vulkan.Offscreen

// NewOffscreen creates a renderer for a width x height scene in logical pixels,
// drawn at scale device pixels per logical pixel, with this desktop's text
// services drawing the glyphs. It fails with platform.ErrNoGraphics when the
// machine has no Vulkan driver.
func NewOffscreen(width, height int, scale float64, clear platform.Color) (*Offscreen, error) {
	return vulkan.NewOffscreen(width, height, scale, clear, newWindowRasterizer)
}
