package ui

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// ControlView is what a control's painter may know of its element: where it is,
// what it says, how it looks and the state the pointer and keyboard put it in.
// It is a value, so the engine's element model stays below the styled controls;
// a painter reads this and its own state, never the engine's node.
type ControlView struct {
	Kind   kind
	Rect   gfx.Rect
	Aux    int32
	ID     gfx.ID
	Label  string
	Text   string
	Font   float32
	Weight float32
	// Variant and Size are the button recipe's choices.
	Variant components.ButtonVariant
	Size    components.ButtonSize
	// Flags are the element's state: the checked, mixed, small, disabled,
	// has-hit and themed bits.
	Flags    uint32
	Hit      gfx.HitOptions
	HasHit   bool
	Disabled bool
	// Color is the element's own text color, when HasColor says it has one.
	Color    gfx.Color
	HasColor bool
	// Icon is the optional icon a button, input or control carries.
	Icon gfx.Icon
	// SwitchRecipe is a switch's own look, when the caller gave one.
	SwitchRecipe func(gfx.Theme, gfx.State, float32) components.SwitchLook
}

// viewOf is the view of element n a control painter is given.
func viewOf(f *frame, n *node) ControlView {
	extra := &f.exts[n.ext]
	return ControlView{
		Kind: n.kind, Rect: n.rect, Aux: n.aux, ID: n.id, Label: n.label, Text: n.text,
		Font: n.fontSize, Weight: n.weight, Variant: n.variant, Size: n.buttonSize,
		Flags: n.flags, Hit: extra.hit, HasHit: n.flags&flagHasHit != 0,
		Disabled: n.flags&flagDisabled != 0, Icon: extra.icon,
		Color: n.color, HasColor: n.flags&flagHasColor != 0,
		SwitchRecipe: extra.switchRecipe,
	}
}
