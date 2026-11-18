package base

import "github.com/vivaui/viva/gfx"

type SpinnerPhase uint8

type InputGroupSemantics struct {
	ID               gfx.ID
	Label            string
	Value            string
	Hint             string
	Focused          bool
	Disabled         bool
	ReadOnly         bool
	Invalid          bool
	Multiline        bool
	SelectionStart   int
	SelectionEnd     int
	CompositionStart int
	CompositionEnd   int
}

// InputGroupSlotKind selects one explicit source-owned addon treatment.
type InputGroupSlotKind uint8

const (
	InputGroupSlotNone InputGroupSlotKind = iota
	InputGroupSlotText
	InputGroupSlotButton
	InputGroupSlotKbd
	// InputGroupSlotIcon shows a compiled icon, such as the magnifier of a
	// search field. It is decoration: it has no hit region.
	InputGroupSlotIcon
	// InputGroupSlotSpinner shows the loading indicator of a field that is
	// searching; InputGroupSlot.Phase picks the frame, and advancing it is the
	// caller's job (see PaintSpinner).
	InputGroupSlotSpinner
)

// InputGroupButtonSize selects the size of an InputGroupSlotButton, named as
// shadcn's InputGroupButton sizes are. The zero value is the default, xs.
type InputGroupButtonSize uint8

const (
	// InputGroupButtonXS is the compact text button: 24 high at the default
	// density, with the icon (if any) before the text.
	InputGroupButtonXS InputGroupButtonSize = iota
	// InputGroupButtonSM is the 32 high text button (Button's small size);
	// inside a group of one control height it is limited to the room within
	// the border.
	InputGroupButtonSM
	// InputGroupButtonIconXS is the 24 pixel square icon button, the clear or
	// reveal button of a field.
	InputGroupButtonIconXS
	// InputGroupButtonIconSM is the 32 pixel square icon button (limited like
	// InputGroupButtonSM).
	InputGroupButtonIconSM
)

// InputGroupSlot describes one addon. Width is optional for inline slots: it is
// the content's own width when zero. Block slots always span the group and
// start their content at its left. Button IDs remain application-owned.
//
// A button shows Icon, Text or both (icon first), sized by ButtonSize; an icon
// button should carry a Label, its accessible name. It is a ghost button, like
// shadcn's InputGroupButton default.
type InputGroupSlot struct {
	Kind InputGroupSlotKind
	ID   gfx.ID
	Icon gfx.Icon
	// IconTransform moves the painted icon inside its addon without moving the
	// addon, its label or its hit region.
	IconTransform gfx.IconTransform
	Text          string
	Label         string
	Hint          string
	Width         float32
	Disabled      bool
	Separated     bool
	ButtonSize    InputGroupButtonSize
	// Phase is the spinner frame of an InputGroupSlotSpinner.
	Phase SpinnerPhase
}

// InputGroupSnapshot is one caller-owned input or textarea presentation with
// four explicit shadcn addon positions.
type InputGroupSnapshot struct {
	ID           gfx.ID
	Label        string
	Value        string
	DisplayValue string
	Placeholder  string
	Hint         string
	Disabled     bool
	ReadOnly     bool
	Invalid      bool
	// Secure makes a single-line control a password field, as for
	// InputSnapshot.Secure.
	Secure        bool
	Focused       bool
	Multiline     bool
	NativeEditing bool
	// HideEditorText makes the platform editor behind a focused control invisible and the
	// component paint the text, selection, composition and caret itself; the editor is only
	// where input arrives (give Caret, SelectionStart/End and CompositionStart/End, and set
	// ShowCaret while the control is focused). See InputSnapshot.HideEditorText.
	HideEditorText   bool
	NavigationKeys   bool
	ShowCaret        bool
	Caret            int
	SelectionStart   int
	SelectionEnd     int
	CompositionStart int
	CompositionEnd   int
	// Blink is the phase of the caret the component paints for a single-line control;
	// see InputSnapshot.Blink. A multi-line control paints its caret steady.
	Blink       *CaretBlink
	InlineStart InputGroupSlot
	InlineEnd   InputGroupSlot
	BlockStart  InputGroupSlot
	BlockEnd    InputGroupSlot
}

// Semantics returns a normalized, allocation-free semantic snapshot for the
// embedded editor without introducing an editor runtime into the component.
func (snapshot InputGroupSnapshot) Semantics() InputGroupSemantics {
	display := snapshot.DisplayValue
	if display == "" {
		display = snapshot.Value
	}
	selectionStart, selectionEnd := inputGroupRange(display, snapshot.SelectionStart, snapshot.SelectionEnd)
	compositionStart, compositionEnd := inputGroupRange(display, snapshot.CompositionStart, snapshot.CompositionEnd)
	return InputGroupSemantics{
		ID: snapshot.ID, Label: snapshot.Label, Value: snapshot.Value, Hint: snapshot.Hint,
		Focused: snapshot.Focused && !snapshot.Disabled, Disabled: snapshot.Disabled,
		ReadOnly: snapshot.ReadOnly, Invalid: snapshot.Invalid, Multiline: snapshot.Multiline,
		SelectionStart: selectionStart, SelectionEnd: selectionEnd,
		CompositionStart: compositionStart, CompositionEnd: compositionEnd,
	}
}

func inputGroupRange(value string, start, end int) (int, int) {
	start = inputGroupBoundary(value, start)
	end = inputGroupBoundary(value, end)
	if start > end {
		start, end = end, start
	}
	return start, end
}

func inputGroupBoundary(value string, offset int) int {
	if offset <= 0 {
		return 0
	}
	if offset >= len(value) {
		return len(value)
	}
	for offset > 0 && value[offset]&0xc0 == 0x80 {
		offset--
	}
	return offset
}
