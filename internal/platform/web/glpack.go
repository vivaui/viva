package web

import (
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

// tileRanges splits a label's device-pixel width into horizontal texture
// ranges no wider than maxTex, with a two-pixel overlap so linear sampling at
// a seam blends neighbouring texels instead of clamping.
func tileRanges(deviceWidth, maxTex int) [][2]int {
	if deviceWidth <= 0 || maxTex <= 0 {
		return nil
	}
	if deviceWidth <= maxTex {
		return [][2]int{{0, deviceWidth}}
	}
	step := max(1, maxTex-2)
	var ranges [][2]int
	for left := 0; left < deviceWidth; left += step {
		right := min(deviceWidth, left+maxTex)
		if right <= left {
			break
		}
		ranges = append(ranges, [2]int{left, right})
	}
	return ranges
}

// packInstance writes one render instance as the five vec4 slots the WebGL2
// scene shader reads: bounds, color, style, Shade1 and Shade2. A rectangle uses
// Bounds and Style.x (radius); a triangle packs its three points and edge mask;
// a shadow packs its casting rectangle, blur, and offset/spread.
func packInstance(dst []float32, instance render.Rectangle) {
	for index := 0; index < 20; index++ {
		dst[index] = 0
	}
	colors := instance.VertexColors()
	setColor := func(at int, color scene.Color) {
		dst[at], dst[at+1], dst[at+2], dst[at+3] = color.Red, color.Green, color.Blue, color.Alpha
	}
	switch instance.Kind {
	case scene.KindTriangle:
		points := instance.Points
		dst[0], dst[1], dst[2], dst[3] = points[0].X, points[0].Y, points[1].X, points[1].Y
		dst[8], dst[9], dst[10], dst[11] = points[2].X, float32(instance.Layer), points[2].Y, float32(instance.EdgeMask)+1
		setColor(4, colors[0])
		setColor(12, colors[1])
		setColor(16, colors[2])
	case scene.KindShadow:
		dst[0], dst[1], dst[2], dst[3] = instance.Bounds.X, instance.Bounds.Y, instance.Bounds.Width, instance.Bounds.Height
		dst[8], dst[9], dst[10], dst[11] = instance.Radius, 0, instance.Shadow.Blur, 0
		setColor(4, instance.Color)
		dst[12], dst[13], dst[14], dst[15] = instance.Shadow.OffsetX, instance.Shadow.OffsetY, instance.Shadow.Spread, 0
	default:
		dst[0], dst[1], dst[2], dst[3] = instance.Bounds.X, instance.Bounds.Y, instance.Bounds.Width, instance.Bounds.Height
		dst[8] = instance.Radius
		setColor(4, instance.Color)
	}
}
