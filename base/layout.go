package base

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
	if axis == AxisX {
		return &r.Width
	}
	return &r.Height
}

func origin(r *Rect, axis int) *float32 {
	if axis == AxisX {
		return &r.X
	}
	return &r.Y
}

// inset is the padding and border on both sides of an axis.
func (n *Node) Inset(axis int) (start, end float32) {
	border := n.BorderWidth
	if n.Flags&flagOutline != 0 {
		border = 0 // an outline is drawn, it does not take room
	}
	return n.Pad[startEdge(axis)] + border, n.Pad[endEdge(axis)] + border
}

func (n *Node) marginOn(axis int) (start, end float32) {
	return n.Margin[startEdge(axis)], n.Margin[endEdge(axis)]
}

// isContainer describes layout, independently of the control's semantic kind.
// A Button or a Badge keeps its identity when it holds caller-owned children.
func (n *Node) isContainer() bool {
	return n.Kind == kindContainer ||
		(n.Kind == kindButton || n.Kind == kindBadge || n.Kind == kindToggle || n.Kind == kindSelect) && n.Flags&flagChildren != 0
}

// flows reports whether a container lays its children out along axis. A Flow does
// across, as a Row does, and wraps onto lines down (see flow.go).
func (n *Node) flows(axis int) bool {
	return (n.Dir == row || n.Dir == flow) && axis == AxisX || n.Dir == column && axis == AxisY
}

// leading is the height of one line of a text: its LineHeight, or the font size
// plus seven, the same leading the components use.
func (n *Node) leading() float32 {
	if n.LineH > 0 {
		return n.LineH
	}
	return n.FontSize + 7
}

// measure gives every element the size its content wants along axis. Elements
// are stored parent before child, so walking the slice backwards visits children
// first and needs no recursion.
func (f *Frame) measure(axis int) {
	f.wrapCursor = 0
	for i := len(f.Nodes) - 1; i >= 0; i-- {
		n := &f.Nodes[i]
		if axis == AxisX && !n.isContainer() {
			f.measureLeaf(n)
		}
		if n.Kind == kindText && n.Flags&flagWrap != 0 && axis == AxisY {
			first := int32(len(f.Lines))
			// An empty text has no line at all, as WrapLines gives it none; anything
			// else that fits is one line and needs no breaking.
			if n.Text != "" && n.Natural <= n.Rect.Width-n.PadX() && strings.IndexByte(n.Text, '\n') < 0 {
				f.Lines = append(f.Lines, n.Text)
			} else {
				f.Lines = f.wrapText(f.Lines, n)
			}
			n.LineFirst, n.LineCount = first, int32(len(f.Lines))-first
			n.Pref[AxisY] = float32(n.LineCount)*n.leading() + n.PadY()
		}
		if n.isContainer() {
			f.measureContainer(n, axis)
		}
		if d := n.Size[axis]; d.Kind == dimPx {
			n.Pref[axis] = d.Value
		}
		n.Raw[axis] = n.Pref[axis]
		n.Pref[axis] = clamp(n.Pref[axis], n.MinS[axis], n.MaxS[axis])
		// A baseline is measured on the clamped size, the one paint lays the
		// line out in.
		if axis == AxisY {
			f.measureBaseline(n)
		}
	}
}

func (f *Frame) measureLeaf(n *Node) {
	if !n.HasOwnTheme() {
		f.measureLeafIn(n)
		return
	}
	saved := f.Palette
	f.Palette = f.OwnTheme(n)
	f.measureLeafIn(n)
	f.Palette = saved
}

// controlMeasurers measures a control of each kind: its geometry comes from the
// style layer, as its painting does, so the engine can sit below the styled
// controls and a page links only the kinds it declares.
var controlMeasurers [controlKindCount]func(f *Frame, n *Node)

// SetControlMeasurer installs the measurement of one control kind. It is a
// bridge for the styled layer; callers of the page API never need it.
func SetControlMeasurer(kind Kind, measurer func(f *Frame, n *Node)) {
	if int(kind) < len(controlMeasurers) {
		controlMeasurers[kind] = measurer
	}
}

func (f *Frame) measureLeafIn(n *Node) {
	switch n.Kind {
	case kindText:
		if n.Flags&flagMono != 0 {
			// A Mono text is as wide as its cells: a wide character takes two, a combining
			// mark none, as the canvas measures and draws it.
			_, ch := f.Surface.MonoCellSize(n.FontSize, n.Face)
			n.Pref = [2]float32{f.Surface.TextWidthStyle(n.Text, n.textStyle()), ch}
		} else if n.TextDecor&textItalic != 0 {
			// An Italic text is as wide as the italic face the canvas draws it in, so the
			// box it is given and the glyphs in it agree.
			n.Pref = [2]float32{f.Surface.TextWidthStyle(n.Text, n.textStyle()), n.leading()}
		} else {
			n.Pref = [2]float32{f.Surface.TextWidthWeight(n.Text, n.FontSize, n.Weight), n.leading()}
		}
		n.Natural = n.Pref[AxisX]
		n.Pref[AxisX] += n.PadX() // a Text takes its Pad and Border like a container
		n.Pref[AxisY] += n.PadY()
	case kindIcon, kindImage:
		n.Pref = [2]float32{16, 16}
	default:
		if int(n.Kind) < len(controlMeasurers) {
			if measurer := controlMeasurers[n.Kind]; measurer != nil {
				measurer(f, n)
				return
			}
		}
		requireInstall(n.Kind, "measurer")
	}
}

func (f *Frame) measureContainer(n *Node, axis int) {
	if n.Dir == grid {
		f.measureGrid(n, axis)
		return
	}
	if n.Dir == custom {
		f.measureCustom(n, axis)
		return
	}
	if n.Dir == flow && axis == AxisY {
		f.measureFlowLines(n)
		return
	}
	var total float32
	count := 0
	for c := n.First; c >= 0; c = f.Nodes[c].Next {
		child := &f.Nodes[c]
		ms, me := child.marginOn(axis)
		size := child.Pref[axis] + ms + me
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
	is, ie := n.Inset(axis)
	n.Pref[axis] = total + is + ie
	if axis == AxisX && n.Ext != 0 && f.Exts[n.Ext].contained {
		// A Container is as wide as it is placed, not as what it holds, so the
		// queries inside it cannot change the width they read.
		n.Pref[axis] = is + ie
	}
	if n.Flags&flagScroll == 0 {
		return
	}
	// What a scroll area holds is its own business: it is as tall as it is told to
	// be, not as its children, and a Sideways one as wide. Content and ContentWidth
	// say how much they need.
	switch s := f.sidewaysState(n); {
	case axis == AxisY:
		n.Content = total
	case s != nil:
		s.ContentWidth = total
	default:
		return
	}
	if n.Size[axis].Kind == dimAuto && n.MaxS[axis] == 0 {
		n.Pref[axis] = is + ie
	}
}

func (n *Node) PadX() float32 {
	s, e := n.Inset(AxisX)
	return s + e
}

func (n *Node) PadY() float32 {
	s, e := n.Inset(AxisY)
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
func (n *Node) stretches(axis int) bool {
	switch n.Kind {
	case kindSlider:
		return (n.Natural > 0) == (axis == AxisY) // as long as its container, never as thick
	case kindProgress:
		return axis == AxisX // a bar is as wide as its column, but a row does not make it tall
	case kindContainer, kindText, kindSpacer, kindSlot, kindInput, kindInputGroup, kindTextarea, kindSegmented, kindTable, kindTabs, kindSelect, kindSplitter:
		return true
	}
	return false
}

// solve sets the size and position along axis of the children of container i,
// then of theirs. The container's own size and position on the axis are known.
func (f *Frame) solve(i int32, axis int) {
	n := &f.Nodes[i]
	if n.First < 0 {
		if n.OutOfFlow&absChildren != 0 {
			f.solveAbsolute(i, axis)
		}
		return
	}
	is, ie := n.Inset(axis)
	start := *origin(&n.Rect, axis) + is
	avail := max(*extent(&n.Rect, axis)-is-ie, 0)
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
	for c := n.First; c >= 0; c = f.Nodes[c].Next {
		f.solve(c, axis)
	}
	if n.OutOfFlow&absChildren != 0 {
		f.solveAbsolute(i, axis)
	}
}

// solveAcross places children on an axis the container does not flow along (the
// width of a Column, the height of a Row, either axis of a Stack): those from first
// up to stop, which is -1 after the last (a line of a Flow is a run of them).
func (f *Frame) solveAcross(n *Node, First, stop int32, axis int, start, avail float32) {
	if axis == AxisX && n.Flags&flagScroll != 0 {
		if s := f.sidewaysState(n); s != nil {
			// A Sideways area lays its children out across its content, or across
			// itself where that is wider, and shows them OffsetX to the left, kept in
			// range of the content it has now.
			start -= snapScroll(min(s.OffsetX, max(s.ContentWidth-avail, 0)))
			avail = max(avail, s.ContentWidth)
		}
	}
	var line float32 // the baseline children are lined up on, from the top
	if axis == AxisY && (n.Dir == row || n.Dir == flow) {
		for c := First; c != stop; c = f.Nodes[c].Next {
			child := &f.Nodes[c]
			if child.alignOn(n) == Baseline {
				line = max(line, child.Margin[0]+child.Baseline)
			}
		}
	}
	for c := First; c != stop; c = f.Nodes[c].Next {
		child := &f.Nodes[c]
		align := child.alignOn(n)
		ms, me := child.marginOn(axis)
		if align == Baseline && axis == AxisY && (n.Dir == row || n.Dir == flow) {
			*extent(&child.Rect, axis) = clamp(child.Pref[axis], child.MinS[axis], f.maxOn(child, axis, avail))
			*origin(&child.Rect, axis) = start + line - child.Baseline
			continue
		}
		size := crossSize(child, axis, avail-ms-me, align, f.maxOn(child, axis, avail-ms-me))
		*extent(&child.Rect, axis) = size
		*origin(&child.Rect, axis) = start + ms + offset(align, avail-ms-me-size)
	}
}

// crossSize is the size of a child on an axis its container does not flow along.
func crossSize(child *Node, axis int, avail float32, align Align, hi float32) float32 {
	d := child.Size[axis]
	lo := child.MinS[axis]
	switch {
	case d.Kind == dimPx:
		return clamp(d.Value, lo, hi)
	case d.Kind == dimPercent:
		return clamp(avail*d.Value/100, lo, hi)
	case d.Kind == dimFill, d.Kind == dimAuto && (align == Stretch || align == Auto && child.stretches(axis)):
		return clamp(max(avail, 0), lo, hi)
	}
	return clamp(child.Pref[axis], lo, hi)
}

// maxOn is the largest size an element may have along axis in a container with avail
// room for it: its MaxWidth or MaxHeight, the share of avail MaxWidthPercent or
// MaxHeightPercent gives, or, for a wrapped Text that may break inside a word, the
// room itself, whichever is smaller. Zero is no limit.
func (f *Frame) maxOn(n *Node, axis int, avail float32) float32 {
	hi := n.MaxS[axis]
	extra := &f.Exts[n.Ext]
	if pct := extra.maxPct[axis]; pct > 0 {
		share := max(avail, 0) * pct / 100
		if hi == 0 || share < hi {
			hi = max(share, 0.001) // zero would mean no limit
		}
	}
	if axis == AxisX && shrinksToRoom(n, extra) && (hi == 0 || avail < hi) {
		// A text that may break inside a word is one character wide at its narrowest,
		// so the width it asks for is the room it has, not its widest line, as CSS
		// overflow-wrap: anywhere says.
		hi = max(avail, 0.001) // zero would mean no limit
	}
	return hi
}

// breaksAnywhere reports whether n, whose Extra is extra, asks to break inside a word
// too wide for its line: a wrapped Text given OverflowWrapAnywhere (see Box.OverflowWrap).
func breaksAnywhere(n *Node, extra *Extra) bool {
	return n.Kind == kindText && n.Flags&flagWrap != 0 && extra.overflowWrap == OverflowWrapAnywhere
}

// shrinksToRoom reports whether the width n asks for is the room it is given rather
// than its widest line. CSS overflow-wrap: anywhere lowers the min-content contribution
// only: a specified width, a percentage or a flex basis is still the size the caller
// asked for, so the room caps only a width the content itself derives.
func shrinksToRoom(n *Node, extra *Extra) bool {
	return breaksAnywhere(n, extra) && n.Size[AxisX].Kind == dimAuto && n.Flags&flagBasis == 0
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
func (f *Frame) solveFlow(n *Node, First, stop int32, axis int, start, avail float32) {
	count := 0
	var fixed float32
	for c := First; c != stop; c = f.Nodes[c].Next {
		child := &f.Nodes[c]
		basis, grow := flexBasis(child, axis, avail), float32(0)
		if child.Flags&flagGrowSet != 0 {
			grow = child.Grow
		} else if child.Size[axis].Kind == dimFill {
			grow = 1
		}
		child.GrowNow = grow
		switch {
		case child.Flags&flagShrinkSet != 0:
			child.ShrinkNow = child.Shrink
		case axis == AxisX && (child.Kind == kindText || child.Kind == kindContainer),
			axis == AxisY && child.Flags&flagScroll != 0:
			// Across a line of text or a container may give way; down a column an
			// element keeps its content's height (as CSS min-height:auto does) unless it
			// is a scroll area, which is what shrinking to fit is for.
			child.ShrinkNow = 1
		default:
			child.ShrinkNow = 0
		}
		*extent(&child.Rect, axis) = basis
		ms, me := child.marginOn(axis)
		fixed += ms + me
		count++
	}
	if count > 1 {
		fixed += n.Gap * float32(count-1)
	}
	scrolls := axis == AxisY && n.Flags&flagScroll != 0

	// Resolve flexible lengths as CSS does: share the room by each unfrozen child's
	// base size, clamp, and freeze the children whose clamp went the way the total
	// violation went (all minimums if the sum grew, all maximums if it shrank),
	// then share again among the rest. Every pass starts from the base sizes.
	for c := First; c != stop; c = f.Nodes[c].Next {
		child := &f.Nodes[c]
		child.Base = *extent(&child.Rect, axis)
		child.Frozen = false
	}
	for pass := 0; pass <= count; pass++ {
		var frozenSize, baseSum, growSum, shrinkSum float32
		for c := First; c != stop; c = f.Nodes[c].Next {
			child := &f.Nodes[c]
			if child.Frozen {
				frozenSize += *extent(&child.Rect, axis)
				continue
			}
			baseSum += child.Base
			growSum += child.GrowNow
			shrinkSum += child.ShrinkNow * child.Base
		}
		room := avail
		if scrolls {
			room = max(avail, baseSum+frozenSize+fixed)
		}
		free := room - frozenSize - baseSum - fixed
		var violation float32
		for c := First; c != stop; c = f.Nodes[c].Next {
			child := &f.Nodes[c]
			if child.Frozen {
				continue
			}
			want := child.Base
			switch {
			case free > 0 && growSum > 0:
				want += free * child.GrowNow / growSum
			case free < 0 && shrinkSum > 0:
				want += free * child.ShrinkNow * child.Base / shrinkSum
			}
			bounded := clamp(want, child.MinS[axis], f.maxOn(child, axis, avail))
			violation += bounded - want
			*extent(&child.Rect, axis) = bounded
			child.Bounded = bounded
			child.Want = want
		}
		done := true
		for c := First; c != stop; c = f.Nodes[c].Next {
			child := &f.Nodes[c]
			if child.Frozen {
				continue
			}
			clampedUp, clampedDown := child.Bounded > child.Want, child.Bounded < child.Want
			if violation == 0 || violation > 0 && clampedUp || violation < 0 && clampedDown {
				child.Frozen = true
			} else {
				done = false
			}
		}
		if done {
			break
		}
	}

	var used float32
	for c := First; c != stop; c = f.Nodes[c].Next {
		child := &f.Nodes[c]
		size := clamp(*extent(&child.Rect, axis), child.MinS[axis], f.maxOn(child, axis, avail))
		*extent(&child.Rect, axis) = size
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
		if state := f.Exts[n.Ext].scrollState; state != nil && state.pinned() {
			n.Scroll = max(used-avail, 0) // at the end of what is there now, not of last frame's
		}
		pos -= snapScroll(n.Scroll)
	}
	for c := First; c != stop; c = f.Nodes[c].Next {
		child := &f.Nodes[c]
		ms, me := child.marginOn(axis)
		pos += ms
		*origin(&child.Rect, axis) = start + pos
		pos += *extent(&child.Rect, axis) + me + between
	}
}

// flexBasis is the size a child starts from along axis before the room is shared: none
// when it fills, else its Basis, its percentage of avail, or its preferred size.
func flexBasis(child *Node, axis int, avail float32) float32 {
	d := child.Size[axis]
	switch {
	case d.Kind == dimFill:
		return 0
	case child.Flags&flagBasis != 0:
		return child.Basis
	case d.Kind == dimPercent:
		return avail * d.Value / 100
	}
	return child.Raw[axis]
}

// alignOn is how a child sits across its container: its own Self, else the
// container's Items.
func (n *Node) alignOn(Parent *Node) Align {
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
func (f *Frame) measureBaseline(n *Node) {
	switch n.Kind {
	case kindText:
		top, bottom := n.Inset(AxisY)
		height := n.leading()
		if n.Flags&flagWrap == 0 {
			height = max(n.Pref[AxisY]-top-bottom, 0)
		}
		row := f.Surface.LabelBaseline(gfx.Rect{Height: height}, gfx.TextStyle{Size: n.FontSize})
		if n.Flags&flagMono != 0 {
			// A Mono line stands on its font's own baseline in its cell; a face no
			// engine measured has no known row, and LabelBaseline's is the closest.
			if mono := f.Surface.MonoBaseline(n.FontSize, n.Face, height); mono > 0 {
				row = mono
			}
		}
		n.Baseline = top + row
	case kindButton, kindBadge, kindToggle, kindSelect:
		if n.Flags&flagChildren == 0 {
			if ControlBaseline != nil {
				n.Baseline = ControlBaseline(f, n)
			}
			break
		}
		fallthrough
	case kindContainer:
		n.Baseline = n.Pref[AxisY]
		for c := n.First; c >= 0; c = f.Nodes[c].Next {
			child := &f.Nodes[c]
			if n.Dir == flow && child.GridRow != 0 {
				break // a Flow's baseline is its first line's
			}
			if child.Kind == kindContainer && child.Baseline == child.Pref[AxisY] && child.First < 0 {
				continue
			}
			value := f.childBaseline(n, child)
			if n.Dir == row || n.Dir == flow {
				for k := n.First; k >= 0 && (n.Dir == row || f.Nodes[k].GridRow == 0); k = f.Nodes[k].Next {
					value = max(value, f.childBaseline(n, &f.Nodes[k]))
				}
			}
			n.Baseline = value
			break
		}
	default:
		n.Baseline = n.Pref[AxisY]
	}
}

// childBaseline uses the same cross-axis alignment and available height as
// solveAcross, so a centered child reports the row it will actually draw on.
func (f *Frame) childBaseline(parent, child *Node) float32 {
	inset, end := parent.Inset(AxisY)
	baseline := inset + child.Margin[0] + child.Baseline
	if parent.Dir != row && parent.Dir != flow {
		return baseline
	}
	align := child.alignOn(parent)
	if align == Baseline {
		return baseline
	}
	avail := max(parent.Pref[AxisY]-inset-end, 0)
	margin := avail - child.Margin[0] - child.Margin[2]
	height := crossSize(child, AxisY, margin, align, f.maxOn(child, AxisY, margin))
	return baseline + offset(align, margin-height)
}
