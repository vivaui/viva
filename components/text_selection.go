package components

import (
	"math"
	"strings"

	"github.com/vivaui/viva/ui"
)

// TextPoint is a place in a run of painted texts that a selection can cross:
// Surface numbers the text in reading order, from 0, and Offset is a byte
// offset in that text, as MarkdownView.OffsetAt reports it.
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
func (selection *TextSelection) Pointer(event ui.PointerEvent, at func(ui.Point) (TextPoint, bool)) TextSelectionResult {
	point := ui.Point{X: event.X, Y: event.Y}
	switch event.Kind {
	case ui.PointerDown:
		if event.Button != ui.PointerButtonPrimary && event.Button != ui.PointerButtonNone {
			return TextSelectionResult{}
		}
		place, ok := at(point)
		if !ok {
			return TextSelectionResult{}
		}
		*selection = TextSelection{Anchor: place, Focus: place, dragging: true}
		return TextSelectionResult{Handled: true, Changed: true}
	case ui.PointerMove:
		if !selection.dragging || !event.Captured {
			return TextSelectionResult{}
		}
		place, ok := at(point)
		if !ok || place == selection.Focus {
			return TextSelectionResult{Handled: true}
		}
		selection.Focus = place
		return TextSelectionResult{Handled: true, Changed: true}
	case ui.PointerUp:
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
	case ui.PointerExit:
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
// past the end of are larger than its text; MarkdownSelection clamps them.
func (selection *TextSelection) In(surface int) MarkdownSelection {
	if selection.Empty() {
		return MarkdownSelection{}
	}
	first, last := selection.Anchor, selection.Focus
	if last.before(first) {
		first, last = last, first
	}
	if surface < first.Surface || surface > last.Surface {
		return MarkdownSelection{}
	}
	if first.Surface == last.Surface {
		return MarkdownSelection{Anchor: selection.Anchor.Offset, Focus: selection.Focus.Offset}
	}
	switch surface {
	case first.Surface:
		return MarkdownSelection{Anchor: first.Offset, Focus: math.MaxInt}
	case last.Surface:
		return MarkdownSelection{Anchor: 0, Focus: last.Offset}
	}
	return MarkdownSelection{Anchor: 0, Focus: math.MaxInt}
}

// Text is the selected text of surfaces texts in reading order, with separator
// between the texts of two surfaces, which is what a copy puts on the
// clipboard. text gives the text of one surface for the part In reports for it;
// surfaces with nothing selected are skipped and add no separator. It returns ""
// for an empty selection.
func (selection *TextSelection) Text(surfaces int, separator string, text func(surface int, part MarkdownSelection) string) string {
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
