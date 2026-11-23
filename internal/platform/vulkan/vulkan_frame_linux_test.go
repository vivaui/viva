//go:build linux

package vulkan

import (
	"image"
	"testing"

	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

// A window frame draws the scene inside a margin, rounds the content's corners
// and draws a shadow in the margin; the margin outside the shadow is transparent.
func TestOffscreenWindowFrameDrawsShadowAndRoundCorners(t *testing.T) {
	const width, height, margin = 200, 160, 24
	offscreen := newTestOffscreen(t, width, height)
	if err := offscreen.renderer.SetFrameMargin(margin); err != nil {
		t.Fatal(err)
	}
	// The scene is in the content's own coordinates: a red fill of all of it.
	red := scene.Color{Red: 1, Alpha: 1}
	pixels := renderScene(t, offscreen, []render.Rectangle{
		{Bounds: scene.Rect{Width: width - 2*margin, Height: height - 2*margin}, Color: red},
	}, nil)

	if r, g, b, a := rgbaAt(pixels, width/2, height/2); r != 255 || g != 0 || b != 0 || a != 255 {
		t.Errorf("content = rgba(%d,%d,%d,%d), want opaque red", r, g, b, a)
	}
	// The scene starts at the margin, not at the corner of the window.
	if _, _, _, a := rgbaAt(pixels, 0, 0); a != 0 {
		t.Errorf("window corner alpha = %d, want transparent", a)
	}
	if r, _, _, a := rgbaAt(pixels, margin+30, margin+30); r != 255 || a != 255 {
		t.Errorf("content just inside the corner = r%d a%d, want red", r, a)
	}
	// The square corner of the content is cut: what is left there is shadow, black, not red,
	// and the shadow reaches into it, so the cut does not leave a notch in the shadow.
	if r, _, _, a := rgbaAt(pixels, margin+1, margin+1); r != 0 || a < 20 || a > 200 {
		t.Errorf("cut corner = r%d a%d, want the soft black shadow and no red", r, a)
	}
	// On the diagonal the arc of a radius-10 corner is 4.1 pixels from the square
	// corner: pixel 2 is outside it, pixel 7 inside.
	if r, _, _, a := rgbaAt(pixels, margin+2, margin+2); r > 40 {
		t.Errorf("pixel outside the arc = r%d a%d, want it cut away", r, a)
	}
	if r, _, _, a := rgbaAt(pixels, margin+7, margin+7); r != 255 || a != 255 {
		t.Errorf("pixel inside the arc = r%d a%d, want red", r, a)
	}
	// The same at the other three corners.
	for _, corner := range [][2]int{{width - margin - 3, margin + 2}, {margin + 2, height - margin - 3}, {width - margin - 3, height - margin - 3}} {
		if r, _, _, _ := rgbaAt(pixels, corner[0], corner[1]); r > 40 {
			t.Errorf("corner pixel %v = r%d, want it cut away", corner, r)
		}
	}
	// Below the content the shadow is black and soft: stronger near the edge.
	near, far := alphaAt(pixels, width/2, height-margin+1), alphaAt(pixels, width/2, height-margin+14)
	if near < 40 || far >= near {
		t.Errorf("shadow alpha is %d near the content and %d farther down, want a fall-off from a visible start", near, far)
	}
	if r, g, b, _ := rgbaAt(pixels, width/2, height-margin+4); r != 0 || g != 0 || b != 0 {
		t.Errorf("shadow color = %d,%d,%d, want black", r, g, b)
	}
	// Past the shadow the window is transparent.
	if a := alphaAt(pixels, width/2, height-1); a != 0 {
		t.Errorf("alpha at the window's bottom edge = %d, want 0", a)
	}
}

func alphaAt(pixels *image.RGBA, x, y int) byte {
	_, _, _, alpha := rgbaAt(pixels, x, y)
	return alpha
}
