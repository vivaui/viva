package base

// Where a Popup goes, in full. Placement is decided on each axis separately:
//
//   - Size: the popup's own size or its content's, limited to what the bounds (the
//     frame, or Within) can hold, so a popup larger than the frame is as large as the
//     frame and sits at its edge. A MaxHeight that makes a long list scroll is the
//     caller's: placement never looks inside the popup.
//   - Along the side axis (vertical for Below and Above, horizontal for Beside and
//     BesideLeft) the popup goes on the requested side of the anchor, gap pixels
//     away. If it does not fit there it goes on the opposite side when that side has
//     more room (so it flips when the other side fits, and when neither does it takes
//     the larger one).
//   - Across (horizontal for Below and Above, vertical for Beside), Start, Center or
//     End line its edges up with the anchor's; it is never flipped, only shifted.
//   - At has no side: its corner is the point.
//   - Last, on both axes, the popup is moved the least that puts it inside the bounds.
//     One that does not fit its side therefore covers part of the anchor.

// Within makes a Popup flip and stay inside r instead of the whole frame, for a menu
// that keeps a margin from the window's edge or stays in a pane.
func (b Box) Within(r Rect) Box {
	b.N()
	b.f.Extra(b.i).within = r
	return b
}

// BesideLeft is Beside for a popup that opens to the left of anchor and flips to its
// right when there is no room on the left.
func (b Box) BesideLeft(Anchor Rect, Gap float32, align Align) Box {
	n := b.N()
	n.Place, n.Anchor, n.PlaceGap, n.PlaceAlign = placeBesideLeft, Anchor, Gap, align
	return b
}

// BesideLeftBox is BesideLeft for an anchor that is an element of the page, as BelowBox
// is to Below.
func (b Box) BesideLeftBox(Anchor Box, Gap float32, align Align) Box {
	b.BesideLeft(Rect{}, Gap, align)
	b.N().AnchorRef = Anchor.i + 1
	return b
}

// placePopup sets a Popup's size and position on an axis by the rule above.
func (f *Frame) placePopup(n *Node, axis int) {
	bounds := f.bounds
	if n.Ext != 0 {
		if w := f.Exts[n.Ext].within; w.Width > 0 && w.Height > 0 {
			bounds = w
		}
	}
	lo, span := *origin(&bounds, axis), *extent(&bounds, axis)
	size := n.Pref[axis]
	switch d := n.Size[axis]; d.Kind {
	case dimPx:
		size = d.Value
	case dimFill:
		size = span
	case dimPercent:
		size = span * d.Value / 100
	}
	size = min(clamp(size, n.MinS[axis], n.MaxS[axis]), max(span, 0))
	anchor := n.Anchor
	if n.AnchorRef > 0 {
		anchor = f.Nodes[n.AnchorRef-1].Rect // laid out on this axis already
	}
	start := *origin(&anchor, axis)
	length := *extent(&anchor, axis)
	at := start
	switch {
	case n.Place == placeBelow || n.Place == placeAbove:
		if axis == AxisY {
			at = sideOf(start, start+length, n.PlaceGap, size, lo, lo+span, n.Place == placeBelow)
		} else {
			at = alignedTo(start, length, size, n.PlaceAlign) + n.PlaceShift
		}
	case n.Place == placeBeside || n.Place == placeBesideLeft:
		if axis == AxisX {
			at = sideOf(start, start+length, n.PlaceGap, size, lo, lo+span, n.Place == placeBeside)
		} else {
			at = alignedTo(start, length, size, n.PlaceAlign) + n.PlaceShift
		}
	}
	at = max(min(at, lo+span-size), lo)
	*extent(&n.Rect, axis), *origin(&n.Rect, axis) = size, at
}

// sideOf is where a popup size long starts when it hangs off the anchor
// [start, end) on the far side (after) or the near one, gap away: on the requested
// side if it fits there, else on the side with more room.
func sideOf(start, end, Gap, Size, lo, hi float32, after bool) float32 {
	atAfter, atBefore := end+Gap, start-Gap-Size
	roomAfter, roomBefore := hi-atAfter, start-Gap-lo
	if after {
		if Size <= roomAfter || roomBefore <= roomAfter {
			return atAfter
		}
		return atBefore
	}
	if Size <= roomBefore || roomAfter <= roomBefore {
		return atBefore
	}
	return atAfter
}

// alignedTo is where a popup size long starts to line its start, middle or end up with
// the anchor's.
func alignedTo(start, length, Size float32, align Align) float32 {
	switch align {
	case End:
		return start + length - Size
	case Center:
		return start + (length-Size)/2
	}
	return start
}
