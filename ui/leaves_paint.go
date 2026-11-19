package ui

import (
	"github.com/vivaui/viva/components"
)

func paintProgress(f *Frame, n *Node) {
	r := n.Rect
	if n.Natural >= 0 {
		components.Progress(f.Surface, f.Palette, r, n.Natural)
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
