package ui

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// textEntryLook is the recipe a themed Input or Textarea is dressed with: the
// page's Styles.TextEntry when it set one, else the theme's.
func frameTextEntryLook(f *Frame, Palette *Theme, state gfx.State) components.TextEntryLook {
	if f.GetStyles() != nil && f.GetStyles().TextEntry != nil {
		return f.GetStyles().TextEntry(*Palette, state)
	}
	return components.TextEntryRecipe(*Palette, state)
}

// frameButtonLook is the recipe a themed button is dressed with: its own ButtonRecipe, else
// the page's Styles.Button, else the theme's.
func frameButtonLook(f *Frame, n *Node, Palette *Theme, state gfx.State) components.ButtonLook {
	if recipe := f.Exts[n.Ext].GetButtonRecipe(); recipe != nil {
		return recipe(*Palette, n.Variant, state)
	}
	if f.GetStyles() != nil && f.GetStyles().Button != nil {
		return f.GetStyles().Button(*Palette, n.Variant, state)
	}
	return components.ButtonRecipe(*Palette, n.Variant, state)
}
