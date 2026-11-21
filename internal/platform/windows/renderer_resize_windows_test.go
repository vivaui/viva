package windows

import "testing"

// swapChainResize decides when the swap chain is rebuilt. A modal drag may keep
// a larger chain than the client and grow it in 64-pixel steps; every other
// present gets a chain of the exact client size, so a completed drag shrinks
// the chain back instead of leaving it skewed by the resize history.
func TestSwapChainResizeKeepsHeadroomOnlyDuringADrag(t *testing.T) {
	cases := []struct {
		name                  string
		bufferWidth           int
		bufferHeight          int
		width                 int
		height                int
		hasTarget             bool
		headroom              bool
		wantResize            bool
		wantWidth, wantHeight int
	}{
		{"no chain yet", 0, 0, 800, 600, false, false, true, 800, 600},
		{"first drag frame takes headroom", 0, 0, 800, 600, false, true, true, 832, 640},
		{"drag reuses a larger chain", 832, 640, 700, 500, true, true, false, 832, 640},
		{"drag grows past the chain", 832, 640, 900, 600, true, true, true, 960, 640},
		{"drag exit shrinks to the client", 832, 640, 800, 600, true, false, true, 800, 600},
		{"no drag, same size", 800, 600, 800, 600, true, false, false, 800, 600},
		{"no drag grows exactly", 800, 600, 900, 700, true, false, true, 900, 700},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resize, width, height := swapChainResize(c.bufferWidth, c.bufferHeight, c.width, c.height, c.hasTarget, c.headroom)
			if resize != c.wantResize || width != c.wantWidth || height != c.wantHeight {
				t.Fatalf("resize=%t size=%dx%d, want resize=%t size=%dx%d", resize, width, height, c.wantResize, c.wantWidth, c.wantHeight)
			}
		})
	}
}
