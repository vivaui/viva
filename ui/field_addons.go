package ui

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// fieldAddons is what sits inside a field beside its text: an icon at the start,
// and a button at the end.
type fieldAddons struct {
	start, end components.InputGroupSlot
}

func (b Box) addons() *fieldAddons {
	n := b.n()
	if n.kind != kindInput {
		panic("ui: Icon is for a Button or a TextField, Action for a TextField")
	}
	extra := b.f.extra(b.i)
	if extra.addons == 0 {
		b.f.addons = append(b.f.addons, fieldAddons{})
		extra.addons = int32(len(b.f.addons))
		b.f.paintAddonsFn = (*Frame).paintAddons
	}
	return &b.f.addons[extra.addons-1]
}

// Icon puts icon on a Button before its label, or alone on an IconOnly one, whose
// label is then its name for assistive technology and its hover hint; and inside a
// TextField before its text, as the magnifier of a search field.
//
//	ui.Button(c, "New host").Icon(plus)
//	ui.Button(c, "Delete").Icon(trash).IconOnly().Ghost()
//	ui.TextField(c, &m.query, "Search", "Search hosts").Icon(search)
func (b Box) Icon(icon gfx.Icon) Box {
	if b.n().kind == kindButton {
		b.f.extra(b.i).icon = icon
		return b
	}
	b.addons().start = components.InputGroupSlot{Kind: components.InputGroupSlotIcon, Icon: icon}
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
func (b Box) Action(icon gfx.Icon, label string) Box {
	a := b.addons()
	a.end = components.InputGroupSlot{
		Kind: components.InputGroupSlotButton, ID: b.actionID(), Icon: icon, Label: label,
		ButtonSize: components.InputGroupButtonIconXS,
	}
	return b
}

// ActionClicked reports whether this paint delivers a press on the field's Action.
func (b Box) ActionClicked() bool {
	n := b.n()
	if n.kind != kindInput || n.ext == 0 || b.f.exts[n.ext].addons == 0 || b.f.canvas == nil {
		return false
	}
	end := b.f.addons[b.f.exts[n.ext].addons-1].end
	return end.Kind == components.InputGroupSlotButton && n.flags&flagDisabled == 0 && b.f.canvas.Clicked(end.ID)
}

// actionID names a field's Action in the field's own scope.
func (b Box) actionID() gfx.ID {
	id := b.n().id
	if id.IsZero() || b.f.canvas == nil {
		return gfx.ID{}
	}
	return b.f.canvas.Tree().Sub(id.Scope(), id.Name()).At(0)
}

// paintAddons paints a field with an icon or a button inside it: the addons where
// InputGroup puts them, the text between them.
func (f *Frame) paintAddons(n *node, snapshot components.InputSnapshot, a *fieldAddons) {
	canvas, theme := f.canvas, f.theme
	disabled := n.flags&flagDisabled != 0 || snapshot.Disabled
	start, end, content := components.InputAddonsLayout(canvas, theme, n.rect, a.start, a.end)
	components.PaintInputAddon(canvas, theme, start, a.start, true, disabled)
	components.PaintInputContent(canvas, theme, n.rect, content, snapshot)
	components.PaintInputAddon(canvas, theme, end, a.end, false, disabled) // its hit over the field's
}
