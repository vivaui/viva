package base

import (
	"testing"
	"unicode/utf8"

	"github.com/vivaui/viva/gfx"
)

// italicProse is long enough to break at the widths these tests give it.
const italicProse = "the emphasis in this paragraph is drawn in the italic face"

// paintedText is one line the page painted: its value and its slant.
type paintedText struct {
	value  string
	italic bool
}

// paintedTexts is every label the page painted, in paint order.
func paintedTexts(canvas *gfx.Canvas) []paintedText {
	texts := canvas.Texts()
	out := make([]paintedText, 0, len(texts))
	for _, painted := range texts {
		out = append(out, paintedText{value: painted.Value, italic: painted.Style.Italic})
	}
	return out
}

// Italic draws a Text in the italic face of its font and leaves every other Text
// upright: it is the element's own type, like Mono, so it travels to the label the
// canvas draws and no farther.
func TestItalicTextIsPaintedInTheItalicFace(t *testing.T) {
	var view View
	canvas := testCanvas()
	f := view.Begin(canvas, gfx.LightTheme(), gfx.Rect{Width: 400, Height: 200})
	f.Column().Children(func() {
		f.Text("upright").FontSize(10)
		f.Text("slanted").FontSize(10).Italic()
	})
	f.End()
	painted := paintedTexts(canvas)
	if len(painted) != 2 {
		t.Fatalf("painted %d labels, want the two Texts: %+v", len(painted), painted)
	}
	if painted[0].value != "upright" || painted[0].italic {
		t.Fatalf("first label = %+v, want the upright Text drawn upright", painted[0])
	}
	if painted[1].value != "slanted" || !painted[1].italic {
		t.Fatalf("second label = %+v, want the Italic Text drawn in the italic face", painted[1])
	}
}

// Every line of a wrapped Italic paragraph is drawn in the italic face: the lines
// are measured and painted through one style, so a slant on the paragraph cannot
// reach some of them and not the rest.
func TestEveryWrappedLineOfAnItalicTextIsItalic(t *testing.T) {
	var view View
	canvas := runeCanvas()
	f := view.Begin(canvas, gfx.LightTheme(), gfx.Rect{Width: 100, Height: 400})
	f.Column().Children(func() {
		f.Wrap(italicProse).FontSize(10).Italic()
	})
	f.End()
	painted := paintedTexts(canvas)
	if len(painted) < 2 {
		t.Fatalf("painted %d labels, want the paragraph broken into lines: %+v", len(painted), painted)
	}
	for index, line := range painted {
		if !line.italic {
			t.Fatalf("line %d %q is upright; every line of an Italic paragraph is drawn in the italic face", index, line.value)
		}
	}
}

// italicWiderEngine measures a line at its upright width and twice that in the
// italic face, so a line wrapped in one face and drawn in the other does not fit
// the box, and the lines of a paragraph say which face they were measured in.
type italicWiderEngine struct{}

func (engine *italicWiderEngine) lineWidth(value string, size float32) float32 {
	return float32(utf8.RuneCountInString(value)) * size
}

func (engine *italicWiderEngine) Width(value string, size, weight float32) float32 {
	return engine.lineWidth(value, size)
}

func (engine *italicWiderEngine) Caret(value string, size, weight float32, offset int) float32 {
	return float32(offset) * size
}

func (engine *italicWiderEngine) Selection(dst []gfx.TextSpan, value string, size, weight float32, start, end int) []gfx.TextSpan {
	return append(dst, gfx.TextSpan{X: float32(start) * size, Width: float32(end-start) * size})
}

func (engine *italicWiderEngine) OffsetAt(value string, size, weight, x float32) int {
	return min(max(int(x/size+0.5), 0), len(value))
}

func (engine *italicWiderEngine) WidthStyle(value string, size, weight float32, italic bool) float32 {
	if italic {
		return 2 * engine.lineWidth(value, size)
	}
	return engine.lineWidth(value, size)
}

func (engine *italicWiderEngine) CaretStyle(value string, size, weight float32, italic bool, offset int) float32 {
	return engine.Caret(value, size, weight, offset)
}

func (engine *italicWiderEngine) SelectionStyle(dst []gfx.TextSpan, value string, size, weight float32, italic bool, start, end int) []gfx.TextSpan {
	return engine.Selection(dst, value, size, weight, start, end)
}

func (engine *italicWiderEngine) OffsetAtStyle(value string, size, weight float32, italic bool, x float32) int {
	return engine.OffsetAt(value, size, weight, x)
}

// A wrapped Italic paragraph is wrapped at the widths of the italic face, and the
// lines it is given are drawn in that face: a Text that became italic on the next
// frame is wrapped again, not drawn in the upright lines it was measured in. The
// italic face here is twice as wide, so lines of the wrong face overflow the box.
func TestAWrappedItalicTextIsWrappedAndDrawnInTheItalicFace(t *testing.T) {
	const width, size = 200, 10 // 20 characters of the upright face, 10 of the italic one
	var view View
	declare := func(italic bool) (*gfx.Canvas, []paintedText) {
		canvas := &gfx.Canvas{}
		canvas.SetTree(&tree)
		canvas.SetTextEngine(&italicWiderEngine{})
		f := view.Begin(canvas, gfx.LightTheme(), gfx.Rect{Width: width, Height: 800})
		f.Column().Children(func() {
			text := f.Wrap(italicProse).FontSize(size)
			if italic {
				text.Italic()
			}
		})
		f.End()
		return canvas, paintedTexts(canvas)
	}
	_, upright := declare(false)
	canvas, slanted := declare(true)
	if len(slanted) <= len(upright) {
		t.Fatalf("%d italic lines and %d upright ones: the paragraph was wrapped as if it were upright", len(slanted), len(upright))
	}
	italic := gfx.TextStyle{Size: size, Italic: true}
	for index, line := range slanted {
		if !line.italic {
			t.Fatalf("line %d %q is drawn upright; an Italic paragraph is drawn in the italic face", index, line.value)
		}
		if got := canvas.TextWidthStyle(line.value, italic); got > width {
			t.Fatalf("line %d %q is %v wide in the italic face, wider than the %v box: it was wrapped in the upright face", index, line.value, got, width)
		}
	}
}

// A single-line Italic Text takes the width the italic face measures it at, so the
// box it is given holds its glyphs: here the italic face is twice as wide, so an
// upright measurement would leave it half the room it needs.
func TestAnItalicTextTakesTheWidthOfItsItalicFace(t *testing.T) {
	var view View
	canvas := &gfx.Canvas{}
	canvas.SetTree(&tree)
	canvas.SetTextEngine(&italicWiderEngine{})
	var box Box
	f := view.Begin(canvas, gfx.LightTheme(), gfx.Rect{Width: 400, Height: 100})
	f.Row().Children(func() {
		box = f.Text("ab").FontSize(10).Italic()
	})
	f.End()
	if got, want := box.Rect().Width, float32(40); got != want {
		t.Fatalf("Italic Text is %v wide, want %v: twice the upright width of %q in the italic face", got, want, "ab")
	}
	painted := paintedTexts(canvas)
	if len(painted) != 1 || painted[0].value != "ab" || !painted[0].italic {
		t.Fatalf("painted %+v, want the one label of the Italic Text", painted)
	}
}

// An Italic Text is not a required field Label and a required Label is not
// italic: the required marker a form field's title appends and the face a Text is
// drawn in are two different properties. The state word's bits were all taken when
// Italic arrived, and 1<<31 is the required marker, so taking that bit for the
// slant made every Italic Label of a page report itself required (and every
// required Label italic).
func TestAnItalicTextIsNotARequiredLabel(t *testing.T) {
	var view View
	var box Box
	f := view.Begin(testCanvas(), gfx.LightTheme(), gfx.Rect{Width: 400, Height: 100})
	f.Column().Children(func() {
		box = f.Text("emphasis").Italic()
	})
	f.End()
	if box.N().Flags&flagRequired != 0 {
		t.Fatal("an Italic Text carries the required marker's bit: Italic must not share a bit with a property it has nothing to do with")
	}
	if box.N().TextDecor&textItalic == 0 {
		t.Fatal("the Text is not marked italic at all")
	}
}
