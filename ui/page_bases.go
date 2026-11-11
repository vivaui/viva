package ui

import (
	"unsafe"
)

// The Bases of a page are its controls without their look: what each does, as an element
// that takes the calls any element takes, for a design of your own. A Base is focusable,
// is clicked by the pointer and the keyboard, and reaches assistive technology as the
// control it is; its Background, Border, Radius, Ring, Hover, Pressed and Focus looks, the
// elements it is built from as children, and Draw and DrawOver are yours. The control
// with the theme's look is the same Base dressed, so a restyled control never copies a
// behavior, and a component library of your own has the keys, the focus, the value
// binding and the popups of the ones that ship.
//
//	on := ui.SwitchBase(c, &m.dark, "Dark mode")
//	on.Radius(999).Background(track(on.State())).Draw(m.paintThumb)
//
// They are declared like their dressed forms: a bound value is written in the paint
// that delivers the click, and Changed says it did.

// ButtonBase is Button without a look: a row centering its children that is clicked by
// the pointer, Enter and Space. label is its accessible name.
func ButtonBase(c C, label string) Box {
	id := c.page.controlID(pkButton)
	b := c.f.ButtonBase(id, label)
	c.page.guard(b, id, label)
	return b
}

// CheckboxBase is Checkbox without a look: the box, which flips *value and is drawn by
// you. label is its accessible name.
func CheckboxBase(c C, value *bool, label string) Box {
	b := c.f.CheckboxBase(c.page.boundID(pkCheckbox, unsafe.Pointer(value)), *value).Label(label)
	c.f.bindBool(b, value)
	return b
}

// SwitchBase is Switch without a look: the track, which flips *value and is drawn by you.
func SwitchBase(c C, value *bool, label string) Box {
	b := c.f.SwitchBase(c.page.boundID(pkSwitch, unsafe.Pointer(value)), *value).Label(label)
	c.f.bindBool(b, value)
	return b
}

// ToggleBase is a two-state button without a look: it flips *value, and to assistive
// technology it is a button that is pressed or not.
func ToggleBase(c C, value *bool, label string) Box {
	b := c.f.ToggleBase(c.page.boundID(pkToggle, unsafe.Pointer(value)), label, *value)
	c.f.bindBool(b, value)
	return b
}

// RadioBase is one option of a radio group or a segmented control without a look: a click
// sets *value to option, and it is a radio button that is chosen while *value is option.
// Name it with c.Key when the options are reordered.
func RadioBase[T comparable](c C, value *T, option T, label string) Box {
	id := c.page.controlID(pkRadio)
	b := c.f.RadioBase(id, label, *value == option)
	choose(c, b, value, option)
	return b
}

// TabBase is one tab of a strip without a look: a click sets *value to tab, and it is a
// tab that is selected while *value is tab. Between tabs the arrow keys are a
// FocusGroup's (FocusHorizontal), or components.TabsKeyDown's.
func TabBase[T comparable](c C, value *T, tab T, label string) Box {
	id := c.page.controlID(pkTab)
	b := c.f.TabBase(id, label, *value == tab)
	choose(c, b, value, tab)
	return b
}

// choose writes option into *value when b is clicked, and records whether it changed.
func choose[T comparable](c C, b Box, value *T, option T) {
	flag := b.n().flags
	changed := c.f.canvas != nil && flag&(flagVeto|flagDisabled) == 0 && c.f.canvas.Clicked(b.n().id) && *value != option
	if changed {
		*value = option
	}
	c.f.bindDone(b, changed)
}

// SliderBase is Slider without a look: it sets state.Value from the pointer across it
// and the keys, and is drawn by you; state.Fraction says how far along the range it is.
func SliderBase(c C, state *SliderState) Box {
	c.page.slides(state)
	return c.f.SliderBase(c.page.boundID(pkSlider, unsafe.Pointer(state)), state)
}

// SelectBase is Select without a look: the state, the trigger, the open list and its
// keys (the arrows highlight, Enter chooses, type-ahead, Escape) are Select's, and
// nothing of it is painted. Draw on it paints the trigger in Rect; its Popup is the open
// list's element, whose Draw paints the panel, with Part(i) the rectangle of option i.
//
//	sel := ui.SelectBase(c, &m.size, sizes, nil).Draw(m.paintTrigger)
//	if list, ok := sel.Popup(); ok {
//		list.Draw(m.paintPanel) // option i is in sel.Part(i)
//	}
func SelectBase[T comparable](c C, value *T, choices []T, label func(T) string) Box {
	return selectOf(c, value, choices, label, true)
}

// TextFieldBase is TextField without a look: the editable text, with no padding,
// background, border or corners of its own.
func TextFieldBase[T TextValue](c C, value T, label, placeholder string) Box {
	return textFieldOf(c, value, label, placeholder, true)
}
