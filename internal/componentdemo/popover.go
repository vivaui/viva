package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var popoverComponent = Component{
	ID: "popover", Title: "Popover", Category: "Overlay", Summary: "锚定、可关闭且不启动后台工作的非模态浮层。",
	paint: (*State).paintPopover,
	Examples: []Example{
		{ID: "default", Title: "锚定浮层", Description: "点击触发器切换底部居中的生产 Popover。", Snippet: `trigger := ui.Button(c, "Open details").Outline().Self(ui.Start).Hit(tree.ID(page, "trigger"), "Open details")
if trigger.Clicked() {
    open = !open
}
if open {
    components.BeginPopover(canvas, theme, viewport, trigger.Rect(), components.PopoverOptions{
        Layer: 3, Scope: m.scope, DismissID: m.dismissID, Side: components.PopoverBottom,
        Align: components.PopoverAlignStart, SideOffset: 8, AnchorDismiss: true,
    }).Paint("Native surface", "Clamped to the viewport without timers.")
}
if canvas.Clicked(m.dismissID) { open = false }`, PageSnippet: `open := false
trigger := ui.Button(c, "Open details").Outline().Self(ui.Start)
if trigger.Clicked() {
    open = !open
}
ui.Popover(c, trigger, &open, ui.PopoverSpec{
    Title: "Native surface",
    Description: "Clamped to the current viewport without timers.",
    Scope: m.scope, DismissID: m.dismissID,
}, func() {})`, PageSnippetZh: `open := false
trigger := ui.Button(c, "Open details").Outline().Self(ui.Start)
if trigger.Clicked() {
    open = !open
}
ui.Popover(c, trigger, &open, ui.PopoverSpec{
    Title: "Native surface",
    Description: "Clamped to the current viewport without timers.",
    Scope: m.scope, DismissID: m.dismissID,
}, func() {})`},
		{ID: "custom-body", Title: "自定义内容", Description: "使用 Begin/End 槽位在浮层正文中追加状态。", Snippet: `label := "Diagnostics disabled"
if enabled {
    label = "Diagnostics enabled"
}
popover := components.BeginPopover(canvas, theme, viewport, anchor, components.PopoverOptions{
    Layer: 3, Scope: m.scope, DismissID: m.dismissID, ContentSize: gfx.Size{Width: 288}, BodyHeight: 34,
})
layout := popover.Begin("Build status", "The stable host owns this overlay state.")
if components.BeginButton(canvas, theme, actionID, layout.Body).Secondary(label).Clicked() {
    enabled = !enabled
}
popover.End()`, PageSnippet: `open := false
trigger := ui.Button(c, "Open details").Outline().Self(ui.Start)
if trigger.Clicked() {
    open = !open
}
enabled := false
ui.Popover(c, trigger, &open, ui.PopoverSpec{
    Title: "Build status",
    Description: "The stable host owns this overlay state.",
    Scope: m.scope, DismissID: m.dismissID,
}, func() {
    label := "Diagnostics disabled"
    if enabled {
        label = "Diagnostics enabled"
    }
    if ui.Button(c, label).Secondary().Clicked() {
        enabled = !enabled
    }
})`, PageSnippetZh: `open := false
trigger := ui.Button(c, "Open details").Outline().Self(ui.Start)
if trigger.Clicked() {
    open = !open
}
enabled := false
ui.Popover(c, trigger, &open, ui.PopoverSpec{
    Title: "Build status",
    Description: "The stable host owns this overlay state.",
    Scope: m.scope, DismissID: m.dismissID,
}, func() {
    label := "Diagnostics disabled"
    if enabled {
        label = "Diagnostics enabled"
    }
    if ui.Button(c, label).Secondary().Clicked() {
        enabled = !enabled
    }
})`},
	},
}

var (
	popoverUI      = tree.Feature("popover")
	popoverTrigger = tree.ID(popoverUI, "trigger")
	popoverAction  = tree.ID(popoverUI, "action")
	popoverDismiss = tree.ID(popoverUI, "dismiss")
	popoverScope   = tree.ID(popoverUI, "content")
)

func (state *State) paintPopover(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	state.drawPage(canvas, theme, size, 560, "Popover", func(c ui.C) {
		demo := ui.State[popoverDemo](c, "popover-demo")
		label := state.Text("打开详情", "Open details")
		if demo.open {
			label = state.Text("关闭详情", "Close details")
		}
		trigger := ui.Button(c, label).Outline().Self(ui.Start).Hit(popoverTrigger, label)
		if trigger.Clicked() {
			demo.open = !demo.open
		}
		title, description := "Native surface", "Clamped to the current viewport without timers."
		if state.exampleID == "custom-body" {
			title, description = "Build status", "The stable host owns this overlay state."
		}
		ui.Popover(c, trigger, &demo.open, ui.PopoverSpec{
			Title: title, Description: description,
			Scope: popoverScope, DismissID: popoverDismiss,
		}, func() {
			if state.exampleID != "custom-body" {
				return
			}
			status := "Diagnostics disabled"
			if demo.enabled {
				status = "Diagnostics enabled"
			}
			if ui.Button(c, status).Secondary().Hit(popoverAction, status).Clicked() {
				demo.enabled = !demo.enabled
			}
		})
		if !demo.open {
			caption(c, state.Text("点击触发器打开锚定浮层。", "Click the trigger to open the anchored popover."))
		}
		state.flags &^= 1
		if demo.open {
			state.flags |= 1
		}
		state.flags &^= 2
		if demo.enabled {
			state.flags |= 2
		}
	})
}

// popoverDemo is the caller-owned open and body state of the page-first Popover.
type popoverDemo struct{ open, enabled bool }
