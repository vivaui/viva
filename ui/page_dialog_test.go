package ui_test

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// A page Dialog keeps the identities, scope and layer the application gave it,
// and reports its dismissal as Frame.Dialog does.
func TestPageDialogKeepsItsExplicitTargets(t *testing.T) {
	var tree gfx.Tree
	root := tree.Feature("dialog-page")
	scope := tree.Sub(root, tree.Name("scope"))
	closeID := scope.ID(tree.Name("close"))
	dismissID := scope.ID(tree.Name("dismiss"))
	open := true

	config := ui.PageConfig{
		Tree: &tree, Root: "dialog-page", Theme: &pageTheme,
		View: func(c ui.C) {
			if !open {
				return
			}
			dialog := ui.Dialog(c, ui.DialogSpec{
				Title: "Delete host?", Scope: scope.ID(tree.Name("scope")),
				DismissID: dismissID, CloseID: closeID, Width: 320,
			})
			dialog.Header(func() { ui.Text(c, "Header slot") })
			dialog.Body(func() { ui.Text(c, "Body text") })
			if dialog.Dismissed() {
				open = false
			}
		},
	}
	d := openPageWith(t, config, app.Config{})
	if !d.page.Overlay() {
		t.Fatal("an open Dialog does not report an overlay")
	}
	if !hasText(d, "Body text") || !hasText(d, "Header slot") {
		t.Fatal("the Dialog body or header is not painted")
	}
	d.click("Close dialog")
	if open || d.page.Overlay() || hasText(d, "Body text") {
		t.Fatalf("close: open=%v overlay=%v", open, d.page.Overlay())
	}
}

func hasText(d *pageDriver, value string) bool {
	for _, text := range d.driver.Snapshot().Texts {
		if text.Value == value {
			return true
		}
	}
	return false
}
