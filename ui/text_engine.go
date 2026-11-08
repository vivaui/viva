package ui

import (
	"errors"
	"unicode/utf8"

	"github.com/vivaui/viva/internal/textmetrics"
)

// TextEngine measures and hit-tests one line of text with the platform's own shaped
// line: the same line that is drawn, with the platform's font fallback, so
// a width, a caret and a click agree with the glyphs on screen.
//
// Sizes are logical font sizes and results are logical pixels from the left end of
// the line. Weight is TextStyle.Weight. Offsets are UTF-8 byte offsets into value; an
// offset inside a grapheme cluster means the start of that cluster, and one outside
// [0, len(value)] is clamped. An implementation is called on the paint goroutine only,
// must not allocate for a line it has measured before, and keeps its shaped lines
// itself (internal/textcache holds them for the backends).
type TextEngine interface {
	// Width is the advance of the whole line; 0 only for an empty line.
	Width(value string, size, weight float32) float32
	// Caret is the x of the caret at offset: the leading edge of the character at
	// offset, the trailing edge of the last one at the end, and where the display
	// puts that boundary in right-to-left text.
	Caret(value string, size, weight float32, offset int) float32
	// Selection appends the extents of [start, end) to dst, left to right: one span,
	// or several where the selection crosses a change of direction.
	Selection(dst []TextSpan, value string, size, weight float32, start, end int) []TextSpan
	// OffsetAt is the byte offset of the character boundary nearest x.
	OffsetAt(value string, size, weight, x float32) int
}

// TextRasterizer is an optional TextEngine capability that draws one shaped
// proportional line into a transparent premultiplied BGRA bitmap at scale.
// Width and height are bitmap pixels. The same platform font, fallback and
// shaping used by TextEngine measurements are used for the pixels.
type TextRasterizer interface {
	TextEngine
	RasterizeText(value string, style TextStyle, scale float32, width, height int) (Bitmap, error)
}

// ErrTextRasterizerUnavailable means the canvas' installed text engine cannot
// render proportional text into a bitmap.
var ErrTextRasterizerUnavailable = errors.New("ui: text rasterizer unavailable")

// StyledTextEngine is a TextEngine that also shapes proportional italic text. Its methods
// are those of TextEngine with the face named: with italic false each answers exactly as
// the TextEngine method of the same name, and with it true, for the italic face of the
// same family, size and weight. A line drawn with TextStyle.Italic must be measured through
// them (Canvas.TextWidthStyle and the like), so that widths, carets, selections and hits
// agree with the glyphs. An engine that does not implement it measures italic text in the
// regular face.
type StyledTextEngine interface {
	TextEngine
	WidthStyle(value string, size, weight float32, italic bool) float32
	CaretStyle(value string, size, weight float32, italic bool, offset int) float32
	SelectionStyle(dst []TextSpan, value string, size, weight float32, italic bool, start, end int) []TextSpan
	OffsetAtStyle(value string, size, weight float32, italic bool, x float32) int
}

// MonoCellEngine is a TextEngine that measures the character cell of a fixed-pitch
// face: the platform engines implement it. Canvas.MonoCellSize asks it, so a terminal
// is laid out on the pitch and line height of the font it is drawn in; an engine
// without it, such as the reference engine, leaves the reference cell (0.6 of the size
// wide, 1.18 high).
type MonoCellEngine interface {
	TextEngine
	// MonoCell is the advance of one character and the height of one line of face's
	// own font at a logical size, in logical pixels, whatever the families that draw
	// what that font lacks. A cell it cannot measure is 0, 0. It is called with the
	// same arguments again only after FontGeneration moved, or the canvas's memo
	// dropped the answer.
	MonoCell(size float32, face MonoFace) (advance, line float32)
}

// FontFamilyAware is a TextEngine whose proportional family the application can
// name while it runs: the platform engines implement it, including the
// browser's document engine. The reference engine's widths know no family.
type FontFamilyAware interface {
	TextEngine
	// SetFontFamily names the family the interface's proportional text is shaped
	// in, or "" for the platform's own interface font (Segoe UI on Windows, the
	// desktop's choice through Pango's sans alias on Linux, the system font on
	// macOS). A name the system has no family for is left to the platform to
	// stand in for, as it does for any text. What was shaped with another family
	// is forgotten, so the next measure shapes again.
	SetFontFamily(family string)
	// FontGeneration counts the changes after which the same line shapes
	// differently: it moves on every SetFontFamily that changed the family, and
	// when the system's own fonts changed under the engine. What the canvas or a
	// renderer remembers of shaped text (a cut line, a rasterized label) is stale
	// once it differs from the generation it was made at. Wrapping around is
	// harmless: only equality is ever asked.
	FontGeneration() uint32
}

// SetTextEngine installs the engine every text measurement of the canvas goes
// through; nil restores the reference metrics (internal/textmetrics). A backend installs
// its platform's engine; tests and tools that want fixed widths leave it unset, or
// install ReferenceTextEngine.
func (canvas *Canvas) SetTextEngine(engine TextEngine) {
	canvas.engine = engine
	canvas.aware, _ = engine.(FontFamilyAware)
	canvas.monoCells, _ = engine.(MonoCellEngine)
	canvas.cellSizes = cellSizeMemos{}
	clear(canvas.fits)
}

// FontGeneration is the generation of the installed engine's fonts
// (FontFamilyAware.FontGeneration), 0 for an engine that has none. A painter
// that sends text to a renderer stamps it on the text, so a renderer holding a
// picture of a label made in other fonts draws it again.
func (canvas *Canvas) FontGeneration() uint32 {
	if canvas.aware == nil {
		return 0
	}
	return canvas.aware.FontGeneration()
}

// RasterizeText draws one proportional line with the canvas' installed platform
// text engine into a transparent premultiplied BGRA bitmap. Scale is device
// pixels per logical pixel; width and height are bitmap pixels.
func (canvas *Canvas) RasterizeText(value string, style TextStyle, scale float32, width, height int) (Bitmap, error) {
	rasterizer, ok := canvas.engine.(TextRasterizer)
	if !ok {
		return Bitmap{}, ErrTextRasterizerUnavailable
	}
	value, _ = resolvedTextDirection(value, style.Direction)
	return rasterizer.RasterizeText(value, style, scale, width, height)
}

// ReferenceTextEngine is the TextEngine over the reference metrics
// (internal/textmetrics): the same widths on every platform, whatever fonts are
// installed. It is what a canvas with no engine measures with, and the way for a
// test or a screenshot to ask for those widths explicitly.
func ReferenceTextEngine() TextEngine { return referenceEngine{} }

type referenceEngine struct{}

func (referenceEngine) Width(value string, size, weight float32) float32 {
	return textmetrics.Width(value, size, weight)
}

func (referenceEngine) Caret(value string, size, weight float32, offset int) float32 {
	return textmetrics.Caret(value, size, weight, offset)
}

func (referenceEngine) Selection(dst []TextSpan, value string, size, weight float32, start, end int) []TextSpan {
	var spans [8]textmetrics.Span
	for _, span := range textmetrics.Selection(spans[:0], value, size, weight, start, end) {
		dst = append(dst, TextSpan{X: span.X, Width: span.Width})
	}
	return dst
}

func (referenceEngine) OffsetAt(value string, size, weight, x float32) int {
	return textmetrics.OffsetAt(value, size, weight, x)
}

// offsetAtByPrefix is the character boundary of value whose prefix width is nearest to
// x: OffsetAt for a measurer that only knows widths.
func offsetAtByPrefix(value string, x float32, width func(prefix string) float32) int {
	best, bestDistance := 0, abs32(x-width(""))
	for offset := range value {
		if offset == 0 {
			continue
		}
		if distance := abs32(x - width(value[:offset])); distance < bestDistance {
			best, bestDistance = offset, distance
		}
	}
	if len(value) > 0 {
		if distance := abs32(x - width(value)); distance < bestDistance {
			best = len(value)
		}
	}
	return best
}

// clampOffset is offset limited to [0, len(value)] and moved back to the start of the
// character it falls inside.
func clampOffset(value string, offset int) int {
	offset = max(0, min(offset, len(value)))
	for offset > 0 && offset < len(value) && !utf8.RuneStart(value[offset]) {
		offset--
	}
	return offset
}

// FontRegistrar is a TextEngine that can take a font the application carries: the platform
// engines implement it.
type FontRegistrar interface {
	TextEngine
	// RegisterFont makes the TrueType or OpenType font in data available to this process only,
	// without installing it, and returns the family it names (the first one, for a collection),
	// ready to give to MonoFamily or FontFamilyAware.SetFontFamily, and a function that takes the
	// registration away. Registering the same data again shares the registration. What was shaped
	// before is forgotten, so a family that stood in for the missing font is drawn again.
	RegisterFont(data []byte) (family string, unregister func(), err error)
}
