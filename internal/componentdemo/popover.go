package componentdemo

import (
	"github.com/vivaui/viva/components"
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
        Layer: 3, Scope: scope, DismissID: dismissID, Side: components.PopoverBottom,
        Align: components.PopoverAlignStart, SideOffset: 8, AnchorDismiss: true,
    }).Paint("Native surface", "Clamped to the viewport without timers.")
}
if canvas.Clicked(dismissID) { open = false }`},
		{ID: "custom-body", Title: "自定义内容", Description: "使用 Begin/End 槽位在浮层正文中追加状态。", Snippet: `label := "Diagnostics disabled"
if enabled {
    label = "Diagnostics enabled"
}
popover := components.BeginPopover(canvas, theme, viewport, anchor, components.PopoverOptions{
    Layer: 3, Scope: scope, DismissID: dismissID, ContentSize: gfx.Size{Width: 288}, BodyHeight: 34,
})
layout := popover.Begin("Build status", "The stable host owns this overlay state.")
if components.BeginButton(canvas, theme, actionID, layout.Body).Secondary(label).Clicked() {
    enabled = !enabled
}
popover.End()`},
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
	open := state.flags&1 != 0
	var anchor ui.Box
	state.drawPage(canvas, theme, size, 560, "Popover", func(c ui.C) {
		label := "Open details"
		if open {
			label = "Close details"
		}
		anchor = ui.Button(c, label).Outline().Self(ui.Start).Hit(popoverTrigger, label)
		if anchor.Clicked() {
			state.flags ^= 1
		}
		if !open {
			caption(c, state.Text("点击触发器打开锚定浮层。", "Click the trigger to open the anchored popover."))
		}
	})
	if !open {
		return
	}

	contentWidth := min(size.Width-48, float32(288))
	bodyHeight := float32(0)
	if state.exampleID == "custom-body" {
		bodyHeight = 34
	}
	popover := components.BeginPopover(canvas, theme, gfx.Rect{Width: size.Width, Height: size.Height}, anchor.Rect(), components.PopoverOptions{
		Layer: 3, Scope: popoverScope, DismissID: popoverDismiss,
		Side: components.PopoverBottom, Align: components.PopoverAlignStart, SideOffset: 8,
		AnchorDismiss: true,
		ContentSize:   gfx.Size{Width: contentWidth}, BodyHeight: bodyHeight,
	})
	if state.exampleID == "custom-body" {
		layout := popover.Begin("Build status", "The stable host owns this overlay state.")
		status := "Diagnostics disabled"
		if state.flags&2 != 0 {
			status = "Diagnostics enabled"
		}
		bounds := gfx.Rect{X: layout.Body.X, Y: layout.Body.Y, Width: layout.Body.Width, Height: 34}
		if components.BeginButton(canvas, theme, popoverAction, bounds).Secondary(status).Clicked() {
			state.flags ^= 2
		}
		popover.End()
	} else {
		popover.Paint("Native surface", "Clamped to the current viewport without timers.")
	}
	if canvas.Clicked(popoverDismiss) {
		state.flags &^= 1
	}
}
