//go:build linux

package linux

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

func TestLabelScissorFollowsTheCanvasClip(t *testing.T) {
	extent := vkExtent2D{Width: 200, Height: 100}
	whole, visible := labelScissor(platform.TextLabel{}, 1, extent)
	if !visible || whole != (vkRect2D{Extent: extent}) {
		t.Fatalf("an unclipped label gets %+v, %v; want the whole target", whole, visible)
	}

	clipped := platform.TextLabel{Clipped: true, Clip: gfx.Rect{X: 10.4, Y: -5, Width: 50.2, Height: 30}}
	got, visible := labelScissor(clipped, 1, extent)
	want := vkRect2D{Offset: [2]int32{10, 0}, Extent: vkExtent2D{Width: 51, Height: 25}}
	if !visible || got != want {
		t.Fatalf("scissor = %+v, %v; want %+v", got, visible, want)
	}

	// A clip reaching past the target is cut to it.
	clipped.Clip = gfx.Rect{X: 150, Y: 60, Width: 500, Height: 500}
	got, visible = labelScissor(clipped, 1, extent)
	want = vkRect2D{Offset: [2]int32{150, 60}, Extent: vkExtent2D{Width: 50, Height: 40}}
	if !visible || got != want {
		t.Fatalf("scissor = %+v, %v; want %+v", got, visible, want)
	}

	// A clip with no pixel in the target draws nothing.
	clipped.Clip = gfx.Rect{X: 300, Y: 0, Width: 10, Height: 10}
	if _, visible = labelScissor(clipped, 1, extent); visible {
		t.Fatal("a clip outside the target is visible")
	}

	// At a fractional scale the clip, in logical pixels, takes every device pixel
	// it touches.
	scaled := platform.TextLabel{Clipped: true, Clip: gfx.Rect{X: 10.2, Y: 4, Width: 20, Height: 10.5}}
	got, visible = labelScissor(scaled, 1.5, extent)
	want = vkRect2D{Offset: [2]int32{15, 6}, Extent: vkExtent2D{Width: 31, Height: 16}}
	if !visible || got != want {
		t.Fatalf("scissor at 1.5x = %+v, %v; want %+v", got, visible, want)
	}
}
