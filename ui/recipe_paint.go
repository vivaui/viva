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
