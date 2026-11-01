package raster

import (
	"math"

	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

// The shapes follow the scene shader of the GPU backends (internal/platform/*/shaders) function
// for function: a pixel is shaded when its center is inside the shape's quad, and coverage comes
// from a signed distance in logical pixels, feathered over one device pixel.

func sqrt32(v float32) float32 { return float32(math.Sqrt(float64(v))) }

func clamp01(v float32) float32 { return min(max(v, 0), 1) }

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// roundedBoxDistance is the signed distance from p to a rounded box centered on the origin.
func roundedBoxDistance(px, py, halfX, halfY, radius float32) float32 {
	qx, qy := abs32(px)-halfX+radius, abs32(py)-halfY+radius
	return sqrt32(max(qx, 0)*max(qx, 0)+max(qy, 0)*max(qy, 0)) + min(max(qx, qy), 0) - radius
}

// drawRectangle paints a rectangle, rounded when it has a radius. A plain rectangle covers the
// pixels whose centers it contains and nothing is blended at its edge; a rounded one is feathered
// by the distance of each center to its outline.
func drawRectangle(c *Canvas, area render.Scissor, rc *render.Rectangle, scale float32) {
	b := rc.Bounds
	if !(b.Width > 0 && b.Height > 0) {
		return
	}
	box := intersect(area, rectangleBox(b, scale))
	if empty(box) {
		return
	}
	col := rc.Color
	radius := min(rc.Radius, min(b.Width, b.Height)*0.5)
	if !(radius > 0) {
		a := col.Alpha
		c.fill(box, col.Red*a, col.Green*a, col.Blue*a, a)
		return
	}
	ps := 1 / scale
	halfW, halfH := b.Width*0.5, b.Height*0.5
	// A pixel whose center is further than half a device pixel inside the outline is fully
	// covered. In the rows that are, the pixels between the corner arcs are all such pixels.
	solidRows := radius >= ps
	for y := box.Top; y < box.Bottom; y++ {
		ly := (float32(y)+0.5)*ps - b.Y
		qy := abs32(ly-halfH) - halfH + radius
		left, right := box.Left, box.Left // [left, right) is the solid middle of the row
		if solidRows && qy <= radius-0.5*ps {
			left = max(box.Left, int(math.Ceil(float64((b.X+radius)*scale)-0.5)))
			right = min(box.Right, int(math.Floor(float64((b.X+b.Width-radius)*scale)-0.5))+1)
			if right < left {
				left, right = box.Left, box.Left
			}
		}
		shade := func(from, to int) {
			i := y*c.Stride + from*4
			for x := from; x < to; x++ {
				lx := (float32(x)+0.5)*ps - b.X
				d := roundedBoxDistance(lx-halfW, ly-halfH, halfW, halfH, radius)
				cov := clamp01(0.5 - d/ps)
				if cov > 0 {
					a := col.Alpha * cov
					over(c.Pix, i, col.Red*a, col.Green*a, col.Blue*a, a)
				}
				i += 4
			}
		}
		if right > left {
			shade(box.Left, left)
			c.fill(render.Scissor{Left: left, Top: y, Right: right, Bottom: y + 1}, col.Red*col.Alpha, col.Green*col.Alpha, col.Blue*col.Alpha, col.Alpha)
			shade(right, box.Right)
		} else {
			shade(box.Left, box.Right)
		}
	}
}

// drawShadow paints a soft shadow: the casting rounded rectangle moved by the offset, grown by
// the spread and blurred over a band of the blur's width either side of its edge, with the
// casting rectangle itself cut out.
func drawShadow(c *Canvas, area render.Scissor, rc *render.Rectangle, scale float32) {
	b, sh := rc.Bounds, rc.Shadow
	box := intersect(area, shadowBox(rc, scale))
	if empty(box) {
		return
	}
	ps := 1 / scale
	col := rc.Color
	halfX, halfY := max(b.Width*0.5+sh.Spread, 0), max(b.Height*0.5+sh.Spread, 0)
	var radius float32
	if rc.Radius > 0 {
		radius = min(max(rc.Radius+sh.Spread, 0), min(halfX, halfY))
	}
	hole := min(rc.Radius, min(b.Width, b.Height)*0.5)
	halfW, halfH := b.Width*0.5, b.Height*0.5
	for y := box.Top; y < box.Bottom; y++ {
		ly := (float32(y)+0.5)*ps - b.Y
		// What does not change along the row: the distance terms in y, and the columns of the casting
		// rectangle that are further than half a device pixel inside it, where the cut-out leaves
		// nothing of the shadow.
		sy := abs32(ly-halfH-sh.OffsetY) - halfY + radius
		hy := abs32(ly-halfH) - halfH + hole
		holeFrom, holeTo := box.Right, box.Right
		if hole >= ps && hy <= hole-0.5*ps {
			holeFrom = max(box.Left, int(math.Ceil(float64((b.X+hole)*scale)-0.5)))
			holeTo = min(box.Right, int(math.Floor(float64((b.X+b.Width-hole)*scale)-0.5))+1)
			if holeTo <= holeFrom {
				holeFrom, holeTo = box.Right, box.Right
			}
		}
		i := y*c.Stride + box.Left*4
		for x := box.Left; x < box.Right; x++ {
			if x == holeFrom {
				i += (holeTo - holeFrom) * 4
				x = holeTo - 1
				holeFrom = box.Right
				continue
			}
			lx := (float32(x)+0.5)*ps - b.X
			sx := abs32(lx-halfW-sh.OffsetX) - halfX + radius
			d := sqrt32(max(sx, 0)*max(sx, 0)+max(sy, 0)*max(sy, 0)) + min(max(sx, sy), 0) - radius
			t := clamp01(0.5 - d/(2*sh.Blur))
			if t > 0 {
				alpha := t * t * (3 - 2*t)
				hx := abs32(lx-halfW) - halfW + hole
				inside := sqrt32(max(hx, 0)*max(hx, 0)+max(hy, 0)*max(hy, 0)) + min(max(hx, hy), 0) - hole
				alpha *= 1 - clamp01(0.5-inside/ps)
				alpha *= col.Alpha
				if alpha > 0 {
					over(c.Pix, i, col.Red*alpha, col.Green*alpha, col.Blue*alpha, alpha)
				}
			}
			i += 4
		}
	}
}

func cross2(ax, ay, bx, by float32) float32 { return ax*by - ay*bx }

// edgeDistance is the signed distance from p to the line through from and to, positive on the
// side the edge turns toward.
func edgeDistance(px, py, fx, fy, tx, ty float32) float32 {
	ex, ey := tx-fx, ty-fy
	length := max(sqrt32(ex*ex+ey*ey), 1e-6)
	return (ex*(py-fy) - ey*(px-fx)) / length
}

func segmentDistance(px, py, ax, ay, bx, by float32) float32 {
	ex, ey := bx-ax, by-ay
	along := clamp01(((px-ax)*ex + (py-ay)*ey) / max(ex*ex+ey*ey, 1e-12))
	dx, dy := px-ax-along*ex, py-ay-along*ey
	return sqrt32(dx*dx + dy*dy)
}

func finite(v float32) bool { return !math.IsNaN(float64(v)) && !math.IsInf(float64(v), 0) }

// drawTriangle paints a triangle of a mesh. EdgeMask names the edges on the silhouette, which are
// feathered over one device pixel; an edge it does not name is shared with another triangle and is
// cut exactly, by the top-left rule, so that the two meet with no gap and no overlap.
func drawTriangle(c *Canvas, area render.Scissor, rc *render.Rectangle, scale float32) {
	pts := rc.Points
	ax, ay := pts[0].X, pts[0].Y
	bx, by := pts[1].X, pts[1].Y
	cx, cy := pts[2].X, pts[2].Y
	abx, aby, acx, acy := bx-ax, by-ay, cx-ax, cy-ay
	bcx, bcy := cx-bx, cy-by
	area2 := cross2(abx, aby, acx, acy)
	longest2 := max(abx*abx+aby*aby, max(acx*acx+acy*acy, bcx*bcx+bcy*bcy))
	if !(finite(ax) && finite(ay) && finite(bx) && finite(by) && finite(cx) && finite(cy) && finite(area2) && finite(longest2)) || abs32(area2) < 1e-8 {
		return
	}
	edgeMask := uint32(rc.EdgeMask)
	bounded := edgeMask != 7 || abs32(area2) < longest2*0.125
	ps := 1 / scale
	lowX := min(ax, min(bx, cx)) - 0.5*ps
	lowY := min(ay, min(by, cy)) - 0.5*ps
	highX := max(ax, max(bx, cx)) + 0.5*ps
	highY := max(ay, max(by, cy)) + 0.5*ps
	box := intersect(area, triangleBox(&pts, scale))
	if empty(box) {
		return
	}
	colors := rc.VertexColors()
	orientation := float32(-1)
	if cross2(bx-ax, by-ay, cx-ax, cy-ay) > 0 {
		orientation = 1
	}
	px := [3][2]float32{{ax, ay}, {bx, by}, {cx, cy}}
	for y := box.Top; y < box.Bottom; y++ {
		dy := float32(y) + 0.5
		py := dy * ps
		i := y*c.Stride + box.Left*4
	pixels:
		for x := box.Left; x < box.Right; x, i = x+1, i+4 {
			dx := float32(x) + 0.5
			ppx := dx * ps
			if ppx < lowX || ppx > highX || py < lowY || py > highY {
				continue
			}
			if bounded {
				for e := uint32(0); e < 3; e++ {
					if edgeMask&(1<<e) != 0 {
						continue
					}
					a, b := px[e], px[(e+1)%3]
					a[0], a[1], b[0], b[1] = a[0]/ps, a[1]/ps, b[0]/ps, b[1]/ps
					direction := orientation
					if a[0] > b[0] || (a[0] == b[0] && a[1] > b[1]) {
						a, b = b, a
						direction = -direction
					}
					ex, ey := b[0]-a[0], b[1]-a[1]
					side := direction * cross2(ex, ey, dx-a[0], dy-a[1])
					ex, ey = ex*direction, ey*direction
					topLeft := ey < 0 || (ey == 0 && ex > 0)
					if side < 0 || (side == 0 && !topLeft) {
						continue pixels
					}
				}
			}
			const far = 1e6
			d0, d1, d2 := float32(far), float32(far), float32(far)
			if edgeMask&1 != 0 {
				d0 = orientation * edgeDistance(ppx, py, ax, ay, bx, by)
			}
			if edgeMask&2 != 0 {
				d1 = orientation * edgeDistance(ppx, py, bx, by, cx, cy)
			}
			if edgeMask&4 != 0 {
				d2 = orientation * edgeDistance(ppx, py, cx, cy, ax, ay)
			}
			inside := min(d0, min(d1, d2))
			if inside < 0 {
				// An acute offset line must not paint past the actual ends.
				s0, s1, s2 := float32(far), float32(far), float32(far)
				if edgeMask&1 != 0 {
					s0 = segmentDistance(ppx, py, ax, ay, bx, by)
				}
				if edgeMask&2 != 0 {
					s1 = segmentDistance(ppx, py, bx, by, cx, cy)
				}
				if edgeMask&4 != 0 {
					s2 = segmentDistance(ppx, py, cx, cy, ax, ay)
				}
				inside = -min(s0, min(s1, s2))
			}
			coverage := float32(1)
			if edgeMask != 0 {
				coverage = clamp01(0.5 + inside/ps)
			}
			// The color at this point of a shaded face (flat when the three are equal).
			ox, oy := ppx-ax, py-ay
			wb, wc := cross2(ox, oy, acx, acy)/area2, cross2(abx, aby, ox, oy)/area2
			shade := func(c0, c1, c2 float32) float32 { return clamp01(c0 + wb*(c1-c0) + wc*(c2-c0)) }
			r := shade(colors[0].Red, colors[1].Red, colors[2].Red)
			g := shade(colors[0].Green, colors[1].Green, colors[2].Green)
			bl := shade(colors[0].Blue, colors[1].Blue, colors[2].Blue)
			al := shade(colors[0].Alpha, colors[1].Alpha, colors[2].Alpha) * coverage
			if al > 0 {
				over(c.Pix, i, r*al, g*al, bl*al, al)
			}
		}
	}
}

// Box is the rectangle of device pixels outside which an instance paints nothing at scale, before
// any clip of its own: a conservative bound that a caller keeping a damage region adds for the
// instance it replaced or put in place. It can be empty.
func Box(rc *render.Rectangle, scale float32) render.Scissor {
	if !(scale > 0) {
		scale = 1
	}
	switch rc.Kind {
	case scene.KindTriangle:
		return triangleBox(&rc.Points, scale)
	case scene.KindShadow:
		if rc.Shadow.Blur > 0 {
			return shadowBox(rc, scale)
		}
	}
	return rectangleBox(rc.Bounds, scale)
}

func rectangleBox(b scene.Rect, scale float32) render.Scissor {
	x0, x1 := pixelSpan(b.X, b.X+b.Width, scale)
	y0, y1 := pixelSpan(b.Y, b.Y+b.Height, scale)
	return render.Scissor{Left: x0, Top: y0, Right: x1, Bottom: y1}
}

func shadowBox(rc *render.Rectangle, scale float32) render.Scissor {
	b := rc.Bounds
	extent := rc.Shadow.Extent()
	x0, x1 := pixelSpan(b.X-extent, b.X+b.Width+extent, scale)
	y0, y1 := pixelSpan(b.Y-extent, b.Y+b.Height+extent, scale)
	return render.Scissor{Left: x0, Top: y0, Right: x1, Bottom: y1}
}

func triangleBox(pts *[3]scene.Point, scale float32) render.Scissor {
	ps := 1 / scale
	lowX := min(pts[0].X, min(pts[1].X, pts[2].X)) - 0.5*ps
	lowY := min(pts[0].Y, min(pts[1].Y, pts[2].Y)) - 0.5*ps
	highX := max(pts[0].X, max(pts[1].X, pts[2].X)) + 0.5*ps
	highY := max(pts[0].Y, max(pts[1].Y, pts[2].Y)) + 0.5*ps
	return render.Scissor{
		Left: int(math.Ceil(float64(lowX*scale) - 0.5)), Top: int(math.Ceil(float64(lowY*scale) - 0.5)),
		Right: int(math.Floor(float64(highX*scale)-0.5)) + 1, Bottom: int(math.Floor(float64(highY*scale)-0.5)) + 1,
	}
}
