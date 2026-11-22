//go:build linux

package vulkan

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// A line wider than a texture holds gets a texture of the window of it that the viewport and
// its clip show, with or without runs. A scroll inside that window keeps the texture and one
// past it rasterizes the line again; a line scrolled out of view, or too tall for any texture,
// is dropped and not rasterized again every frame for that.
func TestVulkanWideLabelWindowFollowsTheViewport(t *testing.T) {
	const scale, viewport = 1.5, 600
	for _, runs := range [][]gfx.TextRun{nil, wholeRun(wideValue)} {
		label := vulkanLabelTexture{label: wideMonoLine(wideValue, 0, runs)}
		if skip, width, height, wide, ok := label.window(scale, viewport); !ok || !wide || skip != 0 || width != 1800 || height != 27 {
			t.Errorf("runs %d, start of the line: window = %d, %d, %d, %v, %v; want 0, 1800, 27, wide", len(runs), skip, width, height, wide, ok)
		}
		label.label.X = -20000 // 30000 device pixels scrolled off the left
		if skip, width, _, _, ok := label.window(scale, viewport); !ok || skip != 30000-600 || width != 1800 {
			t.Errorf("runs %d, scrolled: window = %d, %d, %v; want 29400, 1800", len(runs), skip, width, ok)
		}
		if !label.needsUpload(scale, viewport) {
			t.Errorf("runs %d: a wide line without a texture is not rasterized", len(runs))
		}
		label.view, label.scale, label.skip, label.width, label.height, label.wide = 1, scale, 29400, 1800, 27, true
		label.label.X = -20100
		if label.needsUpload(scale, viewport) {
			t.Errorf("runs %d: a scroll inside the texture's window rasterizes the line again", len(runs))
		}
		label.label.X = -21000
		if !label.needsUpload(scale, viewport) {
			t.Errorf("runs %d: a scroll past the texture's window keeps the texture", len(runs))
		}
		label.label.X = -40000
		if !label.needsUpload(scale, viewport) {
			t.Errorf("runs %d: a line scrolled out of view keeps its texture", len(runs))
		}
		if _, _, _, wide, ok := label.window(scale, viewport); ok || !wide {
			t.Errorf("runs %d: a line scrolled out of view has a window", len(runs))
		}
		label.drop(nil, scale, true)
		if label.needsUpload(scale, viewport) {
			t.Errorf("runs %d: a dropped line out of view is rasterized again", len(runs))
		}
		label.label.X = -20000
		if !label.needsUpload(scale, viewport) {
			t.Errorf("runs %d: a dropped line scrolled back into view is not rasterized", len(runs))
		}
		label.label.Height = 12000 // 18000 device pixels
		if _, _, _, _, ok := label.window(scale, viewport); ok {
			t.Errorf("runs %d: a line too tall for any texture has a window", len(runs))
		}
		label.drop(nil, scale, false)
		if label.needsUpload(scale, viewport) {
			t.Errorf("runs %d: a dropped line too tall for any texture is rasterized again", len(runs))
		}
	}
}
