package ui

import (
	"time"

	"github.com/vivaui/viva/components"
)

// progressHeight and progressWidth are what a Progress is when it is given neither.
const (
	progressHeight = 6
	progressWidth  = 160
	// An indeterminate bar's moving segment is this much of the track and takes
	// indeterminatePeriod to cross it.
	indeterminateShare  = 0.4
	indeterminatePeriod = 1600 * time.Millisecond
	// A spinner turns one spoke every spinnerStep.
	spinnerStep = 100 * time.Millisecond
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
func (f *frame) Progress(fraction float32) Box {
	b := f.box(kindProgress)
	f.nodes[b.i].natural = max(fraction, 0)
	if fraction != fraction {
		f.nodes[b.i].natural = 0
	}
	return b
}

// Indeterminate makes a Progress a sliding segment instead of a fill. It asks for
// the next frame only while it is on screen, and holds still when the system asks
// for reduced motion.
func (b Box) Indeterminate() Box {
	if n := b.n(); n.kind == kindProgress {
		n.natural = -1
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
func (f *frame) Spinner() Box { return f.box(kindSpinner) }

// visible reports whether any of element n is inside the frame and every clip it sits
// in: the elements that animate ask for frames only then.
func (f *frame) visible(n *node) bool {
	r := intersect(n.rect, f.bounds)
	for p := n.parent; p >= 0 && r.Width > 0 && r.Height > 0; p = f.nodes[p].parent {
		if f.nodes[p].flags&flagClip != 0 {
			r = intersect(r, f.nodes[p].rect)
		}
	}
	return r.Width > 0 && r.Height > 0
}

func intersect(a, b Rect) Rect {
	x, y := max(a.X, b.X), max(a.Y, b.Y)
	return Rect{X: x, Y: y, Width: min(a.X+a.Width, b.X+b.Width) - x, Height: min(a.Y+a.Height, b.Y+b.Height) - y}
}

func (f *frame) paintProgress(n *node) {
	r := n.rect
	if n.natural >= 0 {
		components.Progress(f.canvas, f.theme, r, n.natural)
		return
	}
	canvas := f.canvas
	if r.Width <= 0 {
		return
	}
	if r.Height <= 0 {
		r.Height = progressHeight
	}
	canvas.Rounded(r, r.Height/2, f.theme.Muted)
	segment := r.Width * indeterminateShare
	x := r.X + (r.Width-segment)/2
	if !canvas.ReduceMotion() {
		phase := float32(canvas.Now()%indeterminatePeriod) / float32(indeterminatePeriod)
		x = r.X - segment + phase*(r.Width+segment)
		if f.visible(n) {
			canvas.RequestFrame()
		}
	}
	canvas.Clip(r)
	canvas.Rounded(Rect{X: x, Y: r.Y, Width: segment, Height: r.Height}, r.Height/2, f.theme.Primary)
	canvas.Unclip()
}

func (f *frame) paintSpinner(n *node) {
	canvas := f.canvas
	phase := components.SpinnerPhase(0)
	if !canvas.ReduceMotion() {
		phase = components.SpinnerPhase(canvas.Now() / spinnerStep % 8)
		if f.visible(n) {
			canvas.RequestFrameIn(spinnerStep)
		}
	}
	components.PaintSpinner(canvas, f.theme, n.rect, phase)
}
