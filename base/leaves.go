package base

import (
	"time"
)

// progressHeight and progressWidth are what a Progress is when it is given neither.
const (
	ProgressHeight = 6
	ProgressWidth  = 160
	// An indeterminate bar's moving segment is this much of the track and takes
	// indeterminatePeriod to cross it.
	IndeterminateShare  = 0.4
	IndeterminatePeriod = 1600 * time.Millisecond
	// A spinner turns one spoke every spinnerStep.
	SpinnerStep = 100 * time.Millisecond
)

// Progress adds a bar filled to fraction (0 to 1; anything else is clamped) in the
// theme's look, painted by components.Progress. It is 6 tall and as wide as its
// container in a Column, 160 wide in a Row, unless given a Width, Height or Grow.
// Indeterminate turns it into a bar whose segment keeps sliding, for work of unknown
// length. A determinate bar never asks for a frame: change the fraction and repaint,
// as for any other state.
//
//	f.Progress(m.done / m.total).Height(8)
//	f.Progress(0).Indeterminate()
func (f *Frame) Progress(fraction float32) Box {
	b := f.Box(kindProgress)
	f.Nodes[b.i].Natural = max(fraction, 0)
	if fraction != fraction {
		f.Nodes[b.i].Natural = 0
	}
	return b
}

// Indeterminate makes a Progress a sliding segment instead of a fill. It asks for
// the next frame only while it is on screen, and holds still when the system asks
// for reduced motion.
func (b Box) Indeterminate() Box {
	if n := b.N(); n.Kind == kindProgress {
		n.Natural = -1
	}
	return b
}

// Spinner adds the theme's 16 pixel loading indicator, turning clockwise one step
// every 100 ms. It asks for another frame only while it is on screen (and none when
// the system asks for reduced motion, which leaves it still); a spinner inside a
// collapsed section, a tab that is not shown or a scrolled-away list costs nothing.
// Declare it only while the work runs.
//
//	if m.loading { f.Spinner() }
func (f *Frame) Spinner() Box { return f.Box(kindSpinner) }

// visible reports whether any of element n is inside the frame and every clip it sits
// in: the elements that animate ask for frames only then.
func (f *Frame) Visible(n *Node) bool {
	r := intersect(n.Rect, f.bounds)
	for p := n.Parent; p >= 0 && r.Width > 0 && r.Height > 0; p = f.Nodes[p].Parent {
		if f.Nodes[p].Flags&flagClip != 0 {
			r = intersect(r, f.Nodes[p].Rect)
		}
	}
	return r.Width > 0 && r.Height > 0
}

func intersect(a, b Rect) Rect {
	x, y := max(a.X, b.X), max(a.Y, b.Y)
	return Rect{X: x, Y: y, Width: min(a.X+a.Width, b.X+b.Width) - x, Height: min(a.Y+a.Height, b.Y+b.Height) - y}
}
