package ui

import (
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// An AlertDialog is painted by components.AlertDialog and nothing else: the same
// scrim, surface, title, description and actions, with the alert dialog's own
// title metrics, Escape target and focus scope.
func TestAlertDialogIsPaintedLikeTheComponent(t *testing.T) {
	theme := gfx.LightTheme()
	viewport := gfx.Rect{Width: 640, Height: 400}
	spec := AlertDialogSpec{
		Title: "Discard changes?", Description: "The file keeps its older contents.",
		Width: 512, Scope: dlgScope.ID(tree.Name("scope")),
		ActionID: dlgYes, CancelID: dlgNo, ActionLabel: "Discard", CancelLabel: "Cancel",
		ActionVariant: components.ButtonDestructive, InitialFocusID: dlgNo,
	}
	flexed, direct := testCanvas(), testCanvas()
	var view View
	view.Draw(flexed, theme, viewport, func(f *Frame) { f.AlertDialog(spec) })

	dialog := components.BeginAlertDialog(direct, theme, viewport, components.AlertDialogOptions{
		Layer: 2, Scope: spec.Scope, ActionID: spec.ActionID, CancelID: spec.CancelID,
		InitialFocusID: spec.InitialFocusID, ActionVariant: spec.ActionVariant,
		ContentSize: gfx.Size{Width: spec.Width},
	})
	dialog.Paint(spec.Title, spec.Description, spec.ActionLabel, spec.CancelLabel)

	if a, b := flexed.Rectangles(), direct.Rectangles(); len(a) != len(b) {
		t.Fatalf("ui painted %d rectangles, the component %d", len(a), len(b))
	} else {
		for i := range a {
			if a[i].Bounds != b[i].Bounds || a[i].Color != b[i].Color {
				t.Errorf("rectangle %d is %+v, the component's %+v", i, a[i], b[i])
			}
		}
	}
	if a, b := flexed.Texts(), direct.Texts(); len(a) != len(b) {
		t.Fatalf("ui painted %d texts, the component %d", len(a), len(b))
	} else {
		for i := range a {
			if a[i].Value != b[i].Value || a[i].Bounds != b[i].Bounds || a[i].Style != b[i].Style {
				t.Errorf("text %d is %+v, the component's %+v", i, a[i], b[i])
			}
		}
	}
	if a, b := flexed.Hits(), direct.Hits(); len(a) != len(b) {
		t.Fatalf("ui declared %d hits, the component %d", len(a), len(b))
	} else {
		for i := range a {
			if a[i].ID != b[i].ID || a[i].Bounds != b[i].Bounds || a[i].Label != b[i].Label || a[i].Role != b[i].Role {
				t.Errorf("hit %d is %+v, the component's %+v", i, a[i], b[i])
			}
		}
	}
	if got := flexed.EscapeTarget(); got != spec.CancelID {
		t.Errorf("Escape goes to %v, want the cancel action %v", got, spec.CancelID)
	}
	if got := flexed.TrappedFocusScope(); got != spec.Scope {
		t.Errorf("focus is trapped in %v, want %v", got, spec.Scope)
	}
}

// The two actions report the clicks this paint delivers, and only those.
func TestAlertDialogReportsItsTwoActions(t *testing.T) {
	theme := gfx.LightTheme()
	viewport := gfx.Rect{Width: 640, Height: 400}
	spec := AlertDialogSpec{
		Title: "Delete host?", Description: "This cannot be undone.",
		Scope: dlgScope.ID(tree.Name("scope")),
		ActionID: dlgYes, CancelID: dlgNo, ActionLabel: "Delete", CancelLabel: "Cancel",
	}
	confirmed, cancelled := false, false
	var view View
	draw := func(canvas *gfx.Canvas, action gfx.ID) {
		canvas.BeginEventPass(action, gfx.ClickInfo{})
		view.Draw(canvas, theme, viewport, func(f *Frame) {
			d := f.AlertDialog(spec)
			confirmed, cancelled = d.Confirmed(), d.Cancelled()
		})
	}
	draw(testCanvas(), dlgYes)
	if !confirmed || cancelled {
		t.Errorf("a click on the action reported confirmed=%v cancelled=%v, want true and false", confirmed, cancelled)
	}
	draw(testCanvas(), dlgNo)
	if confirmed || !cancelled {
		t.Errorf("a click on the cancel reported confirmed=%v cancelled=%v, want false and true", confirmed, cancelled)
	}
	draw(testCanvas(), gfx.ID{})
	if confirmed || cancelled {
		t.Error("a paint without a click reported an action")
	}
}
