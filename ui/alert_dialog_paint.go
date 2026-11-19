package ui

import (
	"github.com/vivaui/viva/base"
	"github.com/vivaui/viva/components"
)

// paintAlertDialog paints a confirmation: components.AlertDialog owns its scrim,
// surface, title, description, actions, focus trap and Escape target.
func paintAlertDialog(f *Frame, n *Node) {
	d := &base.WidgetsOf(f).GetAlertDialogs()[n.Aux]
	spec := d.SpecPtr()
	options := components.AlertDialogOptions{
		Layer: d.GetLayer(), Scope: spec.Scope,
		ActionID: spec.ActionID, CancelID: spec.CancelID,
		InitialFocusID: spec.InitialFocusID,
		ActionVariant:  spec.ActionVariant,
		ContentSize:    Size{Width: spec.Width},
	}
	if spec.Small {
		options.Size = components.AlertDialogSmall
	}
	dialog := components.BeginAlertDialog(f.Surface, f.Palette, n.Rect, options)
	if spec.Disabled {
		dialog = dialog.Disabled()
	}
	dialog.Paint(spec.Title, spec.Description, spec.ActionLabel, spec.CancelLabel)
}
