package linux

import (
	"fmt"
	"testing"
	"unsafe"

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
	allocations := 0
	allocate := renderer.api.allocateMemory
	renderer.api.allocateMemory = func(device uintptr, info *vkMemoryAllocateInfo, callbacks unsafe.Pointer, memory *uintptr) int32 {
		allocations++
		return allocate(device, info, callbacks, memory)
	}
	if err := offscreen.Set(nil, page(2, 12)); err != nil {
		t.Fatal(err)
	}
	// Twelve new images and the merged staging chunk: the images come out of the
	// blocks the big page made, only the chunk is a driver allocation.
	if allocations > 1 {
		t.Fatalf("replacing twelve labels made %d driver allocations, want the images from the pool", allocations)
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

// Freed image memory goes back to its block and joins its free neighbours, so a
// block whose images are all gone is one free span again.
func TestImageBlockGivesBackAndJoinsSpans(t *testing.T) {
	block := &imageBlock{size: 1024, free: []memorySpan{{0, 1024}}}
	a, _ := block.carve(100, 64)
	b, _ := block.carve(100, 64)
	c, ok := block.carve(100, 256)
	if !ok || a != 0 || b != 128 || c != 256 {
		t.Fatalf("carved at %d, %d, %d (ok %v), want aligned offsets 0, 128, 256", a, b, c, ok)
	}
	block.give(b, 100)
	block.give(a, 100)
	block.give(c, 100)
	if len(block.free) != 1 || block.free[0] != (memorySpan{0, 1024}) {
		t.Fatalf("free spans after giving everything back = %v", block.free)
	}
	if _, ok := block.carve(2048, 1); ok {
		t.Fatal("carved more than the block holds")
	}
}
