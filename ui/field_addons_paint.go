package ui

import (
	"github.com/vivaui/viva/base"
	"github.com/vivaui/viva/components"
)

// paintAddons paints a field with an icon or a button inside it: the addons where
// InputGroup puts them, the text between them.
func paintAddons(f *Frame, n *Node, snapshot base.InputSnapshot, a *FieldAddons) {
	canvas, theme := f.Surface, f.Palette
	disabled := n.Flags&FlagDisabled != 0 || snapshot.Disabled
	start, end, content := components.InputAddonsLayout(canvas, theme, n.Rect, a.GetStart(), a.GetEnd())
	components.PaintInputAddon(canvas, theme, start, a.GetStart(), true, disabled)
	components.PaintInputContent(canvas, theme, n.Rect, content, snapshot)
	components.PaintInputAddon(canvas, theme, end, a.GetEnd(), false, disabled) // its hit over the field's
}
