package base

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/grapheme"
)

// overflowWord is one word no line is wide enough for, the shape of a long URL or
// path someone pasted into a label.
const overflowWord = "https://example.com/a/b/c/d/e/f/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

// graphemeCanvas measures one character as Size wide, a character being what a
// reader sees: a base with its marks, an emoji sequence, a flag. A width is then a
// count of characters, so a test does not depend on a font.
func graphemeCanvas() *gfx.Canvas {
	canvas := &gfx.Canvas{}
	canvas.SetTree(&tree)
	canvas.SetMeasurer(func(Value string, Size, Weight float32) float32 {
		count := 0
		for index := 0; index < len(Value); index = grapheme.Next(Value, index) {
			count++
		}
		return float32(count) * Size
	})
	return canvas
}

// centredParagraph lays text out as a paragraph of a Column as wide as width, centred
// across it: the shape a label takes when it is not stretched to its container, such
// as the title of a card. wrap is what the caller writes after f.Wrap.
func centredParagraph(width float32, value string, wrap func(Box) Box) (lines []string, text Rect, column Rect) {
	return centredParagraphOn(graphemeCanvas(), width, value, wrap)
}

// centredParagraphOn is centredParagraph measured by canvas: a font whose one rune is
// wider than the next cuts a character in half if the break is not kept to a boundary of
// whole characters, so the tests run the same text through both.
func centredParagraphOn(canvas *gfx.Canvas, width float32, value string, wrap func(Box) Box) (lines []string, text Rect, column Rect) {
	var view View
	var paragraph, columnBox Box
	f := view.Begin(canvas, gfx.LightTheme(), gfx.Rect{Width: width, Height: 400})
	columnBox = f.Column().Items(Center).Children(func() {
		paragraph = wrap(f.Wrap(value).FontSize(1))
	})
	f.End()
	n := paragraph.N()
	for k := int32(0); k < n.LineCount; k++ {
		lines = append(lines, f.Lines[n.LineFirst+k])
	}
	return lines, paragraph.Rect(), columnBox.Rect()
}

// A paragraph that fits: OverflowWrapAnywhere changes nothing at all, neither a line
// nor a rectangle, because its narrowest line is already its widest one.
func TestOverflowWrapLeavesATextThatFitsExactlyAsItWas(t *testing.T) {
	const value = "short and sweet"
	plain, plainRect, plainColumn := centredParagraph(40, value, func(b Box) Box { return b })
	anywhere, anywhereRect, anywhereColumn := centredParagraph(40, value, func(b Box) Box {
		return b.OverflowWrap(OverflowWrapAnywhere)
	})
	if strings.Join(plain, "|") != value || strings.Join(anywhere, "|") != value {
		t.Fatalf("a text that fits wrapped: %q and %q", plain, anywhere)
	}
	if plainRect != anywhereRect || plainColumn != anywhereColumn {
		t.Errorf("a text that fits moved: %v then %v, column %v then %v", plainRect, anywhereRect, plainColumn, anywhereColumn)
	}
	near(t, "the text", plainRect, gfx.Rect{X: 12.5, Width: 15, Height: 8})
}

// A word no line is wide enough for keeps the behaviour it had: without the property
// the paragraph asks for the width of its longest line and runs out over both edges of
// a centred column. This is the default path, and it is pinned here so that no later
// change can move it by a byte.
func TestOverflowWrapIsOffByDefault(t *testing.T) {
	lines, rect, column := centredParagraph(5, "aaaaaaaaaaaaaaaaaaaa", func(b Box) Box { return b })
	if len(lines) != 1 || lines[0] != "aaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("a long word without OverflowWrap broke into %q", lines)
	}
	if rect.Width != 20 {
		t.Errorf("a long word without OverflowWrap is %v wide, want 20 (its own width, not the room)", rect.Width)
	}
	if rect.X >= 0 || rect.X+rect.Width <= column.Width {
		t.Errorf("a long word without OverflowWrap sits at %v in a column %v wide: want it to run out over both edges", rect, column)
	}
}

// With OverflowWrapAnywhere the same paragraph asks for the room it is given and breaks
// the word: the column's width is never exceeded, and the last line is not cut.
func TestOverflowWrapKeepsALongWordInsideItsColumn(t *testing.T) {
	lines, rect, column := centredParagraph(5, "aaaaaaaaaaaaaaaaaaaa", func(b Box) Box {
		return b.OverflowWrap(OverflowWrapAnywhere)
	})
	if strings.Join(lines, "") != "aaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("the word came out as %q", lines)
	}
	if rect.Width != column.Width {
		t.Errorf("the paragraph is %v wide in a column %v wide, want the column's width", rect.Width, column.Width)
	}
	if rect.X != 0 {
		t.Errorf("the paragraph starts at %v, want 0: it is as wide as its column", rect.X)
	}
	for _, line := range lines {
		if width := float32(len(line)); width > column.Width {
			t.Errorf("line %q is %v wide, wider than the column (%v)", line, width, column.Width)
		}
	}
	if want := []string{"aaaaa", "aaaaa", "aaaaa", "aaaaa"}; strings.Join(lines, "|") != strings.Join(want, "|") {
		t.Errorf("the word broke as %q, want %q", lines, want)
	}
}

// A break falls between whole characters, never inside one as a reader sees it: an
// emoji sequence, a letter with its combining marks, a flag. Each cut is a UAX #29
// grapheme cluster boundary, and the characters are all still there.
func TestOverflowWrapBreaksBetweenWholeCharacters(t *testing.T) {
	for _, value := range []string{
		strings.Repeat("👨‍👩‍👧", 5),          // family: ZWJ sequences
		strings.Repeat("e\u0301", 6),        // e + combining acute
		strings.Repeat("🇨🇳", 5),             // flags: pairs of regional indicators
		strings.Repeat("👍🏽", 4),             // skin tone modifier
		strings.Repeat("अ\u0300", 4),        // Devanagari with a combining mark
		"漢字\u0308" + strings.Repeat("中", 4), // CJK with a combining voiced mark
	} {
		// A font whose marks and joiner add width breaks a character at a rune boundary
		// unless the cut is kept to a boundary of whole characters.
		for _, canvas := range []*gfx.Canvas{graphemeCanvas(), testCanvas()} {
			for _, width := range []float32{1, 2, 3} {
				lines, rect, column := centredParagraphOn(canvas, width, value, func(b Box) Box {
					return b.OverflowWrap(OverflowWrapAnywhere)
				})
				if strings.Join(lines, "") != value {
					t.Fatalf("%q at %v became %q: characters were lost", value, width, lines)
				}
				if rect.Width > column.Width {
					t.Errorf("%q at %v is %v wide in a column %v wide", value, width, rect.Width, column.Width)
				}
				offset := 0
				for _, line := range lines {
					if offset > 0 && !grapheme.Boundary(value, offset) {
						t.Errorf("%q at %v breaks at byte %d, inside a character: %q", value, width, offset, lines)
					}
					offset += len(line)
				}
			}
		}
	}
}

// The room a paragraph is given caps it whether it is in the flow of its container or
// out of it: an absolute one is as wide as its text wants, unless the text may break
// inside a word, when it is as wide as the room it has.
func TestOverflowWrapCapsAnAbsoluteTextAtItsContainer(t *testing.T) {
	for _, test := range []struct {
		name  string
		wrap  func(Box) Box
		width float32
		lines int
	}{
		{"without", func(b Box) Box { return b }, 20, 1},
		{"anywhere", func(b Box) Box { return b.OverflowWrap(OverflowWrapAnywhere) }, 8, 3},
	} {
		var view View
		var text, stack Box
		f := view.Begin(graphemeCanvas(), gfx.LightTheme(), gfx.Rect{Width: 8, Height: 400})
		stack = f.Stack().Children(func() {
			text = test.wrap(f.Wrap("aaaaaaaaaaaaaaaaaaaa").FontSize(1)).Absolute()
		})
		f.End()
		if stack.Rect().Width != 8 {
			t.Fatalf("%s: the container is %v wide, want 8", test.name, stack.Rect().Width)
		}
		if text.Rect().Width != test.width || int(text.N().LineCount) != test.lines {
			t.Errorf("%s: the text is %v in %d lines, want %v in %d", test.name, text.Rect().Width, text.N().LineCount, test.width, test.lines)
		}
	}
}

// A Row already shares a shortage among the texts in it, so asking to break anywhere
// leaves the line it takes and the width it ends with as they were.
func TestOverflowWrapChangesNothingInARowThatAlreadyShrinksIt(t *testing.T) {
	const value = "aaaaaaaaaaaaaaaaaaaa"
	layout := func(wrap func(Box) Box) (lines []string, rect gfx.Rect) {
		var view View
		var text Box
		f := view.Begin(graphemeCanvas(), gfx.LightTheme(), gfx.Rect{Width: 6, Height: 400})
		f.Row().Children(func() {
			text = wrap(f.Wrap(value).FontSize(1))
		})
		f.End()
		n := text.N()
		for k := int32(0); k < n.LineCount; k++ {
			lines = append(lines, f.Lines[n.LineFirst+k])
		}
		return lines, text.Rect()
	}
	plain, plainRect := layout(func(b Box) Box { return b })
	anywhere, anywhereRect := layout(func(b Box) Box { return b.OverflowWrap(OverflowWrapAnywhere) })
	if strings.Join(plain, "|") != strings.Join(anywhere, "|") || plainRect != anywhereRect {
		t.Errorf("a row laid the text out as %q at %v, and as %q at %v with the property", plain, plainRect, anywhere, anywhereRect)
	}
}

// OverflowWrapAnywhere asks for at most the room the text has, not for a size of its
// own: an explicit MaxWidth that is smaller still wins, and one that is larger does not
// push the text past the room.
func TestOverflowWrapTakesTheSmallerOfItsMaxWidthAndItsRoom(t *testing.T) {
	for _, test := range []struct {
		name string
		max  float32
		want float32
	}{
		{"narrower max", 3, 3},
		{"wider max", 15, 10},
	} {
		lines, rect, column := centredParagraph(10, "aaaaaaaaaaaaaaaaaaaa", func(b Box) Box {
			return b.OverflowWrap(OverflowWrapAnywhere).MaxWidth(test.max)
		})
		if rect.Width != test.want {
			t.Errorf("%s: the paragraph is %v wide, want %v", test.name, rect.Width, test.want)
		}
		if strings.Join(lines, "") != "aaaaaaaaaaaaaaaaaaaa" {
			t.Errorf("%s: the word came out as %q", test.name, lines)
		}
		for _, line := range lines {
			if float32(len(line)) > column.Width {
				t.Errorf("%s: line %q is wider than the column", test.name, line)
			}
		}
	}
}

// A Text that does not wrap is one line and is clipped, so the property has nothing to
// do on it: it stays exactly as wide as its text, as it was before.
func TestOverflowWrapDoesNothingToOneLineText(t *testing.T) {
	const value = "aaaaaaaaaaaaaaaaaaaa"
	layout := func(wrap func(Box) Box) gfx.Rect {
		var view View
		var text Box
		f := view.Begin(graphemeCanvas(), gfx.LightTheme(), gfx.Rect{Width: 5, Height: 400})
		f.Column().Items(Center).Children(func() {
			text = wrap(f.Text(value).FontSize(1))
		})
		f.End()
		return text.Rect()
	}
	plain := layout(func(b Box) Box { return b })
	anywhere := layout(func(b Box) Box { return b.OverflowWrap(OverflowWrapAnywhere) })
	if plain != anywhere {
		t.Errorf("a one-line text moved: %v then %v", plain, anywhere)
	}
}

// A URL breaks where it can first: after its separators, and only then inside the run
// that has none.
func TestOverflowWrapStillPrefersTheSeparatorsOfAUrl(t *testing.T) {
	lines, rect, _ := centredParagraph(20, overflowWord, func(b Box) Box {
		return b.OverflowWrap(OverflowWrapAnywhere)
	})
	if strings.Join(lines, "") != overflowWord {
		t.Fatalf("the URL came out as %q", lines)
	}
	if rect.Width != 20 {
		t.Errorf("the URL is %v wide, want its column's 20", rect.Width)
	}
	if !strings.HasSuffix(lines[0], "/") && !strings.HasPrefix(lines[1], "/") {
		t.Errorf("the URL broke as %q, want the first line to end at one of its separators", lines)
	}
}
