//go:build darwin && vivahead

package headless

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func TestCodeEditorDecorationsPixels(t *testing.T) {
	for _, size := range []struct {
		name          string
		width, height int
	}{{"desktop", 1280, 800}, {"compact", 640, 400}} {
		t.Run(size.name, func(t *testing.T) {
			var editor components.CodeEditor
			var invalidator app.Invalidator
			theme := gfx.LightTheme()
			green, orange, cyan := gfx.RGB(0, 160, 0), gfx.RGB(255, 128, 0), gfx.RGB(64, 160, 191)
			decorations := &components.CodeEditorDecorations{
				GutterWidth: 9, GutterBackground: gfx.RGB(224, 224, 224),
				GutterBars: []components.CodeEditorGutterBar{{Line: 1, Fill: green, Stroke: green}},
				IndicatorLayers: []components.CodeEditorIndicatorLayer{
					{Spans: []text.Span{{Start: 7, End: 9}}, Color: orange},
					{Spans: []text.Span{{Start: 10, End: 10}}, Color: cyan, Style: components.CodeEditorPointAbove},
				},
			}
			var cell, line float32
			config := app.Config{
				Title: size.name, Width: size.width, Height: size.height,
				Invalidator: &invalidator, ClearColor: app.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
				Paint: func(canvas *gfx.Canvas, frame gfx.Size) {
					var height float32
					cell, height = canvas.MonoCellSize(14, gfx.MonoSystem)
					line = max(height, 21)
					editor.Paint(canvas, theme, gfx.Rect{Width: frame.Width, Height: frame.Height}, components.CodeEditorSnapshot{
						ID: gfx.Item(1), Label: "Code editor", Value: "alpha\nbravo\ncharlie\n", FontSize: 14,
						CurrentLine: true, Decorations: decorations,
					})
				},
			}
			session, err := Open(config, nil, Options{Scale: 1})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(session.Close)
			picture, err := session.Image()
			if err != nil {
				t.Fatal(err)
			}
			if picture.Bounds().Dx() != size.width || picture.Bounds().Dy() != size.height {
				t.Fatalf("screenshot dimensions = %v", picture.Bounds())
			}
			content := decorationContentBounds(t, session.Snapshot())
			if content.Width <= 0 || cell <= 0 || line <= 0 {
				t.Fatal("the editor did not publish text geometry")
			}
			barPixels, underlinePixels, pointPixels := 0, 0, 0
			for y := 0; y < picture.Bounds().Dy(); y++ {
				for x := 0; x < picture.Bounds().Dx(); x++ {
					r, g, b := pixelAt(picture, x, y)
					switch [3]uint8{r, g, b} {
					case [3]uint8{0, 160, 0}:
						barPixels++
						if float32(x) >= content.X {
							t.Fatalf("gutter bar covers text at (%d,%d)", x, y)
						}
					case [3]uint8{255, 128, 0}:
						underlinePixels++
						if float32(x) < content.X+cell || float32(y) < content.Y+2*line-2 || float32(y) >= content.Y+2*line {
							t.Fatalf("byte underline has glyph-covering or wrong-position pixels at (%d,%d), content=%+v cell=%v line=%v", x, y, content, cell, line)
						}
					case [3]uint8{64, 160, 191}:
						pointPixels++
					}
				}
			}
			if barPixels == 0 || underlinePixels == 0 || pointPixels == 0 {
				t.Fatalf("blank decorations: bar=%d underline=%d point=%d", barPixels, underlinePixels, pointPixels)
			}
			x := int(math.Floor(float64(theme.Metrics.TextInsetX()))) + 1
			y := int(content.Y + line/2)
			if r, g, b := pixelAt(picture, x, y); r != 224 || g != 224 || b != 224 {
				t.Fatalf("empty lane background was tinted: %d,%d,%d", r, g, b)
			}
			if output := os.Getenv("VIVA_DECORATIONS_SHOTS"); output != "" {
				if err := os.MkdirAll(output, 0700); err != nil {
					t.Fatal(err)
				}
				if err := session.Screenshot(filepath.Join(output, "code-decorations-"+size.name+".png")); err != nil {
					t.Fatal(err)
				}
			}
			decorations.GutterBars, decorations.IndicatorLayers = nil, nil
			invalidator.Invalidate()
			if err := session.Driver().Pump(); err != nil {
				t.Fatal(err)
			}
			plain, err := session.Image()
			if err != nil {
				t.Fatal(err)
			}
			if decorationContentBounds(t, session.Snapshot()) != content {
				t.Fatal("clearing change markers shifted text")
			}
			// The first letter of bravo has no indicator. Its glyph body must stay
			// identical when the later byte span and deletion point are removed.
			for y := int(content.Y + line*1.25); y < int(content.Y+line*1.75); y++ {
				for x := int(content.X); x < int(content.X+cell); x++ {
					if picture.RGBAAt(x, y) != plain.RGBAAt(x, y) {
						t.Fatalf("indicator obscured the unmarked first glyph at (%d,%d)", x, y)
					}
				}
			}
			t.Logf("%dx%d: bar=%d underline=%d point=%d pixels; clearing decorations preserved text geometry and unmarked glyphs", size.width, size.height, barPixels, underlinePixels, pointPixels)
		})
	}
}

func decorationContentBounds(t *testing.T, snapshot app.Snapshot) gfx.Rect {
	t.Helper()
	for index, hit := range snapshot.Hits {
		if hit.ID == gfx.Item(1) {
			if geometry, ok := snapshot.HitTextAt(index); ok {
				return geometry.Bounds
			}
		}
	}
	t.Fatal("the editor has no declared text geometry")
	return gfx.Rect{}
}
