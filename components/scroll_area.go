package components

import "github.com/vivaui/viva/gfx"

const (
	scrollAreaBarSize      float32 = 10
	scrollAreaHoverBarSize float32 = 6
	scrollAreaBarInset     float32 = 1
	scrollAreaMinThumb     float32 = 18
)

// ScrollAreaGeometry is the complete deterministic layout of one externally
// controlled scroll area. Content is already translated by the clamped offset.
type ScrollAreaGeometry struct {
	Bounds          gfx.Rect
	Viewport        gfx.Rect
	Content         gfx.Rect
	VerticalTrack   gfx.Rect
	VerticalThumb   gfx.Rect
	HorizontalTrack gfx.Rect
	HorizontalThumb gfx.Rect
	Corner          gfx.Rect
	OffsetX         float32
	OffsetY         float32
	MaxOffsetX      float32
	MaxOffsetY      float32
}

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
// the vertical thumb. ID zero keeps the visual geometry non-interactive.
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
	bounds := scrollAreaNormalizeRect(area.bounds)
	barSize := area.barSize()
	contentWidth := scrollAreaFiniteNonNegative(area.contentSize.Width)
	contentHeight := scrollAreaFiniteNonNegative(area.contentSize.Height)
	viewport := bounds
	if area.reserveBars {
		viewport = scrollAreaReservedViewport(bounds, contentWidth, contentHeight, barSize)
	}
	if contentWidth < viewport.Width {
		contentWidth = viewport.Width
	}
	if contentHeight < viewport.Height {
		contentHeight = viewport.Height
	}

	// A pinned band needs a window left to scroll in — something to either
	// side of it, and a width inside the viewport — and changes nothing about
	// the translation: Content keeps the unfrozen origin, because the caller
	// clips the pinned columns out of it and paints them itself at the
	// viewport's edge. Only the horizontal bar belongs to the scrolling window
	// past the band.
	frozen := area.frozen
	if frozen <= 0 || frozen >= viewport.Width || contentWidth <= viewport.Width {
		frozen = 0
	}
	maximumX := scrollAreaMax(contentWidth-viewport.Width, 0)
	maximumY := scrollAreaMax(contentHeight-viewport.Height, 0)
	offsetX := scrollAreaClamp(area.offsetX, 0, maximumX)
	offsetY := scrollAreaClamp(area.offsetY, 0, maximumY)
	hasHorizontal := maximumX > 0 && viewport.Height > 0
	hasVertical := maximumY > 0 && viewport.Width > 0

	contentX := viewport.X - offsetX
	geometry := ScrollAreaGeometry{
		Bounds: bounds, Viewport: viewport,
		Content: gfx.Rect{
			X: contentX, Y: viewport.Y - offsetY,
			Width: contentWidth, Height: contentHeight,
		},
		OffsetX: offsetX, OffsetY: offsetY,
		MaxOffsetX: maximumX, MaxOffsetY: maximumY,
	}

	// The bars lie along the outer bounds. Where the viewport already excludes them
	// (ReserveScrollbars) their extents are the viewport's; otherwise the corner is
	// taken out of whichever track shares it. With a pinned band the horizontal bar
	// starts at the band's far edge: its thumb spans the scrolling window, and the
	// track covers the content its offset can reach.
	verticalExtent := bounds.Height
	horizontalExtent := bounds.Width
	if hasHorizontal && hasVertical {
		verticalExtent = scrollAreaMax(verticalExtent-barSize, 0)
		horizontalExtent = scrollAreaMax(horizontalExtent-barSize, 0)
		geometry.Corner = gfx.Rect{
			X:     bounds.X + bounds.Width - barSize,
			Y:     bounds.Y + bounds.Height - barSize,
			Width: barSize, Height: barSize,
		}
	}
	if hasVertical {
		geometry.VerticalTrack = gfx.Rect{
			X: bounds.X + bounds.Width - barSize, Y: bounds.Y,
			Width: barSize, Height: verticalExtent,
		}
		geometry.VerticalThumb = scrollAreaThumb(geometry.VerticalTrack, true, viewport.Height, contentHeight, offsetY, maximumY)
	}
	if hasHorizontal {
		trackX := bounds.X
		horizontalViewport := viewport.Width
		if frozen > 0 {
			trackX = bounds.X + frozen
			horizontalViewport = viewport.Width - frozen
		}
		geometry.HorizontalTrack = gfx.Rect{
			X: trackX, Y: bounds.Y + bounds.Height - barSize,
			Width: scrollAreaMax(horizontalExtent-(trackX-bounds.X), 0), Height: barSize,
		}
		geometry.HorizontalThumb = scrollAreaThumb(geometry.HorizontalTrack, false, horizontalViewport, contentWidth-frozen, offsetX, maximumX)
	}
	return geometry
}

// scrollAreaReservedViewport is bounds less the strips of the scrollbars the content needs
// in it. Adding one bar can shrink the other axis enough to need its bar too, so the two
// are resolved together. The content extents are the caller's, not yet widened to the
// viewport.
func scrollAreaReservedViewport(bounds gfx.Rect, contentWidth, contentHeight, barSize float32) gfx.Rect {
	hasHorizontal, hasVertical := contentWidth > bounds.Width, contentHeight > bounds.Height
	for range 3 {
		width, height := bounds.Width, bounds.Height
		if hasVertical {
			width = scrollAreaMax(width-barSize, 0)
		}
		if hasHorizontal {
			height = scrollAreaMax(height-barSize, 0)
		}
		nextHorizontal, nextVertical := contentWidth > width, contentHeight > height
		if nextHorizontal == hasHorizontal && nextVertical == hasVertical {
			break
		}
		hasHorizontal, hasVertical = nextHorizontal, nextVertical
	}
	viewport := bounds
	if hasVertical {
		viewport.Width = scrollAreaMax(viewport.Width-barSize, 0)
	}
	if hasHorizontal {
		viewport.Height = scrollAreaMax(viewport.Height-barSize, 0)
	}
	return viewport
}

// VerticalOffsetForThumb returns the content offset that places the top edge
// of the vertical thumb at thumbTop: the inverse of the thumb placement, clamped
// to [0, MaxOffsetY]. A caller turns a pointer drag on the scrollbar into its
// owned offset with it, without repeating the track insets. It is zero when
// the area does not scroll vertically.
func (geometry ScrollAreaGeometry) VerticalOffsetForThumb(thumbTop float32) float32 {
	travel := scrollAreaMax(geometry.VerticalTrack.Height-scrollAreaBarInset*2, 0) - geometry.VerticalThumb.Height
	if travel <= 0 || geometry.MaxOffsetY <= 0 {
		return 0
	}
	ratio := (thumbTop - (geometry.VerticalTrack.Y + scrollAreaBarInset)) / travel
	return scrollAreaClamp(ratio*geometry.MaxOffsetY, 0, geometry.MaxOffsetY)
}

// HorizontalOffsetForThumb returns the content offset that places the left edge
// of the horizontal thumb at thumbLeft. It is zero when the area does not scroll
// horizontally.
func (geometry ScrollAreaGeometry) HorizontalOffsetForThumb(thumbLeft float32) float32 {
	travel := scrollAreaMax(geometry.HorizontalTrack.Width-scrollAreaBarInset*2, 0) - geometry.HorizontalThumb.Width
	if travel <= 0 || geometry.MaxOffsetX <= 0 {
		return 0
	}
	ratio := (thumbLeft - (geometry.HorizontalTrack.X + scrollAreaBarInset)) / travel
	return scrollAreaClamp(ratio*geometry.MaxOffsetX, 0, geometry.MaxOffsetX)
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

func scrollAreaThumb(track gfx.Rect, vertical bool, viewportExtent, contentExtent, offset, maximum float32) gfx.Rect {
	inner := gfx.Rect{
		X: track.X + scrollAreaBarInset, Y: track.Y + scrollAreaBarInset,
		Width:  scrollAreaMax(track.Width-scrollAreaBarInset*2, 0),
		Height: scrollAreaMax(track.Height-scrollAreaBarInset*2, 0),
	}
	trackExtent := inner.Width
	if vertical {
		trackExtent = inner.Height
	}
	if trackExtent <= 0 || contentExtent <= 0 {
		return gfx.Rect{}
	}
	thumbExtent := trackExtent * viewportExtent / contentExtent
	thumbExtent = scrollAreaClamp(thumbExtent, scrollAreaMin(scrollAreaMinThumb, trackExtent), trackExtent)
	position := float32(0)
	if maximum > 0 {
		position = (trackExtent - thumbExtent) * offset / maximum
	}
	if vertical {
		return gfx.Rect{X: inner.X, Y: inner.Y + position, Width: inner.Width, Height: thumbExtent}
	}
	return gfx.Rect{X: inner.X + position, Y: inner.Y, Width: thumbExtent, Height: inner.Height}
}

func scrollAreaNormalizeRect(bounds gfx.Rect) gfx.Rect {
	bounds.X = scrollAreaFinite(bounds.X)
	bounds.Y = scrollAreaFinite(bounds.Y)
	bounds.Width = scrollAreaFiniteNonNegative(bounds.Width)
	bounds.Height = scrollAreaFiniteNonNegative(bounds.Height)
	return bounds
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
