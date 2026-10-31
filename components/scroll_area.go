package components

import "github.com/vivaui/viva/ui"

const (
	scrollAreaBarSize  float32 = 10
	scrollAreaBarInset float32 = 1
	scrollAreaMinThumb float32 = 18
)

// ScrollAreaGeometry is the complete deterministic layout of one externally
// controlled scroll area. Content is already translated by the clamped offset.
type ScrollAreaGeometry struct {
	Bounds          ui.Rect
	Viewport        ui.Rect
	Content         ui.Rect
	VerticalTrack   ui.Rect
	VerticalThumb   ui.Rect
	HorizontalTrack ui.Rect
	HorizontalThumb ui.Rect
	Corner          ui.Rect
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
	canvas              *ui.Canvas
	theme               ui.Theme
	bounds              ui.Rect
	contentSize         ui.Size
	offsetX             float32
	offsetY             float32
	verticalID          ui.ID
	verticalLabel       string
	verticalValue       string
	horizontalID        ui.ID
	horizontalLabel     string
	horizontalValue     string
	viewportID          ui.ID
	viewportLabel       string
	disabled            bool
	contentClipIsActive bool
}

// BeginScrollArea declares a source-owned scroll area with explicit content
// extents. Content smaller than the viewport is expanded to the viewport.
func BeginScrollArea(canvas *ui.Canvas, theme ui.Theme, bounds ui.Rect, contentSize ui.Size) ScrollArea {
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
func (area ScrollArea) VerticalScrollbar(id ui.ID, label, value string) ScrollArea {
	area.verticalID = id
	area.verticalLabel = label
	area.verticalValue = value
	return area
}

// HorizontalScrollbar assigns the stable action ID and accessibility strings
// for the horizontal thumb. ID zero keeps the visual geometry non-interactive.
func (area ScrollArea) HorizontalScrollbar(id ui.ID, label, value string) ScrollArea {
	area.horizontalID = id
	area.horizontalLabel = label
	area.horizontalValue = value
	return area
}

// Viewport makes the scrollable content a focusable group of id, so the
// keyboard can reach the area and ScrollForKey can move it. Content controls
// painted after BeginContent keep their own hits and win the press.
func (area ScrollArea) Viewport(id ui.ID, label string) ScrollArea {
	area.viewportID, area.viewportLabel = id, label
	return area
}

// Disabled preserves geometry and typed semantics while making both scrollbars inert.
func (area ScrollArea) Disabled() ScrollArea {
	area.disabled = true
	return area
}

// Geometry returns clamped viewport, translated content, track, thumb, and
// corner geometry without painting or changing Canvas state.
func (area ScrollArea) Geometry() ScrollAreaGeometry {
	bounds := scrollAreaNormalizeRect(area.bounds)
	contentWidth := scrollAreaFiniteNonNegative(area.contentSize.Width)
	contentHeight := scrollAreaFiniteNonNegative(area.contentSize.Height)
	if contentWidth < bounds.Width {
		contentWidth = bounds.Width
	}
	if contentHeight < bounds.Height {
		contentHeight = bounds.Height
	}

	// Scrollbars occupy part of the same outer bounds. Resolve both flags to a
	// fixed point because introducing one scrollbar can reduce the other axis
	// enough to require its partner too. The viewport returned below is the
	// actual clipped content area, not the larger rectangle that includes bars.
	hasHorizontal := contentWidth > bounds.Width
	hasVertical := contentHeight > bounds.Height
	for range 3 {
		availableWidth, availableHeight := bounds.Width, bounds.Height
		if hasVertical {
			availableWidth = scrollAreaMax(availableWidth-scrollAreaBarSize, 0)
		}
		if hasHorizontal {
			availableHeight = scrollAreaMax(availableHeight-scrollAreaBarSize, 0)
		}
		nextHorizontal := hasHorizontal
		if availableWidth > 0 {
			nextHorizontal = contentWidth > availableWidth
		}
		nextVertical := hasVertical
		if availableHeight > 0 {
			nextVertical = contentHeight > availableHeight
		}
		if nextHorizontal == hasHorizontal && nextVertical == hasVertical {
			break
		}
		hasHorizontal, hasVertical = nextHorizontal, nextVertical
	}

	viewport := bounds
	if hasVertical {
		viewport.Width = scrollAreaMax(viewport.Width-scrollAreaBarSize, 0)
	}
	if hasHorizontal {
		viewport.Height = scrollAreaMax(viewport.Height-scrollAreaBarSize, 0)
	}
	maximumX := scrollAreaMax(contentWidth-viewport.Width, 0)
	maximumY := scrollAreaMax(contentHeight-viewport.Height, 0)
	offsetX := scrollAreaClamp(area.offsetX, 0, maximumX)
	offsetY := scrollAreaClamp(area.offsetY, 0, maximumY)

	geometry := ScrollAreaGeometry{
		Bounds: bounds, Viewport: viewport,
		Content: ui.Rect{
			X: viewport.X - offsetX, Y: viewport.Y - offsetY,
			Width: contentWidth, Height: contentHeight,
		},
		OffsetX: offsetX, OffsetY: offsetY,
		MaxOffsetX: maximumX, MaxOffsetY: maximumY,
	}

	if hasHorizontal && hasVertical {
		geometry.Corner = ui.Rect{
			X:     bounds.X + bounds.Width - scrollAreaBarSize,
			Y:     bounds.Y + bounds.Height - scrollAreaBarSize,
			Width: scrollAreaBarSize, Height: scrollAreaBarSize,
		}
	}
	if hasVertical {
		geometry.VerticalTrack = ui.Rect{
			X: bounds.X + bounds.Width - scrollAreaBarSize, Y: bounds.Y,
			Width: scrollAreaBarSize, Height: viewport.Height,
		}
		geometry.VerticalThumb = scrollAreaThumb(geometry.VerticalTrack, true, viewport.Height, contentHeight, offsetY, maximumY)
	}
	if hasHorizontal {
		geometry.HorizontalTrack = ui.Rect{
			X: bounds.X, Y: bounds.Y + bounds.Height - scrollAreaBarSize,
			Width: viewport.Width, Height: scrollAreaBarSize,
		}
		geometry.HorizontalThumb = scrollAreaThumb(geometry.HorizontalTrack, false, viewport.Width, contentWidth, offsetX, maximumX)
	}
	return geometry
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
			area.canvas.HitWithOptions(area.viewportID, geometry.Viewport, area.viewportLabel, ui.HitOptions{Role: ui.RoleGroup})
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

func (area ScrollArea) paintScrollbar(track, thumb ui.Rect, id ui.ID, label, value string) {
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
		area.canvas.HitWithOptions(id, track, label, ui.HitOptions{
			Role: ui.RoleSeparator, Value: value, Disabled: area.disabled, SkipFocus: true,
		})
	}
}

// ScrollForKey applies the keyboard's scrolling to the caller-owned offsets:
// Left/Right move horizontally, Up/Down vertically, PageUp/PageDown by a
// viewport, and Home/End to the start and end vertically. step is the arrow
// step (32 pixels when non-positive). It returns the new offsets and whether
// the key belongs to the scroll area.
func (area ScrollArea) ScrollForKey(x, y float32, key ui.Key, step float32) (float32, float32, bool) {
	geometry := area.Geometry()
	if step <= 0 {
		step = 32
	}
	page := max(geometry.Viewport.Height*0.9, step)
	switch key {
	case ui.KeyLeft:
		x -= step
	case ui.KeyRight:
		x += step
	case ui.KeyUp:
		y -= step
	case ui.KeyDown:
		y += step
	case ui.KeyPageUp:
		y -= page
	case ui.KeyPageDown:
		y += page
	case ui.KeyHome:
		y = 0
	case ui.KeyEnd:
		y = geometry.MaxOffsetY
	default:
		return x, y, false
	}
	return scrollAreaClamp(x, 0, geometry.MaxOffsetX), scrollAreaClamp(y, 0, geometry.MaxOffsetY), true
}

func scrollAreaThumb(track ui.Rect, vertical bool, viewportExtent, contentExtent, offset, maximum float32) ui.Rect {
	inner := ui.Rect{
		X: track.X + scrollAreaBarInset, Y: track.Y + scrollAreaBarInset,
		Width:  scrollAreaMax(track.Width-scrollAreaBarInset*2, 0),
		Height: scrollAreaMax(track.Height-scrollAreaBarInset*2, 0),
	}
	trackExtent := inner.Width
	if vertical {
		trackExtent = inner.Height
	}
	if trackExtent <= 0 || contentExtent <= 0 {
		return ui.Rect{}
	}
	thumbExtent := trackExtent * viewportExtent / contentExtent
	thumbExtent = scrollAreaClamp(thumbExtent, scrollAreaMin(scrollAreaMinThumb, trackExtent), trackExtent)
	position := float32(0)
	if maximum > 0 {
		position = (trackExtent - thumbExtent) * offset / maximum
	}
	if vertical {
		return ui.Rect{X: inner.X, Y: inner.Y + position, Width: inner.Width, Height: thumbExtent}
	}
	return ui.Rect{X: inner.X + position, Y: inner.Y, Width: thumbExtent, Height: inner.Height}
}

func scrollAreaNormalizeRect(bounds ui.Rect) ui.Rect {
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

func scrollAreaExpand(bounds ui.Rect, amount float32) ui.Rect {
	return ui.Rect{
		X: bounds.X - amount, Y: bounds.Y - amount,
		Width: bounds.Width + amount*2, Height: bounds.Height + amount*2,
	}
}
