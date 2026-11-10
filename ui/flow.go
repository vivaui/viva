package ui

import "math"

// A Flow (see Frame.Flow) is a Row that wraps. Across, it breaks its children into
// lines as wide as it is and shares the room of each line as a Row does; down, the
// lines stack from the top, each as tall as its tallest child. Lines are runs of
// consecutive children, and a child's gridRow numbers the line it is on: a line is
// the children from the first with its number up to the first with the next.
//
// Its width is measured as a Row's, the children side by side, so a Flow asks for
// one line and wraps only where it is given less.

// lineGap is the space between the lines of a Flow: its LineGap, else its Gap.
func (n *node) lineGap() float32 {
	if n.gridMin < 0 {
		return n.gap
	}
	return n.gridMin
}

// breakLines puts the children of Flow n on lines no wider than avail, as many on
// each as fit and at least one. A child takes its basis on a line, within its
// minimum and maximum, plus its margins, before the line's room is shared.
func (f *Frame) breakLines(n *node, avail float32) {
	var used float32
	line, count := 0, 0
	for c := n.first; c >= 0; c = f.nodes[c].next {
		child := &f.nodes[c]
		ms, me := child.marginOn(axisX)
		size := clamp(flexBasis(child, axisX, avail), child.minS[axisX], f.maxOn(child, axisX, avail)) + ms + me
		if count > 0 && used+n.gap+size > avail && line < math.MaxUint16 {
			line++
			used, count = 0, 0
		}
		if count > 0 {
			used += n.gap
		}
		used += size
		count++
		child.gridRow = uint16(line)
	}
}

// lineEnd is the child after the last on the line child c is on, or -1 when that
// line is the last.
func (f *Frame) lineEnd(c int32) int32 {
	line := f.nodes[c].gridRow
	c = f.nodes[c].next
	for c >= 0 && f.nodes[c].gridRow == line {
		c = f.nodes[c].next
	}
	return c
}

// lineHeight is how tall the line of the children from first up to stop is: as its
// tallest child with its margins.
func (f *Frame) lineHeight(first, stop int32) float32 {
	var height float32
	for c := first; c != stop; c = f.nodes[c].next {
		child := &f.nodes[c]
		ms, me := child.marginOn(axisY)
		height = max(height, child.pref[axisY]+ms+me)
	}
	return height
}

// measureFlowLines gives Flow n its preferred height: its lines one under the other,
// LineGap apart. The lines were broken when its width was solved, and its children
// were measured at the widths their lines gave them.
func (f *Frame) measureFlowLines(n *node) {
	var total float32
	lines := 0
	for first := n.first; first >= 0; {
		stop := f.lineEnd(first)
		total += f.lineHeight(first, stop)
		lines++
		first = stop
	}
	if lines > 1 {
		total += n.lineGap() * float32(lines-1)
	}
	is, ie := n.inset(axisY)
	n.pref[axisY] = total + is + ie
}

// solveFlowLines places the children of Flow n along axis: across, it breaks them
// into lines and shares each line's width as a Row does; down, it stacks the lines
// from start and places each child within its line's height as a Row does.
func (f *Frame) solveFlowLines(n *node, axis int, start, avail float32) {
	if axis == axisX {
		f.breakLines(n, avail)
		for first := n.first; first >= 0; {
			stop := f.lineEnd(first)
			f.solveFlow(n, first, stop, axisX, start, avail)
			first = stop
		}
		return
	}
	gap := n.lineGap()
	for first := n.first; first >= 0; {
		stop := f.lineEnd(first)
		height := f.lineHeight(first, stop)
		f.solveAcross(n, first, stop, axisY, start, height)
		start += height + gap
		first = stop
	}
}
