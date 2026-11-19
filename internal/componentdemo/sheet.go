package componentdemo

import (
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
if canvas.Clicked(saveID) || canvas.Clicked(closeID) || canvas.Clicked(dismissID) { open = false }`, PageSnippet: `m.backend = "Native GPU"
open := false
if !open {
    if ui.Button(c, "Open settings").Clicked() {
        open = true
    }
}
ui.Drawer(c, &open, "Project settings", ui.DrawerSpec{
    Side: ui.DrawerRight, Size: 360,
    Description: "Changes remain caller-owned until Save.",
}, func() {
    ui.TextField(c, &m.backend, "Rendering backend", "")
}, func() {
    if ui.Button(c, "Save changes").Clicked() {
        open = false
    }
})`, PageSnippetZh: `m.backend = "原生 GPU"
open := false
if !open {
    if ui.Button(c, "打开设置").Clicked() {
        open = true
    }
}
ui.Drawer(c, &open, "项目设置", ui.DrawerSpec{
    Side: ui.DrawerRight, Size: 360,
    Description: "保存前更改由调用方持有。",
}, func() {
    ui.TextField(c, &m.backend, "渲染后端", "")
}, func() {
    if ui.Button(c, "保存更改").Clicked() {
        open = false
    }
})`},
		{ID: "bottom", Title: "底部面板", Description: "复用同一 API 展示底部边缘布局。", Snippet: `sheet := components.BeginSheet(canvas, theme, viewport, components.SheetOptions{
    Layer: 5, Scope: scope, DismissID: dismissID, CloseID: closeID,
    Side: components.SheetBottom, ContentExtent: min(size.Height*0.62, 230),
})
layout := sheet.Begin("Project settings", "Same API, bottom edge.")
canvas.Label("Native GPU", layout.Body, labelStyle)
sheet.Actions(layout, components.ButtonAction{ID: saveID, Label: "Save changes"})
sheet.End()
if canvas.Clicked(saveID) || canvas.Clicked(closeID) || canvas.Clicked(dismissID) { open = false }`, PageSnippet: `m.backend = "Native GPU"
open := false
if !open {
    if ui.Button(c, "Open settings").Clicked() {
        open = true
    }
}
ui.Drawer(c, &open, "Project settings", ui.DrawerSpec{
    Side: ui.DrawerBottom, Size: 230,
    Description: "Same API, bottom edge.",
}, func() {
    ui.TextField(c, &m.backend, "Rendering backend", "")
}, func() {
    if ui.Button(c, "Save changes").Clicked() {
        open = false
    }
})`, PageSnippetZh: `m.backend = "原生 GPU"
open := false
if !open {
    if ui.Button(c, "打开设置").Clicked() {
        open = true
    }
}
ui.Drawer(c, &open, "项目设置", ui.DrawerSpec{
    Side: ui.DrawerBottom, Size: 230,
    Description: "同一 API，底部边缘。",
}, func() {
    ui.TextField(c, &m.backend, "渲染后端", "")
}, func() {
    if ui.Button(c, "保存更改").Clicked() {
        open = false
    }
})`},
	},
}

var (
	sheetUI      = tree.Feature("sheet")
	sheetTrigger = tree.ID(sheetUI, "trigger")
	sheetSave    = tree.ID(sheetUI, "save")
)

func (state *State) paintSheet(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	state.drawPage(canvas, theme, size, 560, "Project settings", func(c ui.C) {
		ui.Text(c, state.Text("留在当前视图编辑。", "Edit without leaving this view.")).FontSize(theme.Metrics.FontSizeS).Muted()
		open := ui.State[bool](c, "open")
		seeded := ui.State[bool](c, "seeded")
		backend := ui.State[string](c, "backend")
		if !*seeded {
			*open = state.flags&1 != 0
			*backend = state.Text("原生 GPU", "Native GPU")
			*seeded = true
		}
		if !*open {
			label := state.Text("打开设置", "Open settings")
			if ui.Button(c, label).Self(ui.Start).Hit(sheetTrigger, label).Clicked() {
				*open = true
			}
		}
		spec := ui.DrawerSpec{
			Side: ui.DrawerRight, Size: 360,
			Description: state.Text("保存前更改由调用方持有。", "Changes remain caller-owned until Save."),
		}
		title := state.Text("项目设置", "Project settings")
		if state.exampleID == "bottom" {
			spec.Side, spec.Size = ui.DrawerBottom, 230
			spec.Description = state.Text("同一 API，底部边缘。", "Same API, bottom edge.")
		}
		ui.Drawer(c, open, title, spec, func() {
			ui.TextField(c, backend, state.Text("渲染后端", "Rendering backend"), "")
		}, func() {
			save := state.Text("保存更改", "Save changes")
			if ui.Button(c, save).Hit(sheetSave, save).Clicked() {
				*open = false
			}
		})
		if *open {
			state.flags |= 1
		} else {
			state.flags &^= 1
		}
	})
}
