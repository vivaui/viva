package components

import (
	"github.com/vivaui/viva/base"
	"github.com/vivaui/viva/gfx"
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
type InputGroupSemantics = base.InputGroupSemantics
type InputGroupSlotKind = base.InputGroupSlotKind

const (
	InputGroupSlotNone    = base.InputGroupSlotNone
	InputGroupSlotText    = base.InputGroupSlotText
	InputGroupSlotButton  = base.InputGroupSlotButton
	InputGroupSlotKbd     = base.InputGroupSlotKbd
	InputGroupSlotIcon    = base.InputGroupSlotIcon
	InputGroupSlotSpinner = base.InputGroupSlotSpinner
)

type InputGroupButtonSize = base.InputGroupButtonSize

const (
	InputGroupButtonXS     = base.InputGroupButtonXS
	InputGroupButtonSM     = base.InputGroupButtonSM
	InputGroupButtonIconXS = base.InputGroupButtonIconXS
	InputGroupButtonIconSM = base.InputGroupButtonIconSM
)

type InputGroupSlot = base.InputGroupSlot
type InputGroupSnapshot = base.InputGroupSnapshot

// InputGroupCommandForKey maps only commands owned by the selected control
// shape. Single-line groups leave Enter to form activation; multiline groups
// request a newline from the caller-owned editor.
func InputGroupCommandForKey(snapshot InputGroupSnapshot, key gfx.Key) (InputGroupCommand, bool) {
	if snapshot.Disabled || snapshot.ReadOnly || !snapshot.Multiline {
		return InputGroupCommandNone, false
	}
	if key == gfx.KeyEnter {
		return InputGroupCommandInsertNewline, true
	}
	return InputGroupCommandNone, false
}

// InputGroupLayout reports every deterministic region produced by a snapshot.
type InputGroupLayout struct {
	Bounds      gfx.Rect
	Control     gfx.Rect
	InlineStart gfx.Rect
	InlineEnd   gfx.Rect
	BlockStart  gfx.Rect
	BlockEnd    gfx.Rect
}

// The geometry of an input group, after Vega. Inline addons sit inside the
// border with pl-2 / pr-2 (a button -ml-1 / -mr-1 and a kbd about -2 from
// that), and the control keeps pl-1.5 / pr-1.5 next to an addon; without an
// addon its text starts where an Input's does (Metrics.TextInsetX).
const (
	inputGroupBlockSize    = float32(32)
	inputGroupIconSize     = 16
	inputGroupControlInset = float32(6)
	inputGroupMultilinePad = float32(8)
)

// PaintInputGroup paints one official bordered group. It only describes the
// current editor/slot snapshot; focus changes, editing, and IME stay external.
func PaintInputGroup(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, snapshot InputGroupSnapshot) InputGroupLayout {
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

	row := gfx.Rect{X: bounds.X, Y: bounds.Y + blockStartHeight, Width: bounds.Width, Height: bounds.Height - blockStartHeight - blockEndHeight}
	startWidth := inputGroupInlineWidth(canvas, theme, snapshot.InlineStart, row.Height)
	endWidth := inputGroupInlineWidth(canvas, theme, snapshot.InlineEnd, row.Height)
	layout.BlockStart = gfx.Rect{X: bounds.X, Y: bounds.Y, Width: bounds.Width, Height: blockStartHeight}
	layout.BlockEnd = gfx.Rect{X: bounds.X, Y: bounds.Y + bounds.Height - blockEndHeight, Width: bounds.Width, Height: blockEndHeight}
	layout.InlineStart = gfx.Rect{X: row.X, Y: row.Y, Width: startWidth, Height: row.Height}
	layout.InlineEnd = gfx.Rect{X: row.X + row.Width - endWidth, Y: row.Y, Width: endWidth, Height: row.Height}
	layout.Control = gfx.Rect{X: row.X + startWidth, Y: row.Y, Width: inputGroupMax(0, row.Width-startWidth-endWidth), Height: row.Height}

	focused := !snapshot.Disabled && (snapshot.Focused || !snapshot.ID.IsZero() && canvas.Focused() == snapshot.ID)
	paintTextEntrySurface(canvas, &theme, bounds, gfx.State{Focused: focused, Invalid: snapshot.Invalid, Disabled: snapshot.Disabled})
	inputGroupPaintSlot(canvas, theme, layout.BlockStart, snapshot.BlockStart, true, false, snapshot.Disabled)
	inputGroupPaintSlot(canvas, theme, layout.BlockEnd, snapshot.BlockEnd, true, false, snapshot.Disabled)
	inputGroupPaintSlot(canvas, theme, layout.InlineStart, snapshot.InlineStart, false, true, snapshot.Disabled)
	inputGroupPaintSlot(canvas, theme, layout.InlineEnd, snapshot.InlineEnd, false, false, snapshot.Disabled)
	content := inputGroupContent(theme, layout.Control, snapshot.Multiline,
		snapshot.InlineStart.Kind != InputGroupSlotNone, snapshot.InlineEnd.Kind != InputGroupSlotNone)
	if (!snapshot.NativeEditing || snapshot.HideEditorText) && content.Width > 0 && content.Height > 0 {
		inputGroupPaintControl(canvas, theme, content, snapshot, focused)
	}

	if !snapshot.ID.IsZero() && layout.Control.Width > 0 {
		// A single line is centered in the field's text band, where the caret and
		// selection live. A multi-line value starts at the content's top, so its
		// hit bounds are the whole content rectangle.
		textBounds := content
		if !snapshot.Multiline {
			lineTop, lineHeight := inputLineBox(content, theme.Metrics.FontSizeM)
			textBounds = gfx.Rect{X: content.X, Y: lineTop, Width: content.Width, Height: lineHeight}
		}
		options := gfx.HitOptions{
			Role: gfx.RoleTextField, Cursor: gfx.PointerCursorText, Value: snapshot.Value, Hint: snapshot.Hint,
			Disabled: snapshot.Disabled, ReadOnly: snapshot.ReadOnly, Multiline: snapshot.Multiline, Secure: snapshot.Secure,
			Invalid:        snapshot.Invalid,
			NavigationKeys: snapshot.NavigationKeys,
			TextBounds:     textBounds, TextSize: theme.Metrics.FontSizeM, TextColor: theme.Foreground,
		}
		if snapshot.HideEditorText {
			options.TextColor = gfx.Color{}
			options.HideNativeOverlays = true
		}
		canvas.HitWithOptions(snapshot.ID, layout.Control, snapshot.Label, options)
	}
	return layout
}

// inputGroupContent is where the control's text sits inside its bounds: the
// same inset as a standalone Input unless an addon is beside it.
// InputAddonsLayout places an inline addon at each end of a single-line field in
// bounds, as PaintInputGroup does, and returns their rectangles and the one the
// text is centered in between them. A slot of kind InputGroupSlotNone takes no room.
func InputAddonsLayout(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, start, end InputGroupSlot) (startBounds, endBounds, content gfx.Rect) {
	startWidth := inputGroupInlineWidth(canvas, theme, start, bounds.Height)
	endWidth := inputGroupInlineWidth(canvas, theme, end, bounds.Height)
	startBounds = gfx.Rect{X: bounds.X, Y: bounds.Y, Width: startWidth, Height: bounds.Height}
	endBounds = gfx.Rect{X: bounds.X + bounds.Width - endWidth, Y: bounds.Y, Width: endWidth, Height: bounds.Height}
	control := gfx.Rect{X: bounds.X + startWidth, Y: bounds.Y, Width: inputGroupMax(0, bounds.Width-startWidth-endWidth), Height: bounds.Height}
	content = inputGroupContent(theme, control, false, start.Kind != InputGroupSlotNone, end.Kind != InputGroupSlotNone)
	return startBounds, endBounds, content
}

// PaintInputAddon paints one inline addon of a field in the rectangle
// InputAddonsLayout gave it, start or end, and declares the hit of a button. Paint
// it after the field's own hit, so that a press on the button is the button's.
func PaintInputAddon(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, slot InputGroupSlot, start, disabled bool) {
	inputGroupPaintSlot(canvas, theme, bounds, slot, false, start, disabled)
}

func inputGroupContent(theme gfx.Theme, control gfx.Rect, multiline, hasStart, hasEnd bool) gfx.Rect {
	left := theme.Metrics.TextInsetX()
	right := left
	if hasStart {
		left = inputGroupControlInset
	}
	if hasEnd {
		right = inputGroupControlInset
	}
	content := gfx.Rect{X: control.X + left, Y: control.Y, Width: inputGroupMax(0, control.Width-left-right), Height: control.Height}
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
func inputGroupContentSize(canvas *gfx.Canvas, theme gfx.Theme, slot InputGroupSlot, rowHeight float32) (width, height float32) {
	switch slot.Kind {
	case InputGroupSlotIcon, InputGroupSlotSpinner:
		return inputGroupIconSize, inputGroupIconSize
	case InputGroupSlotButton:
		return inputGroupButtonSize(canvas, theme, slot, rowHeight)
	case InputGroupSlotKbd:
		return canvas.TextWidthWeight(slot.Text, theme.Metrics.FontSizeS, gfx.WeightMedium) + 12, 20
	default:
		return canvas.TextWidthWeight(slot.Text, theme.Metrics.FontSizeM, gfx.WeightMedium), theme.Metrics.FontSizeM + 7
	}
}

// inputGroupButtonSize sizes a button slot. Heights are Button's (xs and
// icon-xs 24, sm 28 at the default density), and icon-sm is size-8, 32, kept inside the
// border of a row of rowHeight; an icon button is square.
func inputGroupButtonSize(canvas *gfx.Canvas, theme gfx.Theme, slot InputGroupSlot, rowHeight float32) (width, height float32) {
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
		width += canvas.TextWidthWeight(slot.Text, theme.Metrics.FontSizeM, gfx.WeightMedium)
	}
	if inputGroupHasIcon(slot.Icon) {
		width += inputGroupButtonIcon(slot)
		if slot.Text != "" {
			width += 4
		}
	}
	return width, height
}

func inputGroupHasIcon(icon gfx.Icon) bool { return len(icon.Commands) != 0 || len(icon.Layers) != 0 }

// inputGroupButtonIcon is the icon size inside a button slot: 14 in the xs
// text button (svg size-3.5), 16 elsewhere.
func inputGroupButtonIcon(slot InputGroupSlot) float32 {
	if slot.ButtonSize == InputGroupButtonXS {
		return 14
	}
	return inputGroupIconSize
}

func inputGroupInlineWidth(canvas *gfx.Canvas, theme gfx.Theme, slot InputGroupSlot, rowHeight float32) float32 {
	if slot.Kind == InputGroupSlotNone {
		return 0
	}
	if slot.Width > 0 {
		return slot.Width
	}
	width, _ := inputGroupContentSize(canvas, theme, slot, rowHeight)
	return inputGroupOuterPad(slot.Kind) + width
}

func inputGroupPaintSlot(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, slot InputGroupSlot, block, start, groupDisabled bool) {
	if slot.Kind == InputGroupSlotNone || bounds.Width <= 0 || bounds.Height <= 0 {
		return
	}
	disabled := groupDisabled || slot.Disabled
	if slot.Separated {
		if block {
			y := bounds.Y + bounds.Height
			canvas.Fill(gfx.Rect{X: bounds.X + 1, Y: y - 1, Width: inputGroupMax(0, bounds.Width-2), Height: 1}, theme.Border)
		} else {
			x := bounds.X + bounds.Width
			canvas.Fill(gfx.Rect{X: x - 1, Y: bounds.Y + 6, Width: 1, Height: inputGroupMax(0, bounds.Height-12)}, theme.Border)
		}
	}
	// Where the content goes: a block addon starts at the Input inset
	// (justify-start); an inline one sits between its outer padding and the
	// control, centered when a Width gave it more room than it needs.
	inset := theme.Metrics.TextInsetX()
	from, to := bounds.X+inset, bounds.X+bounds.Width-inset
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
	content := gfx.Rect{X: x, Y: bounds.Y + (bounds.Height-height)/2, Width: width, Height: height}
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
		canvas.LabelCenter(slot.Text, content, gfx.TextStyle{Size: theme.Metrics.FontSizeS, Weight: gfx.WeightMedium, Color: color})
	default:
		// A little room either side: a label is clipped to its rectangle, and
		// the measured width of a string is not always the width it draws.
		canvas.LabelCenter(slot.Text, gfx.Rect{X: content.X - 3, Y: bounds.Y, Width: content.Width + 6, Height: bounds.Height}, gfx.TextStyle{
			Size: theme.Metrics.FontSizeM, Weight: gfx.WeightMedium, Color: color,
		})
	}
}

// inputGroupPaintButton paints an addon button: a ghost button that tints its
// background and brightens its content under the pointer, with the icon first
// and the text after it, centered as a unit.
func inputGroupPaintButton(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, slot InputGroupSlot, disabled bool) {
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
	style := gfx.TextStyle{Size: theme.Metrics.FontSizeM, Weight: gfx.WeightMedium, Color: color}
	switch {
	case !inputGroupHasIcon(slot.Icon):
		canvas.LabelCenter(slot.Text, bounds, style)
	case slot.Text == "":
		side := min(inputGroupButtonIcon(slot), bounds.Width, bounds.Height)
		canvas.DrawIconTransformed(slot.Icon, gfx.Rect{X: bounds.X + (bounds.Width-side)/2, Y: bounds.Y + (bounds.Height-side)/2, Width: side, Height: side}, color, slot.IconTransform)
	default:
		side := inputGroupButtonIcon(slot)
		text := canvas.TextWidthWeight(slot.Text, style.Size, style.Weight)
		x := bounds.X + (bounds.Width-side-4-text)/2
		canvas.DrawIconTransformed(slot.Icon, gfx.Rect{X: x, Y: bounds.Y + (bounds.Height-side)/2, Width: side, Height: side}, color, slot.IconTransform)
		canvas.Label(slot.Text, gfx.Rect{X: x + side + 4, Y: bounds.Y, Width: inputGroupMax(0, bounds.X+bounds.Width-x-side-4), Height: bounds.Height}, style)
	}
	if !slot.ID.IsZero() {
		label := slot.Label
		if label == "" {
			label = slot.Text
		}
		canvas.HitWithOptions(slot.ID, bounds, label, gfx.HitOptions{Role: gfx.RoleButton, Hint: slot.Hint, Disabled: disabled})
	}
}

// inputGroupPaintControl paints the embedded control's text through the same
// painters as a standalone Input (single line, so Secure, bidirectional text
// and the selection look all match) or Textarea (several rows).
func inputGroupPaintControl(canvas *gfx.Canvas, theme gfx.Theme, content gfx.Rect, snapshot InputGroupSnapshot, focused bool) {
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
	showCaret := snapshot.ShowCaret
	if focused && showCaret && snapshot.Blink != nil {
		showCaret = snapshot.Blink.Lit(canvas, snapshot.Caret)
	}
	inputPaintText(canvas, theme, content, InputSnapshot{
		Value: snapshot.Value, DisplayValue: snapshot.DisplayValue, Placeholder: snapshot.Placeholder,
		Disabled: snapshot.Disabled, Secure: snapshot.Secure, ShowCaret: showCaret, Caret: snapshot.Caret,
		SelectionStart: snapshot.SelectionStart, SelectionEnd: snapshot.SelectionEnd,
		CompositionStart: snapshot.CompositionStart, CompositionEnd: snapshot.CompositionEnd,
	}, focused)
}

func inputGroupMax(left, right float32) float32 {
	if left > right {
		return left
	}
	return right
}
