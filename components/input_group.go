package components

import (
	"github.com/vivaui/viva/ui"
)

// InputGroupCommand is a request for the caller-owned editor. Addon button
// activation continues through its normal stable canvas hit ID.
type InputGroupCommand uint8

const (
	InputGroupCommandNone InputGroupCommand = iota
	InputGroupCommandInsertNewline
)

// InputGroupSemantics is the typed semantic projection of the embedded text
// control. Selection and composition are normalized UTF-8 byte ranges.
type InputGroupSemantics struct {
	ID               ui.ID
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
	Kind       InputGroupSlotKind
	ID         ui.ID
	Icon       ui.Icon
	Text       string
	Label      string
	Hint       string
	Width      float32
	Disabled   bool
	Separated  bool
	ButtonSize InputGroupButtonSize
	// Phase is the spinner frame of an InputGroupSlotSpinner.
	Phase SpinnerPhase
}

// InputGroupSnapshot is one caller-owned input or textarea presentation with
// four explicit shadcn addon positions.
type InputGroupSnapshot struct {
	ID           ui.ID
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
	Secure           bool
	Focused          bool
	Multiline        bool
	NativeEditing    bool
	NavigationKeys   bool
	ShowCaret        bool
	Caret            int
	SelectionStart   int
	SelectionEnd     int
	CompositionStart int
	CompositionEnd   int
	InlineStart      InputGroupSlot
	InlineEnd        InputGroupSlot
	BlockStart       InputGroupSlot
	BlockEnd         InputGroupSlot
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

// InputGroupCommandForKey maps only commands owned by the selected control
// shape. Single-line groups leave Enter to form activation; multiline groups
// request a newline from the caller-owned editor.
func InputGroupCommandForKey(snapshot InputGroupSnapshot, key ui.Key) (InputGroupCommand, bool) {
	if snapshot.Disabled || snapshot.ReadOnly || !snapshot.Multiline {
		return InputGroupCommandNone, false
	}
	if key == ui.KeyEnter {
		return InputGroupCommandInsertNewline, true
	}
	return InputGroupCommandNone, false
}

// InputGroupLayout reports every deterministic region produced by a snapshot.
type InputGroupLayout struct {
	Bounds      ui.Rect
	Control     ui.Rect
	InlineStart ui.Rect
	InlineEnd   ui.Rect
	BlockStart  ui.Rect
	BlockEnd    ui.Rect
}

// The geometry of an input group, after Vega. Inline addons sit inside the
// border with pl-2 / pr-2 (a button -ml-1 / -mr-1 and a kbd about -2 from
// that), and the control keeps pl-1.5 / pr-1.5 next to an addon; without an
// addon its text starts where an Input's does (inputInsetX).
const (
	inputGroupBlockSize    = float32(32)
	inputGroupIconSize     = 16
	inputGroupControlInset = float32(6)
	inputGroupMultilinePad = float32(8)
)

// PaintInputGroup paints one official bordered group. It only describes the
// current editor/slot snapshot; focus changes, editing, and IME stay external.
func PaintInputGroup(canvas *ui.Canvas, theme ui.Theme, bounds ui.Rect, snapshot InputGroupSnapshot) InputGroupLayout {
	blockStartHeight := inputGroupBlockHeight(snapshot.BlockStart)
	blockEndHeight := inputGroupBlockHeight(snapshot.BlockEnd)
	controlHeight := theme.Metrics.ControlHeight
	if snapshot.Multiline {
		controlHeight = 64
	}
	minimumHeight := blockStartHeight + controlHeight + blockEndHeight
	if bounds.Height < minimumHeight {
		bounds.Height = minimumHeight
	}
	layout := InputGroupLayout{Bounds: bounds}
	if canvas == nil || bounds.Width <= 0 || bounds.Height <= 0 {
		return layout
	}

	row := ui.Rect{X: bounds.X, Y: bounds.Y + blockStartHeight, Width: bounds.Width, Height: bounds.Height - blockStartHeight - blockEndHeight}
	startWidth := inputGroupInlineWidth(canvas, theme, snapshot.InlineStart, row.Height)
	endWidth := inputGroupInlineWidth(canvas, theme, snapshot.InlineEnd, row.Height)
	layout.BlockStart = ui.Rect{X: bounds.X, Y: bounds.Y, Width: bounds.Width, Height: blockStartHeight}
	layout.BlockEnd = ui.Rect{X: bounds.X, Y: bounds.Y + bounds.Height - blockEndHeight, Width: bounds.Width, Height: blockEndHeight}
	layout.InlineStart = ui.Rect{X: row.X, Y: row.Y, Width: startWidth, Height: row.Height}
	layout.InlineEnd = ui.Rect{X: row.X + row.Width - endWidth, Y: row.Y, Width: endWidth, Height: row.Height}
	layout.Control = ui.Rect{X: row.X + startWidth, Y: row.Y, Width: inputGroupMax(0, row.Width-startWidth-endWidth), Height: row.Height}

	focused := !snapshot.Disabled && (snapshot.Focused || !snapshot.ID.IsZero() && canvas.Focused() == snapshot.ID)
	paintTextEntrySurface(canvas, theme, bounds, textEntryState{Focused: focused, Invalid: snapshot.Invalid, Disabled: snapshot.Disabled})
	inputGroupPaintSlot(canvas, theme, layout.BlockStart, snapshot.BlockStart, true, false, snapshot.Disabled)
	inputGroupPaintSlot(canvas, theme, layout.BlockEnd, snapshot.BlockEnd, true, false, snapshot.Disabled)
	inputGroupPaintSlot(canvas, theme, layout.InlineStart, snapshot.InlineStart, false, true, snapshot.Disabled)
	inputGroupPaintSlot(canvas, theme, layout.InlineEnd, snapshot.InlineEnd, false, false, snapshot.Disabled)
	content := inputGroupContent(layout.Control, snapshot.Multiline,
		snapshot.InlineStart.Kind != InputGroupSlotNone, snapshot.InlineEnd.Kind != InputGroupSlotNone)
	if !snapshot.NativeEditing && content.Width > 0 && content.Height > 0 {
		inputGroupPaintControl(canvas, theme, content, snapshot, focused)
	}

	if !snapshot.ID.IsZero() && layout.Control.Width > 0 {
		canvas.HitWithOptions(snapshot.ID, layout.Control, snapshot.Label, ui.HitOptions{
			Role: ui.RoleTextField, Cursor: ui.PointerCursorText, Value: snapshot.Value, Hint: snapshot.Hint,
			Disabled: snapshot.Disabled, ReadOnly: snapshot.ReadOnly, Multiline: snapshot.Multiline, Secure: snapshot.Secure,
			NavigationKeys: snapshot.NavigationKeys,
			TextBounds:     content, TextSize: theme.Metrics.FontSizeM, TextColor: theme.Foreground,
		})
	}
	return layout
}

// inputGroupContent is where the control's text sits inside its bounds: the
// same inset as a standalone Input unless an addon is beside it.
func inputGroupContent(control ui.Rect, multiline, hasStart, hasEnd bool) ui.Rect {
	left, right := inputInsetX, inputInsetX
	if hasStart {
		left = inputGroupControlInset
	}
	if hasEnd {
		right = inputGroupControlInset
	}
	content := ui.Rect{X: control.X + left, Y: control.Y, Width: inputGroupMax(0, control.Width-left-right), Height: control.Height}
	if multiline {
		content.Y += inputGroupMultilinePad
		content.Height = inputGroupMax(0, content.Height-2*inputGroupMultilinePad)
	}
	return content
}

func inputGroupBlockHeight(slot InputGroupSlot) float32 {
	if slot.Kind == InputGroupSlotNone {
		return 0
	}
	return inputGroupBlockSize
}

// inputGroupOuterPad is how far an inline addon's content sits from the
// group's outer edge: the border plus the addon's padding, which a button or a
// kbd pulls in (Vega's has-[>button]:-ml-1 and has-[>kbd]:ml-[-0.15rem]).
func inputGroupOuterPad(kind InputGroupSlotKind) float32 {
	switch kind {
	case InputGroupSlotButton:
		return 1 + 4
	case InputGroupSlotKbd:
		return 1 + 6
	default:
		return 1 + 8
	}
}

// inputGroupContentSize is the natural size of an addon's content in a row of
// the given height: what a slot without an explicit Width is as wide as.
func inputGroupContentSize(canvas *ui.Canvas, theme ui.Theme, slot InputGroupSlot, rowHeight float32) (width, height float32) {
	switch slot.Kind {
	case InputGroupSlotIcon, InputGroupSlotSpinner:
		return inputGroupIconSize, inputGroupIconSize
	case InputGroupSlotButton:
		return inputGroupButtonSize(canvas, theme, slot, rowHeight)
	case InputGroupSlotKbd:
		return canvas.TextWidthWeight(slot.Text, theme.Metrics.FontSizeS, ui.WeightMedium) + 12, 20
	default:
		return canvas.TextWidthWeight(slot.Text, theme.Metrics.FontSizeM, ui.WeightMedium), theme.Metrics.FontSizeM + 7
	}
}

// inputGroupButtonSize sizes a button slot. Heights are Button's (xs and
// icon-xs 24, sm 28 at the default density), and icon-sm is size-8, 32, kept inside the
// border of a row of rowHeight; an icon button is square.
func inputGroupButtonSize(canvas *ui.Canvas, theme ui.Theme, slot InputGroupSlot, rowHeight float32) (width, height float32) {
	size := ButtonSizeXSmall
	switch slot.ButtonSize {
	case InputGroupButtonSM:
		size = ButtonSizeSmall
	case InputGroupButtonIconXS:
		size = ButtonSizeIconXSmall
	case InputGroupButtonIconSM:
		size = ButtonSizeIconSmall
	}
	height, _, _, iconOnly := Button{theme: theme, size: size}.geometry()
	if slot.ButtonSize == InputGroupButtonIconSM {
		height = theme.Metrics.ControlHeight // icon-sm is size-8 in the group, whatever Button's icon-sm is
	}
	height = min(height, inputGroupMax(0, rowHeight-2))
	if iconOnly {
		return height, height
	}
	// px-1.5 for xs and the Button small size's px-2.5 for sm, gap-1 between
	// the icon and the text.
	pad := float32(6)
	if slot.ButtonSize == InputGroupButtonSM {
		pad = 10
	}
	width = pad * 2
	if slot.Text != "" {
		width += canvas.TextWidthWeight(slot.Text, theme.Metrics.FontSizeM, ui.WeightMedium)
	}
	if inputGroupHasIcon(slot.Icon) {
		width += inputGroupButtonIcon(slot)
		if slot.Text != "" {
			width += 4
		}
	}
	return width, height
}

func inputGroupHasIcon(icon ui.Icon) bool { return len(icon.Commands) != 0 || len(icon.Layers) != 0 }

// inputGroupButtonIcon is the icon size inside a button slot: 14 in the xs
// text button (svg size-3.5), 16 elsewhere.
func inputGroupButtonIcon(slot InputGroupSlot) float32 {
	if slot.ButtonSize == InputGroupButtonXS {
		return 14
	}
	return inputGroupIconSize
}

func inputGroupInlineWidth(canvas *ui.Canvas, theme ui.Theme, slot InputGroupSlot, rowHeight float32) float32 {
	if slot.Kind == InputGroupSlotNone {
		return 0
	}
	if slot.Width > 0 {
		return slot.Width
	}
	width, _ := inputGroupContentSize(canvas, theme, slot, rowHeight)
	return inputGroupOuterPad(slot.Kind) + width
}

func inputGroupPaintSlot(canvas *ui.Canvas, theme ui.Theme, bounds ui.Rect, slot InputGroupSlot, block, start, groupDisabled bool) {
	if slot.Kind == InputGroupSlotNone || bounds.Width <= 0 || bounds.Height <= 0 {
		return
	}
	disabled := groupDisabled || slot.Disabled
	if slot.Separated {
		if block {
			y := bounds.Y + bounds.Height
			canvas.Fill(ui.Rect{X: bounds.X + 1, Y: y - 1, Width: inputGroupMax(0, bounds.Width-2), Height: 1}, theme.Border)
		} else {
			x := bounds.X + bounds.Width
			canvas.Fill(ui.Rect{X: x - 1, Y: bounds.Y + 6, Width: 1, Height: inputGroupMax(0, bounds.Height-12)}, theme.Border)
		}
	}
	// Where the content goes: a block addon starts at the Input inset
	// (justify-start); an inline one sits between its outer padding and the
	// control, centered when a Width gave it more room than it needs.
	from, to := bounds.X+inputInsetX, bounds.X+bounds.Width-inputInsetX
	if !block {
		pad := inputGroupOuterPad(slot.Kind)
		from, to = bounds.X, bounds.X+bounds.Width
		if start {
			from += pad
		} else {
			to -= pad
		}
	}
	width, height := inputGroupContentSize(canvas, theme, slot, bounds.Height)
	width = min(width, inputGroupMax(0, to-from))
	x := from
	if !block {
		x += (to - from - width) / 2
	}
	content := ui.Rect{X: x, Y: bounds.Y + (bounds.Height-height)/2, Width: width, Height: height}
	color := theme.MutedForeground
	if disabled {
		color.Alpha *= 0.50
	}
	switch slot.Kind {
	case InputGroupSlotButton:
		inputGroupPaintButton(canvas, theme, content, slot, disabled)
	case InputGroupSlotIcon:
		canvas.DrawIcon(slot.Icon, content, color)
	case InputGroupSlotSpinner:
		spinner := theme
		spinner.Foreground = color
		PaintSpinner(canvas, spinner, content, slot.Phase)
	case InputGroupSlotKbd:
		fill := theme.Muted
		if disabled {
			fill.Alpha *= 0.50
		}
		canvas.Rounded(content, theme.Metrics.RadiusItem(), fill)
		canvas.LabelCenter(slot.Text, content, ui.TextStyle{Size: theme.Metrics.FontSizeS, Weight: ui.WeightMedium, Color: color})
	default:
		// A little room either side: a label is clipped to its rectangle, and
		// the measured width of a string is not always the width it draws.
		canvas.LabelCenter(slot.Text, ui.Rect{X: content.X - 3, Y: bounds.Y, Width: content.Width + 6, Height: bounds.Height}, ui.TextStyle{
			Size: theme.Metrics.FontSizeM, Weight: ui.WeightMedium, Color: color,
		})
	}
}

// inputGroupPaintButton paints an addon button: a ghost button that tints its
// background and brightens its content under the pointer, with the icon first
// and the text after it, centered as a unit.
func inputGroupPaintButton(canvas *ui.Canvas, theme ui.Theme, bounds ui.Rect, slot InputGroupSlot, disabled bool) {
	radius := theme.Metrics.RadiusItem()
	enabled := !disabled && !slot.ID.IsZero()
	color := theme.MutedForeground
	if enabled {
		if canvas.FocusVisible() == slot.ID {
			ring := theme.Ring
			ring.Alpha *= 0.50
			canvas.Rounded(expand(bounds, inputRing), radius+inputRing, ring)
		}
		if canvas.Hovered() == slot.ID || canvas.Pressed() == slot.ID {
			canvas.Rounded(bounds, radius, theme.Accent)
			color = theme.Foreground
		}
	}
	if disabled {
		color.Alpha *= 0.50
	}
	style := ui.TextStyle{Size: theme.Metrics.FontSizeM, Weight: ui.WeightMedium, Color: color}
	switch {
	case !inputGroupHasIcon(slot.Icon):
		canvas.LabelCenter(slot.Text, bounds, style)
	case slot.Text == "":
		side := min(inputGroupButtonIcon(slot), bounds.Width, bounds.Height)
		canvas.DrawIcon(slot.Icon, ui.Rect{X: bounds.X + (bounds.Width-side)/2, Y: bounds.Y + (bounds.Height-side)/2, Width: side, Height: side}, color)
	default:
		side := inputGroupButtonIcon(slot)
		text := canvas.TextWidthWeight(slot.Text, style.Size, style.Weight)
		x := bounds.X + (bounds.Width-side-4-text)/2
		canvas.DrawIcon(slot.Icon, ui.Rect{X: x, Y: bounds.Y + (bounds.Height-side)/2, Width: side, Height: side}, color)
		canvas.Label(slot.Text, ui.Rect{X: x + side + 4, Y: bounds.Y, Width: inputGroupMax(0, bounds.X+bounds.Width-x-side-4), Height: bounds.Height}, style)
	}
	if !slot.ID.IsZero() {
		label := slot.Label
		if label == "" {
			label = slot.Text
		}
		canvas.HitWithOptions(slot.ID, bounds, label, ui.HitOptions{Role: ui.RoleButton, Hint: slot.Hint, Disabled: disabled})
	}
}

// inputGroupPaintControl paints the embedded control's text through the same
// painters as a standalone Input (single line, so Secure, bidirectional text
// and the selection look all match) or Textarea (several rows).
func inputGroupPaintControl(canvas *ui.Canvas, theme ui.Theme, content ui.Rect, snapshot InputGroupSnapshot, focused bool) {
	if snapshot.Multiline {
		display := snapshot.DisplayValue
		if display == "" {
			display = snapshot.Value
		}
		textareaPaintRows(canvas, theme, content, display, snapshot.Placeholder, TextareaSnapshot{
			Disabled: snapshot.Disabled, ShowCaret: snapshot.ShowCaret, Caret: snapshot.Caret,
			SelectionStart: snapshot.SelectionStart, SelectionEnd: snapshot.SelectionEnd,
			CompositionStart: snapshot.CompositionStart, CompositionEnd: snapshot.CompositionEnd,
		}, focused)
		return
	}
	inputPaintText(canvas, theme, content, InputSnapshot{
		Value: snapshot.Value, DisplayValue: snapshot.DisplayValue, Placeholder: snapshot.Placeholder,
		Disabled: snapshot.Disabled, Secure: snapshot.Secure, ShowCaret: snapshot.ShowCaret, Caret: snapshot.Caret,
		SelectionStart: snapshot.SelectionStart, SelectionEnd: snapshot.SelectionEnd,
		CompositionStart: snapshot.CompositionStart, CompositionEnd: snapshot.CompositionEnd,
	}, focused)
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

func inputGroupMax(left, right float32) float32 {
	if left > right {
		return left
	}
	return right
}
