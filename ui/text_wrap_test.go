package ui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/vivaui/viva/gfx"
)

// runeCanvas measures every rune as `size` wide, so a width is a count of characters.
func runeCanvas() *gfx.Canvas {
	canvas := &gfx.Canvas{}
	canvas.SetTree(&tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 { return float32(utf8.RuneCountInString(value)) * size })
	return canvas
}

// paintedLines draws a paragraph and returns what was painted and the height the
// layout gave it.
func paintedLines(t *testing.T, view *View, text string, width float32, maxLines int) ([]string, float32) {
	t.Helper()
	canvas := runeCanvas()
	var box Box
	f := view.Begin(canvas, gfx.LightTheme(), gfx.Rect{Width: width, Height: 600})
	f.Column().Children(func() {
		box = f.Text(text).FontSize(10).MaxLines(maxLines)
	})
	f.End()
	var lines []string
	for _, painted := range canvas.Texts() {
		lines = append(lines, painted.Value)
	}
	return lines, box.Rect().Height
}

func TestParagraphHeightIsTheNumberOfLinesPaintedForEveryKindOfText(t *testing.T) {
	var view View
	for _, text := range []string{
		"今天的天气非常好，我们一起去公园散步。然后回家吃饭，睡觉！",
		"see /usr/local/share/applications/firefox.desktop now",
		"a👨‍👩‍👧‍👦b🇯🇵🇺🇸 and ééé words and more words",
		"first\nsecond line is a bit longer\n\nfourth",
	} {
		for _, width := range []float32{70, 100, 140} { // 7, 10 and 14 characters of 10 points
			lines, height := paintedLines(t, &view, text, width, 0)
			if want := float32(len(lines)) * 17; height != want {
				t.Errorf("%q at %v: %d lines painted but the height is %v, want %v", text, width, len(lines), height, want)
			}
			for _, line := range lines {
				if r, _ := utf8.DecodeRuneInString(line); strings.ContainsRune("，。！", r) {
					t.Errorf("%q at %v: line %q starts with closing punctuation", text, width, line)
				}
			}
		}
	}
}

func TestMaxLinesEndsTheLastLineWithAnEllipsisAndWrapTakesAsManyAsItNeeds(t *testing.T) {
	var view View
	text := "一段很长很长的中文，需要不止两行才能放下，所以会被截断。"
	lines, height := paintedLines(t, &view, text, 100, 2)
	if len(lines) != 2 || !strings.HasSuffix(lines[1], "…") || height != 34 {
		t.Errorf("limited to two lines: %q, %v tall", lines, height)
	}
	if lines, _ = paintedLines(t, &view, text, 100, 0); len(lines) < 3 || strings.Contains(strings.Join(lines, ""), "…") != false {
		t.Errorf("unlimited: %q", lines)
	}
	if lines, _ = paintedLines(t, &view, text, 100, 1); len(lines) != 1 || !strings.HasSuffix(lines[0], "…") {
		t.Errorf("one line is Ellipsis: %q", lines)
	}
}

func TestWrappedTextAllocatesNothingOnceWarmEvenWhenCutOff(t *testing.T) {
	var view View
	canvas := runeCanvas()
	theme := gfx.LightTheme()
	run := func() {
		canvas.Reset()
		view.Draw(canvas, theme, gfx.Rect{Width: 120, Height: 300}, func(f *Frame) {
			f.Column().Fill().Pad(4).Gap(4).Children(func() {
				f.Wrap("一段很长很长的中文，需要不止两行才能放下，所以会被截断。").FontSize(10)
				f.Text("see /usr/local/share/applications/firefox.desktop and more text").FontSize(10).MaxLines(2)
				f.Text("a file name that is far too long to fit.tar.gz").FontSize(10).Ellipsis()
			})
		})
	}
	run()
	if allocs := testing.AllocsPerRun(200, run); allocs != 0 {
		t.Fatalf("a warm page of wrapped and ellipsized text allocates %v times a frame, want 0", allocs)
	}
}

// A paragraph of an empty string is no text at all: it paints no line and takes
// no height, as WrapLines gives it no lines.
func TestEmptyParagraphPaintsNoLineAndTakesNoHeight(t *testing.T) {
	var view View
	for _, maxLines := range []int{0, 1} {
		lines, height := paintedLines(t, &view, "", 100, maxLines)
		if len(lines) != 0 || height != 0 {
			t.Errorf("maxLines %d: painted %q and took %v, want nothing", maxLines, lines, height)
		}
	}
}

func BenchmarkWrappedParagraphs(b *testing.B) {
	var view View
	canvas := runeCanvas()
	theme := gfx.LightTheme()
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		view.Draw(canvas, theme, gfx.Rect{Width: 160, Height: 600}, func(f *Frame) {
			f.Column().Fill().Gap(4).Children(func() {
				for range 20 {
					f.Wrap("连接服务器失败，请检查网络后重试。see /var/log/system/very-long-name.log").FontSize(10)
				}
			})
		})
	}
}

// A chip, a badge or a chat bubble is a Text with Pad and Background: the padding takes
// room, the fill covers it and the text sits inside it, and a paragraph wraps to the width
// that is left once the padding is taken off.
func TestATextTakesItsPadAndBackground(t *testing.T) {
	var view View
	canvas := runeCanvas()
	var chip, bubble Box
	green := gfx.Color{Green: 1, Alpha: 1}
	f := view.Begin(canvas, gfx.LightTheme(), gfx.Rect{Width: 200, Height: 300})
	f.Column().Items(Start).Children(func() {
		chip = f.Text("new").FontSize(10).PadX(6).PadY(2).Background(green)
		bubble = f.Wrap("aaaa bbbb cccc dddd").FontSize(10).Pad(8).Width(80)
	})
	f.End()
	near(t, "chip", chip.Rect(), gfx.Rect{X: 0, Y: 0, Width: 42, Height: 21})
	// 80 wide less 16 of padding is 64 for the text: six characters per line, two words per line is too wide
	if got, want := bubble.Rect().Height, float32(4*17+16); got != want {
		t.Errorf("a padded paragraph is %v tall, want %v (four lines and 16 of padding)", got, want)
	}
	for _, painted := range canvas.Texts() {
		if painted.Value == "new" && (painted.Bounds.X < 6 || painted.Bounds.Y < 2) {
			t.Errorf("the chip's text starts at %v, %v: not inside its padding", painted.Bounds.X, painted.Bounds.Y)
		}
	}
}

// A Hebrew text starts at the right edge of its box and a paragraph of it keeps that for every
// line, including one that starts with an English word; TextEnd flips it.
func TestRightToLeftTextAndParagraphsAlignToTheRight(t *testing.T) {
	var view View
	canvas := runeCanvas()
	f := view.Begin(canvas, gfx.LightTheme(), gfx.Rect{Width: 200, Height: 400})
	f.Column().Items(Start).Children(func() {
		f.Text("שלום").FontSize(10).Width(100)
		f.Text("abc").FontSize(10).Width(100)
		f.Text("abc").FontSize(10).Width(100).TextEnd()
		f.Text("שלום").FontSize(10).Width(100).TextEnd()
		f.Text("שלום עולם abc שלום עולם שלום עולם שלום").FontSize(10).Width(100).MaxLines(5)
	})
	f.End()
	texts := canvas.Texts()
	if len(texts) < 6 {
		t.Fatalf("%d texts", len(texts))
	}
	rightOf := func(text gfx.Text) float32 { return text.Bounds.X + 10*float32(utf8.RuneCountInString(text.Value)) }
	box := texts[1].Bounds.X // the left edge of a 100 wide box
	if got := rightOf(texts[0]); got != box+100 {
		t.Errorf("Hebrew ends at %v, want the right edge %v", got, box+100)
	}
	if texts[1].Bounds.X != box {
		t.Errorf("English starts at %v, want the left edge %v", texts[1].Bounds.X, box)
	}
	if got := rightOf(texts[2]); got != box+100 {
		t.Errorf("English with TextEnd ends at %v, want %v", got, box+100)
	}
	if texts[3].Bounds.X != box {
		t.Errorf("Hebrew with TextEnd starts at %v, want the left edge %v", texts[3].Bounds.X, box)
	}
	for _, line := range texts[4:] {
		if got := rightOf(line); got != box+100 {
			t.Errorf("line %q of a Hebrew paragraph ends at %v, want the right edge %v", strings.TrimSpace(line.Value), got, box+100)
		}
	}
}
