package base

import "github.com/vivaui/viva/gfx"

// splitterThickness is how wide the strip is that can be grabbed; the line drawn in
// it is one pixel, three while the pointer is on it.
const (
	splitterThickness = 8
	splitterStep      = 16 // what an arrow key moves a focused splitter, unless StepFraction says
	splitterMin       = 48 // the smallest either pane is, unless Mins says otherwise
)

// SplitterState is the position of one Splitter, kept in the application's state.
// Pos is the size in pixels of the pane before the splitter (its width in a Row, its
// height in a Column); set it before the first frame (a zero Pos is the pane's
// minimum), give the pane that size, and a drag, an arrow key or a window resize
// keeps it inside the limits. The rest is bookkeeping.
type SplitterState struct {
	// StepFraction, when positive, makes an arrow key move the splitter that
	// share of the room it has (the component's 5% step) instead of 16 pixels.
	StepFraction float32
	Pos          float32
	// Ratio is the share of the container the first pane takes, kept on every
	// layout while it is non-zero. It lets an application say "the editor gets
	// two thirds" without knowing the window's size, and a resize keeps the
	// proportion. A drag or an arrow key updates Ratio from the new position;
	// set Ratio to zero to hold Pos as pixels instead.
	Ratio        float32
	ID           gfx.ID
	dragging     bool
	startPointer float32
	startPos     float32
	lo, hi       float32 // the limits as of the last layout
	Span         float32 // the container's extent along the split
	thickness    float32 // the room the divider took in that layout
}

// Dragging reports whether the splitter is being dragged.
func (s *SplitterState) Dragging() bool { return s.dragging }

// Key moves a focused splitter: Left and Up shrink the first pane, Right and Down
// grow it by 16 pixels, Home and End take it to its smallest and largest. Give it to
// Config.OnKey (or call it from yours); it ignores every other key and every other
// focus.
func (s *SplitterState) Key(event gfx.KeyEvent) gfx.EventResult {
	if s.ID.IsZero() || event.Target != s.ID {
		return gfx.EventResult{}
	}
	step := float32(splitterStep)
	if s.StepFraction > 0 {
		thickness := s.thickness
		if thickness <= 0 {
			thickness = splitterThickness
		}
		step = max(s.Span-thickness, 1) * s.StepFraction
	}
	pos := s.Pos
	switch event.Key {
	case gfx.KeyLeft, gfx.KeyUp:
		pos -= step
	case gfx.KeyRight, gfx.KeyDown:
		pos += step
	case gfx.KeyHome:
		pos = s.lo
	case gfx.KeyEnd:
		pos = s.hi
	default:
		return gfx.EventResult{}
	}
	s.Pos = s.limit(pos)
	s.keepRatio()
	return gfx.EventResult{Handled: true, Repaint: true}
}

// keepRatio stores the pane's current share after a drag or a key, so a window
// resize keeps the proportion the user chose. A state that holds Pos in pixels
// (Ratio zero) is left alone.
func (s *SplitterState) keepRatio() {
	thickness := s.thickness
	if thickness <= 0 {
		thickness = splitterThickness
	}
	if s.Ratio <= 0 || s.Span <= thickness {
		return
	}
	s.Ratio = min(max(s.Pos/(s.Span-thickness), 0), 1)
}

func (s *SplitterState) limit(pos float32) float32 {
	if s.hi > s.lo {
		pos = min(pos, s.hi)
	}
	return max(pos, s.lo)
}

type SplitDecl struct {
	state               *SplitterState
	First               int32 // the pane before the splitter, or -1
	minFirst, minSecond float32
	// thickness is the room the divider takes in the layout; zero is the
	// default strip. The pointer's strip is never narrower than that default.
	thickness float32
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
func (f *Frame) Splitter(ID gfx.ID, state *SplitterState) Box {
	first := int32(-1)
	if f.Parent >= 0 {
		first = f.Nodes[f.Parent].Last
	}
	b := f.Box(kindSplitter)
	n := &f.Nodes[b.i]
	n.ID = ID
	n.Aux = int32(len(WidgetsOf(f).splits))
	WidgetsOf(f).splits = append(WidgetsOf(f).splits, SplitDecl{state: state, First: first, minFirst: splitterMin, minSecond: splitterMin})
	state.ID = ID
	return b
}

// SplitterThickness sets how much room the divider takes in the layout, for a
// split whose panes should meet with a hairline between them. The pointer's strip
// stays at least as wide as the default (8), centred on the line, so a one-pixel
// divider is still easy to grab.
func (b Box) SplitterThickness(thickness float32) Box {
	if n := b.N(); n.Kind == kindSplitter {
		WidgetsOf(b.f).splits[n.Aux].thickness = max(thickness, 0.5)
	}
	return b
}

// Mins sets the least size of the panes either side of a Splitter, along its container.
func (b Box) Mins(First, second float32) Box {
	if n := b.N(); n.Kind == kindSplitter {
		d := &WidgetsOf(b.f).splits[n.Aux]
		d.minFirst, d.minSecond = max(First, 0), max(second, 0)
	}
	return b
}

// splitsRow reports whether the splitter's container lays out left to right, so its
// bar is vertical and its pointer coordinate is x.
func (f *Frame) splitsRow(n *Node) bool { return n.Parent < 0 || f.Nodes[n.Parent].Dir != column }

func (f *Frame) MeasureSplitter(n *Node) {
	thickness := WidgetsOf(f).splits[n.Aux].thickness
	if thickness <= 0 {
		thickness = splitterThickness
	}
	if f.splitsRow(n) {
		n.Pref = [2]float32{thickness, 0}
	} else {
		n.Pref = [2]float32{0, thickness}
	}
}

// splitLimits works out how far the first pane may grow once the page is laid out:
// everything the two panes (and the gaps) have between them, less the bar and the
// second pane's minimum.
func (f *Frame) splitLimits(n *Node) (lo, hi, Span float32) {
	d := &WidgetsOf(f).splits[n.Aux]
	lo = d.minFirst
	if d.First < 0 || n.Next < 0 {
		return lo, lo, 0
	}
	axis := AxisY
	if f.splitsRow(n) {
		axis = AxisX
	}
	first, second := &f.Nodes[d.First], &f.Nodes[n.Next]
	Span = *origin(&second.Rect, axis) + *extent(&second.Rect, axis) - *origin(&first.Rect, axis)
	gaps := Span - *extent(&first.Rect, axis) - *extent(&n.Rect, axis) - *extent(&second.Rect, axis)
	hi = Span - *extent(&n.Rect, axis) - gaps - d.minSecond
	return lo, max(hi, lo), Span
}

// placeSplitRatios gives every splitter with a Ratio its first pane's size on
// this axis before the container shares its room. Doing it in the layout pass,
// rather than after it, means the first frame that shows the pane already has
// the share and a resize keeps it without a second frame.
func (f *Frame) placeSplitRatios(n *Node, axis int, avail float32) {
	if n.First < 0 {
		return
	}
	count := 0
	for c := n.First; c >= 0; c = f.Nodes[c].Next {
		count++
	}
	gaps := n.Gap * float32(max(count-1, 0))
	for c := n.First; c >= 0; c = f.Nodes[c].Next {
		bar := &f.Nodes[c]
		if bar.Kind != kindSplitter {
			continue
		}
		d := &WidgetsOf(f).splits[bar.Aux]
		state := d.state
		if state.Ratio <= 0 || d.First < 0 {
			continue
		}
		thickness := bar.Pref[axis]
		if thickness <= 0 {
			thickness = splitterThickness
		}
		lo := d.minFirst
		hi := max(avail-gaps-thickness-d.minSecond, lo)
		pos := min(max(state.Ratio*(avail-gaps-thickness), lo), hi)
		first := &f.Nodes[d.First]
		first.Raw[axis], first.Pref[axis] = pos, pos
		state.Pos = pos
		state.thickness = thickness
		state.lo, state.hi, state.Span = lo, hi, avail
	}
}

// reportSplitters keeps every position inside its limits after a layout (a window that
// got smaller, a pane that was given a different size) and asks for one more frame when
// it had to move one, so the panes follow.
func (f *Frame) reportSplitters() {
	for i := range f.Nodes {
		n := &f.Nodes[i]
		if n.Kind != kindSplitter {
			continue
		}
		s := WidgetsOf(f).splits[n.Aux].state
		lo, hi, span := f.splitLimits(n)
		s.lo, s.hi, s.Span = lo, hi, span
		axis := AxisY
		if f.splitsRow(n) {
			axis = AxisX
		}
		s.thickness = *extent(&n.Rect, axis)
		if pos := s.limit(s.Pos); pos != s.Pos {
			s.Pos = pos
			if f.Surface != nil {
				f.Surface.RequestFrame()
			}
		}
	}
}

func (f *Frame) PaintSplitter(n *Node) {
	canvas := f.Surface
	d := &WidgetsOf(f).splits[n.Aux]
	s := d.state
	row := f.splitsRow(n)
	r := n.Rect

	if event, ok := canvas.PointerOn(n.ID); ok {
		pointer := event.X
		if !row {
			pointer = event.Y
		}
		switch {
		case event.Kind == gfx.PointerDown && event.Button <= gfx.PointerButtonPrimary:
			s.dragging, s.startPointer, s.startPos = true, pointer, s.Pos
			canvas.Consume()
		case event.Kind == gfx.PointerMove && s.dragging:
			s.lo, s.hi, s.Span = f.splitLimits(n)
			s.Pos = s.limit(s.startPos + pointer - s.startPointer)
			s.keepRatio()
			canvas.Consume()
		case event.Kind == gfx.PointerUp && s.dragging:
			s.dragging = false
			canvas.Consume()
		}
	}

	theme := f.Palette
	pressed := canvas.Pressed() == n.ID
	hovered := canvas.Hovered() == n.ID
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
	if canvas.FocusVisible() == n.ID {
		ring := theme.Ring
		ring.Alpha *= 0.5
		canvas.Rounded(Rect{X: bar.X - 1, Y: bar.Y - 1, Width: bar.Width + 2, Height: bar.Height + 2}, 2, ring)
	}
	if thickness > 1 {
		canvas.Rounded(bar, 1, line)
	} else {
		canvas.Fill(bar, line)
	}
	label := n.Label
	if label == "" {
		label = "Resize panes"
	}
	// The pointer's strip is never narrower than the default, so a hairline
	// divider between the panes is still easy to grab.
	hit := r
	if row {
		if pad := (max(d.thickness, splitterThickness) - r.Width) / 2; pad > 0 {
			hit.X, hit.Width = hit.X-pad, hit.Width+2*pad
		}
	} else {
		if pad := (max(d.thickness, splitterThickness) - r.Height) / 2; pad > 0 {
			hit.Y, hit.Height = hit.Y-pad, hit.Height+2*pad
		}
	}
	// HitWith gives the splitter the application's own semantics: a split
	// pane is a separator in the accessibility tree, and a caller that takes
	// HitWith's options to say so keeps everything else a splitter's.
	options := f.Exts[n.Ext].hit
	if options.Role == 0 {
		options.Role = gfx.RoleSlider
	}
	if options.Cursor == 0 {
		options.Cursor = cursor
	}
	canvas.HitWithOptions(n.ID, hit, label, options)
}
