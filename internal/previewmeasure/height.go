// Package previewmeasure turns one painted documentation canvas into the
// height the component preview should occupy.
package previewmeasure

import "github.com/vivaui/viva/gfx"

// Content measures the painted primitives instead of trusting the iframe's
// current height. The page background covers the viewport and is ignored; every
// other primitive contributes its visible vertical extent.
func Content(canvas *gfx.Canvas, size gfx.Size) int {
	minY, maxY := float32(1e9), float32(-1e9)
	fillsHeight := false
	add := func(bounds, clip gfx.Rect, hasClip bool, alpha float32) {
		if alpha <= 0 {
			return
		}
		// State.Paint clips the whole page to the viewport. That clip must not
		// hide overflow from this measurement; a component's own smaller clip
		// still should.
		viewportClip := clip.Width >= size.Width-1 && clip.Height >= size.Height-1
		if hasClip && !viewportClip {
			bounds = intersect(bounds, clip)
			if bounds.Width <= 0 || bounds.Height <= 0 {
				return
			}
		}
		if bounds.Width >= size.Width-1 && bounds.Height >= size.Height-1 {
			return
		}
		if bounds.Height >= size.Height-1 {
			fillsHeight = true
		}
		if bounds.Y < minY {
			minY = bounds.Y
		}
		if bottom := bounds.Y + bounds.Height; bottom > maxY {
			maxY = bottom
		}
	}
	for _, rectangle := range canvas.Rectangles() {
		add(rectangle.Bounds, rectangle.ClipOf, rectangle.HasClip, rectangle.Color.Alpha)
	}
	for _, text := range canvas.Texts() {
		add(text.Bounds, text.ClipOf, text.HasClip, text.Style.Color.Alpha)
	}
	for _, image := range canvas.Images() {
		if !image.Backdrop {
			add(image.Bounds, image.ClipOf, image.HasClip, 1)
		}
	}
	for _, shadow := range canvas.Shadows() {
		add(shadow.Bounds, shadow.ClipOf, shadow.HasClip, shadow.Color.Alpha)
	}
	for _, triangle := range canvas.Triangles() {
		add(triangleBounds(triangle.Points), triangle.ClipOf, triangle.HasClip, triangle.Color.Alpha)
	}
	if maxY < minY {
		return 0
	}
	padding := gfx.DefaultMetrics().PadSurface
	height := int(maxY - minY + 2*padding + 0.5)
	if fillsHeight && height < 420 {
		height = 420
	}
	if height < 160 {
		return 160
	}
	if height > 520 {
		return 520
	}
	return height
}

func triangleBounds(points [3]gfx.Point) gfx.Rect {
	minX, minY := points[0].X, points[0].Y
	maxX, maxY := points[0].X, points[0].Y
	for _, point := range points[1:] {
		minX, minY = min(minX, point.X), min(minY, point.Y)
		maxX, maxY = max(maxX, point.X), max(maxY, point.Y)
	}
	return gfx.Rect{X: minX, Y: minY, Width: maxX - minX, Height: maxY - minY}
}

func intersect(a, b gfx.Rect) gfx.Rect {
	left, top := max(a.X, b.X), max(a.Y, b.Y)
	right, bottom := min(a.X+a.Width, b.X+b.Width), min(a.Y+a.Height, b.Y+b.Height)
	if right <= left || bottom <= top {
		return gfx.Rect{}
	}
	return gfx.Rect{X: left, Y: top, Width: right - left, Height: bottom - top}
}
