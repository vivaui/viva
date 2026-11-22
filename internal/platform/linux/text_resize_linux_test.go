//go:build linux

package linux

import (
	"fmt"
	"image/color"
	"math"
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/platform/vulkan"
)

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

func TestVulkanProportionalTextureKeepsGlyphOverhang(t *testing.T) {
	services, err := newTextServices()
	if err != nil {
		t.Skipf("no text services: %v", err)
	}
	defer services.close()
	for _, scale := range []float32{1, 1.25} {
		for _, size := range []float32{48, 96} {
			for _, value := range []string{"f", "ff"} {
				label := platform.TextLabel{
					Value: value, Width: 300, Height: size * 2, FontSize: size, Italic: true,
					Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
				}
				height := int(math.Ceil(float64(label.Height * scale)))
				width := int(math.Ceil(float64(label.Width * scale)))
				reserved := vulkan.LabelRasterWidth(label, scale, height)
				pixels, _, err := services.rasterize(label, scale, width, height, false)
				if err != nil {
					t.Fatal(err)
				}
				for y := 0; y < height; y++ {
					for x := reserved; x < width; x++ {
						if pixels[(y*width+x)*4+3] != 0 {
							t.Fatalf("%q size %g scale %g: texture width %d cuts ink at (%d,%d)", value, size, scale, reserved, x, y)
						}
					}
				}
			}
		}
	}
}

// The same texture must survive both shrinking and growing, while the pixels
// remain an exact crop of the full row: colors, wide glyphs and icon overhang
// keep their original placement, also at a fractional display scale.
func TestVulkanGridResizeKeepsTextureAndClipsItsPixels(t *testing.T) {
	for _, scale := range []float64{1, 1.5} {
		t.Run(fmt.Sprintf("scale-%g", scale), func(t *testing.T) {
			offscreen := newScaledOffscreen(t, 170, 35, scale)
			label := platform.TextLabel{
				Value: "abcd界\uf306" + strings.Repeat("x", 24), X: 2.2, Y: 3.2, OriginX: 0.33,
				Width: 128, Height: 24, FontSize: 14, Mono: true, CellWidth: 8,
				Runs: []gfx.TextRun{
					{Bytes: 4, Cells: 4, Color: gfx.RGB(255, 60, 60), Ligatures: true},
					{Column: 4, Bytes: 3, Cells: 2, Color: gfx.RGB(60, 255, 60)},
					{Column: 6, Bytes: 3, Cells: 1, Color: gfx.RGB(60, 60, 255)},
					{Column: 8, Bytes: 24, Cells: 24, Color: gfx.RGB(255, 255, 255), Underline: true, Ligatures: true},
				},
			}
			full := renderScene(t, offscreen, nil, []platform.TextLabel{label})
			services, err := newTextServices()
			if err != nil {
				t.Skipf("no text services: %v", err)
			}
			defer services.close()
			textures := offscreen.Stats().Textures
			for _, width := range []float32{53.5, 67.25, 99.5, 120.25, 41.5} {
				label.Width = width
				label.Clipped = width == 99.5
				label.Clip = gfx.Rect{X: 10, Y: 0, Width: 80, Height: 30}
				// Compare against complete glyphs, then crop. Rasterizing directly
				// into a surface that cuts through a glyph lets Cairo's clipping
				// change its edge coverage; resizing should keep that glyph stable.
				// This also checks ownership of the blank after the icon.
				deviceWidth := int(math.Ceil(float64(label.Width * float32(scale))))
				deviceHeight := int(math.Ceil(float64(label.Height * float32(scale))))
				reservedWidth := vulkan.LabelRasterWidth(label, float32(scale), deviceHeight)
				needed, _ := vulkan.NeededLabelWidth(label)
				fullWidth := int(math.Ceil(float64(needed * float32(scale))))
				complete, _, err := services.rasterize(label, float32(scale), fullWidth, deviceHeight, false)
				if err != nil {
					t.Fatal(err)
				}
				reserved, _, err := services.rasterize(label, float32(scale), reservedWidth, deviceHeight, false)
				if err != nil {
					t.Fatal(err)
				}
				for y := 0; y < deviceHeight; y++ {
					for x := 0; x < deviceWidth*4; x++ {
						if complete[y*fullWidth*4+x] != reserved[y*reservedWidth*4+x] {
							t.Fatalf("width %g reserved columns changed complete glyph raster byte (%d,%d)", width, x, y)
						}
					}
				}
				pixels := renderScene(t, offscreen, nil, []platform.TextLabel{label})
				if made := offscreen.Stats().Textures - textures; made != 0 {
					t.Fatalf("width %g recreated %d images inside reserved columns", width, made)
				}
				x := platform.DevicePosition(label.X, label.OriginX, float32(scale))
				left, right := x, x+int(math.Ceil(float64(label.Width*float32(scale))))
				bottom := pixels.Bounds().Dy()
				if label.Clipped {
					left = max(left, int(math.Floor(float64(label.Clip.X)*scale)))
					right = min(right, int(math.Ceil(float64(label.Clip.X+label.Clip.Width)*scale)))
					bottom = min(bottom, int(math.Ceil(float64(label.Clip.Height)*scale)))
				}
				for y := 0; y < pixels.Bounds().Dy(); y++ {
					for x := 0; x < pixels.Bounds().Dx(); x++ {
						want := color.RGBA{A: 255}
						if x >= left && x < right && y < bottom {
							want = full.RGBAAt(x, y)
						}
						if got := pixels.RGBAAt(x, y); got != want {
							t.Fatalf("width %g pixel (%d,%d) = %v, want %v", width, x, y, got, want)
						}
					}
				}
			}
		})
	}
}
