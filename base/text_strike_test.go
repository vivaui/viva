package base

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
)

// strikeProse is long enough to break into several lines at the widths these
// tests give it.
const strikeProse = "the price of every item in this list was struck through before the sale began"

// strikeInk is the color these tests give the struck texts, so the one fill a
// Strike paints can be told from anything else on the page.
var strikeInk = gfx.Color{Red: 1, Alpha: 1}

// strikeFills are the lines a page painted: one per Strike, since a page of bare
// Texts paints no other rectangle.
func strikeFills(canvas *gfx.Canvas) []gfx.Rectangle {
	return canvas.Rectangles()
}

// labelNamed returns the label of the text value, or fails.
func labelNamed(t *testing.T, canvas *gfx.Canvas, value string) gfx.Text {
	t.Helper()
	for _, painted := range canvas.Texts() {
		if painted.Value == value {
			return painted
		}
	}
	t.Fatalf("%q was not painted; the page painted %+v", value, canvas.Texts())
	return gfx.Text{}
}

// The line a Strike draws: one pixel high, in the text's own color, across the
// middle of the line box the label was given, from the first glyph to the last.
// That is the line components.MarkdownView draws for struck text, and it is the
// text that is struck: the box the Text was laid out in is four hundred pixels
// wide, the strike is as wide as the word.
func TestStrikeDrawsOnePixelThroughTheMiddleOfTheText(t *testing.T) {
	const size = 10 // the rune measurer makes every character ten pixels wide
	var view View
	canvas := runeCanvas()
	f := view.Begin(canvas, gfx.LightTheme(), gfx.Rect{Width: 400, Height: 200})
	f.Column().Children(func() {
		f.Text("struck").FontSize(size).Color(strikeInk).Strike()
		f.Text("plain").FontSize(size)
	})
	f.End()
	fills := strikeFills(canvas)
	if len(fills) != 1 {
		t.Fatalf("painted %d rectangles, want the one strike: %+v", len(fills), fills)
	}
	struck := labelNamed(t, canvas, "struck")
	plain := labelNamed(t, canvas, "plain")
	if struck.Style.Italic || plain.Style.Italic {
		t.Fatalf("a page without Italic painted italics: %+v %+v", struck.Style, plain.Style)
	}
	if plain.Bounds.Y <= struck.Bounds.Y {
		t.Fatalf("the two Texts are not stacked: %+v then %+v", struck.Bounds, plain.Bounds)
	}
	line := fills[0]
	if line.Color != strikeInk {
		t.Errorf("strike color = %v, want the text's color %v", line.Color, strikeInk)
	}
	if line.Bounds.Height != 1 {
		t.Errorf("strike height = %v, want one pixel", line.Bounds.Height)
	}
	if want := struck.Bounds.Y + struck.Bounds.Height/2; line.Bounds.Y != want {
		t.Errorf("strike top = %v, want %v: the middle of the line box %+v", line.Bounds.Y, want, struck.Bounds)
	}
	if line.Bounds.Y <= struck.Bounds.Y || line.Bounds.Y >= struck.Bounds.Y+struck.Bounds.Height {
		t.Errorf("strike %+v is not inside the line box %+v: it would miss the glyphs", line.Bounds, struck.Bounds)
	}
	if line.Bounds.X != struck.Bounds.X {
		t.Errorf("strike starts at %v, want the left end of the line %v", line.Bounds.X, struck.Bounds.X)
	}
	if want := canvas.TextWidthStyle("struck", struck.Style); line.Bounds.Width != want {
		t.Errorf("strike width = %v, want the width of the word %v, not the %v box it was laid out in", line.Bounds.Width, want, struck.Bounds.Width)
	}
}

// A strike follows the text, not the box: a centered or end-aligned Text is
// struck where its glyphs are, so the line starts at the label's own left end.
func TestStrikeFollowsACenteredAndAnEndAlignedText(t *testing.T) {
	const size = 10
	var view View
	canvas := runeCanvas()
	f := view.Begin(canvas, gfx.LightTheme(), gfx.Rect{Width: 400, Height: 200})
	f.Column().Children(func() {
		f.Text("mid").FontSize(size).Color(strikeInk).TextCenter().Strike()
		f.Text("end").FontSize(size).Color(strikeInk).TextEnd().Strike()
	})
	f.End()
	fills := strikeFills(canvas)
	if len(fills) != 2 {
		t.Fatalf("painted %d rectangles, want one strike per Text: %+v", len(fills), fills)
	}
	for index, value := range []string{"mid", "end"} {
		label := labelNamed(t, canvas, value)
		line := fills[index]
		if line.Bounds.X != label.Bounds.X {
			t.Errorf("%q strike starts at %v, want %v: the left end of the drawn line", value, line.Bounds.X, label.Bounds.X)
		}
		if want := canvas.TextWidthStyle(value, label.Style); line.Bounds.Width != want {
			t.Errorf("%q strike width = %v, want %v", value, line.Bounds.Width, want)
		}
		if label.Bounds.X == 0 {
			t.Errorf("%q label was not moved off the left edge, so the test cannot tell the strike from a box-wide one", value)
		}
	}
}

// Every line of a wrapped struck paragraph crosses its own line box, at its own
// width: the lines are painted one by one, so a strike cannot be one long line
// across the paragraph.
func TestEveryLineOfAStruckParagraphIsStruck(t *testing.T) {
	const size, width = 10, 400 // forty characters to a line
	var view View
	canvas := runeCanvas()
	f := view.Begin(canvas, gfx.LightTheme(), gfx.Rect{Width: width, Height: 600})
	f.Column().Children(func() {
		f.Wrap(strikeProse).FontSize(size).Color(strikeInk).Strike()
	})
	f.End()
	lines := canvas.Texts()
	fills := strikeFills(canvas)
	if len(lines) < 2 {
		t.Fatalf("painted %d lines, want the paragraph broken up: %+v", len(lines), lines)
	}
	if len(fills) != len(lines) {
		t.Fatalf("painted %d strikes for %d lines, want one each", len(fills), len(lines))
	}
	previous := float32(-1)
	for index, line := range lines {
		strike := fills[index]
		if strike.Bounds.Height != 1 {
			t.Errorf("line %d strike height = %v, want one pixel", index, strike.Bounds.Height)
		}
		if want := line.Bounds.Y + line.Bounds.Height/2; strike.Bounds.Y != want {
			t.Errorf("line %d strike top = %v, want %v (line box %+v)", index, strike.Bounds.Y, want, line.Bounds)
		}
		if strike.Bounds.Y <= previous {
			t.Errorf("line %d strike at %v is not below the previous one at %v", index, strike.Bounds.Y, previous)
		}
		previous = strike.Bounds.Y
		if strike.Bounds.X != line.Bounds.X {
			t.Errorf("line %d strike starts at %v, want %v", index, strike.Bounds.X, line.Bounds.X)
		}
		if want := canvas.TextWidthStyle(line.Value, line.Style); strike.Bounds.Width != want {
			t.Errorf("line %d %q strike width = %v, want %v", index, line.Value, strike.Bounds.Width, want)
		}
		if strike.Bounds.Width > width {
			t.Errorf("line %d %q strike is %v wide, wider than the %v box", index, line.Value, strike.Bounds.Width, width)
		}
	}
}

// A Mono text is struck across its cells, as it is laid out on them.
func TestStrikeOfAMonoTextSpansItsCells(t *testing.T) {
	const size = 10
	var view View
	canvas := runeCanvas()
	f := view.Begin(canvas, gfx.LightTheme(), gfx.Rect{Width: 400, Height: 100})
	f.Column().Children(func() {
		f.Text("mm").FontSize(size).Mono().Color(strikeInk).Strike()
	})
	f.End()
	fills := strikeFills(canvas)
	if len(fills) != 1 {
		t.Fatalf("painted %d rectangles, want the one strike: %+v", len(fills), fills)
	}
	label := labelNamed(t, canvas, "mm")
	columns := canvas.TextWidthStyle("mm", label.Style)
	if fills[0].Bounds.X != label.Bounds.X || fills[0].Bounds.Width != columns {
		t.Errorf("strike %+v, want the cells %v..%v", fills[0].Bounds, label.Bounds.X, label.Bounds.X+columns)
	}
}

// A Strike changes no layout: a struck Text is measured, broken into the same lines
// and given the same box as the same Text without one. The face is read by the
// painter alone, never by the measurement, so a Text is as wide and as tall struck
// as upright.
func TestAStrikeChangesNoLayout(t *testing.T) {
	const width, size = 400, 10
	draw := func(struck bool) (Box, Box, []string) {
		var view View
		canvas := runeCanvas()
		var single, paragraph Box
		f := view.Begin(canvas, gfx.LightTheme(), gfx.Rect{Width: width, Height: 400})
		f.Column().Children(func() {
			f.Row().Children(func() {
				single = f.Text("struck").FontSize(size)
				if struck {
					single = single.Strike()
				}
			})
			paragraph = f.Wrap(strikeProse).FontSize(size).Width(220)
			if struck {
				paragraph = paragraph.Strike()
			}
		})
		f.End()
		var lines []string
		for _, painted := range canvas.Texts() {
			lines = append(lines, painted.Value)
		}
		return single, paragraph, lines
	}
	uprightText, uprightParagraph, uprightLines := draw(false)
	struckText, struckParagraph, struckLines := draw(true)
	if uprightText.Rect() != struckText.Rect() {
		t.Errorf("a struck Text is %+v, want the upright one's %+v: a strike must not be measured", struckText.Rect(), uprightText.Rect())
	}
	if uprightParagraph.Rect() != struckParagraph.Rect() {
		t.Errorf("a struck paragraph is %+v, want the upright one's %+v", struckParagraph.Rect(), uprightParagraph.Rect())
	}
	if strings.Join(uprightLines, "|") != strings.Join(struckLines, "|") {
		t.Errorf("struck lines %q, want the upright lines %q", struckLines, uprightLines)
	}
}
