package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var sheetComponent = Component{
	ID: "sheet", Title: "Sheet", Category: "Overlay", Summary: "从桌面边缘进入、带焦点陷阱与关闭语义的模态面板。",
	paint: (*State).paintSheet,
	Examples: []Example{
		{ID: "right", Title: "右侧面板", Description: "从右侧打开生产 Sheet 并使用标准关闭按钮。", Snippet: `sheet := components.BeginSheet(canvas, theme, viewport, components.SheetOptions{
    Layer: 5, Scope: scope, DismissID: dismissID, CloseID: closeID,
    Side: components.SheetRight, ContentExtent: min(size.Width*0.78, 360),
})
layout := sheet.Begin("Project settings", "Changes remain caller-owned until Save.")
canvas.Label("Rendering backend", layout.Body, labelStyle)
sheet.Actions(layout, components.ButtonAction{ID: saveID, Label: "Save"})
sheet.End()
if canvas.Clicked(saveID) || canvas.Clicked(closeID) || canvas.Clicked(dismissID) { open = false }`},
		{ID: "bottom", Title: "底部面板", Description: "复用同一 API 展示底部边缘布局。", Snippet: `sheet := components.BeginSheet(canvas, theme, viewport, components.SheetOptions{
    Layer: 5, Scope: scope, DismissID: dismissID, CloseID: closeID,
    Side: components.SheetBottom, ContentExtent: min(size.Height*0.62, 230),
})
layout := sheet.Begin("Project settings", "Same API, bottom edge.")
canvas.Label("Native GPU", layout.Body, labelStyle)
sheet.Actions(layout, components.ButtonAction{ID: saveID, Label: "Save changes"})
sheet.End()
if canvas.Clicked(saveID) || canvas.Clicked(closeID) || canvas.Clicked(dismissID) { open = false }`},
	},
}

var (
	sheetUI      = tree.Feature("sheet")
	sheetTrigger = tree.ID(sheetUI, "trigger")
	sheetDismiss = tree.ID(sheetUI, "dismiss")
	sheetClose   = tree.ID(sheetUI, "close")
	sheetSave    = tree.ID(sheetUI, "save")
	sheetScope   = tree.ID(sheetUI, "content")
)

func (state *State) paintSheet(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	state.drawPage(canvas, theme, size, 560, "Project settings", func(c ui.C) {
		ui.Text(c, "Edit without leaving this view.").FontSize(theme.Metrics.FontSizeS).Muted()
		if ui.Button(c, "Open settings").Self(ui.Start).Hit(sheetTrigger, "Open settings").Clicked() {
			state.flags |= 1
		}
	})

	if state.flags&1 == 0 {
		return
	}
	side := components.SheetRight
	extent := min(size.Width*0.78, float32(360))
	if state.exampleID == "bottom" {
		side = components.SheetBottom
		extent = min(size.Height*0.62, float32(230))
	}
	sheet := components.BeginSheet(canvas, theme, gfx.Rect{Width: size.Width, Height: size.Height}, components.SheetOptions{
		Layer: 5, Scope: sheetScope, DismissID: sheetDismiss, CloseID: sheetClose, Side: side, ContentExtent: extent,
	})
	layout := sheet.Begin("Project settings", "Changes remain caller-owned until Save.")
	canvas.Label("Rendering backend", gfx.Rect{X: layout.Body.X, Y: layout.Body.Y, Width: layout.Body.Width, Height: 22}, gfx.TextStyle{Size: theme.Metrics.FontSizeM, Weight: gfx.WeightMedium, Color: theme.Foreground})
	field := gfx.Rect{X: layout.Body.X, Y: layout.Body.Y + 30, Width: layout.Body.Width, Height: theme.Metrics.ControlHeight}
	canvas.RoundedBorder(field, theme.Metrics.RadiusControl, 1, theme.Border, theme.Input)
	canvas.Label("Native GPU", gfx.Rect{X: field.X + 12, Y: field.Y, Width: max(field.Width-24, 0), Height: field.Height}, gfx.TextStyle{Size: theme.Metrics.FontSizeM, Color: theme.Foreground})
	sheet.Actions(layout, components.ButtonAction{ID: sheetSave, Label: "Save changes"})
	sheet.End()

	if canvas.Clicked(sheetDismiss) || canvas.Clicked(sheetClose) || canvas.Clicked(sheetSave) {
		state.flags &^= 1
	}
}
