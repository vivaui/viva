package components

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestSheetSideGeometry(t *testing.T) {
	viewport := gfx.Rect{X: 10, Y: 20, Width: 800, Height: 600}
	tests := []struct {
		name string
		side SheetSide
		want gfx.Rect
	}{
		{name: "right", side: SheetRight, want: gfx.Rect{X: 510, Y: 20, Width: 300, Height: 600}},
		{name: "left", side: SheetLeft, want: gfx.Rect{X: 10, Y: 20, Width: 300, Height: 600}},
		{name: "top", side: SheetTop, want: gfx.Rect{X: 10, Y: 20, Width: 800, Height: 300}},
		{name: "bottom", side: SheetBottom, want: gfx.Rect{X: 10, Y: 320, Width: 800, Height: 300}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var canvas gfx.Canvas
			layout := BeginSheet(&canvas, gfx.LightTheme(), viewport, SheetOptions{
				Layer: 4, Scope: gfx.Item(5), DismissID: gfx.Item(6), CloseID: gfx.Item(7), Side: test.side, ContentExtent: 300,
			}).Paint("Settings", "Configure the application.")
			if layout.Content != test.want {
				t.Fatalf("content = %+v, want %+v", layout.Content, test.want)
			}
		})
	}
}

func TestSheetUsesModalLayerScopeAndTypedDisabledHits(t *testing.T) {
	var canvas gfx.Canvas
	BeginSheet(&canvas, gfx.DarkTheme(), gfx.Rect{Width: 700, Height: 500}, SheetOptions{
		Layer: 13, Scope: gfx.Item(14), DismissID: gfx.Item(15), CloseID: gfx.Item(16),
	}).Disabled().Paint("Locked", "Complete the operation first.")
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Layer != 13 {
			t.Fatalf("rectangle layer = %d", rectangle.Layer)
		}
	}
	if canvas.TrappedFocusScope() != gfx.Item(14) || !canvas.EscapeTarget().IsZero() {
		t.Fatalf("modal routing = %d / %d", canvas.TrappedFocusScope(), canvas.EscapeTarget())
	}
	if len(targets(&canvas)) != 2 || len(blocks(&canvas)) != 1 {
		t.Fatalf("hits = %+v", canvas.Hits())
	}
	for _, hit := range targets(&canvas) {
		if hit.Scope != gfx.Item(14) || hit.Role != gfx.RoleButton || !hit.Disabled {
			t.Fatalf("disabled sheet hit = %+v", hit)
		}
	}
}

func TestSheetAndDrawerFootersTakeActionsAtTheDefaultButtonHeight(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	viewport := gfx.Rect{Width: 900, Height: 600}
	sheet := BeginSheet(&canvas, theme, viewport, SheetOptions{Layer: 3, Scope: gfx.Item(1), CloseID: gfx.Item(2)})
	layout := sheet.Begin("Edit", "Change things.")
	sheet.Actions(layout, ButtonAction{ID: gfx.Item(3), Label: "Cancel", Variant: ButtonOutline}, ButtonAction{ID: gfx.Item(4), Label: "Save"})
	sheet.End()
	canvas.Reset()
	drawer := BeginDrawer(&canvas, theme, viewport, DrawerOptions{Layer: 3, Scope: gfx.Item(1), CloseID: gfx.Item(2)})
	drawerLayout := drawer.Begin("Edit", "Change things.")
	drawer.Actions(drawerLayout, ButtonAction{ID: gfx.Item(3), Label: "Save"})
	drawer.End()
	for name, footer := range map[string]gfx.Rect{"sheet": layout.Footer, "drawer": drawerLayout.Footer} {
		if footer.Height != dialogFooterHeight(theme.Metrics, ButtonSizeDefault) {
			t.Errorf("%s footer is %v high, want the dialog's %v", name, footer.Height, dialogFooterHeight(theme.Metrics, ButtonSizeDefault))
		}
	}
	for _, hit := range canvas.Hits() {
		if hit.ID == gfx.Item(3) && hit.Bounds.Height != dialogFooterHeight(theme.Metrics, ButtonSizeDefault) {
			t.Errorf("drawer action is %v high", hit.Bounds.Height)
		}
	}
}

func TestSheetAndDrawerFitTheirPartsWhenNoExtentIsGiven(t *testing.T) {
	theme := gfx.LightTheme()
	viewport := gfx.Rect{Width: 900, Height: 600}
	var canvas gfx.Canvas
	bottomSheet := BeginSheet(&canvas, theme, viewport, SheetOptions{Layer: 1, Side: SheetBottom, BodyHeight: 100}).Paint("Edit", "")
	taller := BeginSheet(&canvas, theme, viewport, SheetOptions{Layer: 1, Side: SheetBottom, BodyHeight: 160}).Paint("Edit", "")
	if taller.Content.Height-bottomSheet.Content.Height != 60 || bottomSheet.Body.Height != 100 {
		t.Fatalf("sheet %+v / %+v: want the extent to follow BodyHeight and the body to be exactly that tall", bottomSheet, taller)
	}
	bottomDrawer := BeginDrawer(&canvas, theme, viewport, DrawerOptions{Layer: 1, BodyHeight: 100}).Paint("Edit", "")
	if bottomDrawer.Body.Height != 100 || bottomDrawer.Content.Y+bottomDrawer.Content.Height != viewport.Height {
		t.Fatalf("drawer %+v", bottomDrawer)
	}
}
