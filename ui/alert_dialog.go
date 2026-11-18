package ui

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// AlertDialogSpec declares a confirmation painted by components.AlertDialog: a
// scrim, a centered surface with its title and description, and the confirming
// action joined by an outline cancel action at the right edge. Escape, the cancel
// action and the close target report CancelID; the scrim itself does not dismiss
// it. The application declares it while it is open and stops declaring it to
// close it.
type AlertDialogSpec struct {
	Title, Description string
	// Width is the surface's width, 512 when zero (320 with Small), kept inside
	// the frame.
	Width float32
	// Layer is the compositing layer of the scrim and surface; zero is two above
	// the layer the dialog is declared in, as a Dialog's.
	Layer uint16
	// Scope is the focus scope Tab stays inside. ActionID and CancelID are the two
	// actions; InitialFocusID takes focus when the dialog opens.
	Scope, ActionID, CancelID, InitialFocusID gfx.ID
	// ActionLabel and CancelLabel are the two buttons' labels. ActionVariant
	// dresses the confirming action; the cancel action is outline.
	ActionLabel, CancelLabel string
	ActionVariant            components.ButtonVariant
	// Small uses the alert dialog's compact layout.
	Small bool
	// Disabled makes both actions visible, semantic and inert.
	Disabled bool
}

type alertDialogDecl struct {
	spec  AlertDialogSpec
	layer uint16
}

// AlertDialog adds a confirmation dialog painted by components.AlertDialog, the
// alert dialog's title metrics and geometry included. Chosen reports the action a
// paint is delivering a click on:
//
//	d := f.AlertDialog(AlertDialogSpec{Title: "Discard changes?", Description: "This cannot be undone.",
//		Scope: dlgScope, ActionID: discardID, CancelID: cancelID,
//		ActionLabel: "Discard", CancelLabel: "Cancel", ActionVariant: components.ButtonDestructive})
//	if id, ok := d.Chosen(); ok && id == discardID { discard() }
func (f *frame) AlertDialog(spec AlertDialogSpec) Box {
	layer := spec.Layer
	if layer == 0 {
		layer = f.WidgetLayer() + 2
	}
	root := f.Popup().At(0, 0).Fill()
	r := root.n()
	r.Flags |= flagAlertDialog
	r.Aux = int32(len(widgetsOf(f).alertDialogs))
	widgetsOf(f).alertDialogs = append(widgetsOf(f).alertDialogs, alertDialogDecl{spec: spec, layer: layer})
	return root
}

// AlertDialog declares a confirmation on a page, as Dialog does: the page routes
// its keys, pointer and shortcuts with the modal open. It declares no children;
// declare it where it is open.
func AlertDialog(c C, spec AlertDialogSpec) Box {
	if c == nil || c.page == nil || c.f == nil {
		panic("ui: AlertDialog is for a page")
	}
	p := c.page
	p.ForbidInMemo("AlertDialog")
	p.overlay = true
	return c.f.AlertDialog(spec)
}

// paintAlertDialog paints a confirmation: components.AlertDialog owns its scrim,
// surface, title, description, actions, focus trap and Escape target.
func (f *frame) paintAlertDialog(n *node) {
	d := &widgetsOf(f).alertDialogs[n.Aux]
	spec := &d.spec
	options := components.AlertDialogOptions{
		Layer: d.layer, Scope: spec.Scope,
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
