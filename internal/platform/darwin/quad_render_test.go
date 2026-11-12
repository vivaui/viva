//go:build darwin && vivahead

package darwin

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

func TestOffscreenConcaveFillAndTextFollowTheSameOutline(t *testing.T) {
	offscreen, err := NewOffscreen(120, 120, 1, Color{Alpha: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer offscreen.Close()
	points := [4]gfx.Point{{}, {X: 100}, {Y: 100}, {X: 20, Y: 20}}
	white := gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 0.5}
	var canvas gfx.Canvas
	canvas.FillQuad(gfx.Quad{Points: points, Tint: [4]gfx.Color{white, white, white, white}})
	var triangles []render.Rectangle
	for _, triangle := range canvas.Triangles() {
		triangles = append(triangles, render.Rectangle{Kind: scene.KindTriangle, Color: scene.Color(triangle.Color),
			Points: [3]scene.Point{scene.Point(triangle.Points[0]), scene.Point(triangle.Points[1]), scene.Point(triangle.Points[2])}, EdgeMask: triangle.EdgeMask})
	}
	label := TextLabel{Width: 100, Height: 100, HasQuad: true, Quad: points,
		Bitmap: []byte{128, 128, 128, 128}, BitmapWidth: 1, BitmapHeight: 1, BitmapStride: 4, BitmapID: 1, BitmapUpload: true}
	for index := range label.Tint {
		label.Tint[index] = Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	}
	for _, text := range []bool{false, true} {
		t.Run(fmt.Sprintf("text=%t", text), func(t *testing.T) {
			shapes, labels := triangles, []TextLabel(nil)
			if text {
				shapes, labels = nil, []TextLabel{label}
			}
			if err := offscreen.Set(shapes, labels, nil); err != nil {
				t.Fatal(err)
			}
			pixels, width, _, err := offscreen.Render()
			if err != nil {
				t.Fatal(err)
			}
			if directory := os.Getenv("VIVA_WARPED_SHOTS"); directory != "" {
				picture := image.NewRGBA(image.Rect(0, 0, width, len(pixels)/width/4))
				for row := range picture.Bounds().Dy() {
					for column := range width {
						blue, green, red, alpha := pixelAt(pixels, width, column, row)
						picture.SetRGBA(column, row, color.RGBA{R: red, G: green, B: blue, A: alpha})
					}
				}
				file, err := os.Create(filepath.Join(directory, fmt.Sprintf("concave-text-%t.png", text)))
				if err != nil {
					t.Fatal(err)
				}
				err = png.Encode(file, picture)
				file.Close()
				if err != nil {
					t.Fatal(err)
				}
			}
			for _, point := range [][2]int{{40, 40}, {80, 80}} {
				blue, green, red, _ := pixelAt(pixels, width, point[0], point[1])
				if blue != 0 || green != 0 || red != 0 {
					t.Fatalf("text=%t: outside point %v painted BGRA %d,%d,%d", text, point, blue, green, red)
				}
			}
			_, _, red, _ := pixelAt(pixels, width, 10, 10)
			if red < 120 || red > 136 {
				t.Fatalf("text=%t: interior red=%d, want one half-opacity surface", text, red)
			}
		})
	}
}

func TestOffscreenWarpedSourceTextureMemoryFollowsDestination(t *testing.T) {
	for _, compressed := range []bool{false, true} {
		t.Run(fmt.Sprintf("compressed=%t", compressed), func(t *testing.T) {
			offscreen, err := NewOffscreen(240, 100, 2, Color{Alpha: 1})
			if err != nil {
				t.Fatal(err)
			}
			defer offscreen.Close()
			edge := float32(100000)
			if compressed {
				edge = 200
			}
			label := TextLabel{Value: strings.Repeat("M", 3000), Height: 24, FontSize: 14,
				Color: Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}, HasQuad: true,
				Quad: [4]gfx.Point{{Y: 10}, {X: edge, Y: 20}, {Y: 34}, {X: edge, Y: 44}}}
			for index := range label.Tint {
				label.Tint[index] = Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
			}
			var previous uint64
			for _, sourceWidth := range []float32{100000, 10000000} {
				label.Width = sourceWidth
				if err := offscreen.Set(nil, []TextLabel{label}, nil); err != nil {
					t.Fatal(err)
				}
				bytes := offscreen.LabelTextureBytes()
				t.Logf("source width %.0f, projected width %.0f: native texture bytes %d", sourceWidth, edge, bytes)
				if bytes == 0 || previous != 0 && bytes > previous*2 {
					t.Fatalf("a fixed destination must not allocate textures proportional to source width: %d -> %d", previous, bytes)
				}
				previous = bytes
			}
		})
	}
}

func BenchmarkOffscreenMovingWarpedWideLabel(b *testing.B) {
	offscreen, err := NewOffscreen(240, 100, 2, Color{Alpha: 1})
	if err != nil {
		b.Fatal(err)
	}
	defer offscreen.Close()
	label := TextLabel{Value: strings.Repeat("M", 3000), X: -20000, Width: 40000, Height: 24, FontSize: 14,
		Color: Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}, HasQuad: true}
	for index := range label.Tint {
		label.Tint[index] = Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	}
	labels := []TextLabel{label}
	iteration := 0
	b.ReportAllocs()
	for b.Loop() {
		left := float32(iteration % 20)
		iteration++
		labels[0].Quad = [4]gfx.Point{{X: left, Y: 10}, {X: left + 200, Y: 20}, {X: left, Y: 34}, {X: left + 200, Y: 44}}
		if err := offscreen.Set(nil, labels, nil); err != nil {
			b.Fatal(err)
		}
		if _, _, _, err := offscreen.Render(); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(offscreen.LabelTextureBytes()), "texture-B")
}

func TestOffscreenWideTrapezoidKeepsAffineTintAcrossTiles(t *testing.T) {
	offscreen, err := NewOffscreen(9000, 90, 1.5, Color{Alpha: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer offscreen.Close()
	left, right := Color{Green: 1, Alpha: 1}, Color{Red: 1, Alpha: 1}
	label := TextLabel{Value: strings.Repeat("W", 4000), Width: 36000, Height: 60, FontSize: 36,
		Color: Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}, HasQuad: true,
		Quad: [4]gfx.Point{{Y: 10}, {X: 18000, Y: 10}, {Y: 70}, {X: 9000, Y: 70}},
		Tint: [4]Color{left, right, left, right}}
	if err := offscreen.Set(nil, []TextLabel{label}, nil); err != nil {
		t.Fatal(err)
	}
	pixels, width, _, err := offscreen.Render()
	if err != nil {
		t.Fatal(err)
	}
	for _, center := range []int{2500, 5000, 7500, 8700} {
		count := 0
		for row := 45; row < 64; row++ {
			for column := center - 50; column < center+50; column++ {
				_, green, red, _ := pixelAt(pixels, width, column*3/2, row*3/2)
				if int(red)+int(green) < 80 {
					continue
				}
				sampleX := (float64(column*3/2) + 0.5) / 1.5
				sampleY := (float64(row*3/2) + 0.5) / 1.5
				vertical := (sampleY - 10) / 60
				expected := sampleX / 18000
				if sampleX > 18000*(1-vertical) {
					expected = sampleX/9000 - (1 - vertical)
				}
				got := float64(red) / float64(int(red)+int(green))
				if difference := got - expected; difference < -0.025 || difference > 0.025 {
					t.Fatalf("at %.1f,%.1f: source tint %.3f, want original triangle's affine tint %.3f", sampleX, sampleY, got, expected)
				}
				count++
			}
		}
		if count < 100 {
			t.Fatalf("tile near %d has only %d ink samples", center, count)
		}
	}
}
