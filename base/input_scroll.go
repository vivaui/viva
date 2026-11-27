package base

// InputScroll is the horizontal offset of one single-line field's painted line: how
// far the text of a value too long for the field is moved left so that the caret
// stays in view, as a native single-line editor does. The zero value starts at the
// left edge, which is the whole story for a value that fits.
//
// It is held by the field, next to the text it belongs to, and read during the paint
// that draws the field's own glyphs (see InputSnapshot.Scroll): a component that
// paints the text itself must move it with the caret, because the platform editor
// that would have scrolled it draws nothing. It is not safe for concurrent use, and
// one InputScroll serves one field.
type InputScroll struct {
	offset float32
}

// Follow returns the offset to paint the line with, for a line textWidth wide whose
// caret stands caretX from the line's start, inside a field width across. The offset
// moves only as far as it must: a caret that is still in view leaves the text where
// it is, so typing at the end does not slide the line under a caret that has not
// reached the edge, and End shows the tail of a long value while Home comes back to
// its start. width of zero or less paints nothing, and an offset that no longer
// fits (a shorter value, a wider field) is brought back into range.
func (s *InputScroll) Follow(caretX, textWidth, width float32) float32 {
	if width <= 0 {
		s.offset = 0
		return 0
	}
	// The caret is a stroke one pixel wide, so the room the line has to show its
	// glyphs is the field less that stroke: a caret at the right edge is visible.
	visible := max(width-1, 1)
	most := max(textWidth-visible, 0)
	offset := min(max(s.offset, 0), most)
	if caretX-offset > visible {
		offset = caretX - visible
	} else if caretX < offset {
		offset = caretX
	}
	offset = min(max(offset, 0), most)
	s.offset = offset
	return offset
}

// Rest puts the line back at the field's start, as a field that loses its value does
// not want the next one scrolled by the last one's caret.
func (s *InputScroll) Rest() { s.offset = 0 }

// Offset reports how far the line is painted left of the field's start, as the last
// Follow left it. A field reads it for what it must do itself, such as reporting
// where its text sits to the platform.
func (s *InputScroll) Offset() float32 { return s.offset }
