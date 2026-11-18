package ui

import (
	"github.com/vivaui/viva/components"
)

func (f *frame) paintProgress(n *node) {
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
		r.Height = progressHeight
	}
	canvas.Rounded(r, r.Height/2, f.Palette.Muted)
	segment := r.Width * indeterminateShare
	x := r.X + (r.Width-segment)/2
	if !canvas.ReduceMotion() {
		phase := float32(canvas.Now()%indeterminatePeriod) / float32(indeterminatePeriod)
		x = r.X - segment + phase*(r.Width+segment)
		if f.Visible(n) {
			canvas.RequestFrame()
		}
	}
	canvas.Clip(r)
	canvas.Rounded(Rect{X: x, Y: r.Y, Width: segment, Height: r.Height}, r.Height/2, f.Palette.Primary)
	canvas.Unclip()
}

func (f *frame) paintSpinner(n *node) {
	canvas := f.Surface
	phase := components.SpinnerPhase(0)
	if !canvas.ReduceMotion() {
		phase = components.SpinnerPhase(canvas.Now() / spinnerStep % 8)
		if f.Visible(n) {
			canvas.RequestFrameIn(spinnerStep)
		}
	}
	components.PaintSpinner(canvas, f.Palette, n.Rect, phase)
}
