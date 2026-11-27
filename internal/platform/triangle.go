package platform

import "github.com/vivaui/viva/internal/render"

// PackTriangleCoverage returns the triangle marker and the two shade slots in
// the instance format shared by the GPU renderers. A clipped flat stroke uses
// the shade slots to carry its original coverage edges.
func PackTriangleCoverage(rectangle render.Rectangle) (marker float32, shade1, shade2 [4]float32) {
	const coverageSourceFlag = 1 << 4
	if coverage := rectangle.Coverage; coverage != nil {
		a, b, c := coverage.Points[0], coverage.Points[1], coverage.Points[2]
		return float32((rectangle.EdgeMask | coverageSourceFlag) + 1),
			[4]float32{a.X, a.Y, b.X, b.Y},
			[4]float32{c.X, c.Y, float32(coverage.EdgeMask), 0}
	}
	colors := rectangle.VertexColors()
	return float32(rectangle.EdgeMask + 1),
		[4]float32{colors[1].Red, colors[1].Green, colors[1].Blue, colors[1].Alpha},
		[4]float32{colors[2].Red, colors[2].Green, colors[2].Blue, colors[2].Alpha}
}
