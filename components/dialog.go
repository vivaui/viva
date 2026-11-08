package components

import (
	"time"

	"github.com/vivaui/viva/ui"
)

// DialogOptions carries the stable identities and requested content size for
// one modal dialog. The application owns every ID and the open state.
type DialogOptions struct {
	Layer     uint16
	Scope     ui.ID
	DismissID ui.ID
	CloseID   ui.ID
	// InitialFocusID is the hit that takes keyboard focus when the dialog
	// opens. Leave it zero to keep focus where it was.
	InitialFocusID ui.ID
	ContentSize    ui.Size
	// FitBody sizes the dialog to its parts instead of to ContentSize.Height: the
	// height becomes the header, BodyHeight, the footer and the dialog's own
	// spacing, so the body is exactly as tall as the caller paints it and no
	// guessed height leaves a gap above the footer. ContentSize.Width still
	// applies. The header depends on whether Begin is given a description.
	FitBody    bool
	BodyHeight float32
	// ButtonSize sizes the footer's buttons, and the footer with them. The zero
	// value is ButtonSizeDefault; ButtonSizeSmall makes them as tall as a field.
	ButtonSize ButtonSize
}

// DialogLayout exposes the source-owned slots inside a painted dialog.
type DialogLayout struct {
	Content ui.Rect
	Header  ui.Rect
	Body    ui.Rect
	Footer  ui.Rect
	Close   ui.Rect
}

// Dialog is a modal Vega dialog declaration. Begin keeps its layer and focus
// scope active so callers can paint custom body content before End.
type Dialog struct {
	canvas    *ui.Canvas
	theme     ui.Theme
	viewport  ui.Rect
	options   DialogOptions
	disabled  bool
	hideClose bool
}

// BeginDialog creates a dialog declaration without retaining runtime state.
func BeginDialog(canvas *ui.Canvas, theme ui.Theme, viewport ui.Rect, options DialogOptions) Dialog {
	return Dialog{canvas: canvas, theme: theme, viewport: viewport, options: options}
}

// Disabled keeps dialog actions in the semantic tree while making them inert.
func (dialog Dialog) Disabled() Dialog {
	dialog.disabled = true
	return dialog
}

// WithoutCloseButton hides the standard top-right close action.
func (dialog Dialog) WithoutCloseButton() Dialog {
	dialog.hideClose = true
	return dialog
}

// Actions paints the footer's buttons against its right edge, in the order
// given, so the last one sits at the corner (see ButtonRow for the layout). A
// disabled dialog makes every action inert. Call it between Begin and End. It
// needs the button recipe.
//
//	layout := dialog.Begin("Delete host?", "This cannot be undone.")
//	dialog.Actions(layout,
//		components.ButtonAction{ID: cancelID, Label: "Cancel", Variant: components.ButtonOutline},
//		components.ButtonAction{ID: deleteID, Label: "Delete", Variant: components.ButtonDestructive},
//	)
//	dialog.End()
func (dialog Dialog) Actions(layout DialogLayout, actions ...ButtonAction) {
	overlayActions(dialog.canvas, dialog.theme, layout.Footer, dialog.options.ButtonSize, dialog.disabled, actions)
}

// The header, the body and the footer are three item gaps apart, as the `gap-6`
// of the Vega dialog has it.
const (
	dialogGapsAboveBody = 3
	dialogGapsBelowBody = 3
)

// dialogFooterHeight is the height of the footer's buttons, which are size
// high: by default the Default button, as tall as a field, so a dialog's
// actions are not smaller than the fields they confirm. The overlay's options
// pick another ButtonSize and the footer follows.
func dialogFooterHeight(metrics ui.Metrics, size ButtonSize) float32 {
	return Button{theme: ui.Theme{Metrics: metrics}, size: size}.BoundsHeight()
}

// Paint paints a complete title/description dialog and closes its layer scope.
func (dialog Dialog) Paint(title, description string) DialogLayout {
	layout := dialog.Begin(title, description)
	dialog.End()
	return layout
}

// Layout is where the parts of the dialog go for a viewport, without painting
// anything: Begin paints at exactly these rectangles. The description wraps to
// the dialog's width, so the header grows with its lines; a canvas is needed to
// measure them, and without one the description counts as a single line.
func (dialog Dialog) Layout(description string) DialogLayout {
	var scratch overlayLines
	layout, _ := dialog.layout(&scratch, description)
	return layout
}

// overlayTitleSize is the size of a dialog, sheet or drawer title. Vega sets no
// size on them, so they inherit the base 16px, two steps above the 14px
// description; at Viva's 13px body that is FontSizeM+2.
func overlayTitleSize(metrics ui.Metrics) float32 { return metrics.FontSizeM + 2 }

// dialogTitleHeight is the line box of the title: Vega's `leading-none`.
func dialogTitleHeight(metrics ui.Metrics) float32 { return overlayTitleSize(metrics) + 1 }

// dialogHeaderHeight is the title and, under it, the description's lines.
func dialogHeaderHeight(metrics ui.Metrics, descriptionLines int) float32 {
	height := dialogTitleHeight(metrics)
	if descriptionLines > 0 {
		height += metrics.GapItem + float32(descriptionLines)*overlayLineHeight(metrics)
	}
	return height
}

func (dialog Dialog) layout(scratch *overlayLines, description string) (DialogLayout, []string) {
	if dialog.viewport.Width <= 0 || dialog.viewport.Height <= 0 {
		return DialogLayout{}, nil
	}
	metrics := dialog.theme.Metrics
	padding := metrics.PadSurface + metrics.GapItem
	width := dialog.contentWidth()
	lines := overlayWrap(dialog.canvas, scratch, description, metrics.FontSizeM, ui.WeightRegular, width-2*padding)
	headerHeight := dialogHeaderHeight(metrics, len(lines))
	content := dialog.contentBounds(width, headerHeight)
	inner := content.Inset(ui.All(padding))
	footerHeight := dialogFooterHeight(metrics, dialog.options.ButtonSize)
	bodyY := inner.Y + headerHeight + metrics.GapItem*dialogGapsAboveBody
	bodyHeight := inner.Height - headerHeight - footerHeight - metrics.GapItem*(dialogGapsAboveBody+dialogGapsBelowBody)
	if bodyHeight < 0 {
		bodyHeight = 0
	}
	layout := DialogLayout{
		Content: content,
		Header:  ui.Rect{X: inner.X, Y: inner.Y, Width: inner.Width, Height: headerHeight},
		Body:    ui.Rect{X: inner.X, Y: bodyY, Width: inner.Width, Height: bodyHeight},
		Footer:  ui.Rect{X: inner.X, Y: inner.Y + inner.Height - footerHeight, Width: inner.Width, Height: footerHeight},
	}
	if !dialog.hideClose {
		closeSize := metrics.ControlHeight
		layout.Close = ui.Rect{
			X:     content.X + content.Width - metrics.PadSurface - closeSize,
			Y:     content.Y + metrics.PadSurface,
			Width: closeSize, Height: closeSize,
		}
	}
	return layout, lines
}

// Begin paints the backdrop and surface, then leaves the modal layer and focus
// scope active for caller-owned body content.
func (dialog Dialog) Begin(title, description string) DialogLayout {
	if dialog.canvas == nil || dialog.viewport.Width <= 0 || dialog.viewport.Height <= 0 {
		return DialogLayout{}
	}
	dialog.canvas.PushLayer(dialog.options.Layer)
	if !dialog.options.Scope.IsZero() {
		dialog.canvas.PushFocusScope(dialog.options.Scope)
	}

	key := overlayKey(dialog.options.DismissID, dialog.options.CloseID, dialog.options.Scope)
	beginScrim(dialog.canvas, dialog.theme, key, scrimPresence(dialog.theme, overlayDuration, ui.EaseCSS), dialog.viewport)
	if !dialog.options.DismissID.IsZero() {
		dialog.canvas.HitWithOptions(dialog.options.DismissID, dialog.viewport, "Dismiss dialog", ui.HitOptions{
			Role: ui.RoleButton, Disabled: dialog.disabled, SkipFocus: true, Backdrop: true,
		})
	}

	var scratch overlayLines
	layout, lines := dialog.layout(&scratch, description)
	content, inner := layout.Content, layout.Header
	metrics := dialog.theme.Metrics
	dialog.canvas.BeginPresence(key, overlaySurfacePart, fadePresence(dialog.theme), centerOf(content))
	paintPanel(dialog.canvas, content, dialogRecipe(&dialog.theme))
	// The surface is opaque to pointers: pressing its padding must not reach the
	// dismiss target behind it.
	dialog.canvas.Block(content)
	titleHeight := dialogTitleHeight(metrics)

	dialog.canvas.Label(title, ui.Rect{X: inner.X, Y: inner.Y, Width: inner.Width, Height: titleHeight}, ui.TextStyle{
		Size: overlayTitleSize(metrics), Weight: ui.WeightMedium, Color: dialog.theme.PopoverForeground,
	})
	overlayParagraph(dialog.canvas, lines, ui.Rect{X: inner.X, Y: inner.Y + titleHeight + metrics.GapItem, Width: inner.Width}, overlayLineHeight(metrics), ui.TextStyle{
		Size: metrics.FontSizeM, Weight: ui.WeightRegular, Color: dialog.theme.MutedForeground,
	}, false)

	if !dialog.hideClose {
		dialog.paintClose(layout.Close)
	}
	return layout
}

// End closes the focus scope and layer, then installs modal focus and Escape
// routing. It must match a successful Begin call. It also ends what the dialog
// animates in and out: the dialog is not animated without it.
func (dialog Dialog) End() {
	if dialog.canvas == nil || dialog.viewport.Width <= 0 || dialog.viewport.Height <= 0 {
		return
	}
	dialog.canvas.EndPresence()
	// Claim the trap, Escape and initial focus while the dialog's own scope is
	// still pushed, so a popup painted deeper inside it keeps its own claim.
	if !dialog.options.Scope.IsZero() {
		dialog.canvas.TrapFocus(dialog.options.Scope)
		if !dialog.disabled {
			dialog.canvas.SetInitialFocus(dialog.options.InitialFocusID)
		}
	}
	escapeTarget := dialog.options.DismissID
	if escapeTarget.IsZero() {
		escapeTarget = dialog.options.CloseID
	}
	if dialog.disabled {
		escapeTarget = ui.ID{}
	}
	dialog.canvas.SetEscapeTarget(escapeTarget)
	if !dialog.options.Scope.IsZero() {
		dialog.canvas.PopFocusScope()
	}
	dialog.canvas.PopLayer()
}

// contentWidth is the dialog's width: ContentSize.Width, or Vega's `sm:max-w-md`
// (448) without one, and never wider than the viewport less a 16px margin.
func (dialog Dialog) contentWidth() float32 {
	width := dialog.options.ContentSize.Width
	if width <= 0 {
		width = 448
	}
	return max(min(width, dialog.viewport.Width-32), 0)
}

// contentBounds centers the dialog. With FitBody its height is the padding
// around it, the header, the footer, BodyHeight, and the gaps Layout leaves
// between header, body and footer.
func (dialog Dialog) contentBounds(width, headerHeight float32) ui.Rect {
	metrics := dialog.theme.Metrics
	height := dialog.options.ContentSize.Height
	if dialog.options.FitBody {
		padding := metrics.PadSurface + metrics.GapItem
		height = 2*padding + headerHeight + dialogFooterHeight(metrics, dialog.options.ButtonSize) + max(dialog.options.BodyHeight, 0) +
			(dialogGapsAboveBody+dialogGapsBelowBody)*metrics.GapItem
	}
	if height <= 0 {
		height = 192
	}
	height = max(min(height, dialog.viewport.Height-32), 0)
	return ui.Rect{
		X:     dialog.viewport.X + (dialog.viewport.Width-width)/2,
		Y:     dialog.viewport.Y + (dialog.viewport.Height-height)/2,
		Width: width, Height: height,
	}
}

func (dialog Dialog) paintClose(bounds ui.Rect) {
	fill := dialog.theme.Background
	fill.Alpha = 0
	foreground := dialog.theme.MutedForeground
	if dialog.canvas.Hovered() == dialog.options.CloseID && !dialog.disabled {
		fill = dialog.theme.Accent
		foreground = dialog.theme.AccentForeground
	}
	if dialog.disabled {
		foreground.Alpha *= 0.50
	}
	dialog.canvas.Rounded(bounds, dialog.theme.Metrics.RadiusControl, fill)
	paintCross(dialog.canvas, bounds, 16, foreground)
	if !dialog.options.CloseID.IsZero() {
		dialog.canvas.HitWithOptions(dialog.options.CloseID, bounds, "Close dialog", ui.HitOptions{
			Role: ui.RoleButton, Disabled: dialog.disabled,
		})
	}
}

// The cross is a vector for the same reason as the check (see paintCheck).
var crossIcon = ui.Icon{ViewBox: 24, Stroke: 2, Cap: ui.CapRound, Join: ui.JoinRound, Commands: []ui.PathCommand{
	ui.MoveTo(18, 6), ui.LineTo(6, 18), ui.MoveTo(6, 6), ui.LineTo(18, 18),
}}

// paintCross draws a cross size pixels square, centered in bounds.
func paintCross(canvas *ui.Canvas, bounds ui.Rect, size float32, color ui.Color) {
	canvas.DrawIcon(crossIcon, ui.Rect{X: bounds.X + (bounds.Width-size)/2, Y: bounds.Y + (bounds.Height-size)/2, Width: size, Height: size}, color)
}

// overlayMaxLines bounds how many lines an overlay's description or tooltip text
// wraps to; a longer text ends in an ellipsis on the last line. The lines are
// slices of the text, held in an array on the caller's stack, so wrapping
// allocates nothing.
const overlayMaxLines = 8

// overlayLines is the scratch space for wrapped overlay text.
type overlayLines [overlayMaxLines]string

// overlayLineHeight is the line height of the 14px body text (text-sm, 20px
// leading) of an overlay, at the theme's body size.
func overlayLineHeight(metrics ui.Metrics) float32 { return metrics.FontSizeM + 7 }

// overlayWrap breaks text into lines of at most width at the given size and
// weight, the way a browser wraps a paragraph. Without a canvas to measure on
// the text stays one line.
func overlayWrap(canvas *ui.Canvas, scratch *overlayLines, text string, size, weight, width float32) []string {
	if text == "" {
		return nil
	}
	if canvas == nil || width <= 0 {
		scratch[0] = text
		return scratch[:1]
	}
	return canvas.WrapLines(scratch[:0], text, size, weight, width, overlayMaxLines)
}

// overlayParagraph paints wrapped lines one under the other from the top of
// bounds, each lineHeight high, and returns the height they took.
func overlayParagraph(canvas *ui.Canvas, lines []string, bounds ui.Rect, lineHeight float32, style ui.TextStyle, center bool) float32 {
	for index, line := range lines {
		row := ui.Rect{X: bounds.X, Y: bounds.Y + float32(index)*lineHeight, Width: bounds.Width, Height: lineHeight}
		if center {
			canvas.LabelCenter(line, row, style)
		} else {
			canvas.Label(line, row, style)
		}
	}
	return float32(len(lines)) * lineHeight
}

// overlayActions paints the buttons of a dialog, sheet or drawer footer against
// its right edge at the default button size (see ButtonRow).
func overlayActions(canvas *ui.Canvas, theme ui.Theme, footer ui.Rect, size ButtonSize, disabled bool, actions []ButtonAction) {
	row := BeginButtonRow(canvas, theme, footer).Size(size)
	if disabled {
		row = row.Disabled()
	}
	row.Paint(actions...)
}

// overlayBackdrop is the scrim behind a modal overlay. Vega's is `bg-black/10`
// together with `backdrop-blur-xs`; the blur has no canvas primitive,
// and a 10% scrim alone barely separates a modal from the page, least of all on
// a dark theme. Until a blur exists the scrim is heavier: 50% on a dark theme,
// 40% on a light one. Return to Vega's 10% when the blur lands.
func overlayBackdrop(theme ui.Theme) ui.Color {
	if theme.Background.Red < 0.5 {
		return ui.Color{Alpha: 0.50}
	}
	return ui.Color{Alpha: 0.40}
}

// Overlay motion. Vega gives every overlay an entrance and an exit:
// a scrim that fades, a dialog or a popup that fades while it grows from 95% of
// its size, a popup that also slides 8px away from the edge it opens on, a sheet
// that slides 40px. The recipes declare them with Canvas.BeginPresence around
// what they paint; the canvas animates the group and, once the application stops
// painting the overlay, draws its last frame out. None of it is state of the
// application.
//
// An overlay is kept under one identity, the first of its options' IDs that is
// set (overlayKey). Without one, or with a theme whose Transition has no
// duration, the overlay appears and disappears at once.

// Parts of one overlay that move differently. A canvas keeps them apart by slot.
const (
	overlayBackdropPart uint8 = iota
	overlaySurfacePart
	// overlaySubmenuPart is a submenu open beside the menu it belongs to, which
	// shares that menu's identity.
	overlaySubmenuPart
)

// overlayDuration is Vega's `duration-100` on dialogs, popups and menus. Vega
// also grows them from 95% (`zoom-in-95`); Viva does not. The canvas scales
// shapes but not text, and the 1px borders and radii of the controls inside
// would snap to different pixels at every step, so the contents looked deformed
// until the end of the transition.
const overlayDuration = 100 * time.Millisecond

// overlayKey is the identity an overlay's motion is kept under: the first of ids
// that is set, or zero.
func overlayKey(ids ...ui.ID) ui.ID {
	for _, id := range ids {
		if !id.IsZero() {
			return id
		}
	}
	return ui.ID{}
}

// moving reports whether the theme asks for overlay motion at all.
func moving(theme ui.Theme) bool { return theme.Transition.Duration > 0 }

func scrimPresence(theme ui.Theme, duration time.Duration, easing ui.Easing) ui.Presence {
	if !moving(theme) {
		return ui.Presence{}
	}
	return ui.Presence{Motion: ui.Motion{Duration: duration, Easing: easing}, Fade: true}
}

// fadePresence is the dialog's `fade-in-0`, and its exit.
func fadePresence(theme ui.Theme) ui.Presence {
	if !moving(theme) {
		return ui.Presence{}
	}
	return ui.Presence{Motion: ui.Motion{Duration: overlayDuration, Easing: ui.EaseCSS}, Fade: true}
}

// centerOf is the middle of a rectangle.
func centerOf(bounds ui.Rect) ui.Point {
	return ui.Point{X: bounds.X + bounds.Width/2, Y: bounds.Y + bounds.Height/2}
}

// beginScrim paints the scrim behind a modal overlay as its own group, so it
// fades while the surface in front of it grows.
func beginScrim(canvas *ui.Canvas, theme ui.Theme, key ui.ID, presence ui.Presence, viewport ui.Rect) {
	canvas.BeginPresence(key, overlayBackdropPart, presence, centerOf(viewport))
	canvas.Fill(viewport, overlayBackdrop(theme))
	canvas.EndPresence()
}

// PopupLook is the surface of a panel that floats over the page: what a dialog, a popover, a
// select's list, a command palette, a menu or a hover card is painted on. The panels differ in
// their corners and their shadow (a dialog stands on a scrim and casts none; a list floats and
// does), and each has its own recipe: DialogRecipe, PopoverRecipe and PopupRecipe.
type PopupLook struct {
	// Fill and Border are the colors of the panel and its 1px line.
	Fill, Border ui.Color
	// Radius is the corner radius of the panel.
	Radius float32
	// Shadow is the elevation of a panel that floats over a page; a menu bar's submenu takes a
	// lower one, and a panel that sits inside another has none.
	Shadow ui.ShadowLevel
}

// PopupRecipe resolves the look of a floating panel. It is a pure function of the theme, the
// one definition the popup components paint their surface from (paintPopupSurface), so an
// application that draws a panel of its own takes the same fill, line, corners and shadow.
func PopupRecipe(theme ui.Theme) PopupLook {
	return popupRecipe(&theme)
}

// popupRecipe is the recipe for a theme that is not to be copied: the controls paint every frame,
// and a Theme is large.
func popupRecipe(theme *ui.Theme) PopupLook {
	return PopupLook{Fill: theme.Popover, Border: theme.Border, Radius: theme.Metrics.RadiusPopup(), Shadow: ui.ShadowMD}
}

// paintPopupSurface paints the panel of a popup with the given elevation: the shadow first,
// then the fill with its line. ui.ShadowNone paints a panel that casts none.
func paintPopupSurface(canvas *ui.Canvas, theme *ui.Theme, bounds ui.Rect, shadow ui.ShadowLevel) {
	look := popupRecipe(theme)
	look.Shadow = shadow
	paintPanel(canvas, bounds, look)
}

// paintPanel paints a panel with a resolved look: the shadow it casts, if any, then its fill
// with the 1px line.
func paintPanel(canvas *ui.Canvas, bounds ui.Rect, look PopupLook) {
	if look.Shadow != ui.ShadowNone {
		canvas.Elevate(bounds, look.Radius, look.Shadow)
	}
	canvas.RoundedBorder(bounds, look.Radius, 1, look.Border, look.Fill)
}

// DialogRecipe resolves the look of the panel of a dialog, an alert dialog or a drawer: the
// surface radius, and no shadow, because the panel stands on a scrim. A pure function.
func DialogRecipe(theme ui.Theme) PopupLook {
	return dialogRecipe(&theme)
}

func dialogRecipe(theme *ui.Theme) PopupLook {
	return PopupLook{Fill: theme.Popover, Border: theme.Border, Radius: theme.Metrics.RadiusSurface, Shadow: ui.ShadowNone}
}

// PopoverRecipe resolves the look of the panel of a popover: the control radius and the
// medium shadow. A pure function.
func PopoverRecipe(theme ui.Theme) PopupLook {
	return popoverRecipe(&theme)
}

func popoverRecipe(theme *ui.Theme) PopupLook {
	return PopupLook{Fill: theme.Popover, Border: theme.Border, Radius: theme.Metrics.RadiusControl, Shadow: ui.ShadowMD}
}
