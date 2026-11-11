package ui

import "github.com/vivaui/viva/gfx"

// splitterThickness is how wide the strip is that can be grabbed; the line drawn in
// it is one pixel, three while the pointer is on it.
const (
	splitterThickness = 8
	splitterStep      = 16 // what an arrow key moves a focused splitter
	splitterMin       = 48 // the smallest either pane is, unless Mins says otherwise
)

// SplitterState is the position of one Splitter, kept in the application's state.
// Pos is the size in pixels of the pane before the splitter (its width in a Row, its
// height in a Column); set it before the first frame (a zero Pos is the pane's
// minimum), give the pane that size, and a drag, an arrow key or a window resize
// keeps it inside the limits. A width the user chose is remembered even while the
// window is too narrow for it, and comes back when the window grows; assign Pos
// between paints to choose a new one. The rest is bookkeeping.
type SplitterState struct {
	Pos float32
	// Ratio is the share of the container the first pane takes, kept on every
	// layout while it is non-zero. It lets an application say "the editor gets
	// two thirds" without knowing the window's size, and a resize keeps the
	// proportion. A drag or an arrow key updates Ratio from the new position;
	// set Ratio to zero to hold Pos as pixels instead.
	Ratio float32

	// preferred is the width the user chose, which Pos is clamped from when the
	// window is temporarily too narrow; applied is the Pos the last layout left, so
	// a Pos the program assigned between paints is taken as the new preference.
	preferred    float32
	applied      float32
	hasPreferred bool
	id           gfx.ID
	dragging     bool
	startPointer float32
	startPos     float32
	lo, hi       float32 // the limits as of the last layout
	span         float32 // the container's extent along the split
}

// Dragging reports whether the splitter is being dragged.
func (s *SplitterState) Dragging() bool { return s.dragging }

// Key moves a focused splitter: Left and Up shrink the first pane, Right and Down
// grow it by 16 pixels, Home and End take it to its smallest and largest. A Splitter
// declared in a Page gives it to the page's key routing; a Splitter declared on a
// Frame is given it through Config.OnKey. It ignores every other key and every other
// focus.
func (s *SplitterState) Key(event gfx.KeyEvent) gfx.EventResult {
	if s.id.IsZero() || event.Target != s.id {
		return gfx.EventResult{}
	}
	pos := s.Pos
	switch event.Key {
	case gfx.KeyLeft, gfx.KeyUp:
		pos -= splitterStep
	case gfx.KeyRight, gfx.KeyDown:
		pos += splitterStep
	case gfx.KeyHome:
		pos = s.lo
	case gfx.KeyEnd:
		pos = s.hi
	default:
		return gfx.EventResult{}
	}
	s.Pos = s.limit(pos)
	s.preferred, s.applied = s.Pos, s.Pos
	s.keepRatio()
	return gfx.EventResult{Handled: true, Repaint: true}
}

// splits records that state, a Splitter declared in this paint, takes the keys that
// move its divider while it has the focus.
func (p *Page) splits(state *SplitterState) {
	p.splitters = append(p.splitters, state)
	if p.splitRoute == nil {
		p.keyRoute, p.splitRoute = (*Page).routeKey, (*Page).splitKey
	}
}

// splitKey gives a key to the Splitter that has the focus.
func (p *Page) splitKey(event gfx.KeyEvent) (gfx.EventResult, bool) {
	if event.Release {
		return gfx.EventResult{}, false
	}
	for _, state := range p.splitters {
		if result := state.Key(event); result.Handled {
			return result, true
		}
	}
	return gfx.EventResult{}, false
}

// keepRatio stores the pane's current share after a drag or a key, so a window
// resize keeps the proportion the user chose. A state that holds Pos in pixels
// (Ratio zero) is left alone.
func (s *SplitterState) keepRatio() {
	if s.Ratio <= 0 || s.span <= splitterThickness {
		return
	}
	s.Ratio = min(max(s.Pos/(s.span-splitterThickness), 0), 1)
}

func (s *SplitterState) limit(pos float32) float32 {
	if s.hi > s.lo {
		pos = min(pos, s.hi)
	}
	return max(pos, s.lo)
}

type splitDecl struct {
	state                *SplitterState
	first                int32 // the pane before the splitter, or -1
	minFirst, maxFirst   float32
	minSecond, maxSecond float32
}

// Splitter adds a draggable divider between two siblings of a Row (a vertical bar,
// resized by dragging left and right) or of a Column (a horizontal bar). Declare it
// right after the first pane and before the second, give the first pane state.Pos as
// its size and the second Grow(1):
//
//	f.Row().Fill().Children(func() {
//		f.Column().Width(m.split.Pos).Children(func() { /* left pane */ })
//		f.Splitter(splitID, &m.split).Mins(120, 200)
//		f.Column().Grow(1).Children(func() { /* right pane */ })
//	})
//
// The pointer is captured for the drag, so it may leave the strip, and the cursor
// shows the resize shape over it; with the keyboard, SplitterState.Key moves it. Mins
// sets the least size of the first and the second pane (48 each by default), Maxs the
// greatest. A window too narrow for the width the user chose clamps the panes without
// forgetting it, so they return to it when the window grows. The divider is 8 pixels
// thick, as long as the container is across.
func (f *Frame) Splitter(id gfx.ID, state *SplitterState) Box {
	first := int32(-1)
	if f.parent >= 0 {
		first = f.nodes[f.parent].last
	}
	b := f.box(kindSplitter)
	n := &f.nodes[b.i]
	n.id = id
	n.aux = int32(len(f.widgets.splits))
	f.widgets.splits = append(f.widgets.splits, splitDecl{state: state, first: first, minFirst: splitterMin, minSecond: splitterMin})
	state.adopt()
	state.id = id
	return b
}

// adopt keeps the width the state holds as the preferred one: the width first
// declared, or one the program assigned between paints. reportSplitters only
// clamps Pos, so a window that is briefly too narrow does not replace it.
//
// A splitter that named no size keeps sizing its first pane as the pane asks, as it
// did before: Pos starts at zero, which is "unset" there rather than the minimum it
// means once the splitter owns the pane, so a zero is only taken as a choice after a
// width was chosen. That also keeps a Pos the page clamped, which reportSplitters
// records in applied, from becoming a preferred width of its own.
func (s *SplitterState) adopt() {
	if s.Pos != s.applied && (s.hasPreferred || s.Pos > 0) {
		s.preferred, s.applied, s.hasPreferred = s.Pos, s.Pos, true
	}
}

// Mins sets the least size of the panes either side of a Splitter, along its container.
func (b Box) Mins(first, second float32) Box {
	if n := b.n(); n.kind == kindSplitter {
		d := &b.f.widgets.splits[n.aux]
		d.minFirst, d.minSecond = max(first, 0), max(second, 0)
	}
	return b
}

// Maxs sets the greatest size of the panes either side of a Splitter, along its
// container; it combines with the room there is. A max of zero is no greatest size,
// and the pane's minimum wins over it when the two are said to be smaller than each
// other.
//
//	ui.Splitter(c, &m.split).Mins(120, 200).Maxs(400, 0)
func (b Box) Maxs(first, second float32) Box {
	if n := b.n(); n.kind == kindSplitter {
		d := &b.f.widgets.splits[n.aux]
		d.maxFirst, d.maxSecond = max(first, 0), max(second, 0)
	}
	return b
}

// splitsRow reports whether the splitter's container lays out left to right, so its
// bar is vertical and its pointer coordinate is x.
func (f *Frame) splitsRow(n *node) bool { return n.parent < 0 || f.nodes[n.parent].dir != column }

func (f *Frame) measureSplitter(n *node) {
	if f.splitsRow(n) {
		n.pref = [2]float32{splitterThickness, 0}
	} else {
		n.pref = [2]float32{0, splitterThickness}
	}
}

// splitLimits works out how far the first pane may grow once the page is laid out:
// everything the two panes (and the gaps) have between them, less the bar and the
// second pane's minimum.
func (f *Frame) splitLimits(n *node) (lo, hi, span float32) {
	d := &f.widgets.splits[n.aux]
	lo = d.minFirst
	if d.first < 0 || n.next < 0 {
		return lo, lo, 0
	}
	axis := axisY
	if f.splitsRow(n) {
		axis = axisX
	}
	first, second := &f.nodes[d.first], &f.nodes[n.next]
	span = *origin(&second.rect, axis) + *extent(&second.rect, axis) - *origin(&first.rect, axis)
	gaps := span - *extent(&first.rect, axis) - *extent(&n.rect, axis) - *extent(&second.rect, axis)
	lo, hi = splitBounds(d.minFirst, d.maxFirst, d.minSecond, d.maxSecond, span, gaps, *extent(&n.rect, axis))
	return lo, hi, span
}

// splitBounds is the least and greatest size of the first pane for a span the two
// panes and the splitter share, with the gaps between them: what Mins and Maxs say,
// bounded by the room.
func splitBounds(minFirst, maxFirst, minSecond, maxSecond, span, gaps, thickness float32) (lo, hi float32) {
	lo, hi = max(minFirst, 0), span-gaps-thickness-max(minSecond, 0)
	if maxSecond > 0 {
		lo = max(lo, span-gaps-thickness-maxSecond)
	}
	if maxFirst > 0 {
		hi = min(hi, maxFirst)
	}
	return lo, max(hi, lo)
}

// placeSplitters gives every splitter its first pane's size on this axis before the
// container shares its room: the share from Ratio, or the width the user preferred.
// Doing it in the layout pass, rather than after it, means the frame that shows the
// pane already has the size the window allows, and a resize needs no second frame.
func (f *Frame) placeSplitters(n *node, axis int, avail float32) {
	if n.first < 0 {
		return
	}
	count := 0
	for c := n.first; c >= 0; c = f.nodes[c].next {
		count++
	}
	gaps := n.gap * float32(max(count-1, 0))
	for c := n.first; c >= 0; c = f.nodes[c].next {
		bar := &f.nodes[c]
		if bar.kind != kindSplitter {
			continue
		}
		d := &f.widgets.splits[bar.aux]
		state := d.state
		if d.first < 0 {
			continue
		}
		thickness := bar.pref[axis]
		if thickness <= 0 {
			thickness = splitterThickness
		}
		lo, hi := splitBounds(d.minFirst, d.maxFirst, d.minSecond, d.maxSecond, avail, gaps, thickness)
		var pos float32
		switch {
		case state.Ratio > 0:
			pos = state.Ratio * (avail - gaps - thickness)
		case state.hasPreferred:
			// The width the user chose is what the pane goes back to when the room
			// allows it again; Pos itself may have been clamped last frame.
			pos = state.preferred
		default:
			continue
		}
		pos = min(max(pos, lo), hi)
		first := &f.nodes[d.first]
		first.raw[axis], first.pref[axis] = pos, pos
		state.Pos, state.applied = pos, pos
		state.lo, state.hi, state.span = lo, hi, avail
	}
}

// reportSplitters keeps every position inside its limits after a layout (a window that
// got smaller, a pane that was given a different size) and asks for one more frame when
// it had to move one, so the panes follow.
func (f *Frame) reportSplitters() {
	for i := range f.nodes {
		n := &f.nodes[i]
		if n.kind != kindSplitter {
			continue
		}
		s := f.widgets.splits[n.aux].state
		lo, hi, span := f.splitLimits(n)
		s.lo, s.hi, s.span = lo, hi, span
		if pos := s.limit(s.Pos); pos != s.Pos {
			// Pos follows the room the layout found; preferred stays as the user left
			// it, so placeSplitRatios gives the pane that width when there is room.
			s.Pos = pos
			if f.canvas != nil {
				f.canvas.RequestFrame()
			}
		}
		s.applied = s.Pos
	}
}

func (f *Frame) paintSplitter(n *node) {
	canvas := f.canvas
	d := &f.widgets.splits[n.aux]
	s := d.state
	row := f.splitsRow(n)
	r := n.rect

	if event, ok := canvas.PointerOn(n.id); ok {
		pointer := event.X
		if !row {
			pointer = event.Y
		}
		switch {
		case event.Kind == gfx.PointerDown && event.Button <= gfx.PointerButtonPrimary:
			s.dragging, s.startPointer, s.startPos = true, pointer, s.Pos
			canvas.Consume()
		case event.Kind == gfx.PointerMove && s.dragging:
			s.lo, s.hi, s.span = f.splitLimits(n)
			s.Pos = s.limit(s.startPos + pointer - s.startPointer)
			s.preferred, s.applied = s.Pos, s.Pos
			s.keepRatio()
			canvas.Consume()
		case event.Kind == gfx.PointerUp && s.dragging:
			s.dragging = false
			canvas.Consume()
		}
	}

	theme := f.theme
	pressed := canvas.Pressed() == n.id
	hovered := canvas.Hovered() == n.id
	line, thickness := theme.Border, float32(1)
	switch {
	case pressed:
		line, thickness = theme.Primary, 3
	case hovered:
		line, thickness = theme.Ring, 3
	}
	var bar Rect
	cursor := gfx.PointerCursorResizeVertical
	if row {
		bar = Rect{X: r.X + (r.Width-thickness)/2, Y: r.Y, Width: thickness, Height: r.Height}
		cursor = gfx.PointerCursorResizeHorizontal
	} else {
		bar = Rect{X: r.X, Y: r.Y + (r.Height-thickness)/2, Width: r.Width, Height: thickness}
	}
	if canvas.FocusVisible() == n.id {
		ring := theme.Ring
		ring.Alpha *= 0.5
		canvas.Rounded(Rect{X: bar.X - 1, Y: bar.Y - 1, Width: bar.Width + 2, Height: bar.Height + 2}, 2, ring)
	}
	if thickness > 1 {
		canvas.Rounded(bar, 1, line)
	} else {
		canvas.Fill(bar, line)
	}
	label := n.label
	if label == "" {
		label = "Resize panes"
	}
	canvas.HitWithOptions(n.id, r, label, gfx.HitOptions{Role: gfx.RoleSlider, Cursor: cursor})
}
