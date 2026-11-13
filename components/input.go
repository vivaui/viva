package components

import (
	"unicode/utf8"

	"github.com/vivaui/viva/gfx"
)

// InputSnapshot is one caller-owned single-line editor presentation. Value is
// exposed to accessibility; DisplayValue may contain masking or IME preedit
// text without transferring editing ownership to the component. Secure makes
// it a password field: the value is never exposed, DisplayValue is ignored,
// and the field paints one bullet per character.
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
	HideEditorText   bool
	NavigationKeys   bool
	ShowCaret        bool
	Caret            int
	SelectionStart   int
	SelectionEnd     int
	CompositionStart int
	CompositionEnd   int
	// Text styles the text of the field: its size, weight and color. The zero value is the
	// theme's body size, regular weight and foreground, which is what the shadcn field is.
	Text InputTextStyle
	// Blink is the phase of the caret a field paints itself (see InputTextStyle.Mono and
	// HideEditorText): the caret is drawn only while it is lit. Nil keeps it lit.
	Blink *CaretBlink
}

// InputTextStyle is how the text of a single-line field is set. One value drives everything
// that depends on it, so they cannot disagree: the glyphs painted, the position of the caret,
// the selection and the composition underline, the ellipsis a long placeholder ends in, the
// height of the caret, and the text geometry (size, weight and color) declared to the
// platform's text service.
type InputTextStyle struct {
	// Size is the font size in points; zero is the theme's body size.
	Size float32
	// Weight is the font weight, 100 to 900 as in gfx.TextStyle; zero is regular.
	Weight float32
	// Color is the color of the text; the zero color, which is transparent, is the theme's
	// foreground.
	Color gfx.Color
	// Mono sets the text in a fixed-pitch face, MonoFace choosing which (gfx.TextStyle's own).
	// The platform's text service draws one proportional face, so a mono field is painted
	// by the component: its glyphs, selection, composition underline and caret are drawn in
	// the field, and the platform is kept for what only it does (typing, the input method
	// and the clipboard) with its own text and overlays hidden. The snapshot must then say
	// where the caret, the selection and the composition are (Caret, SelectionStart and so
	// on), and may give the caret's blink phase in Blink.
	Mono     bool
	MonoFace gfx.MonoFace
}

// style is the gfx.TextStyle the field's text is drawn and measured in under a theme: the zero
// size, weight and color are the theme's body size, regular weight and foreground.
func (s InputTextStyle) style(theme *gfx.Theme) gfx.TextStyle {
	style := gfx.TextStyle{Size: s.Size, Weight: s.Weight, Color: s.Color, Mono: s.Mono, MonoFace: s.MonoFace}
	if style.Size <= 0 {
		style.Size = theme.Metrics.FontSizeM
	}
	if style.Weight <= 0 {
		style.Weight = gfx.WeightRegular
	}
	if style.Color == (gfx.Color{}) {
		style.Color = theme.Foreground
	}
	return style
}

// PaintInput paints the official Vega input recipe and returns its normalized
// geometry. Byte offsets describe DisplayValue (or Value when DisplayValue is
// empty) and are clamped to UTF-8 boundaries before painting.
func PaintInput(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, snapshot InputSnapshot) gfx.Rect {
	if bounds.Height <= 0 {
		bounds.Height = theme.Metrics.ControlHeight
	}
	if canvas == nil || bounds.Width <= 0 || bounds.Height <= 0 {
		return bounds
	}

	focused := snapshot.Focused || !snapshot.ID.IsZero() && canvas.Focused() == snapshot.ID
	paintTextEntrySurface(canvas, &theme, bounds, gfx.State{Focused: focused, Invalid: snapshot.Invalid, Disabled: snapshot.Disabled})
	inset := theme.Metrics.TextInsetX()
	content := gfx.Rect{X: bounds.X + inset, Y: bounds.Y, Width: inputMax(0, bounds.Width-2*inset), Height: bounds.Height}
	paintInputContent(canvas, theme, bounds, content, snapshot, focused)
	return bounds
}

// PaintInputContent paints what is inside a single-line field and declares its
// hit region, and nothing around it: no border, fill, shadow or focus color.
// Everything PaintInput does for the text is done here, from the same code, so
// a field the application dresses itself edits, selects, composes and masks
// exactly as the shadcn one does.
//
// bounds is the whole control, which the pointer hits and focus belongs to;
// content is the rectangle the text is centered in, the caller's own, so its
// padding is the caller's to choose. The platform's caret and input-method
// geometry follow the line painted in it, so the value does not move when the
// field opens. A
// zero snapshot ID paints a static preview without a hit region. A content
// rectangle with no room (padding larger than the control) paints no text, and the
// control is still a field the pointer and the keyboard reach.
func PaintInputContent(canvas *gfx.Canvas, theme gfx.Theme, bounds, content gfx.Rect, snapshot InputSnapshot) {
	if canvas == nil || bounds.Width <= 0 || bounds.Height <= 0 {
		return
	}
	focused := snapshot.Focused || !snapshot.ID.IsZero() && canvas.Focused() == snapshot.ID
	paintInputContent(canvas, theme, bounds, content, snapshot, focused)
}

// paintInputContent paints the text of a field and declares its hit over bounds.
// The text bounds the platform is told are the line box centered in content, which
// is where the glyphs are painted: a content rectangle exactly one line tall is its
// own line box, and a taller one (a padded field, or the whole control) is not.
func paintInputContent(canvas *gfx.Canvas, theme gfx.Theme, bounds, content gfx.Rect, snapshot InputSnapshot, focused bool) {
	if snapshot.Alignment == gfx.AlignLeft {
		snapshot.Alignment = gfx.AlignStart
	}
	style := snapshot.Text.style(&theme)
	// A mono field is painted by the component whatever the snapshot says of the platform: the
	// platform's text service cannot draw its face, so its text and overlays are hidden. A
	// field that asked to hide the editor's text is painted by the component too, and the
	// platform keeps only typing, the input method and the clipboard.
	selfPainted := snapshot.Text.Mono || snapshot.HideEditorText
	if snapshot.HideEditorText {
		snapshot.PlatformEditing = false
	}
	if snapshot.Text.Mono {
		snapshot.PlatformEditing = false
		snapshot.ShowCaret = focused
	}
	// A caret the component paints keeps its own blink phase, whether the field is fixed
	// pitch or asked to hide the editor's text; the platform's caret is not drawn, so its
	// blink would otherwise be gone.
	if selfPainted && focused && snapshot.ShowCaret && snapshot.Blink != nil {
		snapshot.ShowCaret = snapshot.Blink.Lit(canvas, snapshot.Caret)
	}
	if !snapshot.PlatformEditing && content.Width > 0 && content.Height > 0 {
		inputPaintText(canvas, theme, content, snapshot, focused)
	}
	if snapshot.ID.IsZero() {
		return
	}
	size, weight, ink := style.Size, style.Weight, style.Color
	// The platform draws no text of a field the component paints: its ink is transparent.
	platformInk := ink
	if selfPainted {
		platformInk = gfx.Color{}
	}
	lineTop, lineHeight := inputLineBox(content, size)
	textBounds := gfx.Rect{X: content.X, Y: lineTop, Width: content.Width, Height: lineHeight}
	canvas.HitWithOptions(snapshot.ID, bounds, snapshot.Label, gfx.HitOptions{
		Role: gfx.RoleTextField, Cursor: gfx.PointerCursorText, Value: snapshot.Value, Hint: snapshot.Hint,
		Disabled: snapshot.Disabled, ReadOnly: snapshot.ReadOnly, Secure: snapshot.Secure,
		NavigationKeys: snapshot.NavigationKeys,
		TextBounds:     textBounds, TextSize: size, TextWeight: weight, TextColor: platformInk, TextAlignment: snapshot.Alignment,
		HideNativeOverlays: selfPainted,
	})
}

// inputPaintText paints the value (or the placeholder) of a single-line field
// with its selection, composition and caret. A Secure field shows one bullet per
// character, and the offsets of the snapshot, which index the real text, are
// mapped onto the bullets. InputGroup paints its embedded single-line control
// through the same function, so the two cannot drift apart.
func inputPaintText(canvas *gfx.Canvas, theme gfx.Theme, content gfx.Rect, snapshot InputSnapshot, focused bool) {
	display := snapshot.DisplayValue
	if display == "" {
		display = snapshot.Value
	}
	if snapshot.Secure {
		display = inputMask(snapshot.Value)
		snapshot.Caret = inputMaskOffset(snapshot.Value, snapshot.Caret)
		snapshot.SelectionStart = inputMaskOffset(snapshot.Value, snapshot.SelectionStart)
		snapshot.SelectionEnd = inputMaskOffset(snapshot.Value, snapshot.SelectionEnd)
		snapshot.CompositionStart = inputMaskOffset(snapshot.Value, snapshot.CompositionStart)
		snapshot.CompositionEnd = inputMaskOffset(snapshot.Value, snapshot.CompositionEnd)
	}
	inputPaintLine(canvas, theme, content, display, snapshot.Placeholder, snapshot, focused)
}

func inputPaintLine(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, value, placeholder string, snapshot InputSnapshot, focused bool) {
	textStyle := snapshot.Text.style(&theme)
	fontSize, ink := textStyle.Size, textStyle.Color
	color := ink
	if value == "" {
		value = placeholder
		color = theme.MutedForeground
		// A placeholder that does not fit ends in an ellipsis, as it does in a native
		// field, instead of being cut mid-word by the field's edge.
		value = inputFitPlaceholder(canvas, value, textStyle, bounds.Width)
	}
	if snapshot.Disabled {
		color.Alpha *= 0.50
	}
	canvas.Clip(bounds)
	bandTop, bandHeight := inputLineBand(bounds, fontSize)
	placeholderShown := snapshot.Value == "" && snapshot.DisplayValue == ""
	style := textStyle
	style.Color = color
	// The caret, selection and composition share the glyphs' origin and follow
	// the visual order of mixed text (a selection may be several stretches).
	origin := canvas.LabelOrigin(value, bounds, style, snapshot.Alignment)
	var ranges [4]gfx.Rect
	selectionStart, selectionEnd := inputRange(value, snapshot.SelectionStart, snapshot.SelectionEnd)
	var selections []gfx.Rect
	if selectionEnd > selectionStart && !placeholderShown {
		selections = inputTextRanges(canvas, ranges[:0], bounds, bandTop, bandHeight, origin, value, textStyle, selectionStart, selectionEnd)
		selectionColor, _ := selectionColors(theme)
		if snapshot.Disabled {
			selectionColor.Alpha *= 0.50
		}
		for _, selection := range selections {
			canvas.Fill(selection, selectionColor)
		}
	}
	compositionStart, compositionEnd := inputRange(value, snapshot.CompositionStart, snapshot.CompositionEnd)
	if compositionEnd > compositionStart && !placeholderShown {
		var underlines [4]gfx.Rect
		underline := ink
		if snapshot.Disabled {
			underline.Alpha *= 0.50
		}
		for _, composition := range inputTextRanges(canvas, underlines[:0], bounds, bandTop, bandHeight, origin, value, textStyle, compositionStart, compositionEnd) {
			canvas.Fill(gfx.Rect{X: composition.X, Y: bandTop + bandHeight - 1, Width: composition.Width, Height: 1}, underline)
		}
	}
	if focused && snapshot.ShowCaret && !snapshot.Disabled {
		caret := inputBoundary(value, snapshot.Caret)
		x := origin + canvas.TextCaretStyle(value, textStyle, caret)
		caretHeight := min(inputCaretHeight(fontSize), bandHeight)
		canvas.Fill(gfx.Rect{X: x, Y: bandTop + (bandHeight-caretHeight)/2, Width: 1, Height: caretHeight}, ink)
	}
	// The line is centered in the field like the caret and the selection, but its
	// box is the text's line box (20px at the 14px body size), not the taller
	// band the caret and selection fill: accessibility and parity report label
	// bounds, and those should describe the glyph line the field draws.
	lineTop, lineHeight := inputLineBox(bounds, fontSize)
	line := gfx.Rect{X: bounds.X, Y: lineTop, Width: bounds.Width, Height: lineHeight}
	canvas.LabelAligned(value, line, style, snapshot.Alignment)
	for _, selection := range selections {
		if selection.Width <= 0 {
			continue
		}
		canvas.Clip(selection)
		_, selectedColor := selectionColors(theme)
		if snapshot.Disabled {
			selectedColor.Alpha *= 0.50
		}
		selected := textStyle
		selected.Color = selectedColor
		canvas.LabelAligned(value, line, selected, snapshot.Alignment)
		canvas.Unclip()
	}
	canvas.Unclip()
}

// inputTextRanges appends the rectangles of the bytes [start, end) of value, a line drawn
// from x = origin in bounds, clamped to bounds: one for left-to-right text, more where the
// range crosses a change of direction. They fill the band of the line (inputLineBand), top
// and height.
func inputTextRanges(canvas *gfx.Canvas, dst []gfx.Rect, bounds gfx.Rect, top, height, origin float32, value string, style gfx.TextStyle, start, end int) []gfx.Rect {
	var spans [4]gfx.TextSpan
	for _, span := range canvas.TextSelectionStyle(spans[:0], value, style, start, end) {
		x, width := origin+span.X, span.Width
		if x < bounds.X {
			width -= bounds.X - x
			x = bounds.X
		}
		if x > bounds.X+bounds.Width {
			x = bounds.X + bounds.Width
		}
		if x+width > bounds.X+bounds.Width {
			width = bounds.X + bounds.Width - x
		}
		dst = append(dst, gfx.Rect{X: x, Y: top, Width: inputMax(0, width), Height: height})
	}
	return dst
}

// inputLineBox is the box of the glyph line itself, centered in a field: a
// 20px line at the 14px body size. Labels and the declared text bounds use it;
// the caret and selection use the taller inputLineBand.
func inputLineBox(bounds gfx.Rect, fontSize float32) (top, height float32) {
	height = inputMax(0, min(fontSize+6, bounds.Height))
	return bounds.Y + (bounds.Height-height)/2, height
}

// inputLineBand is the vertical band a line of text of fontSize occupies in a
// field of bounds: the room a selection fills, the caret sits in and a
// composition is underlined at the foot of. It is centered in the field and
// follows the size of the text, 24px of a 32px field at the 14px body size, and
// never reaches nearer the border than 4px.
func inputLineBand(bounds gfx.Rect, fontSize float32) (top, height float32) {
	height = inputMax(0, min(fontSize+10, bounds.Height-8))
	return bounds.Y + (bounds.Height-height)/2, height
}

// inputCaretHeight is how tall the caret is for text of fontSize: the 16px of
// the 14px body size, and as much taller or shorter as the text is.
func inputCaretHeight(fontSize float32) float32 { return fontSize + 2 }

func inputRange(value string, start, end int) (int, int) {
	start = inputBoundary(value, start)
	end = inputBoundary(value, end)
	if start > end {
		start, end = end, start
	}
	return start, end
}

func inputBoundary(value string, offset int) int {
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

func inputMax(left, right float32) float32 {
	if left > right {
		return left
	}
	return right
}

// inputBullets is the longest mask a password field paints; longer values show
// this many bullets. It is a constant so masking never allocates.
const inputBullets = "••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••"

// inputMask returns one bullet per character of value, up to the cap.
func inputMask(value string) string {
	count := min(utf8.RuneCountInString(value), len(inputBullets)/3)
	return inputBullets[:count*3]
}

// inputMaskOffset converts a byte offset into value to the matching byte
// offset into its mask.
func inputMaskOffset(value string, offset int) int {
	count := utf8.RuneCountInString(value[:inputBoundary(value, offset)])
	return min(count, len(inputBullets)/3) * 3
}

const (
	// inputRing is the width of the focus ring around a button-like control
	// (Vega's ring-3). Text-entry controls have none: focus is their border.
	inputRing = float32(3)
)

// textEntryDark reports whether theme is a dark appearance, which Vega fills
// text-entry controls for (dark:bg-input/30).
func textEntryDark(theme *gfx.Theme) bool { return theme.Background.Red < 0.5 }

// TextEntryLook is the surface of a text-entry control in one state: what Input, Textarea,
// InputGroup, Select, NativeSelect, Combobox, MultiSelect, SingleSelect and the slots of
// InputOTP are painted with. It is the recipe those controls share, as plain values: an
// application that builds a field from its own elements (ui.InputBase) reads it to take
// the shadcn surface for a state and change what it wants of it.
type TextEntryLook struct {
	// Fill and Border are the colors of the surface. Over an opaque theme they are
	// flattened onto the page background, as RoundedBorder needs, so a translucent token
	// does not tint the control's interior; over a translucent theme they keep their alpha.
	Fill, Border gfx.Color
	// BorderWidth and Radius are the line and the corners.
	BorderWidth, Radius float32
	// Shadow is the elevation under the control.
	Shadow gfx.ShadowLevel
}

// TextEntryRecipe resolves the text-entry recipe for state:
//
//   - border-input and a transparent fill, dark:bg-input/30 (hover input/50);
//   - focus: the border alone turns the accent color; there is no halo around
//     the control, which would wash the field and crowd its neighbors;
//   - invalid: border-destructive, which also wins over the focus color;
//   - disabled: opacity-50 on all of it.
//
// Only Hovered, Focused, Invalid and Disabled of state are read. It is a pure function:
// the same theme and state give the same look, and nothing is painted.
func TextEntryRecipe(theme gfx.Theme, state gfx.State) TextEntryLook {
	return textEntryRecipe(&theme, state)
}

// textEntryRecipe is TextEntryRecipe for a theme that is not to be copied: the controls paint
// every frame, and a Theme is large.
func textEntryRecipe(theme *gfx.Theme, state gfx.State) TextEntryLook {
	border, fill := textEntryTokens(theme, state)
	if theme.Background.Alpha >= 1 {
		border, fill = border.Over(theme.Background), fill.Over(theme.Background)
	}
	return TextEntryLook{Fill: fill, Border: border, BorderWidth: TextEntryBorderWidth, Radius: theme.Metrics.RadiusControl, Shadow: gfx.ShadowXS}
}

// TextEntryBorderWidth is the line around a text-entry control, in every state.
const TextEntryBorderWidth = 1

// textEntryColors is TextEntryRecipe's colors flattened onto the page background.
func textEntryColors(theme *gfx.Theme, state gfx.State) (border, fill gfx.Color) {
	border, fill = textEntryTokens(theme, state)
	return border.Over(theme.Background), fill.Over(theme.Background)
}

// textEntryTokens keeps alpha intact for surfaces composed over a material.
func textEntryTokens(theme *gfx.Theme, state gfx.State) (border, fill gfx.Color) {
	border = theme.Input
	fill = theme.Background
	fill.Alpha = 0
	if textEntryDark(theme) {
		fill = theme.Input
		fill.Alpha *= 0.30
		if state.Hovered && !state.Disabled {
			fill.Alpha = theme.Input.Alpha * 0.50
		}
	}
	if state.Focused && !state.Disabled {
		border = theme.Primary
	}
	if state.Invalid {
		border = theme.Danger
	}
	if state.Disabled {
		border.Alpha *= 0.50
		fill.Alpha *= 0.50
	}
	return border, fill
}

// paintTextEntrySurface paints the border and fill of one text-entry control:
// the single place the Input family takes its look from.
func paintTextEntrySurface(canvas *gfx.Canvas, theme *gfx.Theme, bounds gfx.Rect, state gfx.State) {
	paintTextEntryLook(canvas, theme.Background.Alpha, bounds, textEntryRecipe(theme, state))
}

// paintTextEntryLook paints a resolved look: the shadow, then the fill with its line, as one
// solid shape over an opaque page and as a fill and an outline over a translucent one.
func paintTextEntryLook(canvas *gfx.Canvas, pageAlpha float32, bounds gfx.Rect, look TextEntryLook) {
	canvas.Elevate(bounds, look.Radius, look.Shadow) // shadow-xs
	if pageAlpha < 1 {
		canvas.Rounded(bounds, look.Radius, look.Fill)
		canvas.RoundedOutline(bounds, look.Radius, look.BorderWidth, look.Border)
	} else {
		canvas.RoundedBorder(bounds, look.Radius, look.BorderWidth, look.Border, look.Fill)
	}
}

// inputFitPlaceholder is value when it fits width in style, else its start ending in an ellipsis.
// A proportional face is cut where WrapLines cuts a one-line text; a fixed-pitch one is cut at the
// last whole cell that fits, since every character is as wide as the cell.
func inputFitPlaceholder(canvas *gfx.Canvas, value string, style gfx.TextStyle, width float32) string {
	if !style.Mono {
		var scratch [1]string
		if lines := canvas.WrapLines(scratch[:0], value, style.Size, style.Weight, width, 1); len(lines) == 1 {
			return lines[0]
		}
		return value
	}
	cell, _ := canvas.MonoCellSize(style.Size, style.MonoFace)
	if cell <= 0 {
		return value
	}
	cells := int(width / cell)
	if utf8.RuneCountInString(value) <= cells {
		return value
	}
	if cells < 1 {
		return ""
	}
	end := 0
	for count := 0; count < cells-1 && end < len(value); count++ {
		_, size := utf8.DecodeRuneInString(value[end:])
		end += size
	}
	return value[:end] + "…"
}
