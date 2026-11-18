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
func (b Box) Icon(icon gfx.Icon) Box {
	if b.n().Kind == kindButton {
		b.f.Extra(b.i).icon = icon
		return b
	}
	b.addons().start = style.InputGroupSlot{Kind: style.InputGroupSlotIcon, Icon: icon}
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
func (b Box) Action(icon gfx.Icon, Label string) Box {
	a := b.addons()
	a.end = style.InputGroupSlot{
		Kind: style.InputGroupSlotButton, ID: b.actionID(), Icon: icon, Label: Label,
		ButtonSize: components.InputGroupButtonIconXS,
	}
	return b
}

// ActionClicked reports whether this paint delivers a press on the field's Action. The
// keyboard goes back to the field then, so a clear or a reveal does not end the typing.
func (b Box) ActionClicked() bool {
	n := b.n()
	if n.Kind != kindInput || n.Ext == 0 || b.f.Exts[n.Ext].addons == 0 || b.f.Surface == nil {
		return false
	}
	end := b.f.addons[b.f.Exts[n.Ext].addons-1].end
	if end.Kind != style.InputGroupSlotButton || n.Flags&flagDisabled != 0 || !b.f.Surface.Clicked(end.ID) {
		return false
	}
	b.f.Surface.RequestFocus(widgetsOf(b.f).inputs[n.Aux].ID)
	return true
}

// paintAddons paints a field with an icon or a button inside it: the addons where
// InputGroup puts them, the text between them.
func (f *frame) paintAddons(n *node, snapshot style.InputSnapshot, a *fieldAddons) {
	canvas, theme := f.Surface, f.Palette
	disabled := n.Flags&flagDisabled != 0 || snapshot.Disabled
	start, end, content := components.InputAddonsLayout(canvas, theme, n.Rect, a.start, a.end)
	components.PaintInputAddon(canvas, theme, start, a.start, true, disabled)
	components.PaintInputContent(canvas, theme, n.Rect, content, snapshot)
	components.PaintInputAddon(canvas, theme, end, a.end, false, disabled) // its hit over the field's
}
