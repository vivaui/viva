package ui

import "math"

// RoundedOutline strokes inside bounds without painting the interior. It is
// useful over translucent fills, where a solid border backing would show through.
func (canvas *Canvas) RoundedOutline(bounds Rect, radius, width float32, color Color) {
	if width <= 0 || color.Alpha <= 0 || bounds.Width <= 0 || bounds.Height <= 0 {
		return
	}
	if canvas.pixelScale > 0 {
		bounds = canvas.snapBox(bounds, radius)
		width = canvas.borderWidth(width)
	}
	if bounds.Width <= width*2 || bounds.Height <= width*2 {
		canvas.Rounded(bounds, radius, color)
		return
	}
	radius = max(0, min(radius, bounds.Width/2, bounds.Height/2))
	innerRadius := max(0, radius-width)
	// The two contours have matching vertices: join them directly instead of
	// flattening Beziers and triangulating a general polygon with a hole.
	segments := arcSegments(float64(radius*max(1, canvas.pixelScale)), math.Pi/2)
	var arc [65]Point // arcSegments is capped at 64; no retained geometry cache.
	sinStep, cosStep := math.Sincos(math.Pi / (2 * float64(segments)))
	x, y := 1.0, 0.0
	for index := 0; index < segments; index++ {
		arc[index] = Point{X: float32(x), Y: float32(y)}
		x, y = x*cosStep-y*sinStep, y*cosStep+x*sinStep
	}
	arc[segments] = Point{Y: 1} // Exact tangent endpoints keep straight seams closed.
	var firstOuter, firstInner, previousOuter, previousInner Point
	for corner := 0; corner < 4; corner++ {
		left, top := corner >= 2, corner == 0 || corner == 3
		outerCenter := Point{X: bounds.X + bounds.Width - radius, Y: bounds.Y + bounds.Height - radius}
		innerCenter := Point{X: bounds.X + bounds.Width - width - innerRadius, Y: bounds.Y + bounds.Height - width - innerRadius}
		if left {
			outerCenter.X, innerCenter.X = bounds.X+radius, bounds.X+width+innerRadius
		}
		if top {
			outerCenter.Y, innerCenter.Y = bounds.Y+radius, bounds.Y+width+innerRadius
		}
		for index := 0; index <= segments; index++ {
			direction := arc[index]
			switch corner {
			case 0:
				direction = Point{X: direction.Y, Y: -direction.X}
			case 2:
				direction = Point{X: -direction.Y, Y: direction.X}
			case 3:
				direction = Point{X: -direction.X, Y: -direction.Y}
			}
			outer := Point{X: outerCenter.X + radius*direction.X, Y: outerCenter.Y + radius*direction.Y}
			inner := Point{X: innerCenter.X + innerRadius*direction.X, Y: innerCenter.Y + innerRadius*direction.Y}
			if corner == 0 && index == 0 {
				firstOuter, firstInner = outer, inner
			} else {
				canvas.appendOutlineSegment(previousOuter, outer, previousInner, inner, color)
			}
			previousOuter, previousInner = outer, inner
		}
	}
	canvas.appendOutlineSegment(previousOuter, firstOuter, previousInner, firstInner, color)
}

func (canvas *Canvas) appendOutlineSegment(outerA, outerB, innerA, innerB Point, color Color) {
	// Feather only the contour edges. Internal diagonals and segment seams
	// must remain fully covered, especially with translucent strokes.
	if outerA != outerB {
		canvas.appendTriangleEdges(outerA, outerB, innerB, color, 1<<0)
	}
	if innerA != innerB {
		canvas.appendTriangleEdges(outerA, innerB, innerA, color, 1<<1)
	}
}
