package base

import (
	"math"
	"strings"

	"github.com/vivaui/viva/gfx"
)

// TextPoint is a place in a run of painted texts that a selection can cross:
// Surface numbers the text in reading order, from 0, and Offset is a byte
// offset in that text, as MarkdownView.OffsetAt or a TextSelectionArea reports it.
type TextPoint struct {
	Surface int
	Offset  int
}

func (point TextPoint) before(other TextPoint) bool {
	return point.Surface < other.Surface || point.Surface == other.Surface && point.Offset < other.Offset
}

// TextSelection is the text a user has selected with the pointer, which may
// begin in one painted text and end in another: the answers of a conversation,
// the rows of a detail page. The caller-owned value holds where the drag began
// (Anchor) and where the pointer is now (Focus), each in one surface, and
// whether a drag is in progress. Its zero value is no selection.
//
// A surface is any painted text with a stable position in reading order: a
// MarkdownView, a table, a plain text block. The application numbers its
// surfaces, tells Pointer which point of the window each surface lies at, and
// asks In what each surface should paint highlighted and Text what a copy puts
// on the clipboard. TextSelection allocates nothing while the pointer moves, and
// schedules no work: the caller repaints when a result says the selection
// changed.
type TextSelection struct {
	Anchor   TextPoint
	Focus    TextPoint
	dragging bool
}

// TextRange is the selected part of one text, in byte offsets: of a Text in a
// TextSelectionArea, of the rendered text of a MarkdownView. Anchor is where the
// user began and Focus is where the pointer is now; either may be ahead of the
// other. Range clamps and orders them against the text, so a selection that was
// made before the text changed still copies the part that is there.
type TextRange struct {
	Anchor, Focus int
}

// Range returns the selected bytes, ordered and clamped to text. Clamping snaps
// both ends back to the start of the rune they point into, because a selection
// kept while a streaming message shortened can land inside a multi-byte
// character; without the snap SelectedText would return invalid UTF-8.
func (s TextRange) Range(text []byte) (start, end int) {
	start, end = s.Anchor, s.Focus
	if start > end {
		start, end = end, start
	}
	start = snapRune(text, min(max(start, 0), len(text)))
	end = snapRune(text, min(max(end, 0), len(text)))
	return start, end
}

// rangeIn is Range for a text held as a string.
func (s TextRange) rangeIn(text string) (start, end int) {
	start, end = s.Anchor, s.Focus
	if start > end {
		start, end = end, start
	}
	start = snapRune(text, min(max(start, 0), len(text)))
	end = snapRune(text, min(max(end, 0), len(text)))
	return start, end
}

// snapRune moves offset back to the leading byte of the rune it points into.
// Offsets that are already at a boundary, including len(text), are returned as
// they are.
func snapRune[T string | []byte](text T, offset int) int {
	for offset > 0 && offset < len(text) && text[offset]&0xC0 == 0x80 {
		offset--
	}
	return offset
}

// TextSelectionResult says what Pointer did with an event.
type TextSelectionResult struct {
	// Handled is true when the event belongs to the selection, so nothing else
	// should act on it.
	Handled bool
	// Changed is true when the selection or the drag changed and the window
	// should be painted again.
	Changed bool
}

// Empty reports whether nothing is selected: the anchor and the focus are the
// same place.
func (selection *TextSelection) Empty() bool { return selection.Anchor == selection.Focus }

// Dragging reports whether the pointer is held down and extending the selection.
func (selection *TextSelection) Dragging() bool { return selection.dragging }

// EndDrag ends a drag in progress and keeps what is selected, for a drag whose
// release the window will not see: a press elsewhere, a window that lost focus.
func (selection *TextSelection) EndDrag() { selection.dragging = false }

// Clear drops the selection and any drag in progress.
func (selection *TextSelection) Clear() { *selection = TextSelection{} }

// Select makes the selection run from one point to another without a pointer,
// as "select all" does, and ends any drag.
func (selection *TextSelection) Select(anchor, focus TextPoint) {
	*selection = TextSelection{Anchor: anchor, Focus: focus}
}

// SelectAll selects every character of surfaces texts, where length gives the
// length of each. With no surface, or only empty ones, it clears the selection.
func (selection *TextSelection) SelectAll(surfaces int, length func(surface int) int) {
	first, last := -1, -1
	for surface := range surfaces {
		if length(surface) > 0 {
			if first < 0 {
				first = surface
			}
			last = surface
		}
	}
	if first < 0 {
		selection.Clear()
		return
	}
	selection.Select(TextPoint{Surface: first}, TextPoint{Surface: last, Offset: length(last)})
}

// Pointer follows one pointer event. A primary press begins a selection at the
// point at() reports for it; a held move extends it; the release ends the drag
// and drops a selection that covers nothing; the pointer leaving the window
// ends the drag and keeps what was selected.
//
// at maps a window point to a place in the surfaces: the surface the point is
// over, or the nearest, and the offset in it. It reports false for a point that
// is over no surface, which neither begins nor extends a selection; a press on
// it is left unhandled so the caller can clear the selection or let the press
// go on. A press that should not start a selection, such as one on a link or a
// button painted over the text, is the caller's to keep from reaching Pointer.
func (selection *TextSelection) Pointer(event gfx.PointerEvent, at func(gfx.Point) (TextPoint, bool)) TextSelectionResult {
	point := gfx.Point{X: event.X, Y: event.Y}
	switch event.Kind {
	case gfx.PointerDown:
		if event.Button != gfx.PointerButtonPrimary && event.Button != gfx.PointerButtonNone {
			return TextSelectionResult{}
		}
		place, ok := at(point)
		if !ok {
			return TextSelectionResult{}
		}
		*selection = TextSelection{Anchor: place, Focus: place, dragging: true}
		return TextSelectionResult{Handled: true, Changed: true}
	case gfx.PointerMove:
		if !selection.dragging || !event.Captured {
			return TextSelectionResult{}
		}
		place, ok := at(point)
		if !ok || place == selection.Focus {
			return TextSelectionResult{Handled: true}
		}
		selection.Focus = place
		return TextSelectionResult{Handled: true, Changed: true}
	case gfx.PointerUp:
		if !selection.dragging {
			return TextSelectionResult{}
		}
		if place, ok := at(point); ok {
			selection.Focus = place
		}
		selection.dragging = false
		if selection.Empty() {
			selection.Clear()
		}
		return TextSelectionResult{Handled: true, Changed: true}
	case gfx.PointerExit:
		// A held press keeps the pointer outside the window, and Exit reaches the
		// application only once the press is over: the button came up beyond the
		// window, or the system took the press away.
		if !selection.dragging {
			return TextSelectionResult{}
		}
		selection.dragging = false
		return TextSelectionResult{Handled: true, Changed: true}
	}
	return TextSelectionResult{}
}

// In is the part of one surface's text the selection covers, to paint
// highlighted and to copy: the zero value for a surface outside it, the whole
// text for one it passes through completely, and in the surface where the
// selection begins or ends only the part on its side. Within one surface the
// Anchor and the Focus keep the order the drag gave them, so a drag upward is
// not turned round on each paint. The offsets of a surface the selection runs
// past the end of are larger than its text; TextRange.Range clamps them.
func (selection *TextSelection) In(surface int) TextRange {
	if selection.Empty() {
		return TextRange{}
	}
	first, last := selection.Anchor, selection.Focus
	if last.before(first) {
		first, last = last, first
	}
	if surface < first.Surface || surface > last.Surface {
		return TextRange{}
	}
	if first.Surface == last.Surface {
		return TextRange{Anchor: selection.Anchor.Offset, Focus: selection.Focus.Offset}
	}
	switch surface {
	case first.Surface:
		return TextRange{Anchor: first.Offset, Focus: math.MaxInt}
	case last.Surface:
		return TextRange{Anchor: 0, Focus: last.Offset}
	}
	return TextRange{Anchor: 0, Focus: math.MaxInt}
}

// Text is the selected text of surfaces texts in reading order, with separator
// between the texts of two surfaces, which is what a copy puts on the
// clipboard. text gives the text of one surface for the part In reports for it;
// surfaces with nothing selected are skipped and add no separator. It returns ""
// for an empty selection.
func (selection *TextSelection) Text(surfaces int, separator string, text func(surface int, part TextRange) string) string {
	if selection.Empty() {
		return ""
	}
	first, last := selection.Anchor.Surface, selection.Focus.Surface
	if last < first {
		first, last = last, first
	}
	first, last = max(first, 0), min(last, surfaces-1)
	var out strings.Builder
	for surface := first; surface <= last; surface++ {
		piece := text(surface, selection.In(surface))
		if piece == "" {
			continue
		}
		if out.Len() > 0 {
			out.WriteString(separator)
		}
		out.WriteString(piece)
	}
	return out.String()
}

// Clamp keeps the selection inside surfaces texts after they changed, as when a
// streamed answer grew or a row was removed: each end moves to the nearest
// surface and into its text, and a selection whose ends meet is cleared.
// Surface 0 through surfaces-1 are the ones that exist; length gives the length
// of each. The order of Anchor and Focus is kept. A drag in progress goes on.
func (selection *TextSelection) Clamp(surfaces int, length func(surface int) int) {
	if surfaces <= 0 {
		selection.Clear()
		return
	}
	clampPoint := func(point TextPoint) TextPoint {
		if point.Surface < 0 {
			return TextPoint{}
		}
		if point.Surface >= surfaces {
			last := surfaces - 1
			return TextPoint{Surface: last, Offset: length(last)}
		}
		return TextPoint{Surface: point.Surface, Offset: min(max(point.Offset, 0), length(point.Surface))}
	}
	selection.Anchor, selection.Focus = clampPoint(selection.Anchor), clampPoint(selection.Focus)
	if selection.Empty() && !selection.dragging {
		selection.Clear()
	}
}
