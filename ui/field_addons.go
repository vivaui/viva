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
	if n.Kind != kindInput {
		panic("ui: Icon is for a Button or a TextField, Action for a TextField")
	}
	extra := b.f.Extra(b.i)
	if extra.addons == 0 {
		b.f.addons = append(b.f.addons, fieldAddons{})
		extra.addons = int32(len(b.f.addons))
		b.f.paintAddonsFn = (*frame).paintAddons
	}
	return &b.f.addons[extra.addons-1]
}

// actionID names a field's Action in the field's own scope.
func (b Box) actionID() gfx.ID {
	id := b.n().ID
	if id.IsZero() || b.f.Surface == nil {
		return gfx.ID{}
	}
	return b.f.Surface.Tree().Sub(id.Scope(), id.Name()).At(0)
}
