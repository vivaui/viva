package ui

// Absolute positioning takes an element out of its container's flow and places it
// by its edges inside the container, the way CSS position:absolute does:
//
//	f.Stack().Size(40, 40).Children(func() {
//		f.Slot().Fill()
//		f.Row().Absolute().Top(-4).Right(-4).Size(12, 12).Radius(6).Background(red)
//	})
//
// An absolute element takes no room (its container is as large as its in-flow
// children make it, and they are laid out as if it were not there), is placed
// against the container's padding box (its rectangle less its border), is painted
// after the in-flow children in the order declared, and so wins pointer hits over
// them, and is clipped with them when the container clips.

// Bits of node.outOfFlow.
const (
	absSelf     uint8 = 1 << 0 // the element is absolute
	absChildren uint8 = 1 << 1 // the container has absolute children
	absEdgeBit        = 2      // bits 2..5 say which of top, right, bottom, left are set
)

// Absolute takes the element out of the flow of its container and places it by the
// edges given with Top, Right, Bottom, Left and Inset, measured from the container's
// padding box. On an axis with both edges set and no size of its own it stretches
// between them; with one edge it sits there at its own size; with neither it is placed
// by Self (or the container's Items), at the start unless asked otherwise. A Popup, or
// the root, cannot be absolute.
func (b Box) Absolute() Box {
	f, n := b.f, b.n()
	if n.outOfFlow&absSelf != 0 || n.parent < 0 {
		return b
	}
	parent := &f.nodes[n.parent]
	prev := int32(-1)
	for c := parent.first; c >= 0 && c != b.i; c = f.nodes[c].next {
		prev = c
	}
	if prev < 0 {
		parent.first = n.next
	} else {
		f.nodes[prev].next = n.next
	}
	if parent.last == b.i {
		parent.last = prev
	}
	n.next = -1
	n.outOfFlow |= absSelf
	parent.outOfFlow |= absChildren
	f.scratch.abs = append(f.scratch.abs, b.i)
	return b
}

// Top, Right, Bottom and Left give the distance of an absolute element's edge from the
// same edge of its container (negative reaches outside it). They have no effect on an
// element that is not Absolute.
func (b Box) Top(v float32) Box    { return b.edge(0, v) }
func (b Box) Right(v float32) Box  { return b.edge(1, v) }
func (b Box) Bottom(v float32) Box { return b.edge(2, v) }
func (b Box) Left(v float32) Box   { return b.edge(3, v) }

// Inset sets all four edges: Absolute().Inset(0) fills the container.
func (b Box) Inset(v float32) Box { return b.Top(v).Right(v).Bottom(v).Left(v) }

func (b Box) edge(side int, v float32) Box {
	b.n().outOfFlow |= 1 << (absEdgeBit + side)
	b.f.extra(b.i).edges[side] = v
	return b
}

// solveAbsolute places the absolute children of container i on an axis, then lays out
// what is inside them. The container's own rectangle on the axis is known.
func (f *frame) solveAbsolute(i int32, axis int) {
	p := &f.nodes[i]
	border := p.borderWidth
	if p.flags&flagOutline != 0 {
		border = 0
	}
	lo := *origin(&p.rect, axis) + border
	span := max(*extent(&p.rect, axis)-2*border, 0)
	for _, ai := range f.scratch.abs {
		a := &f.nodes[ai]
		if a.parent != i {
			continue
		}
		ms, me := a.marginOn(axis)
		hasStart := a.outOfFlow&(1<<(absEdgeBit+startEdge(axis))) != 0
		hasEnd := a.outOfFlow&(1<<(absEdgeBit+endEdge(axis))) != 0
		var edgeStart, edgeEnd float32
		if a.ext != 0 {
			edgeStart, edgeEnd = f.exts[a.ext].edges[startEdge(axis)], f.exts[a.ext].edges[endEdge(axis)]
		}
		var size float32
		switch d := a.size[axis]; {
		case d.kind == dimPx:
			size = d.value
		case d.kind == dimPercent:
			size = span * d.value / 100
		case d.kind == dimFill, d.kind == dimAuto && hasStart && hasEnd:
			size = span - ms - me
			if hasStart {
				size -= edgeStart
			}
			if hasEnd {
				size -= edgeEnd
			}
		default:
			size = a.pref[axis]
		}
		size = clamp(size, a.minS[axis], a.maxS[axis])
		at := lo + ms
		switch {
		case hasStart:
			at += edgeStart
		case hasEnd:
			at = lo + span - edgeEnd - me - size
		default:
			at += offset(a.alignOn(p), span-ms-me-size)
		}
		*extent(&a.rect, axis), *origin(&a.rect, axis) = size, at
		f.solve(ai, axis)
	}
}

// paintAbsolute paints the absolute children of container i, after its other children.
func (f *frame) paintAbsolute(i int32) {
	for _, ai := range f.scratch.abs {
		if f.nodes[ai].parent == i {
			f.paint(ai)
		}
	}
}
