package base

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
	if n := b.N(); n.Kind == kindCheckbox {
		if mixed {
			n.Flags |= flagMixed
		} else {
			n.Flags &^= flagMixed
		}
	}
	return b
}

func (b Box) Caption(Text string) Box {
	n := b.N()
	if n.Kind != kindCheckbox && n.Kind != kindSwitch {
		return b
	}
	n.Text = Text
	b.f.captionFn = (*Frame).paintCaption
	if n.Label == "" {
		n.Label = Text
	}
	return b
}

// captionGap is the room between a control and its Caption (`gap-2`).
const CaptionGap = 8

func (f *Frame) CaptionSize(n *Node) float32 {
	if n.FontSize > 0 {
		return n.FontSize
	}
	return f.Palette.Metrics.FontSizeM
}

// paintCaption paints a Checkbox's or Switch's Caption after the control's bounds and
// makes it a label of the control, so a click on it is a click on the control.
// paintCaptionView is paintCaption for a control painter that reads a
// ControlView rather than the engine's element.
func (f *Frame) PaintCaptionView(v ControlView, control Rect) {
	size := v.Font
	if size <= 0 {
		size = f.Palette.Metrics.FontSizeM
	}
	line := size + 7
	x := control.X + control.Width + CaptionGap
	r := Rect{X: x, Y: v.Rect.Y + (v.Rect.Height-line)/2, Width: max(v.Rect.X+v.Rect.Width-x, 0), Height: line}
	color := fade(f.Palette.Foreground, f.Opacity)
	if v.HasColor {
		color = v.Color
	}
	if v.Disabled {
		color.Alpha *= 0.5
	}
	f.Surface.Label(v.Text, r, gfx.TextStyle{Size: size, Weight: gfx.WeightMedium, Color: color})
	f.Surface.HitLabel(v.ID, r, v.Text, v.Disabled)
}

func (f *Frame) paintCaption(n *Node, control Rect) {
	size := f.CaptionSize(n)
	line := size + 7
	x := control.X + control.Width + CaptionGap
	r := Rect{X: x, Y: n.Rect.Y + (n.Rect.Height-line)/2, Width: max(n.Rect.X+n.Rect.Width-x, 0), Height: line}
	disabled := n.Flags&flagDisabled != 0
	color := fade(f.Palette.Foreground, f.Opacity)
	if n.Flags&flagHasColor != 0 {
		color = n.Color // faded with the element already
	}
	if disabled {
		color.Alpha *= 0.5
	}
	f.Surface.Label(n.Text, r, gfx.TextStyle{Size: size, Weight: gfx.WeightMedium, Color: color})
	f.Surface.HitLabel(n.ID, r, n.Text, disabled)
}
