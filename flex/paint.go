package flex

import (
	"math"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/ui"
)

func (f *Frame) paint(i int32) {
	n := &f.nodes[i]
	if n.flags&flagDialog != 0 {
		f.paintDialog(n)
		return
	}
	r := n.rect
	canvas := f.canvas
	extra := &f.exts[n.ext]
	style := f.resolve(n, extra)
	// What the element is drawn with fades with it and with what it is inside; the
	// controls inside it are painted with a theme faded the same way.
	parentOpacity, parentTheme := f.opacity, ui.Theme{}
	f.opacity *= style.opacity
	if style.opacity < 1 {
		parentTheme = f.theme // a theme is large: copied only for what is faded
		f.theme = f.theme.WithOpacity(style.opacity)
	}
	f.paintRing(r, n.radius, &style) // outside the rectangle: before a Clip of the element's own
	if n.flags&flagClip != 0 {
		canvas.Clip(r)
		defer canvas.Unclip()
	}

	hasHit := n.flags&flagHasHit != 0 && !n.id.IsZero()
	background, border := fade(style.background, f.opacity), fade(style.border, f.opacity)
	if style.shadow != ui.ShadowNone {
		canvas.ElevateFaded(r, n.radius, style.shadow, f.opacity)
	}
	switch {
	case n.borderWidth > 0 && extra.gradient == 0 && f.opacity >= 1:
		canvas.RoundedBorder(r, n.radius, n.borderWidth, border, background)
	default:
		// RoundedBorder paints the border color as a solid backing under the fill, which a
		// gradient or a faded fill would show through. For those the fill, then the
		// gradient, go down whole, as CSS's background reaches under its border, and the
		// border is only a line over them. An element that is neither keeps RoundedBorder,
		// so what an application painted before looks as it did.
		if background.Alpha > 0 {
			if n.radius > 0 {
				canvas.Rounded(r, n.radius, background)
			} else {
				canvas.Fill(r, background)
			}
		}
		if extra.gradient != 0 {
			f.paintGradient(n, extra)
		}
		if n.borderWidth > 0 {
			canvas.RoundedOutline(r, n.radius, n.borderWidth, border)
		}
	}
	if extra.moves {
		canvas.MoveRegion(r)
	}
	if hasHit && (n.kind == kindContainer || n.kind == kindText || n.kind == kindIcon || n.kind == kindSlot) {
		h := extra.hitOut
		hit := ui.Rect{X: r.X - h[3], Y: r.Y - h[0], Width: r.Width + h[1] + h[3], Height: r.Height + h[0] + h[2]}
		canvas.HitWithOptions(n.id, hit, n.label, extra.hit)
	}

	// A Text and an Icon draw with their color, which is the one the looks give.
	color := n.color
	n.color = fade(style.color, f.opacity)
	if n.hasOwnTheme() {
		saved := f.theme
		f.theme = f.ownTheme(n)
		f.paintControl(n, r)
		f.theme = saved
	} else {
		f.paintControl(n, r)
	}
	n.color = color

	for c := n.first; c >= 0; c = f.nodes[c].next {
		f.paint(c)
	}
	if n.outOfFlow&absChildren != 0 {
		f.paintAbsolute(i)
	}
	f.opacity = parentOpacity
	if style.opacity < 1 {
		f.theme = parentTheme
	}
}

// paintControl paints what the element is: its text, icon, or control. The
// background and border of its box are already down.
func (f *Frame) paintControl(n *node, r ui.Rect) {
	canvas := f.canvas
	extra := &f.exts[n.ext]
	switch n.kind {
	case kindText:
		f.paintText(n)
	case kindIcon:
		canvas.DrawIconTransformed(extra.icon, ui.Rect{X: r.X, Y: r.Y, Width: n.pref[axisX], Height: n.pref[axisY]}, n.color, extra.iconTransform)
	case kindImage:
		canvas.DrawBitmap(f.images[n.aux], r)
	case kindButton:
		f.paintButton(n)
	case kindCheckbox:
		side := components.CheckboxExtent(f.theme)
		bounds := ui.Rect{X: r.X + (r.Width-side)/2, Y: r.Y + (r.Height-side)/2, Width: side, Height: side}
		checkbox := components.BeginCheckbox(canvas, f.theme, n.id, bounds, n.flags&flagChecked != 0)
		if n.label != "" {
			checkbox = checkbox.Label(n.label)
		}
		if n.flags&flagDisabled != 0 {
			checkbox = checkbox.Disabled()
		}
		checkbox.Paint()
	case kindInput:
		components.PaintInput(canvas, f.theme, r, f.inputs[n.aux])
	case kindInputGroup:
		components.PaintInputGroup(canvas, f.theme, r, f.groups[n.aux])
	case kindTextarea:
		components.PaintTextarea(canvas, f.theme, r, f.textareas[n.aux])
	case kindSegmented:
		choice := f.segmented[n.aux]
		components.BeginSegmentedControl(canvas, f.theme, r, choice.options, choice.selected).Paint()
	case kindTable, kindTabs, kindSelect:
		f.paintWidget(n)
	case kindProgress:
		f.paintProgress(n)
	case kindSpinner:
		f.paintSpinner(n)
	case kindSplitter:
		f.paintSplitter(n)
	case kindSlider:
		f.paintSlider(n)
	case kindSwitch:
		small := n.flags&flagSmall != 0
		width, height := components.SwitchExtent(f.theme, small)
		bounds := ui.Rect{X: r.X + (r.Width-width)/2, Y: r.Y + (r.Height-height)/2, Width: width, Height: height}
		control := components.BeginSwitch(canvas, f.theme, n.id, bounds, n.flags&flagChecked != 0)
		if small {
			control = control.Small()
		}
		if n.label != "" {
			control = control.Label(n.label)
		}
		if n.flags&flagDisabled != 0 {
			control = control.Disabled()
		}
		control.Paint()
	}
}

func (f *Frame) paintText(n *node) {
	if n.text == "" {
		return
	}
	left, right := n.inset(axisX)
	top, bottom := n.inset(axisY)
	r := ui.Rect{X: n.rect.X + left, Y: n.rect.Y + top, Width: max(n.rect.Width-left-right, 0), Height: max(n.rect.Height-top-bottom, 0)}
	style := ui.TextStyle{Size: n.fontSize, Weight: n.weight, Color: n.color, Mono: n.flags&flagMono != 0, MonoFace: n.face, Direction: n.textDirection}
	alignment := ui.AlignStart
	switch {
	case n.flags&flagTextCenter != 0:
		alignment = ui.AlignCenter
	case n.textEnd:
		alignment = ui.AlignEnd
	}
	if n.flags&flagWrap != 0 {
		// A paragraph has one direction: each of its lines follows it, not the first strong
		// character of the line.
		if style.Direction == ui.DirectionAuto {
			style.Direction = ui.TextDirectionOf(n.text)
		}
		lh := n.leading()
		for k := int32(0); k < n.lineCount; k++ {
			bounds := ui.Rect{X: r.X, Y: r.Y + float32(k)*lh, Width: r.Width, Height: lh}
			f.canvas.LabelAligned(f.lines[n.lineFirst+k], bounds, style, alignment)
		}
		return
	}
	height := max(n.pref[axisY]-top-bottom, 0)
	bounds := ui.Rect{X: r.X, Y: r.Y + max((r.Height-height)/2, 0), Width: r.Width, Height: height}
	clipped := r.Width < n.natural
	if clipped {
		f.canvas.Clip(r)
	}
	f.canvas.LabelAligned(n.text, bounds, style, alignment)
	if clipped {
		f.canvas.Unclip()
	}
}

func (f *Frame) paintButton(n *node) {
	r := n.rect
	button := components.BeginButton(f.canvas, f.theme, n.id, ui.Rect{X: r.X, Y: r.Y, Width: r.Width}).Size(n.buttonSize)
	// A button is as tall as its size says unless the element was given more or less
	// room: a Height, or a Grow along a column. It then fills that room.
	if h := r.Height; h > 0 && math.Abs(float64(h-button.BoundsHeight())) > 0.01 {
		button = button.Height(h)
	}
	if n.flags&flagDisabled != 0 {
		button = button.Disabled()
	}
	switch n.variant {
	case components.ButtonSecondary:
		button.Secondary(n.label)
	case components.ButtonOutline:
		button.Outline(n.label)
	case components.ButtonGhost:
		button.Ghost(n.label)
	case components.ButtonLink:
		button.Link(n.label)
	case components.ButtonDestructive:
		button.Destructive(n.label)
	case components.ButtonDestructiveOutline:
		button.DestructiveOutline(n.label)
	default:
		button.Default(n.label)
	}
}
