package ui

import (
	"math"
	"strings"

	"github.com/vivaui/viva/gfx"
)

// snapScroll is the scroll offset the layout applies, in whole logical pixels.
// ScrollState.Offset keeps its fraction (the scrollbar and the application read
// it as reported), but the painted content moves on the logical pixel grid: a
// label's sub-pixel origin then stays constant while scrolling, so its native
// texture is reused instead of rebuilt. See app/painter.go's roundPosition.
func snapScroll(offset float32) float32 {
	return float32(math.Round(float64(offset)))
}

// The layout is solved one axis at a time, because a wrapped paragraph is as tall
// as the width it gets: widths first (measure, then solve), then heights (measure,
// then solve). Each is the same code with an axis, so a Row and a Column are one
// algorithm.

func startEdge(axis int) int { return [2]int{3, 0}[axis] } // left, top
func endEdge(axis int) int   { return [2]int{1, 2}[axis] } // right, bottom

func extent(r *Rect, axis int) *float32 {
	if axis == axisX {
		return &r.Width
	}
	return &r.Height
}

func origin(r *Rect, axis int) *float32 {
	if axis == axisX {
		return &r.X
	}
	return &r.Y
}

// inset is the padding and border on both sides of an axis.
func (n *node) inset(axis int) (start, end float32) {
	border := n.BorderWidth
	if n.Flags&flagOutline != 0 {
		border = 0 // an outline is drawn, it does not take room
	}
	return n.Pad[startEdge(axis)] + border, n.Pad[endEdge(axis)] + border
}

func (n *node) marginOn(axis int) (start, end float32) {
	return n.Margin[startEdge(axis)], n.Margin[endEdge(axis)]
}

// flows reports whether a container lays its children out along axis. A Flow does
// across, as a Row does, and wraps onto lines down (see flow.go).
func (n *node) flows(axis int) bool {
	return (n.Dir == row || n.Dir == flow) && axis == axisX || n.Dir == column && axis == axisY
}

// leading is the height of one line of a text: its LineHeight, or the font size
// plus seven, the same leading the components use.
func (n *node) leading() float32 {
	if n.LineH > 0 {
		return n.LineH
	}
	return n.FontSize + 7
}

// measure gives every element the size its content wants along axis. Elements
// are stored parent before child, so walking the slice backwards visits children
// first and needs no recursion.
func (f *frame) measure(axis int) {
	f.wrapCursor = 0
	for i := len(f.nodes) - 1; i >= 0; i-- {
		n := &f.nodes[i]
		if axis == axisX && n.Kind != kindContainer {
			f.measureLeaf(n)
		}
		if n.Kind == kindText && n.Flags&flagWrap != 0 && axis == axisY {
			first := int32(len(f.lines))
			// An empty text has no line at all, as WrapLines gives it none; anything
			// else that fits is one line and needs no breaking.
			if n.text != "" && n.Natural <= n.rect.Width-n.padX() && strings.IndexByte(n.text, '\n') < 0 {
				f.lines = append(f.lines, n.text)
			} else {
				f.lines = f.wrapText(f.lines, n)
			}
			n.LineFirst, n.LineCount = first, int32(len(f.lines))-first
			n.pref[axisY] = float32(n.LineCount)*n.leading() + n.padY()
		}
		if n.Kind == kindContainer {
			f.measureContainer(n, axis)
		}
		if d := n.Size[axis]; d.Kind == dimPx {
			n.pref[axis] = d.value
		}
		n.raw[axis] = n.pref[axis]
		n.pref[axis] = clamp(n.pref[axis], n.MinS[axis], n.MaxS[axis])
		// A baseline is measured on the clamped size, the one paint lays the
		// line out in.
		if axis == axisY {
			f.measureBaseline(n)
		}
	}
}

func (f *frame) measureLeaf(n *node) {
	if !n.hasOwnTheme() {
		f.measureLeafIn(n)
		return
	}
	saved := f.theme
	f.theme = f.ownTheme(n)
	f.measureLeafIn(n)
	f.theme = saved
}

// controlMeasurer measures a control: its geometry comes from the style layer,
// as its painting does, so the engine can sit below the styled controls.
var controlMeasurer = (*frame).measureControls

func (f *frame) measureLeafIn(n *node) {
	switch n.Kind {
	case kindText:
		if n.Flags&flagMono != 0 {
			// A Mono text is as wide as its cells: a wide character takes two, a combining
			// mark none, as the canvas measures and draws it.
			_, ch := f.canvas.MonoCellSize(n.FontSize, n.Face)
			n.pref = [2]float32{f.canvas.TextWidthStyle(n.text, n.textStyle()), ch}
		} else {
			n.pref = [2]float32{f.canvas.TextWidthWeight(n.text, n.FontSize, n.Weight), n.leading()}
		}
		n.Natural = n.pref[axisX]
		n.pref[axisX] += n.padX() // a Text takes its Pad and Border like a container
		n.pref[axisY] += n.padY()
	case kindIcon, kindImage:
		n.pref = [2]float32{16, 16}
	default:
		controlMeasurer(f, n)
	}
}

func (f *frame) measureContainer(n *node, axis int) {
	if n.Dir == grid {
		f.measureGrid(n, axis)
		return
	}
	if n.Dir == custom {
		f.measureCustom(n, axis)
		return
	}
	if n.Dir == flow && axis == axisY {
		f.measureFlowLines(n)
		return
	}
	var total float32
	count := 0
	for c := n.First; c >= 0; c = f.nodes[c].Next {
		child := &f.nodes[c]
		ms, me := child.marginOn(axis)
		size := child.pref[axis] + ms + me
		if n.flows(axis) {
			total += size
		} else {
			total = max(total, size)
		}
		count++
	}
	if n.flows(axis) && count > 1 {
		total += n.Gap * float32(count-1)
	}
	is, ie := n.inset(axis)
	n.pref[axis] = total + is + ie
	if n.Flags&flagScroll == 0 {
		return
	}
	// What a scroll area holds is its own business: it is as tall as it is told to
	// be, not as its children, and a Sideways one as wide. Content and ContentWidth
	// say how much they need.
	switch s := f.sidewaysState(n); {
	case axis == axisY:
		n.Content = total
	case s != nil:
		s.ContentWidth = total
	default:
		return
	}
	if n.Size[axis].Kind == dimAuto && n.MaxS[axis] == 0 {
		n.pref[axis] = is + ie
	}
}

func (n *node) padX() float32 {
	s, e := n.inset(axisX)
	return s + e
}

func (n *node) padY() float32 {
	s, e := n.inset(axisY)
	return s + e
}

func clamp(v, lo, hi float32) float32 {
	if hi > 0 && v > hi {
		v = hi
	}
	if v < lo {
		v = lo
	}
	return max(v, 0)
}

// stretches reports whether an element with no size of its own fills the room
// across its container: containers and text do, controls keep their own size.
func (n *node) stretches(axis int) bool {
	switch n.Kind {
	case kindSlider:
		return (n.Natural > 0) == (axis == axisY) // as long as its container, never as thick
	case kindProgress:
		return axis == axisX // a bar is as wide as its column, but a row does not make it tall
	case kindContainer, kindText, kindSpacer, kindSlot, kindInput, kindInputGroup, kindTextarea, kindSegmented, kindTable, kindTabs, kindSelect, kindSplitter:
		return true
	}
	return false
}

// solve sets the size and position along axis of the children of container i,
// then of theirs. The container's own size and position on the axis are known.
func (f *frame) solve(i int32, axis int) {
	n := &f.nodes[i]
	if n.First < 0 {
		if n.outOfFlow&absChildren != 0 {
			f.solveAbsolute(i, axis)
		}
		return
	}
	is, ie := n.inset(axis)
	start := *origin(&n.rect, axis) + is
	avail := max(*extent(&n.rect, axis)-is-ie, 0)
	switch {
	case n.Dir == grid:
		f.solveGrid(n, axis, start, avail)
	case n.Dir == custom:
		f.solveCustom(n, axis, start, avail)
	case n.Dir == flow:
		f.solveFlowLines(n, axis, start, avail)
	case n.flows(axis):
		f.placeSplitRatios(n, axis, avail)
		f.solveFlow(n, n.First, -1, axis, start, avail)
	default:
		f.solveAcross(n, n.First, -1, axis, start, avail)
	}
	for c := n.First; c >= 0; c = f.nodes[c].Next {
		f.solve(c, axis)
	}
	if n.outOfFlow&absChildren != 0 {
		f.solveAbsolute(i, axis)
	}
}

// solveAcross places children on an axis the container does not flow along (the
// width of a Column, the height of a Row, either axis of a Stack): those from first
// up to stop, which is -1 after the last (a line of a Flow is a run of them).
func (f *frame) solveAcross(n *node, First, stop int32, axis int, start, avail float32) {
	if axis == axisX && n.Flags&flagScroll != 0 {
		if s := f.sidewaysState(n); s != nil {
			// A Sideways area lays its children out across its content, or across
			// itself where that is wider, and shows them OffsetX to the left, kept in
			// range of the content it has now.
			start -= snapScroll(min(s.OffsetX, max(s.ContentWidth-avail, 0)))
			avail = max(avail, s.ContentWidth)
		}
	}
	var line float32 // the baseline children are lined up on, from the top
	if axis == axisY && (n.Dir == row || n.Dir == flow) {
		for c := First; c != stop; c = f.nodes[c].Next {
			child := &f.nodes[c]
			if child.alignOn(n) == Baseline {
				line = max(line, child.Margin[0]+child.baseline)
			}
		}
	}
	for c := First; c != stop; c = f.nodes[c].Next {
		child := &f.nodes[c]
		align := child.alignOn(n)
		ms, me := child.marginOn(axis)
		if align == Baseline && axis == axisY && (n.Dir == row || n.Dir == flow) {
			*extent(&child.rect, axis) = clamp(child.pref[axis], child.MinS[axis], f.maxOn(child, axis, avail))
			*origin(&child.rect, axis) = start + line - child.baseline
			continue
		}
		size := crossSize(child, axis, avail-ms-me, align, f.maxOn(child, axis, avail-ms-me))
		*extent(&child.rect, axis) = size
		*origin(&child.rect, axis) = start + ms + offset(align, avail-ms-me-size)
	}
}

// crossSize is the size of a child on an axis its container does not flow along.
func crossSize(child *node, axis int, avail float32, align Align, hi float32) float32 {
	d := child.Size[axis]
	lo := child.MinS[axis]
	switch {
	case d.Kind == dimPx:
		return clamp(d.value, lo, hi)
	case d.Kind == dimPercent:
		return clamp(avail*d.value/100, lo, hi)
	case d.Kind == dimFill, d.Kind == dimAuto && (align == Stretch || align == Auto && child.stretches(axis)):
		return clamp(max(avail, 0), lo, hi)
	}
	return clamp(child.pref[axis], lo, hi)
}

// maxOn is the largest size an element may have along axis in a container with avail
// room for it: its MaxWidth or MaxHeight, or the share of avail MaxWidthPercent or
// MaxHeightPercent gives, whichever is smaller. Zero is no limit.
func (f *frame) maxOn(n *node, axis int, avail float32) float32 {
	hi := n.MaxS[axis]
	if pct := f.exts[n.Ext].maxPct[axis]; pct > 0 {
		share := max(avail, 0) * pct / 100
		if hi == 0 || share < hi {
			hi = max(share, 0.001) // zero would mean no limit
		}
	}
	return hi
}

func offset(align Align, free float32) float32 {
	switch align {
	case Center:
		return free / 2
	case End:
		return free
	}
	return 0
}

// solveFlow shares the room along the axis the container flows on, the way CSS
// flexbox does: each child starts at its basis (a Basis, a percentage, or its
// preferred size), then free room goes to the children that grow, or a shortage is
// taken from the children that shrink, in proportion; a child that hits its minimum
// or maximum is frozen there and the rest is shared again among the others. It shares
// it among the children from first up to stop, which is -1 after the last (a line of
// a Flow is a run of them).
func (f *frame) solveFlow(n *node, First, stop int32, axis int, start, avail float32) {
	count := 0
	var fixed float32
	for c := First; c != stop; c = f.nodes[c].Next {
		child := &f.nodes[c]
		basis, grow := flexBasis(child, axis, avail), float32(0)
		if child.Flags&flagGrowSet != 0 {
			grow = child.Grow
		} else if child.Size[axis].Kind == dimFill {
			grow = 1
		}
		child.growNow = grow
		switch {
		case child.Flags&flagShrinkSet != 0:
			child.shrinkNow = child.Shrink
		case axis == axisX && (child.Kind == kindText || child.Kind == kindContainer),
			axis == axisY && child.Flags&flagScroll != 0:
			// Across a line of text or a container may give way; down a column an
			// element keeps its content's height (as CSS min-height:auto does) unless it
			// is a scroll area, which is what shrinking to fit is for.
			child.shrinkNow = 1
		default:
			child.shrinkNow = 0
		}
		*extent(&child.rect, axis) = basis
		ms, me := child.marginOn(axis)
		fixed += ms + me
		count++
	}
	if count > 1 {
		fixed += n.Gap * float32(count-1)
	}
	scrolls := axis == axisY && n.Flags&flagScroll != 0

	// Resolve flexible lengths as CSS does: share the room by each unfrozen child's
	// base size, clamp, and freeze the children whose clamp went the way the total
	// violation went (all minimums if the sum grew, all maximums if it shrank),
	// then share again among the rest. Every pass starts from the base sizes.
	for c := First; c != stop; c = f.nodes[c].Next {
		child := &f.nodes[c]
		child.base = *extent(&child.rect, axis)
		child.frozen = false
	}
	for pass := 0; pass <= count; pass++ {
		var frozenSize, baseSum, growSum, shrinkSum float32
		for c := First; c != stop; c = f.nodes[c].Next {
			child := &f.nodes[c]
			if child.frozen {
				frozenSize += *extent(&child.rect, axis)
				continue
			}
			baseSum += child.base
			growSum += child.growNow
			shrinkSum += child.shrinkNow * child.base
		}
		room := avail
		if scrolls {
			room = max(avail, baseSum+frozenSize+fixed)
		}
		free := room - frozenSize - baseSum - fixed
		var violation float32
		for c := First; c != stop; c = f.nodes[c].Next {
			child := &f.nodes[c]
			if child.frozen {
				continue
			}
			want := child.base
			switch {
			case free > 0 && growSum > 0:
				want += free * child.growNow / growSum
			case free < 0 && shrinkSum > 0:
				want += free * child.shrinkNow * child.base / shrinkSum
			}
			bounded := clamp(want, child.MinS[axis], f.maxOn(child, axis, avail))
			violation += bounded - want
			*extent(&child.rect, axis) = bounded
			child.bounded = bounded
			child.want = want
		}
		done := true
		for c := First; c != stop; c = f.nodes[c].Next {
			child := &f.nodes[c]
			if child.frozen {
				continue
			}
			clampedUp, clampedDown := child.bounded > child.want, child.bounded < child.want
			if violation == 0 || violation > 0 && clampedUp || violation < 0 && clampedDown {
				child.frozen = true
			} else {
				done = false
			}
		}
		if done {
			break
		}
	}

	var used float32
	for c := First; c != stop; c = f.nodes[c].Next {
		child := &f.nodes[c]
		size := clamp(*extent(&child.rect, axis), child.MinS[axis], f.maxOn(child, axis, avail))
		*extent(&child.rect, axis) = size
		used += size
	}
	used += fixed
	room := avail
	if scrolls {
		room = max(avail, used)
	}
	leftover := room - used
	pos, between := float32(0), n.Gap
	switch n.Justify {
	case Center:
		if leftover > 0 {
			pos = leftover / 2
		}
	case End:
		pos = leftover // what does not fit overflows at the start, as the end is where it is anchored
	case Between:
		if leftover > 0 && count > 1 {
			between += leftover / float32(count-1)
		}
	}
	if scrolls {
		if state := f.exts[n.Ext].scrollState; state != nil && state.pinned() {
			n.Scroll = max(used-avail, 0) // at the end of what is there now, not of last frame's
		}
		pos -= snapScroll(n.Scroll)
	}
	for c := First; c != stop; c = f.nodes[c].Next {
		child := &f.nodes[c]
		ms, me := child.marginOn(axis)
		pos += ms
		*origin(&child.rect, axis) = start + pos
		pos += *extent(&child.rect, axis) + me + between
	}
}

// flexBasis is the size a child starts from along axis before the room is shared: none
// when it fills, else its Basis, its percentage of avail, or its preferred size.
func flexBasis(child *node, axis int, avail float32) float32 {
	d := child.Size[axis]
	switch {
	case d.Kind == dimFill:
		return 0
	case child.Flags&flagBasis != 0:
		return child.Basis
	case d.Kind == dimPercent:
		return avail * d.value / 100
	}
	return child.raw[axis]
}

// alignOn is how a child sits across its container: its own Self, else the
// container's Items.
func (n *node) alignOn(Parent *node) Align {
	if n.Self != Auto {
		return n.Self
	}
	return Parent.Items
}

// measureBaseline finds the distance from the top of an element to its first line
// of text, for Baseline alignment. Text sits on the row the platform draws it on:
// LabelBaseline's for interface-font lines, the mono font's own baseline
// (Canvas.MonoBaseline) for a Mono text whose face the canvas measured. A control's
// label uses LabelBaseline's row; an element with no text has its bottom edge as
// baseline. The row is measured on an unstretched element: one that cross-stretches
// paints its line centered in the taller box, below this row. An icon-only button
// has no line, but still reports this row: its icon is centered where a label's
// line would sit, so it aligns with the text beside it instead of pinning to the
// row's bottom edge.
func (f *frame) measureBaseline(n *node) {
	switch n.Kind {
	case kindText:
		top, bottom := n.inset(axisY)
		height := n.leading()
		if n.Flags&flagWrap == 0 {
			height = max(n.pref[axisY]-top-bottom, 0)
		}
		row := f.canvas.LabelBaseline(gfx.Rect{Height: height}, gfx.TextStyle{Size: n.FontSize})
		if n.Flags&flagMono != 0 {
			// A Mono line stands on its font's own baseline in its cell; a face no
			// engine measured has no known row, and LabelBaseline's is the closest.
			if mono := f.canvas.MonoBaseline(n.FontSize, n.Face, height); mono > 0 {
				row = mono
			}
		}
		n.baseline = top + row
	case kindButton:
		n.baseline = controlBaseline(f, n)
	case kindContainer:
		n.baseline = n.pref[axisY]
		is, _ := n.inset(axisY)
		for c := n.First; c >= 0; c = f.nodes[c].Next {
			child := &f.nodes[c]
			if n.Dir == flow && child.GridRow != 0 {
				break // a Flow's baseline is its first line's
			}
			if child.Kind == kindContainer && child.baseline == child.pref[axisY] && child.First < 0 {
				continue
			}
			value := is + child.Margin[0] + child.baseline
			if n.Dir == row || n.Dir == flow {
				for k := n.First; k >= 0 && (n.Dir == row || f.nodes[k].GridRow == 0); k = f.nodes[k].Next {
					other := &f.nodes[k]
					value = max(value, is+other.Margin[0]+other.baseline)
				}
			}
			n.baseline = value
			break
		}
	default:
		n.baseline = n.pref[axisY]
	}
}
