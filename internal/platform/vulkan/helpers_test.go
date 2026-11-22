//go:build linux

package vulkan

import (
	"errors"
	"fmt"
	"image"
	"strings"
	"testing"
	"unsafe"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
)

// solidRasterizer draws every label as a solid block of its size: the tests of the
// renderer check where and how a label's texture is drawn, not the shapes of glyphs,
// which belong to the platform's text engine.
type solidRasterizer struct{}

func (solidRasterizer) Rasterize(label platform.TextLabel, scale float32, skip, width, height int, allowLCD bool) ([]byte, bool, error) {
	pixels := make([]byte, width*height*4)
	for index := range pixels {
		pixels[index] = 0xff
	}
	return pixels, false, nil
}
func (solidRasterizer) Spans() []ColorSpan { return nil }
func (solidRasterizer) Close()             {}

func newSolidRasterizer() (TextRasterizer, error) { return solidRasterizer{}, nil }

// newTestOffscreen skips the test on a machine with no Vulkan driver, as the
// other graphics tests do, and fails on any other error.
func newTestOffscreen(t *testing.T, width, height int) *Offscreen {
	t.Helper()
	return newScaledOffscreen(t, width, height, 1)
}

// newScaledOffscreen is newTestOffscreen at a display scale.
func newScaledOffscreen(t *testing.T, width, height int, scale float64) *Offscreen {
	t.Helper()
	offscreen, err := NewOffscreen(width, height, scale, platform.Color{Alpha: 1}, newSolidRasterizer)
	if errors.Is(err, platform.ErrNoGraphics) {
		t.Skipf("no Vulkan device: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(offscreen.Close)
	return offscreen
}

func renderScene(t *testing.T, offscreen *Offscreen, rectangles []render.Rectangle, labels []platform.TextLabel) *image.RGBA {
	t.Helper()
	if err := offscreen.Set(rectangles, labels); err != nil {
		t.Fatal(err)
	}
	pixels, err := offscreen.Image()
	if err != nil {
		t.Fatal(err)
	}
	return pixels
}

func rgbaAt(pixels *image.RGBA, x, y int) (red, green, blue, alpha byte) {
	offset := pixels.PixOffset(x, y)
	return pixels.Pix[offset], pixels.Pix[offset+1], pixels.Pix[offset+2], pixels.Pix[offset+3]
}

func isPainted(pixels *image.RGBA, x, y int) bool {
	red, green, blue, _ := rgbaAt(pixels, x, y)
	return int(red)+int(green)+int(blue) > 60
}

// vkStructureHeadlessSurfaceCreateInfo is VK_STRUCTURE_TYPE_HEADLESS_SURFACE_CREATE_INFO_EXT.
const vkStructureHeadlessSurfaceCreateInfo = 1000256000

type vkHeadlessSurfaceCreateInfo struct {
	SType uint32
	_     uint32
	Next  uintptr
	Flags uint32
	_     uint32
}

// headlessSurfaceSource makes surfaces with VK_EXT_headless_surface, which needs no
// window system: what a window renderer presents to when there is no display.
func headlessSurfaceSource(t testing.TB) SurfaceSource {
	t.Helper()
	var create func(uintptr, *vkHeadlessSurfaceCreateInfo, unsafe.Pointer, *uintptr) int32
	if err := Bind(&create, "vkCreateHeadlessSurfaceEXT"); err != nil {
		if errors.Is(err, platform.ErrNoGraphics) {
			t.Skipf("no Vulkan device: %v", err)
		}
		t.Fatal(err)
	}
	return SurfaceSource{
		Extension: "VK_EXT_headless_surface",
		Create: func(instance uintptr) (uintptr, error) {
			info := vkHeadlessSurfaceCreateInfo{SType: vkStructureHeadlessSurfaceCreateInfo}
			var surface uintptr
			if err := vkCheck(create(instance, &info, nil, &surface), "create headless surface"); err != nil {
				return 0, err
			}
			return surface, nil
		},
	}
}

func resizeTestLabels(count int, width float32) []platform.TextLabel {
	labels := make([]platform.TextLabel, count)
	for i := range labels {
		value := fmt.Sprintf("row-%04d-%s", i, strings.Repeat("x", 80))
		labels[i] = platform.TextLabel{
			Value: value, Y: float32(i * 20), Width: width, Height: 20,
			FontSize: 14, Mono: true, CellWidth: 8,
			Runs: []gfx.TextRun{{Bytes: int32(len(value)), Cells: int32(len(value))}},
		}
	}
	return labels
}

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

// windowSurface is a surface for a renderer to present to: an X window where there is a
// display (which every Vulkan driver presents to), else a headless surface. The
// returned resize changes the surface's size where it can, and reports false where the
// surface has no size of its own.
func windowSurface(t testing.TB, width, height uint32) (source SurfaceSource, resize func(width, height uint32) bool) {
	t.Helper()
	if windowSystemSurface != nil {
		return windowSystemSurface(t, width, height)
	}
	return headlessSurfaceSource(t), func(uint32, uint32) bool { return false }
}

// windowSystemSurface is set by the files that know a window system.
var windowSystemSurface func(t testing.TB, width, height uint32) (SurfaceSource, func(width, height uint32) bool)
