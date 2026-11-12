//go:build darwin && vivahead

package headless

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

func TestScreenshotDrawsBothEndsOfCompressedWideQuad(t *testing.T) {
	white := gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	line := strings.Repeat("M", 500) + strings.Repeat(" ", 2000) + strings.Repeat("8", 500)
	var width float32
	session, err := Open(app.Config{
		Title: "compressed wide quad", Width: 220, Height: 120, ClearColor: app.Color{Alpha: 1},
		Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
			canvas.Clip(gfx.Rect{Width: 220, Height: 120})
			style := gfx.TextStyle{Size: 14, Color: white}
			width = canvas.TextWidthStyle(line, style)
			canvas.LabelQuad(line, gfx.Rect{X: -width / 2, Y: 10, Width: width, Height: 24}, gfx.Quad{
				Points: [4]gfx.Point{{X: 10, Y: 70}, {X: 210, Y: 20}, {X: 10, Y: 94}, {X: 210, Y: 44}},
				Tint:   [4]gfx.Color{white, white, white, white},
			}, style)
		},
	}, nil, Options{Scale: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	picture, err := session.Image()
	if err != nil {
		t.Fatal(err)
	}
	saveWarpedShot(t, session, "wide-compressed.png")
	if width*2 <= 16384 {
		t.Fatalf("source width %.0f is not over the texture limit", width*2)
	}
	for _, area := range []image.Rectangle{image.Rect(20, 100, 85, 188), image.Rect(355, 40, 420, 100)} {
		if count := warpedInk(picture, area); count < 30 {
			t.Errorf("source end at %v has only %d ink pixels; both ends must survive the warp", area, count)
		}
	}
}

func TestScreenshotRefreshesSourceWindowWhenOnlyWideQuadMoves(t *testing.T) {
	white := gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	line := strings.Repeat("M", 1000) + strings.Repeat(" ", 2000) + strings.Repeat("8", 1000)
	var offset, tail float32
	session, err := Open(app.Config{
		Title: "moving wide quad", Width: 240, Height: 90, ClearColor: app.Color{Alpha: 1},
		Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
			canvas.Clip(gfx.Rect{Width: 240, Height: 90})
			style := gfx.TextStyle{Size: 14, Color: white}
			width := canvas.TextWidthStyle(line, style)
			tail = canvas.TextWidthStyle(strings.Repeat("M", 1000)+strings.Repeat(" ", 2000), style)
			canvas.LabelQuad(line, gfx.Rect{X: 100000, Y: 10, Width: width, Height: 24}, gfx.Quad{
				Points: [4]gfx.Point{{X: offset, Y: 20}, {X: offset + width, Y: 30}, {X: offset, Y: 44}, {X: offset + width, Y: 54}},
				Tint:   [4]gfx.Color{white, white, white, white},
			}, style)
		},
		OnScroll: func(gfx.ScrollEvent) gfx.EventResult { return gfx.EventResult{Handled: true, Repaint: true} },
	}, nil, Options{Scale: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	for index := range 3 {
		if index == 1 {
			offset = -tail - 20
		} else {
			offset = 0
		}
		if index != 0 {
			if err := session.Scroll(gfx.ScrollEvent{X: 10, Y: 20, DeltaY: -1}); err != nil {
				t.Fatal(err)
			}
		}
		picture, err := session.Image()
		if err != nil {
			t.Fatal(err)
		}
		if index == 1 {
			saveWarpedShot(t, session, "wide-moved-tail.png")
		}
		for _, area := range []image.Rectangle{image.Rect(10, 35, 100, 115), image.Rect(370, 35, 460, 115)} {
			if count := warpedInk(picture, area); count < 100 {
				t.Errorf("move %d: visible source at %v has only %d ink pixels", index, area, count)
			}
		}
	}
}

func warpedInk(picture *image.RGBA, area image.Rectangle) int {
	count := 0
	for row := area.Min.Y; row < area.Max.Y; row++ {
		for column := area.Min.X; column < area.Max.X; column++ {
			if picture.RGBAAt(column, row).R > 30 {
				count++
			}
		}
	}
	return count
}

func saveWarpedShot(t *testing.T, session *Session, name string) {
	t.Helper()
	if directory := os.Getenv("VIVA_WARPED_SHOTS"); directory != "" {
		if err := session.Screenshot(filepath.Join(directory, name)); err != nil {
			t.Fatal(err)
		}
	}
}
