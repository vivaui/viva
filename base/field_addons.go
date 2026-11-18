package base

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
)

// fieldAddons is what sits inside a field beside its text: an icon at the start,
// and a button at the end.
type FieldAddons struct {
	start, end style.InputGroupSlot
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
