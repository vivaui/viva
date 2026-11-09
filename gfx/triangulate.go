package gfx

// This file turns closed contour rings into triangles: contour rings are
// classified into fills and holes by winding, and each fill with its holes is
// triangulated by earcut (see earcut.go). All scratch memory lives on the
// Canvas so steady-state paints allocate nothing.

// fillContours triangulates the flattened contours in canvas.pathPoints as
// one fill under the given rule.
func (canvas *Canvas) fillContours(rule PathFillRule, color Color) {
	count := len(canvas.contourEnds)
	if count == 0 {
		return
	}
	if cap(canvas.parentScratch) < count*3 {
		canvas.parentScratch = make([]int32, count*3)
		canvas.consumedScratch = make([]bool, count*2)
	}
	parents := canvas.parentScratch[:count]
	depths := canvas.parentScratch[count : count*2]
	drawnFlags := canvas.parentScratch[count*2 : count*3]
	consumed := canvas.consumedScratch[:count]

	// Compute ring areas once; degenerate rings cannot contain anything.
	areas := canvas.areasScratch
	if cap(areas) < count {
		areas = make([]float64, 0, count*2)
	}
	areas = areas[:0]
	for index := int32(0); index < int32(count); index++ {
		ring := canvas.contourSlice(index)
		areas = append(areas, polygonArea(ring))
	}
	canvas.areasScratch = areas

	for index := int32(0); index < int32(count); index++ {
		parents[index] = -1
		depths[index] = 0
		drawnFlags[index] = 0
		consumed[index] = false
		if abs64(areas[index]) < 1e-6 {
			consumed[index] = true
			continue
		}
		best := float64(1 << 60)
		for other := int32(0); other < int32(count); other++ {
			// Parent candidates are strictly larger containing rings.
			if other == index || abs64(areas[other]) <= abs64(areas[index]) {
				continue
			}
			if canvas.ringContains(other, index) {
				if abs64(areas[other]) < best {
					best = abs64(areas[other])
					parents[index] = other
				}
			}
		}
	}
	for index := int32(0); index < int32(count); index++ {
		depth := int32(0)
		for walk := parents[index]; walk >= 0; walk = parents[walk] {
			depth++
		}
		depths[index] = depth
	}

	// A ring is drawn with its direct children as holes when its chain
	// winding (nonzero) or nesting parity (even-odd) says it bounds a
	// filled region. Winding-zero rings are holes; their nonzero
	// descendants bound islands drawn as separate fills.
	winding := func(index int32) int32 {
		if rule == PathFillEvenOdd {
			if depths[index]%2 == 0 {
				return 1
			}
			return 0
		}
		total := int32(0)
		for walk := index; walk >= 0; walk = parents[walk] {
			if areas[walk] > 0 {
				total++
			} else {
				total--
			}
		}
		return total
	}
	// A nonzero-winding ring whose parent is already drawn adds no new
	// area: its region lies inside the parent's triangles. Rings inside
	// holes re-emerge and must be drawn again.
	var drawn func(int32) bool
	drawn = func(index int32) bool {
		if drawnFlags[index] == 0 {
			parent := parents[index]
			covered := parent >= 0 && drawn(parent)
			if winding(index) != 0 && !covered {
				drawnFlags[index] = 1
			} else {
				drawnFlags[index] = -1
			}
		}
		return drawnFlags[index] == 1
	}
	isHole := func(index int32) bool {
		return winding(index) == 0
	}

	for index := int32(0); index < int32(count); index++ {
		if consumed[index] || !drawn(index) {
			continue
		}
		holes := int32(0)
		for other := int32(0); other < int32(count); other++ {
			if parents[other] == index && !consumed[other] && isHole(other) {
				holes++
			}
		}
		canvas.ringPoints = canvas.ringPoints[:0]
		canvas.ringEnds = canvas.ringEnds[:0]
		canvas.ringPoints = appendRing(canvas.ringPoints, canvas.contourSlice(index))
		canvas.ringEnds = append(canvas.ringEnds, int32(len(canvas.ringPoints)))
		if holes == 0 {
			canvas.earClipRings(color)
			consumed[index] = true
			continue
		}
		// Bridge holes from largest to smallest for determinism.
		for holes > 0 {
			largest := int32(-1)
			largestArea := float64(-1)
			for other := int32(0); other < int32(count); other++ {
				if parents[other] == index && !consumed[other] && isHole(other) && abs64(areas[other]) > largestArea {
					largest, largestArea = other, abs64(areas[other])
				}
			}
			if largest < 0 {
				break
			}
			consumed[largest] = true
			holes--
			canvas.ringPoints = appendRing(canvas.ringPoints, canvas.contourSlice(largest))
			canvas.ringEnds = append(canvas.ringEnds, int32(len(canvas.ringPoints)))
		}
		canvas.earClipRings(color)
		consumed[index] = true
	}
}

// contourSlice returns the flattened contour with the given index.
func (canvas *Canvas) contourSlice(index int32) []Point {
	start := int32(0)
	if index > 0 {
		start = canvas.contourEnds[index-1]
	}
	return canvas.pathPoints[start:canvas.contourEnds[index]]
}

// ringContains reports whether any vertex of contour `inner` lies inside
// contour `outer` by winding number, tolerating shared vertices.
func (canvas *Canvas) ringContains(outer, inner int32) bool {
	ring := canvas.contourSlice(outer)
	points := canvas.contourSlice(inner)
	tries := len(points)
	if tries > 4 {
		tries = 4
	}
	for index := 0; index < tries; index++ {
		if windingNumber(points[index], ring) != 0 {
			return true
		}
	}
	return false
}

// polygonArea returns the signed shoelace area of a ring.
func polygonArea(ring []Point) float64 {
	total := 0.0
	for index := range ring {
		current := ring[index]
		next := ring[(index+1)%len(ring)]
		total += float64(current.X)*float64(next.Y) - float64(next.X)*float64(current.Y)
	}
	return total * 0.5
}

func abs64(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

// windingNumber returns the winding number of point around a ring using the
// standard crossing rule.
func windingNumber(point Point, ring []Point) int {
	winding := 0
	for index := range ring {
		a := ring[index]
		b := ring[(index+1)%len(ring)]
		if a.Y <= point.Y {
			if b.Y > point.Y && crossProduct(b, a, point) > 0 {
				winding++
			}
		} else if b.Y <= point.Y && crossProduct(b, a, point) < 0 {
			winding--
		}
	}
	return winding
}

// crossProduct returns the z of (b-a) × (p-a).
func crossProduct(a, b, p Point) float32 {
	return (b.X-a.X)*(p.Y-a.Y) - (b.Y-a.Y)*(p.X-a.X)
}

// pointInTriangle tests strict containment with boundary tolerance.
func pointInTriangle(p, a, b, c Point) bool {
	d1 := crossProduct(a, b, p)
	d2 := crossProduct(b, c, p)
	d3 := crossProduct(c, a, p)
	const eps = 1e-4
	hasPositive := d1 > eps || d2 > eps || d3 > eps
	hasNegative := d1 < -eps || d2 < -eps || d3 < -eps
	return !(hasPositive && hasNegative)
}
