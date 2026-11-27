package render

import (
	"fmt"
	"math"

	"github.com/vivaui/viva/internal/scene"
)

// ValidateRectangles checks the shapes of a scene before a renderer takes them: no
// non-finite bounds, radius or color, and a triangle with a valid edge mask.
func ValidateRectangles(rectangles []Rectangle) error {
	for index, rectangle := range rectangles {
		bounds := rectangle.Bounds
		if !finiteFloat(bounds.X) || !finiteFloat(bounds.Y) || !finiteFloat(bounds.Width) || !finiteFloat(bounds.Height) {
			return fmt.Errorf("render: rectangle %d has non-finite bounds", index)
		}
		if !finiteFloat(rectangle.Radius) || rectangle.Radius < 0 {
			return fmt.Errorf("render: rectangle %d has invalid radius %g", index, rectangle.Radius)
		}
		if !finiteFloat(rectangle.Color.Red) || !finiteFloat(rectangle.Color.Green) || !finiteFloat(rectangle.Color.Blue) || !finiteFloat(rectangle.Color.Alpha) {
			return fmt.Errorf("render: rectangle %d has a non-finite color", index)
		}
		if rectangle.Kind == scene.KindShadow {
			shadow := rectangle.Shadow
			if !finiteFloat(shadow.OffsetX) || !finiteFloat(shadow.OffsetY) || !finiteFloat(shadow.Blur) || !finiteFloat(shadow.Spread) || shadow.Blur < 0 {
				return fmt.Errorf("render: shadow %d has an invalid blur or offset", index)
			}
		}
		if rectangle.Kind == scene.KindTriangle {
			for pointIndex, point := range rectangle.Points {
				if !finiteFloat(point.X) || !finiteFloat(point.Y) {
					return fmt.Errorf("render: triangle %d point %d is non-finite", index, pointIndex)
				}
			}
			if !validTriangleEdgeMask(rectangle.EdgeMask) {
				return fmt.Errorf("render: triangle %d has invalid edge mask %d", index, rectangle.EdgeMask)
			}
			if source := rectangle.Coverage; source != nil {
				if rectangle.Shade.On || rectangle.EdgeMask > 7 ||
					source.EdgeMask&scene.StrokeQuadEdgeMask == 0 || !validTriangleEdgeMask(source.EdgeMask) {
					return fmt.Errorf("render: triangle %d has invalid stroke coverage", index)
				}
				for pointIndex, point := range source.Points {
					if !finiteFloat(point.X) || !finiteFloat(point.Y) {
						return fmt.Errorf("render: triangle %d stroke coverage point %d is non-finite", index, pointIndex)
					}
				}
			}
		}
	}
	return nil
}

func validTriangleEdgeMask(mask uint8) bool {
	if mask <= 7 {
		return true
	}
	// The first half has long edge 0→1 and an optional start cap 2→0.
	// The second has long edge 2→0 and an optional end cap 1→2.
	switch mask {
	case scene.StrokeQuadEdgeMask | 0b001, scene.StrokeQuadEdgeMask | 0b101,
		scene.StrokeQuadEdgeMask | 0b100, scene.StrokeQuadEdgeMask | 0b110:
		return true
	default:
		return false
	}
}

func finiteFloat(value float32) bool {
	return !math.IsNaN(float64(value)) && !math.IsInf(float64(value), 0)
}
