package ui

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

func paintProgress(f *Frame, n *Node) {
	r := n.Rect
	if n.Natural >= 0 {
		components.Progress(f.Surface, f.Palette, r, n.Natural)
		paintProgressHit(f, n, r, n.Natural)
		return
	}
	canvas := f.Surface
	if r.Width <= 0 {
		return
	}
	if r.Height <= 0 {
		r.Height = ProgressHeight
	}
	canvas.Rounded(r, r.Height/2, f.Palette.Muted)
	segment := r.Width * IndeterminateShare
	x := r.X + (r.Width-segment)/2
	if !canvas.ReduceMotion() {
		phase := float32(canvas.Now()%IndeterminatePeriod) / float32(IndeterminatePeriod)
		x = r.X - segment + phase*(r.Width+segment)
		if f.Visible(n) {
			canvas.RequestFrame()
		}
	}
	canvas.Clip(r)
	canvas.Rounded(Rect{X: x, Y: r.Y, Width: segment, Height: r.Height}, r.Height/2, f.Palette.Primary)
	canvas.Unclip()
	paintProgressHit(f, n, r, -1)
}

// paintProgressHit declares the accessible progress bar: a read-only range for a
// determinate bar, no number while it is indeterminate, and never a keyboard stop
// or a press target. A bar is exposed when it is named: Label is what assistive
// technology announces, and an unnamed bar would be an empty node that also took
// the pointer from whatever holds it.
func paintProgressHit(f *Frame, n *Node, r Rect, fraction float32) {
	if n.ID.IsZero() || n.Label == "" {
		return
	}
	options := gfx.HitOptions{Role: gfx.RoleProgressBar, SkipFocus: true, Range: gfx.ValueRange{Min: 0, Max: 100}}
	if fraction >= 0 {
		options.Range.Set = true
		options.Range.Now = float64(min(max(fraction, 0), 1)) * 100
	}
	f.Surface.HitWithOptions(n.ID, r, n.Label, options)
}

func paintSpinner(f *Frame, n *Node) {
	canvas := f.Surface
	phase := components.SpinnerPhase(0)
	if !canvas.ReduceMotion() {
		phase = components.SpinnerPhase(canvas.Now() / SpinnerStep % 8)
		if f.Visible(n) {
			canvas.RequestFrameIn(SpinnerStep)
		}
	}
	components.PaintSpinner(canvas, f.Palette, n.Rect, phase)
}
