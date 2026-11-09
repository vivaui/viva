package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var drawerComponent = Component{
	ID: "drawer", Title: "Drawer", Category: "Overlays", Summary: "从视口边缘进入的模态表面与显式关闭语义。",
	paint: (*State).paintDrawer, initialize: (*State).initializeDrawer,
	Examples: []Example{
		{ID: "default", Title: "底部抽屉", Description: "生产 Drawer 暴露正文和页脚槽。", Snippet: `drawer := components.BeginDrawer(canvas, theme, viewport, components.DrawerOptions{
    Layer: 2, Scope: scope, CloseID: closeID,
    Side: components.DrawerBottom, ContentExtent: min(248, size.Height*0.72),
}).WithHandle()
layout := drawer.Begin("Workspace settings", "Applies immediately.")
canvas.Label("Native settings.", layout.Body, labelStyle)
drawer.Actions(layout, components.ButtonAction{ID: saveID, Label: "Save"})
drawer.End()
if canvas.Clicked(saveID) || canvas.Clicked(closeID) { open = false }`},
		{ID: "sides", Title: "侧边抽屉", Description: "右侧抽屉使用相同的模态与焦点契约。", Snippet: `drawer := components.BeginDrawer(canvas, theme, viewport, components.DrawerOptions{
    Layer: 2, Scope: scope, CloseID: closeID,
    Side: components.DrawerRight, ContentExtent: min(300, size.Width*0.78),
})
layout := drawer.Begin("Workspace settings", "Same modal contract on the right edge.")
canvas.Label("Native settings.", layout.Body, labelStyle)
drawer.Actions(layout, components.ButtonAction{ID: saveID, Label: "Save changes"})
drawer.End()
if canvas.Clicked(saveID) || canvas.Clicked(closeID) { open = false }`},
		{ID: "disabled", Title: "禁用抽屉", Description: "保留可见结构但关闭交互。", Snippet: `drawer := components.BeginDrawer(canvas, theme, viewport, components.DrawerOptions{
    Layer: 2, Scope: scope, CloseID: closeID, Side: components.DrawerBottom,
}).WithHandle().Disabled()
layout := drawer.Begin("Workspace settings", "Structure remains visible; interaction is off.")
drawer.Actions(layout, components.ButtonAction{ID: saveID, Label: "Save changes"})
drawer.End()`},
	},
}

var (
	drawerUI    = tree.Feature("drawer")
	drawerOpen  = tree.ID(drawerUI, "open")
	drawerClose = tree.ID(drawerUI, "close")
	drawerFocus = tree.ID(drawerUI, "focus")
	drawerSave  = tree.ID(drawerUI, "save")
)

func (state *State) initializeDrawer() { state.flags |= 1 }

func (state *State) paintDrawer(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	open := state.flags&1 != 0
	state.page(canvas, theme, size, 480, state.Text("Drawer · 边缘模态", "Drawer · Edge modal"), func(f *ui.Frame) {
		// The page behind an open drawer is just its title.
		if !open && f.Button(drawerOpen, "Open drawer").Self(ui.Start).Clicked() {
			state.flags |= 1
		}
	})
	if !open {
		return
	}
	side := components.DrawerBottom
	extent := min(float32(248), size.Height*0.72)
	if state.exampleID == "sides" {
		side = components.DrawerRight
		extent = min(float32(300), size.Width*0.78)
	}
	viewport := gfx.Rect{Width: size.Width, Height: size.Height}
	drawer := components.BeginDrawer(canvas, theme, viewport, components.DrawerOptions{Layer: 2, Scope: drawerFocus, CloseID: drawerClose, Side: side, ContentExtent: extent}).WithHandle()
	if state.exampleID == "disabled" {
		drawer = drawer.Disabled()
	}
	layout := drawer.Begin("Workspace settings", "Applies immediately.")
	canvas.Label("Native settings.", layout.Body, gfx.TextStyle{Size: theme.Metrics.FontSizeM, Color: theme.Foreground})
	drawer.Actions(layout, components.ButtonAction{ID: drawerSave, Label: "Save changes"})
	drawer.End()
	if canvas.Clicked(drawerSave) || canvas.Clicked(drawerClose) {
		state.flags &^= 1
	}
}
