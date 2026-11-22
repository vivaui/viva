package base

import (
	"unsafe"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
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
	ActionVariant            style.ButtonVariant
	// Small uses the alert dialog's compact layout.
	Small bool
	// Disabled makes both actions visible, semantic and inert.
	Disabled bool
}

type AlertDialogDecl struct {
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
func (f *Frame) AlertDialog(spec AlertDialogSpec) Box {
	layer := spec.Layer
	if layer == 0 {
		layer = f.WidgetLayer() + 2
	}
	root := f.Popup().At(0, 0).Fill()
	r := root.N()
	r.Flags |= flagAlertDialog
	r.Aux = int32(len(WidgetsOf(f).alertDialogs))
	WidgetsOf(f).alertDialogs = append(WidgetsOf(f).alertDialogs, AlertDialogDecl{spec: spec, layer: layer})
	return root
}

// AlertDialog asks the user to confirm while *open, as Modal shows a dialog: a
// scrim, the title and description, and the confirming action beside an outline
// cancel action. It reports true in the paint that delivers the confirming
// action. That action, the cancel action and Escape set *open to false; a press on
// the scrim does not, as an alert dialog must be answered. The page routes its
// keys, pointer and shortcuts while it is open, gives the keyboard to the cancel
// action as it opens and back to where it was as it closes.
//
//	if ui.AlertDialog(c, &m.confirmDelete, ui.AlertDialogSpec{
//		Title: "Delete host?", Description: "This cannot be undone.",
//		ActionLabel: "Delete", CancelLabel: "Cancel", ActionVariant: ui.ButtonDestructive,
//	}) {
//		m.deleteHost()
//	}
//
// Scope, ActionID and CancelID are named from open when the spec leaves them
// zero; a page that addresses the actions itself (a test, a shortcut) names them.
func AlertDialog(c C, open *bool, spec AlertDialogSpec) bool {
	if c == nil || c.page == nil || c.f == nil {
		panic("ui: AlertDialog is for a page")
	}
	p := c.page
	p.ForbidInMemo("AlertDialog")
	if !*open {
		return false
	}
	p.overlay = true
	if spec.Scope.IsZero() || spec.ActionID.IsZero() || spec.CancelID.IsZero() {
		scope := p.BoundScope(pkAlertDialog, unsafe.Pointer(open))
		if spec.Scope.IsZero() {
			spec.Scope = scope.At(0)
		}
		if spec.ActionID.IsZero() {
			spec.ActionID = scope.At(1)
		}
		if spec.CancelID.IsZero() {
			spec.CancelID = scope.At(2)
		}
	}
	if spec.InitialFocusID.IsZero() {
		spec.InitialFocusID = spec.CancelID
	}
	id, chosen := c.f.AlertDialog(spec).Chosen()
	if !chosen {
		return false
	}
	*open = false
	return id == spec.ActionID
}
