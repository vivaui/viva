package base

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
)

// ControlView is what a control's painter may know of its element: where it is,
// what it says, how it looks and the state the pointer and keyboard put it in.
// It is a value, so the engine's element model stays below the styled controls;
// a painter reads this and its own state, never the engine's node.
type ControlView struct {
	Kind   Kind
	Rect   gfx.Rect
	Aux    int32
	ID     gfx.ID
	Label  string
	Text   string
	Font   float32
	Weight float32
	// Variant and Size are the button recipe's choices.
	Variant style.ButtonVariant
	Size    style.Size
	// Flags are the element's state: the checked, mixed, small, disabled,
	// has-hit and themed bits.
	Flags    uint32
	Hit      gfx.HitOptions
	HasHit   bool
	Exact    bool // HitExact: the control targets its laid-out rectangle
	Disabled bool
	// Color is the element's own text color, when HasColor says it has one.
	Color    gfx.Color
	HasColor bool
	// Icon is the optional icon a button, input or control carries.
	Icon gfx.Icon
	// SwitchRecipe is a switch's own look, when the caller gave one.
	SwitchRecipe func(gfx.Theme, gfx.State, float32) style.SwitchLook
}

// viewOf is the view of element n a control painter is given.
func ViewOf(f *Frame, n *Node) ControlView {
	extra := &f.Exts[n.Ext]
	return ControlView{
		Kind: n.Kind, Rect: n.Rect, Aux: n.Aux, ID: n.ID, Label: n.Label, Text: n.Text,
		Font: n.FontSize, Weight: n.Weight, Variant: n.Variant, Size: n.ButtonSize,
		Flags: n.Flags, Hit: extra.hit, HasHit: n.Flags&flagHasHit != 0, Exact: extra.hitExact,
		Disabled: n.Flags&flagDisabled != 0, Icon: extra.icon,
		Color: n.Color, HasColor: n.Flags&flagHasColor != 0,
		SwitchRecipe: extra.switchRecipe,
	}
}
