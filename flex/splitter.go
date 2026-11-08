package flex

import "github.com/vivaui/viva/ui"

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
// keeps it inside the limits. The rest is bookkeeping.
type SplitterState struct {
	Pos float32
	// Ratio is the share of the container the first pane takes, kept on every
	// layout while it is non-zero. It lets an application say "the editor gets
	// two thirds" without knowing the window's size, and a resize keeps the
	// proportion. A drag or an arrow key updates Ratio from the new position;
	// set Ratio to zero to hold Pos as pixels instead.
	Ratio        float32
	id           ui.ID
	dragging     bool
	startPointer float32
	startPos     float32
	lo, hi       float32 // the limits as of the last layout
	span         float32 // the container's extent along the split
}

// Dragging reports whether the splitter is being dragged.
func (s *SplitterState) Dragging() bool { return s.dragging }

// Key moves a focused splitter: Left and Up shrink the first pane, Right and Down
// grow it by 16 pixels, Home and End take it to its smallest and largest. Give it to
// Config.OnKey (or call it from yours); it ignores every other key and every other
// focus.
func (s *SplitterState) Key(event ui.KeyEvent) ui.EventResult {
	if s.id.IsZero() || event.Target != s.id {
		return ui.EventResult{}
	}
	pos := s.Pos
	switch event.Key {
	case ui.KeyLeft, ui.KeyUp:
		pos -= splitterStep
	case ui.KeyRight, ui.KeyDown:
		pos += splitterStep
	case ui.KeyHome:
		pos = s.lo
	case ui.KeyEnd:
		pos = s.hi
	default:
		return ui.EventResult{}
	}
	s.Pos = s.limit(pos)
	s.keepRatio()
	return ui.EventResult{Handled: true, Repaint: true}
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
	state               *SplitterState
	first               int32 // the pane before the splitter, or -1
	minFirst, minSecond float32
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
// sets the least size of the first and the second pane (48 each by default). The
// divider is 8 pixels thick, as long as the container is across.
func (f *Frame) Splitter(id ui.ID, state *SplitterState) Box {
	first := int32(-1)
	if f.parent >= 0 {
		first = f.nodes[f.parent].last
	}
	b := f.box(kindSplitter)
	n := &f.nodes[b.i]
	n.id = id
	n.aux = int32(len(f.widgets.splits))
	f.widgets.splits = append(f.widgets.splits, splitDecl{state: state, first: first, minFirst: splitterMin, minSecond: splitterMin})
	state.id = id
	return b
}

// Mins sets the least size of the panes either side of a Splitter, along its container.
func (b Box) Mins(first, second float32) Box {
	if n := b.n(); n.kind == kindSplitter {
		d := &b.f.widgets.splits[n.aux]
		d.minFirst, d.minSecond = max(first, 0), max(second, 0)
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
	hi = span - *extent(&n.rect, axis) - gaps - d.minSecond
	return lo, max(hi, lo), span
}

// placeSplitRatios gives every splitter with a Ratio its first pane's size on
// this axis before the container shares its room. Doing it in the layout pass,
// rather than after it, means the first frame that shows the pane already has
// the share and a resize keeps it without a second frame.
func (f *Frame) placeSplitRatios(n *node, axis int, avail float32) {
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
		if state.Ratio <= 0 || d.first < 0 {
			continue
		}
		thickness := bar.pref[axis]
		if thickness <= 0 {
			thickness = splitterThickness
		}
		lo := d.minFirst
		hi := max(avail-gaps-thickness-d.minSecond, lo)
		pos := min(max(state.Ratio*(avail-gaps-thickness), lo), hi)
		first := &f.nodes[d.first]
		first.raw[axis], first.pref[axis] = pos, pos
		state.Pos = pos
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
			s.Pos = pos
			if f.canvas != nil {
				f.canvas.RequestFrame()
			}
		}
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
		case event.Kind == ui.PointerDown && event.Button <= ui.PointerButtonPrimary:
			s.dragging, s.startPointer, s.startPos = true, pointer, s.Pos
			canvas.Consume()
		case event.Kind == ui.PointerMove && s.dragging:
			s.lo, s.hi, s.span = f.splitLimits(n)
			s.Pos = s.limit(s.startPos + pointer - s.startPointer)
			s.keepRatio()
			canvas.Consume()
		case event.Kind == ui.PointerUp && s.dragging:
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
	var bar ui.Rect
	cursor := ui.PointerCursorResizeVertical
	if row {
		bar = ui.Rect{X: r.X + (r.Width-thickness)/2, Y: r.Y, Width: thickness, Height: r.Height}
		cursor = ui.PointerCursorResizeHorizontal
	} else {
		bar = ui.Rect{X: r.X, Y: r.Y + (r.Height-thickness)/2, Width: r.Width, Height: thickness}
	}
	if canvas.FocusVisible() == n.id {
		ring := theme.Ring
		ring.Alpha *= 0.5
		canvas.Rounded(ui.Rect{X: bar.X - 1, Y: bar.Y - 1, Width: bar.Width + 2, Height: bar.Height + 2}, 2, ring)
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
	canvas.HitWithOptions(n.id, r, label, ui.HitOptions{Role: ui.RoleSlider, Cursor: cursor})
}
