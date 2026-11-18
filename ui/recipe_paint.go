package ui

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// textEntryInsets is the room the shadcn text-entry controls leave between their edge and
// their text: across, the metrics' text inset; down, nothing for a line (which is centered
// in the control's whole height) and a fixed margin for rows that start at the top.
func textEntryInsets(Palette *Theme, k Kind) (x, y float32) {
	x = Palette.Metrics.TextInsetX()
	if k == kindTextarea {
		y = components.TextareaPadY
	}
	return x, y
}

// textEntryLook is the recipe a themed Input or Textarea is dressed with: the
// page's Styles.TextEntry when it set one, else the theme's.
func (f *frame) TextEntryLook(Palette *Theme, state gfx.State) components.TextEntryLook {
	if f.styles != nil && f.styles.TextEntry != nil {
		return f.styles.TextEntry(*Palette, state)
	}
	return components.TextEntryRecipe(*Palette, state)
}
