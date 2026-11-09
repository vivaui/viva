package ui

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// The Base of a control is the control without its look: what it does, as an element
// that takes the calls any element takes. A Base is focusable, is clicked by the pointer
// and the keyboard, and reaches assistive technology as the control it is; what it looks
// like is the element's own Background, Border, Radius, Shadow, Ring and Hover, Pressed
// and Focus looks, the controls it is built from as children, and Draw and DrawOver for
// what those cannot say. The control with the theme's look is the Base dressed by a
// recipe (see the components package: TextEntryRecipe, CheckboxRecipe, SwitchRecipe), so
// restyling a control never means copying its behavior.
//
//	f.ButtonBase(id, "Add").Pad(8).Radius(6).Background(c.Muted).
//		Hover(ui.Look{}.Background(c.Hover)).Children(func() { f.Icon(plus); f.Text("Add") })
//
// The Bases are InputBase and TextareaBase (frame.go), SliderBase (slider.go), SelectBase (select.go), ButtonBase,
// CheckboxBase and SwitchBase, and the items the composite controls are made of: ToggleBase,
// TabBase, RadioBase (the option of a segmented control or a radio group), OptionBase (the row
// of a list) and ComboboxBase (the trigger of a select). A strip of tabs or a segmented
// control is a Row of these, with the arrow keys of components.TabsKeyDown and
// SegmentedControlKeyDown (a test composes both end to end); Tabs and Segmented are the same
// with the theme's look. A Select has a state machine of its own (the open list, the focus
// scope, Escape, type-ahead), so it has a whole-control Base, SelectBase.
// A container with Hit is the Base of any other clickable thing.

// ButtonBase adds a button without a look: a row centering its children that takes the
// keyboard focus, and reports Clicked for the pointer, Enter and Space. label is its
// accessible name. It is as large as its children and its Pad; Button is the same with
// the theme's look.
func (f *Frame) ButtonBase(id gfx.ID, label string) Box {
	b := f.Row().Center()
	return b.HitWith(id, label, gfx.HitOptions{Role: gfx.RoleButton})
}

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
func (f *Frame) CheckboxBase(id gfx.ID, checked bool) Box {
	theme := f.theme
	side := components.CheckboxExtent(theme)
	scale := theme.Metrics.ControlHeight / 32
	b := f.Row().Center().Size(side, side).HitOutset(8*scale, 12*scale, 8*scale, 12*scale)
	if checked {
		f.nodes[b.i].flags |= flagChecked
	}
	return b.HitWith(id, "checkbox", gfx.HitOptions{Role: gfx.RoleCheckbox, Checked: checked})
}

// SwitchBase adds a switch without a look: the size of the shadcn track, that reports
// Clicked and is a switch in the given state to assistive technology, with the pointer
// region reaching beyond the track as the shadcn control's does. The track and the thumb
// are the caller's, drawn with Draw; gfx.Canvas.Animate gives the thumb's travel, and
// components.SwitchRecipe the colors at a point of it. components.SwitchExtent is the
// size of the small switch.
func (f *Frame) SwitchBase(id gfx.ID, on bool) Box {
	width, height := components.SwitchExtent(f.theme, false)
	b := f.Row().Center().Size(width, height).HitOutset(8, 12, 8, 12)
	if on {
		f.nodes[b.i].flags |= flagChecked
	}
	return b.HitWith(id, "switch", gfx.HitOptions{Role: gfx.RoleSwitch, Checked: on})
}

// ToggleBase adds a two-state button without a look, as ButtonBase is a button: it reports
// Clicked, and to assistive technology it is a button that is pressed or not.
func (f *Frame) ToggleBase(id gfx.ID, label string, pressed bool) Box {
	b := f.Row().Center()
	if pressed {
		f.nodes[b.i].flags |= flagChecked | flagSelected
	}
	return b.HitWith(id, label, gfx.HitOptions{Role: gfx.RoleButton, Checked: pressed, Selected: pressed})
}

// TabBase adds one tab without a look: it reports Clicked, and to assistive technology it
// is a tab that is selected or not. A strip is a Row (or a Column) of them; the arrow keys
// between them are components.TabsKeyDown's.
func (f *Frame) TabBase(id gfx.ID, label string, selected bool) Box {
	b := f.Row().Center()
	if selected {
		f.nodes[b.i].flags |= flagSelected
	}
	return b.HitWith(id, label, gfx.HitOptions{Role: gfx.RoleTab, Value: label, Selected: selected})
}

// RadioBase adds one option of a segmented control or a radio group without a look: it
// reports Clicked, and to assistive technology it is a radio button that is chosen or not.
func (f *Frame) RadioBase(id gfx.ID, label string, chosen bool) Box {
	b := f.Row().Center()
	if chosen {
		f.nodes[b.i].flags |= flagChecked | flagSelected
	}
	return b.HitWith(id, label, gfx.HitOptions{Role: gfx.RoleRadio, Value: label, Checked: chosen, Selected: chosen})
}

// OptionBase adds one row of a list without a look, such as the options of a select's
// popup: it reports Clicked, and to assistive technology it is an option that is selected or
// not.
func (f *Frame) OptionBase(id gfx.ID, label string, selected bool) Box {
	b := f.Row()
	if selected {
		f.nodes[b.i].flags |= flagSelected
	}
	return b.HitWith(id, label, gfx.HitOptions{Role: gfx.RoleOption, Value: label, Selected: selected})
}

// ComboboxBase adds the trigger of a select without a look: it reports Clicked, and to
// assistive technology it is a combobox that is expanded or not and has value as its value.
// The list it opens is a Popup of OptionBase rows.
func (f *Frame) ComboboxBase(id gfx.ID, label, value string, open bool) Box {
	b := f.Row().Center()
	if open {
		f.nodes[b.i].flags |= flagOpen
	}
	return b.HitWith(id, label, gfx.HitOptions{Role: gfx.RoleCombobox, Value: value, Expanded: open, Expandable: true})
}
