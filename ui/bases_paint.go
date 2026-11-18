package ui

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// CheckboxBase adds a checkbox without a look: the size of the shadcn box, centered, that
// reports Clicked and is a checkbox in the given state to assistive technology, with the
// pointer region reaching beyond the box as the shadcn control's does. Give it a
// Background, Border and Radius, and draw the mark with Draw:
//
//	look := components.CheckboxRecipe(f.Theme(), state)
//	f.CheckboxBase(id, on).Radius(look.Radius).Background(look.Fill).Border(1, look.Border).
//		Draw(func(d *ui.DrawContext) { components.PaintCheckboxMark(d.Canvas, d.Rect, d.State, look.Mark) })
//
// Label names it. A mixed box is HitWith the gfx.HitOptions it needs, and State.Mixed is the
// caller's to say, as it is for Invalid.
func (f *frame) CheckboxBase(ID gfx.ID, checked bool) Box {
	theme := f.Palette
	side := components.CheckboxExtent(theme)
	scale := theme.Metrics.ControlHeight / 32
	b := f.Row().Center().Size(side, side).HitOutset(8*scale, 12*scale, 8*scale, 12*scale)
	if checked {
		f.Nodes[b.i].Flags |= flagChecked
	}
	return b.HitWith(ID, "checkbox", gfx.HitOptions{Role: gfx.RoleCheckbox, Checked: checked})
}

// SwitchBase adds a switch without a look: the size of the shadcn track, that reports
// Clicked and is a switch in the given state to assistive technology, with the pointer
// region reaching beyond the track as the shadcn control's does. The track and the thumb
// are the caller's, drawn with Draw; gfx.Canvas.Animate gives the thumb's travel, and
// components.SwitchRecipe the colors at a point of it. components.SwitchExtent is the
// size of the small switch.
func (f *frame) SwitchBase(ID gfx.ID, on bool) Box {
	width, height := components.SwitchExtent(f.Palette, false)
	b := f.Row().Center().Size(width, height).HitOutset(8, 12, 8, 12)
	if on {
		f.Nodes[b.i].Flags |= flagChecked
	}
	return b.HitWith(ID, "switch", gfx.HitOptions{Role: gfx.RoleSwitch, Checked: on})
}
