package components

import "github.com/vivaui/viva/ui"

// ButtonVariant selects one of the shadcn Vega button treatments. Destructive is
// Vega's quiet one: a danger-tinted surface with a danger label, not a solid red
// block.
type ButtonVariant uint8

const (
	ButtonDefault ButtonVariant = iota
	ButtonDestructive
	ButtonOutline
	ButtonSecondary
	ButtonGhost
	ButtonLink
	// ButtonDestructiveOutline is the bordered form of a dangerous action: a danger
	// colored label on an outline button, tinted with the danger color only under
	// the pointer. Viva's own variant (shadcn has none), for an action that sits
	// among ordinary controls such as a settings row.
	ButtonDestructiveOutline
)

// ButtonSize selects one of the official shadcn button sizes.
type ButtonSize uint8

const (
	ButtonSizeDefault ButtonSize = iota
	ButtonSizeXSmall
	ButtonSizeSmall
	ButtonSizeLarge
	ButtonSizeIcon
	ButtonSizeIconXSmall
	ButtonSizeIconSmall
	ButtonSizeIconLarge
)

// ButtonState carries transient pointer, focus, and validation state for
// painting fixed previews or tests.
type ButtonState struct {
	Hovered bool
	Pressed bool
	Focused bool
	Invalid bool
}

// Button is one allocation-free in-flight button declaration.
type Button struct {
	canvas   *ui.Canvas
	theme    ui.Theme
	id       ui.ID
	bounds   ui.Rect
	state    ButtonState
	size     ButtonSize
	icon     ui.Icon
	disabled bool
}

// BeginButton starts a button declaration. Hit ID zero paints a static preview
// without adding an interaction region.
func BeginButton(canvas *ui.Canvas, theme ui.Theme, id ui.ID, bounds ui.Rect) Button {
	state := ButtonState{}
	if canvas != nil && !id.IsZero() {
		state.Hovered = canvas.Hovered() == id
		state.Pressed = canvas.Pressed() == id
		state.Focused = canvas.FocusVisible() == id
	}
	return Button{canvas: canvas, theme: theme, id: id, bounds: bounds, state: state}
}

// WithState pins a visual state, overriding canvas-derived interaction state.
func (button Button) WithState(state ButtonState) Button {
	button.state = state
	return button
}

// WithIcon adds a leading vector icon. At an icon size the label is hidden
// visually and retained as the accessible name and hover hint.
func (button Button) WithIcon(icon ui.Icon) Button {
	button.icon = icon
	return button
}

func (button Button) hasIcon() bool {
	return len(button.icon.Commands) > 0 || len(button.icon.Layers) > 0
}

// Size selects an official size variant.
func (button Button) Size(size ButtonSize) Button {
	button.size = size
	return button
}

// XSmall selects the 24px compact size at default density.
func (button Button) XSmall() Button { return button.Size(ButtonSizeXSmall) }

// Small selects the 32px size at default density.
func (button Button) Small() Button { return button.Size(ButtonSizeSmall) }

// Large selects the 40px size at default density.
func (button Button) Large() Button { return button.Size(ButtonSizeLarge) }

// Icon selects the square default icon size.
func (button Button) Icon() Button { return button.Size(ButtonSizeIcon) }

// IconXSmall selects the square 24px icon size at default density.
func (button Button) IconXSmall() Button { return button.Size(ButtonSizeIconXSmall) }

// IconSmall selects the square 32px icon size at default density.
func (button Button) IconSmall() Button { return button.Size(ButtonSizeIconSmall) }

// IconLarge selects the square 40px icon size at default density.
func (button Button) IconLarge() Button { return button.Size(ButtonSizeIconLarge) }

// Invalid applies the official destructive validation ring.
func (button Button) Invalid() Button {
	button.state.Invalid = true
	return button
}

// Disabled paints the official 50% opacity state and exposes disabled typed
// semantics when the declaration has a nonzero ID.
func (button Button) Disabled() Button {
	button.disabled = true
	return button
}

// The leading icon keeps one square, one gap and one vertical inset in every
// size, so SizeToFit and paint agree and a narrow control shrinks the icon
// instead of drawing it past its own edge.
const (
	buttonIconSize  = 16
	buttonIconGap   = 8
	buttonIconInset = 8
)

func buttonIconExtent(height float32) float32 {
	return max(0, min(float32(buttonIconSize), height-buttonIconInset))
}

// SizeToFit widens the button to its label plus the selected size padding.
func (button Button) SizeToFit(label string) Button {
	height, padding, fontSize, iconOnly := button.geometry()
	button.bounds.Width = button.canvas.TextWidthWeight(label, fontSize, ui.WeightMedium) + padding*2
	if button.hasIcon() {
		if iconOnly {
			button.bounds.Width = height
		} else {
			button.bounds.Width += buttonIconExtent(height)
			if label != "" {
				button.bounds.Width += buttonIconGap
			}
		}
	}
	button.bounds.Height = height
	return button
}

// CenterIn centers the button's current bounds within area. Call it after
// SizeToFit when an empty state or dialog action needs a centered control.
func (button Button) CenterIn(area ui.Rect) Button {
	button.bounds.X = area.X + (area.Width-button.bounds.Width)/2
	button.bounds.Y = area.Y + (area.Height-button.bounds.Height)/2
	return button
}

// ButtonResult is what painting a button reports back. Clicked is true in the
// event pass of a click on that button (see ui.Canvas.Clicked), so the code that
// declares a button is the code that handles it:
//
//	if components.BeginButton(canvas, theme, save, bounds).Default("Save").Clicked() {
//		model.save()
//	}
type ButtonResult struct {
	canvas *ui.Canvas
	id     ui.ID
}

// Clicked reports whether this paint is delivering a click on the button.
func (result ButtonResult) Clicked() bool {
	return result.canvas != nil && result.canvas.Clicked(result.id)
}

// BoundsWidth exposes the fitted width for callers laying out neighbors.
func (button Button) BoundsWidth() float32 { return button.bounds.Width }

// BoundsHeight exposes the height the selected size paints at.
func (button Button) BoundsHeight() float32 {
	height, _, _, _ := button.geometry()
	return height
}

// Default paints the primary official default variant.
func (button Button) Default(label string) ButtonResult { return button.paint(label, ButtonDefault) }

// Destructive paints the official destructive variant.
func (button Button) Destructive(label string) ButtonResult {
	return button.paint(label, ButtonDestructive)
}

// Secondary paints the official secondary variant.
func (button Button) Secondary(label string) ButtonResult {
	return button.paint(label, ButtonSecondary)
}

// Outline paints the official bordered variant.
func (button Button) Outline(label string) ButtonResult { return button.paint(label, ButtonOutline) }

// DestructiveOutline paints the bordered, danger-labelled variant.
func (button Button) DestructiveOutline(label string) ButtonResult {
	return button.paint(label, ButtonDestructiveOutline)
}

// Ghost paints the official borderless variant.
func (button Button) Ghost(label string) ButtonResult { return button.paint(label, ButtonGhost) }

// Link paints the official link-colored button variant.
func (button Button) Link(label string) ButtonResult { return button.paint(label, ButtonLink) }

// ButtonAction is one button of a ButtonRow. ID is the application's stable hit
// identity. The zero Variant is the primary Default button, so the confirming
// action needs no variant at all.
type ButtonAction struct {
	ID       ui.ID
	Label    string
	Variant  ButtonVariant
	Disabled bool
}

// ButtonRow is a row of buttons against the right edge of an area: the footer
// of a card, a panel or a dialog.
type ButtonRow struct {
	canvas   *ui.Canvas
	theme    ui.Theme
	area     ui.Rect
	size     ButtonSize
	sized    bool
	disabled bool
}

// BeginButtonRow declares a row of small buttons inside area. The buttons are
// vertically centered in it.
func BeginButtonRow(canvas *ui.Canvas, theme ui.Theme, area ui.Rect) ButtonRow {
	return ButtonRow{canvas: canvas, theme: theme, area: area}
}

// Disabled keeps every button in the semantic tree while making them inert.
func (row ButtonRow) Disabled() ButtonRow {
	row.disabled = true
	return row
}

// Size picks the buttons' size instead of the small default, for a row that has
// to fit a bar shorter than a footer: XSmall is 24px at default density.
func (row ButtonRow) Size(size ButtonSize) ButtonRow {
	row.size, row.sized = size, true
	return row
}

func (row ButtonRow) button(id ui.ID, bounds ui.Rect) Button {
	button := BeginButton(row.canvas, row.theme, id, bounds)
	if row.sized {
		return button.Size(row.size)
	}
	return button.Small()
}

// Paint paints the buttons in the order given, so the last one sits at the
// corner. Each is as wide as its own label needs (the button's padding
// included) and neighbours are one item gap apart; when they cannot all fit
// they shrink together.
//
//	components.BeginButtonRow(canvas, theme, footer).Paint(
//		components.ButtonAction{ID: cancelID, Label: "Cancel", Variant: components.ButtonOutline},
//		components.ButtonAction{ID: saveID, Label: "Save"},
//	)
func (row ButtonRow) Paint(actions ...ButtonAction) {
	area := row.area
	if row.canvas == nil || len(actions) == 0 || area.Width <= 0 || area.Height <= 0 {
		return
	}
	gaps := row.theme.Metrics.GapItem * float32(len(actions)-1)
	fitted := func(action ButtonAction) float32 {
		return row.button(ui.ID{}, area).SizeToFit(action.Label).BoundsWidth()
	}
	var labels float32
	for _, action := range actions {
		labels += fitted(action)
	}
	scale := float32(1)
	if labels+gaps > area.Width {
		scale = max((area.Width-gaps)/labels, 0)
	}
	x := area.X + area.Width - (labels*scale + gaps)
	for _, action := range actions {
		width := fitted(action) * scale
		button := row.button(action.ID, ui.Rect{X: x, Y: area.Y, Width: width})
		height, _, _, _ := button.geometry()
		button.bounds.Y = area.Y + (area.Height-height)/2
		if action.Disabled || row.disabled {
			button = button.Disabled()
		}
		button.paint(action.Label, action.Variant)
		x += width + row.theme.Metrics.GapItem
	}
}

func (button Button) paint(label string, variant ButtonVariant) ButtonResult {
	result := ButtonResult{canvas: button.canvas, id: button.id}
	if button.canvas == nil || button.bounds.Width <= 0 {
		return result
	}
	height, _, fontSize, iconOnly := button.geometry()
	button.bounds.Height = height
	if iconOnly {
		button.bounds.Width = height
	}
	radius := button.theme.Metrics.RadiusControl
	enabled := !button.disabled

	style := buttonStyleFor(button.theme, variant, enabled && button.state.Hovered, enabled && button.state.Pressed)
	if variant == ButtonOutline {
		button.canvas.Elevate(button.bounds, radius, ui.ShadowXS) // shadow-xs
	}
	if button.state.Focused || button.state.Invalid {
		ring, border := buttonRing(button.theme, variant, button.state.Invalid)
		if button.theme.Background.Alpha < 1 {
			button.canvas.RoundedOutline(expand(button.bounds, 3), radius+3, 2, ring)
		} else {
			button.canvas.Rounded(expand(button.bounds, 3), radius+3, ring)
		}
		style.border, style.bordered = border, true
	}
	if button.disabled {
		style.background.Alpha *= 0.50
		style.foreground.Alpha *= 0.50
		style.border.Alpha *= 0.50
	}
	if style.bordered && style.filled {
		paintBordered(button.canvas, button.theme, button.bounds, radius, style.border, style.background)
	} else if style.filled {
		button.canvas.Rounded(button.bounds, radius, style.background)
	}
	hint := ""
	if button.hasIcon() {
		size := min(buttonIconExtent(height), button.bounds.Width)
		width := size
		if !iconOnly && label != "" {
			width = min(button.bounds.Width, width+buttonIconGap+button.canvas.TextWidthWeight(label, fontSize, ui.WeightMedium))
		}
		x := button.bounds.X + (button.bounds.Width-width)/2
		if size > 0 {
			button.canvas.DrawIcon(button.icon, ui.Rect{X: x, Y: button.bounds.Y + (height-size)/2, Width: size, Height: size}, style.foreground)
		}
		if iconOnly {
			hint = label
		} else if label != "" {
			text := ui.Rect{X: x + size + buttonIconGap, Y: button.bounds.Y, Width: max(0, width-size-buttonIconGap), Height: height}
			paintButtonLabel(button.canvas, label, text, fontSize, style)
		}
	} else {
		paintButtonLabel(button.canvas, label, button.bounds, fontSize, style)
	}
	if !button.id.IsZero() {
		button.canvas.HitWithOptions(button.id, button.bounds, label, ui.HitOptions{
			Role: ui.RoleButton, Disabled: button.disabled, Hint: hint,
		})
	}
	return result
}

func (button Button) geometry() (height, padding, fontSize float32, iconOnly bool) {
	metrics := button.theme.Metrics
	height = metrics.ControlHeight
	padding = 10
	fontSize = metrics.FontSizeM
	switch button.size {
	case ButtonSizeXSmall:
		height, padding, fontSize = metrics.ControlHeight-8, 8, metrics.FontSizeS
	case ButtonSizeSmall:
		height = metrics.ControlHeight - 4
	case ButtonSizeLarge:
		height = metrics.ControlHeight + 4
	case ButtonSizeIcon:
		iconOnly = true
	case ButtonSizeIconXSmall:
		height, iconOnly = metrics.ControlHeight-8, true
		fontSize = metrics.FontSizeS
	case ButtonSizeIconSmall:
		height, iconOnly = metrics.ControlHeight-4, true
	case ButtonSizeIconLarge:
		height, iconOnly = metrics.ControlHeight+4, true
	}
	if height < 20 {
		height = 20
	}
	return height, padding, fontSize, iconOnly
}

// buttonStyle is what one variant looks like in one pointer state. Button and
// ButtonGroup both take their colors from buttonStyleFor, so the two cannot
// drift apart.
type buttonStyle struct {
	background, foreground, border ui.Color
	// filled paints background; bordered paints a 1px border; underlined
	// underlines the label (a hovered link).
	filled, bordered, underlined bool
}

// paintBordered paints a shape with a 1px border. Canvas.RoundedBorder lays the
// border color down as a solid backing shape under the fill, so a translucent or
// empty fill would show it through the whole interior; both colors are flattened
// onto the page (the theme's Background) first, as the Input recipe does. A fill
// with no alpha therefore leaves the page showing.
func paintBordered(canvas *ui.Canvas, theme ui.Theme, bounds ui.Rect, radius float32, border, fill ui.Color) {
	if theme.Background.Alpha < 1 {
		canvas.Rounded(bounds, radius, fill)
		canvas.RoundedOutline(bounds, radius, 1, border)
		return
	}
	canvas.RoundedBorder(bounds, radius, 1, border.Over(theme.Background), fill.Over(theme.Background))
}

// isDark reports whether the theme is a dark appearance. Viva's Theme has no
// explicit mode, so every component that needs one asks the background.
func isDark(theme ui.Theme) bool { return theme.Background.Red < 0.5 }

// withAlpha is color at alpha, as a translucent color to be laid over whatever
// is behind it (shadcn's bg-destructive/10).
func withAlpha(color ui.Color, alpha float32) ui.Color {
	color.Alpha = alpha
	return color
}

// scaledAlpha is color with its own alpha multiplied by factor (bg-input/30).
func scaledAlpha(color ui.Color, factor float32) ui.Color {
	color.Alpha *= factor
	return color
}

// outlineBorder is the border of an outline button: border-border in a light
// theme, border-input in a dark one.
func outlineBorder(theme ui.Theme) ui.Color {
	if isDark(theme) {
		return theme.Input
	}
	return theme.Border
}

// invalidRing is the ring around a control that fails validation:
// ring-destructive/20, /40 in a dark theme.
func invalidRing(theme ui.Theme) ui.Color {
	if isDark(theme) {
		return withAlpha(theme.Danger, 0.40)
	}
	return withAlpha(theme.Danger, 0.20)
}

// invalidBorder is the border of an invalid control: border-destructive, /50 in a
// dark theme.
func invalidBorder(theme ui.Theme) ui.Color {
	if isDark(theme) {
		return withAlpha(theme.Danger, 0.50)
	}
	return theme.Danger
}

// buttonRing is the ring and the border a focused or invalid button wears.
// Focus is ring-ring/50 around a ring-colored border; the destructive variant
// uses the danger color for both, as shadcn's does.
func buttonRing(theme ui.Theme, variant ButtonVariant, invalid bool) (ring, border ui.Color) {
	switch {
	case invalid:
		return invalidRing(theme), invalidBorder(theme)
	case variant == ButtonDestructive:
		return invalidRing(theme), withAlpha(theme.Danger, 0.40)
	}
	return scaledAlpha(theme.Ring, 0.50), theme.Ring
}

// buttonStyleFor is the Vega recipe of a variant: its colors at rest, under the
// pointer, and held down. Vega has no pressed state; a press goes one step past
// hover, because a desktop control needs the answer.
func buttonStyleFor(theme ui.Theme, variant ButtonVariant, hovered, pressed bool) buttonStyle {
	dark := isDark(theme)
	style := buttonStyle{
		background: theme.Primary, foreground: theme.PrimaryForeground,
		border: outlineBorder(theme), filled: true,
	}
	switch variant {
	case ButtonSecondary:
		style.background = theme.Secondary
		style.foreground = theme.SecondaryForeground
		switch {
		case pressed:
			style.background = blend(theme.Secondary, theme.Foreground, 0.10)
		case hovered:
			style.background = blend(theme.Secondary, theme.Foreground, 0.05)
		}
	case ButtonOutline:
		style.foreground = theme.Foreground
		style.bordered = true
		if dark {
			style.background = scaledAlpha(theme.Input, 0.30)
			switch {
			case pressed:
				style.background = scaledAlpha(theme.Input, 0.70)
			case hovered:
				style.background = scaledAlpha(theme.Input, 0.50)
			}
		} else {
			style.background = theme.Background
			switch {
			case pressed:
				style.background = blend(theme.Muted, theme.Foreground, 0.05)
			case hovered:
				style.background = theme.Muted
			}
		}
	case ButtonDestructiveOutline:
		style.background = theme.Background
		style.foreground = theme.Danger
		style.border = style.border.Over(theme.Background)
		style.bordered = true
		switch {
		case pressed:
			style.background = tinted(theme.Danger, 0.18, theme.Background)
		case hovered:
			style.background = tinted(theme.Danger, 0.10, theme.Background)
		}
	case ButtonGhost:
		style.foreground = theme.Foreground
		style.filled = pressed || hovered
		switch {
		case dark && pressed:
			style.background = scaledAlpha(theme.Muted, 0.80)
		case dark:
			style.background = scaledAlpha(theme.Muted, 0.50)
		case pressed:
			style.background = blend(theme.Muted, theme.Foreground, 0.05)
		default:
			style.background = theme.Muted
		}
	case ButtonDestructive:
		style.foreground = theme.Danger
		rest, over, held := float32(0.10), float32(0.20), float32(0.30)
		if dark {
			rest, over, held = 0.20, 0.30, 0.40
		}
		style.background = withAlpha(theme.Danger, rest)
		switch {
		case pressed:
			style.background = withAlpha(theme.Danger, held)
		case hovered:
			style.background = withAlpha(theme.Danger, over)
		}
	case ButtonLink:
		style.foreground = theme.Primary
		style.filled = false
		style.underlined = hovered || pressed
	default:
		switch {
		case pressed:
			style.background.Alpha *= 0.70
		case hovered:
			style.background.Alpha *= 0.80
		}
	}
	return style
}

// paintButtonLabel centers a button's label and, for a hovered link, underlines it.
func paintButtonLabel(canvas *ui.Canvas, label string, bounds ui.Rect, fontSize float32, style buttonStyle) {
	canvas.LabelCenter(label, bounds, ui.TextStyle{
		Size: fontSize, Weight: ui.WeightMedium, Color: style.foreground,
	})
	if !style.underlined {
		return
	}
	width := min(canvas.TextWidthWeight(label, fontSize, ui.WeightMedium), bounds.Width)
	canvas.Fill(ui.Rect{
		X: bounds.X + (bounds.Width-width)/2, Y: bounds.Y + bounds.Height/2 + fontSize*0.6,
		Width: width, Height: 1,
	}, style.foreground)
}

func blend(color, overlay ui.Color, amount float32) ui.Color {
	if amount < 0 {
		amount = 0
	}
	if amount > 1 {
		amount = 1
	}
	return ui.Color{
		Red:   color.Red + (overlay.Red-color.Red)*amount,
		Green: color.Green + (overlay.Green-color.Green)*amount,
		Blue:  color.Blue + (overlay.Blue-color.Blue)*amount,
		Alpha: color.Alpha,
	}
}

func expand(bounds ui.Rect, amount float32) ui.Rect {
	return ui.Rect{
		X: bounds.X - amount, Y: bounds.Y - amount,
		Width: bounds.Width + amount*2, Height: bounds.Height + amount*2,
	}
}

// tinted is color laid over base at alpha, as an opaque color.
func tinted(color ui.Color, alpha float32, base ui.Color) ui.Color {
	color.Alpha = alpha
	return color.Over(base)
}
