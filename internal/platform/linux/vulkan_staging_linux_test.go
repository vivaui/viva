package linux

import (
	"fmt"
	"testing"

	"github.com/vivaui/viva/internal/platform"
)

// Labels uploaded frame after frame go through the staging chunks the renderer
// keeps: no upload waits for the whole device, a page larger than a chunk takes
// more and the next upload merges them back into one, and what is drawn is the
// text of the latest frame.
func TestTextUploadsReuseStagingWithoutIdlingTheDevice(t *testing.T) {
	offscreen := newTestOffscreen(t, 400, 300)
	renderer := offscreen.renderer
	idles := 0
	idle := renderer.api.deviceWaitIdle
	renderer.api.deviceWaitIdle = func(device uintptr) int32 {
		idles++
		return idle(device)
	}
	page := func(round, rows int) []platform.TextLabel {
		labels := make([]platform.TextLabel, rows)
		for index := range labels {
			labels[index] = platform.TextLabel{
				Value: fmt.Sprintf("round %d row %d of a long line of text", round, index),
				X:     4, Y: float32(index%12) * 24, Width: 390, Height: 22, FontSize: 15,
				Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
			}
		}
		return labels
	}
	if err := offscreen.Set(nil, page(0, 4)); err != nil {
		t.Skipf("text is not available here: %v", err)
	}
	// Enough rows that one upload outgrows the first chunk.
	big := page(1, 600)
	if err := offscreen.Set(nil, big); err != nil {
		t.Fatal(err)
	}
	if len(renderer.text.staging.chunks) < 2 {
		t.Skipf("the page fit one chunk (%d); the test needs a larger one", len(renderer.text.staging.chunks))
	}
	if err := offscreen.Set(nil, page(2, 12)); err != nil {
		t.Fatal(err)
	}
	if len(renderer.text.staging.chunks) != 1 {
		t.Fatalf("the next upload kept %d chunks, want them merged into one", len(renderer.text.staging.chunks))
	}
	if idles != 0 {
		t.Fatalf("uploads waited for the device to idle %d times", idles)
	}
	pixels, err := offscreen.Image()
	if err != nil {
		t.Fatal(err)
	}
	painted := 0
	for y := 0; y < 24; y++ {
		for x := 0; x < 400; x++ {
			if isPainted(pixels, x, y) {
				painted++
			}
		}
	}
	if painted < 30 {
		t.Fatal("the latest frame's first row drew no glyphs")
	}
}
