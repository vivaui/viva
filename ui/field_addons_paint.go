package ui

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
)

// Icon puts icon on a Button before its label, or alone on an IconOnly one, whose
// label is then its name for assistive technology and its hover hint; and inside a
// TextField before its text, as the magnifier of a search field.
//
//	ui.Button(c, "New host").Icon(plus)
//	ui.Button(c, "Delete").Icon(trash).IconOnly().Ghost()
//	ui.TextField(c, &m.query, "Search", "Search hosts").Icon(search)
func boxIcon(b Box, icon gfx.Icon) Box {
	if b.N().Kind == KindButton {
		b.GetF().Extra(b.GetI()).SetIcon(icon)
		return b
	}
	b.Addons().SetStart(style.InputGroupSlot{Kind: style.InputGroupSlotIcon, Icon: icon})
	return b
}

// Action puts a small icon button inside a TextField at its end: the clear of a
// search field, the eye that shows a password. label is its name for assistive
// technology and a test; ActionClicked reports a press on it.
//
//	field := ui.TextField(c, &m.query, "Search", "").Action(cross, "Clear")
//	if field.ActionClicked() {
//		m.query = ""
//	}
func boxAction(b Box, icon gfx.Icon, Label string) Box {
	a := b.Addons()
	a.SetEnd(style.InputGroupSlot{
		Kind: style.InputGroupSlotButton, ID: b.ActionID(), Icon: icon, Label: Label,
		ButtonSize: components.InputGroupButtonIconXS,
	})
	return b
}

// ActionClicked reports whether this paint delivers a press on the field's Action. The
// keyboard goes back to the field then, so a clear or a reveal does not end the typing.
func boxActionClicked(b Box) bool {
	n := b.N()
	if n.Kind != KindInput || n.Ext == 0 || b.GetF().Exts[n.Ext].GetAddons() == 0 || b.GetF().Surface == nil {
		return false
	}
	end := b.GetF().GetAddons()[b.GetF().Exts[n.Ext].GetAddons()-1].GetEnd()
	if end.Kind != style.InputGroupSlotButton || n.Flags&FlagDisabled != 0 || !b.GetF().Surface.Clicked(end.ID) {
		return false
	}
	b.GetF().Surface.RequestFocus(WidgetsOf(b.GetF()).GetInputs()[n.Aux].ID)
	return true
}

// paintAddons paints a field with an icon or a button inside it: the addons where
// InputGroup puts them, the text between them.
func paintAddons(f *Frame, n *Node, snapshot style.InputSnapshot, a *FieldAddons) {
	canvas, theme := f.Surface, f.Palette
	disabled := n.Flags&FlagDisabled != 0 || snapshot.Disabled
	start, end, content := components.InputAddonsLayout(canvas, theme, n.Rect, a.GetStart(), a.GetEnd())
	components.PaintInputAddon(canvas, theme, start, a.GetStart(), true, disabled)
	components.PaintInputContent(canvas, theme, n.Rect, content, snapshot)
	components.PaintInputAddon(canvas, theme, end, a.GetEnd(), false, disabled) // its hit over the field's
}
