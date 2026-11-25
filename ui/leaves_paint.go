package ui

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

func paintProgress(f *Frame, n *Node) {
	r := n.Rect
	palette := f.Palette
	switch n.Status {
	case StatusSuccess:
		palette.Primary = palette.Success
	case StatusWarning:
		palette.Primary = palette.WarningColor()
	case StatusDanger:
		palette.Primary = palette.Danger
	}
	if n.Circle {
		paintProgressRing(f, n, palette)
		return
	}
	if n.Natural >= 0 {
		components.Progress(f.Surface, palette, r, n.Natural)
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
	canvas.Rounded(Rect{X: x, Y: r.Y, Width: segment, Height: r.Height}, r.Height/2, palette.Primary)
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

// paintProgressRing draws a circular Progress: a muted ring and over it the arc of the
// fraction, clockwise from twelve o'clock with round ends. An indeterminate one is an
// arc of a quarter turn that keeps going round, and holds still when the system asks
// for reduced motion.
func paintProgressRing(f *Frame, n *Node, palette Theme) {
	canvas := f.Surface
	r := n.Rect
	side := min(r.Width, r.Height)
	if side <= 0 {
		return
	}
	box := Rect{X: r.X + (r.Width-side)/2, Y: r.Y + (r.Height-side)/2, Width: side, Height: side}
	width := min(max(side*0.09, 3), 10)
	radius := (side - width) / 2
	builder := canvas.PathBuilder()
	builder.Arc(side/2, side/2, radius, -90, 360)
	canvas.DrawPath(builder.Path(side, side), box, gfx.PathStyle{Stroke: palette.Muted, StrokeWidth: width})
	filled := min(n.Natural, 1)
	start, sweep, fraction := float32(-90), filled*360, filled
	if n.Natural < 0 {
		sweep, fraction = 90, -1
		if !canvas.ReduceMotion() {
			start += 360 * float32(canvas.Now()%IndeterminatePeriod) / float32(IndeterminatePeriod)
			if f.Visible(n) {
				canvas.RequestFrame()
			}
		}
	}
	if sweep > 0 {
		builder = canvas.PathBuilder()
		builder.Arc(side/2, side/2, radius, start, min(sweep, 359.9))
		canvas.DrawPath(builder.Path(side, side), box, gfx.PathStyle{Stroke: palette.Primary, StrokeWidth: width, Cap: gfx.CapRound})
	}
	paintProgressHit(f, n, box, fraction)
}
