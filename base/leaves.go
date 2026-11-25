package base

import (
	"time"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
)

// progressHeight and progressWidth are what a Progress is when it is given neither.
const (
	ProgressHeight = 6
	ProgressWidth  = 160
	// ProgressRing is the side of a circular Progress given no size.
	ProgressRing = 64
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
// as for any other state. ID names the bar to assistive technology, which reads it
// as a progress bar and never focuses or presses it; Label names it. A zero ID
// declares no accessible node.
//
//	f.Progress(uploadID, m.done/m.total).Label("Uploading").Height(8)
//	f.Progress(gfx.ID{}, 0).Indeterminate()
func (f *Frame) Progress(ID gfx.ID, fraction float32) Box {
	b := f.Box(kindProgress)
	n := &f.Nodes[b.i]
	n.ID = ID
	n.Natural = max(fraction, 0)
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

// Circle draws a Progress as a ring that fills clockwise from twelve o'clock, 64 across
// unless given a Width and Height (it is as large as the smaller of the two), and an
// Indeterminate one as an arc that keeps turning. It keeps the status color and the
// accessible progress bar of the bar.
//
//	ui.Progress(c, m.done/m.total).Circle().Size(80, 80)
func (b Box) Circle() Box {
	n := b.N()
	if n.Kind != kindProgress {
		panic("ui: Circle is for a Progress")
	}
	n.Circle = true
	return b
}

// Segments draws a Progress bar as n separate segments with a gap between them, the first
// ones filled for the fraction (a fraction of a segment fills it from one half), for work
// that comes in steps: "3 of 5 stages". n of one or less is the ordinary bar, and a ring
// keeps being a ring.
//
//	ui.Progress(c, float32(m.stage)/float32(m.stages)).Segments(m.stages)
func (b Box) Segments(n int) Box {
	node := b.N()
	if node.Kind != kindProgress {
		panic("ui: Segments is for a Progress")
	}
	node.Segments = uint8(min(max(n, 0), 255))
	return b
}

// Status colors the fill of a Progress by the outcome it reports: success, warning or
// danger take the theme's color for it in place of the primary one, and the default
// keeps the primary. It is the same vocabulary as an alert or a tag.
//
//	ui.Progress(c, m.done/m.total).Status(ui.StatusSuccess)
func (b Box) Status(status style.Status) Box {
	n := b.N()
	if n.Kind != kindProgress {
		panic("ui: Status is for a Progress")
	}
	n.Status = status
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
