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
//
// Mixed paints a Checkbox as neither on nor off, a dash in a filled box: the select-all of a
// list of which only some rows are chosen. Declare it from the rows' state each paint; a
// press still flips the bound value, so the application decides what the next state is.
// It has no effect on any other element.
func (b Box) Mixed(mixed bool) Box {
	if n := b.n(); n.Kind == kindCheckbox {
		if mixed {
			n.Flags |= flagMixed
		} else {
			n.Flags &^= flagMixed
		}
	}
	return b
}

func (b Box) Caption(text string) Box {
	n := b.n()
	if n.Kind != kindCheckbox && n.Kind != kindSwitch {
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
	if n.FontSize > 0 {
		return n.FontSize
	}
	return f.theme.Metrics.FontSizeM
}

// paintCaption paints a Checkbox's or Switch's Caption after the control's bounds and
// makes it a label of the control, so a click on it is a click on the control.
// paintCaptionView is paintCaption for a control painter that reads a
// ControlView rather than the engine's element.
func (f *frame) paintCaptionView(v ControlView, control Rect) {
	size := v.Font
	if size <= 0 {
		size = f.theme.Metrics.FontSizeM
	}
	line := size + 7
	x := control.X + control.Width + captionGap
	r := Rect{X: x, Y: v.Rect.Y + (v.Rect.Height-line)/2, Width: max(v.Rect.X+v.Rect.Width-x, 0), Height: line}
	color := fade(f.theme.Foreground, f.Opacity)
	if v.HasColor {
		color = v.Color
	}
	if v.Disabled {
		color.Alpha *= 0.5
	}
	f.Surface.Label(v.Text, r, gfx.TextStyle{Size: size, Weight: gfx.WeightMedium, Color: color})
	f.Surface.HitLabel(v.ID, r, v.Text, v.Disabled)
}

func (f *frame) paintCaption(n *node, control Rect) {
	size := f.captionSize(n)
	line := size + 7
	x := control.X + control.Width + captionGap
	r := Rect{X: x, Y: n.rect.Y + (n.rect.Height-line)/2, Width: max(n.rect.X+n.rect.Width-x, 0), Height: line}
	disabled := n.Flags&flagDisabled != 0
	color := fade(f.theme.Foreground, f.Opacity)
	if n.Flags&flagHasColor != 0 {
		color = n.Color // faded with the element already
	}
	if disabled {
		color.Alpha *= 0.5
	}
	f.Surface.Label(n.text, r, gfx.TextStyle{Size: size, Weight: gfx.WeightMedium, Color: color})
	f.Surface.HitLabel(n.ID, r, n.text, disabled)
}
