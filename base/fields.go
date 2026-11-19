package base

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
	"github.com/vivaui/viva/text"
)

type InputSnapshot struct {
	ID    gfx.ID
	Label string
	// Alignment places the text within the content rectangle. Zero preserves
	// the ordinary direction-dependent start alignment, which is left-to-right
	// for Latin text. AlignRight suits a numeric field. The label, caret,
	// selection, composition and platform editor all take the same alignment.
	Alignment       gfx.Alignment
	Value           string
	DisplayValue    string
	Placeholder     string
	Hint            string
	Disabled        bool
	ReadOnly        bool
	Invalid         bool
	Secure          bool
	Focused         bool
	PlatformEditing bool
	// HideEditorText makes the platform editor behind a focused input invisible and the
	// component paint the text, selection, composition and caret itself; the editor is
	// only where input arrives (give Caret, SelectionStart/End and CompositionStart/End,
	// and set ShowCaret while the field is focused). A platform that lays out its own text
	// can put the line a fraction of a pixel away from the canvas line; with the text
	// painted here it cannot move when the field opens. A fixed-pitch field does this
	// always (see InputTextStyle.Mono).
	HideEditorText bool
	NavigationKeys bool
	ShowCaret      bool
	Caret          int
	SelectionStart int
	SelectionEnd   int
	// SelectionRanges, when nonempty, replaces the single selection for
	// painting: the selections of a multi-selection editor (see
	// text.Editor.SelectionSpansInto). They are byte offsets into Value, in
	// order, non-overlapping, and may sit anywhere on the line. Collapsed ones
	// paint nothing. The caret is still Caret, so only the primary selection has
	// one.
	SelectionRanges  []text.Span
	CompositionStart int
	CompositionEnd   int
	// Text styles the text of the field: its size, weight and color. The zero value is the
	// theme's body size, regular weight and foreground, which is what the shadcn field is.
	Text style.InputTextStyle
	// Blink is the phase of the caret a field paints itself (see InputTextStyle.Mono and
	// HideEditorText): the caret is drawn only while it is lit. Nil keeps it lit.
	Blink *CaretBlink
}
