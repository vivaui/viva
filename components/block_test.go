package components

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// targets returns the hits that are interaction targets, leaving out opaque
// blocks.
func targets(canvas *gfx.Canvas) []gfx.Hit {
	var result []gfx.Hit
	for _, hit := range canvas.Hits() {
		if !hit.Block {
			result = append(result, hit)
		}
	}
	return result
}

// blocks returns the opaque regions.
func blocks(canvas *gfx.Canvas) []gfx.Hit {
	var result []gfx.Hit
	for _, hit := range canvas.Hits() {
		if hit.Block {
			result = append(result, hit)
		}
	}
	return result
}

// A modal's panel must shield the dismiss target behind it. The block sits
// after the backdrop and before the panel's own controls, so those still win.
func TestModalPanelsShieldTheirDismissTargetButNotTheirOwnControls(t *testing.T) {
	viewport := gfx.Rect{Width: 800, Height: 600}
	for name, paint := range map[string]func(*gfx.Canvas){
		"dialog": func(canvas *gfx.Canvas) {
			BeginDialog(canvas, gfx.LightTheme(), viewport, DialogOptions{Layer: 1, Scope: gfx.Item(7), DismissID: gfx.Item(1), CloseID: gfx.Item(2)}).Paint("T", "D")
		},
		"sheet": func(canvas *gfx.Canvas) {
			BeginSheet(canvas, gfx.LightTheme(), viewport, SheetOptions{Layer: 1, Scope: gfx.Item(7), DismissID: gfx.Item(1), CloseID: gfx.Item(2)}).Paint("T", "D")
		},
		"drawer": func(canvas *gfx.Canvas) {
			BeginDrawer(canvas, gfx.LightTheme(), viewport, DrawerOptions{Layer: 1, Scope: gfx.Item(7), DismissID: gfx.Item(1), CloseID: gfx.Item(2)}).Paint("T", "D")
		},
	} {
		t.Run(name, func(t *testing.T) {
			var canvas gfx.Canvas
			paint(&canvas)
			hits := canvas.Hits()
			order := map[string]int{}
			for index, hit := range hits {
				switch {
				case hit.Block:
					order["block"] = index
				case hit.ID == gfx.Item(1):
					order["backdrop"] = index
				case hit.ID == gfx.Item(2):
					order["close"] = index
				}
			}
			if len(order) != 3 || !(order["backdrop"] < order["block"] && order["block"] < order["close"]) {
				t.Fatalf("declaration order %v; want backdrop, then block, then the close control", order)
			}
			panel := hits[order["block"]].Bounds
			if panel.Width <= 0 || panel.Height <= 0 || panel.Width >= viewport.Width && panel.Height >= viewport.Height {
				t.Fatalf("block %+v must cover the panel, not the whole viewport", panel)
			}
		})
	}
}
