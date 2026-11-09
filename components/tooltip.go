package components

import (
	"time"

	"github.com/vivaui/viva/gfx"
)

// TooltipSide selects the anchor edge used by a tooltip.
type TooltipSide uint8

const (
	TooltipTop TooltipSide = iota
	TooltipRight
	TooltipBottom
	TooltipLeft
)

// TooltipAlign selects tooltip placement along the cross axis.
type TooltipAlign uint8

const (
	TooltipAlignCenter TooltipAlign = iota
	TooltipAlignStart
	TooltipAlignEnd
)

// TooltipState is caller-owned tooltip visibility. Its zero value is closed.
// Transitions are synchronous value operations and never schedule work.
type TooltipState bool

// IsOpen reports whether the tooltip should be painted.
func (state TooltipState) IsOpen() bool { return bool(state) }

// Open returns the open state.
func (TooltipState) Open() TooltipState { return true }

// Close returns the closed state.
func (TooltipState) Close() TooltipState { return false }

// Toggle returns the opposite visibility state.
func (state TooltipState) Toggle() TooltipState { return !state }

// Dismiss closes the tooltip after an application-owned dismiss action.
func (TooltipState) Dismiss() TooltipState { return false }

// Escape closes the tooltip after the runtime routes an Escape key press.
func (TooltipState) Escape() TooltipState { return false }

// OutsidePress closes the tooltip after the runtime identifies an outside press.
func (TooltipState) OutsidePress() TooltipState { return false }

// TooltipDelays defines caller-selected delays. The application remains
// responsible for scheduling and cancelling deadline delivery.
type TooltipDelays struct {
	Open  time.Duration
	Close time.Duration
}

// OpenDeadline returns the earliest time at which an open transition may run.
func (delays TooltipDelays) OpenDeadline(now time.Time) time.Time {
	return tooltipDeadline(now, delays.Open)
}

// CloseDeadline returns the earliest time at which a close transition may run.
func (delays TooltipDelays) CloseDeadline(now time.Time) time.Time {
	return tooltipDeadline(now, delays.Close)
}

func tooltipDeadline(now time.Time, delay time.Duration) time.Time {
	if delay <= 0 {
		return now
	}
	return now.Add(delay)
}

// TooltipDefaultSideOffset is the gap between a tooltip and the control it
// names when TooltipOptions.SideOffset is zero. The compact icon buttons of
// desktop toolbars read better with the tooltip clear of them.
const TooltipDefaultSideOffset = float32(10)

// TooltipOptions describes a synchronous tooltip popup. Opening policy and
// timing are application-owned; painting never starts a timer or worker.
type TooltipOptions struct {
	Layer uint16
	// ID names the tooltip for its entrance and exit, as HoverCardOptions.ID does.
	// Leave it zero for a tooltip that appears and disappears at once.
	ID          gfx.ID
	Side        TooltipSide
	Align       TooltipAlign
	SideOffset  float32
	AlignOffset float32
	MaxWidth    float32
}

// Tooltip is one noninteractive Vega tooltip declaration.
type Tooltip struct {
	canvas   *gfx.Canvas
	theme    gfx.Theme
	viewport gfx.Rect
	anchor   gfx.Rect
	options  TooltipOptions
	disabled bool
}

// BeginTooltip creates a tooltip declaration.
func BeginTooltip(canvas *gfx.Canvas, theme gfx.Theme, viewport, anchor gfx.Rect, options TooltipOptions) Tooltip {
	return Tooltip{canvas: canvas, theme: theme, viewport: viewport, anchor: anchor, options: options}
}

// Disabled suppresses the popup for a disabled trigger.
func (tooltip Tooltip) Disabled() Tooltip {
	tooltip.disabled = true
	return tooltip
}

// Paint renders the label and arrow on the caller-provided layer. A label wider
// than MaxWidth wraps. The tooltip opens on the opposite side when the requested
// one has no room, and its arrow follows.
func (tooltip Tooltip) Paint(label string) gfx.Rect {
	if tooltip.disabled || tooltip.canvas == nil || label == "" || tooltip.viewport.Width <= 0 || tooltip.viewport.Height <= 0 {
		return gfx.Rect{}
	}
	metrics := tooltip.theme.Metrics
	maxWidth := tooltip.options.MaxWidth
	if maxWidth <= 0 {
		maxWidth = 320
	}
	maxWidth = min(maxWidth, tooltip.viewport.Width-8)
	const padX, padY = float32(12), float32(6)
	var scratch overlayLines
	lines := overlayWrap(tooltip.canvas, &scratch, label, metrics.FontSizeS, gfx.WeightRegular, maxWidth-2*padX)
	// One line takes the width of its text; a wrapped label fills the whole
	// maximum, as a `w-fit max-w-xs` box does.
	textWidth := maxWidth - 2*padX
	if len(lines) == 1 {
		textWidth = tooltip.canvas.TextWidthWeight(lines[0], metrics.FontSizeS, gfx.WeightRegular)
	}
	lineHeight := tooltipLineHeight(metrics)
	size := gfx.Size{Width: textWidth + 2*padX, Height: float32(len(lines))*lineHeight + 2*padY}
	offset := tooltip.options.SideOffset
	if offset == 0 {
		offset = TooltipDefaultSideOffset
	}
	content, side := PlaceOverlay(tooltip.anchor, size, tooltip.viewport, OverlayPlacement{
		Side: tooltipOverlaySide(tooltip.options.Side), Align: OverlayAlign(tooltip.options.Align),
		SideOffset: offset, AlignOffset: tooltip.options.AlignOffset, Margin: 4,
	})
	tooltip.canvas.PushLayer(tooltip.options.Layer)
	tooltip.canvas.BeginPresence(overlayKey(tooltip.options.ID), overlaySurfacePart,
		popupPresence(tooltip.theme, tooltipDuration, side), popupPivot(content, side, tooltip.anchor))
	tooltip.canvas.Rounded(content, metrics.RadiusControl, tooltip.theme.Foreground)
	overlayParagraph(tooltip.canvas, lines, gfx.Rect{X: content.X + padX, Y: content.Y + padY, Width: content.Width - 2*padX}, lineHeight, gfx.TextStyle{
		Size: metrics.FontSizeS, Weight: gfx.WeightRegular, Color: tooltip.theme.Background,
	}, false)
	tooltip.paintArrow(content, tooltipSideOf(side))
	tooltip.canvas.EndPresence()
	tooltip.canvas.PopLayer()
	return content
}

// tooltipLineHeight is the 16px line of `text-xs`.
func tooltipLineHeight(metrics gfx.Metrics) float32 { return metrics.FontSizeS + 5 }

func tooltipOverlaySide(side TooltipSide) OverlaySide {
	switch side {
	case TooltipRight:
		return OverlayRight
	case TooltipBottom:
		return OverlayBottom
	case TooltipLeft:
		return OverlayLeft
	}
	return OverlayTop
}

func tooltipSideOf(side OverlaySide) TooltipSide {
	switch side {
	case OverlayRight:
		return TooltipRight
	case OverlayBottom:
		return TooltipBottom
	case OverlayLeft:
		return TooltipLeft
	}
	return TooltipTop
}

// The arrow is the visible half of shadcn's 45°-rotated 10px square: about ten
// pixels wide where it meets the body and four deep, so its tip reaches the
// default side offset. Canvas has no triangle or rotation primitive, so the
// triangle is a staircase of half-pixel steps (one physical pixel on a 2x
// display). Each rectangle starts just inside the body and reaches one step
// farther out than the wider one before it; nesting leaves no seam to
// anti-alias between steps, and the union is the stair-stepped triangle.
const (
	tooltipArrowBase  = float32(10)
	tooltipArrowStep  = float32(0.5)
	tooltipArrowSteps = 8
	tooltipArrowInset = float32(1)
)

func (tooltip Tooltip) paintArrow(content gfx.Rect, side TooltipSide) {
	// Center on the trigger, but keep the base on the flat part of the body so
	// it never straddles a rounded corner after the body was clamped on screen.
	radius := tooltip.theme.Metrics.RadiusControl
	edge, center := float32(0), float32(0)
	switch side {
	case TooltipRight:
		edge, center = content.X, tooltipArrowCenter(tooltip.anchor.Y+tooltip.anchor.Height/2, content.Y, content.Height, radius)
	case TooltipBottom:
		edge, center = content.Y, tooltipArrowCenter(tooltip.anchor.X+tooltip.anchor.Width/2, content.X, content.Width, radius)
	case TooltipLeft:
		edge, center = content.X+content.Width, tooltipArrowCenter(tooltip.anchor.Y+tooltip.anchor.Height/2, content.Y, content.Height, radius)
	default:
		edge, center = content.Y+content.Height, tooltipArrowCenter(tooltip.anchor.X+tooltip.anchor.Width/2, content.X, content.Width, radius)
	}
	for step := range tooltipArrowSteps {
		reach := float32(step+1) * tooltipArrowStep
		// Chord of the 45° triangle at the middle of this step.
		width := tooltipArrowBase - 2*(float32(step)+0.5)*tooltipArrowStep
		var bounds gfx.Rect
		switch side {
		case TooltipRight:
			bounds = gfx.Rect{X: edge - reach, Y: center - width/2, Width: reach + tooltipArrowInset, Height: width}
		case TooltipBottom:
			bounds = gfx.Rect{X: center - width/2, Y: edge - reach, Width: width, Height: reach + tooltipArrowInset}
		case TooltipLeft:
			bounds = gfx.Rect{X: edge - tooltipArrowInset, Y: center - width/2, Width: reach + tooltipArrowInset, Height: width}
		default:
			bounds = gfx.Rect{X: center - width/2, Y: edge - tooltipArrowInset, Width: width, Height: reach + tooltipArrowInset}
		}
		tooltip.canvas.Fill(bounds, tooltip.theme.Foreground)
	}
}

// tooltipArrowCenter clamps the trigger's center to the straight part of the
// body edge, or centers the arrow when the body is too short to hold it.
func tooltipArrowCenter(anchorCenter, start, length, radius float32) float32 {
	margin := radius + tooltipArrowBase/2
	if length < 2*margin {
		return start + length/2
	}
	return min(max(anchorCenter, start+margin), start+length-margin)
}

// HoverTooltipDelay is how long the pointer rests on a control before its
// tooltip appears. The pinned shadcn TooltipProvider uses delay 0, so a
// tooltip appears with the pointer; an application may pass a longer delay to
// Paint when it wants one.
const HoverTooltipDelay = 0

// HoverTooltip shows the Hint of whichever hit the pointer has rested on for a
// while, as a tooltip beside it. The application keeps one value, declares
// Hint on the hits that deserve a tooltip (an icon-only button is the usual
// case) and calls Paint last in each frame, after everything the tooltip may
// overlap. Holding no timer or goroutine, it asks the canvas for one frame at
// the moment a tooltip is due and for nothing while idle.
type HoverTooltip struct {
	target gfx.ID
	since  time.Duration
	// suppressed is the control a click just hid the tooltip of; it stays
	// hidden until the pointer leaves that control, as Base UI's closeOnClick
	// does.
	suppressed gfx.ID
	// focus shows the hint of the keyboard-focused control too, without a delay.
	focus bool
	// shown is the control whose tooltip the last Paint drew, so a press or an
	// explicit Dismiss can suppress that control until the pointer leaves.
	shown gfx.ID
}

// Showing returns the control whose tooltip the last Paint drew, or zero.
func (hover *HoverTooltip) Showing() gfx.ID { return hover.shown }

// Dismiss hides the tooltip that is showing and keeps it hidden until the
// pointer leaves that control, returning whether anything was showing. The
// application calls it when Escape reaches the tooltip's control.
func (hover *HoverTooltip) Dismiss() bool {
	if hover.shown.IsZero() {
		return false
	}
	hover.suppressed, hover.target, hover.shown = hover.shown, gfx.ID{}, gfx.ID{}
	return true
}

// ShowOnFocus makes a keyboard-focused control with a Hint show its tooltip at
// once, as a web tooltip trigger does on focus-visible. It is off by default
// because some applications use Hint only as an accessibility description.
func (hover *HoverTooltip) ShowOnFocus() { hover.focus = true }

// Paint draws the tooltip of the hovered hit once it has been hovered for
// delay, and reports whether it did. The tooltip goes below the control, or
// above it when the viewport has no room below (see PlaceOverlay). It stays hidden while a button
// is pressed and for hits without a Hint.
func (hover *HoverTooltip) Paint(canvas *gfx.Canvas, theme gfx.Theme, viewport gfx.Rect, options TooltipOptions, delay time.Duration) bool {
	hovered := canvas.Hovered()
	if !canvas.Pressed().IsZero() {
		if !hover.shown.IsZero() {
			hover.suppressed = hover.shown
		}
		hover.target, hover.shown = gfx.ID{}, gfx.ID{}
		return false
	}
	if !hover.suppressed.IsZero() {
		if hovered == hover.suppressed {
			hover.target, hover.shown = gfx.ID{}, gfx.ID{}
			return false
		}
		hover.suppressed = gfx.ID{}
	}
	id := hovered
	immediate := false
	if hover.focus {
		if focused := canvas.FocusVisible(); !focused.IsZero() {
			id, immediate = focused, true
		}
	}
	if id.IsZero() {
		hover.target, hover.shown = gfx.ID{}, gfx.ID{}
		return false
	}
	hits := canvas.Hits()
	var hit gfx.Hit
	found := false
	for index := len(hits) - 1; index >= 0; index-- {
		if hits[index].ID == id && !hits[index].Block {
			hit, found = hits[index], hits[index].Hint != ""
			break
		}
	}
	if !found {
		hover.target, hover.shown = gfx.ID{}, gfx.ID{}
		return false
	}
	now := canvas.Now()
	if hover.target != id {
		hover.target, hover.since = id, now
	}
	if immediate {
		delay = 0
	}
	if remaining := delay - (now - hover.since); remaining > 0 {
		canvas.RequestFrameIn(remaining)
		hover.shown = gfx.ID{}
		return false
	}
	options.Side = TooltipBottom
	options.ID = id
	BeginTooltip(canvas, theme, viewport, hit.Bounds, options).Paint(hit.Hint)
	hover.shown = id
	return true
}
