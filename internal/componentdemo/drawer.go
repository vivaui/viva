package componentdemo

import (
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
if canvas.Clicked(saveID) || canvas.Clicked(closeID) { open = false }`, PageSnippet: `open := true
if !open {
    if ui.Button(c, "Open drawer").Clicked() {
        open = true
    }
}
ui.Drawer(c, &open, "Workspace settings", ui.DrawerSpec{
    Side: ui.DrawerBottom, Size: 248,
    Description: "Applies immediately.",
}, func() {
    ui.Text(c, "Native settings.")
}, func() {
    if ui.Button(c, "Save changes").Clicked() {
        open = false
    }
})`, PageSnippetZh: `open := true
if !open {
    if ui.Button(c, "打开抽屉").Clicked() {
        open = true
    }
}
ui.Drawer(c, &open, "工作区设置", ui.DrawerSpec{
    Side: ui.DrawerBottom, Size: 248,
    Description: "立即生效。",
}, func() {
    ui.Text(c, "原生设置。")
}, func() {
    if ui.Button(c, "保存更改").Clicked() {
        open = false
    }
})`},
		{ID: "sides", Title: "侧边抽屉", Description: "右侧抽屉使用相同的模态与焦点契约。", Snippet: `drawer := components.BeginDrawer(canvas, theme, viewport, components.DrawerOptions{
    Layer: 2, Scope: scope, CloseID: closeID,
    Side: components.DrawerRight, ContentExtent: min(300, size.Width*0.78),
})
layout := drawer.Begin("Workspace settings", "Same modal contract on the right edge.")
canvas.Label("Native settings.", layout.Body, labelStyle)
drawer.Actions(layout, components.ButtonAction{ID: saveID, Label: "Save changes"})
drawer.End()
if canvas.Clicked(saveID) || canvas.Clicked(closeID) { open = false }`, PageSnippet: `open := true
if !open {
    if ui.Button(c, "Open drawer").Clicked() {
        open = true
    }
}
ui.Drawer(c, &open, "Workspace settings", ui.DrawerSpec{
    Side: ui.DrawerRight, Size: 300,
    Description: "Same modal contract on the right edge.",
}, func() {
    ui.Text(c, "Native settings.")
}, func() {
    if ui.Button(c, "Save changes").Clicked() {
        open = false
    }
})`, PageSnippetZh: `open := true
if !open {
    if ui.Button(c, "打开抽屉").Clicked() {
        open = true
    }
}
ui.Drawer(c, &open, "工作区设置", ui.DrawerSpec{
    Side: ui.DrawerRight, Size: 300,
    Description: "右侧边缘使用相同的模态契约。",
}, func() {
    ui.Text(c, "原生设置。")
}, func() {
    if ui.Button(c, "保存更改").Clicked() {
        open = false
    }
})`},
		{ID: "disabled", Title: "禁用抽屉", Description: "保留可见结构但关闭交互。", Snippet: `drawer := components.BeginDrawer(canvas, theme, viewport, components.DrawerOptions{
    Layer: 2, Scope: scope, CloseID: closeID, Side: components.DrawerBottom,
}).WithHandle().Disabled()
layout := drawer.Begin("Workspace settings", "Structure remains visible; interaction is off.")
drawer.Actions(layout, components.ButtonAction{ID: saveID, Label: "Save changes"})
drawer.End()`, PageSnippet: `open := true
if !open {
    ui.Button(c, "Open drawer").Disabled(true)
}
ui.Drawer(c, &open, "Workspace settings", ui.DrawerSpec{
    Side: ui.DrawerBottom, Size: 248, Disabled: true,
    Description: "Structure remains visible; interaction is off.",
}, func() {
    ui.Text(c, "Native settings.")
}, func() {
    ui.Button(c, "Save changes").Disabled(true)
})`, PageSnippetZh: `open := true
if !open {
    ui.Button(c, "打开抽屉").Disabled(true)
}
ui.Drawer(c, &open, "工作区设置", ui.DrawerSpec{
    Side: ui.DrawerBottom, Size: 248, Disabled: true,
    Description: "结构保持可见，交互已关闭。",
}, func() {
    ui.Text(c, "原生设置。")
}, func() {
    ui.Button(c, "保存更改").Disabled(true)
})`},
	},
}

var (
	drawerUI   = tree.Feature("drawer")
	drawerOpen = tree.ID(drawerUI, "open")
	drawerSave = tree.ID(drawerUI, "save")
)

func (state *State) initializeDrawer() { state.flags |= 1 }

func (state *State) paintDrawer(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	state.drawPage(canvas, theme, size, 480, state.Text("Drawer · 边缘模态", "Drawer · Edge modal"), func(c ui.C) {
		// The drawer's open state lives on the page; the demo's flag mirrors it so
		// the surrounding state stays inspectable.
		open := ui.State[bool](c, "open")
		seeded := ui.State[bool](c, "seeded")
		if !*seeded {
			*open = state.flags&1 != 0
			*seeded = true
		}
		if !*open {
			label := state.Text("打开抽屉", "Open drawer")
			if state.exampleID == "disabled" {
				ui.Button(c, label).Self(ui.Start).Hit(drawerOpen, label).Disabled(true)
			} else if ui.Button(c, label).Self(ui.Start).Hit(drawerOpen, label).Clicked() {
				*open = true
			}
		}
		spec := ui.DrawerSpec{
			Side: ui.DrawerBottom, Size: 248,
			Description: state.Text("立即生效。", "Applies immediately."),
		}
		title, body := state.Text("工作区设置", "Workspace settings"), state.Text("原生设置。", "Native settings.")
		switch state.exampleID {
		case "sides":
			spec.Side, spec.Size = ui.DrawerRight, 300
			spec.Description = state.Text("右侧边缘使用相同的模态契约。", "Same modal contract on the right edge.")
		case "disabled":
			spec.Disabled = true
			spec.Description = state.Text("结构保持可见，交互已关闭。", "Structure remains visible; interaction is off.")
		}
		ui.Drawer(c, open, title, spec, func() {
			ui.Text(c, body)
		}, func() {
			save := state.Text("保存更改", "Save changes")
			if state.exampleID == "disabled" {
				ui.Button(c, save).Hit(drawerSave, save).Disabled(true)
				return
			}
			if ui.Button(c, save).Hit(drawerSave, save).Clicked() {
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
