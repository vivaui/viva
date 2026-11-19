package components

import (
	"github.com/vivaui/viva/base"
	"github.com/vivaui/viva/gfx"
)

const (
	scrollAreaBarSize              = base.ScrollAreaBarSize
	scrollAreaHoverBarSize float32 = 6
	scrollAreaBarInset     float32 = 1
)

// ScrollArea paints the visible scrollbars around a caller-painted clipped
// viewport. The caller owns offsets and content extents. Scoped wheel routing,
// pointer capture, inertial scrolling, and scroll scheduling belong to the
// runtime and are intentionally not simulated here.
type ScrollArea struct {
	canvas              *gfx.Canvas
	theme               gfx.Theme
	bounds              gfx.Rect
	contentSize         gfx.Size
	offsetX             float32
	offsetY             float32
	verticalID          gfx.ID
	verticalLabel       string
	verticalValue       string
	horizontalID        gfx.ID
	horizontalLabel     string
	horizontalValue     string
	viewportID          gfx.ID
	viewportLabel       string
	disabled            bool
	reserveBars         bool
	hoverBars           bool
	barsRevealed        bool
	barWidth            float32
	frozen              float32
	contentClipIsActive bool
}

// BeginScrollArea declares a source-owned scroll area with explicit content
// extents. Content smaller than the viewport is expanded to the viewport.
func BeginScrollArea(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, contentSize gfx.Size) ScrollArea {
	return ScrollArea{canvas: canvas, theme: theme, bounds: bounds, contentSize: contentSize}
}

// Offset sets the caller-owned horizontal and vertical scroll offsets.
func (area ScrollArea) Offset(x, y float32) ScrollArea {
	area.offsetX = x
	area.offsetY = y
	return area
}

// VerticalScrollbar assigns the stable action ID and accessibility strings for
// the vertical thumb. ID zero keeps the visual geometry non-interactive. An ID
// zero with an empty label omits the overlay while preserving its geometry, for
// a caller that paints the vertical bar itself.
func (area ScrollArea) VerticalScrollbar(id gfx.ID, label, value string) ScrollArea {
	area.verticalID = id
	area.verticalLabel = label
	area.verticalValue = value
	return area
}

// HorizontalScrollbar assigns the stable action ID and accessibility strings
// for the horizontal thumb. ID zero keeps the visual geometry non-interactive.
func (area ScrollArea) HorizontalScrollbar(id gfx.ID, label, value string) ScrollArea {
	area.horizontalID = id
	area.horizontalLabel = label
	area.horizontalValue = value
	return area
}

// Viewport makes the scrollable content a focusable group of id, so the
// keyboard can reach the area and ScrollForKey can move it. Content controls
// painted after BeginContent keep their own hits and win the press.
func (area ScrollArea) Viewport(id gfx.ID, label string) ScrollArea {
	area.viewportID, area.viewportLabel = id, label
	return area
}

// ReserveScrollbars sets the scrollbars beside the viewport instead of over its
// edge: Viewport excludes the bar strips, so content painted in it is never
// covered by a bar. Content wider than the strip left beside a vertical bar scrolls
// horizontally, so a caller sizes content meant to fit the viewport to the bounds less
// that bar. The default keeps the bars over the content.
func (area ScrollArea) ReserveScrollbars() ScrollArea {
	area.reserveBars = true
	return area
}

// HoverScrollbars switches to a hover scroll area, as shadcn takes it from
// Radix's `type="hover"`: the bars narrow to a small overlay thumb that floats
// over the content edge and paints only while hovered is true. hovered is the
// caller's own pointer-over-the-area verdict, so the component needs no pointer
// state, hide delay, timer, or animation; a caller that keeps its pointer
// capture alive passes that state too. Content keeps its full width, unlike
// ReserveScrollbars.
func (area ScrollArea) HoverScrollbars(hovered bool) ScrollArea {
	area.hoverBars = true
	area.barsRevealed = hovered
	return area
}

// ScrollbarSize overrides the thickness of both bars: a vertical bar's width or
// a horizontal bar's height. A zero or negative size keeps the style's default
// (the full-size bar, or the narrower hover bar). The thumb and a reserved
// viewport follow the same size, so a caller can give a hover bar the weight of
// a full-size one without moving the content between hidden and revealed.
func (area ScrollArea) ScrollbarSize(size float32) ScrollArea {
	area.barWidth = size
	return area
}

// Frozen declares that the leading px of the content stays pinned at the
// viewport's leading edge while the rest scrolls, as the frozen columns of a
// table stay put. The pinned width leaves MaxOffsetX the value an unfrozen
// area reports — the offset never scrolls the pinned columns — while the
// horizontal track, thumb and drag mapping shrink to the window past them and
// span exactly what scrolls. A zero or negative px, one with nothing to scroll
// around, or one at or past the viewport, freezes nothing. The vertical
// geometry is the unfrozen one.
func (area ScrollArea) Frozen(px float32) ScrollArea {
	area.frozen = px
	return area
}

// Disabled preserves geometry and typed semantics while making both scrollbars inert.
func (area ScrollArea) Disabled() ScrollArea {
	area.disabled = true
	return area
}

// barSize is the width of a bar's track: the caller's override, the narrow
// hover overlay, or the full-size bar.
func (area ScrollArea) barSize() float32 {
	if width := scrollAreaFiniteNonNegative(area.barWidth); width > 0 {
		return width
	}
	if area.hoverBars {
		return scrollAreaHoverBarSize
	}
	return scrollAreaBarSize
}

// Geometry returns clamped viewport, translated content, track, thumb, and
// corner geometry without painting or changing Canvas state. A frozen area
// keeps the unfrozen content, offset and vertical geometry; only the
// horizontal track, thumb and corner move to the scrolling window past the
// pinned band, spanning exactly what scrolls.
func (area ScrollArea) Geometry() ScrollAreaGeometry {
	return base.LayoutScrollArea(area.bounds, area.contentSize, base.ScrollAreaLayout{
		OffsetX: area.offsetX, OffsetY: area.offsetY,
		BarSize: area.barSize(), ReserveBars: area.reserveBars, Frozen: area.frozen,
	})
}

// BeginContent pushes the viewport clip and returns translated content geometry.
// Call EndContent after painting the caller-owned content.
func (area *ScrollArea) BeginContent() ScrollAreaGeometry {
	geometry := area.Geometry()
	if area.canvas != nil && !area.contentClipIsActive && geometry.Viewport.Width > 0 && geometry.Viewport.Height > 0 {
		if !area.viewportID.IsZero() && !area.disabled {
			area.canvas.HitWithOptions(area.viewportID, geometry.Viewport, area.viewportLabel, gfx.HitOptions{Role: gfx.RoleGroup})
		}
		area.canvas.Clip(geometry.Viewport)
		area.contentClipIsActive = true
	}
	return geometry
}

// EndContent closes the viewport clip and paints the visible scrollbars.
func (area *ScrollArea) EndContent() ScrollAreaGeometry {
	geometry := area.Geometry()
	if area.canvas == nil {
		return geometry
	}
	if area.contentClipIsActive {
		area.canvas.Unclip()
		area.contentClipIsActive = false
	}
	area.paintScrollbar(geometry.VerticalTrack, geometry.VerticalThumb, area.verticalID, area.verticalLabel, area.verticalValue)
	area.paintScrollbar(geometry.HorizontalTrack, geometry.HorizontalThumb, area.horizontalID, area.horizontalLabel, area.horizontalValue)
	return geometry
}

func (area ScrollArea) paintScrollbar(track, thumb gfx.Rect, id gfx.ID, label, value string) {
	if id.IsZero() && label == "" {
		return
	}
	if area.hoverBars && !area.barsRevealed {
		return
	}
	if track.Width <= 0 || track.Height <= 0 || thumb.Width <= 0 || thumb.Height <= 0 {
		return
	}
	color := area.theme.Border
	if !area.disabled && (area.canvas.Hovered() == id || area.canvas.Pressed() == id) {
		color = area.theme.MutedForeground
		color.Alpha *= 0.72
	}
	if area.disabled {
		color.Alpha *= 0.50
	}
	if !area.disabled && area.canvas.FocusVisible() == id {
		ring := area.theme.Ring
		ring.Alpha *= 0.50
		area.canvas.Rounded(scrollAreaExpand(thumb, 2), scrollAreaMin(thumb.Width, thumb.Height)/2+2, ring)
	}
	area.canvas.Rounded(thumb, scrollAreaMin(thumb.Width, thumb.Height)/2, color)
	if !id.IsZero() {
		area.canvas.HitWithOptions(id, track, label, gfx.HitOptions{
			Role: gfx.RoleSeparator, Value: value, Disabled: area.disabled, SkipFocus: true,
		})
	}
}

// ScrollForKey applies the keyboard's scrolling to the caller-owned offsets:
// Left/Right move horizontally, Up/Down vertically, PageUp/PageDown by a
// viewport, and Home/End to the start and end vertically. step is the arrow
// step (32 pixels when non-positive). It returns the new offsets and whether
// the key belongs to the scroll area.
func (area ScrollArea) ScrollForKey(x, y float32, key gfx.Key, step float32) (float32, float32, bool) {
	geometry := area.Geometry()
	if step <= 0 {
		step = 32
	}
	page := max(geometry.Viewport.Height*0.9, step)
	switch key {
	case gfx.KeyLeft:
		x -= step
	case gfx.KeyRight:
		x += step
	case gfx.KeyUp:
		y -= step
	case gfx.KeyDown:
		y += step
	case gfx.KeyPageUp:
		y -= page
	case gfx.KeyPageDown:
		y += page
	case gfx.KeyHome:
		y = 0
	case gfx.KeyEnd:
		y = geometry.MaxOffsetY
	default:
		return x, y, false
	}
	return scrollAreaClamp(x, 0, geometry.MaxOffsetX), scrollAreaClamp(y, 0, geometry.MaxOffsetY), true
}

func scrollAreaFinite(value float32) float32 {
	if value != value || value > 3.4028235e+38 || value < -3.4028235e+38 {
		return 0
	}
	return value
}

func scrollAreaFiniteNonNegative(value float32) float32 {
	value = scrollAreaFinite(value)
	if value < 0 {
		return 0
	}
	return value
}

func scrollAreaClamp(value, minimum, maximum float32) float32 {
	value = scrollAreaFinite(value)
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func scrollAreaMax(left, right float32) float32 {
	if left > right {
		return left
	}
	return right
}

func scrollAreaMin(left, right float32) float32 {
	if left < right {
		return left
	}
	return right
}

func scrollAreaExpand(bounds gfx.Rect, amount float32) gfx.Rect {
	return gfx.Rect{
		X: bounds.X - amount, Y: bounds.Y - amount,
		Width: bounds.Width + amount*2, Height: bounds.Height + amount*2,
	}
}

// Aliases into the style vocabulary, kept for callers of this component.
type ScrollAreaGeometry = base.ScrollAreaGeometry
