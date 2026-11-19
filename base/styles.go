package base

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
)

// Styles replaces the recipes a page's styled controls paint with, for an
// application whose look differs from the theme's shadcn recipes but which
// wants the controls themselves rather than a Base at every call site.
//
// A nil field keeps the theme's recipe (components.ButtonRecipe and the rest).
// A recipe is a pure function of the theme and the control's state; it is
// called at every paint, so it must not allocate on the steady path, block, or
// read anything but its arguments. It changes the look, not the measurements:
// a button's height, an input's padding and the controls' hit regions still
// come from the size variant and gfx.Theme.Metrics, so a page can also adjust
// density once in the theme.
//
// A single control with a look of its own is still styled through its Base,
// which also avoids recomputing a recipe for every control. Styles is for the
// page-wide case.
type Styles struct {
	// Button dresses Button, and the buttons a component paints.
	Button func(Palette gfx.Theme, Variant style.ButtonVariant, state gfx.State) style.ButtonLook
	// Checkbox dresses Checkbox.
	Checkbox func(Palette gfx.Theme, state gfx.State) style.CheckboxLook
	// Switch dresses Switch, progress being how far the thumb has travelled.
	Switch func(Palette gfx.Theme, state gfx.State, progress float32) style.SwitchLook
	// TextEntry dresses Input and Textarea: their fill, border, corners and
	// shadow. Their padding, and where the text sits, stay the metrics'.
	TextEntry func(Palette gfx.Theme, state gfx.State) style.TextEntryLook
}
