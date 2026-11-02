package ui

import (
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

// FontFamilyAware is a TextEngine whose proportional family the application can
// name while it runs: the platform engines implement it. The reference engine's
// widths know no family, and a backend whose text the system draws without a
// say in the family (the browser) does not.
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
