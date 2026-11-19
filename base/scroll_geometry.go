package base

import "github.com/vivaui/viva/gfx"

// ScrollAreaBarSize is the thickness of a full-size scrollbar, as
// components.ScrollArea draws it when no size overrides it.
const ScrollAreaBarSize float32 = 10

// scrollAreaMinThumb is the shortest a bar's thumb gets, so a very long list
// still has a grabbable end.
const scrollAreaMinThumb float32 = 18

// ScrollAreaLayout is how a scroll area's bars are laid out: where it is
// scrolled, how thick its bars are, and the two shapes a table or a code view
// may give them. It is the value form of the options components.ScrollArea
// takes, for a caller that computes the bars without painting them.
type ScrollAreaLayout struct {
	// OffsetX and OffsetY are the content offsets, clamped to the scrollable
	// range before the thumbs are placed.
	OffsetX, OffsetY float32
	// BarSize is the thickness of both bars, in pixels.
	BarSize float32
	// ReserveBars puts the bars beside the viewport instead of over its edge.
	ReserveBars bool
	// Frozen pins the leading Frozen pixels of the content while the rest
	// scrolls; only the horizontal track and thumb shrink to the moving window.
	Frozen float32
}

// LayoutScrollArea computes the viewport, translated content, tracks, thumbs and
// corner of a scroll area. Content smaller than the viewport is widened to it,
// and offsets are clamped, as components.ScrollArea does. It is deterministic and
// allocation free.
func LayoutScrollArea(bounds gfx.Rect, content gfx.Size, layout ScrollAreaLayout) ScrollAreaGeometry {
	bounds = scrollAreaNormalizeRect(bounds)
	barSize := scrollAreaFiniteNonNegative(layout.BarSize)
	contentWidth := scrollAreaFiniteNonNegative(content.Width)
	contentHeight := scrollAreaFiniteNonNegative(content.Height)
	viewport := bounds
	if layout.ReserveBars {
		viewport = scrollAreaReservedViewport(bounds, contentWidth, contentHeight, barSize)
	}
	if contentWidth < viewport.Width {
		contentWidth = viewport.Width
	}
	if contentHeight < viewport.Height {
		contentHeight = viewport.Height
	}

	frozen := scrollAreaFiniteNonNegative(layout.Frozen)
	if frozen <= 0 || frozen >= viewport.Width || contentWidth <= viewport.Width {
		frozen = 0
	}
	maximumX := scrollAreaMax(contentWidth-viewport.Width, 0)
	maximumY := scrollAreaMax(contentHeight-viewport.Height, 0)
	offsetX := scrollAreaClamp(layout.OffsetX, 0, maximumX)
	offsetY := scrollAreaClamp(layout.OffsetY, 0, maximumY)
	hasHorizontal := maximumX > 0 && viewport.Height > 0
	hasVertical := maximumY > 0 && viewport.Width > 0

	geometry := ScrollAreaGeometry{
		Bounds: bounds, Viewport: viewport,
		Content: gfx.Rect{
			X: viewport.X - offsetX, Y: viewport.Y - offsetY,
			Width: contentWidth, Height: contentHeight,
		},
		OffsetX: offsetX, OffsetY: offsetY,
		MaxOffsetX: maximumX, MaxOffsetY: maximumY,
	}

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

func scrollAreaFiniteNonNegative(value float32) float32 {
	value = scrollAreaFinite(value)
	if value < 0 {
		return 0
	}
	return value
}

func scrollAreaMin(left, right float32) float32 {
	if left < right {
		return left
	}
	return right
}
