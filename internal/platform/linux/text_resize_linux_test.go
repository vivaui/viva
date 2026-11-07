//go:build linux

package linux

import (
	"fmt"
	"image/color"
	"math"
	"strings"
	"testing"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

func resizeTestLabels(count int, width float32) []platform.TextLabel {
	labels := make([]platform.TextLabel, count)
	for i := range labels {
		value := fmt.Sprintf("row-%04d-%s", i, strings.Repeat("x", 80))
		labels[i] = platform.TextLabel{
			Value: value, Y: float32(i * 20), Width: width, Height: 20,
			FontSize: 14, Mono: true, CellWidth: 8,
			Runs: []ui.TextRun{{Bytes: int32(len(value)), Cells: int32(len(value))}},
		}
	}
	return labels
}

func BenchmarkVulkanTextLiveResize(b *testing.B) {
	for _, count := range []int{256, 1024} {
		b.Run(fmt.Sprintf("ClippedWidths/%d", count), func(b *testing.B) {
			narrow := resizeTestLabels(count, 60)
			wide := append([]platform.TextLabel(nil), narrow...)
			for i := range wide {
				wide[i].Width++
			}
			var state vulkanText
			if err := state.update(nil, narrow); err != nil {
				b.Fatal(err)
			}
			for _, labels := range [][]platform.TextLabel{wide, narrow} {
				if err := state.update(nil, labels); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				labels := narrow
				if i&1 == 0 {
					labels = wide
				}
				if err := state.update(nil, labels); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(fmt.Sprintf("Unchanged/%d", count), func(b *testing.B) {
			labels := resizeTestLabels(count, 800)
			var state vulkanText
			if err := state.update(nil, labels); err != nil {
				b.Fatal(err)
			}
			if err := state.update(nil, labels); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := state.update(nil, labels); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(fmt.Sprintf("RepeatedClippedWidths/%d", count), func(b *testing.B) {
			narrow := resizeTestLabels(count, 60)
			for i := range narrow {
				narrow[i].Value, narrow[i].Runs = narrow[0].Value, narrow[0].Runs
			}
			wide := append([]platform.TextLabel(nil), narrow...)
			for i := range wide {
				wide[i].Width++
			}
			var state vulkanText
			for _, labels := range [][]platform.TextLabel{narrow, wide, narrow} {
				if err := state.update(nil, labels); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// The old textures were clipped at the previous width. Growing
				// beyond them requires new pictures, even for duplicate rows.
				for j := range state.labels {
					state.labels[j].view = uintptr(j + 1)
					state.labels[j].scale = 1
					state.labels[j].width = int(state.labels[j].label.Width)
				}
				labels := narrow
				if i&1 == 0 {
					labels = wide
				}
				if err := state.update(nil, labels); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
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
				reserved := labelRasterWidth(label, scale, height)
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

func TestVulkanTextResizeKeepsDistinctDuplicateTextures(t *testing.T) {
	labels := resizeTestLabels(3, 800)
	labels[1].Value, labels[1].Runs = labels[0].Value, labels[0].Runs
	var state vulkanText
	if err := state.update(nil, labels); err != nil {
		t.Fatal(err)
	}
	for i := range state.labels {
		state.labels[i].view = uintptr(i + 1)
	}
	next := []platform.TextLabel{labels[2], labels[0], labels[1]}
	if err := state.update(nil, next); err != nil {
		t.Fatal(err)
	}
	seen := make(map[uintptr]bool)
	for i, label := range state.labels {
		if label.view == 0 || seen[label.view] {
			t.Fatalf("slot %d retained invalid or duplicate image view %d", i, label.view)
		}
		seen[label.view] = true
	}
	if state.labels[0].view != 3 {
		t.Fatalf("moved unique label view = %d, want 3", state.labels[0].view)
	}
}

func TestVulkanTextResizeInvalidLabelKeepsPreviousFrame(t *testing.T) {
	labels := resizeTestLabels(3, 800)
	var state vulkanText
	if err := state.update(nil, labels); err != nil {
		t.Fatal(err)
	}
	state.labels[0].view = 42
	next := append([]platform.TextLabel(nil), labels...)
	next[0].Value = "changed"
	next[2].Width = -1
	if err := state.update(nil, next); err == nil {
		t.Fatal("accepted negative width")
	}
	if state.labels[0].view != 42 || state.labels[0].label.Value != labels[0].Value {
		t.Fatal("invalid update discarded the previous frame")
	}
}

func TestVulkanGridResizeReusesReservedColumnsAcrossIndexShift(t *testing.T) {
	labels := resizeTestLabels(2, 70)
	var state vulkanText
	if err := state.update(nil, labels); err != nil {
		t.Fatal(err)
	}
	for i := range state.labels {
		state.labels[i].view = uintptr(i + 1)
		state.labels[i].width = 128
		state.labels[i].scale = 1
	}
	next := []platform.TextLabel{labels[1], labels[0]}
	next[0].Width, next[1].Width = 105, 35
	if err := state.update(nil, next); err != nil {
		t.Fatal(err)
	}
	for i, want := range []uintptr{2, 1} {
		if state.labels[i].view != want || state.labels[i].label.Width != next[i].Width {
			t.Fatalf("row %d lost its texture or new visible width: %+v", i, state.labels[i])
		}
	}
	next[0].Width = 129
	if err := state.update(nil, next); err != nil {
		t.Fatal(err)
	}
	if state.labels[0].view != 0 || state.labels[1].view != 1 {
		t.Fatal("growing beyond the reserved columns did not invalidate only the wider row")
	}
	next[1].Runs = append([]ui.TextRun(nil), next[1].Runs...)
	next[1].Runs[0].Color = ui.RGB(255, 0, 0)
	if err := state.update(nil, next); err != nil {
		t.Fatal(err)
	}
	if state.labels[1].view != 0 {
		t.Fatal("a changed run color kept its old texture")
	}
}

func TestVulkanGridTextureReservationIsBounded(t *testing.T) {
	label := resizeTestLabels(1, 70)[0]
	if got := labelRasterWidth(label, 1, 20); got != 128 {
		t.Fatalf("reserved width = %d, want 128", got)
	}
	label.Value = "hello"
	label.Runs = []ui.TextRun{{Bytes: 5, Cells: 5}}
	if got := labelRasterWidth(label, 1, 20); got != 50 {
		t.Fatalf("short row width = %d, want its ink and overhang only", got)
	}
	label.Value = strings.Repeat("x", 100000)
	label.Runs = []ui.TextRun{{Bytes: int32(len(label.Value)), Cells: int32(len(label.Value))}}
	for _, test := range []struct {
		width  float32
		height int
		want   int
	}{{70, 20, 128}, {16360, 20, 16384}, {1000, 16384, 1024}} {
		label.Width = test.width
		if got := labelRasterWidth(label, 1, test.height); got != test.want {
			t.Fatalf("width %g height %d reserves %d pixels, want %d", test.width, test.height, got, test.want)
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
				Runs: []ui.TextRun{
					{Bytes: 4, Cells: 4, Color: ui.RGB(255, 60, 60), Ligatures: true},
					{Column: 4, Bytes: 3, Cells: 2, Color: ui.RGB(60, 255, 60)},
					{Column: 6, Bytes: 3, Cells: 1, Color: ui.RGB(60, 60, 255)},
					{Column: 8, Bytes: 24, Cells: 24, Color: ui.RGB(255, 255, 255), Underline: true, Ligatures: true},
				},
			}
			full := renderScene(t, offscreen, nil, []platform.TextLabel{label})
			renderer := offscreen.renderer
			creates := 0
			createImage := renderer.api.createImage
			renderer.api.createImage = func(device uintptr, info *vkImageCreateInfo, allocator unsafe.Pointer, image *uintptr) int32 {
				creates++
				return createImage(device, info, allocator, image)
			}
			for _, width := range []float32{53.5, 67.25, 99.5, 120.25, 41.5} {
				label.Width = width
				label.Clipped = width == 99.5
				label.Clip = ui.Rect{X: 10, Y: 0, Width: 80, Height: 30}
				// Compare against complete glyphs, then crop. Rasterizing directly
				// into a surface that cuts through a glyph lets Cairo's clipping
				// change its edge coverage; resizing should keep that glyph stable.
				// This also checks ownership of the blank after the icon.
				deviceWidth := int(math.Ceil(float64(label.Width * float32(scale))))
				deviceHeight := int(math.Ceil(float64(label.Height * float32(scale))))
				reservedWidth := labelRasterWidth(label, float32(scale), deviceHeight)
				needed, _ := neededLabelWidth(label)
				fullWidth := int(math.Ceil(float64(needed * float32(scale))))
				complete, _, err := renderer.text.services.rasterize(label, float32(scale), fullWidth, deviceHeight, false)
				if err != nil {
					t.Fatal(err)
				}
				reserved, _, err := renderer.text.services.rasterize(label, float32(scale), reservedWidth, deviceHeight, false)
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
				if creates != 0 {
					t.Fatalf("width %g recreated %d images inside reserved columns", width, creates)
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

func TestVulkanTextResizeMatchesUnclippedWidthAndRejectsClippedWidth(t *testing.T) {
	labels := resizeTestLabels(2, 80)
	labels[1].Value = "short"
	labels[1].Runs = []ui.TextRun{{Bytes: 5, Cells: 5}}
	var state vulkanText
	if err := state.update(nil, labels); err != nil {
		t.Fatal(err)
	}
	state.labels[0].view, state.labels[1].view = 41, 42
	next := []platform.TextLabel{labels[1], labels[0]}
	next[0].Width, next[1].Width = 200, 85
	if err := state.update(nil, next); err != nil {
		t.Fatal(err)
	}
	if state.labels[0].view != 42 {
		t.Fatalf("unclipped moved text view = %d, want 42", state.labels[0].view)
	}
	if state.labels[1].view != 0 {
		t.Fatal("reused texture that was clipped at a narrower width")
	}
	state.labels[1].view = 99
	next[1].Width = 80
	if err := state.update(nil, next); err != nil {
		t.Fatal(err)
	}
	if state.labels[0].view != 42 || state.labels[1].view != 0 {
		t.Fatal("shrinking a clipped label did not retain only the unaffected texture")
	}
}

func TestVulkanTextResizeBitmapMatchesSourceAndKeepsDrawSize(t *testing.T) {
	one, two := []byte{0, 0, 255, 255}, []byte{0, 0, 255, 255}
	labels := []platform.TextLabel{
		{Width: 10, Height: 10, Bitmap: one, BitmapWidth: 1, BitmapHeight: 1, BitmapStride: 4, BitmapID: 1},
		{Width: 10, Height: 10, Bitmap: two, BitmapWidth: 1, BitmapHeight: 1, BitmapStride: 4, BitmapID: 1},
	}
	var state vulkanText
	if err := state.update(nil, labels); err != nil {
		t.Fatal(err)
	}
	state.labels[0].view, state.labels[1].view = 41, 42
	next := []platform.TextLabel{labels[1], labels[0]}
	next[0].Width, next[0].Height = 20, 30
	if err := state.update(nil, next); err != nil {
		t.Fatal(err)
	}
	if state.labels[0].view != 42 || state.labels[1].view != 41 {
		t.Fatal("bitmap source identity was lost when the labels moved")
	}
	if state.labels[0].label.Width != 20 || state.labels[0].label.Height != 30 {
		t.Fatal("retained bitmap did not take its new drawn dimensions")
	}
}
