package linux

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

// wideValue is a log line of 4096 characters: at 8 logical pixels a cell and 150% it is
// 49152 device pixels wide, three times what a texture holds.
var wideValue = strings.Repeat("0123456789abcdef", 256)

func wideMonoLine(value string, x float32, runs []gfx.TextRun) platform.TextLabel {
	return platform.TextLabel{
		Value: value, X: x, Y: 4, Width: float32(len(value))*8 + 8, Height: 18, FontSize: 13, CellWidth: 8,
		Mono: true, Runs: runs, Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
		Clipped: true, Clip: gfx.Rect{Width: 400, Height: 40},
	}
}

func wholeRun(value string) []gfx.TextRun {
	return []gfx.TextRun{{Bytes: int32(len(value)), Cells: int32(len(value)), Weight: 400, Color: gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}}}
}

// wideBackends open a 400 by 40 scene at 150%, cleared to clear, with the Vulkan renderer and
// with the CPU one, and return what draws labels on it. A backend the machine cannot run skips.
var wideBackends = []struct {
	name string
	open func(t *testing.T, clear platform.Color) func(labels ...platform.TextLabel) *image.RGBA
	// same reports whether two composited pixels match. The Vulkan text sampler
	// filters linearly, so a window starting off a texel centre can mix 1/256 of
	// its neighbour; the CPU renderer samples the glyph bitmap directly.
	same func(a, b color.RGBA) bool
}{
	{"Vulkan", func(t *testing.T, clear platform.Color) func(labels ...platform.TextLabel) *image.RGBA {
		t.Helper()
		offscreen, err := NewOffscreen(400, 40, 1.5, clear)
		if errors.Is(err, platform.ErrNoGraphics) {
			t.Skipf("no Vulkan device: %v", err)
		}
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(offscreen.Close)
		return func(labels ...platform.TextLabel) *image.RGBA { return renderScene(t, offscreen, nil, labels) }
	}, withinALevel},
	{"CPU", func(t *testing.T, clear platform.Color) func(labels ...platform.TextLabel) *image.RGBA {
		t.Helper()
		offscreen, err := NewCPUOffscreen(400, 40, 1.5, clear)
		if err != nil {
			t.Skipf("no text services: %v", err) // Pango and Cairo are system libraries
		}
		t.Cleanup(offscreen.Close)
		return func(labels ...platform.TextLabel) *image.RGBA {
			t.Helper()
			if err := offscreen.Set(nil, labels); err != nil {
				t.Fatal(err)
			}
			pixels, err := offscreen.Image()
			if err != nil {
				t.Fatal(err)
			}
			return pixels
		}
	}, func(a, b color.RGBA) bool { return a == b }},
}

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

// A monospaced line wider than any texture, such as a long log line shown unwrapped, is drawn
// from a window of it at its start and scrolled, on an opaque target and a transparent one,
// without and with runs, by the Vulkan renderer and by the CPU one. Its pixels are the pixels
// of the start of the same line, which one texture holds, drawn where it is: the window moves
// no glyph. Drawing it whole asked for a texture larger than the bound, and the error stopped
// the window drawing anything again. The Vulkan scene draws each scroll in turn, so the
// texture of the line is windowed again as it scrolls.
func TestWideMonoLineIsDrawnFromAWindowOfIt(t *testing.T) {
	scrolls := []int{0, 1000} // characters scrolled off the left
	for _, backend := range wideBackends {
		t.Run(backend.name, func(t *testing.T) {
			for _, clear := range []platform.Color{{Alpha: 1}, {}} {
				draw := backend.open(t, clear)
				for _, withRuns := range []bool{false, true} {
					name := map[bool]string{false: "transparent", true: "opaque"}[clear.Alpha >= 1] +
						map[bool]string{false: "", true: " with runs"}[withRuns]
					wide := make([]*image.RGBA, len(scrolls))
					for index, scrolled := range scrolls {
						var runs []gfx.TextRun
						if withRuns {
							runs = wholeRun(wideValue)
						}
						wide[index] = draw(wideMonoLine(wideValue, -8*float32(scrolled), runs))
					}
					for index, scrolled := range scrolls {
						start := wideValue[:scrolled+300]
						var runs []gfx.TextRun
						if withRuns {
							runs = wholeRun(start)
						}
						want := draw(wideMonoLine(start, -8*float32(scrolled), runs))
						got, painted := wide[index], 0
						for y := 0; y < 60; y++ {
							for x := 0; x < 600; x++ {
								if !backend.same(got.RGBAAt(x, y), want.RGBAAt(x, y)) {
									t.Fatalf("%s, %d characters scrolled: pixel %d,%d is %v, the start of the line's is %v",
										name, scrolled, x, y, got.RGBAAt(x, y), want.RGBAAt(x, y))
								}
								if isPainted(got, x, y) {
									painted++
								}
							}
						}
						if painted < 1000 {
							t.Errorf("%s, %d characters scrolled: %d pixels of text painted", name, scrolled, painted)
						}
					}
				}
			}
		})
	}
}

// withinALevel reports whether no channel of a and b differs by more than one level.
func withinALevel(a, b color.RGBA) bool {
	near := func(p, q uint8) bool { return int(p)-int(q) <= 1 && int(q)-int(p) <= 1 }
	return near(a.R, b.R) && near(a.G, b.G) && near(a.B, b.B) && near(a.A, b.A)
}

// The CPU window's scene draws a line wider than a texture holds from the window of it the
// canvas shows. A scroll inside that window keeps its picture, one past it draws a new window,
// and a line scrolled off the canvas has no picture until it is scrolled back; after each the
// canvas is what a scene built with the line where it is paints.
func TestSoftSceneDrawsAWideLineFromWhatTheCanvasShows(t *testing.T) {
	services, err := newTextServices()
	if err != nil {
		t.Skipf("no text services: %v", err)
	}
	const scale = 1.5
	clear := platform.Color{Alpha: 1}
	label := wideMonoLine(wideValue, 0, wholeRun(wideValue))
	live, err := newSoftScene(services, 600, 60, scale, clear, nil, []platform.TextLabel{label})
	if err != nil {
		t.Fatal(err)
	}
	live.paint()
	for _, step := range []struct {
		name string
		x    float32
		skip int // the first column of the line's picture, or -1 for none
	}{
		{"scrolled inside its picture", -100, 0},
		{"scrolled past its picture", -8000, 11400},
		{"scrolled a little", -8100, 11400},
		{"scrolled off the canvas", -40000, -1},
		{"scrolled back", -8000, 11400},
	} {
		next := label
		next.X = step.x
		if _, err := live.apply(platform.Frame{TextLabels: []platform.TextLabel{next}}); err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}
		live.paint()
		slot := &live.labels[0]
		switch {
		case step.skip < 0 && (slot.hasImage || slot.visible):
			t.Errorf("%s: the line has a picture", step.name)
		case step.skip >= 0 && (!slot.hasImage || !slot.visible || slot.image.skip != step.skip || slot.image.width != 1800):
			t.Errorf("%s: picture from column %d, %d wide (drawn %v); want %d, 1800", step.name, slot.image.skip, slot.image.width, slot.hasImage, step.skip)
		}
		whole, err := newSoftScene(services, live.canvas.Width, live.canvas.Height, scale, clear, nil, []platform.TextLabel{next})
		if err != nil {
			t.Fatal(err)
		}
		whole.paint()
		if !bytes.Equal(live.canvas.Pix, whole.canvas.Pix) {
			t.Fatalf("%s: canvas differs from a full paint", step.name)
		}
	}
}

// A label no texture holds, an image larger than one or a line taller than one, is not drawn,
// and the labels beside it still are: one label a renderer cannot hold does not fail the frame.
// The CPU renderer has no texture and may draw the image.
func TestLabelNoTextureHoldsDoesNotFailTheFrame(t *testing.T) {
	const imageWidth = platform.LabelTextureLimit + 16
	tall := platform.TextLabel{Value: "tall", X: 200, Y: 0, Width: 40, Height: platform.LabelTextureLimit + 1, FontSize: 13,
		Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}}
	wide := platform.TextLabel{X: 0, Y: 30, Width: imageWidth, Height: 2,
		Bitmap: make([]byte, imageWidth*2*4), BitmapWidth: imageWidth, BitmapHeight: 2, BitmapStride: imageWidth * 4, BitmapID: 1, BitmapVersion: 1}
	beside := platform.TextLabel{Value: "beside", X: 10, Y: 4, Width: 80, Height: 18, FontSize: 13,
		Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}}
	for _, backend := range wideBackends {
		t.Run(backend.name, func(t *testing.T) {
			pixels := backend.open(t, platform.Color{Alpha: 1})(tall, wide, beside)
			painted := 0
			for y := 0; y < 40; y++ { // the label beside, at 150%
				for x := 0; x < 150; x++ {
					if isPainted(pixels, x, y) {
						painted++
					}
				}
			}
			if painted == 0 {
				t.Fatal("the label beside one no texture holds was not drawn")
			}
		})
	}
}
