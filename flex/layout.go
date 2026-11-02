package flex

import (
	"strings"
	"unicode/utf8"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/ui"
)

// The layout is solved one axis at a time, because a wrapped paragraph is as tall
// as the width it gets: widths first (measure, then solve), then heights (measure,
// then solve). Each is the same code with an axis, so a Row and a Column are one
// algorithm.

func startEdge(axis int) int { return [2]int{3, 0}[axis] } // left, top
func endEdge(axis int) int   { return [2]int{1, 2}[axis] } // right, bottom

func extent(r *ui.Rect, axis int) *float32 {
	if axis == axisX {
		return &r.Width
	}
	return &r.Height
}

func origin(r *ui.Rect, axis int) *float32 {
	if axis == axisX {
		return &r.X
	}
	return &r.Y
}

// inset is the padding and border on both sides of an axis.
func (n *node) inset(axis int) (start, end float32) {
	border := n.borderWidth
	if n.flags&flagOutline != 0 {
		border = 0 // an outline is drawn, it does not take room
	}
	return n.pad[startEdge(axis)] + border, n.pad[endEdge(axis)] + border
}

func (n *node) marginOn(axis int) (start, end float32) {
	return n.margin[startEdge(axis)], n.margin[endEdge(axis)]
}

// flows reports whether a container lays its children out along axis.
func (n *node) flows(axis int) bool {
	return n.dir == row && axis == axisX || n.dir == column && axis == axisY
}

// leading is the height of one line of a text: its LineHeight, or the font size
// plus seven, the same leading the components use.
func (n *node) leading() float32 {
	if n.lineH > 0 {
		return n.lineH
	}
	return n.fontSize + 7
}

// measure gives every element the size its content wants along axis. Elements
// are stored parent before child, so walking the slice backwards visits children
// first and needs no recursion.
func (f *Frame) measure(axis int) {
	f.wrapCursor = 0
	for i := len(f.nodes) - 1; i >= 0; i-- {
		n := &f.nodes[i]
		if axis == axisX && n.kind != kindContainer {
			f.measureLeaf(n)
		}
		if n.kind == kindText && n.flags&flagWrap != 0 && axis == axisY {
			first := int32(len(f.lines))
			// An empty text has no line at all, as WrapLines gives it none; anything
			// else that fits is one line and needs no breaking.
			if n.text != "" && n.natural <= n.rect.Width-n.padX() && strings.IndexByte(n.text, '\n') < 0 {
				f.lines = append(f.lines, n.text)
			} else {
				f.lines = f.wrapText(f.lines, n)
			}
			n.lineFirst, n.lineCount = first, int32(len(f.lines))-first
			n.pref[axisY] = float32(n.lineCount)*n.leading() + n.padY()
		}
		if n.kind == kindContainer {
			f.measureContainer(n, axis)
		}
		if d := n.size[axis]; d.kind == dimPx {
			n.pref[axis] = d.value
		}
		if axis == axisY {
			f.measureBaseline(n)
		}
		n.raw[axis] = n.pref[axis]
		n.pref[axis] = clamp(n.pref[axis], n.minS[axis], n.maxS[axis])
	}
}

func (f *Frame) measureLeaf(n *node) {
	if !n.hasOwnTheme() {
		f.measureLeafIn(n)
		return
	}
	saved := f.theme
	f.theme = f.ownTheme(n)
	f.measureLeafIn(n)
	f.theme = saved
}

func (f *Frame) measureLeafIn(n *node) {
	switch n.kind {
	case kindText:
		if n.flags&flagMono != 0 {
			cw, ch := f.canvas.MonoCellSize(n.fontSize, n.face)
			n.pref = [2]float32{cw * float32(utf8.RuneCountInString(n.text)), ch}
		} else {
			n.pref = [2]float32{f.canvas.TextWidthWeight(n.text, n.fontSize, n.weight), n.leading()}
		}
		n.natural = n.pref[axisX]
		n.pref[axisX] += n.padX() // a Text takes its Pad and Border like a container
		n.pref[axisY] += n.padY()
	case kindButton:
		button := components.BeginButton(f.canvas, f.theme, ui.ID{}, ui.Rect{}).Size(n.buttonSize)
		if d := n.size[axisY]; d.kind == dimPx {
			button = button.Height(d.value) // an icon button is as wide as it is tall
		}
		button = button.SizeToFit(n.label)
		n.pref = [2]float32{button.BoundsWidth(), button.BoundsHeight()}
		if n.buttonSize >= components.ButtonSizeIcon {
			n.pref[axisX] = n.pref[axisY]
		}
	case kindCheckbox:
		w, h := components.CheckboxHitExtent(f.theme)
		n.pref = [2]float32{w, h}
	case kindSwitch:
		w, h := components.SwitchHitExtent(f.theme, n.flags&flagSmall != 0)
		n.pref = [2]float32{w, h}
	case kindIcon, kindImage:
		n.pref = [2]float32{16, 16}
	case kindSegmented:
		var width float32 = 8
		for _, option := range f.segmented[n.aux].options {
			width += f.canvas.TextWidthWeight(option.Label, f.theme.Metrics.FontSizeM, ui.WeightMedium) + 28
		}
		n.pref = [2]float32{width, f.theme.Metrics.ControlHeight}
	case kindInput, kindInputGroup:
		n.pref = [2]float32{0, f.theme.Metrics.ControlHeight}
		if n.kind == kindInputGroup && f.groups[n.aux].Multiline {
			n.pref[axisY] = 64
		}
	case kindTextarea:
		n.pref = [2]float32{0, 64}
	case kindTable, kindTabs, kindSelect:
		f.measureWidget(n)
	case kindProgress:
		n.pref = [2]float32{progressWidth, progressHeight}
	case kindSpinner:
		n.pref = [2]float32{16, 16}
	case kindSplitter:
		f.measureSplitter(n)
	case kindSlider:
		f.measureSlider(n)
	}
}

func (f *Frame) measureContainer(n *node, axis int) {
	if n.dir == grid {
		f.measureGrid(n, axis)
		return
	}
	var total float32
	count := 0
	for c := n.first; c >= 0; c = f.nodes[c].next {
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
		total += n.gap * float32(count-1)
	}
	is, ie := n.inset(axis)
	n.pref[axis] = total + is + ie
	if axis == axisY && n.flags&flagScroll != 0 {
		// What a scroll area holds is its own business: it is as tall as it is
		// told to be, not as its children. Content says how much they need.
		n.content = total
		if n.size[axisY].kind == dimAuto && n.maxS[axisY] == 0 {
			n.pref[axisY] = is + ie
		}
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
	switch n.kind {
	case kindSlider:
		return (n.natural > 0) == (axis == axisY) // as long as its container, never as thick
	case kindProgress:
		return axis == axisX // a bar is as wide as its column, but a row does not make it tall
	case kindContainer, kindText, kindSpacer, kindSlot, kindInput, kindInputGroup, kindTextarea, kindSegmented, kindTable, kindTabs, kindSelect, kindSplitter:
		return true
	}
	return false
}

// solve sets the size and position along axis of the children of container i,
// then of theirs. The container's own size and position on the axis are known.
func (f *Frame) solve(i int32, axis int) {
	n := &f.nodes[i]
	if n.first < 0 {
		if n.outOfFlow&absChildren != 0 {
			f.solveAbsolute(i, axis)
		}
		return
	}
	is, ie := n.inset(axis)
	start := *origin(&n.rect, axis) + is
	avail := max(*extent(&n.rect, axis)-is-ie, 0)
	switch {
	case n.dir == grid:
		f.solveGrid(n, axis, start, avail)
	case n.flows(axis):
		f.solveFlow(n, axis, start, avail)
	default:
		f.solveAcross(n, axis, start, avail)
	}
	for c := n.first; c >= 0; c = f.nodes[c].next {
		f.solve(c, axis)
	}
	if n.outOfFlow&absChildren != 0 {
		f.solveAbsolute(i, axis)
	}
}

// solveAcross places children on an axis the container does not flow along (the
// width of a Column, the height of a Row, either axis of a Stack).
func (f *Frame) solveAcross(n *node, axis int, start, avail float32) {
	var line float32 // the baseline children are lined up on, from the top
	if axis == axisY && n.dir == row {
		for c := n.first; c >= 0; c = f.nodes[c].next {
			child := &f.nodes[c]
			if child.alignOn(n) == Baseline {
				line = max(line, child.margin[0]+child.baseline)
			}
		}
	}
	for c := n.first; c >= 0; c = f.nodes[c].next {
		child := &f.nodes[c]
		align := child.alignOn(n)
		ms, me := child.marginOn(axis)
		if align == Baseline && axis == axisY && n.dir == row {
			*extent(&child.rect, axis) = clamp(child.pref[axis], child.minS[axis], f.maxOn(child, axis, avail))
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
	d := child.size[axis]
	lo := child.minS[axis]
	switch {
	case d.kind == dimPx:
		return clamp(d.value, lo, hi)
	case d.kind == dimPercent:
		return clamp(avail*d.value/100, lo, hi)
	case d.kind == dimFill, d.kind == dimAuto && (align == Stretch || align == Auto && child.stretches(axis)):
		return clamp(max(avail, 0), lo, hi)
	}
	return clamp(child.pref[axis], lo, hi)
}

// maxOn is the largest size an element may have along axis in a container with avail
// room for it: its MaxWidth or MaxHeight, or the share of avail MaxWidthPercent or
// MaxHeightPercent gives, whichever is smaller. Zero is no limit.
func (f *Frame) maxOn(n *node, axis int, avail float32) float32 {
	hi := n.maxS[axis]
	if pct := f.exts[n.ext].maxPct[axis]; pct > 0 {
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
// or maximum is frozen there and the rest is shared again among the others.
func (f *Frame) solveFlow(n *node, axis int, start, avail float32) {
	count := 0
	var fixed float32
	for c := n.first; c >= 0; c = f.nodes[c].next {
		child := &f.nodes[c]
		d := child.size[axis]
		basis, grow := child.raw[axis], float32(0)
		switch {
		case child.flags&flagBasis != 0:
			basis = child.basis
		case d.kind == dimPercent:
			basis = avail * d.value / 100
		}
		if child.flags&flagGrowSet != 0 {
			grow = child.grow
		}
		if d.kind == dimFill {
			basis = 0
			if child.flags&flagGrowSet == 0 {
				grow = 1
			}
		}
		child.growNow = grow
		switch {
		case child.flags&flagShrinkSet != 0:
			child.shrinkNow = child.shrink
		case axis == axisX && (child.kind == kindText || child.kind == kindContainer),
			axis == axisY && child.flags&flagScroll != 0:
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
		fixed += n.gap * float32(count-1)
	}
	scrolls := axis == axisY && n.flags&flagScroll != 0

	// Resolve flexible lengths as CSS does: share the room by each unfrozen child's
	// base size, clamp, and freeze the children whose clamp went the way the total
	// violation went (all minimums if the sum grew, all maximums if it shrank),
	// then share again among the rest. Every pass starts from the base sizes.
	for c := n.first; c >= 0; c = f.nodes[c].next {
		child := &f.nodes[c]
		child.base = *extent(&child.rect, axis)
		child.frozen = false
	}
	for pass := 0; pass <= count; pass++ {
		var frozenSize, baseSum, growSum, shrinkSum float32
		for c := n.first; c >= 0; c = f.nodes[c].next {
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
		for c := n.first; c >= 0; c = f.nodes[c].next {
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
			bounded := clamp(want, child.minS[axis], f.maxOn(child, axis, avail))
			violation += bounded - want
			*extent(&child.rect, axis) = bounded
			child.bounded = bounded
			child.want = want
		}
		done := true
		for c := n.first; c >= 0; c = f.nodes[c].next {
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
	for c := n.first; c >= 0; c = f.nodes[c].next {
		child := &f.nodes[c]
		size := clamp(*extent(&child.rect, axis), child.minS[axis], f.maxOn(child, axis, avail))
		*extent(&child.rect, axis) = size
		used += size
	}
	used += fixed
	room := avail
	if scrolls {
		room = max(avail, used)
	}
	leftover := room - used
	pos, between := float32(0), n.gap
	switch n.justify {
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
		if state := f.exts[n.ext].scrollState; state != nil && state.pinned() {
			n.scroll = max(used-avail, 0) // at the end of what is there now, not of last frame's
		}
		pos -= n.scroll
	}
	for c := n.first; c >= 0; c = f.nodes[c].next {
		child := &f.nodes[c]
		ms, me := child.marginOn(axis)
		pos += ms
		*origin(&child.rect, axis) = start + pos
		pos += *extent(&child.rect, axis) + me + between
	}
}

// alignOn is how a child sits across its container: its own Self, else the
// container's Items.
func (n *node) alignOn(parent *node) Align {
	if n.self != Auto {
		return n.self
	}
	return parent.items
}

// measureBaseline finds the distance from the top of an element to its first line
// of text, for Baseline alignment. Text sits at its leading's middle with an
// ascent of about four fifths of the font size; a control's label is centered; an
// element with no text has its bottom edge as baseline.
func (f *Frame) measureBaseline(n *node) {
	switch n.kind {
	case kindText:
		top, _ := n.inset(axisY)
		n.baseline = top + (n.leading()-n.fontSize)/2 + n.fontSize*0.8
	case kindButton:
		size := f.theme.Metrics.FontSizeM
		if n.hasOwnTheme() {
			size = f.ownTheme(n).Metrics.FontSizeM
		}
		n.baseline = n.pref[axisY]/2 + size*0.3
	case kindContainer:
		n.baseline = n.pref[axisY]
		is, _ := n.inset(axisY)
		for c := n.first; c >= 0; c = f.nodes[c].next {
			child := &f.nodes[c]
			if child.kind == kindContainer && child.baseline == child.pref[axisY] && child.first < 0 {
				continue
			}
			value := is + child.margin[0] + child.baseline
			if n.dir == row {
				for k := n.first; k >= 0; k = f.nodes[k].next {
					other := &f.nodes[k]
					value = max(value, is+other.margin[0]+other.baseline)
				}
			}
			n.baseline = value
			break
		}
	default:
		n.baseline = n.pref[axisY]
	}
}
