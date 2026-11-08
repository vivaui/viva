//go:build darwin && vivahead

package headless

import (
	"fmt"
	"image"
	"math"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// A label wider than a Metal texture draws: the renderer rasterizes the part
// of it in view, and again when the label moves so another part is in view.
func TestHeadlessScreenshotDrawsLabelWiderThanATexture(t *testing.T) {
	const scale = 2
	head, blank, tail := strings.Repeat("8", 100), strings.Repeat(" ", 3000), strings.Repeat("8", 1000)
	line := head + blank + tail
	white := ui.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	var left, gap, width float32
	config := app.Config{
		Title: "wide label", Width: 400, Height: 60, ClearColor: app.Color{Alpha: 1},
		Paint: func(canvas *ui.Canvas, _ ui.Size) {
			style := ui.TextStyle{Size: 14, Color: white}
			gap = canvas.TextWidthStyle(head+blank, style)
			width = canvas.TextWidthStyle(line, style)
			canvas.Label(line, ui.Rect{X: left, Y: 20, Width: width, Height: 20}, style)
		},
		OnScroll: func(ui.ScrollEvent) ui.EventResult { return ui.EventResult{Handled: true, Repaint: true} },
	}
	session, err := Open(config, nil, Options{Scale: scale})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	// ink reports the first and last device column with glyph pixels in the
	// label's row, or -1 when the row is empty.
	ink := func() (first, last int) {
		t.Helper()
		picture, err := session.Image()
		if err != nil {
			t.Fatal(err)
		}
		first, last = -1, -1
		for x := 0; x < picture.Bounds().Dx(); x++ {
			if column(picture, x, 20*scale, 40*scale) {
				if first < 0 {
					first = x
				}
				last = x
			}
		}
		return first, last
	}
	move := func(to float32) {
		t.Helper()
		left = to
		if err := session.Scroll(ui.ScrollEvent{X: 200, Y: 30, DeltaY: -1}); err != nil {
			t.Fatal(err)
		}
	}

	if width*scale <= 16384 {
		t.Fatalf("label is %.0f device pixels wide, not over the texture limit", width*scale)
	}
	if first, last := ink(); first < 0 || first > 4 || last < 400*scale-4 {
		t.Fatalf("start of the label: ink %d..%d, want it across the window", first, last)
	}
	// Move the label so that the run of eights after the blank starts 100
	// pixels into the window: far outside the part rasterized for the first frame.
	// A move by whole pixels keeps the label's texture, so the renderer has to
	// see that its part no longer covers the window.
	move(100 - float32(math.Floor(float64(gap))))
	if first, last := ink(); first < 100*scale-2 || first > 100*scale+8 || last < 400*scale-4 {
		t.Fatalf("eights after the blank: ink %d..%d, want %d..%d", first, last, 100*scale, 400*scale)
	}
	move(0)
	if first, last := ink(); first < 0 || first > 4 || last < 400*scale-4 {
		t.Fatalf("back at the start: ink %d..%d, want it across the window", first, last)
	}
}

func TestHeadlessScreenshotDrawsEntireWideViewportAtFractionalScale(t *testing.T) {
	const viewportWidth = 9000
	text := strings.Repeat("8", 1800)
	var left, width float32
	session, err := Open(app.Config{
		Title: "wide viewport", Width: viewportWidth, Height: 36, ClearColor: app.Color{Alpha: 1},
		Paint: func(canvas *ui.Canvas, _ ui.Size) {
			style := ui.TextStyle{Size: 14, Color: ui.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}}
			width = canvas.TextWidthStyle(text, style)
			canvas.Label(text, ui.Rect{X: left, Width: width, Height: 30}, style)
		},
		OnScroll: func(ui.ScrollEvent) ui.EventResult { return ui.EventResult{Handled: true, Repaint: true} },
	}, nil, Options{Scale: 1.5})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	for _, offset := range []float32{0, -1000} {
		left = offset
		if offset != 0 {
			if err := session.Scroll(ui.ScrollEvent{X: 10, Y: 10, DeltaY: -1}); err != nil {
				t.Fatal(err)
			}
		}
		picture, err := session.Image()
		if err != nil {
			t.Fatal(err)
		}
		if width <= viewportWidth-left {
			t.Fatalf("label width %.0f does not cover the viewport at offset %.0f", width, left)
		}
		for _, start := range []int{100, 12000, 12500, picture.Bounds().Dx() - 150} {
			visible := false
			for position := start; position < start+100; position++ {
				if column(picture, position, 0, picture.Bounds().Dy()) {
					visible = true
					break
				}
			}
			if !visible {
				t.Fatalf("label at offset %.0f has no ink in columns %d..%d", offset, start, start+100)
			}
		}
	}
}

// column says whether a column of picture has a light pixel between top and bottom.
func column(picture *image.RGBA, x, top, bottom int) bool {
	for y := top; y < bottom; y++ {
		red, _, _, _ := picture.At(x, y).RGBA()
		if red > 0x8000 {
			return true
		}
	}
	return false
}

// BenchmarkHeadlessScreenshotMovingLabels renders frames whose labels move by
// a whole pixel each time, so every frame keeps their textures: ordinary labels,
// and one label wider than a texture scrolled 40 pixels a frame.
func BenchmarkHeadlessScreenshotMovingLabels(b *testing.B) {
	white := ui.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	wide := strings.Repeat("0123456789", 400)
	for _, test := range []struct {
		name string
		step float32
		draw func(canvas *ui.Canvas, left float32, style ui.TextStyle)
	}{
		{"ordinary", 1, func(canvas *ui.Canvas, left float32, style ui.TextStyle) {
			for row := 0; row < 24; row++ {
				canvas.Label(fmt.Sprintf("%02d the quick brown fox jumps over the lazy dog", row),
					ui.Rect{X: 10 + left, Y: float32(row * 20), Width: 320, Height: 20}, style)
			}
		}},
		{"wide", -40, func(canvas *ui.Canvas, left float32, style ui.TextStyle) {
			canvas.Label(wide, ui.Rect{X: left, Y: 20, Width: canvas.TextWidthStyle(wide, style), Height: 20}, style)
		}},
	} {
		b.Run(test.name, func(b *testing.B) {
			var left float32
			session, err := Open(app.Config{
				Title: "moving labels", Width: 640, Height: 480, ClearColor: app.Color{Alpha: 1},
				Paint: func(canvas *ui.Canvas, _ ui.Size) {
					test.draw(canvas, left, ui.TextStyle{Size: 13, Color: white})
				},
				OnScroll: func(ui.ScrollEvent) ui.EventResult { return ui.EventResult{Handled: true, Repaint: true} },
			}, nil, Options{Scale: 2})
			if err != nil {
				b.Fatal(err)
			}
			defer session.Close()
			if _, err := session.Image(); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				left = float32(index%400) * test.step
				if err := session.Scroll(ui.ScrollEvent{X: 200, Y: 30, DeltaY: -1}); err != nil {
					b.Fatal(err)
				}
				if _, err := session.Image(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
