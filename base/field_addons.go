package base

import (
	"github.com/vivaui/viva/gfx"
)

// fieldAddons is what sits inside a field beside its text: an icon at the start,
// and a button at the end.
type FieldAddons struct {
	start, end InputGroupSlot
}

func (b Box) Addons() *FieldAddons {
	n := b.N()
	if n.Kind != kindInput {
		panic("ui: Icon is for a Button or a TextField, Action for a TextField")
	}
	extra := b.f.Extra(b.i)
	if extra.addons == 0 {
		b.f.addons = append(b.f.addons, FieldAddons{})
		extra.addons = int32(len(b.f.addons))
		b.f.PaintAddonsFn = (*Frame).paintAddons
	}
	return &b.f.addons[extra.addons-1]
}

// actionID names a field's Action in the field's own scope.
func (b Box) ActionID() gfx.ID {
	id := b.N().ID
	if id.IsZero() || b.f.Surface == nil {
		return gfx.ID{}
	}
	return b.f.Surface.Tree().Sub(id.Scope(), id.Name()).At(0)
}

// Icon puts an icon at the start of the field's addons: the field's leading
// icon. It sets the element's icon directly when it is a button.
func (b Box) Icon(icon gfx.Icon) Box {
	if b.N().Kind == KindButton {
		b.GetF().Extra(b.GetI()).SetIcon(icon)
		return b
	}
	b.Addons().SetStart(InputGroupSlot{Kind: InputGroupSlotIcon, Icon: icon})
	return b
}

// Action puts an icon button at the end of the field's addons.
func (b Box) Action(icon gfx.Icon, Label string) Box {
	a := b.Addons()
	a.SetEnd(InputGroupSlot{
		Kind: InputGroupSlotButton, ID: b.ActionID(), Icon: icon, Label: Label,
		ButtonSize: InputGroupButtonIconXS,
	})
	return b
}

// ActionClicked reports whether this paint delivered a press on the field's
// Action, and moves the focus back to the field.
func (b Box) ActionClicked() bool {
	n := b.N()
	if n.Kind != KindInput || n.Ext == 0 || b.GetF().Exts[n.Ext].GetAddons() == 0 || b.GetF().Surface == nil {
		return false
	}
	end := b.GetF().GetAddons()[b.GetF().Exts[n.Ext].GetAddons()-1].GetEnd()
	if end.Kind != InputGroupSlotButton || n.Flags&FlagDisabled != 0 || !b.GetF().Surface.Clicked(end.ID) {
		return false
	}
	b.GetF().Surface.RequestFocus(WidgetsOf(b.GetF()).GetInputs()[n.Aux].ID)
	return true
}
