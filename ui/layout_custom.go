package ui

// Layout measures and places the in-flow children of a CustomLayout container.
// The page runs it in four phases, in the same order it lays out any container:
//
//   - MeasureWidth: how wide the container wants its content, before its width is
//     known. The children have their preferred widths in the context.
//   - PlaceWidth: divide that width between the children and set where each starts.
//     The page lays out each child at the width it was given, then measures heights.
//   - MeasureHeight: how tall the container wants its content. The width it was
//     given is ContentWidth(), and Height(i) is the height of child i at the width
//     PlaceWidth gave it. A layout that balances columns by height chooses them here
//     or in PlaceHeight, from those heights.
//   - PlaceHeight: divide that height between the children and set where each starts.
//     It may move a child horizontally too: the page shifts the child's subtree with
//     it and places its popups again from the new anchor.
//
// A layout is kept in the model: it is called on every paint, after the page has
// declared its children, and must not declare, paint or keep the context. Put
// scratch slices on the layout itself; the paint path allocates nothing once warm.
// The layout owns its children's sizes and positions: Width, Height, Fill, Grow,
// Shrink and Margin on a child are not applied by the page there. Absolute children
// belong to the page, not the layout, and are placed after it. A CustomLayout is not
// declared in a Memo. For scrolling or virtualization wrap it in Scroll, List or
// Tree rather than implementing them in the layout.
//
//	type masonry struct {
//		columns int
//		// scratch, sized once
//		columnHeight []float32
//		columnOf     []int
//	}
//
//	func (m *masonry) MeasureWidth(*ui.LayoutContext) float32 { return 0 } // FillWidth
//	func (m *masonry) PlaceWidth(ctx *ui.LayoutContext, bounds ui.Rect) { ... }
//	func (m *masonry) MeasureHeight(ctx *ui.LayoutContext) float32 { ... }
//	func (m *masonry) PlaceHeight(ctx *ui.LayoutContext, bounds ui.Rect) { ... }
//
//	ui.CustomLayout(c, &m.masonry).FillWidth().Children(func() { ... })
type Layout interface {
	MeasureWidth(ctx *LayoutContext) float32
	PlaceWidth(ctx *LayoutContext, bounds Rect)
	MeasureHeight(ctx *LayoutContext) float32
	PlaceHeight(ctx *LayoutContext, bounds Rect)
}

type layoutPhase uint8

const (
	phaseMeasureWidth layoutPhase = iota
	phasePlaceWidth
	phaseMeasureHeight
	phasePlaceHeight
)

// LayoutContext is what a Layout measures and places its children with. It is valid
// only for the call that received it; do not keep it. Children are the container's
// in-flow children, in declaration order.
type LayoutContext struct {
	frame    *Frame
	children []int32
	beforeX  []float32 // the X of each child before PlaceHeight
	width    float32   // content width, known from PlaceWidth on
	phase    layoutPhase
}

// Count returns how many in-flow children the container has.
func (c *LayoutContext) Count() int { return len(c.children) }

// Width returns the preferred width of child i, before the container places it.
func (c *LayoutContext) Width(i int) float32 {
	return c.frame.nodes[c.child(i)].pref[axisX]
}

// Height returns the preferred height of child i: after PlaceWidth, at the width
// that was given to it.
func (c *LayoutContext) Height(i int) float32 {
	return c.frame.nodes[c.child(i)].pref[axisY]
}

// SetWidth gives child i its width. It is a PlaceWidth call.
func (c *LayoutContext) SetWidth(i int, v float32) {
	c.must(c.phase == phasePlaceWidth, "SetWidth is for PlaceWidth")
	c.frame.nodes[c.child(i)].rect.Width = max(v, 0)
}

// SetX places the left edge of child i. It is a PlaceWidth call, or a PlaceHeight
// call for a layout that balances columns after measuring heights: the page moves
// the child's subtree with it, and places its popups again from the new anchor.
func (c *LayoutContext) SetX(i int, v float32) {
	c.must(c.phase == phasePlaceWidth || c.phase == phasePlaceHeight, "SetX is for PlaceWidth or PlaceHeight")
	c.frame.nodes[c.child(i)].rect.X = v
}

// SetHeight gives child i its height. It is a PlaceHeight call.
func (c *LayoutContext) SetHeight(i int, v float32) {
	c.must(c.phase == phasePlaceHeight, "SetHeight is for PlaceHeight")
	c.frame.nodes[c.child(i)].rect.Height = max(v, 0)
}

// SetY places the top edge of child i. It is a PlaceHeight call.
func (c *LayoutContext) SetY(i int, v float32) {
	c.must(c.phase == phasePlaceHeight, "SetY is for PlaceHeight")
	c.frame.nodes[c.child(i)].rect.Y = v
}

// ContentWidth is the width the container was given, from PlaceWidth on; it is
// what MeasureHeight divides between the children.
func (c *LayoutContext) ContentWidth() float32 { return c.width }

func (c *LayoutContext) child(i int) int32 {
	if i < 0 || i >= len(c.children) {
		panic("ui: LayoutContext child index out of range")
	}
	return c.children[i]
}

func (c *LayoutContext) must(ok bool, message string) {
	if !ok {
		panic("ui: " + message)
	}
}

// begin prepares the context for one phase of container n.
func (c *LayoutContext) begin(f *Frame, n *node, phase layoutPhase) {
	c.frame, c.phase, c.width = f, phase, 0
	c.children = c.children[:0]
	for i := n.first; i >= 0; i = f.nodes[i].next {
		c.children = append(c.children, i)
	}
}

// measureCustom gives a CustomLayout container its preferred size on axis.
func (f *Frame) measureCustom(n *node, axis int) {
	layout := f.exts[n.ext].layout
	if layout == nil {
		panic("ui: a CustomLayout container has no Layout")
	}
	ctx := &f.layoutCtx
	if axis == axisX {
		ctx.begin(f, n, phaseMeasureWidth)
		n.pref[axisX] = max(layout.MeasureWidth(ctx), 0) + n.padX()
		return
	}
	ctx.begin(f, n, phaseMeasureHeight)
	ctx.width = max(n.rect.Width-n.padX(), 0)
	n.pref[axisY] = max(layout.MeasureHeight(ctx), 0) + n.padY()
}

// solveCustom gives a CustomLayout's children their extent and origin on axis,
// then lets the page lay out each child there.
func (f *Frame) solveCustom(n *node, axis int, start, avail float32) {
	layout := f.exts[n.ext].layout
	if layout == nil {
		panic("ui: a CustomLayout container has no Layout")
	}
	ctx := &f.layoutCtx
	if axis == axisX {
		ctx.begin(f, n, phasePlaceWidth)
		for _, i := range ctx.children {
			child := &f.nodes[i]
			child.rect.Width = clamp(child.pref[axisX], child.minS[axisX], f.maxOn(child, axisX, avail))
			child.rect.X = start
		}
		ctx.width = avail
		layout.PlaceWidth(ctx, Rect{X: start, Width: avail})
		return
	}
	ctx.begin(f, n, phasePlaceHeight)
	ctx.beforeX = ctx.beforeX[:0]
	for _, i := range ctx.children {
		child := &f.nodes[i]
		ctx.beforeX = append(ctx.beforeX, child.rect.X)
		child.rect.Height = clamp(child.pref[axisY], child.minS[axisY], f.maxOn(child, axisY, avail))
		child.rect.Y = start
	}
	is, _ := n.inset(axisX)
	ctx.width = max(n.rect.Width-n.padX(), 0)
	layout.PlaceHeight(ctx, Rect{X: n.rect.X + is, Y: start, Width: ctx.width, Height: avail})
	for index, i := range ctx.children {
		if delta := f.nodes[i].rect.X - ctx.beforeX[index]; delta != 0 {
			f.shiftSubtreeX(i, delta)
		}
	}
}

// shiftSubtreeX moves the descendants of child i by delta after the child itself
// was placed. The nodes of a subtree are contiguous: the first node after it that
// is not its descendant ends the range. Popups are not descendants; End places
// them again from their anchors' final rectangles.
func (f *Frame) shiftSubtreeX(i int32, delta float32) {
	if delta == 0 {
		return
	}
	start := int(i) + 1
	end := start
	for end < len(f.nodes) {
		p := f.nodes[end].parent
		if p < 0 {
			break
		}
		for p > i {
			p = f.nodes[p].parent
		}
		if p != i {
			break
		}
		end++
	}
	for j := start; j < end; j++ {
		f.nodes[j].rect.X += delta
	}
	f.shiftedX = true
}
