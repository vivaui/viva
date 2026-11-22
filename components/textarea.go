package components

import (
	"github.com/vivaui/viva/base"
	"github.com/vivaui/viva/gfx"
)

// TextareaCommandForKey maps only keys owned by a multiline editor. Text
// insertion, deletion, movement, selection extension, and IME remain platform
// or application editor responsibilities.
func TextareaCommandForKey(snapshot TextareaSnapshot, key gfx.Key) (TextareaCommand, bool) {
	if snapshot.Disabled || snapshot.ReadOnly {
		return TextareaCommandNone, false
	}
	if key == gfx.KeyEnter {
		return TextareaCommandInsertNewline, true
	}
	return TextareaCommandNone, false
}

// PaintTextarea paints the official Vega multiline recipe and returns its
// normalized geometry. The official minimum height is 64 logical pixels.
func PaintTextarea(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, snapshot TextareaSnapshot) gfx.Rect {
	if bounds.Height < 64 {
		bounds.Height = 64
	}
	if canvas == nil || bounds.Width <= 0 {
		return bounds
	}
	focused := !snapshot.Disabled && (snapshot.Focused || !snapshot.ID.IsZero() && canvas.Focused() == snapshot.ID)
	paintTextEntrySurface(canvas, &theme, bounds, gfx.State{Focused: focused, Invalid: snapshot.Invalid, Disabled: snapshot.Disabled})
	inset := theme.Metrics.TextInsetX()
	area := gfx.Rect{X: bounds.X + inset, Y: bounds.Y + textareaPadY, Width: textareaMax(0, bounds.Width-2*inset), Height: textareaMax(0, bounds.Height-2*textareaPadY)}
	paintTextareaContent(canvas, theme, bounds, area, snapshot, focused)
	return bounds
}

// TextareaPadY is the room above and below the rows of text in the shadcn textarea.
const TextareaPadY = textareaPadY

// PaintTextareaContent paints what is inside a multiline field and declares its hit
// region, and nothing around it: no border, fill, shadow or focus color. It is what
// PaintTextarea paints for the text, rows, selection, composition and caret, so a field
// the application dresses itself edits and scrolls exactly as the shadcn one does.
//
// bounds is the whole control, which the pointer hits and focus belongs to; area is the
// rectangle the rows are laid out in, the caller's own, which the snapshot's InsetTop,
// InsetBottom and InsetRight then shrink (room for a toolbar or a button inside the box).
func PaintTextareaContent(canvas *gfx.Canvas, theme gfx.Theme, bounds, area gfx.Rect, snapshot TextareaSnapshot) {
	if canvas == nil || bounds.Width <= 0 || bounds.Height <= 0 {
		return
	}
	focused := !snapshot.Disabled && (snapshot.Focused || !snapshot.ID.IsZero() && canvas.Focused() == snapshot.ID)
	paintTextareaContent(canvas, theme, bounds, area, snapshot, focused)
}

func paintTextareaContent(canvas *gfx.Canvas, theme gfx.Theme, bounds, area gfx.Rect, snapshot TextareaSnapshot, focused bool) {
	top := min(max(snapshot.InsetTop, 0), max(area.Height, 0))
	content := gfx.Rect{X: area.X, Y: area.Y + top, Width: textareaMax(0, area.Width-snapshot.InsetRight), Height: textareaMax(0, area.Height-top-snapshot.InsetBottom)}
	display := snapshot.DisplayValue
	if display == "" {
		display = snapshot.Value
	}
	layout := snapshot.Layout
	if layout != nil {
		layout.Painted, layout.CaretShown = false, false
	}
	if !snapshot.NativeEditing {
		textareaPaintRows(canvas, theme, content, display, snapshot.Placeholder, snapshot, focused)
	}
	if !snapshot.ID.IsZero() {
		options := gfx.HitOptions{
			Role: gfx.RoleTextField, Cursor: gfx.PointerCursorText, Value: snapshot.Value, Hint: snapshot.Hint,
			Disabled: snapshot.Disabled, ReadOnly: snapshot.ReadOnly, Multiline: true, Invalid: snapshot.Invalid,
			TextBounds: content, TextSize: textareaHitSize(theme, snapshot), TextColor: theme.Foreground,
		}
		if snapshot.HideEditorText {
			options.TextColor = gfx.Color{}
			options.HideNativeOverlays = true
		}
		if layout != nil && !snapshot.NativeEditing {
			options.Placement = layout
		}
		canvas.HitWithOptions(snapshot.ID, bounds, snapshot.Label, options)
	}
}

// textareaRows walks the rows of the text being painted: the layout's, kept from
// the last frame, or, for a box without one, wrapped one at a time as they are
// reached, which allocates nothing and lets a box that shows its first rows
// skip the rest.
type textareaRows struct {
	layout *TextareaLayout
	canvas *gfx.Canvas
	text   string
	size   float32
	width  float32
	index  int
	from   int
}

// next returns the row at the cursor, where the row after it starts (-1 for
// none), and false past the last.
func (rows *textareaRows) next() (row gfx.TextRow, after int, ok bool) {
	if rows.layout != nil {
		if rows.index >= len(rows.layout.Rows) {
			return gfx.TextRow{}, -1, false
		}
		row, after = rows.layout.Rows[rows.index], -1
		if rows.index+1 < len(rows.layout.Rows) {
			after = rows.layout.Rows[rows.index+1].Start
		}
		rows.index++
		return row, after, true
	}
	if rows.from < 0 {
		return gfx.TextRow{}, -1, false
	}
	row, rows.from = rows.canvas.WrapRow(rows.text, rows.from, rows.size, gfx.WeightRegular, rows.width)
	rows.index++
	return row, rows.from, true
}

// textareaHitSize is the text size a textarea reports to the platform.
func textareaHitSize(theme gfx.Theme, snapshot TextareaSnapshot) float32 {
	size, _ := textareaTextStyle(theme, snapshot)
	return size
}

// textareaTextStyle is the size and weight of a textarea's rows: the
// snapshot's own when set, else the theme's body size at the regular weight.
func textareaTextStyle(theme gfx.Theme, snapshot TextareaSnapshot) (size, weight float32) {
	size, weight = theme.Metrics.FontSizeM, gfx.WeightRegular
	if snapshot.TextSize > 0 {
		size = snapshot.TextSize
	}
	if snapshot.TextWeight > 0 {
		weight = snapshot.TextWeight
	}
	return size, weight
}

func textareaPaintRows(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, value, placeholder string, snapshot TextareaSnapshot, focused bool) {
	fontSize, weight := textareaTextStyle(theme, snapshot)
	lineHeight := fontSize + 7
	placeholderShown := value == ""
	if placeholderShown {
		value = placeholder
	}
	color := theme.Foreground
	if placeholderShown {
		color = theme.MutedForeground
	}
	if snapshot.Disabled {
		color.Alpha *= 0.50
	}
	selectionStart, selectionEnd := textareaRange(value, snapshot.SelectionStart, snapshot.SelectionEnd)
	compositionStart, compositionEnd := textareaRange(value, snapshot.CompositionStart, snapshot.CompositionEnd)
	caret := textareaBoundary(value, snapshot.Caret)
	width := bounds.Width
	if snapshot.NoWrap {
		width = 0
	}

	// The layout follows the text of the box, not its hint.
	layout := snapshot.Layout
	rows := textareaRows{canvas: canvas, text: value, size: fontSize, width: width}
	var scroll float32
	if layout != nil {
		layout.Fit(canvas, snapshot.LayoutText(), fontSize, bounds.Width, !snapshot.NoWrap)
		layout.Content, layout.LineHeight = bounds, lineHeight
		if !placeholderShown {
			layout.Follow(caret, layout.RowOf(caret))
			rows.layout, rows.index = layout, int(layout.Scroll/lineHeight)
			scroll = layout.Scroll
		} else {
			layout.Scroll = 0
			layout.FollowedValid = false
		}
		layout.Painted = true
	}

	upstream := layout != nil && !placeholderShown && layout.Upstream && layout.UpstreamCaret == caret
	// Rows wrap by the reference metrics and the platform draws them with its own
	// advances, a pixel or three wider at the end of a full row: the clip, and the
	// label of a left-aligned row, reach into the padding so that does not cut the
	// last letter.
	clip := bounds
	overhang := theme.Metrics.TextInsetX() - 1
	clip.X -= overhang
	clip.Width += 2 * overhang
	canvas.Clip(clip)
	for {
		row, after, ok := rows.next()
		if !ok {
			break
		}
		index := rows.index - 1
		lineBounds := gfx.Rect{X: bounds.X, Y: bounds.Y + float32(index)*lineHeight - scroll, Width: bounds.Width, Height: lineHeight}
		if lineBounds.Y >= bounds.Y+bounds.Height {
			break
		}
		// A row that wrapped does not draw the spaces left hanging at its end.
		line := value[row.Start:row.End]
		if after == row.End {
			line = trimTrailingSpaces(line)
		}
		lineEnd := row.Start + len(line)
		style := gfx.TextStyle{Size: fontSize, Weight: weight, Color: color}
		origin := canvas.LabelOrigin(line, lineBounds, style, gfx.AlignStart)
		labelBounds := lineBounds
		if origin == lineBounds.X {
			labelBounds.Width += overhang // see clip
		}
		var selections []gfx.Rect
		var ranges [4]gfx.Rect
		// At a wrap, the end of one row is the start of the next: the caret is drawn on
		// the next unless it got to the end of the first by going there.
		holdsCaret := caret >= row.Start && (after < 0 || caret < after)
		if after >= 0 && after == row.End && caret == after {
			holdsCaret = upstream
		}
		if upstream && caret == row.Start && row.Start > 0 && value[row.Start-1] != '\n' {
			holdsCaret = false
		}
		if !placeholderShown {
			if len(snapshot.SelectionRanges) > 0 {
				// A multi-selection editor paints every selection; two on one row
				// show as two bands, and every row a range crosses gets its part.
				selections = ranges[:0]
				for _, span := range snapshot.SelectionRanges {
					start, end := textareaRange(value, span.Start, span.End)
					selections = textareaLineRanges(canvas, selections, labelBounds, origin, line, fontSize, row.Start, lineEnd, start, end)
				}
			} else {
				selections = textareaLineRanges(canvas, ranges[:0], labelBounds, origin, line, fontSize, row.Start, lineEnd, selectionStart, selectionEnd)
			}
			selectionColor, _ := selectionColors(theme)
			if snapshot.Disabled {
				selectionColor.Alpha *= 0.50
			}
			for _, selection := range selections {
				canvas.Fill(selection, selectionColor)
			}
			var underlines [4]gfx.Rect
			underline := theme.Foreground
			if snapshot.Disabled {
				underline.Alpha *= 0.50
			}
			for _, composition := range textareaLineRanges(canvas, underlines[:0], labelBounds, origin, line, fontSize, row.Start, lineEnd, compositionStart, compositionEnd) {
				canvas.Fill(gfx.Rect{X: composition.X, Y: lineBounds.Y + lineHeight - 3, Width: composition.Width, Height: 1}, underline)
			}
			if holdsCaret {
				caretOffset := textareaBoundary(line, min(caret-row.Start, len(line)))
				x := min(origin+canvas.TextCaret(line, fontSize, gfx.WeightRegular, caretOffset), labelBounds.X+labelBounds.Width-1)
				textareaPlaceCaret(canvas, theme, layout, lineBounds, x, focused && snapshot.ShowCaret && !snapshot.Disabled)
			}
		}
		if placeholderShown && index == 0 {
			// An empty box has its caret at the start, in front of the hint.
			textareaPlaceCaret(canvas, theme, layout, lineBounds, origin, focused && snapshot.ShowCaret && !snapshot.Disabled)
		}
		if snapshot.PaintLine != nil {
			snapshot.PaintLine(canvas, line, labelBounds, style)
		} else {
			canvas.Label(line, labelBounds, style)
		}
		for _, selection := range selections {
			if selection.Width <= 0 {
				continue
			}
			canvas.Clip(selection)
			_, selectedColor := selectionColors(theme)
			if snapshot.Disabled {
				selectedColor.Alpha *= 0.50
			}
			canvas.Label(line, labelBounds, gfx.TextStyle{Size: fontSize, Weight: weight, Color: selectedColor})
			canvas.Unclip()
		}
	}
	canvas.Unclip()
}

// textareaPlaceCaret records where the caret is for the runtime and paints it
// when the box shows one.
func textareaPlaceCaret(canvas *gfx.Canvas, theme gfx.Theme, layout *TextareaLayout, row gfx.Rect, x float32, show bool) {
	if layout != nil {
		layout.Caret, layout.CaretShown = gfx.Rect{X: x, Y: row.Y, Width: 1, Height: row.Height}, true
	}
	if show {
		canvas.Fill(gfx.Rect{X: x, Y: row.Y + 2, Width: 1, Height: inputCaretHeight(theme.Metrics.FontSizeM)}, theme.Foreground)
	}
}

// textareaLineRanges appends the rectangles of the part of [start, end) that lies on the
// line (starting at lineStart in the value and drawn from x = origin), one for
// left-to-right text, more where the range crosses a change of direction.
func textareaLineRanges(canvas *gfx.Canvas, dst []gfx.Rect, bounds gfx.Rect, origin float32, line string, size float32, lineStart, lineEnd, start, end int) []gfx.Rect {
	start = max(start, lineStart)
	end = min(end, lineEnd)
	if end <= start {
		return dst
	}
	localStart := textareaBoundary(line, start-lineStart)
	localEnd := textareaBoundary(line, end-lineStart)
	var spans [4]gfx.TextSpan
	for _, span := range canvas.TextSelection(spans[:0], line, size, gfx.WeightRegular, localStart, localEnd) {
		x, width := origin+span.X, span.Width
		if x+width > bounds.X+bounds.Width {
			width = bounds.X + bounds.Width - x
		}
		dst = append(dst, gfx.Rect{X: x, Y: bounds.Y + 1, Width: textareaMax(0, width), Height: textareaMax(0, bounds.Height-2)})
	}
	return dst
}

func textareaRange(value string, start, end int) (int, int) {
	start = textareaBoundary(value, start)
	end = textareaBoundary(value, end)
	if start > end {
		start, end = end, start
	}
	return start, end
}

// selectionColors returns the semantic selection pair. Hand-built themes from
// before the pair existed keep their former Primary-based appearance.
func selectionColors(theme gfx.Theme) (background, foreground gfx.Color) {
	background, foreground = theme.Selection, theme.SelectionForeground
	if background.Alpha == 0 {
		background = theme.Primary
	}
	if foreground.Alpha == 0 {
		foreground = theme.PrimaryForeground
	}
	return background, foreground
}

func textareaBoundary(value string, offset int) int {
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

func textareaMax(left, right float32) float32 {
	if left > right {
		return left
	}
	return right
}

// textareaPadY is the space above the first row and below the last (py-2).
const textareaPadY = 8

func trimTrailingSpaces(value string) string {
	for len(value) > 0 && value[len(value)-1] == ' ' {
		value = value[:len(value)-1]
	}
	return value
}

// Aliases into the style vocabulary.
type TextareaSnapshot = base.TextareaSnapshot
type TextareaSemantics = base.TextareaSemantics
type TextareaMovement = base.TextareaMovement
type TextareaLayout = base.TextareaLayout
type TextareaCommand = base.TextareaCommand

const (
	TextareaCommandInsertNewline = base.TextareaCommandInsertNewline
	TextareaCommandNone          = base.TextareaCommandNone
	TextareaMoveDown             = base.TextareaMoveDown
	TextareaMoveLineEnd          = base.TextareaMoveLineEnd
	TextareaMoveLineStart        = base.TextareaMoveLineStart
	TextareaMoveNone             = base.TextareaMoveNone
	TextareaMovePageDown         = base.TextareaMovePageDown
	TextareaMovePageUp           = base.TextareaMovePageUp
	TextareaMoveTextEnd          = base.TextareaMoveTextEnd
	TextareaMoveTextStart        = base.TextareaMoveTextStart
	TextareaMoveUp               = base.TextareaMoveUp
)

func TextareaMovementForKey(key gfx.Key, modifiers gfx.Modifiers) (TextareaMovement, bool) {
	return base.TextareaMovementForKey(key, modifiers)
}
