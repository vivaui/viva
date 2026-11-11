package ui

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
)

// cellCanvas is runeCanvas with a fixed-pitch cell half the size wide, so a Mono width is
// a count of cells and differs from the proportional one wherever a wide character sits.
func cellCanvas() *gfx.Canvas {
	canvas := runeCanvas()
	canvas.SetCellMeasurer(func(size float32, _ gfx.MonoFace) (float32, float32) { return size / 2, size * 1.2 })
	return canvas
}

// monoLines draws a Mono paragraph of at most maxLines lines width wide, at a size whose
// cell is 5, and returns what was painted and the width and height the layout gave it.
func monoLines(t *testing.T, text string, width float32, maxLines int) ([]string, gfx.Rect) {
	t.Helper()
	var view View
	canvas := cellCanvas()
	var box Box
	f := view.Begin(canvas, gfx.LightTheme(), gfx.Rect{Width: width, Height: 600})
	f.Column().Children(func() {
		box = f.Text(text).Mono().FontSize(10).MaxLines(maxLines)
	})
	f.End()
	var lines []string
	for _, painted := range canvas.Texts() {
		lines = append(lines, painted.Value)
	}
	return lines, box.Rect()
}

// A Mono Text is as wide as the cells it is drawn on: "分离的 HEAD" is eight characters but
// eleven cells, three of them wide, so a row gives it eleven cells and not eight.
func TestAMonoTextIsAsWideAsItsCells(t *testing.T) {
	var view View
	canvas := cellCanvas()
	var mono, plain Box
	f := view.Begin(canvas, gfx.LightTheme(), gfx.Rect{Width: 400, Height: 100})
	f.Row().Children(func() {
		mono = f.Text("分离的 HEAD").Mono().FontSize(10)
		plain = f.Text("分离的 HEAD").FontSize(10)
	})
	f.End()
	if got := mono.Rect().Width; got != 55 {
		t.Fatalf("Mono text is %v wide, want 55 (11 cells of 5)", got)
	}
	if got := plain.Rect().Width; got != 80 {
		t.Fatalf("proportional text is %v wide, want 80 (8 characters of 10)", got)
	}
}

// A Mono paragraph breaks where its cells fill the line and cuts its last line to the cells
// left beside the ellipsis, so no line is drawn past the width it was given.
func TestAMonoParagraphBreaksAndCutsOnItsCells(t *testing.T) {
	lines, rect := monoLines(t, "分离的 HEAD", 40, 0)
	if strings.Join(lines, "|") != "分离的|HEAD" {
		t.Fatalf("eight cells wide: painted %q, want 分离的|HEAD", lines)
	}
	if rect.Height != 2*(10+7) {
		t.Fatalf("two lines take %v, want %v", rect.Height, 2*(10+7))
	}
	// Eight characters fit in ten cells' width by count, but the text is eleven cells.
	if lines, _ = monoLines(t, "分离的 HEAD", 50, 1); len(lines) != 1 || !strings.HasSuffix(lines[0], "…") {
		t.Fatalf("ten cells wide, one line: painted %q, want it cut", lines)
	}
	for _, width := range []float32{30, 50, 60, 85} {
		lines, _ = monoLines(t, "分离的 HEAD · 新仓库", width, 2)
		canvas := cellCanvas()
		for _, line := range lines {
			if got := canvas.TextWidthStyle(line, gfx.TextStyle{Size: 10, Mono: true}); got > width {
				t.Fatalf("%v wide: line %q of %q is %v wide", width, line, lines, got)
			}
		}
	}
}

// A page of Mono paragraphs, wrapped and cut, builds nothing once warm, as a proportional
// one does not.
func TestWrappedMonoTextAllocatesNothingOnceWarm(t *testing.T) {
	var view View
	canvas := cellCanvas()
	theme := gfx.LightTheme()
	run := func() {
		canvas.Reset()
		view.Draw(canvas, theme, gfx.Rect{Width: 120, Height: 300}, func(f *Frame) {
			f.Column().Fill().Pad(4).Gap(4).Children(func() {
				f.Wrap("一段很长很长的中文，需要不止两行才能放下，所以会被截断。").Mono().FontSize(10)
				f.Text("fix(desktop): 编辑器滚到最后一行贴底即停止，并让缩略图与滚动条对齐").Mono().FontSize(10).MaxLines(2)
				f.Text("分离的 HEAD · 新仓库 · feature/1794-viva-workbench-shell").Mono().FontSize(10).Ellipsis()
			})
		})
	}
	run()
	if allocs := testing.AllocsPerRun(200, run); allocs != 0 {
		t.Fatalf("a warm page of wrapped and ellipsized Mono text allocates %v times a frame, want 0", allocs)
	}
}

func BenchmarkWrappedMonoParagraphs(b *testing.B) {
	var view View
	canvas := cellCanvas()
	theme := gfx.LightTheme()
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		view.Draw(canvas, theme, gfx.Rect{Width: 160, Height: 600}, func(f *Frame) {
			f.Column().Fill().Gap(4).Children(func() {
				for range 20 {
					f.Wrap("连接服务器失败，请检查网络后重试。see /var/log/system/very-long-name.log").Mono().FontSize(10)
				}
			})
		})
	}
}
