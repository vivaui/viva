//go:build darwin && vivahead

package darwin

import (
	"testing"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
	"github.com/vivaui/viva/ui"
)

func pixelAt(pixels []byte, width, x, y int) (blue, green, red, alpha byte) {
	offset := (y*width + x) * 4
	return pixels[offset], pixels[offset+1], pixels[offset+2], pixels[offset+3]
}

func TestOffscreenDrawsPremultipliedBGRABitmapPixels(t *testing.T) {
	offscreen, err := NewOffscreen(20, 10, 1, Color{Alpha: 1})
	if err != nil {
		t.Skipf("no Metal device: %v", err)
	}
	defer offscreen.Close()
	pixels := []byte{0, 0, 255, 255, 0, 255, 0, 255}
	labels := []platform.TextLabel{{
		X: 2, Y: 2, Width: 16, Height: 6,
		Bitmap: pixels, BitmapWidth: 2, BitmapHeight: 1, BitmapStride: 8, BitmapID: 1, BitmapUpload: true,
	}}
	if err := offscreen.Set(nil, labels, nil); err != nil {
		t.Fatal(err)
	}
	got, width, _, err := offscreen.Render()
	if err != nil {
		t.Fatal(err)
	}
	if blue, green, red, alpha := pixelAt(got, width, 5, 5); red < 240 || green > 5 || blue > 5 || alpha < 240 {
		t.Fatalf("left pixel = BGRA %d,%d,%d,%d", blue, green, red, alpha)
	}
	if blue, green, red, alpha := pixelAt(got, width, 15, 5); green < 240 || red > 5 || blue > 5 || alpha < 240 {
		t.Fatalf("right pixel = BGRA %d,%d,%d,%d", blue, green, red, alpha)
	}
}

func TestOffscreenTriangleMeshHasNoInternalFeathering(t *testing.T) {
	offscreen, err := NewOffscreen(100, 100, 2, Color{Alpha: 1})
	if err != nil {
		t.Skipf("no Metal device: %v", err)
	}
	defer offscreen.Close()

	white := scene.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	triangles := []render.Rectangle{
		{
			Kind: scene.KindTriangle, Color: white, EdgeMask: 1<<0 | 1<<1,
			Points: [3]scene.Point{{X: 10, Y: 10}, {X: 90, Y: 10}, {X: 90, Y: 90}},
		},
		{
			Kind: scene.KindTriangle, Color: white, EdgeMask: 1<<1 | 1<<2,
			Points: [3]scene.Point{{X: 10, Y: 10}, {X: 90, Y: 90}, {X: 10, Y: 90}},
		},
	}
	if err := offscreen.Set(triangles, nil, nil); err != nil {
		t.Fatal(err)
	}
	pixels, width, _, err := offscreen.Render()
	if err != nil {
		t.Fatal(err)
	}
	for coordinate := 40; coordinate < 160; coordinate += 7 {
		blue, green, red, alpha := pixelAt(pixels, width, coordinate, coordinate)
		if red < 250 || green < 250 || blue < 250 || alpha != 255 {
			t.Fatalf("shared diagonal pixel %d = rgba(%d,%d,%d,%d), want opaque white", coordinate, red, green, blue, alpha)
		}
	}
}

func TestOffscreenShadedTriangleInterpolatesItsCornerColors(t *testing.T) {
	offscreen, err := NewOffscreen(120, 120, 1, Color{Alpha: 1})
	if err != nil {
		t.Skipf("no Metal device: %v", err)
	}
	defer offscreen.Close()
	triangle := render.Rectangle{
		Kind: scene.KindTriangle, Color: scene.Color{Red: 1, Alpha: 1},
		Shade:  scene.Shade{On: true, Colors: [2]scene.Color{{Green: 1, Alpha: 1}, {Blue: 1, Alpha: 1}}},
		Points: [3]scene.Point{{X: 10, Y: 10}, {X: 110, Y: 10}, {X: 10, Y: 110}},
	}
	if err := offscreen.Set([]render.Rectangle{triangle}, nil, nil); err != nil {
		t.Fatal(err)
	}
	pixels, width, _, err := offscreen.Render()
	if err != nil {
		t.Fatal(err)
	}
	for _, at := range [][2]int{{20, 20}, {60, 20}, {20, 60}, {44, 44}, {30, 70}} {
		w1, w2 := (float64(at[0])+0.5-10)/100, (float64(at[1])+0.5-10)/100
		want := [3]float64{(1 - w1 - w2) * 255, w1 * 255, w2 * 255} // red, green, blue
		blue, green, red, alpha := pixelAt(pixels, width, at[0], at[1])
		got := [3]float64{float64(red), float64(green), float64(blue)}
		for channel := range got {
			if diff := got[channel] - want[channel]; diff > 8 || diff < -8 {
				t.Fatalf("pixel %v = rgb(%v), want about rgb(%.0f, %.0f, %.0f)", at, got, want[0], want[1], want[2])
			}
		}
		if alpha != 255 {
			t.Fatalf("pixel %v alpha = %d", at, alpha)
		}
	}
}

func TestOffscreenRendersRectanglesLabelsAndTheEditorWithoutAWindow(t *testing.T) {
	offscreen, err := NewOffscreen(200, 100, 2, Color{Red: 0, Green: 0, Blue: 0, Alpha: 1})
	if err != nil {
		t.Skipf("no Metal device: %v", err)
	}
	defer offscreen.Close()

	rectangles := []render.Rectangle{
		{Kind: scene.KindRectangle, Bounds: scene.Rect{X: 10, Y: 10, Width: 40, Height: 20}, Color: scene.Color{Red: 1, Alpha: 1}},
		// A later layer paints over an earlier one regardless of instance order.
		{Kind: scene.KindRectangle, Bounds: scene.Rect{X: 30, Y: 10, Width: 40, Height: 20}, Color: scene.Color{Green: 1, Alpha: 1}, Layer: 1},
		{Kind: scene.KindRectangle, Bounds: scene.Rect{X: 30, Y: 10, Width: 40, Height: 20}, Color: scene.Color{Blue: 1, Alpha: 1}},
	}
	labels := []TextLabel{{
		Value: "Hello", X: 10, Y: 50, Width: 80, Height: 20, FontSize: 14,
		Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
	}}
	input := &TextInput{
		X: 100, Y: 50, Width: 90, Height: 24, FontSize: 14,
		Color:   platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
		State:   TextState{Value: "abc"},
		OnEvent: func(TextEvent) (TextState, error) { return TextState{}, nil },
	}
	if err := offscreen.Set(rectangles, labels, input); err != nil {
		t.Fatal(err)
	}
	pixels, width, height, err := offscreen.Render()
	if err != nil {
		t.Fatal(err)
	}
	if width != 400 || height != 200 {
		t.Fatalf("size = %dx%d device pixels, want 400x200 at scale 2", width, height)
	}

	if b, g, r, a := pixelAt(pixels, width, 40, 40); r != 255 || g != 0 || b != 0 || a != 255 {
		t.Errorf("red rectangle = %d,%d,%d,%d", r, g, b, a)
	}
	// x=55..99 (logical 27..49) overlaps the layer-0 blue and layer-1 green: green wins.
	if b, g, r, _ := pixelAt(pixels, width, 120, 40); r != 0 || g != 255 || b != 0 {
		t.Errorf("layered rectangle = %d,%d,%d, want the layer-1 green", r, g, b)
	}
	if b, g, r, _ := pixelAt(pixels, width, 300, 20); r != 0 || g != 0 || b != 0 {
		t.Errorf("background = %d,%d,%d, want the clear color", r, g, b)
	}

	lit := func(x0, y0, x1, y1 int) int {
		count := 0
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				if b, g, r, _ := pixelAt(pixels, width, x, y); int(r)+int(g)+int(b) > 200 {
					count++
				}
			}
		}
		return count
	}
	if lit(20, 100, 180, 140) < 30 {
		t.Error("the label drew no glyph pixels")
	}
	if lit(200, 100, 380, 148) < 30 {
		t.Error("the native editor overlay drew no glyph pixels")
	}

	// A second render of the same scene is byte-identical: headless output is
	// deterministic.
	again, _, _, err := offscreen.Render()
	if err != nil {
		t.Fatal(err)
	}
	for index := range pixels {
		if pixels[index] != again[index] {
			t.Fatalf("render is not deterministic at byte %d", index)
		}
	}
}

// gridLabels builds a 120x40 terminal screen: one fixed-pitch label per row,
// colored in stretches of colorRun characters. With wide set, every other
// stretch is a double-width character of its own run, as CJK text is.
func gridLabels(wide bool) []TextLabel {
	const columns, rows, colorRun = 120, 40, 12
	labels := make([]TextLabel, rows)
	for row := range labels {
		var text []byte
		var runs []ui.TextRun
		for column := 0; column < columns; {
			color := ui.Color{Red: 0.9, Green: 0.9, Blue: 0.9, Alpha: 1}
			if (column/colorRun+row)%3 == 1 {
				color = ui.Color{Green: 0.8, Blue: 0.3, Alpha: 1}
			}
			if wide && column%24 >= 12 && column+2 <= columns {
				piece := "世"
				text = append(text, piece...)
				runs = append(runs, ui.TextRun{Column: int32(column), Bytes: int32(len(piece)), Cells: 2, Color: color})
				column += 2
				continue
			}
			end := min(column+colorRun, columns)
			for ; column < end; column++ {
				text = append(text, byte('a'+(column+row)%26))
			}
			runs = append(runs, ui.TextRun{Column: int32(end - colorRun), Bytes: int32(colorRun), Cells: int32(colorRun), Color: color})
		}
		labels[row] = TextLabel{
			Value: string(text), X: 0, Y: float32(row * 16), Width: 120 * 7.8, Height: 16, FontSize: 13,
			Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}, Mono: true, CellWidth: 7.8, Runs: runs,
		}
	}
	return labels
}

func benchmarkGrid(b *testing.B, wide bool) {
	offscreen, err := NewOffscreen(940, 640, 2, Color{Alpha: 1})
	if err != nil {
		b.Skipf("no Metal device: %v", err)
	}
	defer offscreen.Close()
	labels := gridLabels(wide)
	b.ReportAllocs()
	for b.Loop() {
		// This benchmark measures rebuilding the screen, not retained reuse.
		if err := offscreen.Set(nil, nil, nil); err != nil {
			b.Fatal(err)
		}
		if err := offscreen.Set(nil, labels, nil); err != nil {
			b.Fatal(err)
		}
	}
}

// Rasterizing a whole 120x40 screen of colored fixed-pitch text from scratch:
// the cost of a scroll or a full-screen redraw when no line can be reused.
func BenchmarkRasterizeTerminalScreenASCII(b *testing.B) { benchmarkGrid(b, false) }
func BenchmarkRasterizeTerminalScreenWide(b *testing.B)  { benchmarkGrid(b, true) }

// A label a canvas clip cuts partway is painted only inside the clip, at the
// target's own scale; the rest of the same quad stays untouched.
func TestOffscreenScissorsAClippedLabelToItsClip(t *testing.T) {
	offscreen, err := NewOffscreen(20, 10, 2, Color{Alpha: 1})
	if err != nil {
		t.Skipf("no Metal device: %v", err)
	}
	defer offscreen.Close()
	pixels := []byte{0, 0, 255, 255, 0, 255, 0, 255}
	label := platform.TextLabel{
		X: 2, Y: 2, Width: 16, Height: 6,
		Bitmap: pixels, BitmapWidth: 2, BitmapHeight: 1, BitmapStride: 8, BitmapID: 1, BitmapUpload: true,
	}
	render := func(label platform.TextLabel) ([]byte, int) {
		t.Helper()
		if err := offscreen.Set(nil, []platform.TextLabel{label}, nil); err != nil {
			t.Fatal(err)
		}
		got, width, _, err := offscreen.Render()
		if err != nil {
			t.Fatal(err)
		}
		return got, width
	}
	lit := func(got []byte, width, x, y int) bool {
		blue, green, red, alpha := pixelAt(got, width, x*2, y*2) // logical to physical
		return alpha > 200 && (red > 200 || green > 200 || blue > 200)
	}

	got, width := render(label)
	if !lit(got, width, 5, 5) || !lit(got, width, 15, 5) || !lit(got, width, 5, 7) {
		t.Fatal("the unclipped label does not cover its bounds")
	}

	label.Clip, label.Clipped = ui.Rect{X: 0, Y: 0, Width: 10, Height: 6}, true
	got, width = render(label)
	if !lit(got, width, 5, 5) {
		t.Fatal("the clipped label lost the part inside its clip")
	}
	if lit(got, width, 15, 5) {
		t.Fatal("the clipped label painted right of its clip")
	}
	if lit(got, width, 5, 7) {
		t.Fatal("the clipped label painted below its clip")
	}

	// A clip with no area paints nothing, and the next frame without a clip
	// paints the whole label again: the scissor does not leak.
	label.Clip = ui.Rect{X: 30, Y: 0, Width: 5, Height: 5}
	got, width = render(label)
	if lit(got, width, 5, 5) || lit(got, width, 15, 5) {
		t.Fatal("a clip outside the target still painted")
	}
	label.Clipped = false
	got, width = render(label)
	if !lit(got, width, 15, 5) {
		t.Fatal("the scissor stayed set for the next frame")
	}
}

// clipScene renders the instances at the given scale on a size x size scene
// cleared to black and returns the pixels.
func clipScene(t *testing.T, size int, scale float64, rectangles []render.Rectangle) (pixels []byte, width int) {
	t.Helper()
	offscreen, err := NewOffscreen(size, size, scale, Color{Alpha: 1})
	if err != nil {
		t.Skipf("no Metal device: %v", err)
	}
	defer offscreen.Close()
	if err := offscreen.Set(rectangles, nil, nil); err != nil {
		t.Fatal(err)
	}
	pixels, width, _, err = offscreen.Render()
	if err != nil {
		t.Fatal(err)
	}
	return pixels, width
}

func isPainted(pixels []byte, width, x, y int) bool {
	blue, green, red, _ := pixelAt(pixels, width, x, y)
	return int(blue)+int(green)+int(red) > 60
}

func TestOffscreenClipCutsARoundedRectangleOnlyWhereTheClipIs(t *testing.T) {
	white := scene.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	// The rectangle spans 10..90; the clip cuts its right and bottom parts away
	// at 60 but leaves its top-left corner (radius 20) in place.
	pixels, width := clipScene(t, 100, 1, []render.Rectangle{{
		Bounds: scene.Rect{X: 10, Y: 10, Width: 80, Height: 80}, Radius: 20, Color: white,
		Clipped: true, Clip: scene.Rect{X: 0, Y: 0, Width: 60, Height: 60},
	}})
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			if (x >= 60 || y >= 60) && isPainted(pixels, width, x, y) {
				t.Fatalf("pixel (%d,%d) outside the clip is painted", x, y)
			}
		}
	}
	if !isPainted(pixels, width, 50, 50) || !isPainted(pixels, width, 59, 59) {
		t.Fatal("the part inside the clip is missing")
	}
	if isPainted(pixels, width, 11, 11) {
		t.Fatal("the uncut top-left corner is not rounded")
	}
	if !isPainted(pixels, width, 30, 30) {
		t.Fatal("the rectangle's interior is missing")
	}
}

func TestOffscreenClipCutsATriangle(t *testing.T) {
	white := scene.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	pixels, width := clipScene(t, 100, 1, []render.Rectangle{{
		Kind: scene.KindTriangle, Color: white, EdgeMask: 7,
		Points:  [3]scene.Point{{X: 10, Y: 10}, {X: 90, Y: 10}, {X: 10, Y: 90}},
		Clipped: true, Clip: scene.Rect{X: 0, Y: 0, Width: 50, Height: 50},
	}})
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			if (x >= 50 || y >= 50) && isPainted(pixels, width, x, y) {
				t.Fatalf("pixel (%d,%d) outside the clip is painted", x, y)
			}
		}
	}
	if !isPainted(pixels, width, 25, 25) {
		t.Fatal("the part of the triangle inside the clip is missing")
	}
}

func TestOffscreenClipsOnOneLayerAreIndependentAndKeepPaintOrder(t *testing.T) {
	red := scene.Color{Red: 1, Alpha: 1}
	green := scene.Color{Green: 1, Alpha: 1}
	whole := scene.Rect{X: 0, Y: 0, Width: 100, Height: 100}
	pixels, width := clipScene(t, 100, 1, []render.Rectangle{
		{Bounds: whole, Color: red, Clipped: true, Clip: scene.Rect{X: 0, Y: 0, Width: 40, Height: 100}},
		{Bounds: whole, Color: red, Clipped: true, Clip: scene.Rect{X: 60, Y: 0, Width: 40, Height: 100}},
		// A later, unclipped rectangle paints over the earlier clipped ones.
		{Bounds: scene.Rect{X: 20, Y: 20, Width: 20, Height: 20}, Color: green},
	})
	if isPainted(pixels, width, 50, 50) {
		t.Fatal("the gap between the two clips is painted")
	}
	if _, _, r, _ := pixelAt(pixels, width, 10, 80); r < 250 {
		t.Fatal("the first clip's rectangle is missing")
	}
	if _, _, r, _ := pixelAt(pixels, width, 90, 80); r < 250 {
		t.Fatal("the second clip's rectangle is missing")
	}
	if b, g, r, _ := pixelAt(pixels, width, 30, 30); g < 250 || r > 5 || b > 5 {
		t.Fatalf("the later rectangle does not paint over the earlier one: BGR %d,%d,%d", b, g, r)
	}
}

func TestOffscreenClipLandsOnTheNearestDevicePixelAtTwoTimes(t *testing.T) {
	white := scene.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	// At 2x the clip edges 10.3 and 20.6 are device pixels 20.6 and 41.2, which
	// round to 21 and 41: pixels 21..40 are painted and 20 and 41 are not.
	pixels, width := clipScene(t, 50, 2, []render.Rectangle{{
		Bounds: scene.Rect{X: 0, Y: 0, Width: 50, Height: 50}, Color: white,
		Clipped: true, Clip: scene.Rect{X: 10.3, Y: 10.3, Width: 10.3, Height: 10.3},
	}})
	for _, test := range []struct {
		x, y int
		want bool
	}{
		{20, 30, false}, {21, 30, true}, {40, 30, true}, {41, 30, false},
		{30, 20, false}, {30, 21, true}, {30, 40, true}, {30, 41, false},
	} {
		if got := isPainted(pixels, width, test.x, test.y); got != test.want {
			t.Errorf("pixel (%d,%d) painted = %v, want %v", test.x, test.y, got, test.want)
		}
	}
}

func TestOffscreenEditorIsCoveredByAHigherLayerAndCoversALowerOne(t *testing.T) {
	render := func(editorLayer uint16) int {
		offscreen, err := NewOffscreen(200, 100, 1, Color{Alpha: 1})
		if err != nil {
			t.Skipf("no Metal device: %v", err)
		}
		defer offscreen.Close()
		// An opaque popup on layer 2 over the field's area.
		popup := []render.Rectangle{{
			Kind: scene.KindRectangle, Bounds: scene.Rect{X: 90, Y: 40, Width: 100, Height: 40},
			Color: scene.Color{Blue: 0.2, Alpha: 1}, Layer: 2,
		}}
		input := &TextInput{
			X: 100, Y: 50, Width: 90, Height: 24, FontSize: 14, Layer: editorLayer,
			Color:   platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
			State:   TextState{Value: "abc"},
			OnEvent: func(TextEvent) (TextState, error) { return TextState{}, nil },
		}
		if err := offscreen.Set(popup, nil, input); err != nil {
			t.Fatal(err)
		}
		pixels, width, _, err := offscreen.Render()
		if err != nil {
			t.Fatal(err)
		}
		lit := 0
		for y := 45; y < 80; y++ {
			for x := 95; x < 190; x++ {
				if b, g, r, _ := pixelAt(pixels, width, x, y); r > 150 && g > 150 && b > 150 {
					lit++
				}
			}
		}
		return lit
	}
	if covered := render(1); covered != 0 {
		t.Errorf("the editor on layer 1 shows through a popup on layer 2 (%d lit pixels)", covered)
	}
	if above := render(3); above < 20 {
		t.Errorf("the editor on layer 3 drew %d lit pixels over a popup on layer 2, want its glyphs", above)
	}
}

// A shadow darkens only what lies outside its caster, fades to nothing within
// the reach it declares, and is deeper on the side its offset pushes it to.
func TestOffscreenShadowFadesAwayOutsideItsCaster(t *testing.T) {
	offscreen, err := NewOffscreen(200, 200, 1, Color{Red: 1, Green: 1, Blue: 1, Alpha: 1})
	if err != nil {
		t.Skipf("no Metal device: %v", err)
	}
	defer offscreen.Close()
	shadow := render.Rectangle{
		Kind: scene.KindShadow, Bounds: scene.Rect{X: 60, Y: 60, Width: 80, Height: 80}, Radius: 8,
		Color: scene.Color{Alpha: 0.5}, Shadow: scene.Shadow{OffsetY: 4, Blur: 12},
	}
	if err := offscreen.Set([]render.Rectangle{shadow}, nil, nil); err != nil {
		t.Fatal(err)
	}
	pixels, width, _, err := offscreen.Render()
	if err != nil {
		t.Fatal(err)
	}
	level := func(x, y int) int {
		_, green, _, _ := pixelAt(pixels, width, x, y)
		return int(green)
	}
	if level(100, 100) != 255 {
		t.Errorf("the caster's inside = %d, want it left untouched", level(100, 100))
	}
	if level(100, 143) > 200 {
		t.Errorf("just under the caster = %d, want a clearly darkened pixel", level(100, 143))
	}
	if level(100, 170) != 255 || level(30, 100) != 255 || level(100, 30) != 255 {
		t.Errorf("beyond the reach: %d %d %d, want untouched", level(100, 170), level(30, 100), level(100, 30))
	}
	for y := 141; y < 160; y++ {
		if level(100, y) > level(100, y+1) {
			t.Errorf("shadow below the caster is not fading away at y=%d: %d then %d", y, level(100, y), level(100, y+1))
		}
	}
	if level(100, 143) >= level(100, 57) {
		t.Errorf("shadow below (%d) is not deeper than above (%d) although it is offset downwards", level(100, 143), level(100, 57))
	}
}

// The same screen with every ASCII run shaped for ligatures: the cost of
// allowing substitutions on a full redraw.
func BenchmarkRasterizeTerminalScreenLigatures(b *testing.B) {
	offscreen, err := NewOffscreen(940, 640, 2, Color{Alpha: 1})
	if err != nil {
		b.Skipf("no Metal device: %v", err)
	}
	defer offscreen.Close()
	labels := gridLabels(false)
	for row := range labels {
		for run := range labels[row].Runs {
			labels[row].Runs[run].Ligatures = true
		}
	}
	b.ReportAllocs()
	for b.Loop() {
		if err := offscreen.Set(nil, nil, nil); err != nil {
			b.Fatal(err)
		}
		if err := offscreen.Set(nil, labels, nil); err != nil {
			b.Fatal(err)
		}
	}
}
