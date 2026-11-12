package ui

import (
	"github.com/vivaui/viva/gfx"
)

// Caption shows text beside a Checkbox or Switch, after the control, as shadcn puts a
// Label next to one: a click on the text toggles the control (it reports through the
// control's Clicked), it is the control's name for accessibility unless Label gives
// another, and it dims with Disabled. The element is the control and its text: it starts
// at its left edge instead of centering the control in the room it is given. On other
// elements it does nothing.
//
//	f.Checkbox(syncID, m.sync).Caption("Sync over SSH")
func (b Box) Caption(text string) Box {
	n := b.n()
	if n.kind != kindCheckbox && n.kind != kindSwitch {
		return b
	}
	n.text = text
	b.f.captionFn = (*frame).paintCaption
	if n.label == "" {
		n.label = text
	}
	return b
}

// captionGap is the room between a control and its Caption (`gap-2`).
const captionGap = 8

func (f *frame) captionSize(n *node) float32 {
	if n.fontSize > 0 {
		return n.fontSize
	}
	return f.theme.Metrics.FontSizeM
}

// paintCaption paints a Checkbox's or Switch's Caption after the control's bounds and
// makes it a label of the control, so a click on it is a click on the control.
func (f *frame) paintCaption(n *node, control Rect) {
	size := f.captionSize(n)
	line := size + 7
	x := control.X + control.Width + captionGap
	r := Rect{X: x, Y: n.rect.Y + (n.rect.Height-line)/2, Width: max(n.rect.X+n.rect.Width-x, 0), Height: line}
	disabled := n.flags&flagDisabled != 0
	color := fade(f.theme.Foreground, f.opacity)
	if n.flags&flagHasColor != 0 {
		color = n.color // faded with the element already
	}
	if disabled {
		color.Alpha *= 0.5
	}
	f.canvas.Label(n.text, r, gfx.TextStyle{Size: size, Weight: gfx.WeightMedium, Color: color})
	f.canvas.HitLabel(n.id, r, n.text, disabled)
}
