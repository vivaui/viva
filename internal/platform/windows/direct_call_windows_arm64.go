//go:build windows && arm64

package windows

import "github.com/ebitengine/purego"

type d2dPoint struct{ X, Y float32 }
type directTextCall struct {
	drawLayout func(this uintptr, origin d2dPoint, layout, brush uintptr, options uint32)
}

func (target *directTarget) initTextCall() {
	// D2D1_POINT_2F is a homogeneous floating-point aggregate on Windows ARM64:
	// its two floats belong in s0/s1, and the layout pointer follows this in x1.
	// Packing the point into a uintptr shifts that pointer and crashes Direct2D.
	purego.RegisterFunc(&target.drawLayout, vtable(target.target, d2dDrawTextLayout))
}
func (target *directTarget) drawTextLayout(x, y float32, layout uintptr, options uint32) {
	target.drawLayout(target.target, d2dPoint{x, y}, layout, target.brush, options)
}
