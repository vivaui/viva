//go:build darwin && vivahead

package headless

import (
	"fmt"
	"image"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// A Mono label is drawn on the cells TextWidthStyle measures it by: pixel for pixel
// as the LabelRuns line of the same text, at sizes where the cell, a whole number of
// device pixels, is wider or narrower than the font's own advance. Right-aligned
// numbers end where their grid lines do, and a line longer than the 256 characters
// macOS draws glyph by glyph at once stays on its cells to the end. In an installed
// family the line is the grid line whose stretches of ASCII have Ligatures.
func TestHeadlessMonoLabelIsDrawnLikeItsGridLine(t *testing.T) {
	white := ui.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	short, long := strings.Repeat("0123456789", 6), strings.Repeat("0123456789", 30)
	numbers := []string{"7", "42", "1024", "65536"}
	menlo := ui.MonoFamily("Menlo")
	for _, test := range []struct {
		name  string
		scale float64
		size  float32
		line  string
		face  ui.MonoFace
	}{
		{"14 at 1x", 1, 14, short, ui.MonoSystem},
		{"15 at 1x", 1, 15, short, ui.MonoSystem},
		{"12 at 2x", 2, 12, short, ui.MonoSystem},
		{"15 at 2x", 2, 15, short, ui.MonoSystem},
		{"300 characters", 1, 14, long, ui.MonoSystem},
		{"an installed family at 14 at 1x", 1, 14, short, menlo},
		{"an installed family at 15 at 2x", 2, 15, short, menlo},
	} {
		t.Run(test.name, func(t *testing.T) {
			style := ui.TextStyle{Size: test.size, Color: white, Mono: true, MonoFace: test.face}
			ligatures := test.face == menlo
			// The grid line of the text, in runs of 200 characters as a terminal row
			// of several colors would have them.
			var runs []ui.TextRun
			for column := 0; column < len(test.line); column += 200 {
				count := int32(min(200, len(test.line)-column))
				runs = append(runs, ui.TextRun{Column: int32(column), Bytes: count, Cells: count, Color: white, Ligatures: ligatures})
			}
			session, err := Open(app.Config{
				Title: "mono grid", Width: int(float32(len(test.line))*test.size*0.7) + 20, Height: 40 * (1 + len(numbers)),
				ClearColor: app.Color{Alpha: 1},
				Paint: func(canvas *ui.Canvas, _ ui.Size) {
					cell, _ := canvas.MonoCellSize(style.Size, style.MonoFace)
					width := cell * float32(len(test.line))
					canvas.Label(test.line, ui.Rect{X: 10, Width: width, Height: 20}, style)
					canvas.LabelRuns(test.line, ui.Rect{X: 10, Y: 20, Width: width, Height: 20}, style, cell, runs)
					for index, number := range numbers {
						y := float32(40 * (1 + index))
						span := cell * float32(len(number))
						canvas.LabelAligned(number, ui.Rect{X: 10, Y: y, Width: width, Height: 20}, style, ui.AlignEnd)
						canvas.LabelRuns(number, ui.Rect{X: 10 + (width - span), Y: y + 20, Width: span, Height: 20}, style, cell,
							[]ui.TextRun{{Bytes: int32(len(number)), Cells: int32(len(number)), Color: white, Ligatures: ligatures && len(number) > 1}})
					}
				},
			}, nil, Options{Scale: test.scale})
			if err != nil {
				t.Fatal(err)
			}
			defer session.Close()
			picture, err := session.Image()
			if err != nil {
				t.Fatal(err)
			}
			row := int(20 * test.scale)
			for index, value := range append([]string{test.line}, numbers...) {
				label := 2 * index * row
				if first, _ := inkColumns(picture, label, label+row); first < 0 {
					t.Fatalf("%q: no ink in the label's row", value)
				}
				if x, y, same := sameRows(picture, label, label+row, row); !same {
					first, last := inkColumns(picture, label, label+row)
					gridFirst, gridLast := inkColumns(picture, label+row, label+2*row)
					t.Errorf("%q: the label differs from its grid line at %d,%d: ink %d..%d, the grid line's %d..%d",
						value, x, y, first, last, gridFirst, gridLast)
				}
			}
		})
	}
}

func TestHeadlessMonoLabelShapesWholeClusters(t *testing.T) {
	white := ui.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	style := ui.TextStyle{Size: 20, Color: white, Mono: true}
	for _, value := range []string{"👩‍💻", "🇨🇳", "क्ष"} {
		t.Run(value, func(t *testing.T) {
			session, err := Open(app.Config{
				Title: "mono clusters", Width: 220, Height: 48,
				ClearColor: app.Color{Alpha: 1},
				Paint: func(canvas *ui.Canvas, _ ui.Size) {
					cell, _ := canvas.MonoCellSize(style.Size, style.MonoFace)
					cells := int32(canvas.TextWidthStyle(value, style) / cell)
					canvas.Label(value, ui.Rect{X: 10, Width: 200, Height: 24}, style)
					canvas.LabelRuns(value, ui.Rect{X: 10, Y: 24, Width: 200, Height: 24}, style, cell,
						[]ui.TextRun{{Bytes: int32(len(value)), Cells: cells, Color: white}})
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
			if first, _ := inkColumns(picture, 0, 48); first < 0 {
				t.Fatal("no ink in the label's row")
			}
			if x, y, same := sameRows(picture, 0, 48, 48); !same {
				t.Errorf("%q differs from its grid line at %d,%d", value, x, y)
			}
		})
	}
}

// sameRows reports whether the height rows of picture from top are the same pixels as
// those from other, and where they first differ.
func sameRows(picture *image.RGBA, top, other, height int) (x, y int, same bool) {
	for y := 0; y < height; y++ {
		for x := 0; x < picture.Bounds().Dx(); x++ {
			if picture.RGBAAt(x, top+y) != picture.RGBAAt(x, other+y) {
				return x, top + y, false
			}
		}
	}
	return 0, 0, true
}

// inkColumns is the first and last column of picture with a light pixel between top
// and bottom, or -1 and -1.
func inkColumns(picture *image.RGBA, top, bottom int) (first, last int) {
	first, last = -1, -1
	for x := 0; x < picture.Bounds().Dx(); x++ {
		for y := top; y < bottom; y++ {
			if picture.RGBAAt(x, y).R > 0x40 {
				if first < 0 {
					first = x
				}
				last = x
				break
			}
		}
	}
	return first, last
}

// BenchmarkHeadlessScreenshotChangingMonoLabels renders frames of 24 Mono labels whose
// text changes every frame, so every label is rasterized again each time, in the system
// face and in an installed family.
func BenchmarkHeadlessScreenshotChangingMonoLabels(b *testing.B) {
	for _, face := range []struct {
		name string
		face ui.MonoFace
	}{{"system", ui.MonoSystem}, {"family", ui.MonoFamily("Menlo")}} {
		b.Run(face.name, func(b *testing.B) {
			white := ui.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
			var frame int
			session, err := Open(app.Config{
				Title: "mono labels", Width: 640, Height: 480, ClearColor: app.Color{Alpha: 1},
				Paint: func(canvas *ui.Canvas, _ ui.Size) {
					style := ui.TextStyle{Size: 13, Color: white, Mono: true, MonoFace: face.face}
					for row := 0; row < 24; row++ {
						canvas.Label(fmt.Sprintf("%6d  func (editor *Editor) column(index, tabWidth int) int {", frame+row),
							ui.Rect{X: 10, Y: float32(row * 20), Width: 620, Height: 20}, style)
					}
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
				frame = index + 1
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
